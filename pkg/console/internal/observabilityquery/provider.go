package observabilityquery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pole-io/pole-server/apis/pkg/types"
	"github.com/pole-io/pole-server/pkg/console/internal/common/model"
)

type Provider interface {
	PlatformOverview(ctx context.Context, query PlatformOverviewQuery) (*PlatformOverview, error)
	PlatformTraces(ctx context.Context, query TraceQuery) (*TraceList, error)
	TraceDetail(ctx context.Context, traceID string) (*TraceDetail, error)
	ServiceEvents(ctx context.Context, query EventLogQuery) (*model.EventRecordLogResponse, error)
	Operations(ctx context.Context, query OperationLogQuery) (*model.OperationLogResponse, error)
}

type PlatformOverviewQuery struct {
	StartTime string `json:"start_time,omitempty"`
	EndTime   string `json:"end_time,omitempty"`
	Step      string `json:"step,omitempty"`
	Category  string `json:"category,omitempty"`
	API       string `json:"api,omitempty"`
}

type EventLogQuery struct {
	Namespace string
	Service   string
	Resource  string
	EventType string
	StartTime string
	EndTime   string
	Limit     string
	Cursor    string
	Direction string
}

type OperationLogQuery struct {
	Namespace     string
	ResourceType  string
	ResourceName  string
	OperationType string
	Operator      string
	StartTime     string
	EndTime       string
	Limit         string
	Cursor        string
	Direction     string
}

type ProviderStatus struct {
	Provider   string `json:"provider"`
	Endpoint   string `json:"endpoint,omitempty"`
	Database   string `json:"database,omitempty"`
	Configured bool   `json:"configured"`
}

type StatValue struct {
	Name   string            `json:"name"`
	Value  float64           `json:"value"`
	Unit   string            `json:"unit,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
}

type PlatformOverview struct {
	Provider   ProviderStatus            `json:"provider"`
	Stats      []StatValue               `json:"stats"`
	Series     []TimeSeries              `json:"series"`
	Components []PlatformComponentMetric `json:"components"`
	Resources  []PlatformResourceMetric  `json:"resources"`
	Runtime    []PlatformRuntimeMetric   `json:"runtime"`
}

type PlatformComponentMetric struct {
	ID           string    `json:"id"`
	Category     string    `json:"category"`
	Component    string    `json:"component"`
	Role         string    `json:"role"`
	Namespace    string    `json:"namespace"`
	Pod          string    `json:"pod"`
	API          string    `json:"api"`
	Status       string    `json:"status"`
	CPU          float64   `json:"cpu"`
	Memory       float64   `json:"memory"`
	RequestCount float64   `json:"requestCount"`
	QPS          float64   `json:"qps"`
	P95          float64   `json:"p95"`
	P99          float64   `json:"p99"`
	ErrorRate    float64   `json:"errorRate"`
	RestartCount int64     `json:"restartCount"`
	Protocol     string    `json:"protocol"`
	Series       []float64 `json:"series"`
}

type PlatformResourceMetric struct {
	ID           string    `json:"id"`
	Component    string    `json:"component"`
	Namespace    string    `json:"namespace"`
	Pod          string    `json:"pod"`
	CPU          float64   `json:"cpu"`
	Memory       float64   `json:"memory"`
	MemoryUnit   string    `json:"memoryUnit"`
	RestartCount int64     `json:"restartCount"`
	CPUSeries    []float64 `json:"cpuSeries"`
	MemorySeries []float64 `json:"memorySeries"`
}

type PlatformRuntimeMetric struct {
	Name        string    `json:"name"`
	Title       string    `json:"title"`
	Category    string    `json:"category"`
	Value       float64   `json:"value"`
	Unit        string    `json:"unit"`
	Description string    `json:"description"`
	Series      []float64 `json:"series"`
}

type TimeSeries struct {
	Name   string            `json:"name"`
	Unit   string            `json:"unit,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
	Points []Point           `json:"points"`
}

type Point struct {
	Timestamp int64   `json:"timestamp"`
	Value     float64 `json:"value"`
}

type provider struct {
	config Config
	client *http.Client
}

func NewProvider(config Config) Provider {
	normalized := config.Normalize()
	timeout, err := time.ParseDuration(normalized.Timeout)
	if err != nil || timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &provider{
		config: normalized,
		client: &http.Client{Timeout: timeout},
	}
}

