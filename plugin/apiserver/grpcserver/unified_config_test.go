package grpcserver

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	configv1 "github.com/pole-io/specification/source/go/api/v1/config"
	apiconfig "github.com/pole-io/specification/source/go/api/v1/config_manage"
	apimodel "github.com/pole-io/specification/source/go/api/v1/model"

	"github.com/pole-io/pole-server/apis/observability/statis"
	"github.com/pole-io/pole-server/apis/pkg/types/metrics"
	"github.com/pole-io/pole-server/apis/store"
	commonlog "github.com/pole-io/pole-server/pkg/common/log"
	"github.com/pole-io/pole-server/pkg/config/unified"
	grpcv1 "github.com/pole-io/pole-server/plugin/apiserver/grpcserver/discover/v1"
)

type unifiedRuntimeStatis struct {
	statis.Statis
	calls chan metrics.CallMetric
}

func (s unifiedRuntimeStatis) ReportCallMetrics(metric metrics.CallMetric) { s.calls <- metric }

type deniedConfigAuthorizer struct{}

func (deniedConfigAuthorizer) Authorize(context.Context, *configv1.ConfigKey, unified.Action) (string, error) {
	return "", unified.ErrAccessDenied
}

type unusedConfigRepository struct{ store.UnifiedConfigStore }

func TestConfigDiscoverProductionStreamWrapperHandlesDomainFrames(t *testing.T) {
	statistics := unifiedRuntimeStatis{calls: make(chan metrics.CallMetric, 8)}
	base := &BaseGrpcServer{log: commonlog.GetScopeOrDefaultByName(commonlog.APIServerLoggerName), statis: statistics}
	core, err := unified.New(unusedConfigRepository{}, deniedConfigAuthorizer{})
	if err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer(grpc.StreamInterceptor(base.streamInterceptor))
	apiconfig.RegisterConfigGRPCServer(server, grpcv1.NewConfigGRPCServer(grpcv1.WithUnifiedConfigServer(core)))
	listener := bufconn.Listen(1 << 20)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	t.Cleanup(func() { _ = listener.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, "passthrough:///wrapped-config", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	stream, err := apiconfig.NewConfigGRPCClient(conn).Discover(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, request := range []*configv1.DiscoverRequest{{Key: &configv1.ConfigKey{Environment: "prod", Group: "app", Name: "secret"}}, {}} {
		if err := stream.Send(request); err != nil {
			t.Fatal(err)
		}
		response, err := stream.Recv()
		if err != nil || response.GetCode() == uint32(apimodel.Code_ExecuteSuccess) || response.GetData() != nil {
			t.Fatalf("wrapped stream lost failure frame: %v %v", response, err)
		}
		select {
		case metric := <-statistics.calls:
			if metric.API != "/v1.ConfigGRPC/Discover" || metric.Code != int(response.GetCode()) {
				t.Fatalf("domain metric lost: %#v", metric)
			}
		case <-time.After(time.Second):
			t.Fatal("production wrapper skipped domain metric")
		}
	}
	if err := stream.CloseSend(); err != nil {
		t.Fatal(err)
	}
	if _, err := stream.Recv(); err != io.EOF {
		t.Fatalf("wrapped stream EOF: %v", err)
	}
	// 非旧 ResponseMessage 且已有传输失败时，日志只记录传输状态，不解引用旧 response。
	base.postprocess(&VirtualStream{Code: -2, StartTime: time.Now(), Method: "/v1.ConfigGRPC/Discover"}, nil)
}
