package sqldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	configv1 "github.com/pole-io/specification/source/go/api/v1/config"

	"github.com/pole-io/pole-server/apis/store"
)

const testUnifiedEpoch = "f38b3cae-fc65-4b8a-9b48-896ec959de20"

func newUnifiedSQLMock(t *testing.T) (*stableStore, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, mock.ExpectationsWereMet())
		db.Close()
	})
	// slave 故意不配置；所有读取必须使用 master。
	return &stableStore{master: &BaseDB{DB: db}}, mock
}

func testUnifiedKey() *configv1.ConfigKey {
	return &configv1.ConfigKey{Environment: "dev", Group: "service", Name: "app.yaml"}
}

func expectUnifiedSnapshot(t *testing.T, mock sqlmock.Sqlmock, state *store.UnifiedConfigAggregate, position uint64, lock bool) {
	expectUnifiedHeader(t, mock, state, position, lock)
	if len(state.GetMembers()) > 0 {
		rows := sqlmock.NewRows([]string{"release_id", "release_data"})
		for _, member := range state.Members {
			data, err := proto.Marshal(member.Release)
			require.NoError(t, err)
			rows.AddRow(member.Release.Id, data)
		}
		mock.ExpectQuery(regexp.QuoteMeta("SELECT release_id, release_data FROM unified_config_release WHERE key_hash = ?")).
			WithArgs(sqlmock.AnyArg()).WillReturnRows(rows)
	}
}

func expectUnifiedHeader(t *testing.T, mock sqlmock.Sqlmock, state *store.UnifiedConfigAggregate, position uint64, lock bool) {
	t.Helper()
	mock.ExpectBegin()
	clockQuery := "SELECT epoch, position FROM unified_config_clock WHERE scope = ?"
	stateQuery := "SELECT state FROM unified_config_state WHERE key_hash = ?"
	if lock {
		mock.ExpectQuery(regexp.QuoteMeta("SELECT kind FROM namespace WHERE name = ? AND flag != 1 LOCK IN SHARE MODE")).WithArgs(testUnifiedKey().Environment).WillReturnRows(sqlmock.NewRows([]string{"kind"}).AddRow(0))
		clockQuery += " FOR UPDATE"
		stateQuery += " FOR UPDATE"
	}
	// 有序期望固定 namespace → clock → state 锁序。
	mock.ExpectQuery(regexp.QuoteMeta(clockQuery)).WithArgs(unifiedConfigScope).
		WillReturnRows(sqlmock.NewRows([]string{"epoch", "position"}).AddRow(testUnifiedEpoch, position))
	query := mock.ExpectQuery(regexp.QuoteMeta(stateQuery)).WithArgs(sqlmock.AnyArg())
	if state == nil {
		query.WillReturnError(sql.ErrNoRows)
	} else {
		header := state.Clone()
		for _, member := range header.Members {
			member.Release = &configv1.ConfigRelease{Id: member.Release.Id}
		}
		payload, err := json.Marshal(header)
		require.NoError(t, err)
		query.WillReturnRows(sqlmock.NewRows([]string{"state"}).AddRow(payload))
	}
}

