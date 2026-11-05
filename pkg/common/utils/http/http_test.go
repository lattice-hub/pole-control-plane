package httputils

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/emicklei/go-restful/v3"
	"github.com/stretchr/testify/assert"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func TestParseHeaderContextPreservesRequestSpan(t *testing.T) {
	provider := sdktrace.NewTracerProvider()
	ctx, span := provider.Tracer("test").Start(context.Background(), "request")
	defer span.End()

	httpRequest := httptest.NewRequest("GET", "/naming/v1/namespaces", nil).WithContext(ctx)
	req := restful.NewRequest(httpRequest)
	rsp := restful.NewResponse(httptest.NewRecorder())

	parsed := ParseHeaderContext(req, rsp)
	assert.Equal(t, trace.SpanFromContext(ctx).SpanContext(), trace.SpanFromContext(parsed).SpanContext())
}
