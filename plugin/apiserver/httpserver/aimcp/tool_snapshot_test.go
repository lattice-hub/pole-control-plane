package aimcp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/emicklei/go-restful/v3"
	"github.com/golang/mock/gomock"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/require"

	specai "github.com/pole-io/specification/source/go/api/v1/ai"

	"github.com/pole-io/pole-server/pkg/selfmanager"
	storemock "github.com/pole-io/pole-server/plugin/store/mock"
)

func TestSnapshotToolsMatchesPoleMCPToolCatalog(t *testing.T) {
	httpServer := &HTTPServer{mcpSvr: server.NewMCPServer("pole-test", "v1")}
	httpServer.addMcpTools()

	tools, err := httpServer.SnapshotTools(context.Background())
	require.NoError(t, err)
	require.Len(t, tools, 10)
	names := make(map[string]bool, len(tools))
	for _, tool := range tools {
		names[tool.GetName()] = true
		require.True(t, json.Valid([]byte(tool.GetInputSchema())), tool.GetName())
	}
	for _, name := range []string{
		"list_namespaces", "create_namespaces", "update_namespaces", "delete_namespaces",
		"list_mcp_servers", "create_mcp_servers", "update_mcp_servers", "delete_mcp_servers",
		"list_mcp_server_tools", "get_config_state",
	} {
		require.True(t, names[name], name)
	}
}

func TestSelfCapabilityProbeEndpointRequiresMachineIdentityAndLimitsBody(t *testing.T) {
	controller := gomock.NewController(t)
	storage := storemock.NewMockStore(controller)
	storage.EXPECT().GetMCPServerByName("pole-control-plane", "pole-system").Return(&specai.MCPServer{
		Name: "pole-control-plane", Namespace: "pole-system", Reference: "pole-self-manager",
		BackendType: "address", BackendAddress: "http://pole-server:8090/ai/mcp/v1/sse",
	}, nil)
	masterKey := base64.StdEncoding.EncodeToString(make([]byte, 32))
	httpServer := &HTTPServer{
		storage:      storage,
		mcpSvr:       server.NewMCPServer("pole-test", "v1"),
		selfProbeKey: masterKey,
	}
	container := restful.NewContainer()
	container.Add(httpServer.GetMCPAccessServer(nil))

	target := "/ai/mcp/v1/self-capabilities/probe"
	anonymous := httptest.NewRecorder()
	anonymousRequest := httptest.NewRequest(http.MethodPost, target,
		strings.NewReader(`{"allowlist":[]}`))
	anonymousRequest.Header.Set("Content-Type", "application/json")
	container.ServeHTTP(anonymous, anonymousRequest)
	require.Equal(t, http.StatusUnauthorized, anonymous.Code)

	body := []byte(`{"allowlist":["list_namespaces"]}`)
	valid := httptest.NewRecorder()
	validRequest := httptest.NewRequest(http.MethodPost, target, bytes.NewReader(body))
	signProbeRequest(t, masterKey, validRequest, body)
	container.ServeHTTP(valid, validRequest)
	require.Equal(t, http.StatusNoContent, valid.Code, valid.Body.String())

	oversizedBody := bytes.Repeat([]byte("x"), (64<<10)+1)
	oversized := httptest.NewRecorder()
	oversizedRequest := httptest.NewRequest(http.MethodPost, target, bytes.NewReader(oversizedBody))
	signProbeRequest(t, masterKey, oversizedRequest, oversizedBody)
	container.ServeHTTP(oversized, oversizedRequest)
	require.Equal(t, http.StatusRequestEntityTooLarge, oversized.Code, oversized.Body.String())
}

func signProbeRequest(t *testing.T, masterKey string, request *http.Request, body []byte) {
	t.Helper()
	timestamp, signature, err := selfmanager.SignCapabilityProbe(masterKey,
		request.Method, request.URL.Path, body, time.Now())
	require.NoError(t, err)
	request.Header.Set(selfmanager.ProbeTimestampHeader, timestamp)
	request.Header.Set(selfmanager.ProbeSignatureHeader, signature)
	request.Header.Set("Content-Type", "application/json")
}

func TestProbeSelfCapabilitiesChecksReservedRegistryProjectionAndAllowlist(t *testing.T) {
	controller := gomock.NewController(t)
	storage := storemock.NewMockStore(controller)
	storage.EXPECT().GetMCPServerByName("pole-control-plane", "pole-system").Return(&specai.MCPServer{
		Name: "pole-control-plane", Namespace: "pole-system", Reference: "pole-self-manager",
		BackendType: "address", BackendAddress: "http://pole-server:8090/ai/mcp/v1/sse",
	}, nil).Times(2)
	httpServer := &HTTPServer{
		storage: storage,
		mcpSvr:  server.NewMCPServer("pole-test", "v1"),
	}
	httpServer.addMcpTools()

	require.NoError(t, httpServer.ProbeSelfCapabilities(context.Background(),
		[]string{"list_namespaces", "list_mcp_servers"}))
	require.ErrorContains(t, httpServer.ProbeSelfCapabilities(context.Background(),
		[]string{"missing_tool"}), `Pole MCP tool "missing_tool" is not available`)
}