func (p *provider) PlatformOverview(ctx context.Context, query PlatformOverviewQuery) (*PlatformOverview, error) {
	overview := &PlatformOverview{
		Provider: ProviderStatus{
			Provider:   p.config.Provider,
			Endpoint:   p.config.Endpoint,
			Database:   p.config.Database,
			Configured: p.config.Configured(),
		},
		Stats:      []StatValue{},
		Series:     []TimeSeries{},
		Components: []PlatformComponentMetric{},
		Resources:  []PlatformResourceMetric{},
		Runtime:    []PlatformRuntimeMetric{},
	}
	if !p.config.Configured() {
		return overview, nil
	}
	if p.config.Provider != DefaultProvider {
		return overview, fmt.Errorf("unsupported observability provider %q", p.config.Provider)
	}

	start, end := queryRange(query)
	countRows, err := p.queryRequestCounts(ctx, query, start, end)
	if err != nil {
		if isMissingTable(err) {
			return overview, nil
		}
		return nil, err
	}
	durationRows, err := p.queryRequestDurationBuckets(ctx, query, start, end)
	if err != nil && !isMissingTable(err) {
		return nil, err
	}
	seriesRows, err := p.queryRequestSeries(ctx, query, start, end)
	if err != nil && !isMissingTable(err) {
		return nil, err
	}
	runtimeRows, err := p.queryGoRuntimeMetrics(ctx, start, end)
	if err != nil {
		return nil, err
	}
	overview.Components = buildComponents(countRows, durationRows, seriesRows, end.Sub(start))
	overview.Stats = buildStats(overview.Components)
	overview.Series = buildSeries(countRows)
	overview.Resources = []PlatformResourceMetric{}
	overview.Runtime = runtimeRows
	return overview, nil
}

func (p *provider) ServiceEvents(ctx context.Context, query EventLogQuery) (*model.EventRecordLogResponse, error) {
	resp := &model.EventRecordLogResponse{
		Code: 200000,
		Info: "success",
		Data: []*model.EventRecord{},
	}
	if !p.config.Configured() {
		return resp, nil
	}
	if p.config.Provider != DefaultProvider {
		return resp, fmt.Errorf("unsupported observability provider %q", p.config.Provider)
	}
	start, end := logRange(query.StartTime, query.EndTime)
	rows, err := p.queryLogRows(ctx, start, end, logFetchLimit(query.Limit))
	if err != nil {
		if isMissingTable(err) {
			return resp, nil
		}
		return nil, err
	}
	limit := responseLimit(query.Limit)
	for _, row := range rows {
		if row.LogAttrs["pole.event.kind"] != "service" {
			continue
		}
		if !matchString(query.Namespace, row.LogAttrs["pole.namespace"]) ||
			!matchString(query.Service, row.LogAttrs["pole.service.name"]) ||
			!matchString(query.Resource, row.LogAttrs["pole.resource.name"]) ||
			!matchString(query.EventType, row.LogAttrs["pole.discovery.event"]) {
			continue
		}
		record := &model.EventRecord{
			Cursor:    strconv.Itoa(len(resp.Data) + 1),
			EventType: valueOr(row.LogAttrs["pole.discovery.event"], row.LogAttrs["event.name"]),
			Namespace: row.LogAttrs["pole.namespace"],
			Service:   row.LogAttrs["pole.service.name"],
			Resource:  valueOr(row.LogAttrs["pole.resource.name"], row.Body),
			EventTime: row.Timestamp.Local().Format(time.DateTime),
			Server:    row.LogAttrs["pole.server.address"],
		}
		resp.Data = append(resp.Data, record)
		resp.Cursor = record.Cursor
		if len(resp.Data) > limit {
			resp.HasNext = true
			resp.Data = resp.Data[:limit]
			break
		}
	}
	resp.Size = uint32(len(resp.Data))
	return resp, nil
}

func (p *provider) Operations(ctx context.Context, query OperationLogQuery) (*model.OperationLogResponse, error) {
	resp := &model.OperationLogResponse{
		Code: 200000,
		Info: "success",
		Data: []*model.OperationRecord{},
	}
	if !p.config.Configured() {
		return resp, nil
	}
	if p.config.Provider != DefaultProvider {
		return resp, fmt.Errorf("unsupported observability provider %q", p.config.Provider)
	}
	start, end := logRange(query.StartTime, query.EndTime)
	rows, err := p.queryLogRows(ctx, start, end, logFetchLimit(query.Limit))
	if err != nil {
		if isMissingTable(err) {
			return resp, nil
		}
		return nil, err
	}
	limit := responseLimit(query.Limit)
	for _, row := range rows {
		if row.LogAttrs["pole.event.kind"] != "audit" {
			continue
		}
		if !matchString(query.Namespace, row.LogAttrs["pole.namespace"]) ||
			!matchString(query.ResourceType, row.LogAttrs["pole.resource.type"]) ||
			!matchString(query.ResourceName, row.LogAttrs["pole.resource.name"]) ||
			!matchString(query.OperationType, row.LogAttrs["pole.audit.operation"]) ||
			!matchString(query.Operator, row.LogAttrs["pole.audit.operator"]) {
			continue
		}
		record := &model.OperationRecord{
			Cursor:          strconv.Itoa(len(resp.Data) + 1),
			ResourceType:    row.LogAttrs["pole.resource.type"],
			ResourceName:    row.LogAttrs["pole.resource.name"],
			Namespace:       row.LogAttrs["pole.namespace"],
			OperationType:   row.LogAttrs["pole.audit.operation"],
			Operator:        row.LogAttrs["pole.audit.operator"],
			OperationDetail: types.SanitizeOperationDetail(types.Resource(row.LogAttrs["pole.resource.type"]), row.Body),
			HappenTime:      row.Timestamp.Local().Format(time.DateTime),
			Server:          row.LogAttrs["pole.server.address"],
		}
		resp.Data = append(resp.Data, record)
		resp.Cursor = record.Cursor
		if len(resp.Data) > limit {
			resp.HasNext = true
			resp.Data = resp.Data[:limit]
			break
		}
	}
	resp.Size = uint32(len(resp.Data))
	return resp, nil
}

