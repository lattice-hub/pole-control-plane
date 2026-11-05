package otel

import (
	"context"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	otelmetric "go.opentelemetry.io/otel/metric"

	"github.com/pole-io/pole-server/apis"
	metricstypes "github.com/pole-io/pole-server/apis/pkg/types/metrics"
	commonotel "github.com/pole-io/pole-server/pkg/common/otel"
)

const (
	PluginName = "otel"

	attrAPI              = "pole.api.name"
	attrCacheResult      = "pole.cache.result"
	attrCacheType        = "pole.cache.type"
	attrClientKind       = "pole.client.kind"
	attrComponent        = "pole.component"
	attrErrorCode        = "pole.error.code"
	attrNamespace        = "pole.namespace"
	attrOperation        = "pole.operation"
	attrProtocol         = "pole.protocol"
	attrResult           = "pole.result"
	attrService          = "pole.service.name"
	attrServiceStatus    = "pole.service.status"
	attrInstanceStatus   = "pole.instance.status"
	attrTrafficDirection = "pole.traffic.direction"
)

type StatisWorker struct {
	shutdown commonotel.OtelShutdown

	requestCount       otelmetric.Int64Counter
	requestDuration    otelmetric.Float64Histogram
	storeCount         otelmetric.Int64Counter
	storeDuration      otelmetric.Float64Histogram
	componentCount     otelmetric.Int64Counter
	componentDuration  otelmetric.Float64Histogram
	cacheCallCount     otelmetric.Int64Counter
	clientDiscoverCall otelmetric.Int64Counter
	clientDiscoverCost otelmetric.Float64Histogram

	clientConnectionCount otelmetric.Int64Gauge
	serviceCount          otelmetric.Int64Gauge
	instanceCount         otelmetric.Int64Gauge
}

func (s *StatisWorker) Name() string {
	return PluginName
}

func (s *StatisWorker) Initialize(conf *apis.ConfigEntry) error {
	opt, err := loadOptions(confOption(conf))
	if err != nil {
		return err
	}
	if opt.Endpoint != "" && opt.SetupSDK {
		shutdown, err := commonotel.SetupOTelSDK(context.Background(), &opt.Config)
		if err != nil {
			return err
		}
		s.shutdown = shutdown
	}
	return s.registerMetrics()
}

func (s *StatisWorker) Type() apis.PluginType {
	return apis.PluginTypeStatis
}

func (s *StatisWorker) Destroy() error {
	if s.shutdown == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.shutdown(ctx)
}

func (s *StatisWorker) ReportCallMetrics(metric metricstypes.CallMetric) {
	switch metric.Type {
	case metricstypes.ServerCallMetric:
		attrs := callAttrs(metric, "apiserver")
		s.requestCount.Add(context.Background(), times(metric), otelmetric.WithAttributes(attrs...))
		recordDuration(s.requestDuration, metric.Duration, attrs)
	case metricstypes.StoreCallMetric:
		attrs := callAttrs(metric, "store")
		s.storeCount.Add(context.Background(), times(metric), otelmetric.WithAttributes(attrs...))
		recordDuration(s.storeDuration, metric.Duration, attrs)
	case metricstypes.ProtobufCacheCallMetric, metricstypes.DiscoverCacheCallMetric:
		attrs := []attribute.KeyValue{
			attribute.String(attrCacheType, valueOrUnknown(metric.Protocol)),
			attribute.String(attrCacheResult, cacheResult(metric.Success)),
		}
		s.cacheCallCount.Add(context.Background(), times(metric), otelmetric.WithAttributes(attrs...))
	default:
		attrs := callAttrs(metric, componentName(metric.Type))
		s.componentCount.Add(context.Background(), times(metric), otelmetric.WithAttributes(attrs...))
		recordDuration(s.componentDuration, metric.Duration, attrs)
	}
}