func expectUnifiedStateWrite(mock sqlmock.Sqlmock) {
	key := testUnifiedKey()
	mock.ExpectExec("INSERT INTO unified_config_state").
		WithArgs(sqlmock.AnyArg(), key.Environment, key.Group, key.Name, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
}

func TestUnifiedConfigReadMissingHasCursorAndOwnedKey(t *testing.T) {
	s, mock := newUnifiedSQLMock(t)
	key := testUnifiedKey()
	expectUnifiedSnapshot(t, mock, nil, 7, false)
	mock.ExpectCommit()
	snapshot, err := s.ReadUnifiedConfig(context.Background(), key)
	require.NoError(t, err)
	require.Equal(t, uint64(7), snapshot.Cursor.Position)
	require.Equal(t, testUnifiedEpoch, snapshot.Cursor.Epoch)
	require.True(t, proto.Equal(key, snapshot.State.Key))
	snapshot.State.Key.Name = "changed"
	require.Equal(t, "app.yaml", key.Name)
}

func TestUnifiedConfigReadReturnsIndependentNestedState(t *testing.T) {
	s, mock := newUnifiedSQLMock(t)
	state := &store.UnifiedConfigAggregate{Key: testUnifiedKey(), Draft: &configv1.ConfigDraft{Key: testUnifiedKey(), Revision: 4}}
	for range 2 {
		expectUnifiedSnapshot(t, mock, state, 5, false)
		mock.ExpectCommit()
	}
	first, err := s.ReadUnifiedConfig(context.Background(), testUnifiedKey())
	require.NoError(t, err)
	first.State.Draft.Key.Name = "changed"
	first.Cursor.Position = 999
	second, err := s.ReadUnifiedConfig(context.Background(), testUnifiedKey())
	require.NoError(t, err)
	require.Equal(t, "app.yaml", second.State.Draft.Key.Name)
	require.Equal(t, uint64(5), second.Cursor.Position)
}

func TestUnifiedConfigUpdateDraftDoesNotAdvanceCursor(t *testing.T) {
	s, mock := newUnifiedSQLMock(t)
	expectUnifiedSnapshot(t, mock, nil, 8, true)
	expectUnifiedStateWrite(mock)
	mock.ExpectCommit()
	var callbackState *store.UnifiedConfigAggregate
	snapshot, err := s.UpdateUnifiedConfig(context.Background(), testUnifiedKey(), func(state *store.UnifiedConfigAggregate) (*store.UnifiedConfigChange, error) {
		callbackState = state
		state.Draft = &configv1.ConfigDraft{Key: testUnifiedKey(), Revision: 1}
		return &store.UnifiedConfigChange{Kind: "draft"}, nil
	})
	require.NoError(t, err)
	require.Equal(t, uint64(8), snapshot.Cursor.Position)
	require.Equal(t, uint64(1), snapshot.State.Draft.Revision)
	callbackState.Draft.Key.Name = "retained-reference"
	require.Equal(t, "app.yaml", snapshot.State.Draft.Key.Name)
}

func TestUnifiedConfigNoopDiscardsCallbackChanges(t *testing.T) {
	s, mock := newUnifiedSQLMock(t)
	expectUnifiedSnapshot(t, mock, nil, 8, true)
	mock.ExpectCommit()
	snapshot, err := s.UpdateUnifiedConfig(context.Background(), testUnifiedKey(), func(state *store.UnifiedConfigAggregate) (*store.UnifiedConfigChange, error) {
		state.Offline = true
		return nil, nil
	})
	require.NoError(t, err)
	require.False(t, snapshot.State.Offline)
	require.Equal(t, uint64(8), snapshot.Cursor.Position)
}

func TestUnifiedConfigCallbackFailureRollsBack(t *testing.T) {
	s, mock := newUnifiedSQLMock(t)
	expectUnifiedSnapshot(t, mock, nil, 8, true)
	mock.ExpectRollback()
	failure := errors.New("domain rejected operation")
	snapshot, err := s.UpdateUnifiedConfig(context.Background(), testUnifiedKey(), func(state *store.UnifiedConfigAggregate) (*store.UnifiedConfigChange, error) {
		state.Offline = true
		return nil, failure
	})
	require.ErrorIs(t, err, failure)
	require.Nil(t, snapshot)
}

func TestUnifiedConfigRuntimeCommitsStateEventAndClockTogether(t *testing.T) {
	s, mock := newUnifiedSQLMock(t)
	expectUnifiedSnapshot(t, mock, nil, 8, true)
	expectUnifiedStateWrite(mock)
	key := testUnifiedKey()
	mock.ExpectExec("INSERT INTO unified_config_event").
		WithArgs(unifiedConfigScope, testUnifiedEpoch, uint64(9), key.Environment, key.Group, key.Name, "offline", "tester").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE unified_config_clock SET position").WithArgs(uint64(9), unifiedConfigScope).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	snapshot, err := s.UpdateUnifiedConfig(context.Background(), key, func(state *store.UnifiedConfigAggregate) (*store.UnifiedConfigChange, error) {
		state.Offline = true
		return &store.UnifiedConfigChange{Runtime: true, Kind: "offline", Actor: "tester"}, nil
	})
	require.NoError(t, err)
	require.True(t, snapshot.State.Offline)
	require.Equal(t, uint64(9), snapshot.Cursor.Position)
}