type requestCountRow struct {
	Timestamp int64
	Value     float64
	Service   string
	API       string
	Component string
	Protocol  string
	Result    string
	Namespace string
}

type durationRow struct {
	Timestamp int64
	Value     float64
	Service   string
	API       string
	Component string
	Protocol  string
	Result    string
	Bound     string
	Namespace string
}

type runtimeMetricDef struct {
	Name        string
	Title       string
	Category    string
	Unit        string
	Description string
	Table       string
	SumTable    string
	CountTable  string
	Mode        string
	Scale       float64
}

const (
	runtimeMetricModeLatest   = "latest"
	runtimeMetricModeIncrease = "increase"
	runtimeMetricModeAverage  = "average"
)

var goRuntimeMetricDefs = []runtimeMetricDef{
	{
		Name:        "process.runtime.go.goroutines",
		Title:       "Goroutines",
		Category:    "concurrency",
		Unit:        "{goroutine}",
		Description: "当前 goroutine 数",
		Table:       "process_runtime_go_goroutines",
		Mode:        runtimeMetricModeLatest,
		Scale:       1,
	},
	{
		Name:        "process.runtime.go.mem.heap_alloc",
		Title:       "Heap Alloc",
		Category:    "memory",
		Unit:        "MiB",
		Description: "Go heap 已分配内存",
		Table:       "process_runtime_go_mem_heap_alloc_bytes",
		Mode:        runtimeMetricModeLatest,
		Scale:       1024 * 1024,
	},
	{
		Name:        "process.runtime.go.mem.heap_inuse",
		Title:       "Heap Inuse",
		Category:    "memory",
		Unit:        "MiB",
		Description: "Go heap 正在使用内存",
		Table:       "process_runtime_go_mem_heap_inuse_bytes",
		Mode:        runtimeMetricModeLatest,
		Scale:       1024 * 1024,
	},
	{
		Name:        "process.runtime.go.mem.heap_sys",
		Title:       "Heap Sys",
		Category:    "memory",
		Unit:        "MiB",
		Description: "Go runtime 从系统获取的 heap 内存",
		Table:       "process_runtime_go_mem_heap_sys_bytes",
		Mode:        runtimeMetricModeLatest,
		Scale:       1024 * 1024,
	},
	{
		Name:        "process.runtime.go.gc.count",
		Title:       "GC Count",
		Category:    "gc",
		Unit:        "{collection}",
		Description: "时间窗内 GC 次数",
		Table:       "process_runtime_go_gc_count_total",
		Mode:        runtimeMetricModeIncrease,
		Scale:       1,
	},
	{
		Name:        "process.runtime.go.gc.pause",
		Title:       "GC Pause Avg",
		Category:    "gc",
		Unit:        "ms",
		Description: "时间窗内 GC pause 平均值",
		SumTable:    "process_runtime_go_gc_pause_ns_sum",
		CountTable:  "process_runtime_go_gc_pause_ns_count",
		Mode:        runtimeMetricModeAverage,
		Scale:       1000 * 1000,
	},
	{
		Name:        "process.runtime.go.mem.heap_objects",
		Title:       "Heap Objects",
		Category:    "memory",
		Unit:        "{object}",
		Description: "Go heap 存活对象数",
		Table:       "process_runtime_go_mem_heap_objects",
		Mode:        runtimeMetricModeLatest,
		Scale:       1,
	},
	{
		Name:        "go.schedule.duration",
		Title:       "Schedule Latency",
		Category:    "scheduler",
		Unit:        "ms",
		Description: "Go scheduler 调度延迟平均值",
		SumTable:    "go_schedule_duration_seconds_sum",
		CountTable:  "go_schedule_duration_seconds_count",
		Mode:        runtimeMetricModeAverage,
		Scale:       0.001,
	},
}

