package consulserver

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/anypb"

	configv1 "github.com/pole-io/specification/source/go/api/v1/config"
	apimodel "github.com/pole-io/specification/source/go/api/v1/model"

	"github.com/pole-io/pole-server/apis/apiserver"
	"github.com/pole-io/pole-server/apis/pkg/types"
)

type discoverFunc func(context.Context, *configv1.DiscoverRequest, string) *apimodel.Response

func (f discoverFunc) DiscoverForProtocol(ctx context.Context, input *configv1.DiscoverRequest, protocol string) *apimodel.Response {
	return f(ctx, input, protocol)
}

func resultResponse(input *configv1.DiscoverRequest, content []byte, absent bool) *apimodel.Response {
	result := &configv1.ConfigResult{Key: input.Key, Presence: configv1.ConfigPresence_CONFIG_PRESENCE_PRESENT,
		ReleaseId: "release", SnapshotRevision: "snapshot", ObservationCursor: &configv1.Cursor{Scope: "config", Epoch: "epoch", Position: 7},
		Snapshot: &configv1.RenderSnapshot{Engine: "literal", Content: content}}
	if absent {
		result.Presence = configv1.ConfigPresence_CONFIG_PRESENCE_ABSENT
		result.ReleaseId, result.Snapshot = "", nil
	}
	data, _ := anypb.New(result)
	return &apimodel.Response{Code: uint32(apimodel.Code_ExecuteSuccess), Data: data}
}

func TestKVPreservesBinaryAndEmptyValues(t *testing.T) {
	for _, content := range [][]byte{nil, {}, {0xff, 0, '\n'}, []byte("value")} {
		for _, raw := range []bool{false, true} {
			reader := discoverFunc(func(_ context.Context, input *configv1.DiscoverRequest, protocol string) *apimodel.Response {
				if protocol != "consul" {
					t.Fatalf("protocol = %q", protocol)
				}
				return resultResponse(input, content, false)
			})
			target := "/v1/kv/group/name"
			if raw {
				target += "?raw"
			}
			response := httptest.NewRecorder()
			NewHandler(reader, "prod", true).ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
			if response.Code != http.StatusOK || response.Header().Get("X-Consul-Index") != "8" || response.Header().Get("X-Pole-Config-Epoch") != "epoch" {
				t.Fatalf("response = %d %v %s", response.Code, response.Header(), response.Body.String())
			}
			if raw {
				if !bytes.Equal(response.Body.Bytes(), content) {
					t.Fatalf("raw bytes changed: %x", response.Body.Bytes())
				}
				continue
			}
			var entries []struct {
				Key         string
				Value       string
				ModifyIndex uint64
			}
			if err := json.Unmarshal(response.Body.Bytes(), &entries); err != nil || len(entries) != 1 || entries[0].Key != "group/name" || entries[0].Value != base64.StdEncoding.EncodeToString(content) || entries[0].ModifyIndex != 8 {
				t.Fatalf("KV envelope = %s, error = %v", response.Body.String(), err)
			}
			// 空值必须是 base64 空字符串，不能变成 JSON null 或缺失配置。
			if bytes.Contains(response.Body.Bytes(), []byte(`"Value":null`)) {
				t.Fatal("empty value became null")
			}
		}
	}
}

func TestKVMapsIdentityLabelsAndCredentials(t *testing.T) {
	reader := discoverFunc(func(ctx context.Context, input *configv1.DiscoverRequest, _ string) *apimodel.Response {
		if input.Key.Environment != "stage" || input.Key.Group != "g" || input.Key.Name != "nested/name" || input.Labels["zone"] != "west" {
			t.Fatalf("identity = %v", input)
		}
		if ctx.Value(types.ContextAuthTokenKey) != "token" || types.GetRequestHeader(ctx)["X-Consul-Token"][0] != "token" {
			t.Fatal("credential was not propagated")
		}
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("read has no deadline")
		}
		return resultResponse(input, []byte("ok"), false)
	})
	request := httptest.NewRequest(http.MethodGet, "/v1/kv/g/nested/name?dc=stage&label.zone=west", nil)
	request.Header.Set("X-Consul-Token", "token")
	response := httptest.NewRecorder()
	NewHandler(reader, "prod", true).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatal(response.Code, response.Body.String())
	}
}

func TestKVRejectsUnsupportedActionsBeforeRead(t *testing.T) {
	reader := discoverFunc(func(context.Context, *configv1.DiscoverRequest, string) *apimodel.Response {
		t.Fatal("invalid operation called domain service")
		return nil
	})
	for _, test := range []struct {
		method, target string
		enabled        bool
		status         int
	}{
		{http.MethodGet, "/v1/kv/g/n", false, 404},
		{http.MethodPut, "/v1/kv/g/n", true, 405},
		{http.MethodGet, "/v1/kv/g/n?recurse", true, 400},
		{http.MethodGet, "/v1/kv/g/n?label.a=1&label.a=2", true, 400},
		{http.MethodGet, "/v1/kv/g/n?index=invalid", true, 400},
		{http.MethodGet, "/v1/kv/g/n?wait=11m", true, 400},
		{http.MethodGet, "/v1/kv/group", true, 400},
	} {
		response := httptest.NewRecorder()
		NewHandler(reader, "prod", test.enabled).ServeHTTP(response, httptest.NewRequest(test.method, test.target, nil))
		if response.Code != test.status {
			t.Fatalf("%s %s = %d", test.method, test.target, response.Code)
		}
	}
}

