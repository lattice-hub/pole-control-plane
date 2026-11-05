package mysql

import (
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestGetHistoryRedactsAccountCredentialsAndPreservesPagination(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	fetcher := &OperationFetcher{slave: &BaseDB{DB: db}}
	rows := sqlmock.NewRows([]string{"id", "resource_type", "namespace", "resource_name", "operation_type", "operator", "detail", "happen_time", "server"}).
		AddRow(9, "User", "default", "user", "Create", "admin", `{"id":"visible","Password":"password-sentinel","authToken":"token-sentinel","metadata":{"json":"{\"auth_token\":\"nested-sentinel\"}"}}`, "2026-11-04 12:00:00", "server").
		AddRow(8, "UserGroup", "default", "group", "Update", "admin", `{"auth_token":"malformed-sentinel"`, "2026-11-04 11:00:00", "server").
		AddRow(7, "Routing", "default", "route", "Update", "admin", "release route rule", "2026-11-04 10:00:00", "server")
	query := "SELECT id, resource_type, namespace, resource_name, operation_type, operator, detail, happen_time, server FROM operation_history WHERE 1=1 ORDER BY id DESC LIMIT ? OFFSET ?"
	mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs(3, 3).WillReturnRows(rows)
	records, err := fetcher.GetHistory(map[string]string{"limit": "3", "offset": "3"})
	require.NoError(t, err)
	require.Len(t, records, 3)
	for _, record := range records {
		require.NotContains(t, record.OperationDetail, "sentinel")
	}
	require.Contains(t, records[0].OperationDetail, "visible")
	require.Equal(t, "Create", records[0].OperationType)
	require.Equal(t, "admin", records[0].Operator)
	require.Equal(t, "release route rule", records[2].OperationDetail)
	require.NoError(t, mock.ExpectationsWereMet())
	mock.ExpectClose()
}
