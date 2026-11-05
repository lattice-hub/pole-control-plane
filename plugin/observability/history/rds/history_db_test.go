package logger

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/pole-io/pole-server/apis/pkg/types"
)

func TestHistoryDBNeverWritesAccountCredentials(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	worker := &HistoryDB{db: db}
	for _, resource := range []types.Resource{types.RUser, types.RUserGroup, types.RUserGroupRelation} {
		entry := &types.RecordEntry{ResourceType: resource, ResourceName: "audit-user", Operator: "admin", OperationType: types.OCreate,
			Detail: `{"id":"visible","Password":"rds-password-sentinel","auth_token":"rds-token-sentinel"}`, HappenTime: time.Unix(1, 0)}
		mock.ExpectExec("INSERT INTO operation_history").WithArgs(resource, "", "audit-user", types.OCreate, "admin",
			`{"id":"visible"}`, entry.HappenTime, sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
		worker.Record(entry)
	}
	mock.ExpectExec("INSERT INTO operation_history").WithArgs(types.RRouting, "", "route", types.OUpdate, "admin",
		"release route rule", time.Unix(1, 0), sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(2, 1))
	worker.Record(&types.RecordEntry{ResourceType: types.RRouting, ResourceName: "route", Operator: "admin", OperationType: types.OUpdate,
		Detail: "release route rule", HappenTime: time.Unix(1, 0)})
	require.NoError(t, mock.ExpectationsWereMet())
	mock.ExpectClose()
}
