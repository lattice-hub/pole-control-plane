package apolloserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/emicklei/go-restful/v3"
	"google.golang.org/protobuf/types/known/anypb"

	configv1 "github.com/pole-io/specification/source/go/api/v1/config"
	apimodel "github.com/pole-io/specification/source/go/api/v1/model"
)

type fakeDiscover func(*configv1.DiscoverRequest, string) *apimodel.Response

func (f fakeDiscover) DiscoverForProtocol(_ context.Context, request *configv1.DiscoverRequest, protocol string) *apimodel.Response {
	response := f(request, protocol)
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

func present(content []byte) *apimodel.Response {
	data, _ := anypb.New(&configv1.ConfigResult{Presence: configv1.ConfigPresence_CONFIG_PRESENCE_PRESENT, SnapshotRevision: "selected-gray-and-values", ObservationCursor: &configv1.Cursor{Position: 42}, Snapshot: &configv1.RenderSnapshot{Engine: "literal", Content: content, Format: "properties"}})
	return &apimodel.Response{Code: uint32(apimodel.Code_ExecuteSuccess), Data: data}
}

func TestApolloSnapshotRevisionAndNotification(t *testing.T) {
	server := &ApolloServer{configSvr: fakeDiscover(func(request *configv1.DiscoverRequest, protocol string) *apimodel.Response {
		if protocol != "apollo" || request.Key.Environment != "dev" || request.Key.Group != "app" {
			t.Fatal(request, protocol)
		}
		return present([]byte("x=y\r\n"))
	})}
	request := &GetConfigFileRequest{AppId: "app", Cluster: "dev", Filename: "application.properties"}
	response, err := server.GetConfigFile(context.Background(), request)
	if err != nil || response.Configurations["x"] != "y" || response.ReleaseKey != "selected-gray-and-values" {
		t.Fatal(response, err)
	}
	request.Version = response.ReleaseKey
	response, err = server.GetConfigFile(context.Background(), request)
	if err != nil || response != nil {
		t.Fatal(response, err)
	}
	changed, err := server.WatchConfigFile(context.Background(), &WatchConfigFileRequest{AppId: "app", Cluster: "dev", Notifications: []*ApolloConfigNotification{{NamespaceName: "application.properties", NotificationId: 41}}})
	if err != nil || len(changed.Notifications) != 1 || changed.Notifications[0].NotificationId != 42 {
		t.Fatal(changed, err)
	}
}

func TestApolloFailureDoesNotFallback(t *testing.T) {
	calls := 0
	server := &ApolloServer{configSvr: fakeDiscover(func(*configv1.DiscoverRequest, string) *apimodel.Response {
		calls++
		return &apimodel.Response{Code: uint32(apimodel.Code_ClientAPINotOpen)}
	})}
	response, err := server.GetConfigFile(context.Background(), &GetConfigFileRequest{AppId: "app", Cluster: "dev", DataCenter: "other", Filename: "application"})
	if response != nil || err == nil || calls != 1 {
		t.Fatal(response, err, calls)
	}
}

func TestApolloRawLiteralBytes(t *testing.T) {
	body := []byte{0, 255, 13, 10}
	server := &ApolloServer{configSvr: fakeDiscover(func(*configv1.DiscoverRequest, string) *apimodel.Response { return present(body) })}
	container := restful.NewContainer()
	container.Add(server.GetApolloServer())
	request := httptest.NewRequest(http.MethodGet, "/configfiles/app/dev/application", nil)
	response := httptest.NewRecorder()
	container.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.String() != string(body) {
		t.Fatal(response.Code, response.Body.Bytes())
	}
}

func TestApolloPropertiesMappingKeepsApplicationPlaceholders(t *testing.T) {
	t.Setenv("POLE_TEST_PROPERTIES_ENV", "must-not-expand")
	for _, test := range []struct {
		name, content string
		expected      map[string]string
	}{
		{"key-only", "keyOnly\n", map[string]string{"keyOnly": ""}},
		{"continuation", "continued=one\\\r\n  two\n", map[string]string{"continued": "onetwo"}},
		{"continuation-lf", "continued=one\\\n  two\n", map[string]string{"continued": "onetwo"}},
		{"escaped-key", "escaped\\ key\\:part=value\n", map[string]string{"escaped key:part": "value"}},
		{"environment-literal", "x=${POLE_TEST_PROPERTIES_ENV}\n", map[string]string{"x": "${POLE_TEST_PROPERTIES_ENV}"}},
		{"self-reference-literal", "x=${x}\n", map[string]string{"x": "${x}"}},
		{"cycle-literal", "x=${y}\ny=${x}\n", map[string]string{"x": "${y}", "y": "${x}"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := &ApolloServer{configSvr: fakeDiscover(func(*configv1.DiscoverRequest, string) *apimodel.Response { return present([]byte(test.content)) })}
			response, err := server.GetConfigFile(context.Background(), &GetConfigFileRequest{AppId: "app", Cluster: "dev", Filename: "application.properties"})
			if err != nil {
				t.Fatal(err)
			}
			if len(response.Configurations) != len(test.expected) {
				t.Fatal(response.Configurations)
			}
			for key, value := range test.expected {
				actual, exists := response.Configurations[key]
				if !exists || actual != value {
					t.Fatalf("%q=%q want %q", key, actual, value)
				}
			}
			if string(response.Content) != test.content {
				t.Fatal("raw content changed")
			}
		})
	}
	server := &ApolloServer{configSvr: fakeDiscover(func(*configv1.DiscoverRequest, string) *apimodel.Response { return present([]byte("x=\\uZZZZ\n")) })}
	if _, err := server.GetConfigFile(context.Background(), &GetConfigFileRequest{AppId: "app", Cluster: "dev", Filename: "application.properties"}); err == nil {
		t.Fatal("invalid unicode escape accepted")
	}
}
