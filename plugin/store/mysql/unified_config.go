package sqldb

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"

	configv1 "github.com/pole-io/specification/source/go/api/v1/config"
	apimodel "github.com/pole-io/specification/source/go/api/v1/model"

	"github.com/pole-io/pole-server/apis/store"
)

const unifiedConfigScope = "config"

var _ store.UnifiedConfigStore = (*stableStore)(nil)
var _ store.UnifiedConfigEpochRotator = (*stableStore)(nil)
var _ store.UnifiedConfigRuntimeReader = (*stableStore)(nil)

// ensureUnifiedConfigSchema 只创建新聚合表；重启复用持久 epoch，不修改旧配置表。
func ensureUnifiedConfigSchema(db *BaseDB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS unified_config_file_template (key_hash BINARY(32) NOT NULL PRIMARY KEY, group_name VARCHAR(128) NOT NULL, config_name VARCHAR(128) NOT NULL, payload MEDIUMBLOB NOT NULL, revision BIGINT UNSIGNED NOT NULL) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin`,
		`CREATE TABLE IF NOT EXISTS unified_config_file_environment (environment VARCHAR(128) NOT NULL, group_name VARCHAR(128) NOT NULL, config_name VARCHAR(128) NOT NULL, key_hash BINARY(32) NOT NULL, PRIMARY KEY (environment, group_name, config_name)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin`,

		`CREATE TABLE IF NOT EXISTS unified_config_state (
			key_hash BINARY(32) NOT NULL,
			environment TEXT NOT NULL,
			group_name TEXT NOT NULL,
			config_name TEXT NOT NULL,
			state MEDIUMBLOB NOT NULL,
			PRIMARY KEY (key_hash)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin`,
		`CREATE TABLE IF NOT EXISTS unified_config_clock (
			scope VARCHAR(32) NOT NULL,
			epoch CHAR(36) NOT NULL,
			position BIGINT UNSIGNED NOT NULL,
			PRIMARY KEY (scope)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin`,
		`CREATE TABLE IF NOT EXISTS unified_config_event (
			scope VARCHAR(32) NOT NULL,
			epoch CHAR(36) NOT NULL,
			position BIGINT UNSIGNED NOT NULL,
			environment TEXT NOT NULL,
			group_name TEXT NOT NULL,
			config_name TEXT NOT NULL,
			kind TEXT NOT NULL,
			actor TEXT NOT NULL,
			PRIMARY KEY (scope, epoch, position)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin`,
		`CREATE TABLE IF NOT EXISTS unified_config_release (
			key_hash BINARY(32) NOT NULL,
			release_id VARBINARY(128) NOT NULL,
			release_data MEDIUMBLOB NOT NULL,
			PRIMARY KEY (key_hash, release_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin`,
	}
	for _, statement := range statements {
		if _, err := db.DB.Exec(statement); err != nil {
			return err
		}
	}
	epoch, err := uuid.NewRandom()
	if err != nil {
		return err
	}
	_, err = db.DB.Exec(`INSERT INTO unified_config_clock (scope, epoch, position) VALUES (?, ?, 0)
		ON DUPLICATE KEY UPDATE scope = VALUES(scope)`, unifiedConfigScope, epoch.String())
	return err
}

// ReadUnifiedConfig 返回主库同一快照中的聚合和游标；缺失 key 返回仅含 key 的空聚合。
// 例如 clock=7 且 key 尚未创建 → 空聚合、cursor=7；不会使用从库旧状态搭配新游标。
func (s *stableStore) ReadUnifiedConfig(ctx context.Context, key *configv1.ConfigKey) (*store.UnifiedConfigSnapshot, error) {
	return s.readUnifiedConfig(ctx, key, false)
}

// ReadUnifiedConfigRuntime 只补齐 active 成员的不可变快照；header、release、cursor 同事务读取。
// 例如历史有 40 个大版本但仅正式版生效 → 加载 1 份 release，无草稿和 inactive 正文。
func (s *stableStore) ReadUnifiedConfigRuntime(ctx context.Context, key *configv1.ConfigKey) (*store.UnifiedConfigSnapshot, error) {
	return s.readUnifiedConfig(ctx, key, true)
}

func (s *stableStore) readUnifiedConfig(ctx context.Context, key *configv1.ConfigKey, runtime bool) (*store.UnifiedConfigSnapshot, error) {
	key, keyHash, err := unifiedConfigIdentity(key)
	if err != nil {
		return nil, err
	}
	tx, err := s.master.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	cursor, err := readUnifiedConfigClock(ctx, tx, false)
	if err != nil {
		return nil, err
	}
	state, err := readUnifiedConfigState(ctx, tx, key, keyHash, false, runtime)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &store.UnifiedConfigSnapshot{State: state, Cursor: cursor}, nil
}

// UpdateUnifiedConfig 按 namespace → clock → state 固定锁序执行回调。
// Namespace 共享锁与环境删除的排他锁互斥，阻止缓存尚未刷新时向已删除环境写入；
// 时钟锁固定所有运行时提交顺序，状态、事件和游标同事务提交。
// 输入 Runtime=false → 只写草稿且 cursor 不变；回调或任何 SQL 失败 → 整体回滚。
func (s *stableStore) UpdateUnifiedConfig(ctx context.Context, key *configv1.ConfigKey, mutate store.UnifiedConfigMutation) (*store.UnifiedConfigSnapshot, error) {
	key, keyHash, err := unifiedConfigIdentity(key)
	if err != nil {
		return nil, err
	}
	if mutate == nil {
		return nil, errors.New("unified config mutation is required")
	}
	tx, err := s.master.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err := lockUnifiedConfigNamespace(ctx, tx, key.Environment); err != nil {
		return nil, err
	}
	cursor, err := readUnifiedConfigClock(ctx, tx, true)
	if err != nil {
		return nil, err
	}
	state, err := readUnifiedConfigState(ctx, tx, key, keyHash, true, false)
	if err != nil {
		return nil, err
	}
	next := state.Clone()
	change, err := mutate(next)
	if err != nil {
		return nil, err
	}
	if change != nil {
		if !sameUnifiedConfigKey(next.GetKey(), key) {
			return nil, store.ErrUnifiedConfigKeyInvalid
		}
		if err := writeUnifiedConfigState(ctx, tx, key, keyHash, state, next); err != nil {
			return nil, err
		}
		if change.Runtime {
			if cursor.Position == math.MaxUint64 {
				return nil, errors.New("unified config cursor exhausted; rotate epoch before writing")
			}
			cursor.Position++
			if _, err := tx.ExecContext(ctx, `INSERT INTO unified_config_event
				(scope, epoch, position, environment, group_name, config_name, kind, actor)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, cursor.Scope, cursor.Epoch, cursor.Position,
				key.Environment, key.Group, key.Name, change.Kind, change.Actor); err != nil {
				return nil, err
			}
			if _, err := tx.ExecContext(ctx, `UPDATE unified_config_clock SET position = ? WHERE scope = ?`, cursor.Position, cursor.Scope); err != nil {
				return nil, err
			}
		}
		// 返回完整内部聚合，由领域层生成有界管理投影；不共享回调引用。
		state = next.Clone()
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &store.UnifiedConfigSnapshot{State: state, Cursor: cursor}, nil
}

// ReadUnifiedConfigEvents 从 after 之后重放，nil 从当前 epoch 的起点开始。
// 返回第二项是本次快照的高水位；分页有数据时必须用最后一个 event.Cursor 续读。
// scope/epoch 不匹配或 position 超前会明确拒绝，调用方应重新读取完整快照。
func (s *stableStore) ReadUnifiedConfigEvents(ctx context.Context, after *configv1.Cursor, limit uint32) ([]*store.UnifiedConfigEvent, *configv1.Cursor, error) {
	if limit == 0 || limit > 1000 {
		limit = 1000
	}
	tx, err := s.master.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()
	cursor, err := readUnifiedConfigClock(ctx, tx, false)
	if err != nil {
		return nil, nil, err
	}
	var position uint64
	if after != nil {
		if after.Scope != cursor.Scope || after.Epoch != cursor.Epoch || after.Position > cursor.Position {
			return nil, cursor, store.ErrUnifiedConfigCursorInvalid
		}
		position = after.Position
	}
	rows, err := tx.QueryContext(ctx, `SELECT position, environment, group_name, config_name, kind, actor
		FROM unified_config_event WHERE scope = ? AND epoch = ? AND position > ? AND position <= ?
		ORDER BY position ASC LIMIT ?`, cursor.Scope, cursor.Epoch, position, cursor.Position, limit)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	events := make([]*store.UnifiedConfigEvent, 0)
	for rows.Next() {
		event := &store.UnifiedConfigEvent{Cursor: &configv1.Cursor{Scope: cursor.Scope, Epoch: cursor.Epoch}, Key: &configv1.ConfigKey{}}
		if err := rows.Scan(&event.Cursor.Position, &event.Key.Environment, &event.Key.Group, &event.Key.Name, &event.Kind, &event.Actor); err != nil {
			return nil, nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}
	return events, cursor, nil
}

// RotateUnifiedConfigEpoch 是数据库恢复后的显式换代入口；保留聚合，清除旧事件并归零游标。
// 恢复工具必须在重新开放服务前调用；随机持久 epoch 使旧客户端不能把恢复后的状态当作旧时间线。
func (s *stableStore) RotateUnifiedConfigEpoch(ctx context.Context) (*configv1.Cursor, error) {
	epoch, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}
	tx, err := s.master.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := readUnifiedConfigClock(ctx, tx, true); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM unified_config_event WHERE scope = ?`, unifiedConfigScope); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE unified_config_clock SET epoch = ?, position = 0 WHERE scope = ?`, epoch.String(), unifiedConfigScope); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &configv1.Cursor{Scope: unifiedConfigScope, Epoch: epoch.String()}, nil
}

func readUnifiedConfigClock(ctx context.Context, tx *sql.Tx, lock bool) (*configv1.Cursor, error) {
	query := `SELECT epoch, position FROM unified_config_clock WHERE scope = ?`
	if lock {
		query += " FOR UPDATE"
	}
	cursor := &configv1.Cursor{Scope: unifiedConfigScope}
	if err := tx.QueryRowContext(ctx, query, unifiedConfigScope).Scan(&cursor.Epoch, &cursor.Position); err != nil {
		return nil, err
	}
	return cursor, nil
}

func readUnifiedConfigState(ctx context.Context, tx *sql.Tx, key *configv1.ConfigKey, keyHash []byte, lock, runtime bool) (*store.UnifiedConfigAggregate, error) {
	query := `SELECT state FROM unified_config_state WHERE key_hash = ?`
	if lock {
		query += " FOR UPDATE"
	}
	var payload []byte
	if err := tx.QueryRowContext(ctx, query, keyHash).Scan(&payload); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &store.UnifiedConfigAggregate{Key: proto.Clone(key).(*configv1.ConfigKey)}, nil
		}
		return nil, err
	}
	state := &store.UnifiedConfigAggregate{}
	if err := json.Unmarshal(payload, state); err != nil {
		return nil, fmt.Errorf("decode unified config state: %w", err)
	}
	if !sameUnifiedConfigKey(state.Key, key) {
		return nil, errors.New("unified config stored identity mismatch")
	}
	if runtime {
		state.Draft = nil
		active := make([]*store.UnifiedConfigMember, 0)
		for _, member := range state.Members {
			if member.GetActive() {
				active = append(active, member)
			}
		}
		state.Members = active
	}
	if err := hydrateUnifiedConfigReleases(ctx, tx, keyHash, state, runtime); err != nil {
		return nil, err
	}
	return state, nil
}

func hydrateUnifiedConfigReleases(ctx context.Context, tx *sql.Tx, keyHash []byte, state *store.UnifiedConfigAggregate, runtime bool) error {
	if len(state.Members) == 0 {
		return nil
	}
	references := make(map[string]*store.UnifiedConfigMember, len(state.Members))
	args := []any{keyHash}
	for _, member := range state.Members {
		id := member.GetRelease().GetId()
		if id == "" || references[id] != nil {
			return errors.New("invalid unified config release reference")
		}
		references[id] = member
		args = append(args, id)
	}
	query := `SELECT release_id, release_data FROM unified_config_release WHERE key_hash = ?`
	if runtime {
		query += " AND release_id IN (" + strings.TrimSuffix(strings.Repeat("?,", len(state.Members)), ",") + ")"
	} else {
		args = args[:1]
	}
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var data []byte
		if err := rows.Scan(&id, &data); err != nil {
			return err
		}
		member := references[id]
		if member == nil {
			continue
		}
		release := &configv1.ConfigRelease{}
		if err := proto.Unmarshal(data, release); err != nil {
			return fmt.Errorf("decode unified config release: %w", err)
		}
		if release.Id != id || !sameUnifiedConfigKey(release.Key, state.Key) {
			return errors.New("unified config release identity mismatch")
		}
		member.Release = release
		delete(references, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(references) != 0 {
		return errors.New("unified config release reference is missing")
	}
	return rows.Close()
}

// writeUnifiedConfigState 将新增 release 独立保存；既有快照不可更新或删除。
// header 只保留 draft、集合元数据及 ID/active 引用，避免每次发布重写全部历史正文。
func writeUnifiedConfigState(ctx context.Context, tx *sql.Tx, key *configv1.ConfigKey, keyHash []byte, previous, next *store.UnifiedConfigAggregate) error {
	existing := make(map[string]*configv1.ConfigRelease, len(previous.Members))
	for _, member := range previous.Members {
		existing[member.Release.Id] = member.Release
	}
	header := next.Clone()
	seen := make(map[string]bool, len(next.Members))
	for index, member := range next.Members {
		release := member.GetRelease()
		id := release.GetId()
		if id == "" || len(id) > 128 || seen[id] || !sameUnifiedConfigKey(release.GetKey(), key) {
			return errors.New("invalid unified config release identity")
		}
		seen[id] = true
		if prior := existing[id]; prior != nil {
			if !proto.Equal(prior, release) {
				return errors.New("unified config release snapshot is immutable")
			}
			delete(existing, id)
		} else {
			data, err := proto.Marshal(release)
			if err != nil {
				return fmt.Errorf("encode unified config release: %w", err)
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO unified_config_release (key_hash, release_id, release_data)
				VALUES (?, ?, ?)`, keyHash, id, data); err != nil {
				return err
			}
		}
		header.Members[index].Release = &configv1.ConfigRelease{Id: id}
	}
	if len(existing) > 0 {
		return errors.New("unified config release history cannot be removed")
	}
	payload, err := json.Marshal(header)
	if err != nil {
		return fmt.Errorf("encode unified config state: %w", err)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO unified_config_state
		(key_hash, environment, group_name, config_name, state) VALUES (?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE state = VALUES(state)`, keyHash, key.Environment, key.Group, key.Name, payload)
	return err
}

func unifiedConfigIdentity(key *configv1.ConfigKey) (*configv1.ConfigKey, []byte, error) {
	if key == nil || key.Environment == "" || key.Group == "" || key.Name == "" ||
		!utf8.ValidString(key.Environment) || !utf8.ValidString(key.Group) || !utf8.ValidString(key.Name) {
		return nil, nil, store.ErrUnifiedConfigKeyInvalid
	}
	// JSON 数组保留字段边界，避免 a/b+c 与 a+b/c 这类分隔符碰撞；只使用领域标识字段。
	key = &configv1.ConfigKey{Environment: key.Environment, Group: key.Group, Name: key.Name}
	canonical, err := json.Marshal([]string{key.Environment, key.Group, key.Name})
	if err != nil {
		return nil, nil, err
	}
	hash := sha256.Sum256(canonical)
	return key, hash[:], nil
}

func sameUnifiedConfigKey(left, right *configv1.ConfigKey) bool {
	return left != nil && right != nil && left.Environment == right.Environment && left.Group == right.Group && left.Name == right.Name
}

// lockUnifiedConfigNamespace 使用当前读并持锁到配置事务结束；单纯先查缓存或 COUNT 无法排除并发删除。
func lockUnifiedConfigNamespace(ctx context.Context, tx *sql.Tx, environment string) error {
	if environment == SystemNamespace {
		return store.NewStatusError(store.NotFoundNamespace, "configuration requires a valid business namespace")
	}
	var kind apimodel.NamespaceKind
	err := tx.QueryRowContext(ctx, `SELECT kind FROM namespace WHERE name = ? AND flag != 1 LOCK IN SHARE MODE`, environment).Scan(&kind)
	if errors.Is(err, sql.ErrNoRows) || err == nil && kind != apimodel.NamespaceKind_NAMESPACE_KIND_BUSINESS {
		return store.NewStatusError(store.NotFoundNamespace, "configuration requires a valid business namespace")
	}
	return err
}