func (p *provider) queryRequestCounts(ctx context.Context, query PlatformOverviewQuery, start, end time.Time) ([]requestCountRow, error) {
	sql := "select max(greptime_timestamp), max(greptime_value) - min(greptime_value), service_name, pole_api_name, pole_component, pole_protocol, pole_result, k8s_namespace_name from pole_control_plane_request_count_total"
	sql += platformWhere(query, start, end)
	sql += " group by service_name, service_instance_id, pole_api_name, pole_component, pole_protocol, pole_result, k8s_namespace_name"

	result, err := p.querySQL(ctx, sql)
	if err != nil {
		return nil, err
	}
	rows := make([]requestCountRow, 0, len(result.Rows))
	for _, row := range result.Rows {
		rows = append(rows, requestCountRow{
			Timestamp: int64Value(row[0]),
			Value:     floatValue(row[1]),
			Service:   stringValue(row[2]),
			API:       stringValue(row[3]),
			Component: stringValue(row[4]),
			Protocol:  stringValue(row[5]),
			Result:    stringValue(row[6]),
			Namespace: stringValue(row[7]),
		})
	}
	return rows, nil
}

func (p *provider) queryRequestSeries(ctx context.Context, query PlatformOverviewQuery, start, end time.Time) ([]requestCountRow, error) {
	sql := "select greptime_timestamp, greptime_value, service_name, pole_api_name, pole_component, pole_protocol, pole_result, k8s_namespace_name from pole_control_plane_request_count_total"
	sql += platformWhere(query, start, end)
	sql += " order by greptime_timestamp desc limit 10000"

	result, err := p.querySQL(ctx, sql)
	if err != nil {
		return nil, err
	}
	rows := make([]requestCountRow, 0, len(result.Rows))
	for _, row := range result.Rows {
		rows = append(rows, requestCountRow{
			Timestamp: int64Value(row[0]),
			Value:     floatValue(row[1]),
			Service:   stringValue(row[2]),
			API:       stringValue(row[3]),
			Component: stringValue(row[4]),
			Protocol:  stringValue(row[5]),
			Result:    stringValue(row[6]),
			Namespace: stringValue(row[7]),
		})
	}
	return rows, nil
}

func (p *provider) queryRequestDurationBuckets(ctx context.Context, query PlatformOverviewQuery, start, end time.Time) ([]durationRow, error) {
	sql := "select max(greptime_timestamp), max(greptime_value) - min(greptime_value), service_name, pole_api_name, pole_component, pole_protocol, pole_result, le, k8s_namespace_name from pole_control_plane_request_duration_seconds_bucket"
	sql += platformWhere(query, start, end)
	sql += " group by service_name, service_instance_id, pole_api_name, pole_component, pole_protocol, pole_result, le, k8s_namespace_name"

	result, err := p.querySQL(ctx, sql)
	if err != nil {
		return nil, err
	}
	rows := make([]durationRow, 0, len(result.Rows))
	for _, row := range result.Rows {
		rows = append(rows, durationRow{
			Timestamp: int64Value(row[0]),
			Value:     floatValue(row[1]),
			Service:   stringValue(row[2]),
			API:       stringValue(row[3]),
			Component: stringValue(row[4]),
			Protocol:  stringValue(row[5]),
			Result:    stringValue(row[6]),
			Bound:     stringValue(row[7]),
			Namespace: stringValue(row[8]),
		})
	}
	return rows, nil
}

func (p *provider) queryGoRuntimeMetrics(ctx context.Context, start, end time.Time) ([]PlatformRuntimeMetric, error) {
	metrics := make([]PlatformRuntimeMetric, 0, len(goRuntimeMetricDefs))
	for _, def := range goRuntimeMetricDefs {
		points, value, err := p.queryGoRuntimeMetric(ctx, def, start, end)
		if err != nil {
			return nil, err
		}
		if len(points) == 0 {
			continue
		}
		metrics = append(metrics, PlatformRuntimeMetric{
			Name:        def.Name,
			Title:       def.Title,
			Category:    def.Category,
			Value:       round2(value),
			Unit:        def.Unit,
			Description: def.Description,
			Series:      compactPointValues(points, 12, def.Scale),
		})
	}
	return metrics, nil
}

func (p *provider) queryGoRuntimeMetric(ctx context.Context, def runtimeMetricDef, start, end time.Time) ([]Point, float64, error) {
	switch def.Mode {
	case runtimeMetricModeAverage:
		sumPoints, err := p.queryMetricPoints(ctx, def.SumTable, start, end)
		if err != nil {
			return nil, 0, err
		}
		countPoints, err := p.queryMetricPoints(ctx, def.CountTable, start, end)
		if err != nil {
			return nil, 0, err
		}
		if len(sumPoints) == 0 || len(countPoints) == 0 {
			return nil, 0, nil
		}
		sum := counterIncrease(sumPoints)
		count := counterIncrease(countPoints)
		if count <= 0 {
			return nil, 0, nil
		}
		value := sum / count / def.Scale
		return sumPoints, value, nil
	case runtimeMetricModeIncrease:
		points, err := p.queryMetricPoints(ctx, def.Table, start, end)
		if err != nil {
			return nil, 0, err
		}
		if len(points) == 0 {
			return nil, 0, nil
		}
		return points, counterIncrease(points) / def.Scale, nil
	default:
		points, err := p.queryMetricPoints(ctx, def.Table, start, end)
		if err != nil {
			return nil, 0, err
		}
		if len(points) == 0 {
			return nil, 0, nil
		}
		return points, points[len(points)-1].Value / def.Scale, nil
	}
}

