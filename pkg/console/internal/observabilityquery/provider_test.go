package observabilityquery

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProviderPlatformOverviewQueriesGreptimeDB(t *testing.T) {
	queries := make([]string, 0, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/sql", r.URL.Path)
		require.Equal(t, "public", r.URL.Query().Get("db"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		queries = append(queries, string(body))

		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(string(body), "pole_control_plane_request_count_total"):
			_, _ = w.Write([]byte(`{"output":[{"records":{"rows":[[1784386237437,199,"pole-control-plane","ListServices","apiserver","HTTP","success","pole-system"],[1784386236437,100,"pole-control-plane","ListServices","apiserver","HTTP","success","pole-system"],[1784386237437,1,"pole-control-plane","ListServices","apiserver","HTTP","failure","pole-system"],[1784386236437,0,"pole-control-plane","ListServices","apiserver","HTTP","failure","pole-system"]]}}]}`))
		case strings.Contains(string(body), "pole_control_plane_request_duration_seconds_bucket"):
			_, _ = w.Write([]byte(`{"output":[{"records":{"rows":[[1784386237437,90,"pole-control-plane","ListServices","apiserver","HTTP","success","0.01","pole-system"],[1784386236437,0,"pole-control-plane","ListServices","apiserver","HTTP","success","0.01","pole-system"],[1784386237437,95,"pole-control-plane","ListServices","apiserver","HTTP","success","0.025","pole-system"],[1784386236437,0,"pole-control-plane","ListServices","apiserver","HTTP","success","0.025","pole-system"],[1784386237437,99,"pole-control-plane","ListServices","apiserver","HTTP","success","0.05","pole-system"],[1784386236437,0,"pole-control-plane","ListServices","apiserver","HTTP","success","0.05","pole-system"],[1784386237437,100,"pole-control-plane","ListServices","apiserver","HTTP","success","inf","pole-system"],[1784386236437,0,"pole-control-plane","ListServices","apiserver","HTTP","success","inf","pole-system"]]}}]}`))
		case strings.Contains(string(body), "process_runtime_go_goroutines"):
			_, _ = w.Write([]byte(`{"output":[{"records":{"rows":[[1784386237436,128],[1784386237437,144]]}}]}`))
		case strings.Contains(string(body), "process_runtime_go_mem_heap_alloc_bytes"):
			_, _ = w.Write([]byte(`{"output":[{"records":{"rows":[[1784386237436,10485760],[1784386237437,12582912]]}}]}`))
		case strings.Contains(string(body), "process_runtime_go_gc_count_total"):
			_, _ = w.Write([]byte(`{"output":[{"records":{"rows":[[1784386237436,5],[1784386237437,8]]}}]}`))
		case strings.Contains(string(body), "process_runtime_go_gc_pause_ns_sum"):
			_, _ = w.Write([]byte(`{"output":[{"records":{"rows":[[1784386237436,2000000],[1784386237437,8000000]]}}]}`))
		case strings.Contains(string(body), "process_runtime_go_gc_pause_ns_count"):
			_, _ = w.Write([]byte(`{"output":[{"records":{"rows":[[1784386237436,1],[1784386237437,3]]}}]}`))
		default:
			_, _ = w.Write([]byte(`{"output":[{"records":{"rows":[]}}]}`))
		}
	}))
	t.Cleanup(server.Close)

	provider := NewProvider(Config{Endpoint: server.URL})
	overview, err := provider.PlatformOverview(context.Background(), PlatformOverviewQuery{
		Category:  "control-plane",
		API:       "ListServices",
		StartTime: "1784386236437",
		EndTime:   "1784386237437",
	})
	require.NoError(t, err)

	require.True(t, overview.Provider.Configured)
	require.Equal(t, "public", overview.Provider.Database)
	require.Len(t, overview.Components, 1)
	component := overview.Components[0]
	require.Equal(t, "pole-control-plane", component.Component)
	require.Equal(t, "ListServices", component.API)
	require.Equal(t, "control-plane", component.Category)
	require.Equal(t, 100.0, component.RequestCount)
	require.Equal(t, 100.0, component.QPS)
	require.Equal(t, 1.0, component.ErrorRate)
	require.Equal(t, 25.0, component.P95)
	require.Equal(t, 50.0, component.P99)
	require.Equal(t, "pole-system", component.Namespace)
	require.Empty(t, component.Pod)
	require.Equal(t, "HTTP", component.Protocol)
	require.NotEmpty(t, component.Series)
	require.NotEmpty(t, overview.Runtime)
	require.Equal(t, "process.runtime.go.goroutines", overview.Runtime[0].Name)
	require.Equal(t, 144.0, overview.Runtime[0].Value)
	require.Empty(t, overview.Resources)
	require.Len(t, overview.Stats, 4)
	require.Len(t, overview.Series, 1)
	require.GreaterOrEqual(t, len(queries), 7)
	require.Contains(t, queries[0], "pole_api_name+%3D+%27ListServices%27")
}

