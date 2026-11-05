package otel

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/pole-io/pole-server/apis"
	metricstypes "github.com/pole-io/pole-server/apis/pkg/types/metrics"
)

func TestStatisWorkerReportsControlPlaneMetrics(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)
	t.Cleanup(func() {
		require.NoError(t, provider.Shutdown(context.Background()))
		otel.SetMeterProvider(metricnoop.NewMeterProvider())
	})

	worker := &StatisWorker{}
	require.NoError(t, worker.Initialize(&apis.ConfigEntry{Name: PluginName}))
	t.Cleanup(func() {
		require.NoError(t, worker.Destroy())
	})

	worker.ReportCallMetrics(metricstypes.CallMetric{
		Type:     metricstypes.ServerCallMetric,
		API:      "CreateService",
		Protocol: "HTTP",
		Code:     200000,
		Duration: 25 * time.Millisecond,
	})
	worker.ReportCallMetrics(metricstypes.CallMetric{
		Type:     metricstypes.StoreCallMetric,
		API:      "Exec",
		Protocol: "MySQL",
		Code:     0,
		Success:  true,
		Duration: 5 * time.Millisecond,
	})
	worker.ReportCallMetrics(metricstypes.CallMetric{
		Type:     metricstypes.DiscoverCacheCallMetric,
		Protocol: "gRPC",
		Success:  true,
		Times:    2,
	})

	metrics := collectMetrics(t, reader)
	require.Contains(t, metrics, "pole.control_plane.request.count")
	require.Contains(t, metrics, "pole.control_plane.request.duration")
	require.Contains(t, metrics, "pole.control_plane.store.request.count")
	require.Contains(t, metrics, "pole.control_plane.store.request.duration")
	require.Contains(t, metrics, "pole.control_plane.cache.call.count")
}

func TestResultFromCodeTreatsHTTPSuccessAsSuccess(t *testing.T) {
	require.Equal(t, "success", resultFromCode(200, false))
	require.Equal(t, "success", resultFromCode(204, false))
	require.Equal(t, "failure", resultFromCode(400, false))
	require.Equal(t, "failure", resultFromCode(500, false))
}

func TestStatisWorkerReportsDiscoveryMetrics(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)
	t.Cleanup(func() {
		require.NoError(t, provider.Shutdown(context.Background()))
		otel.SetMeterProvider(metricnoop.NewMeterProvider())
	})

	worker := &StatisWorker{}
	require.NoError(t, worker.Initialize(&apis.ConfigEntry{Name: PluginName}))
	t.Cleanup(func() {
		require.NoError(t, worker.Destroy())
	})

	worker.ReportDiscoveryMetrics(
		metricstypes.DiscoveryMetric{
			Type:    metricstypes.ServiceMetrics,
			Total:   3,
			Online:  2,
			Offline: 1,
			Labels: []attribute.KeyValue{
				attribute.String(metricstypes.LabelNamespace, "default"),
			},
		},
		metricstypes.DiscoveryMetric{
			Type:    metricstypes.InstanceMetrics,
			Total:   4,
			Online:  3,
			Isolate: 1,
			Labels: []attribute.KeyValue{
				attribute.String(metricstypes.LabelNamespace, "default"),
				attribute.String(metricstypes.LabelService, "checkout"),
			},
		},
		metricstypes.DiscoveryMetric{
			Type:  metricstypes.ClientMetrics,
			Total: 5,
		},
	)

	metrics := collectMetrics(t, reader)
	require.Contains(t, metrics, "pole.discovery.service.count")
	require.Contains(t, metrics, "pole.discovery.instance.count")
	require.Contains(t, metrics, "pole.control_plane.client.connection.count")
}

func TestStatisWorkerExportsToCollector(t *testing.T) {
	endpoint := os.Getenv("POLE_OTEL_COLLECTOR_ENDPOINT")
	if endpoint == "" {
		t.Skip("set POLE_OTEL_COLLECTOR_ENDPOINT to run collector integration smoke")
	}

	worker := &StatisWorker{}
	require.NoError(t, worker.Initialize(&apis.ConfigEntry{
		Name: PluginName,
		Option: map[string]interface{}{
			"endpoint":     endpoint,
			"pushInterval": "1s",
			"timeout":      "5s",
			"compressor":   "gzip",
			"serviceName":  "pole-control-plane-test",
			"environment":  "local",
			"cluster":      "local-docker",
		},
	}))

	worker.ReportCallMetrics(metricstypes.CallMetric{
		Type:     metricstypes.ServerCallMetric,
		API:      "OtelSmoke",
		Protocol: "HTTP",
		Code:     200000,
		Duration: 15 * time.Millisecond,
	})

	time.Sleep(2 * time.Second)
	require.NoError(t, worker.Destroy())
}

func collectMetrics(t *testing.T, reader *sdkmetric.ManualReader) map[string]metricdata.Metrics {
	t.Helper()

	var data metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &data))

	metrics := make(map[string]metricdata.Metrics)
	for _, scopeMetrics := range data.ScopeMetrics {
		for _, metric := range scopeMetrics.Metrics {
			metrics[metric.Name] = metric
		}
	}
	return metrics
}
