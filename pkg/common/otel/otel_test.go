package otel

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResourceAttributesIncludeServiceInstanceID(t *testing.T) {
	hostname, err := os.Hostname()
	require.NoError(t, err)

	attrs := resourceAttributes(&Config{ServiceName: "pole-control-plane"})
	values := make(map[string]string, len(attrs))
	for _, attr := range attrs {
		values[string(attr.Key)] = attr.Value.AsString()
	}

	require.Equal(t, hostname, values["service.instance.id"])
}
