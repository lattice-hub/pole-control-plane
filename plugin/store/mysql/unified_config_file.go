package sqldb

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"

	"github.com/pole-io/pole-server/apis/store"
	configv1 "github.com/pole-io/specification/source/go/api/v1/config"
	"google.golang.org/protobuf/proto"
)

func configFileHash(k *configv1.ConfigFileKey) []byte {
	b, _ := json.Marshal([]string{k.GetGroup(), k.GetName()})
	h := sha256.Sum256(b)
	return h[:]
}
func (s *stableStore) ReadConfigFileTemplate(ctx context.Context, k *configv1.ConfigFileKey) (*configv1.ConfigFileTemplate, error) {
	var b []byte
	err := s.master.DB.QueryRowContext(ctx, `SELECT payload FROM unified_config_file_template WHERE key_hash=?`, configFileHash(k)).Scan(&b)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	v := &configv1.ConfigFileTemplate{}
	if err = proto.Unmarshal(b, v); err != nil {
		return nil, err
	}
	if v.GetKey().GetGroup() != k.GetGroup() || v.GetKey().GetName() != k.GetName() {
		return nil, errors.New("file key mismatch")
	}
	return v, nil
}
func (s *stableStore) SaveConfigFileTemplate(ctx context.Context, t *configv1.ConfigFileTemplate, expected uint64) (*configv1.ConfigFileTemplate, error) {
	if expected == math.MaxUint64 {
		return nil, store.ErrConfigFileConflict
	}
	tx, err := s.master.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	// 按稳定顺序锁定所有环境，防止绑定与环境删除并发交错。
	environments := append([]string(nil), t.Environments...)
	sort.Strings(environments)
	for _, environment := range environments {
		if err = lockUnifiedConfigNamespace(ctx, tx, environment); err != nil {
			return nil, err
		}
	}
	// 配置写入先锁 clock 再锁 state；共享正文首次创建沿用相同锁序，
	// 因 MySQL 缺失行的间隙锁彼此兼容，不能单靠共享锁阻止并发首次草稿。
	if expected == 0 {
		if _, err = readUnifiedConfigClock(ctx, tx, true); err != nil {
			return nil, err
		}
	}
	// 插入占位行使首次并发创建也由数据库唯一键串行化。
	hash := configFileHash(t.GetKey())
	_, err = tx.ExecContext(ctx, `INSERT INTO unified_config_file_template (key_hash,group_name,config_name,payload,revision) VALUES (?,?,?, ?,0) ON DUPLICATE KEY UPDATE key_hash=key_hash`, hash, t.Key.Group, t.Key.Name, []byte{})
	if err != nil {
		return nil, err
	}
	var revision uint64
	var group, name string
	err = tx.QueryRowContext(ctx, `SELECT revision,group_name,config_name FROM unified_config_file_template WHERE key_hash=? FOR UPDATE`, hash).Scan(&revision, &group, &name)
	if err != nil {
		return nil, err
	}
	if group != t.Key.Group || name != t.Key.Name {
		return nil, errors.New("file key mismatch")
	}
	if revision != expected {
		return nil, store.ErrConfigFileConflict
	}
	if revision == 0 {
		// 与草稿写入共享行锁，缺失行的间隙锁也覆盖并发首次插入。
		for _, env := range environments {
			key, stateHash, err := unifiedConfigIdentity(&configv1.ConfigKey{Environment: env, Group: t.Key.Group, Name: t.Key.Name})
			if err != nil {
				return nil, err
			}
			var stateBytes []byte
			err = tx.QueryRowContext(ctx, `SELECT state FROM unified_config_state WHERE key_hash = ? LOCK IN SHARE MODE`, stateHash).Scan(&stateBytes)
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			if err != nil {
				return nil, err
			}
			state := &store.UnifiedConfigAggregate{}
			if err = json.Unmarshal(stateBytes, state); err != nil {
				return nil, err
			}
			if !sameUnifiedConfigKey(state.Key, key) {
				return nil, errors.New("unified config stored identity mismatch")
			}
			if draft := state.GetDraft(); draft != nil && !store.SameConfigFileSharedDefinition(draft.GetDefinition(), t.GetDefinition()) {
				return nil, store.ErrConfigFileConflict
			}
		}
	}
	result := proto.Clone(t).(*configv1.ConfigFileTemplate)
	result.Revision = revision + 1
	b, err := proto.Marshal(result)
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE unified_config_file_template SET payload=?,revision=? WHERE key_hash=?`, b, result.Revision, hash)
	if err != nil {
		return nil, err
	}
	for _, env := range result.Environments {
		_, err = tx.ExecContext(ctx, `INSERT INTO unified_config_file_environment (environment,group_name,config_name,key_hash) VALUES (?,?,?,?) ON DUPLICATE KEY UPDATE key_hash=VALUES(key_hash)`, env, t.Key.Group, t.Key.Name, hash)
		if err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

type fileCursor struct{ Environment, Group, Query, LastGroup, LastName string }

func (s *stableStore) ListConfigFiles(ctx context.Context, req *configv1.ListConfigFilesRequest) (*configv1.ConfigFileList, error) {
	cursor := fileCursor{Environment: req.Environment, Group: req.Group, Query: req.Query}
	if req.PageToken != "" {
		b, err := base64.RawURLEncoding.DecodeString(req.PageToken)
		if err != nil || len(b) > 2048 {
			return nil, store.ErrUnifiedConfigCursorInvalid
		}
		var c fileCursor
		if json.Unmarshal(b, &c) != nil || c.Environment != cursor.Environment || c.Group != cursor.Group || c.Query != cursor.Query {
			return nil, store.ErrUnifiedConfigCursorInvalid
		}
		cursor = c
	}
	size := int(req.PageSize)
	if size == 0 {
		size = 50
	}
	if size > 100 {
		return nil, store.ErrUnifiedConfigCursorInvalid
	}
	// UNION 得到单环境键集合，所有过滤参数化；只读取当前环境聚合。
	rows, err := s.master.DB.QueryContext(ctx, `SELECT x.group_name,x.config_name,t.payload,st.state FROM (SELECT group_name,config_name FROM unified_config_state WHERE environment=? UNION SELECT group_name,config_name FROM unified_config_file_environment WHERE environment=?) x LEFT JOIN unified_config_file_environment e ON e.environment=? AND e.group_name=x.group_name AND e.config_name=x.config_name LEFT JOIN unified_config_file_template t ON t.key_hash=e.key_hash LEFT JOIN unified_config_state st ON st.environment=? AND st.group_name=x.group_name AND st.config_name=x.config_name WHERE (?='' OR x.group_name=?) AND (?='' OR LOCATE(?,x.group_name)>0 OR LOCATE(?,x.config_name)>0) AND (x.group_name>? OR (x.group_name=? AND x.config_name>?)) ORDER BY x.group_name,x.config_name LIMIT ?`, req.Environment, req.Environment, req.Environment, req.Environment, req.Group, req.Group, req.Query, req.Query, req.Query, cursor.LastGroup, cursor.LastGroup, cursor.LastName, size+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := &configv1.ConfigFileList{}
	for rows.Next() {
		var g, n string
		var t, b []byte
		if err = rows.Scan(&g, &n, &t, &b); err != nil {
			return nil, err
		}
		if len(list.Items) == size {
			last := list.Items[len(list.Items)-1].Key
			cursor.LastGroup = last.Group
			cursor.LastName = last.Name
			encoded, _ := json.Marshal(cursor)
			list.NextPageToken = base64.RawURLEncoding.EncodeToString(encoded)
			break
		}
		item := &configv1.ConfigFileSummary{Key: &configv1.ConfigFileKey{Group: g, Name: n}, Environment: req.Environment}
		if len(t) > 0 {
			v := &configv1.ConfigFileTemplate{}
			if err = proto.Unmarshal(t, v); err != nil {
				return nil, err
			}
			item.HasSharedTemplate = true
			item.Description = v.Description
		}
		if len(b) > 0 {
			state := &store.UnifiedConfigAggregate{}
			if err = json.Unmarshal(b, state); err != nil {
				return nil, fmt.Errorf("decode state: %w", err)
			}
			item.HasDraft = state.Draft != nil
			item.DraftRevision = state.GetDraft().GetRevision()
			item.Offline = state.Offline
			for _, m := range state.Members {
				if m.GetActive() {
					item.HasActiveRelease = true
					break
				}
			}
		}
		list.Items = append(list.Items, item)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}
