package sqldb

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	configv1 "github.com/pole-io/specification/source/go/api/v1/config"
	apimodel "github.com/pole-io/specification/source/go/api/v1/model"

	"github.com/pole-io/pole-server/apis/store"
	"github.com/pole-io/pole-server/pkg/config/unified"
)

// TestUnifiedConfigMySQLContract 默认跳过；UNIFIED_CONFIG_TEST_DSN 必须指向 loopback 上的专用测试库。
// 测试再创建唯一子库，只清理自己创建的库，不通过现有集成测试的共享 MYSQL_HOST 配置连接。
func TestUnifiedConfigMySQLContract(t *testing.T) {
	dsn := os.Getenv("UNIFIED_CONFIG_TEST_DSN")
	if dsn == "" {
		t.Skip("set UNIFIED_CONFIG_TEST_DSN to an isolated local MySQL test database")
	}
	cfg, err := mysqldriver.ParseDSN(dsn)
	require.NoError(t, err)
	host, _, err := net.SplitHostPort(cfg.Addr)
	require.NoError(t, err)
	address := net.ParseIP(host)
	require.True(t, cfg.Net == "tcp" && address != nil && address.IsLoopback(), "integration DSN must use a loopback TCP address")
	require.True(t, strings.HasPrefix(cfg.DBName, "unified_config_test_"), "integration DSN must name a dedicated unified_config_test_ database")
	cfg.Timeout, cfg.ReadTimeout, cfg.WriteTimeout = 5*time.Second, 10*time.Second, 10*time.Second
	admin, err := sql.Open("mysql", cfg.FormatDSN())
	require.NoError(t, err)
	t.Cleanup(func() { admin.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	require.NoError(t, admin.PingContext(ctx))
	database := "unified_config_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err = admin.ExecContext(ctx, "CREATE DATABASE `"+database+"` CHARACTER SET utf8mb4 COLLATE utf8mb4_bin")
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, cleanupErr := admin.ExecContext(cleanupCtx, "DROP DATABASE `"+database+"`")
		require.NoError(t, cleanupErr)
	})
	cfg.DBName = database
	newStore := func() *stableStore {
		db, err := sql.Open("mysql", cfg.FormatDSN())
		require.NoError(t, err)
		db.SetMaxOpenConns(1)
		t.Cleanup(func() { db.Close() })
		require.NoError(t, db.PingContext(ctx))
		return &stableStore{master: &BaseDB{DB: db}}
	}
	first, second, observer := newStore(), newStore(), newStore()
	require.NoError(t, ensureUnifiedConfigSchema(first.master))
	// 与生产 namespace 的主键、删除标记、类型及 LockNamespace 查询列一致。
	_, err = first.master.DB.ExecContext(ctx, `CREATE TABLE namespace (
 name VARCHAR(64) NOT NULL PRIMARY KEY, comment VARCHAR(1024), token VARCHAR(64) NOT NULL,
 owner VARCHAR(1024) NOT NULL, flag TINYINT NOT NULL DEFAULT 0,
 ctime TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, mtime TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
 service_export_to TEXT, metadata TEXT, kind TINYINT NOT NULL DEFAULT 0) ENGINE=InnoDB`)
	require.NoError(t, err)
	_, err = first.master.DB.ExecContext(ctx, `INSERT INTO namespace (name,token,owner) VALUES ('dev','','owner')`)
	require.NoError(t, err)
	_, err = first.master.DB.ExecContext(ctx, `INSERT INTO namespace (name,token,owner) VALUES ('file-dev','','owner'),('file-stage','','owner')`)
	require.NoError(t, err)
	t.Run("SharedTemplateReconnectAndDirectory", func(t *testing.T) {
		ctx := context.Background()
		template := &configv1.ConfigFileTemplate{Key: &configv1.ConfigFileKey{Group: "file-test", Name: "alpha"}, Environments: []string{"file-dev", "file-stage"}, Definition: &configv1.ConfigDefinition{Engine: "literal", Content: []byte("shared")}}
		saved, err := first.SaveConfigFileTemplate(ctx, template, 0)
		require.NoError(t, err)
		require.Equal(t, uint64(1), saved.Revision)
		require.NoError(t, ensureUnifiedConfigSchema(second.master))
		loaded, err := second.ReadConfigFileTemplate(ctx, template.Key)
		require.NoError(t, err)
		require.True(t, proto.Equal(saved, loaded))
		page, err := second.ListConfigFiles(ctx, &configv1.ListConfigFilesRequest{Environment: "file-stage", Group: "file-test", PageSize: 1})
		require.NoError(t, err)
		require.Len(t, page.Items, 1)
		require.True(t, page.Items[0].HasSharedTemplate)
		require.False(t, page.Items[0].HasDraft)
		count, err := observer.CountUnifiedConfigs(ctx, "file-stage")
		require.NoError(t, err)
		require.Equal(t, uint64(1), count)
		counts, err := observer.CountUnifiedConfigsByEnvironment(ctx)
		require.NoError(t, err)
		require.Equal(t, uint64(1), counts["file-stage"])

		_, err = second.SaveConfigFileTemplate(ctx, template, 0)
		require.ErrorIs(t, err, store.ErrConfigFileConflict)
		empty, err := observer.ListConfigFiles(ctx, &configv1.ListConfigFilesRequest{Environment: "other", Group: "file-test"})
		require.NoError(t, err)
		require.Empty(t, empty.Items)
	})
	key := testUnifiedKey()
	initial, err := first.ReadUnifiedConfig(ctx, key)
	require.NoError(t, err)
	require.Equal(t, uint64(0), initial.Cursor.Position)
	require.True(t, proto.Equal(key, initial.State.Key))
	_, err = uuid.Parse(initial.Cursor.Epoch)
	require.NoError(t, err)
	// 重复 schema 初始化不能换代或清空状态。
	require.NoError(t, ensureUnifiedConfigSchema(second.master))
	unchanged, err := second.ReadUnifiedConfig(ctx, key)
	require.NoError(t, err)
	require.True(t, proto.Equal(initial.Cursor, unchanged.Cursor))

	t.Run("SharedSourceDeleteWins", func(t *testing.T) {
		env := "source-delete-first"
		_, err := first.master.DB.ExecContext(ctx, `INSERT INTO namespace (name,token,owner) VALUES (?,'','owner')`, env)
		require.NoError(t, err)
		deleting, err := second.CreateTransaction()
		require.NoError(t, err)
		defer deleting.Commit()
		_, err = deleting.LockNamespace(env)
		require.NoError(t, err)
		var connection uint64
		require.NoError(t, first.master.DB.QueryRowContext(ctx, "SELECT CONNECTION_ID()").Scan(&connection))
		result := make(chan error, 1)
		go func() {
			_, err := first.SaveConfigFileTemplate(ctx, &configv1.ConfigFileTemplate{Key: &configv1.ConfigFileKey{Group: "app", Name: "x"}, Environments: []string{env}, Definition: &configv1.ConfigDefinition{Engine: "literal"}}, 0)
			result <- err
		}()
		require.NoError(t, waitUnifiedConfigLock(ctx, observer.master.DB, connection))
		require.NoError(t, deleting.DeleteNamespace(env))
		require.Error(t, <-result)
		count, err := observer.CountUnifiedConfigs(ctx, env)
		require.NoError(t, err)
		require.Zero(t, count)
	})
	t.Run("SharedSourceSaveWins", func(t *testing.T) {
		env := "source-save-first"
		key := &configv1.ConfigKey{Environment: env, Group: "app", Name: "x"}
		_, err := first.master.DB.ExecContext(ctx, `INSERT INTO namespace (name,token,owner) VALUES (?,'','owner')`, env)
		require.NoError(t, err)
		_, err = first.UpdateUnifiedConfig(ctx, key, func(state *store.UnifiedConfigAggregate) (*store.UnifiedConfigChange, error) {
			return &store.UnifiedConfigChange{}, nil
		})
		require.NoError(t, err)
		blocker, err := second.master.DB.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer blocker.Rollback()
		_, hash, err := unifiedConfigIdentity(key)
		require.NoError(t, err)
		var ignored []byte
		require.NoError(t, blocker.QueryRowContext(ctx, `SELECT state FROM unified_config_state WHERE key_hash=? FOR UPDATE`, hash).Scan(&ignored))
		var sourceConnection uint64
		require.NoError(t, first.master.DB.QueryRowContext(ctx, "SELECT CONNECTION_ID()").Scan(&sourceConnection))
		saved := make(chan error, 1)
		go func() {
			_, err := first.SaveConfigFileTemplate(ctx, &configv1.ConfigFileTemplate{Key: &configv1.ConfigFileKey{Group: key.Group, Name: key.Name}, Environments: []string{env}, Definition: &configv1.ConfigDefinition{Engine: "literal"}}, 0)
			saved <- err
		}()
		require.NoError(t, waitUnifiedConfigLock(ctx, observer.master.DB, sourceConnection))
		deleter := newStore()
		var deleteConnection uint64
		require.NoError(t, deleter.master.DB.QueryRowContext(ctx, "SELECT CONNECTION_ID()").Scan(&deleteConnection))
		deleting, err := deleter.CreateTransaction()
		require.NoError(t, err)
		defer deleting.Commit()
		locked := make(chan error, 1)
		go func() { _, err := deleting.LockNamespace(env); locked <- err }()
		require.NoError(t, waitUnifiedConfigLock(ctx, observer.master.DB, deleteConnection))
		require.NoError(t, blocker.Commit())
		require.NoError(t, <-saved)
		require.NoError(t, <-locked)
		count, err := observer.CountUnifiedConfigs(ctx, env)
		require.NoError(t, err)
		require.Equal(t, uint64(1), count)
	})
	t.Run("SharedSourceWaitsForDraftAndRejectsMismatch", func(t *testing.T) {
		env := "source-draft-first"
		key := &configv1.ConfigKey{Environment: env, Group: "app", Name: "draft-x"}
		_, err := first.master.DB.ExecContext(ctx, `INSERT INTO namespace (name,token,owner) VALUES (?,'','owner')`, env)
		require.NoError(t, err)
		acquired, release := make(chan struct{}), make(chan struct{})
		defer close(release)
		repository := &heldUnifiedMutation{stableStore: first, acquired: acquired, release: release}
		done := make(chan error, 1)
		go func() {
			_, err := repository.UpdateUnifiedConfig(ctx, key, func(state *store.UnifiedConfigAggregate) (*store.UnifiedConfigChange, error) {
				state.Draft = &configv1.ConfigDraft{Key: key, Revision: 1, Definition: &configv1.ConfigDefinition{Engine: "literal", Content: []byte("existing")}}
				return &store.UnifiedConfigChange{}, nil
			})
			done <- err
		}()
		<-acquired
		var sourceConnection uint64
		require.NoError(t, second.master.DB.QueryRowContext(ctx, "SELECT CONNECTION_ID()").Scan(&sourceConnection))
		saved := make(chan error, 1)
		go func() {
			_, err := second.SaveConfigFileTemplate(ctx, &configv1.ConfigFileTemplate{Key: &configv1.ConfigFileKey{Group: key.Group, Name: key.Name}, Environments: []string{env}, Definition: &configv1.ConfigDefinition{Engine: "literal", Content: []byte("different")}}, 0)
			saved <- err
		}()
		require.NoError(t, waitUnifiedConfigLock(ctx, observer.master.DB, sourceConnection))
		release <- struct{}{}
		require.NoError(t, <-done)
		require.ErrorIs(t, <-saved, store.ErrConfigFileConflict)
		found, err := observer.ReadConfigFileTemplate(ctx, &configv1.ConfigFileKey{Group: key.Group, Name: key.Name})
		require.NoError(t, err)
		require.Nil(t, found)
	})

	t.Run("NamespaceDeleteWinsRejectsStaleAuthorizedSave", func(t *testing.T) {
		key := &configv1.ConfigKey{Environment: "delete-first", Group: "app", Name: "config"}
		_, err := first.master.DB.ExecContext(ctx, `INSERT INTO namespace (name,token,owner) VALUES (?,'','owner')`, key.Environment)
		require.NoError(t, err)
		service, err := unified.New(first, staleNamespaceAuthorization{})
		require.NoError(t, err)
		before, err := observer.ReadUnifiedConfig(ctx, key)
		require.NoError(t, err)
		deleting, err := second.CreateTransaction()
		require.NoError(t, err)
		defer deleting.Commit()
		environment, err := deleting.LockNamespace(key.Environment)
		require.NoError(t, err)
		require.NotNil(t, environment)
		count, err := observer.CountUnifiedConfigs(ctx, key.Environment)
		require.NoError(t, err)
		require.Zero(t, count)
		var writerConnection uint64
		require.NoError(t, first.master.DB.QueryRowContext(ctx, "SELECT CONNECTION_ID()").Scan(&writerConnection))
		saved := make(chan *apimodel.Response, 1)
		go func() {
			saved <- service.SaveDraft(ctx, &configv1.SaveDraftRequest{Key: key, Definition: &configv1.ConfigDefinition{Engine: "literal", Content: []byte("draft")}})
		}()
		require.NoError(t, waitUnifiedConfigLock(ctx, observer.master.DB, writerConnection))
		require.NoError(t, deleting.DeleteNamespace(key.Environment))
		response := <-saved
		require.NotEqual(t, uint32(apimodel.Code_ExecuteSuccess), response.Code)
		require.Nil(t, response.Data)
		// 缓存仍然允许时，后续请求同样必须由当前主库反校验拒绝。
		response = service.SaveDraft(ctx, &configv1.SaveDraftRequest{Key: key, Definition: &configv1.ConfigDefinition{Engine: "literal"}})
		require.NotEqual(t, uint32(apimodel.Code_ExecuteSuccess), response.Code)
		require.Nil(t, response.Data)
		after, err := observer.ReadUnifiedConfig(ctx, key)
		require.NoError(t, err)
		require.Nil(t, after.State.Draft)
		require.True(t, proto.Equal(before.Cursor, after.Cursor))
		count, err = observer.CountUnifiedConfigs(ctx, key.Environment)
		require.NoError(t, err)
		require.Zero(t, count)
	})

	t.Run("SaveWinsDeletionWaitsAndSeesPersistedAggregate", func(t *testing.T) {
		key := &configv1.ConfigKey{Environment: "save-first", Group: "app", Name: "config"}
		_, err := first.master.DB.ExecContext(ctx, `INSERT INTO namespace (name,token,owner) VALUES (?,'','owner')`, key.Environment)
		require.NoError(t, err)
		acquired, release := make(chan struct{}), make(chan struct{})
		defer close(release)
		repository := &heldUnifiedMutation{stableStore: first, acquired: acquired, release: release}
		service, err := unified.New(repository, staleNamespaceAuthorization{})
		require.NoError(t, err)
		saved := make(chan *apimodel.Response, 1)
		go func() {
			saved <- service.SaveDraft(ctx, &configv1.SaveDraftRequest{Key: key, Definition: &configv1.ConfigDefinition{Engine: "literal", Content: []byte("draft")}})
		}()
		select {
		case <-acquired:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		var deletingConnection uint64
		require.NoError(t, second.master.DB.QueryRowContext(ctx, "SELECT CONNECTION_ID()").Scan(&deletingConnection))
		deleting, err := second.CreateTransaction()
		require.NoError(t, err)
		counted := make(chan uint64, 1)
		failures := make(chan error, 1)
		go func() {
			defer deleting.Commit()
			environment, err := deleting.LockNamespace(key.Environment)
			if err != nil {
				failures <- err
				return
			}
			if environment == nil {
				failures <- errors.New("namespace disappeared")
				return
			}
			count, err := observer.CountUnifiedConfigs(ctx, key.Environment)
			if err != nil {
				failures <- err
				return
			}
			// 与领域删除保护相同：存在任何聚合就不调用 DeleteNamespace。
			if count == 0 {
				err = deleting.DeleteNamespace(key.Environment)
			} else {
				err = deleting.Commit()
			}
			if err != nil {
				failures <- err
				return
			}
			counted <- count
		}()
		require.NoError(t, waitUnifiedConfigLock(ctx, observer.master.DB, deletingConnection))
		release <- struct{}{}
		response := <-saved
		require.Equal(t, uint32(apimodel.Code_ExecuteSuccess), response.Code)
		select {
		case count := <-counted:
			require.Equal(t, uint64(1), count)
		case err := <-failures:
			t.Fatal(err)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		var flag int
		require.NoError(t, observer.master.DB.QueryRowContext(ctx, `SELECT flag FROM namespace WHERE name=?`, key.Environment).Scan(&flag))
		require.Zero(t, flag)
		// 下线及仅历史聚合仍计入环境保护，不能把存量数据当作零。
		_, err = first.UpdateUnifiedConfig(ctx, key, func(state *store.UnifiedConfigAggregate) (*store.UnifiedConfigChange, error) {
			state.Offline = true
			return &store.UnifiedConfigChange{}, nil
		})
		require.NoError(t, err)
		count, err := observer.CountUnifiedConfigs(ctx, key.Environment)
		require.NoError(t, err)
		require.Equal(t, uint64(1), count)
	})

	t.Run("DraftAndCallbackRollback", func(t *testing.T) {
		draft, err := first.UpdateUnifiedConfig(ctx, key, func(state *store.UnifiedConfigAggregate) (*store.UnifiedConfigChange, error) {
			state.Draft = &configv1.ConfigDraft{Key: testUnifiedKey(), Revision: 1}
			return &store.UnifiedConfigChange{Kind: "draft"}, nil
		})
		require.NoError(t, err)
		require.True(t, proto.Equal(initial.Cursor, draft.Cursor))
		failure := errors.New("injected callback failure")
		_, err = second.UpdateUnifiedConfig(ctx, key, func(state *store.UnifiedConfigAggregate) (*store.UnifiedConfigChange, error) {
			state.Draft.Revision = 99
			state.ReleaseSetRevision = 99
			state.Offline = true
			return &store.UnifiedConfigChange{Runtime: true}, failure
		})
		require.ErrorIs(t, err, failure)
		after, err := first.ReadUnifiedConfig(ctx, key)
		require.NoError(t, err)
		requireUnifiedAggregateEqual(t, draft.State, after.State)
		require.True(t, proto.Equal(draft.Cursor, after.Cursor))
		events, watermark, err := first.ReadUnifiedConfigEvents(ctx, nil, 10)
		require.NoError(t, err)
		require.Empty(t, events)
		require.True(t, proto.Equal(initial.Cursor, watermark))
	})

	t.Run("EventInsertFailureRollsBackStateAndClock", func(t *testing.T) {
		before, err := first.ReadUnifiedConfig(ctx, key)
		require.NoError(t, err)
		// 让状态写入成功后的 outbox INSERT 真正失败，检验 InnoDB 回滚整笔事务。
		_, err = observer.master.DB.ExecContext(ctx, `CREATE TRIGGER unified_config_test_abort_event
			BEFORE INSERT ON unified_config_event FOR EACH ROW
			SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'injected outbox failure'`)
		require.NoError(t, err)
		defer func() {
			_, err := observer.master.DB.ExecContext(ctx, `DROP TRIGGER unified_config_test_abort_event`)
			require.NoError(t, err)
		}()
		_, err = second.UpdateUnifiedConfig(ctx, key, func(state *store.UnifiedConfigAggregate) (*store.UnifiedConfigChange, error) {
			state.Offline = true
			state.ReleaseSetRevision = 99
			return &store.UnifiedConfigChange{Runtime: true, Kind: "publish"}, nil
		})
		require.ErrorContains(t, err, "injected outbox failure")
		after, err := first.ReadUnifiedConfig(ctx, key)
		require.NoError(t, err)
		requireUnifiedAggregateEqual(t, before.State, after.State)
		require.True(t, proto.Equal(before.Cursor, after.Cursor))
		events, watermark, err := first.ReadUnifiedConfigEvents(ctx, nil, 10)
		require.NoError(t, err)
		require.Empty(t, events)
		require.True(t, proto.Equal(before.Cursor, watermark))
	})

	t.Run("ConcurrentCASAndPendingSnapshot", func(t *testing.T) {
		// 第一笔更新持有 clock 锁，第二个独立连接真实进入 LOCK WAIT 后才释放。
		// 读取应仍看到事务提交前的状态+游标；两个 expected=0 的条件写只有一个成功。
		firstEntered := make(chan struct{})
		releaseFirst := make(chan struct{})
		defer close(releaseFirst)
		results := make(chan error, 2)
		conflict := errors.New("release set revision conflict")
		go func() {
			_, err := first.UpdateUnifiedConfig(ctx, key, func(state *store.UnifiedConfigAggregate) (*store.UnifiedConfigChange, error) {
				close(firstEntered)
				<-releaseFirst
				if state.ReleaseSetRevision != 0 {
					return nil, conflict
				}
				state.ReleaseSetRevision = 1
				return &store.UnifiedConfigChange{Runtime: true, Kind: "publish", Actor: "first"}, nil
			})
			results <- err
		}()
		select {
		case <-firstEntered:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		var secondConnection uint64
		require.NoError(t, second.master.DB.QueryRowContext(ctx, "SELECT CONNECTION_ID()").Scan(&secondConnection))
		go func() {
			_, err := second.UpdateUnifiedConfig(ctx, key, func(state *store.UnifiedConfigAggregate) (*store.UnifiedConfigChange, error) {
				if state.ReleaseSetRevision != 0 {
					return nil, conflict
				}
				state.ReleaseSetRevision = 1
				return &store.UnifiedConfigChange{Runtime: true, Kind: "publish", Actor: "second"}, nil
			})
			results <- err
		}()
		require.NoError(t, waitUnifiedConfigLock(ctx, observer.master.DB, secondConnection))
		pending, err := observer.ReadUnifiedConfig(ctx, key)
		require.NoError(t, err)
		require.Equal(t, uint64(0), pending.State.ReleaseSetRevision)
		require.Equal(t, uint64(0), pending.Cursor.Position)
		// 发送一个值释放第一笔事务；defer close 保证测试失败时也能释放它。
		releaseFirst <- struct{}{}
		var success, conflicts int
		for range 2 {
			select {
			case err := <-results:
				if err == nil {
					success++
				} else if errors.Is(err, conflict) {
					conflicts++
				} else {
					t.Fatal(err)
				}
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
		}
		require.Equal(t, 1, success)
		require.Equal(t, 1, conflicts)
		committed, err := observer.ReadUnifiedConfig(ctx, key)
		require.NoError(t, err)
		require.Equal(t, uint64(1), committed.State.ReleaseSetRevision)
		require.Equal(t, uint64(1), committed.Cursor.Position)
		events, watermark, err := observer.ReadUnifiedConfigEvents(ctx, nil, 10)
		require.NoError(t, err)
		require.Len(t, events, 1)
		require.Equal(t, "first", events[0].Actor)
		require.True(t, proto.Equal(committed.Cursor, events[0].Cursor))
		require.True(t, proto.Equal(committed.Cursor, watermark))
	})

	t.Run("SnapshotStateAndCursorStayConsistentDuringCommits", func(t *testing.T) {
		writerDone := make(chan error, 1)
		go func() {
			for range 24 {
				_, err := first.UpdateUnifiedConfig(ctx, key, func(state *store.UnifiedConfigAggregate) (*store.UnifiedConfigChange, error) {
					state.ReleaseSetRevision++
					return &store.UnifiedConfigChange{Runtime: true, Kind: "publish", Actor: "writer"}, nil
				})
				if err != nil {
					writerDone <- err
					return
				}
			}
			writerDone <- nil
		}()
		for range 48 {
			snapshot, err := second.ReadUnifiedConfig(ctx, key)
			require.NoError(t, err)
			require.Equal(t, snapshot.State.ReleaseSetRevision, snapshot.Cursor.Position)
		}
		select {
		case err := <-writerDone:
			require.NoError(t, err)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	})

	t.Run("FreshConnectionsReplayPersistedCursorAndHistory", func(t *testing.T) {
		// 关闭原 store 的所有连接，重新建 store 与 schema；不能依赖旧实例内存计数器。
		require.NoError(t, first.master.DB.Close())
		require.NoError(t, second.master.DB.Close())
		rebuilt := newStore()
		require.NoError(t, ensureUnifiedConfigSchema(rebuilt.master))
		snapshot, err := rebuilt.ReadUnifiedConfig(ctx, key)
		require.NoError(t, err)
		require.Equal(t, uint64(25), snapshot.Cursor.Position)
		require.Equal(t, initial.Cursor.Epoch, snapshot.Cursor.Epoch)
		require.Equal(t, uint64(25), snapshot.State.ReleaseSetRevision)
		events, watermark, err := rebuilt.ReadUnifiedConfigEvents(ctx, initial.Cursor, 100)
		require.NoError(t, err)
		require.Len(t, events, 25)
		for index, event := range events {
			require.Equal(t, uint64(index+1), event.Cursor.Position)
			require.Equal(t, initial.Cursor.Epoch, event.Cursor.Epoch)
		}
		require.True(t, proto.Equal(snapshot.Cursor, watermark))
		// 返回对象是独立副本，修改不会影响之后从数据库取得的状态。
		snapshot.State.Draft.Key.Name = "changed"
		again, err := rebuilt.ReadUnifiedConfig(ctx, key)
		require.NoError(t, err)
		require.Equal(t, key.Name, again.State.Draft.Key.Name)
		rotated, err := rebuilt.RotateUnifiedConfigEpoch(ctx)
		require.NoError(t, err)
		require.NotEqual(t, initial.Cursor.Epoch, rotated.Epoch)
		require.Equal(t, uint64(0), rotated.Position)
		_, _, err = rebuilt.ReadUnifiedConfigEvents(ctx, initial.Cursor, 10)
		require.ErrorIs(t, err, store.ErrUnifiedConfigCursorInvalid)
		preserved, err := rebuilt.ReadUnifiedConfig(ctx, key)
		require.NoError(t, err)
		requireUnifiedAggregateEqual(t, again.State, preserved.State)
		require.True(t, proto.Equal(rotated, preserved.Cursor))
		events, watermark, err = rebuilt.ReadUnifiedConfigEvents(ctx, nil, 10)
		require.NoError(t, err)
		require.Empty(t, events)
		require.True(t, proto.Equal(rotated, watermark))
	})

	t.Run("LargeHistoryUsesIndependentReleasesWithoutOrphans", func(t *testing.T) {
		storage := newStore()
		historyKey := &configv1.ConfigKey{Environment: "dev", Group: "history", Name: "large.bin"}
		content := bytes.Repeat([]byte("x"), 512*1024)
		for index := range 34 {
			_, err := storage.UpdateUnifiedConfig(ctx, historyKey, func(state *store.UnifiedConfigAggregate) (*store.UnifiedConfigChange, error) {
				for _, member := range state.Members {
					member.Active = false
				}
				state.Members = append(state.Members, &store.UnifiedConfigMember{Active: true, Release: &configv1.ConfigRelease{
					Id: uuid.NewString(), Key: proto.Clone(historyKey).(*configv1.ConfigKey), Sequence: uint64(index + 1),
					Kind:       configv1.ReleaseKind_RELEASE_KIND_NORMAL,
					Definition: &configv1.ConfigDefinition{Engine: "literal", Content: content},
				}})
				state.ReleaseSetRevision++
				return &store.UnifiedConfigChange{Runtime: true, Kind: "publish"}, nil
			})
			require.NoError(t, err)
		}
		full, err := storage.ReadUnifiedConfig(ctx, historyKey)
		require.NoError(t, err)
		require.Len(t, full.State.Members, 34)
		fullData, err := json.Marshal(full.State)
		require.NoError(t, err)
		require.Greater(t, len(fullData), 16*1024*1024)
		runtime, err := storage.ReadUnifiedConfigRuntime(ctx, historyKey)
		require.NoError(t, err)
		require.Nil(t, runtime.State.Draft)
		require.Len(t, runtime.State.Members, 1)
		require.Equal(t, content, runtime.State.Members[0].Release.Definition.Content)
		require.True(t, proto.Equal(full.Cursor, runtime.Cursor))
		_, keyHash, err := unifiedConfigIdentity(historyKey)
		require.NoError(t, err)
		var headerData []byte
		require.NoError(t, storage.master.DB.QueryRowContext(ctx, "SELECT state FROM unified_config_state WHERE key_hash = ?", keyHash).Scan(&headerData))
		require.Less(t, len(headerData), 4096)
		header := &store.UnifiedConfigAggregate{}
		require.NoError(t, json.Unmarshal(headerData, header))
		for _, member := range header.Members {
			require.Nil(t, member.Release.Definition)
			require.NotEmpty(t, member.Release.Id)
		}
		// 已有历史正文不能覆写；失败不能推进 clock 或改写引用。
		_, err = storage.UpdateUnifiedConfig(ctx, historyKey, func(state *store.UnifiedConfigAggregate) (*store.UnifiedConfigChange, error) {
			state.Members[0].Release.Definition.Content = []byte("tampered")
			return &store.UnifiedConfigChange{Runtime: true}, nil
		})
		require.ErrorContains(t, err, "snapshot is immutable")
		var releasesBefore int
		require.NoError(t, storage.master.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM unified_config_release WHERE key_hash = ?", keyHash).Scan(&releasesBefore))
		require.Equal(t, 34, releasesBefore)
		_, err = storage.master.DB.ExecContext(ctx, `CREATE TRIGGER unified_config_test_abort_new_release
			BEFORE INSERT ON unified_config_event FOR EACH ROW
			SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'injected orphan check failure'`)
		require.NoError(t, err)
		defer func() {
			_, err := storage.master.DB.ExecContext(ctx, `DROP TRIGGER unified_config_test_abort_new_release`)
			require.NoError(t, err)
		}()
		_, err = storage.UpdateUnifiedConfig(ctx, historyKey, func(state *store.UnifiedConfigAggregate) (*store.UnifiedConfigChange, error) {
			state.Members = append(state.Members, &store.UnifiedConfigMember{Active: true, Release: &configv1.ConfigRelease{
				Id: uuid.NewString(), Key: proto.Clone(historyKey).(*configv1.ConfigKey), Sequence: 35,
				Definition: &configv1.ConfigDefinition{Engine: "literal", Content: content},
			}})
			return &store.UnifiedConfigChange{Runtime: true}, nil
		})
		require.ErrorContains(t, err, "injected orphan check failure")
		var releasesAfter int
		require.NoError(t, storage.master.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM unified_config_release WHERE key_hash = ?", keyHash).Scan(&releasesAfter))
		require.Equal(t, releasesBefore, releasesAfter)
		after, err := storage.ReadUnifiedConfig(ctx, historyKey)
		require.NoError(t, err)
		requireUnifiedAggregateEqual(t, full.State, after.State)
		require.True(t, proto.Equal(full.Cursor, after.Cursor))
	})
}

func waitUnifiedConfigLock(ctx context.Context, db *sql.DB, connection uint64) error {
	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var waiting int
		if err := db.QueryRowContext(waitCtx, `SELECT COUNT(*) FROM performance_schema.data_lock_waits w
			JOIN performance_schema.threads th ON th.THREAD_ID = w.REQUESTING_THREAD_ID
			WHERE th.PROCESSLIST_ID = ?`, connection).Scan(&waiting); err != nil {
			return err
		}
		if waiting > 0 {
			return nil
		}
		select {
		case <-ticker.C:
		case <-waitCtx.Done():
			return fmt.Errorf("second connection did not enter lock wait: %w", waitCtx.Err())
		}
	}
}

// 聚合是普通 Go 对象，其嵌套 protobuf 按语义比较，忽略懒初始化缓存。
func requireUnifiedAggregateEqual(t *testing.T, expected, actual *store.UnifiedConfigAggregate) {
	t.Helper()
	require.True(t, proto.Equal(expected.Key, actual.Key), "aggregate key changed")
	require.True(t, proto.Equal(expected.Draft, actual.Draft), "aggregate draft changed")
	require.Equal(t, expected.ReleaseSetRevision, actual.ReleaseSetRevision)
	require.Equal(t, expected.Incarnation, actual.Incarnation)
	require.Equal(t, expected.Offline, actual.Offline)
	require.Len(t, actual.Members, len(expected.Members))
	for i, member := range expected.Members {
		require.Equal(t, member.Active, actual.Members[i].Active)
		require.True(t, proto.Equal(member.Release, actual.Members[i].Release), "release changed")
	}
}

// 模拟删除前已经通过缓存许可的身份；存储仍必须在当前事务内复核 namespace。
type staleNamespaceAuthorization struct{}

func (staleNamespaceAuthorization) Authorize(context.Context, *configv1.ConfigKey, unified.Action) (string, error) {
	return "verified-cached-user", nil
}

type heldUnifiedMutation struct {
	*stableStore
	acquired chan struct{}
	release  chan struct{}
}

func (s *heldUnifiedMutation) UpdateUnifiedConfig(ctx context.Context, key *configv1.ConfigKey, mutate store.UnifiedConfigMutation) (*store.UnifiedConfigSnapshot, error) {
	return s.stableStore.UpdateUnifiedConfig(ctx, key, func(state *store.UnifiedConfigAggregate) (*store.UnifiedConfigChange, error) {
		close(s.acquired)
		select {
		case <-s.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return mutate(state)
	})
}
