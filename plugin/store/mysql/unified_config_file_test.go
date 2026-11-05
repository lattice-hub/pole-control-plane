package sqldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/pole-io/pole-server/apis/store"
	configv1 "github.com/pole-io/specification/source/go/api/v1/config"
	"github.com/stretchr/testify/require"
)

func TestConfigFileCASRollsBackOnConflict(t *testing.T) {
	s, m := newUnifiedSQLMock(t)
	key := &configv1.ConfigFileKey{Group: "g", Name: "n"}
	v := &configv1.ConfigFileTemplate{Key: key, Environments: []string{"a"}}
	m.ExpectBegin()
	m.ExpectQuery("SELECT kind FROM namespace").WithArgs("a").WillReturnRows(sqlmock.NewRows([]string{"kind"}).AddRow(0))
	m.ExpectExec("INSERT INTO unified_config_file_template").WithArgs(sqlmock.AnyArg(), "g", "n", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 0))
	m.ExpectQuery("SELECT revision,group_name,config_name").WithArgs(sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"revision", "group_name", "config_name"}).AddRow(2, "g", "n"))
	m.ExpectRollback()
	_, err := s.SaveConfigFileTemplate(context.Background(), v, 1)
	require.ErrorIs(t, err, store.ErrConfigFileConflict)
}
func TestConfigFilePageTokenIsScoped(t *testing.T) {
	s, _ := newUnifiedSQLMock(t)
	_, err := s.ListConfigFiles(context.Background(), &configv1.ListConfigFilesRequest{Environment: "prod", PageToken: "eyJFbnZpcm9ubWVudCI6ImRldiJ9"})
	require.ErrorIs(t, err, store.ErrUnifiedConfigCursorInvalid)
}

func TestConfigFileSaveRejectsDeletedEnvironment(t *testing.T) {
	s, m := newUnifiedSQLMock(t)
	m.ExpectBegin()
	m.ExpectQuery("SELECT kind FROM namespace").WithArgs("gone").WillReturnError(sql.ErrNoRows)
	m.ExpectRollback()
	_, err := s.SaveConfigFileTemplate(context.Background(), &configv1.ConfigFileTemplate{Key: &configv1.ConfigFileKey{Group: "g", Name: "n"}, Environments: []string{"gone"}}, 0)
	require.Error(t, err)
}

func TestConfigFileInventoryIncludesTemplateOnly(t *testing.T) {
	s, m := newUnifiedSQLMock(t)
	m.ExpectQuery("SELECT COUNT\\(\\*\\) FROM .*unified_config_state.*UNION.*unified_config_file_environment").WithArgs("stage", "stage").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	count, err := s.CountUnifiedConfigs(context.Background(), "stage")
	require.NoError(t, err)
	require.Equal(t, uint64(1), count)
}

func TestConfigFileCreateRejectsDifferentConcurrentDraft(t *testing.T) {
	s, m := newUnifiedSQLMock(t)
	key := &configv1.ConfigFileKey{Group: "g", Name: "n"}
	state := &store.UnifiedConfigAggregate{Key: &configv1.ConfigKey{Environment: "a", Group: "g", Name: "n"}, Draft: &configv1.ConfigDraft{Definition: &configv1.ConfigDefinition{Engine: "literal", Content: []byte("previous")}}}
	payload, err := json.Marshal(state)
	require.NoError(t, err)
	m.ExpectBegin()
	m.ExpectQuery("SELECT kind FROM namespace").WithArgs("a").WillReturnRows(sqlmock.NewRows([]string{"kind"}).AddRow(0))
	m.ExpectQuery("SELECT epoch, position FROM unified_config_clock").WithArgs(unifiedConfigScope).WillReturnRows(sqlmock.NewRows([]string{"epoch", "position"}).AddRow(testUnifiedEpoch, 0))
	m.ExpectExec("INSERT INTO unified_config_file_template").WithArgs(sqlmock.AnyArg(), "g", "n", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery("SELECT revision,group_name,config_name").WithArgs(sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"revision", "group_name", "config_name"}).AddRow(0, "g", "n"))
	m.ExpectQuery("SELECT state FROM unified_config_state WHERE key_hash = \\? LOCK IN SHARE MODE").WithArgs(sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"state"}).AddRow(payload))
	m.ExpectRollback()
	_, err = s.SaveConfigFileTemplate(context.Background(), &configv1.ConfigFileTemplate{Key: key, Environments: []string{"a"}, Definition: &configv1.ConfigDefinition{Engine: "literal", Content: []byte("new")}}, 0)
	require.ErrorIs(t, err, store.ErrConfigFileConflict)
}