func (p *provider) queryMetricPoints(ctx context.Context, table string, start, end time.Time) ([]Point, error) {
	if strings.TrimSpace(table) == "" {
		return nil, nil
	}
	sql := "select greptime_timestamp, greptime_value from " + table
	sql += metricWhere(start, end, "pole-control-plane")
	sql += " order by greptime_timestamp asc limit 1000"
	result, err := p.querySQL(ctx, sql)
	if err != nil {
		if isMissingTable(err) {
			return nil, nil
		}
		return nil, err
	}
	points := make([]Point, 0, len(result.Rows))
	for _, row := range result.Rows {
		points = append(points, Point{Timestamp: int64Value(row[0]), Value: floatValue(row[1])})
	}
	return points, nil
}

type logRow struct {
	Timestamp     time.Time
	SeverityText  string
	Body          string
	LogAttrs      map[string]string
	ResourceAttrs map[string]string
}

func (p *provider) queryLogRows(ctx context.Context, start, end time.Time, limit int) ([]logRow, error) {
	if limit <= 0 {
		limit = 1000
	}
	sql := "select timestamp, severity_text, body, log_attributes, resource_attributes from pole_events"
	sql += fmt.Sprintf(" where timestamp >= '%s' and timestamp <= '%s'", start.UTC().Format("2006-01-02 15:04:05.000"), end.UTC().Format("2006-01-02 15:04:05.000"))
	sql += fmt.Sprintf(" order by timestamp desc limit %d", limit)

	result, err := p.querySQL(ctx, sql)
	if err != nil {
		return nil, err
	}
	rows := make([]logRow, 0, len(result.Rows))
	for _, row := range result.Rows {
		if len(row) < 5 {
			continue
		}
		logAttrs := stringMapValue(row[3])
		resourceAttrs := stringMapValue(row[4])
		for key, value := range resourceAttrs {
			if _, ok := logAttrs[key]; !ok {
				logAttrs[key] = value
			}
		}
		rows = append(rows, logRow{
			Timestamp:     timeValue(row[0]),
			SeverityText:  stringValue(row[1]),
			Body:          stringValue(row[2]),
			LogAttrs:      logAttrs,
			ResourceAttrs: resourceAttrs,
		})
	}
	return rows, nil
}

type sqlResult struct {
	Rows [][]any
}

type greptimeResponse struct {
	Output []struct {
		Records *struct {
			Rows [][]any `json:"rows"`
		} `json:"records,omitempty"`
		Error string `json:"error,omitempty"`
	} `json:"output"`
	Error string `json:"error,omitempty"`
}

