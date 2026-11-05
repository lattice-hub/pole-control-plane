package otel

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	globalotel "go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestStartSpanCreatesInternalChildAndRecordsError(t *testing.T) {
	previousProvider := globalotel.GetTracerProvider()
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	globalotel.SetTracerProvider(provider)
	t.Cleanup(func() {
		globalotel.SetTracerProvider(previousProvider)
	})

	ctx, parent := provider.Tracer("test").Start(context.Background(), "parent")
	_, end := StartSpan(ctx, "service.namespace.get", "service")
	end(errors.New("boom"))
	end(nil)
	parent.End()

	spans := recorder.Ended()
	require.Len(t, spans, 2)
	child := spans[0]
	assert.Equal(t, "service.namespace.get", child.Name())
	assert.Equal(t, parent.SpanContext().SpanID(), child.Parent().SpanID())
	assert.Equal(t, "Error", child.Status().Code.String())
	require.Len(t, child.Events(), 1)
	assert.Equal(t, "exception", child.Events()[0].Name)

	attrs := child.Attributes()
	require.Len(t, attrs, 1)
	assert.Equal(t, "pole.span.layer", string(attrs[0].Key))
	assert.Equal(t, "service", attrs[0].Value.AsString())
}

func TestStartSpanOnceDeduplicatesWithinRequest(t *testing.T) {
	previousProvider := globalotel.GetTracerProvider()
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	globalotel.SetTracerProvider(provider)
	t.Cleanup(func() {
		globalotel.SetTracerProvider(previousProvider)
	})

	ctx := WithRequestSpanState(context.Background())
	_, endFirst := StartSpanOnce(ctx, "auth.console.read", "auth.console.read", "auth")
	_, endSecond := StartSpanOnce(ctx, "auth.console.read", "auth.console.read", "auth")
	endFirst(nil)
	endSecond(nil)

	spans := recorder.Ended()
	require.Len(t, spans, 1)
	assert.Equal(t, "auth.console.read", spans[0].Name())
}
