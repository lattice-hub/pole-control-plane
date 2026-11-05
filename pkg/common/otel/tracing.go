package otel

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "github.com/pole-io/pole-server"

type requestSpanStateKey struct{}

type requestSpanState struct {
	mu   sync.Mutex
	seen map[string]struct{}
}

func WithRequestSpanState(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Value(requestSpanStateKey{}).(*requestSpanState); ok {
		return ctx
	}
	return context.WithValue(ctx, requestSpanStateKey{}, &requestSpanState{seen: make(map[string]struct{})})
}

func StartSpanOnce(ctx context.Context, key, name, layer string,
	attrs ...attribute.KeyValue) (context.Context, func(error)) {
	state, ok := ctx.Value(requestSpanStateKey{}).(*requestSpanState)
	if ok {
		state.mu.Lock()
		_, seen := state.seen[key]
		if !seen {
			state.seen[key] = struct{}{}
		}
		state.mu.Unlock()
		if seen {
			return ctx, func(error) {}
		}
	}
	return StartSpan(ctx, name, layer, attrs...)
}

func StartSpan(ctx context.Context, name, layer string, attrs ...attribute.KeyValue) (context.Context, func(error)) {
	if ctx == nil {
		ctx = context.Background()
	}
	attrs = append([]attribute.KeyValue{attribute.String("pole.span.layer", layer)}, attrs...)
	ctx, span := otel.Tracer(tracerName).Start(ctx, name,
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(attrs...),
	)
	var once sync.Once
	return ctx, func(err error) {
		once.Do(func() {
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			}
			span.End()
		})
	}
}
