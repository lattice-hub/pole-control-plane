package sqldb

import (
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/pole-io/pole-server/apis/pkg/types"
	"github.com/pole-io/pole-server/apis/store"
)

func TestGetEnvironmentPromotionTopologyReadsUnixModifyTime(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer rawDB.Close()
	repository := &environmentPromotionStore{master: &BaseDB{DB: rawDB}}
	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT draft_revision, published_revision, draft_json, modify_by, UNIX_TIMESTAMP(mtime)
		FROM environment_promotion_topology WHERE id = 1`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"draft_revision", "published_revision", "draft_json", "modify_by", "mtime",
		}).AddRow(1, 1, `{"draft_revision":1,"published_revision":1}`, "admin", int64(123)))

	topology, err := repository.GetEnvironmentPromotionTopology()

	require.NoError(t, err)
	require.Equal(t, time.Unix(123, 0), topology.ModifyTime)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListEnvironmentPromotionTopologyRevisionsReadsUnixCreateTime(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer rawDB.Close()
	repository := &environmentPromotionStore{master: &BaseDB{DB: rawDB}}
	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT revision, topology_json, comment, create_by, UNIX_TIMESTAMP(ctime)
		FROM environment_promotion_topology_revision ORDER BY revision DESC`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"revision", "topology_json", "comment", "create_by", "ctime",
		}).AddRow(1, `{}`, "initial", "admin", int64(456)))

	revisions, err := repository.ListEnvironmentPromotionTopologyRevisions()

	require.NoError(t, err)
	require.Len(t, revisions, 1)
	require.Equal(t, time.Unix(456, 0), revisions[0].CreateTime)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSaveEnvironmentPromotionTopologyCreatesFirstDraftRevision(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer rawDB.Close()
	repository := &environmentPromotionStore{master: &BaseDB{DB: rawDB}}
	topology := &types.EnvironmentPromotionTopology{
		Edges: []types.EnvironmentPromotionEdge{{ID: "dev-tst", Source: "dev", Target: "tst"}},
	}
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT draft_revision FROM environment_promotion_topology WHERE id = 1 FOR UPDATE`)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO environment_promotion_topology").
		WithArgs(sqlmock.AnyArg(), "").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	require.NoError(t, repository.SaveEnvironmentPromotionTopology(topology, 0))
	require.Equal(t, uint64(1), topology.DraftRevision)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSaveEnvironmentPromotionTopologyRejectsStaleDraftRevision(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer rawDB.Close()
	repository := &environmentPromotionStore{master: &BaseDB{DB: rawDB}}
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT draft_revision FROM environment_promotion_topology WHERE id = 1 FOR UPDATE`)).
		WillReturnRows(sqlmock.NewRows([]string{"draft_revision"}).AddRow(3))
	mock.ExpectRollback()

	err = repository.SaveEnvironmentPromotionTopology(&types.EnvironmentPromotionTopology{}, 2)

	require.ErrorIs(t, err, store.ErrEnvironmentPromotionRevisionConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}