func TestProviderPlatformTracesAndDetailQueryGreptimeDB(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Contains(t, string(body), "pole_traces")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":[{"records":{"rows":[
			["2026-08-05 16:00:00.000123","2026-08-05 16:00:00.020123",20000000,"","trace-a","span-root","SPAN_KIND_SERVER","GET:/naming/v1/services","STATUS_CODE_OK","","pole-control-plane"],
			["2026-08-05 16:00:00.004567","2026-08-05 16:00:00.014567",10000000,"span-root","trace-a","span-child","SPAN_KIND_INTERNAL","store.ListServices","STATUS_CODE_ERROR","mysql timeout","pole-control-plane"]
		]}}]}`))
	}))
	t.Cleanup(server.Close)

	provider := NewProvider(Config{Endpoint: server.URL})
	traces, err := provider.PlatformTraces(context.Background(), TraceQuery{
		StartTime: "1785916799000",
		EndTime:   "1785916801000",
		Service:   "pole-control-plane",
		API:       "GET:/naming/v1/services",
		Limit:     "10",
	})
	require.NoError(t, err)
	require.Len(t, traces.Data, 1)
	require.Equal(t, "trace-a", traces.Data[0].TraceID)
	require.Equal(t, "error", traces.Data[0].Status)
	require.Equal(t, 20.0, traces.Data[0].DurationMS)
	require.Equal(t, 2, traces.Data[0].SpanCount)

	detail, err := provider.TraceDetail(context.Background(), "trace-a")
	require.NoError(t, err)
	require.Equal(t, "trace-a", detail.Summary.TraceID)
	require.Len(t, detail.Spans, 2)
	require.Equal(t, "mysql timeout", detail.Spans[1].StatusMessage)
	require.Equal(t, int64(1785916800000123), detail.Spans[0].StartTimeUS)
	require.Equal(t, int64(1785916800004567), detail.Spans[1].StartTimeUS)
}

func TestSummarizeTracesPreservesSubMillisecondDuration(t *testing.T) {
	summaries := summarizeTraces([]TraceSpan{{
		TraceID:     "trace-sub-ms",
		SpanID:      "span-root",
		Name:        "GET:/health",
		ServiceName: "pole-control-plane",
		StartTime:   1000,
		EndTime:     1000,
		StartTimeUS: 1000000,
		EndTimeUS:   1000590,
		DurationMS:  0.59,
	}})

	require.Len(t, summaries, 1)
	require.Equal(t, 0.59, summaries[0].DurationMS)
}

func TestProviderPlatformOverviewReturnsEmptyWhenMetricTableMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":[{"error":"Table not found: pole_control_plane_request_count_total"}]}`))
	}))
	t.Cleanup(server.Close)

	provider := NewProvider(Config{Endpoint: server.URL})
	overview, err := provider.PlatformOverview(context.Background(), PlatformOverviewQuery{})
	require.NoError(t, err)
	require.True(t, overview.Provider.Configured)
	require.Empty(t, overview.Components)
	require.Empty(t, overview.Stats)
}

func TestProviderServiceEventsQueriesPoleEvents(t *testing.T) {
	var queryBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/sql", r.URL.Path)
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		queryBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":[{"records":{"rows":[
			["2026-07-19 09:05:00","INFO","default/checkout/10.0.0.1:8080","{\"pole.event.kind\":\"service\",\"event.name\":\"InstanceOffline\",\"pole.discovery.event\":\"InstanceOffline\",\"pole.namespace\":\"default\",\"pole.service.name\":\"checkout\",\"pole.resource.name\":\"default/checkout/10.0.0.1:8080\",\"pole.server.address\":\"node-0\"}","{\"service.name\":\"pole-control-plane\"}"],
			["2026-07-19 09:04:00","INFO","audit body","{\"pole.event.kind\":\"audit\",\"event.name\":\"pole.audit.operation\"}","{}"]
		]}}]}`))
	}))
	t.Cleanup(server.Close)

	provider := NewProvider(Config{Endpoint: server.URL})
	events, err := provider.ServiceEvents(context.Background(), EventLogQuery{
		Namespace: "default",
		Service:   "checkout",
		EventType: "InstanceOffline",
		Limit:     "10",
	})
	require.NoError(t, err)
	require.Contains(t, queryBody, "pole_events")
	require.Len(t, events.Data, 1)
	require.Equal(t, "InstanceOffline", events.Data[0].EventType)
	require.Equal(t, "default", events.Data[0].Namespace)
	require.Equal(t, "checkout", events.Data[0].Service)
	require.Equal(t, "node-0", events.Data[0].Server)
	require.Equal(t, uint32(1), events.Size)
}

func TestProviderOperationsQueriesPoleEvents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/sql", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":[{"records":{"rows":[
			["2026-07-19 09:07:00","INFO","release route rule","{\"pole.event.kind\":\"audit\",\"event.name\":\"pole.audit.operation\",\"pole.audit.operation\":\"Update\",\"pole.audit.operator\":\"admin\",\"pole.namespace\":\"mall-prod\",\"pole.resource.type\":\"Routing\",\"pole.resource.name\":\"checkout-gray-route\",\"pole.server.address\":\"node-1\"}","{}"],
			["2026-07-19 09:06:00","INFO","default/checkout","{\"pole.event.kind\":\"service\",\"event.name\":\"InstanceOnline\"}","{}"]
		]}}]}`))
	}))
	t.Cleanup(server.Close)

	provider := NewProvider(Config{Endpoint: server.URL})
	operations, err := provider.Operations(context.Background(), OperationLogQuery{
		ResourceType:  "Routing",
		OperationType: "Update",
		Operator:      "admin",
		Limit:         "10",
	})
	require.NoError(t, err)
	require.Len(t, operations.Data, 1)
	require.Equal(t, "Routing", operations.Data[0].ResourceType)
	require.Equal(t, "checkout-gray-route", operations.Data[0].ResourceName)
	require.Equal(t, "Update", operations.Data[0].OperationType)
	require.Equal(t, "release route rule", operations.Data[0].OperationDetail)
	require.Equal(t, "node-1", operations.Data[0].Server)
	require.Equal(t, uint32(1), operations.Size)
}