func TestKVAbsenceAndFailuresRemainDistinct(t *testing.T) {
	for _, test := range []struct {
		code   apimodel.Code
		status int
	}{
		{apimodel.Code_ExecuteSuccess, 404},
		{apimodel.Code_AuthTokenForbidden, 403},
		{apimodel.Code_ClientAPINotOpen, 403},
		{apimodel.Code_ConfigUnavailable, 503},
	} {
		reader := discoverFunc(func(_ context.Context, input *configv1.DiscoverRequest, _ string) *apimodel.Response {
			if test.code == apimodel.Code_ExecuteSuccess {
				return resultResponse(input, nil, true)
			}
			return &apimodel.Response{Code: uint32(test.code)}
		})
		response := httptest.NewRecorder()
		NewHandler(reader, "prod", true).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/kv/g/n", nil))
		if response.Code != test.status || (test.status != 404 && response.Header().Get("X-Consul-Index") != "") {
			t.Fatalf("failure mapped to state: %d %v", response.Code, response.Header())
		}
	}
}

func TestKVBlockingIndexAndCancellation(t *testing.T) {
	reader := discoverFunc(func(_ context.Context, input *configv1.DiscoverRequest, _ string) *apimodel.Response {
		return resultResponse(input, []byte("current"), false)
	})
	for _, target := range []string{"/v1/kv/g/n?index=100&wait=5m", "/v1/kv/g/n?index=8&wait=0s"} {
		response := httptest.NewRecorder()
		NewHandler(reader, "prod", true).ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
		if response.Code != 200 || response.Header().Get("X-Consul-Index") != "8" {
			t.Fatal("blocking baseline failed", response.Code, response.Header())
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancelling := discoverFunc(func(_ context.Context, input *configv1.DiscoverRequest, _ string) *apimodel.Response {
		cancel()
		return resultResponse(input, nil, false)
	})
	response := httptest.NewRecorder()
	NewHandler(cancelling, "prod", true).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/kv/g/n?index=8", nil).WithContext(ctx))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatal("cancel was converted to success", response.Code)
	}
}

func TestServerOptionsAndDisabledAdapter(t *testing.T) {
	server := &Server{}
	if err := server.Initialize(context.Background(), map[string]interface{}{}, nil); err == nil {
		t.Fatal("implicit environment accepted")
	}
	if err := server.Initialize(context.Background(), map[string]interface{}{"environment": "prod", "listenPort": -1}, nil); err == nil {
		t.Fatal("invalid port accepted")
	}
	if err := server.Initialize(context.Background(), map[string]interface{}{"environment": "prod", "listenIP": "::1", "listenPort": 8501}, map[string]apiserver.APIConfig{"client": {Enable: true, Include: []string{"unrelated"}}}); err != nil {
		t.Fatal(err)
	}
	if server.GetProtocol() != Protocol || server.GetPort() != 8501 || server.server.Addr != "[::1]:8501" {
		t.Fatal("listener options not applied", server)
	}
	response := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/kv/g/n", nil))
	if response.Code != 404 {
		t.Fatal("disabled adapter served request", response.Code)
	}
	// 服务发现权限不会自动开放配置转换。
	if err := server.Initialize(context.Background(), map[string]interface{}{"environment": "prod"}, map[string]apiserver.APIConfig{"client": {Enable: true, Include: []string{apiserver.DiscoverAccess}}}); err != nil {
		t.Fatal(err)
	}
	response = httptest.NewRecorder()
	server.server.Handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/kv/g/n", nil))
	if response.Code != 404 {
		t.Fatal("discovery access enabled configuration", response.Code)
	}
}

func TestServerRapidRestartKeepsListenerOwnership(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	options := map[string]interface{}{"environment": "prod", "listenIP": "127.0.0.1", "listenPort": port}
	server := &Server{}
	if err := server.Initialize(context.Background(), options, nil); err != nil {
		t.Fatal(err)
	}
	defer server.Stop()
	failures := make(chan error, 16)
	server.Run(failures)
	// 连续替换时旧 Run 的 goroutine 必须只操作它原来拥有的 http.Server。
	for index := 0; index < 8; index++ {
		if err := server.Restart(options, nil, failures); err != nil {
			t.Fatal(err)
		}
	}
	client := &http.Client{Timeout: 200 * time.Millisecond}
	defer client.CloseIdleConnections()
	deadline := time.Now().Add(3 * time.Second)
	for {
		select {
		case failure := <-failures:
			t.Fatal(failure)
		default:
		}
		response, err := client.Get("http://127.0.0.1:" + strconv.Itoa(port) + "/v1/kv/g/n")
		if err == nil {
			_, _ = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
			if response.StatusCode != 404 {
				t.Fatal("disabled configuration listener changed behavior", response.StatusCode)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("restarted listener unavailable", err)
		}
		time.Sleep(time.Millisecond)
	}
}
