package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/pole-io/pole-server/apis/apiserver"
	"github.com/pole-io/pole-server/apis/observability/statis"
	"github.com/pole-io/pole-server/apis/pkg/types/metrics"
	"github.com/pole-io/pole-server/pkg/service/healthcheck"
	authhttp "github.com/pole-io/pole-server/plugin/apiserver/httpserver/auth"
	confighttp "github.com/pole-io/pole-server/plugin/apiserver/httpserver/config"
	discovery "github.com/pole-io/pole-server/plugin/apiserver/httpserver/discover"
)

type unifiedHTTPContainerStatis struct{ statis.Statis }

func (unifiedHTTPContainerStatis) ReportCallMetrics(metrics.CallMetric) {}

func TestUnifiedConfigProductionContainerHasOneV1RootAndDispatchesRoutes(t *testing.T) {
	server := &HTTPServer{
		openAPI:   map[string]apiserver.APIConfig{"console": {Enable: true, Include: []string{"config"}}, "client": {Enable: true, Include: []string{apiserver.DiscoverAccess}}},
		configSvr: &confighttp.HTTPServer{}, discoverSvr: &discovery.HTTPServer{}, authSvr: &authhttp.HTTPServer{},
		healthCheckServer: &healthcheck.Server{}, enablePprof: &atomic.Bool{}, statis: unifiedHTTPContainerStatis{},
	}
	container, err := server.createRestfulContainer()
	if err != nil {
		t.Fatal(err)
	}
	roots := 0
	for _, ws := range container.RegisteredWebServices() {
		if ws.RootPath() == "/config/v2" {
			t.Fatal("v2 alias was registered")
		}
		if ws.RootPath() == "/config/v1" {
			roots++
		}
	}
	if roots != 1 {
		t.Fatalf("config root registered %d times, want once", roots)
	}
	for _, action := range []string{"draft", "state", "preview", "publish", "stop", "stop-all-gray", "copy-release-to-draft", "rollback", "offline", "files", "file-template", "file-template-draft", "promote-gray"} {
		request := httptest.NewRequest(http.MethodPost, "/config/v1/"+action, strings.NewReader("{"))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		container.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s is unreachable or did not parse JSON: %d %s", action, response.Code, response.Body)
		}
	}
	for _, action := range []string{"groups", "promote", "resolve", "watch", "templates"} {
		request := httptest.NewRequest(http.MethodPost, "/config/v1/"+action, strings.NewReader("{}"))
		response := httptest.NewRecorder()
		container.ServeHTTP(response, request)
		if response.Code != http.StatusNotFound {
			t.Fatalf("obsolete %s route returned %d", action, response.Code)
		}
	}
	request := httptest.NewRequest(http.MethodPost, "/config/v2/resolve", strings.NewReader("{}"))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	container.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("v2 alias remained reachable: %d", response.Code)
	}
}

func TestUnifiedConfigProductionContainerRespectsV1APIFlags(t *testing.T) {
	for _, test := range []struct {
		name            string
		console, client apiserver.APIConfig
		actions         []string
	}{
		{name: "disabled"},
		{name: "management read only", console: apiserver.APIConfig{Enable: true, Include: []string{"default-read"}}, actions: []string{"state", "files", "file-template"}},
		{name: "runtime only", client: apiserver.APIConfig{Enable: true, Include: []string{apiserver.DiscoverAccess}}},
		{name: "unrelated includes", console: apiserver.APIConfig{Enable: true, Include: []string{"service"}}, client: apiserver.APIConfig{Enable: true, Include: []string{apiserver.RegisterAccess}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := &HTTPServer{openAPI: map[string]apiserver.APIConfig{"console": test.console, "client": test.client}, configSvr: &confighttp.HTTPServer{}, discoverSvr: &discovery.HTTPServer{}, authSvr: &authhttp.HTTPServer{}, healthCheckServer: &healthcheck.Server{}, enablePprof: &atomic.Bool{}, statis: unifiedHTTPContainerStatis{}}
			container, err := server.createRestfulContainer()
			if err != nil {
				t.Fatal(err)
			}
			enabled := map[string]bool{}
			for _, action := range test.actions {
				enabled[action] = true
			}
			for _, action := range []string{"draft", "state", "preview", "publish", "stop", "stop-all-gray", "copy-release-to-draft", "rollback", "offline", "files", "file-template", "file-template-draft", "promote-gray", "resolve", "watch", "groups"} {
				request := httptest.NewRequest(http.MethodPost, "/config/v1/"+action, strings.NewReader("{"))
				request.Header.Set("Content-Type", "application/json")
				response := httptest.NewRecorder()
				container.ServeHTTP(response, request)
				if enabled[action] && response.Code != http.StatusBadRequest {
					t.Fatalf("enabled %s returned %d", action, response.Code)
				}
				if !enabled[action] && response.Code != http.StatusNotFound {
					t.Fatalf("disabled %s returned %d", action, response.Code)
				}
			}
		})
	}
}