func TestUnifiedConfigRuntimeSQLFailuresRollback(t *testing.T) {
	for _, stage := range []string{"state", "event", "clock", "commit"} {
		t.Run(stage, func(t *testing.T) {
			s, mock := newUnifiedSQLMock(t)
			expectUnifiedSnapshot(t, mock, nil, 8, true)
			failure := errors.New("injected " + stage + " failure")
			if stage == "state" {
				mock.ExpectExec("INSERT INTO unified_config_state").WillReturnError(failure)
			} else {
				expectUnifiedStateWrite(mock)
				if stage == "event" {
					mock.ExpectExec("INSERT INTO unified_config_event").WillReturnError(failure)
				} else {
					mock.ExpectExec("INSERT INTO unified_config_event").WillReturnResult(sqlmock.NewResult(0, 1))
					if stage == "clock" {
						mock.ExpectExec("UPDATE unified_config_clock").WillReturnError(failure)
					} else {
						mock.ExpectExec("UPDATE unified_config_clock").WillReturnResult(sqlmock.NewResult(0, 1))
					}
				}
			}
			if stage == "commit" {
				mock.ExpectCommit().WillReturnError(failure)
			} else {
				mock.ExpectRollback()
			}
			snapshot, err := s.UpdateUnifiedConfig(context.Background(), testUnifiedKey(), func(state *store.UnifiedConfigAggregate) (*store.UnifiedConfigChange, error) {
				state.Offline = true
				return &store.UnifiedConfigChange{Runtime: true}, nil
			})
			require.ErrorIs(t, err, failure)
			require.Nil(t, snapshot)
		})
	}
}

func TestUnifiedConfigRejectsChangedKey(t *testing.T) {
	s, mock := newUnifiedSQLMock(t)
	expectUnifiedSnapshot(t, mock, nil, 0, true)
	mock.ExpectRollback()
	_, err := s.UpdateUnifiedConfig(context.Background(), testUnifiedKey(), func(state *store.UnifiedConfigAggregate) (*store.UnifiedConfigChange, error) {
		state.Key.Name = "other"
		return &store.UnifiedConfigChange{}, nil
	})
	require.ErrorIs(t, err, store.ErrUnifiedConfigKeyInvalid)
}

func TestUnifiedConfigIdentityIsUnambiguousAndRejectsMissingFields(t *testing.T) {
	_, first, err := unifiedConfigIdentity(&configv1.ConfigKey{Environment: "a/b", Group: "c", Name: "d"})
	require.NoError(t, err)
	_, second, err := unifiedConfigIdentity(&configv1.ConfigKey{Environment: "a", Group: "b/c", Name: "d"})
	require.NoError(t, err)
	require.NotEqual(t, first, second)
	for _, key := range []*configv1.ConfigKey{nil, {}, {Environment: "dev"}, {Environment: "dev", Group: "service"}, {Environment: "dev", Group: "service", Name: string([]byte{0xff})}} {
		_, _, err := unifiedConfigIdentity(key)
		require.ErrorIs(t, err, store.ErrUnifiedConfigKeyInvalid)
	}
}

func TestUnifiedConfigPersistedClockIsSharedAcrossStoreInstances(t *testing.T) {
	s, mock := newUnifiedSQLMock(t)
	otherInstance := &stableStore{master: s.master}
	// 第二个 store 没有任何内存游标，仍从数据库读到第一笔提交后的 position。
	for _, position := range []uint64{8, 9} {
		expectUnifiedSnapshot(t, mock, nil, position, true)
		expectUnifiedStateWrite(mock)
		mock.ExpectExec("INSERT INTO unified_config_event").
			WithArgs(unifiedConfigScope, testUnifiedEpoch, position+1, "dev", "service", "app.yaml", "offline", "").
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec("UPDATE unified_config_clock").WithArgs(position+1, unifiedConfigScope).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()
	}
	mutate := func(state *store.UnifiedConfigAggregate) (*store.UnifiedConfigChange, error) {
		state.Offline = true
		return &store.UnifiedConfigChange{Runtime: true, Kind: "offline"}, nil
	}
	first, err := s.UpdateUnifiedConfig(context.Background(), testUnifiedKey(), mutate)
	require.NoError(t, err)
	second, err := otherInstance.UpdateUnifiedConfig(context.Background(), testUnifiedKey(), mutate)
	require.NoError(t, err)
	require.Equal(t, uint64(9), first.Cursor.Position)
	require.Equal(t, uint64(10), second.Cursor.Position)
	require.Equal(t, first.Cursor.Epoch, second.Cursor.Epoch)
}

