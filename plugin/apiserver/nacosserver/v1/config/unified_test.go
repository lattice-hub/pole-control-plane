package config

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/emicklei/go-restful/v3"
	"google.golang.org/protobuf/types/known/anypb"

	configv1 "github.com/pole-io/specification/source/go/api/v1/config"
	apimodel "github.com/pole-io/specification/source/go/api/v1/model"

	"github.com/pole-io/pole-server/apis/pkg/types"
	"github.com/pole-io/pole-server/plugin/apiserver/nacosserver/model"
)

type fakeDiscover func(context.Context, *configv1.DiscoverRequest, string) *apimodel.Response

func (f fakeDiscover) DiscoverForProtocol(ctx context.Context, request *configv1.DiscoverRequest, protocol string) *apimodel.Response {
	response := f(ctx, request, protocol)
	if response.Data != nil {
		result := &configv1.ConfigResult{}
		if response.Data.UnmarshalTo(result) == nil {
			result.Key = request.Key
			result.ReleaseId = "release"
			if result.SnapshotRevision == "" {
				result.SnapshotRevision = "snapshot"
			}
			if result.ObservationCursor == nil {
				result.ObservationCursor = &configv1.Cursor{}
			}
			result.ObservationCursor.Scope = "config"
			result.ObservationCursor.Epoch = "epoch"
			response.Data, _ = anypb.New(result)
		}
	}
	return response
}

func TestNacosV1BytesChecksumAndAuthentication(t *testing.T) {
	server := &ConfigServer{configSvr: fakeDiscover(func(ctx context.Context, request *configv1.DiscoverRequest, protocol string) *apimodel.Response {
		if ctx.Value(types.ContextAuthTokenKey) != "test-token" || request.Key.Environment != "test" || protocol != "nacos" {
			t.Fatal(request, protocol)
		}
		data, _ := anypb.New(&configv1.ConfigResult{Presence: configv1.ConfigPresence_CONFIG_PRESENCE_PRESENT, Snapshot: &configv1.RenderSnapshot{Engine: "literal", Content: []byte{0, 255, 13, 10}}})
		return &apimodel.Response{Code: uint32(apimodel.Code_ExecuteSuccess), Data: data}
	})}
	container := restful.NewContainer()
	ws, _ := server.GetClientServer()
	container.Add(ws)
	request := httptest.NewRequest(http.MethodGet, "/nacos/v1/cs/configs?tenant=test&group=g&dataId=n&accessToken=test-token", nil)
	response := httptest.NewRecorder()
	container.ServeHTTP(response, request)
	if response.Code != 200 || response.Body.String() != string([]byte{0, 255, 13, 10}) || response.Header().Get(model.HeaderContentMD5) == "" {
		t.Fatal(response.Code, response.Body.Bytes(), response.Header())
	}
}

func TestNacosV1FailureIsNotEmptyChange(t *testing.T) {
	server := &ConfigServer{configSvr: fakeDiscover(func(context.Context, *configv1.DiscoverRequest, string) *apimodel.Response {
		return &apimodel.Response{Code: uint32(apimodel.Code_ClientAPINotOpen)}
	})}
	request := restful.NewRequest(httptest.NewRequest(http.MethodPost, "/", nil))
	response := httptest.NewRecorder()
	server.handleWatch(context.Background(), &model.ConfigWatchContext{Request: request, Items: []*model.ConfigListenItem{{Tenant: "test", Group: "g", DataId: "n", Md5: "old"}}}, restful.NewResponse(response))
	if response.Code != http.StatusForbidden {
		t.Fatal(response.Code, response.Body.String())
	}
	if _, err := server.handlePublishConfig(context.Background(), &model.ConfigFile{}); err == nil {
		t.Fatal("legacy write enabled")
	}
}
