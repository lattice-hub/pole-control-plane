package discover

import (
	"testing"

	"github.com/pole-io/pole-server/apis/apiserver"
	"google.golang.org/grpc"

	v1 "github.com/pole-io/pole-server/plugin/apiserver/grpcserver/discover/v1"
)

func TestGRPCServerHasOnlyOneClientConfigService(t *testing.T) {
	server := grpc.NewServer()
	t.Cleanup(server.Stop)
	g := &GRPCServer{openAPI: map[string]apiserver.APIConfig{"client": {Enable: true, Include: []string{apiserver.ConfigAccess}}}, dsvr: v1.NewDiscoverGRPCServer(), csvr: v1.NewConfigGRPCServer()}
	g.registerServices(server)
	configServices := 0
	for name, info := range server.GetServiceInfo() {
		if name == "pole.config.v1.ConfigService" || name == "pole.config.v2.ConfigService" {
			t.Fatalf("parallel config service registered: %s", name)
		}
		if name == "v1.ConfigGRPC" {
			configServices++
			if len(info.Methods) != 1 {
				t.Fatalf("config method count = %d, want 1", len(info.Methods))
			}
			for _, method := range info.Methods {
				if method.Name != "Discover" || !method.IsClientStream || !method.IsServerStream {
					t.Fatalf("management RPC registered: %s", method.Name)
				}
			}
		}
	}
	if configServices != 1 {
		t.Fatalf("config services registered = %d", configServices)
	}
}

func TestConfigGRPCRegistrationRespectsClientConfigFlag(t *testing.T) {
	for _, test := range []struct {
		name       string
		client     apiserver.APIConfig
		registered bool
		absent     bool
	}{
		{name: "client absent", absent: true},
		{name: "disabled"},
		{name: "discover only", client: apiserver.APIConfig{Enable: true, Include: []string{apiserver.DiscoverAccess}}},
		{name: "config enabled", client: apiserver.APIConfig{Enable: true, Include: []string{apiserver.ConfigAccess}}, registered: true},
		{name: "empty includes default", client: apiserver.APIConfig{Enable: true}, registered: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := grpc.NewServer()
			t.Cleanup(server.Stop)
			apiConfig := map[string]apiserver.APIConfig{}
			if !test.absent {
				apiConfig["client"] = test.client
			}
			g := &GRPCServer{openAPI: apiConfig, dsvr: v1.NewDiscoverGRPCServer(), csvr: v1.NewConfigGRPCServer()}
			g.registerServices(server)
			_, registered := server.GetServiceInfo()["v1.ConfigGRPC"]
			if registered != test.registered {
				t.Fatalf("config registered=%v, want=%v", registered, test.registered)
			}
		})
	}
}