func TestUnifiedConfigStoredIdentityMismatchFailsClosed(t *testing.T) {
	s, mock := newUnifiedSQLMock(t)
	wrongKey := testUnifiedKey()
	wrongKey.Name = "another.yaml"
	expectUnifiedSnapshot(t, mock, &store.UnifiedConfigAggregate{Key: wrongKey}, 8, false)
	mock.ExpectRollback()
	snapshot, err := s.ReadUnifiedConfig(context.Background(), testUnifiedKey())
	require.ErrorContains(t, err, "stored identity mismatch")
	require.Nil(t, snapshot)
}

func TestUnifiedConfigRotateFailureRollsBackOldLogAndEpoch(t *testing.T) {
	s, mock := newUnifiedSQLMock(t)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT epoch, position FROM unified_config_clock WHERE scope = \\? FOR UPDATE").
		WithArgs(unifiedConfigScope).WillReturnRows(sqlmock.NewRows([]string{"epoch", "position"}).AddRow(testUnifiedEpoch, 8))
	mock.ExpectExec("DELETE FROM unified_config_event").WithArgs(unifiedConfigScope).WillReturnResult(sqlmock.NewResult(0, 8))
	failure := errors.New("epoch update failure")
	mock.ExpectExec("UPDATE unified_config_clock").WillReturnError(failure)
	mock.ExpectRollback()
	cursor, err := s.RotateUnifiedConfigEpoch(context.Background())
	require.ErrorIs(t, err, failure)
	require.Nil(t, cursor)
}

func expectUnifiedEventClock(mock sqlmock.Sqlmock, position uint64) {
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT epoch, position FROM unified_config_clock").WithArgs(unifiedConfigScope).
		WillReturnRows(sqlmock.NewRows([]string{"epoch", "position"}).AddRow(testUnifiedEpoch, position))
}

func TestUnifiedConfigEventsReplayUsesEpochAndSnapshotBound(t *testing.T) {
	s, mock := newUnifiedSQLMock(t)
	expectUnifiedEventClock(mock, 8)
	mock.ExpectQuery("SELECT position, environment, group_name, config_name, kind, actor").
		WithArgs(unifiedConfigScope, testUnifiedEpoch, uint64(4), uint64(8), uint32(2)).
		WillReturnRows(sqlmock.NewRows([]string{"position", "environment", "group_name", "config_name", "kind", "actor"}).
			AddRow(5, "dev", "service", "app.yaml", "publish", "tester").
			AddRow(6, "dev", "service", "app.yaml", "offline", "tester"))
	mock.ExpectCommit()
	events, watermark, err := s.ReadUnifiedConfigEvents(context.Background(), &configv1.Cursor{Scope: unifiedConfigScope, Epoch: testUnifiedEpoch, Position: 4}, 2)
	require.NoError(t, err)
	require.Len(t, events, 2)
	require.Equal(t, uint64(5), events[0].Cursor.Position)
	require.Equal(t, uint64(6), events[1].Cursor.Position)
	require.Equal(t, uint64(8), watermark.Position)
	events[0].Cursor.Epoch = "changed"
	events[0].Key.Name = "changed"
	require.Equal(t, testUnifiedEpoch, events[1].Cursor.Epoch)
	require.Equal(t, testUnifiedEpoch, watermark.Epoch)
	require.Equal(t, "app.yaml", events[1].Key.Name)
}

func TestUnifiedConfigEventsEmptyPageStillReturnsCursor(t *testing.T) {
	s, mock := newUnifiedSQLMock(t)
	expectUnifiedEventClock(mock, 0)
	mock.ExpectQuery("SELECT position, environment").WithArgs(unifiedConfigScope, testUnifiedEpoch, uint64(0), uint64(0), uint32(1000)).
		WillReturnRows(sqlmock.NewRows([]string{"position", "environment", "group_name", "config_name", "kind", "actor"}))
	mock.ExpectCommit()
	events, cursor, err := s.ReadUnifiedConfigEvents(context.Background(), nil, 0)
	require.NoError(t, err)
	require.Empty(t, events)
	require.Equal(t, testUnifiedEpoch, cursor.Epoch)
}

