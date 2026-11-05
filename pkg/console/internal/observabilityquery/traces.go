package observabilityquery

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

const traceTable = "pole_traces"

type TraceQuery struct {
	StartTime     string
	EndTime       string
	Service       string
	API           string
	Limit         string
	ErrorOnly     string
	MinDurationMS string
}

type TraceList struct {
	Data []TraceSummary `json:"data"`
	Size int            `json:"size"`
}

type TraceSummary struct {
	TraceID     string  `json:"trace_id"`
	RootSpan    string  `json:"root_span"`
	ServiceName string  `json:"service_name"`
	StartTime   int64   `json:"start_time"`
	DurationMS  float64 `json:"duration_ms"`
	Status      string  `json:"status"`
	SpanCount   int     `json:"span_count"`
}

type TraceDetail struct {
	Summary TraceSummary `json:"summary"`
	Spans   []TraceSpan  `json:"spans"`
}

type TraceSpan struct {
	TraceID       string  `json:"trace_id"`
	SpanID        string  `json:"span_id"`
	ParentSpanID  string  `json:"parent_span_id"`
	ServiceName   string  `json:"service_name"`
	Name          string  `json:"name"`
	Kind          string  `json:"kind"`
	Status        string  `json:"status"`
	StatusMessage string  `json:"status_message,omitempty"`
	StartTime     int64   `json:"start_time"`
	EndTime       int64   `json:"end_time"`
	StartTimeUS   int64   `json:"start_time_us"`
	EndTimeUS     int64   `json:"end_time_us"`
	DurationMS    float64 `json:"duration_ms"`
}

func (p *provider) PlatformTraces(ctx context.Context, query TraceQuery) (*TraceList, error) {
	result := &TraceList{Data: []TraceSummary{}}
	if !p.config.Configured() {
		return result, nil
	}
	start, end := traceRange(query.StartTime, query.EndTime)
	spans, err := p.queryTraceSpans(ctx, start, end, query.Service, "", traceFetchLimit(query.Limit))
	if err != nil {
		if isMissingTable(err) {
			return result, nil
		}
		return nil, err
	}
	api := normalizeTraceAPI(query.API)
	if api != "" {
		filtered := spans[:0]
		matchedTraceIDs := make(map[string]struct{})
		for _, span := range spans {
			if strings.Contains(normalizeTraceAPI(span.Name), api) {
				matchedTraceIDs[span.TraceID] = struct{}{}
			}
		}
		for _, span := range spans {
			if _, ok := matchedTraceIDs[span.TraceID]; ok {
				filtered = append(filtered, span)
			}
		}
		spans = filtered
	}
	summaries := summarizeTraces(spans)
	minDuration := parsePositiveFloat(query.MinDurationMS)
	errorOnly, _ := strconv.ParseBool(query.ErrorOnly)
	limit := responseLimit(query.Limit)
	for _, summary := range summaries {
		if errorOnly && summary.Status != "error" {
			continue
		}
		if summary.DurationMS < minDuration {
			continue
		}
		result.Data = append(result.Data, summary)
		if len(result.Data) >= limit {
			break
		}
	}
	result.Size = len(result.Data)
	return result, nil
}

func (p *provider) TraceDetail(ctx context.Context, traceID string) (*TraceDetail, error) {
	traceID = strings.TrimSpace(traceID)
	if traceID == "" {
		return nil, fmt.Errorf("trace_id is required")
	}
	spans, err := p.queryTraceSpans(ctx, time.Time{}, time.Time{}, "", traceID, 2000)
	if err != nil {
		if isMissingTable(err) {
			return &TraceDetail{Spans: []TraceSpan{}}, nil
		}
		return nil, err
	}
	summaries := summarizeTraces(spans)
	detail := &TraceDetail{Spans: spans}
	if len(summaries) > 0 {
		detail.Summary = summaries[0]
	}
	return detail, nil
}

