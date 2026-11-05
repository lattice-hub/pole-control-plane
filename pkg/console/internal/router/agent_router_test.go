package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/anypb"

	configv1 "github.com/pole-io/specification/source/go/api/v1/config"
	apimodel "github.com/pole-io/specification/source/go/api/v1/model"

	bootstrap "github.com/pole-io/pole-server/pkg/console/config"
)

func TestAgentRouterPreviewsAndStagesConfigDraftWithoutPublishing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const (
		userID = "alice"
		token  = "pole-token"
		secret = "agent-test-secret"
	)
	var mu sync.Mutex
	content := "timeout: 3s\n"
	putCalls := 0
	var upstreamPaths []string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		upstreamPaths = append(upstreamPaths, r.Method+" "+r.URL.Path)
		assert.Equal(t, token, r.Header.Get("Authorization"))
		assert.Empty(t, r.Header.Get("X-Pole-User"))
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-Id", "upstream-request")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/config/v1/state":
			writeAgentConfigState(t, w, "application.yaml", content, uint64(putCalls)+1)
		case r.Method == http.MethodPost && r.URL.Path == "/config/v1/draft":
			payload, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			request := &configv1.SaveDraftRequest{}
			require.NoError(t, protojson.Unmarshal(payload, request))
			require.Equal(t, uint64(putCalls)+1, request.ExpectedDraftRevision)
			content = string(request.Definition.Content)
			putCalls++
			writeAgentConfigState(t, w, "application.yaml", content, uint64(putCalls)+1)
		default:
			t.Fatalf("unexpected upstream request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer backend.Close()

	router := newAgentTestRouter(t, backend.URL, secret)
	console := httptest.NewServer(router)
	defer console.Close()

	prepareBody := `{"namespace":"default","group":"orders","name":"application.yaml","desiredContent":"timeout: 5s\n"}`
	prepareReq, err := http.NewRequest(http.MethodPost, console.URL+"/ai/agent/v1/proposals/config-file", strings.NewReader(prepareBody))
	require.NoError(t, err)
	prepareReq.Header.Set("Content-Type", "application/json")
	prepareReq.Header.Set("X-Pole-User", userID)
	prepareReq.Header.Set("Authorization", "untrusted-browser-token")
	prepareReq.Header.Set("X-Request-Id", "console-request")
	prepareReq.AddCookie(newTestJWTCookie(t, userID, token, secret))
	prepareResp, err := http.DefaultClient.Do(prepareReq)
	require.NoError(t, err)
	defer prepareResp.Body.Close()
	require.Equal(t, http.StatusOK, prepareResp.StatusCode)
	var prepared struct {
		Data struct {
			ID          string `json:"id"`
			Version     uint64 `json:"version"`
			PreviewHash string `json:"previewHash"`
			Status      string `json:"status"`
			After       struct {
				Content string `json:"content"`
			} `json:"after"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(prepareResp.Body).Decode(&prepared))
	require.Equal(t, "preview_ready", prepared.Data.Status)
	require.Equal(t, "timeout: 5s\n", prepared.Data.After.Content)
	require.NotEmpty(t, prepared.Data.PreviewHash)
	require.Zero(t, putCalls)

	confirmBody := `{"proposalVersion":` + fmt.Sprint(prepared.Data.Version) + `,"previewHash":` + mustJSON(t, prepared.Data.PreviewHash) + `,"idempotencyKey":"confirm-1"}`
	confirmURL := console.URL + "/ai/agent/v1/proposals/" + prepared.Data.ID + "/confirm"
	firstReceipt := confirmAgentProposal(t, confirmURL, userID, token, secret, confirmBody)
	secondReceipt := confirmAgentProposal(t, confirmURL, userID, token, secret, confirmBody)
	require.Equal(t, firstReceipt, secondReceipt)
	require.Equal(t, "waiting_for_publish", firstReceipt.Status)
	require.Equal(t, "/configuration?environment=default&group=orders&name=application.yaml", firstReceipt.DetailURL)
	require.Equal(t, "upstream-request", firstReceipt.RequestID)
	require.Equal(t, 1, putCalls)
	require.Equal(t, "timeout: 5s\n", content)
	for _, path := range upstreamPaths {
		assert.NotContains(t, path, "release")
	}
}

func TestAgentUnknownAPIRouteReturnsJSON404(t *testing.T) {
	gin.SetMode(gin.TestMode)
	backend := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer backend.Close()
	router := newAgentTestRouter(t, backend.URL, "secret")
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ai/agent/v1/unknown", nil)
	router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.Contains(t, recorder.Header().Get("Content-Type"), "application/json")
	require.NotContains(t, recorder.Body.String(), "<html")
}

func TestAgentRouterAcceptsValidatedHeaderCredentialsForAPIClients(t *testing.T) {
	gin.SetMode(gin.TestMode)
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Empty(t, r.Header.Get("X-Pole-User"))
		require.Equal(t, "api-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		writeAgentConfigState(t, w, "app.yaml", "before", 1)
	}))
	defer backend.Close()

	router := newAgentTestRouter(t, backend.URL, "secret")
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/ai/agent/v1/proposals/config-file", strings.NewReader(
		`{"namespace":"default","group":"orders","name":"app.yaml","desiredContent":"after"}`,
	))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Pole-User", "api-user")
	req.Header.Set("Authorization", "api-token")
	router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), `"status":"preview_ready"`)
}

func TestAgentRuntimeAndTurnFailClosedWhenLLMGatewayIsNotConfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := newAgentTestRouter(t, "127.0.0.1:1", "secret")

	runtimeRecorder := httptest.NewRecorder()
	runtimeRequest := httptest.NewRequest(http.MethodGet, "/ai/agent/v1/runtime", nil)
	runtimeRequest.Header.Set("X-Pole-User", "admin")
	runtimeRequest.AddCookie(newTestJWTCookie(t, "admin", "pole-token", "secret"))
	router.ServeHTTP(runtimeRecorder, runtimeRequest)
	require.Equal(t, http.StatusOK, runtimeRecorder.Code, runtimeRecorder.Body.String())
	require.Contains(t, runtimeRecorder.Body.String(), `"configured":false`)
	require.Contains(t, runtimeRecorder.Body.String(), `"mcpConnected":false`)
	require.Contains(t, runtimeRecorder.Body.String(), `"mcpTools":[]`)
	require.Contains(t, runtimeRecorder.Body.String(), `"tools":[]`)
	require.NotContains(t, runtimeRecorder.Body.String(), "apiKey")
	require.NotContains(t, runtimeRecorder.Body.String(), "baseURL")

	turnRecorder := httptest.NewRecorder()
	turnRequest := httptest.NewRequest(http.MethodPost, "/ai/agent/v1/turns", strings.NewReader(
		`{"sessionId":"local-session","message":"列出命名空间","history":[]}`,
	))
	turnRequest.Header.Set("Content-Type", "application/json")
	turnRequest.Header.Set("X-Pole-User", "admin")
	turnRequest.AddCookie(newTestJWTCookie(t, "admin", "pole-token", "secret"))
	router.ServeHTTP(turnRecorder, turnRequest)
	require.Equal(t, http.StatusServiceUnavailable, turnRecorder.Code, turnRecorder.Body.String())
	require.Contains(t, turnRecorder.Body.String(), `"category":"RUNTIME_UNAVAILABLE"`)
}

func TestPoleAgentPublishesA2ACardAndProtectsMessageEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := newAgentTestRouter(t, "127.0.0.1:1", "secret")

	cardRecorder := httptest.NewRecorder()
	router.ServeHTTP(cardRecorder, httptest.NewRequest(http.MethodGet, "/.well-known/agent-card.json", nil))
	require.Equal(t, http.StatusOK, cardRecorder.Code, cardRecorder.Body.String())
	require.Contains(t, cardRecorder.Body.String(), `"name":"pole-control-plane"`)
	require.Contains(t, cardRecorder.Body.String(), `"protocolVersion":"0.3.0"`)
	require.Contains(t, cardRecorder.Body.String(), `"preferredTransport":"JSONRPC"`)
	require.Contains(t, cardRecorder.Body.String(), `"defaultInputModes":["text/plain"]`)
	require.Contains(t, cardRecorder.Body.String(), `"bearerAuth"`)
	require.Contains(t, cardRecorder.Body.String(), `"pole-control-plane-management"`)
	require.Contains(t, cardRecorder.Body.String(), `/ai/agent/a2a/v1`)

	messageRecorder := httptest.NewRecorder()
	messageRequest := httptest.NewRequest(http.MethodPost, "/ai/agent/a2a/v1", strings.NewReader(
		`{"jsonrpc":"2.0","id":"request-1","method":"message/send","params":{"message":{"kind":"message","messageId":"message-1","role":"user","parts":[{"kind":"text","text":"列出命名空间"}]}}}`,
	))
	messageRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(messageRecorder, messageRequest)
	require.Equal(t, http.StatusUnauthorized, messageRecorder.Code, messageRecorder.Body.String())

	for _, test := range []struct {
		name     string
		body     string
		wantCode int
		wantRPC  string
	}{
		{name: "syntax", body: `{`, wantCode: http.StatusBadRequest, wantRPC: `"code":-32700`},
		{name: "invalid-id", body: `{"jsonrpc":"2.0","id":true,"method":"message/send"}`,
			wantCode: http.StatusBadRequest, wantRPC: `"code":-32600`},
		{name: "notification", body: `{"jsonrpc":"2.0","method":"message/send","params":{"message":{"parts":[]}}}`,
			wantCode: http.StatusNoContent},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/ai/agent/a2a/v1", strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Pole-User", "admin")
			request.AddCookie(newTestJWTCookie(t, "admin", "pole-token", "secret"))
			router.ServeHTTP(recorder, request)
			require.Equal(t, test.wantCode, recorder.Code, recorder.Body.String())
			if test.wantRPC != "" {
				require.Contains(t, recorder.Body.String(), test.wantRPC)
			}
		})
	}
}

type agentReceipt struct {
	ProposalID string `json:"proposalId"`
	Status     string `json:"status"`
	RequestID  string `json:"requestId"`
	DetailURL  string `json:"detailUrl"`
}

func confirmAgentProposal(t *testing.T, target, userID, token, secret, body string) agentReceipt {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, target, bytes.NewBufferString(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Pole-User", userID)
	req.Header.Set("X-Request-Id", "confirm-request")
	req.AddCookie(newTestJWTCookie(t, userID, token, secret))
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, string(bodyBytes))
	var envelope struct {
		Data agentReceipt `json:"data"`
	}
	require.NoError(t, json.Unmarshal(bodyBytes, &envelope))
	return envelope.Data
}

func newAgentTestRouter(t *testing.T, backendURL, secret string) *gin.Engine {
	t.Helper()
	return NewRouter(&bootstrap.Config{
		WebServer: bootstrap.WebServer{
			CoreURL: "/core/v1", NamingURL: "/naming/v1",
			AuthURL: "/auth/v1", ConfigURL: "/config/v1", MonitorURL: "/api/v1",
			JWT: bootstrap.JWT{SecretKey: secret, Expired: 1800},
		},
		PoleServer: bootstrap.PoleServer{Address: strings.TrimPrefix(backendURL, "http://")},
	}, testAssets("<html>agent test</html>"))
}

func mustJSON(t *testing.T, value string) string {
	t.Helper()
	content, err := json.Marshal(value)
	require.NoError(t, err)
	return string(content)
}

// writeAgentConfigState 使用真实 protobuf JSON/Any 模拟新的统一草稿读取和保存响应。
func writeAgentConfigState(t *testing.T, w http.ResponseWriter, name, content string, revision uint64) {
	t.Helper()
	key := &configv1.ConfigKey{Environment: "default", Group: "orders", Name: name}
	data, err := anypb.New(&configv1.ConfigState{Key: key, Draft: &configv1.ConfigDraft{Key: key, Revision: revision, Definition: &configv1.ConfigDefinition{Engine: "literal", Content: []byte(content), Format: "yaml"}}})
	require.NoError(t, err)
	payload, err := protojson.Marshal(&apimodel.Response{Code: uint32(apimodel.Code_ExecuteSuccess), Data: data})
	require.NoError(t, err)
	_, _ = w.Write(payload)
}