func (s *StatisWorker) ReportDiscoveryMetrics(metric ...metricstypes.DiscoveryMetric) {
	for i := range metric {
		m := metric[i]
		switch m.Type {
		case metricstypes.ClientMetrics:
			s.clientConnectionCount.Record(context.Background(), m.Total, otelmetric.WithAttributes(
				attribute.String(attrClientKind, "sdk"),
			))
		case metricstypes.ServiceMetrics:
			attrs := discoveryAttrs(m.Labels)
			recordStatusGauge(s.serviceCount, m.Total, attrs, attrServiceStatus, "total")
			recordStatusGauge(s.serviceCount, m.Online, attrs, attrServiceStatus, "online")
			recordStatusGauge(s.serviceCount, m.Offline, attrs, attrServiceStatus, "offline")
			recordStatusGauge(s.serviceCount, m.Abnormal, attrs, attrServiceStatus, "abnormal")
		case metricstypes.InstanceMetrics:
			attrs := discoveryAttrs(m.Labels)
			recordStatusGauge(s.instanceCount, m.Total, attrs, attrInstanceStatus, "total")
			recordStatusGauge(s.instanceCount, m.Online, attrs, attrInstanceStatus, "online")
			recordStatusGauge(s.instanceCount, m.Abnormal, attrs, attrInstanceStatus, "abnormal")
			recordStatusGauge(s.instanceCount, m.Isolate, attrs, attrInstanceStatus, "isolate")
		}
	}
}

func (s *StatisWorker) ReportDiscoverCall(metric metricstypes.ClientDiscoverMetric) {
	attrs := []attribute.KeyValue{
		attribute.String(attrOperation, valueOrUnknown(metric.Action)),
		attribute.String(attrNamespace, valueOrUnknown(metric.Namespace)),
		attribute.String(attrResult, resultFromBool(metric.Success)),
	}
	s.clientDiscoverCall.Add(context.Background(), 1, otelmetric.WithAttributes(attrs...))
	if metric.CostTime > 0 {
		s.clientDiscoverCost.Record(
			context.Background(),
			float64(metric.CostTime)/float64(time.Second.Milliseconds()),
			otelmetric.WithAttributes(attrs...),
		)
	}
}

func (s *StatisWorker) registerMetrics() error {
	meter := commonotel.Meter()
	var err error
	if s.requestCount, err = meter.Int64Counter("pole.control_plane.request.count",
		otelmetric.WithDescription("control-plane API request count"),
		otelmetric.WithUnit("{request}"),
	); err != nil {
		return err
	}
	if s.requestDuration, err = meter.Float64Histogram("pole.control_plane.request.duration",
		otelmetric.WithDescription("control-plane API request duration"),
		otelmetric.WithUnit("s"),
		otelmetric.WithExplicitBucketBoundaries(0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10),
	); err != nil {
		return err
	}
	if s.storeCount, err = meter.Int64Counter("pole.control_plane.store.request.count",
		otelmetric.WithDescription("control-plane store request count"),
		otelmetric.WithUnit("{request}"),
	); err != nil {
		return err
	}
	if s.storeDuration, err = meter.Float64Histogram("pole.control_plane.store.request.duration",
		otelmetric.WithDescription("control-plane store request duration"),
		otelmetric.WithUnit("s"),
	); err != nil {
		return err
	}
	if s.componentCount, err = meter.Int64Counter("pole.control_plane.component.request.count",
		otelmetric.WithDescription("control-plane internal component operation count"),
		otelmetric.WithUnit("{request}"),
	); err != nil {
		return err
	}
	if s.componentDuration, err = meter.Float64Histogram("pole.control_plane.component.request.duration",
		otelmetric.WithDescription("control-plane internal component operation duration"),
		otelmetric.WithUnit("s"),
	); err != nil {
		return err
	}
	if s.cacheCallCount, err = meter.Int64Counter("pole.control_plane.cache.call.count",
		otelmetric.WithDescription("control-plane cache call count"),
		otelmetric.WithUnit("{call}"),
	); err != nil {
		return err
	}
	if s.clientDiscoverCall, err = meter.Int64Counter("pole.control_plane.client.discover.count",
		otelmetric.WithDescription("control-plane client discover/config call count"),
		otelmetric.WithUnit("{request}"),
	); err != nil {
		return err
	}
	if s.clientDiscoverCost, err = meter.Float64Histogram("pole.control_plane.client.discover.duration",
		otelmetric.WithDescription("control-plane client discover/config call duration"),
		otelmetric.WithUnit("s"),
	); err != nil {
		return err
	}
	if s.clientConnectionCount, err = meter.Int64Gauge("pole.control_plane.client.connection.count",
		otelmetric.WithDescription("control-plane SDK/config/discovery client connection count"),
		otelmetric.WithUnit("{connection}"),
	); err != nil {
		return err
	}
	if s.serviceCount, err = meter.Int64Gauge("pole.discovery.service.count",
		otelmetric.WithDescription("Pole service count by namespace and status"),
		otelmetric.WithUnit("{service}"),
	); err != nil {
		return err
	}
	if s.instanceCount, err = meter.Int64Gauge("pole.discovery.instance.count",
		otelmetric.WithDescription("Pole instance count by service and status"),
		otelmetric.WithUnit("{instance}"),
	); err != nil {
		return err
	}
	return nil
}