func (p *provider) querySQL(ctx context.Context, sql string) (*sqlResult, error) {
	endpoint, err := url.Parse(strings.TrimRight(p.config.Endpoint, "/") + "/v1/sql")
	if err != nil {
		return nil, err
	}
	query := endpoint.Query()
	query.Set("db", p.config.Database)
	endpoint.RawQuery = query.Encode()

	body := "sql=" + url.QueryEscape(sql)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewBufferString(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("greptimedb query failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}
	var decoded greptimeResponse
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return nil, err
	}
	if decoded.Error != "" {
		return nil, errors.New(decoded.Error)
	}
	if len(decoded.Output) == 0 {
		return &sqlResult{}, nil
	}
	if decoded.Output[0].Error != "" {
		return nil, errors.New(decoded.Output[0].Error)
	}
	if decoded.Output[0].Records == nil {
		return &sqlResult{}, nil
	}
	return &sqlResult{Rows: decoded.Output[0].Records.Rows}, nil
}

func platformWhere(query PlatformOverviewQuery, start, end time.Time) string {
	filters := []string{
		fmt.Sprintf("greptime_timestamp >= '%s'", start.UTC().Format("2006-01-02 15:04:05.000")),
		fmt.Sprintf("greptime_timestamp <= '%s'", end.UTC().Format("2006-01-02 15:04:05.000")),
		"service_instance_id is not null",
	}
	if query.API != "" {
		filters = append(filters, "pole_api_name = '"+escapeSQL(query.API)+"'")
	}
	if query.Category != "" {
		if components := componentsForCategory(query.Category); len(components) > 0 {
			quoted := make([]string, 0, len(components))
			for _, component := range components {
				quoted = append(quoted, "'"+escapeSQL(component)+"'")
			}
			filters = append(filters, "pole_component in ("+strings.Join(quoted, ",")+")")
		}
	}
	return " where " + strings.Join(filters, " and ")
}

func metricWhere(start, end time.Time, serviceName string) string {
	filters := []string{
		fmt.Sprintf("greptime_timestamp >= '%s'", start.UTC().Format("2006-01-02 15:04:05.000")),
		fmt.Sprintf("greptime_timestamp <= '%s'", end.UTC().Format("2006-01-02 15:04:05.000")),
	}
	if serviceName != "" {
		filters = append(filters, "service_name = '"+escapeSQL(serviceName)+"'")
	}
	return " where " + strings.Join(filters, " and ")
}

func componentsForCategory(category string) []string {
	switch category {
	case "control-plane":
		return []string{"apiserver", "system", "xds"}
	case "storage":
		return []string{"store"}
	case "observability":
		return []string{"telemetry"}
	default:
		return nil
	}
}

func queryRange(query PlatformOverviewQuery) (time.Time, time.Time) {
	end := time.Now()
	if parsed, ok := parseMillis(query.EndTime); ok {
		end = parsed
	}
	start := end.Add(-1 * time.Hour)
	if parsed, ok := parseMillis(query.StartTime); ok {
		start = parsed
	}
	if !start.Before(end) {
		start = end.Add(-1 * time.Hour)
	}
	return start, end
}

func parseMillis(raw string) (time.Time, bool) {
	if raw == "" {
		return time.Time{}, false
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.UnixMilli(value), true
}

func buildComponents(countRows []requestCountRow, durationRows []durationRow, seriesRows []requestCountRow, window time.Duration) []PlatformComponentMetric {
	type aggregate struct {
		metric PlatformComponentMetric
		total  float64
		errors float64
		series []Point
	}
	type countKey struct {
		component string
		result    string
	}
	aggregates := make(map[string]*aggregate)
	countsByResult := make(map[countKey][]Point)
	for _, row := range countRows {
		key := componentKey(row.Service, row.API, row.Component, row.Protocol)
		item := aggregates[key]
		if item == nil {
			item = &aggregate{
				metric: PlatformComponentMetric{
					ID:        key,
					Category:  categoryFromComponent(row.Component),
					Component: valueOr(row.Service, "pole-control-plane"),
					Role:      roleFromComponent(row.Component),
					Namespace: valueOr(row.Namespace, "unknown"),
					Pod:       "",
					API:       valueOr(row.API, "unknown"),
					Status:    "healthy",
					Protocol:  valueOr(row.Protocol, "unknown"),
				},
			}
			aggregates[key] = item
		}
		resultKey := countKey{component: key, result: valueOr(row.Result, "unknown")}
		countsByResult[resultKey] = append(countsByResult[resultKey], Point{Timestamp: row.Timestamp, Value: row.Value})
	}
	for _, row := range seriesRows {
		if item := aggregates[componentKey(row.Service, row.API, row.Component, row.Protocol)]; item != nil {
			item.series = append(item.series, Point{Timestamp: row.Timestamp, Value: row.Value})
		}
	}

	for resultKey, points := range countsByResult {
		item := aggregates[resultKey.component]
		if item == nil {
			continue
		}
		increase := counterIncrease(points)
		item.total += increase
		if resultKey.result == "failure" {
			item.errors += increase
		}
	}

	quantilesByKey := aggregateDurationQuantiles(durationRows)
	windowSeconds := window.Seconds()
	if windowSeconds <= 0 {
		windowSeconds = 1
	}
	components := make([]PlatformComponentMetric, 0, len(aggregates))
	for key, item := range aggregates {
		metric := item.metric
		metric.RequestCount = round2(item.total)
		metric.QPS = round4(item.total / windowSeconds)
		if item.total > 0 {
			metric.ErrorRate = round2(item.errors / item.total * 100)
		}
		if quantiles, ok := quantilesByKey[key]; ok {
			metric.P95 = round2(quantiles.P95 * float64(time.Second.Milliseconds()))
			metric.P99 = round2(quantiles.P99 * float64(time.Second.Milliseconds()))
		}
		if metric.ErrorRate > 1 {
			metric.Status = "warning"
		}
		metric.Series = compactCounterSeries(item.series, 12)
		components = append(components, metric)
	}
	return components
}

type durationQuantiles struct {
	P95 float64
	P99 float64
}

type histogramBucket struct {
	upperBound float64
	count      float64
	infinite   bool
}

func aggregateDurationQuantiles(rows []durationRow) map[string]durationQuantiles {
	type bucketKey struct {
		component string
		bound     string
	}
	points := make(map[bucketKey][]Point)
	for _, row := range rows {
		key := componentKey(row.Service, row.API, row.Component, row.Protocol)
		pointKey := bucketKey{component: key, bound: row.Bound}
		points[pointKey] = append(points[pointKey], Point{Timestamp: row.Timestamp, Value: row.Value})
	}
	bucketsByComponent := make(map[string][]histogramBucket)
	for key, values := range points {
		bound := strings.TrimSpace(strings.ToLower(key.bound))
		infinite := bound == "inf" || bound == "+inf"
		upperBound, err := strconv.ParseFloat(bound, 64)
		if err != nil && !infinite {
			continue
		}
		bucketsByComponent[key.component] = append(bucketsByComponent[key.component], histogramBucket{
			upperBound: upperBound,
			count:      counterIncrease(values),
			infinite:   infinite,
		})
	}
	result := make(map[string]durationQuantiles, len(bucketsByComponent))
	for component, buckets := range bucketsByComponent {
		sort.SliceStable(buckets, func(i, j int) bool {
			if buckets[i].infinite != buckets[j].infinite {
				return !buckets[i].infinite
			}
			return buckets[i].upperBound < buckets[j].upperBound
		})
		result[component] = durationQuantiles{
			P95: histogramQuantile(buckets, 0.95),
			P99: histogramQuantile(buckets, 0.99),
		}
	}
	return result
}

func histogramQuantile(buckets []histogramBucket, quantile float64) float64 {
	if len(buckets) == 0 {
		return 0
	}
	total := buckets[len(buckets)-1].count
	if total <= 0 {
		return 0
	}
	target := total * quantile
	lastFinite := 0.0
	for _, bucket := range buckets {
		if bucket.infinite {
			return lastFinite
		}
		lastFinite = bucket.upperBound
		if bucket.count >= target {
			return bucket.upperBound
		}
	}
	return lastFinite
}

func buildStats(components []PlatformComponentMetric) []StatValue {
	var total, errors, latency float64
	for _, component := range components {
		total += component.RequestCount
		errors += component.RequestCount * component.ErrorRate / 100
		latency += component.P95
	}
	errorRate := 0.0
	if total > 0 {
		errorRate = errors / total * 100
	}
	avgLatency := 0.0
	if len(components) > 0 {
		avgLatency = latency / float64(len(components))
	}
	return []StatValue{
		{Name: "component_count", Value: float64(len(components)), Unit: "{component}"},
		{Name: "request_count", Value: round2(total), Unit: "{request}"},
		{Name: "error_rate", Value: round2(errorRate), Unit: "%"},
		{Name: "avg_latency", Value: round2(avgLatency), Unit: "ms"},
	}
}

func buildSeries(rows []requestCountRow) []TimeSeries {
	points := make([]Point, 0, len(rows))
	for i := len(rows) - 1; i >= 0; i-- {
		points = append(points, Point{Timestamp: rows[i].Timestamp, Value: rows[i].Value})
	}
	return []TimeSeries{{
		Name:   "pole.control_plane.request.count",
		Unit:   "{request}",
		Labels: map[string]string{"pole.component": "apiserver"},
		Points: points,
	}}
}

func compactCounterSeries(points []Point, size int) []float64 {
	if len(points) == 0 {
		return []float64{}
	}
	increases := counterIncreases(points)
	if len(increases) == 0 || sumFloat(increases) == 0 {
		increases = counterValues(points)
	}
	values := make([]float64, 0, size)
	start := len(increases) - size
	if start < 0 {
		start = 0
	}
	for _, value := range increases[start:] {
		values = append(values, value)
	}
	for len(values) < size {
		values = append([]float64{0}, values...)
	}
	return values
}

func compactPointValues(points []Point, size int, scale float64) []float64 {
	if len(points) == 0 {
		return []float64{}
	}
	if scale <= 0 {
		scale = 1
	}
	values := make([]float64, 0, size)
	start := len(points) - size
	if start < 0 {
		start = 0
	}
	for _, point := range points[start:] {
		values = append(values, round2(point.Value/scale))
	}
	for len(values) < size {
		values = append([]float64{0}, values...)
	}
	return values
}

func counterIncrease(points []Point) float64 {
	increases := counterIncreases(points)
	var total float64
	for _, value := range increases {
		total += value
	}
	if total == 0 {
		return maxPointValue(points)
	}
	return total
}

func counterIncreases(points []Point) []float64 {
	if len(points) == 0 {
		return []float64{}
	}
	sorted := append([]Point(nil), points...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Timestamp < sorted[j].Timestamp
	})
	if len(sorted) == 1 {
		return []float64{sorted[0].Value}
	}
	values := make([]float64, 0, len(sorted)-1)
	for i := 1; i < len(sorted); i++ {
		delta := sorted[i].Value - sorted[i-1].Value
		if delta < 0 {
			delta = sorted[i].Value
		}
		values = append(values, delta)
	}
	return values
}

func counterValues(points []Point) []float64 {
	sorted := append([]Point(nil), points...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Timestamp < sorted[j].Timestamp
	})
	values := make([]float64, 0, len(sorted))
	for _, point := range sorted {
		values = append(values, point.Value)
	}
	return values
}

