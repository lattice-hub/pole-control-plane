package observabilityquery

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProviderOperationsRedactsHistoricalAccountCredentials(t *testing.T) {
	rows := [][]any{}
	for _, sample := range []struct{ resource, detail string }{
		{"User", `{"id":"visible","Password":"upper-sentinel","authToken":"camel-sentinel","auth_token":"snake-sentinel"}`},
		{"User", `{"id":"visible","metadata":{"json":"{\"password\":\"nested-sentinel\"}"}}`},
		{"User", `{"password":"malformed-sentinel"`},
		{"UserGroup", `{"id":"visible","relation":{"users":[{"Password":"member-sentinel"}]}}`},
		{"UserGroupRelation", `{"users":[{"id":"visible","auth_token":"relation-sentinel"}]}`},
		{"Routing", "release route rule"},
	} {
		attrs, err := json.Marshal(map[string]string{"pole.event.kind": "audit", "pole.resource.type": sample.resource,
			"pole.resource.name": "audit-resource", "pole.audit.operator": "admin", "pole.audit.operation": "Create"})
		require.NoError(t, err)
		rows = append(rows, []any{"2026-11-04 12:00:00", "INFO", sample.detail, string(attrs), "{}"})
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"output": []any{map[string]any{"records": map[string]any{"rows": rows}}}}))
	}))
	t.Cleanup(server.Close)
	provider := NewProvider(Config{Endpoint: server.URL})
	response, err := provider.Operations(context.Background(), OperationLogQuery{Limit: "10"})
	require.NoError(t, err)
	require.Len(t, response.Data, len(rows))
	require.Equal(t, uint32(len(rows)), response.Size)
	for _, record := range response.Data {
		require.NotContains(t, record.OperationDetail, "sentinel")
		require.Equal(t, "admin", record.Operator)
	}
	require.Contains(t, response.Data[0].OperationDetail, "visible")
	require.Equal(t, "release route rule", response.Data[len(rows)-1].OperationDetail)
	page, err := provider.Operations(context.Background(), OperationLogQuery{Limit: "1", ResourceType: "User"})
	require.NoError(t, err)
	require.Len(t, page.Data, 1)
	require.Equal(t, uint32(1), page.Size)
	require.True(t, page.HasNext)
}
