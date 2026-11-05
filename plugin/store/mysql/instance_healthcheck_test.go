package sqldb

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	apiservice "github.com/pole-io/specification/source/go/api/v1/service_manage"
)

func TestFetchInstanceWithMetaRowsRestoresHTTPHealthCheckPath(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	columns := []string{
		"id", "service_id", "vpc_id", "host", "port", "protocol", "version",
		"health_status", "isolate", "weight", "enable_health_check", "logic_set",
		"region", "zone", "campus", "priority", "revision", "flag", "check_type",
		"ttl", "ctime", "mtime", "metadata",
	}
	rows := sqlmock.NewRows(columns).AddRow(
		"instance-id", "service-id", "", "127.0.0.1", 8080, "HTTP", "v1",
		1, 0, 100, 1, "", "", "", "", 0, "revision", 0,
		int32(apiservice.HealthCheck_HTTP), 5, int64(1), int64(2),
		`{"internal-healthcheck_path":"/ready"}`,
	)
	mock.ExpectQuery("SELECT instance health check").WillReturnRows(rows)

	resultRows, err := db.Query("SELECT instance health check")
	require.NoError(t, err)
	instances, err := fetchInstanceWithMetaRows(resultRows)
	require.NoError(t, err)

	instance := instances["instance-id"]
	require.NotNil(t, instance)
	require.Equal(t, "/ready", instance.HealthCheck().GetHttp().GetPath())
	require.Equal(t, "/ready", instance.Metadata()["internal-healthcheck_path"])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFetchExpandInstanceRowsScansMetadataColumn(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	columns := []string{
		"id", "service_id", "vpc_id", "host", "port", "protocol", "version",
		"health_status", "isolate", "weight", "enable_health_check", "logic_set",
		"region", "zone", "campus", "priority", "revision", "flag", "check_type",
		"ttl", "service_name", "namespace", "ctime", "mtime", "metadata",
	}
	rows := sqlmock.NewRows(columns).AddRow(
		"instance-id", "service-id", "", "127.0.0.1", 8080, "HTTP", "v1",
		1, 0, 100, 1, "", "", "", "", 0, "revision", 0,
		int32(apiservice.HealthCheck_HTTP), 5, "service-name", "namespace-name",
		int64(1), int64(2), `{"internal-healthcheck_path":"/ready"}`,
	)
	mock.ExpectQuery("SELECT expanded instances").WillReturnRows(rows)

	resultRows, err := db.Query("SELECT expanded instances")
	require.NoError(t, err)
	instances, err := fetchExpandInstanceRows(resultRows)
	require.NoError(t, err)
	require.Len(t, instances, 1)
	require.Equal(t, "service-name", instances[0].Service())
	require.Equal(t, "namespace-name", instances[0].Namespace())
	require.Equal(t, "/ready", instances[0].HealthCheck().GetHttp().GetPath())
	require.Equal(t, "/ready", instances[0].Metadata()["internal-healthcheck_path"])
	require.NoError(t, mock.ExpectationsWereMet())
}