func (p *provider) queryTraceSpans(ctx context.Context, start, end time.Time, service, traceID string, limit int) ([]TraceSpan, error) {
	filters := make([]string, 0, 4)
	if !start.IsZero() {
		filters = append(filters, fmt.Sprintf("timestamp >= '%s'", start.UTC().Format("2006-01-02 15:04:05.000")))
	}
	if !end.IsZero() {
		filters = append(filters, fmt.Sprintf("timestamp <= '%s'", end.UTC().Format("2006-01-02 15:04:05.000")))
	}
	if strings.TrimSpace(service) != "" {
		filters = append(filters, "service_name = '"+escapeSQL(service)+"'")
	}
	if traceID != "" {
		filters = append(filters, "trace_id = '"+escapeSQL(traceID)+"'")
	}
	sql := "select timestamp, timestamp_end, duration_nano, parent_span_id, trace_id, span_id, span_kind, span_name, span_status_code, span_status_message, service_name from " + traceTable
	if len(filters) > 0 {
		sql += " where " + strings.Join(filters, " and ")
	}
	sql += fmt.Sprintf(" order by timestamp desc limit %d", limit)
	result, err := p.querySQL(ctx, sql)
	if err != nil {
		return nil, err
	}
	spans := make([]TraceSpan, 0, len(result.Rows))
	for _, row := range result.Rows {
		if len(row) < 11 {
			continue
		}
		startTime := timeValue(row[0])
		endTime := timeValue(row[1])
		durationMS := floatValue(row[2]) / float64(time.Millisecond)
		if durationMS <= 0 && !startTime.IsZero() && !endTime.IsZero() {
			durationMS = float64(endTime.Sub(startTime)) / float64(time.Millisecond)
		}
		spans = append(spans, TraceSpan{
			StartTime:     startTime.UnixMilli(),
			EndTime:       endTime.UnixMilli(),
			StartTimeUS:   startTime.UnixMicro(),
			EndTimeUS:     endTime.UnixMicro(),
			DurationMS:    round2(durationMS),
			ParentSpanID:  stringValue(row[3]),
			TraceID:       stringValue(row[4]),
			SpanID:        stringValue(row[5]),
			Kind:          stringValue(row[6]),
			Name:          stringValue(row[7]),
			Status:        traceStatus(stringValue(row[8])),
			StatusMessage: stringValue(row[9]),
			ServiceName:   stringValue(row[10]),
		})
	}
	sort.SliceStable(spans, func(i, j int) bool { return spans[i].StartTimeUS < spans[j].StartTimeUS })
	return spans, nil
}

func summarizeTraces(spans []TraceSpan) []TraceSummary {
	grouped := make(map[string][]TraceSpan)
	for _, span := range spans {
		if span.TraceID != "" {
			grouped[span.TraceID] = append(grouped[span.TraceID], span)
		}
	}
	summaries := make([]TraceSummary, 0, len(grouped))
	for traceID, traceSpans := range grouped {
		root := traceSpans[0]
		startTime := root.StartTime
		endTime := root.EndTime
		startTimeUS := root.StartTimeUS
		endTimeUS := root.EndTimeUS
		durationMS := root.DurationMS
		status := "ok"
		for _, span := range traceSpans {
			if span.ParentSpanID == "" {
				root = span
			}
			if span.StartTime < startTime {
				startTime = span.StartTime
			}
			if span.EndTime > endTime {
				endTime = span.EndTime
			}
			if span.StartTimeUS > 0 && (startTimeUS == 0 || span.StartTimeUS < startTimeUS) {
				startTimeUS = span.StartTimeUS
			}
			if span.EndTimeUS > endTimeUS {
				endTimeUS = span.EndTimeUS
			}
			spanEndMS := float64(span.StartTimeUS-startTimeUS)/1000 + span.DurationMS
			if spanEndMS > durationMS {
				durationMS = spanEndMS
			}
			if span.Status == "error" {
				status = "error"
			}
		}
		wallDuration := float64(endTime - startTime)
		if startTimeUS > 0 && endTimeUS >= startTimeUS {
			wallDuration = float64(endTimeUS-startTimeUS) / 1000
		}
		if wallDuration > durationMS {
			durationMS = wallDuration
		}
		summaries = append(summaries, TraceSummary{
			TraceID:     traceID,
			RootSpan:    root.Name,
			ServiceName: root.ServiceName,
			StartTime:   startTime,
			DurationMS:  round2(durationMS),
			Status:      status,
			SpanCount:   len(traceSpans),
		})
	}
	sort.SliceStable(summaries, func(i, j int) bool { return summaries[i].StartTime > summaries[j].StartTime })
	return summaries
}

func traceRange(startRaw, endRaw string) (time.Time, time.Time) {
	end := time.Now()
	if parsed, ok := parseFlexibleTime(endRaw); ok {
		end = parsed
	}
	start := end.Add(-1 * time.Hour)
	if parsed, ok := parseFlexibleTime(startRaw); ok {
		start = parsed
	}
	if !start.Before(end) {
		start = end.Add(-1 * time.Hour)
	}
	return start, end
}

func traceFetchLimit(raw string) int {
	limit := responseLimit(raw) * 100
	if limit < 1000 {
		return 1000
	}
	if limit > 10000 {
		return 10000
	}
	return limit
}

func traceStatus(value string) string {
	if strings.Contains(strings.ToUpper(value), "ERROR") {
		return "error"
	}
	return "ok"
}

func normalizeTraceAPI(value string) string {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(value)), "/")
}

func parsePositiveFloat(raw string) float64 {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || value < 0 {
		return 0
	}
	return value
}
