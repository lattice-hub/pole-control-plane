package limiter

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	apiv2 "github.com/pole-io/specification/source/go/api/v1/traffic_manage/ratelimiter"
)

func TestRunningCanStartStopAndStartAgain(t *testing.T) {
	for range 2 {
		running, err := Start(context.Background(), testConfig())
		require.NoError(t, err)
		require.Len(t, running.Endpoints(), 1)

		address := strings.TrimSuffix(running.Endpoints()[0], "/grpc")
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		conn, err := grpc.DialContext(ctx, address,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock())
		cancel()
		require.NoError(t, err)

		response, err := apiv2.NewRateLimitGRPCClient(conn).TimeAdjust(
			context.Background(), &apiv2.TimeAdjustRequest{})
		require.NoError(t, err)
		require.NotZero(t, response.GetServerTimestamp())
		require.NoError(t, conn.Close())

		stopCtx, stopCancel := context.WithTimeout(context.Background(), 3*time.Second)
		require.NoError(t, running.Stop(stopCtx))
		stopCancel()
		require.NoError(t, running.Wait())
	}
}

func TestStartReturnsListenerBindError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = listener.Close() }()
	port := listener.Addr().(*net.TCPAddr).Port

	cfg := testConfig()
	cfg.APIServers[0].Option["port"] = port
	_, err = Start(context.Background(), cfg)

	require.ErrorContains(t, err, "listen limiter grpc")
	require.ErrorContains(t, err, strconv.Itoa(port))
}

func TestRunningServesQuotaLeaseProtocol(t *testing.T) {
	running, err := Start(context.Background(), testConfig())
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		require.NoError(t, running.Stop(ctx))
	})

	address := strings.TrimSuffix(running.Endpoints()[0], "/grpc")
	dialContext, dialCancel := context.WithTimeout(context.Background(), 3*time.Second)
	conn, err := grpc.DialContext(dialContext, address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock())
	dialCancel()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })

	stream, err := apiv2.NewRateLimitGRPCClient(conn).Service(context.Background())
	require.NoError(t, err)
	require.NoError(t, stream.Send(&apiv2.RateLimitRequest{
		Cmd: apiv2.RateLimitCmd_INIT,
		RateLimitInitRequest: &apiv2.RateLimitInitRequest{
			Target:   &apiv2.LimitTarget{Namespace: "test", Service: "llm"},
			ClientId: "grpc-lease-client",
			Totals: []*apiv2.QuotaTotal{{
				Mode:       apiv2.QuotaMode_WHOLE,
				Duration:   60,
				MaxAmount:  10,
				Accounting: apiv2.QuotaAccounting_CONSUMABLE,
			}},
			SlideCount: 1,
			Mode:       apiv2.Mode_BATCH_OCCUPY,
		},
	}))
	initialized, err := stream.Recv()
	require.NoError(t, err)
	require.Equal(t, uint32(200000), initialized.GetRateLimitInitResponse().GetCode())
	counterKey := initialized.GetRateLimitInitResponse().GetCounters()[0].GetCounterKey()
	clientKey := initialized.GetRateLimitInitResponse().GetClientKey()

	require.NoError(t, stream.Send(&apiv2.RateLimitRequest{
		Cmd: apiv2.RateLimitCmd_RESERVE,
		QuotaReserveRequest: &apiv2.QuotaReserveRequest{
			ClientKey:      clientKey,
			IdempotencyKey: "grpc-reserve",
			Reservations: []*apiv2.QuotaReservation{{
				CounterKey: counterKey,
				Amount:     10,
			}},
			TtlSeconds: 30,
		},
	}))
	reserved, err := stream.Recv()
	require.NoError(t, err)
	require.Equal(t, uint32(200000), reserved.GetQuotaReserveResponse().GetCode())
	leaseID := reserved.GetQuotaReserveResponse().GetLeaseId()

	require.NoError(t, stream.Send(&apiv2.RateLimitRequest{
		Cmd: apiv2.RateLimitCmd_UPDATE,
		QuotaUpdateRequest: &apiv2.QuotaUpdateRequest{
			ClientKey: clientKey,
			LeaseId:   leaseID,
			Sequence:  1,
			Consumptions: []*apiv2.QuotaConsumption{{
				CounterKey:    counterKey,
				ConsumedTotal: 4,
			}},
		},
	}))
	updated, err := stream.Recv()
	require.NoError(t, err)
	require.Equal(t, uint32(4), updated.GetQuotaUpdateResponse().GetConsumptions()[0].GetConsumedTotal())

	require.NoError(t, stream.Send(&apiv2.RateLimitRequest{
		Cmd: apiv2.RateLimitCmd_SETTLE,
		QuotaSettleRequest: &apiv2.QuotaSettleRequest{
			ClientKey: clientKey,
			LeaseId:   leaseID,
			Sequence:  2,
		},
	}))
	settled, err := stream.Recv()
	require.NoError(t, err)
	require.Equal(t, uint32(6), settled.GetQuotaSettleResponse().GetSettlements()[0].GetReturnedAmount())
	require.NoError(t, stream.CloseSend())
}

func TestRunningServesQuotaLeaseSnapshot(t *testing.T) {
	cfg := testConfig()
	cfg.APIServers = append(cfg.APIServers, APIServerConfig{
		Name:   "http",
		Option: map[string]interface{}{"ip": "127.0.0.1", "port": 0},
	})
	running, err := Start(context.Background(), cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		require.NoError(t, running.Stop(ctx))
	})

	var httpEndpoint string
	for _, endpoint := range running.Endpoints() {
		if strings.HasSuffix(endpoint, "/http") {
			httpEndpoint = strings.TrimSuffix(endpoint, "/http")
		}
	}
	require.NotEmpty(t, httpEndpoint)
	response, err := http.Get("http://" + httpEndpoint + "/maintain/quota-leases")
	require.NoError(t, err)
	defer func() { require.NoError(t, response.Body.Close()) }()
	require.Equal(t, http.StatusOK, response.StatusCode)
	var snapshot struct {
		GeneratedAt int64 `json:"generatedAt"`
		Leases      []any `json:"leases"`
	}
	require.NoError(t, json.NewDecoder(response.Body).Decode(&snapshot))
	require.NotZero(t, snapshot.GeneratedAt)
	require.Empty(t, snapshot.Leases)
}

func testConfig() Config {
	return Config{
		APIServers: []APIServerConfig{{
			Name: "grpc",
			Option: map[string]interface{}{
				"ip":   "127.0.0.1",
				"port": 0,
			},
		}},
		Limit: validLimitConfig(),
	}
}

func validLimitConfig() LimitConfig {
	return LimitConfig{
		NodeID: 1,
	}
}
