package config

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/emicklei/go-restful/v3"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/anypb"

	configv1 "github.com/pole-io/specification/source/go/api/v1/config"
	apimodel "github.com/pole-io/specification/source/go/api/v1/model"

	"github.com/pole-io/pole-server/apis/pkg/types"
	"github.com/pole-io/pole-server/pkg/config/unified"
)

func unifiedTestContainer(consoleEnabled bool, include []string) *restful.Container {
	container := restful.NewContainer()
	container.Add((&HTTPServer{}).GetUnifiedAccessServer(consoleEnabled, include))
	return container
}

func unifiedTestPOST(container *restful.Container, path string, body []byte) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/config/v1/"+path, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	container.ServeHTTP(response, request)
	return response
}

func TestUnifiedHTTPManagementActions(t *testing.T) {
	container := unifiedTestContainer(true, nil)
	actions := []string{"files", "file-template", "file-template-draft", "draft", "state", "preview", "publish", "promote-gray", "stop", "stop-all-gray", "copy-release-to-draft", "rollback", "offline"}
	for _, action := range actions {
		response := unifiedTestPOST(container, action, []byte("{"))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s: HTTP %d, want 400", action, response.Code)
		}
		var envelope apimodel.Response
		if err := protojson.Unmarshal(response.Body.Bytes(), &envelope); err != nil || envelope.GetCode() != uint32(apimodel.Code_InvalidParameter) || envelope.GetData() != nil {
			t.Fatalf("%s: invalid public failure %s: %v", action, response.Body, err)
		}
	}
	for _, removed := range []string{"resolve", "watch", "promote", "groups", "file", "templates", "ConfigDiscover"} {
		if response := unifiedTestPOST(container, removed, []byte("{}")); response.Code != http.StatusNotFound {
			t.Fatalf("obsolete %s route returned HTTP %d", removed, response.Code)
		}
	}
	for _, removed := range []string{"groups", "file", "files/search", "templates", "GetConfigFile"} {
		request := httptest.NewRequest(http.MethodGet, "/config/v1/"+removed, nil)
		response := httptest.NewRecorder()
		container.ServeHTTP(response, request)
		if response.Code != http.StatusNotFound {
			t.Fatalf("obsolete GET %s route returned HTTP %d", removed, response.Code)
		}
	}
}

func TestUnifiedHTTPFlagsAndBodyLimit(t *testing.T) {
	for _, test := range []struct {
		name         string
		enabled      bool
		include      []string
		state, draft bool
	}{
		{name: "disabled"},
		{name: "read only", enabled: true, include: []string{defaultReadAccess}, state: true},
		{name: "all", enabled: true, state: true, draft: true},
		{name: "unrelated", enabled: true, include: []string{"service"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			container := unifiedTestContainer(test.enabled, test.include)
			for _, route := range []struct {
				path    string
				enabled bool
			}{{"state", test.state}, {"draft", test.draft}, {"promote-gray", test.draft}} {
				response := unifiedTestPOST(container, route.path, []byte("{"))
				expected := http.StatusNotFound
				if route.enabled {
					expected = http.StatusBadRequest
				}
				if response.Code != expected {
					t.Fatalf("%s: HTTP %d, want %d", route.path, response.Code, expected)
				}
			}
		})
	}
	container := unifiedTestContainer(true, nil)
	response := unifiedTestPOST(container, "draft", bytes.Repeat([]byte(" "), unifiedMaxBodyBytes+1))
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized document: HTTP %d", response.Code)
	}
	var envelope apimodel.Response
	if err := protojson.Unmarshal(response.Body.Bytes(), &envelope); err != nil || envelope.GetCode() != uint32(apimodel.Code_InvalidParameter) {
		t.Fatalf("oversized document response: %s: %v", response.Body, err)
	}
	response = unifiedTestPOST(container, "state", []byte("{}"))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("missing service: HTTP %d", response.Code)
	}
}

func TestUnifiedPublicResponseUsesTypedAnyAndCodeMapping(t *testing.T) {
	snapshot, err := anypb.New(&configv1.ConfigState{Key: &configv1.ConfigKey{Environment: "dev", Group: "app", Name: "settings"}})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/config/v1/state", bytes.NewReader(nil))
	response := httptest.NewRecorder()
	writeUnified(restful.NewRequest(request), restful.NewResponse(response), &apimodel.Response{Code: uint32(apimodel.Code_ExecuteSuccess), Data: snapshot}, http.StatusOK)
	if err := protojson.Unmarshal(response.Body.Bytes(), &apimodel.Response{}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(response.Body.Bytes(), []byte("\"@type\":\"type.googleapis.com/pole.config.v1.ConfigState\"")) {
		t.Fatalf("Any lacks protobuf type URL: %s", response.Body)
	}
	for _, test := range []struct {
		code   apimodel.Code
		status int
	}{
		{apimodel.Code_ExecuteSuccess, 200}, {apimodel.Code_InvalidParameter, 400},
		{apimodel.Code_DataConflict, 409}, {apimodel.Code_NotFoundResource, 404},
		{apimodel.Code_Unauthorized, 401}, {apimodel.Code_NotAllowedAccess, 401},
		{apimodel.Code_AuthTokenForbidden, 403}, {apimodel.Code_OperationRoleForbidden, 403},
		{apimodel.Code_ConfigUnavailable, 503}, {apimodel.Code_ConfigCursorReset, 409},
		{apimodel.Code_ConfigRenderFailed, 422}, {apimodel.Code_ClientAPINotOpen, 400},
	} {
		if got := unifiedHTTPStatus(uint32(test.code)); got != test.status {
			t.Fatalf("%s: HTTP %d, want %d", test.code, got, test.status)
		}
	}
}

func TestUnifiedHTTPContextKeepsCredentialWithoutTrustingStaffname(t *testing.T) {
	ws := new(restful.WebService)
	ws.Path("/config/v1").Consumes(restful.MIME_JSON).Produces(restful.MIME_JSON)
	var gotContext context.Context
	ws.Route(unifiedPOST(ws, "/probe").To(unifiedHandler(&HTTPServer{unifiedService: &unified.Service{}},
		func() *configv1.GetStateRequest { return &configv1.GetStateRequest{} },
		func(ctx context.Context, _ *configv1.GetStateRequest) *apimodel.Response {
			gotContext = ctx
			return unifiedFailure(apimodel.Code_ExecuteSuccess, "")
		})))
	container := restful.NewContainer()
	container.Add(ws)
	request := httptest.NewRequest(http.MethodPost, "/config/v1/probe", bytes.NewReader([]byte("{}")))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(types.HeaderAuthorizationKey, "test-token")
	request.Header.Set("Staffname", "forged-actor")
	response := httptest.NewRecorder()
	container.ServeHTTP(response, request)
	if response.Code != http.StatusOK || gotContext == nil {
		t.Fatalf("request did not reach service: HTTP %d", response.Code)
	}
	if got := gotContext.Value(types.ContextAuthTokenKey); got != "test-token" {
		t.Fatalf("credential was lost: %v", got)
	}
	if got := gotContext.Value(types.ContextOperator); got == "forged-actor" {
		t.Fatal("untrusted Staffname became operator")
	}
}