func TestUnifiedConfigEventsRejectsInvalidCursors(t *testing.T) {
	for _, after := range []*configv1.Cursor{
		{Scope: "other", Epoch: testUnifiedEpoch},
		{Scope: unifiedConfigScope, Epoch: "old-epoch"},
		{Scope: unifiedConfigScope, Epoch: testUnifiedEpoch, Position: 9},
	} {
		t.Run(after.String(), func(t *testing.T) {
			s, mock := newUnifiedSQLMock(t)
			expectUnifiedEventClock(mock, 8)
			mock.ExpectRollback()
			events, cursor, err := s.ReadUnifiedConfigEvents(context.Background(), after, 10)
			require.ErrorIs(t, err, store.ErrUnifiedConfigCursorInvalid)
			require.Nil(t, events)
			require.Equal(t, uint64(8), cursor.Position)
		})
	}
}

func TestUnifiedConfigRotateEpochIsAtomicAndRandom(t *testing.T) {
	s, mock := newUnifiedSQLMock(t)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT epoch, position FROM unified_config_clock WHERE scope = \\? FOR UPDATE").
		WithArgs(unifiedConfigScope).WillReturnRows(sqlmock.NewRows([]string{"epoch", "position"}).AddRow(testUnifiedEpoch, 8))
	mock.ExpectExec("DELETE FROM unified_config_event").WithArgs(unifiedConfigScope).WillReturnResult(sqlmock.NewResult(0, 8))
	mock.ExpectExec("UPDATE unified_config_clock SET epoch = \\?, position = 0 WHERE scope = \\?").
		WithArgs(sqlmock.AnyArg(), unifiedConfigScope).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	cursor, err := s.RotateUnifiedConfigEpoch(context.Background())
	require.NoError(t, err)
	require.Equal(t, uint64(0), cursor.Position)
	require.NotEqual(t, testUnifiedEpoch, cursor.Epoch)
	_, err = uuid.Parse(cursor.Epoch)
	require.NoError(t, err)
}

