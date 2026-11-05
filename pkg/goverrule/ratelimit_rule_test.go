package goverrule

import (
	"testing"

	"github.com/stretchr/testify/require"

	apitraffic "github.com/pole-io/specification/source/go/api/v1/traffic_manage"
)

func TestRateLimitConversionPreservesServiceScope(t *testing.T) {
	request := &apitraffic.RateLimit{
		Name:      "llm-streaming-tpm",
		Namespace: "Polaris",
		Callee: &apitraffic.DestinationService{
			Namespace: "runtime",
			Service:   "pole.controller.orbstack",
		},
		Type: apitraffic.RateLimit_LOCAL,
	}

	stored, err := api2RateLimit(request)
	require.NoError(t, err)

	view, err := rateLimit2Console(stored)
	require.NoError(t, err)
	require.Equal(t, request.GetNamespace(), view.GetNamespace())
	require.Equal(t, request.GetCallee(), view.GetCallee())
}
