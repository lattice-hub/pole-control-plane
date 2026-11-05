package otel

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pole-io/pole-server/apis"
	"github.com/pole-io/pole-server/apis/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestHistoryWorkerEmitsAuditLogRecord(t *testing.T) {
	payloadCh := make(chan map[string]any, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var payload map[string]any
		require.NoError(t, json.Unmarshal(body, &payload))
		payloadCh <- payload
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	worker := &HistoryWorker{}
	require.NoError(t, worker.Initialize(&apis.ConfigEntry{
		Option: map[string]interface{}{
			"logsEndpoint": server.URL,
			"serviceName":  "pole-control-plane",
			"logQueueSize": 4,
		},
	}))
	worker.Record(&types.RecordEntry{
		ResourceType:  types.RRouting,
		ResourceName:  "checkout-gray",
		Namespace:     "default",
		Operator:      "admin",
		OperationType: types.OUpdate,
		Detail:        "update route",
		HappenTime:    time.Unix(1, 0),
	})
	require.NoError(t, worker.Destroy())

	select {
	case payload := <-payloadCh:
		record := firstLogRecord(t, payload)
		require.Equal(t, "update route", record["body"].(map[string]any)["stringValue"])
		attrs := attrMap(record["attributes"].([]any))
		require.Equal(t, eventNameAuditOperation, attrs["event.name"])
		require.Equal(t, "audit", attrs["pole.event.kind"])
		require.Equal(t, "Routing", attrs["pole.resource.type"])
		require.Equal(t, "Update", attrs["pole.audit.operation"])
	case <-time.After(time.Second):
		t.Fatal("history otel payload was not emitted")
	}
}

func firstLogRecord(t *testing.T, payload map[string]any) map[string]any {
	t.Helper()
	resourceLogs := payload["resourceLogs"].([]any)
	scopeLogs := resourceLogs[0].(map[string]any)["scopeLogs"].([]any)
	records := scopeLogs[0].(map[string]any)["logRecords"].([]any)
	return records[0].(map[string]any)
}

func attrMap(attrs []any) map[string]string {
	out := make(map[string]string, len(attrs))
	for _, attr := range attrs {
		item := attr.(map[string]any)
		out[item["key"].(string)] = item["value"].(map[string]any)["stringValue"].(string)
	}
	return out
}

func TestHistoryWorkerExportsSafeAccountDetails(t *testing.T) {
	payloads := make(chan string, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		payloads <- string(body)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	worker := &HistoryWorker{}
	require.NoError(t, worker.Initialize(&apis.ConfigEntry{Option: map[string]interface{}{
		"logsEndpoint": server.URL, "serviceName": "pole-control-plane", "logQueueSize": 4,
	}}))
	for _, resource := range []types.Resource{types.RUser, types.RUserGroup, types.RUserGroupRelation} {
		worker.Record(&types.RecordEntry{ResourceType: resource, ResourceName: "audit-resource", OperationType: types.OCreate,
			Detail: `{"id":"visible","Password":"otel-password-sentinel","metadata":{"nested":"{\"auth_token\":\"otel-token-sentinel\"}"}}`})
	}
	require.NoError(t, worker.Destroy())
	select {
	case payload := <-payloads:
		require.NotContains(t, payload, "sentinel")
		require.Contains(t, payload, "visible")
		require.Contains(t, payload, "pole.audit.operation")
	case <-time.After(time.Second):
		t.Fatal("account audit payload was not exported")
	}
	for len(payloads) > 0 {
		require.NotContains(t, <-payloads, "sentinel")
	}
}