func maxPointValue(points []Point) float64 {
	var max float64
	for _, point := range points {
		if point.Value > max {
			max = point.Value
		}
	}
	return max
}

func sumFloat(values []float64) float64 {
	var total float64
	for _, value := range values {
		total += value
	}
	return total
}

func componentKey(service, api, component, protocol string) string {
	return strings.Join([]string{valueOr(service, "unknown"), valueOr(api, "unknown"), valueOr(component, "unknown"), valueOr(protocol, "unknown")}, "|")
}

func categoryFromComponent(component string) string {
	switch component {
	case "store":
		return "storage"
	case "telemetry":
		return "observability"
	default:
		return "control-plane"
	}
}

func roleFromComponent(component string) string {
	switch component {
	case "store":
		return "元数据存储"
	case "xds":
		return "xDS 资源构建"
	case "system":
		return "内部任务"
	default:
		return "控制面 API"
	}
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func stringMapValue(value any) map[string]string {
	if value == nil {
		return map[string]string{}
	}
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]string, len(typed))
		for key, val := range typed {
			out[key] = stringValue(val)
		}
		return out
	case map[string]string:
		out := make(map[string]string, len(typed))
		for key, val := range typed {
			out[key] = val
		}
		return out
	case string:
		raw := strings.TrimSpace(typed)
		if raw == "" {
			return map[string]string{}
		}
		var decoded map[string]any
		if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
			return map[string]string{}
		}
		return stringMapValue(decoded)
	default:
		raw := strings.TrimSpace(fmt.Sprint(value))
		if raw == "" {
			return map[string]string{}
		}
		var decoded map[string]any
		if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
			return map[string]string{}
		}
		return stringMapValue(decoded)
	}
}