func confOption(conf *apis.ConfigEntry) map[string]interface{} {
	if conf == nil {
		return nil
	}
	return conf.Option
}

func callAttrs(metric metricstypes.CallMetric, component string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String(attrComponent, component),
		attribute.String(attrAPI, valueOrUnknown(metric.API)),
		attribute.String(attrProtocol, valueOrUnknown(metric.Protocol)),
		attribute.String(attrResult, resultFromCode(metric.Code, metric.Success)),
		attribute.String(attrErrorCode, strconv.Itoa(metric.Code)),
		attribute.String(attrTrafficDirection, trafficDirection(metric.TrafficDirection)),
	}
}

func discoveryAttrs(labels []attribute.KeyValue) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String(attrNamespace, valueOrUnknown(labelValue(labels, metricstypes.LabelNamespace))),
		attribute.String(attrService, labelValue(labels, metricstypes.LabelService)),
	}
}

func recordStatusGauge(gauge otelmetric.Int64Gauge, value int64, attrs []attribute.KeyValue, statusKey, status string) {
	labels := append([]attribute.KeyValue{}, attrs...)
	labels = append(labels, attribute.String(statusKey, status))
	gauge.Record(context.Background(), value, otelmetric.WithAttributes(labels...))
}

func recordDuration(histogram otelmetric.Float64Histogram, duration time.Duration, attrs []attribute.KeyValue) {
	if duration <= 0 {
		return
	}
	histogram.Record(context.Background(), duration.Seconds(), otelmetric.WithAttributes(attrs...))
}

func labelValue(labels []attribute.KeyValue, key string) string {
	for i := range labels {
		if string(labels[i].Key) == key {
			return labels[i].Value.AsString()
		}
	}
	return ""
}

func times(metric metricstypes.CallMetric) int64 {
	if metric.Times <= 0 {
		return 1
	}
	return int64(metric.Times)
}

func resultFromCode(code int, success bool) string {
	if success || code == 0 || code == 200000 || code >= 200 && code < 300 {
		return "success"
	}
	return "failure"
}

func resultFromBool(success bool) string {
	if success {
		return "success"
	}
	return "failure"
}

func cacheResult(success bool) string {
	if success {
		return "hit"
	}
	return "miss"
}

func trafficDirection(direction metricstypes.TrafficDirection) string {
	if direction == "" {
		return string(metricstypes.TrafficDirectionInBound)
	}
	return string(direction)
}

func componentName(t metricstypes.CallMetricType) string {
	switch t {
	case metricstypes.SystemCallMetric:
		return "system"
	case metricstypes.RedisCallMetric:
		return "redis"
	case metricstypes.XDSResourceBuildCallMetric:
		return "xds"
	default:
		return string(t)
	}
}

func valueOrUnknown(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "unknown"
	}
	return v
}