func TestUnifiedConfigSchemaIsAdditiveAndPreservesEpoch(t *testing.T) {
	s, mock := newUnifiedSQLMock(t)
	for _, table := range []string{"unified_config_file_template", "unified_config_file_environment", "unified_config_state", "unified_config_clock", "unified_config_event", "unified_config_release"} {
		mock.ExpectExec("CREATE TABLE IF NOT EXISTS " + table).WillReturnResult(sqlmock.NewResult(0, 0))
	}
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO unified_config_clock (scope, epoch, position) VALUES (?, ?, 0)
		ON DUPLICATE KEY UPDATE scope = VALUES(scope)`)).WithArgs(unifiedConfigScope, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))
	require.NoError(t, ensureUnifiedConfigSchema(s.master))
}

func testUnifiedRelease(id string) *configv1.ConfigRelease {
	return &configv1.ConfigRelease{Id: id, Key: testUnifiedKey(), Sequence: 1,
		Definition: &configv1.ConfigDefinition{Engine: "literal", Content: []byte("private definition")}}
}

func TestUnifiedConfigRuntimeLoadsOnlyActiveRelease(t *testing.T) {
	s, mock := newUnifiedSQLMock(t)
	state := &store.UnifiedConfigAggregate{Key: testUnifiedKey(), Draft: &configv1.ConfigDraft{Revision: 7}, Members: []*store.UnifiedConfigMember{
		{Release: testUnifiedRelease("past")}, {Release: testUnifiedRelease("active"), Active: true},
	}}
	expectUnifiedHeader(t, mock, state, 12, false)
	data, err := proto.Marshal(state.Members[1].Release)
	require.NoError(t, err)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT release_id, release_data FROM unified_config_release WHERE key_hash = ? AND release_id IN (?)")).
		WithArgs(sqlmock.AnyArg(), "active").WillReturnRows(sqlmock.NewRows([]string{"release_id", "release_data"}).AddRow("active", data))
	mock.ExpectCommit()
	snapshot, err := s.ReadUnifiedConfigRuntime(context.Background(), testUnifiedKey())
	require.NoError(t, err)
	require.Nil(t, snapshot.State.Draft)
	require.Len(t, snapshot.State.Members, 1)
	require.True(t, proto.Equal(state.Members[1].Release, snapshot.State.Members[0].Release))
	require.Equal(t, uint64(12), snapshot.Cursor.Position)
}

func TestUnifiedConfigReleaseMissingFailsClosed(t *testing.T) {
	s, mock := newUnifiedSQLMock(t)
	expectUnifiedHeader(t, mock, &store.UnifiedConfigAggregate{Key: testUnifiedKey(), Members: []*store.UnifiedConfigMember{{Release: testUnifiedRelease("missing"), Active: true}}}, 12, false)
	mock.ExpectQuery("SELECT release_id, release_data FROM unified_config_release").
		WillReturnRows(sqlmock.NewRows([]string{"release_id", "release_data"}))
	mock.ExpectRollback()
	snapshot, err := s.ReadUnifiedConfigRuntime(context.Background(), testUnifiedKey())
	require.ErrorContains(t, err, "release reference is missing")
	require.Nil(t, snapshot)
}

func TestUnifiedConfigExistingReleaseCannotBeChangedOrRemoved(t *testing.T) {
	for _, remove := range []bool{false, true} {
		t.Run(map[bool]string{false: "change", true: "remove"}[remove], func(t *testing.T) {
			s, mock := newUnifiedSQLMock(t)
			expectUnifiedSnapshot(t, mock, &store.UnifiedConfigAggregate{Key: testUnifiedKey(), Members: []*store.UnifiedConfigMember{{Release: testUnifiedRelease("immutable"), Active: true}}}, 12, true)
			mock.ExpectRollback()
			_, err := s.UpdateUnifiedConfig(context.Background(), testUnifiedKey(), func(state *store.UnifiedConfigAggregate) (*store.UnifiedConfigChange, error) {
				if remove {
					state.Members = nil
				} else {
					state.Members[0].Release.Definition.Content = []byte("changed")
				}
				return &store.UnifiedConfigChange{Runtime: true}, nil
			})
			if remove {
				require.ErrorContains(t, err, "history cannot be removed")
			} else {
				require.ErrorContains(t, err, "snapshot is immutable")
			}
		})
	}
}

func TestUnifiedConfigReleaseInsertRollsBackWithOutboxFailure(t *testing.T) {
	s, mock := newUnifiedSQLMock(t)
	expectUnifiedSnapshot(t, mock, nil, 0, true)
	mock.ExpectExec("INSERT INTO unified_config_release").WithArgs(sqlmock.AnyArg(), "new-release", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	expectUnifiedStateWrite(mock)
	failure := errors.New("outbox failed")
	mock.ExpectExec("INSERT INTO unified_config_event").WillReturnError(failure)
	mock.ExpectRollback()
	_, err := s.UpdateUnifiedConfig(context.Background(), testUnifiedKey(), func(state *store.UnifiedConfigAggregate) (*store.UnifiedConfigChange, error) {
		state.Members = []*store.UnifiedConfigMember{{Release: testUnifiedRelease("new-release"), Active: true}}
		return &store.UnifiedConfigChange{Runtime: true}, nil
	})
	require.ErrorIs(t, err, failure)
}

func TestUnifiedConfigWriteRechecksNamespaceBeforeClockAndMutation(t *testing.T) {
	for _, test := range []struct {
		name        string
		environment string
		kind        int
		failure     error
	}{{"missing", "dev", 0, sql.ErrNoRows}, {"system kind", "dev", 1, nil}, {"unknown kind", "dev", 99, nil}, {"reserved", "pole-system", 0, nil}, {"database failure", "dev", 0, errors.New("namespace query unavailable")}} {
		t.Run(test.name, func(t *testing.T) {
			s, mock := newUnifiedSQLMock(t)
			mock.ExpectBegin()
			if test.environment != SystemNamespace {
				query := mock.ExpectQuery(regexp.QuoteMeta("SELECT kind FROM namespace WHERE name = ? AND flag != 1 LOCK IN SHARE MODE")).WithArgs(test.environment)
				if test.failure != nil {
					query.WillReturnError(test.failure)
				} else {
					query.WillReturnRows(sqlmock.NewRows([]string{"kind"}).AddRow(test.kind))
				}
			}
			mock.ExpectRollback()
			key := testUnifiedKey()
			key.Environment = test.environment
			result, err := s.UpdateUnifiedConfig(context.Background(), key, func(*store.UnifiedConfigAggregate) (*store.UnifiedConfigChange, error) {
				t.Fatal("invalid namespace reached mutation")
				return nil, nil
			})
			require.Error(t, err)
			require.Nil(t, result)
			if test.name == "database failure" {
				require.ErrorIs(t, err, test.failure)
			} else {
				require.Equal(t, store.NotFoundNamespace, store.Code(err))
			}
		})
	}
}