func timeValue(value any) time.Time {
	switch typed := value.(type) {
	case time.Time:
		return typed
	case float64:
		return unixAuto(int64(typed))
	case int64:
		return unixAuto(typed)
	case int:
		return unixAuto(int64(typed))
	case json.Number:
		parsed, _ := typed.Int64()
		return unixAuto(parsed)
	default:
		raw := strings.TrimSpace(fmt.Sprint(value))
		if raw == "" {
			return time.Time{}
		}
		for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05.999", "2006-01-02 15:04:05"} {
			if parsed, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
				return parsed
			}
		}
		if numeric, err := strconv.ParseInt(raw, 10, 64); err == nil {
			return unixAuto(numeric)
		}
		return time.Time{}
	}
}

func unixAuto(value int64) time.Time {
	switch {
	case value > 1e17:
		return time.Unix(0, value)
	case value > 1e14:
		return time.UnixMicro(value)
	case value > 1e11:
		return time.UnixMilli(value)
	default:
		return time.Unix(value, 0)
	}
}

func int64Value(value any) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case int64:
		return typed
	case int:
		return int64(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return parsed
	default:
		parsed, _ := strconv.ParseInt(fmt.Sprint(value), 10, 64)
		return parsed
	}
}

func floatValue(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case int64:
		return float64(typed)
	case int:
		return float64(typed)
	case json.Number:
		parsed, _ := typed.Float64()
		return parsed
	default:
		parsed, _ := strconv.ParseFloat(fmt.Sprint(value), 64)
		return parsed
	}
}

func round2(value float64) float64 {
	parsed, _ := strconv.ParseFloat(fmt.Sprintf("%.2f", value), 64)
	return parsed
}

func round4(value float64) float64 {
	parsed, _ := strconv.ParseFloat(fmt.Sprintf("%.4f", value), 64)
	return parsed
}

func escapeSQL(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}

func valueOr(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func logRange(startRaw, endRaw string) (time.Time, time.Time) {
	end := time.Now()
	if parsed, ok := parseFlexibleTime(endRaw); ok {
		end = parsed
	}
	start := end.Add(-24 * time.Hour)
	if parsed, ok := parseFlexibleTime(startRaw); ok {
		start = parsed
	}
	if !start.Before(end) {
		start = end.Add(-24 * time.Hour)
	}
	return start, end
}

func parseFlexibleTime(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	if parsed, ok := parseMillis(raw); ok {
		return parsed, true
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, time.RubyDate, time.UnixDate, time.ANSIC, time.DateTime, "2006-01-02 15:04:05"} {
		if parsed, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func responseLimit(raw string) int {
	limit, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || limit <= 0 {
		return 10
	}
	if limit > 200 {
		return 200
	}
	return limit
}

func logFetchLimit(raw string) int {
	limit := responseLimit(raw)
	fetchLimit := limit*20 + 100
	if fetchLimit > 1000 {
		return 1000
	}
	return fetchLimit
}

func matchString(expect, actual string) bool {
	expect = strings.TrimSpace(expect)
	if expect == "" {
		return true
	}
	return strings.Contains(strings.ToLower(actual), strings.ToLower(expect))
}

func isMissingTable(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "table not found") ||
		strings.Contains(message, "does not exist") ||
		strings.Contains(message, "unknown table") ||
		strings.Contains(message, "table doesn't exist")
}
