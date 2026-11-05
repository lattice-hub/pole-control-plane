package ratelimitv2_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	apiv2 "github.com/pole-io/specification/source/go/api/v1/traffic_manage/ratelimiter"

	limiterapi "github.com/pole-io/pole-server/pkg/limiter/internal/api/v2"
	"github.com/pole-io/pole-server/pkg/limiter/internal/config"
	"github.com/pole-io/pole-server/pkg/limiter/internal/ratelimitv2"
	"github.com/pole-io/pole-server/pkg/limiter/internal/statistics/echo"
	"github.com/pole-io/pole-server/pkg/limiter/internal/utils"
)

type testStream struct{}

func (testStream) Send(*apiv2.RateLimitResponse) error {
	return nil
}

type leaseHarness struct {
	server   *ratelimitv2.Server
	client   ratelimitv2.Client
	counters []uint32
}

func TestServerReserveIsAtomicAcrossCounters(t *testing.T) {
	harness := newLeaseHarness(t,
		quotaTotal(60, 10, apiv2.QuotaAccounting_CONSUMABLE),
		quotaTotal(120, 5, apiv2.QuotaAccounting_CONSUMABLE),
	)

	failed := harness.reserve("atomic-failure", 30,
		reservation(harness.counters[0], 4),
		reservation(harness.counters[1], 6),
	)
	require.Equal(t, uint32(limiterapi.QuotaExceeded), failed.GetCode())

	succeeded := harness.reserve("after-atomic-failure", 30,
		reservation(harness.counters[0], 10),
	)
	require.Equal(t, uint32(limiterapi.ExecuteSuccess), succeeded.GetCode())
}

func TestServerReserveIsIdempotent(t *testing.T) {
	harness := newLeaseHarness(t, quotaTotal(60, 10, apiv2.QuotaAccounting_CONSUMABLE))

	first := harness.reserve("same-request", 30, reservation(harness.counters[0], 4))
	second := harness.reserve("same-request", 30, reservation(harness.counters[0], 4))
	require.Equal(t, uint32(limiterapi.ExecuteSuccess), first.GetCode())
	require.Equal(t, first.GetLeaseId(), second.GetLeaseId())

	require.Equal(t, uint32(limiterapi.ExecuteSuccess),
		harness.reserve("remaining", 30, reservation(harness.counters[0], 6)).GetCode())
	require.Equal(t, uint32(limiterapi.QuotaExceeded),
		harness.reserve("overbook", 30, reservation(harness.counters[0], 1)).GetCode())
}

func TestServerUpdateIsIdempotentPerCounter(t *testing.T) {
	harness := newLeaseHarness(t, quotaTotal(60, 10, apiv2.QuotaAccounting_CONSUMABLE))
	lease := harness.reserve("update", 30, reservation(harness.counters[0], 10))
	require.Equal(t, uint32(limiterapi.ExecuteSuccess), lease.GetCode())

	request := &apiv2.QuotaUpdateRequest{
		ClientKey: harness.client.ClientKey(),
		LeaseId:   lease.GetLeaseId(),
		Sequence:  1,
		Consumptions: []*apiv2.QuotaConsumption{
			consumption(harness.counters[0], 4),
		},
	}
	first := harness.server.UpdateQuota(harness.client, request)
	second := harness.server.UpdateQuota(harness.client, request)
	require.Equal(t, uint32(limiterapi.ExecuteSuccess), first.GetCode())
	require.Equal(t, first.GetConsumptions(), second.GetConsumptions())
	require.Equal(t, uint32(limiterapi.InvalidSequence), harness.server.UpdateQuota(
		harness.client,
		&apiv2.QuotaUpdateRequest{
			ClientKey: harness.client.ClientKey(),
			LeaseId:   lease.GetLeaseId(),
			Sequence:  1,
			Consumptions: []*apiv2.QuotaConsumption{
				consumption(harness.counters[0], 5),
			},
		},
	).GetCode())
	require.Equal(t, uint32(limiterapi.InvalidConsumedTotal), harness.server.UpdateQuota(
		harness.client,
		&apiv2.QuotaUpdateRequest{
			ClientKey: harness.client.ClientKey(),
			LeaseId:   lease.GetLeaseId(),
			Sequence:  2,
			Consumptions: []*apiv2.QuotaConsumption{
				consumption(harness.counters[0], 3),
			},
		},
	).GetCode())

	settled := harness.server.SettleQuota(harness.client, &apiv2.QuotaSettleRequest{
		ClientKey: harness.client.ClientKey(),
		LeaseId:   lease.GetLeaseId(),
		Sequence:  2,
	})
	require.Equal(t, uint32(limiterapi.ExecuteSuccess), settled.GetCode())
	require.Equal(t, uint32(6), settled.GetSettlements()[0].GetReturnedAmount())
	duplicateSettle := harness.server.SettleQuota(harness.client, &apiv2.QuotaSettleRequest{
		ClientKey: harness.client.ClientKey(),
		LeaseId:   lease.GetLeaseId(),
		Sequence:  2,
	})
	require.Equal(t, settled.GetSettlements(), duplicateSettle.GetSettlements())
	require.Equal(t, uint32(limiterapi.ExecuteSuccess),
		harness.reserve("remaining-after-update", 30, reservation(harness.counters[0], 6)).GetCode())
}

func TestServerRejectsOutOfBoundsUpdate(t *testing.T) {
	harness := newLeaseHarness(t, quotaTotal(60, 10, apiv2.QuotaAccounting_CONSUMABLE))
	lease := harness.reserve("bounded", 30, reservation(harness.counters[0], 5))

	updated := harness.server.UpdateQuota(harness.client, &apiv2.QuotaUpdateRequest{
		ClientKey: harness.client.ClientKey(),
		LeaseId:   lease.GetLeaseId(),
		Sequence:  1,
		Consumptions: []*apiv2.QuotaConsumption{
			consumption(harness.counters[0], 6),
		},
	})
	require.Equal(t, uint32(limiterapi.InvalidConsumedTotal), updated.GetCode())

	settled := harness.server.SettleQuota(harness.client, &apiv2.QuotaSettleRequest{
		ClientKey: harness.client.ClientKey(),
		LeaseId:   lease.GetLeaseId(),
		Sequence:  1,
	})
	require.Equal(t, uint32(limiterapi.ExecuteSuccess), settled.GetCode())
	require.Equal(t, uint32(5), settled.GetSettlements()[0].GetReturnedAmount())
	require.Equal(t, uint32(limiterapi.ExecuteSuccess),
		harness.reserve("all-returned", 30, reservation(harness.counters[0], 10)).GetCode())
}

func TestServerSettleCommitsFinalPerCounterTotals(t *testing.T) {
	harness := newLeaseHarness(t,
		quotaTotal(60, 1, apiv2.QuotaAccounting_CONSUMABLE),
		quotaTotal(120, 10, apiv2.QuotaAccounting_CONSUMABLE),
		quotaTotal(180, 1, apiv2.QuotaAccounting_OCCUPANCY),
	)
	lease := harness.reserve("mixed", 30,
		reservation(harness.counters[0], 1),
		reservation(harness.counters[1], 10),
		reservation(harness.counters[2], 1),
	)
	require.Equal(t, uint32(limiterapi.ExecuteSuccess), lease.GetCode())

	updated := harness.server.UpdateQuota(harness.client, &apiv2.QuotaUpdateRequest{
		ClientKey: harness.client.ClientKey(),
		LeaseId:   lease.GetLeaseId(),
		Sequence:  1,
		Consumptions: []*apiv2.QuotaConsumption{
			consumption(harness.counters[0], 1),
			consumption(harness.counters[1], 4),
			consumption(harness.counters[2], 1),
		},
	})
	require.Equal(t, uint32(limiterapi.ExecuteSuccess), updated.GetCode())

	settled := harness.server.SettleQuota(harness.client, &apiv2.QuotaSettleRequest{
		ClientKey: harness.client.ClientKey(),
		LeaseId:   lease.GetLeaseId(),
		Sequence:  2,
		Consumptions: []*apiv2.QuotaConsumption{
			consumption(harness.counters[1], 6),
		},
	})
	require.Equal(t, uint32(limiterapi.ExecuteSuccess), settled.GetCode())
	require.Equal(t, []*apiv2.QuotaSettlement{
		{CounterKey: harness.counters[0], ConsumedTotal: 1, ReturnedAmount: 0},
		{CounterKey: harness.counters[1], ConsumedTotal: 6, ReturnedAmount: 4},
		{CounterKey: harness.counters[2], ConsumedTotal: 1, ReturnedAmount: 1},
	}, settled.GetSettlements())

	require.Equal(t, uint32(limiterapi.ExecuteSuccess), harness.reserve("tpm-left", 30,
		reservation(harness.counters[1], 4),
		reservation(harness.counters[2], 1),
	).GetCode())
	require.Equal(t, uint32(limiterapi.QuotaExceeded), harness.reserve("rpm-spent", 30,
		reservation(harness.counters[0], 1),
	).GetCode())
}

func TestServerOccupancyReturnsAllOnSettle(t *testing.T) {
	harness := newLeaseHarness(t, quotaTotal(60, 1, apiv2.QuotaAccounting_OCCUPANCY))
	lease := harness.reserve("occupancy", 30, reservation(harness.counters[0], 1))

	updated := harness.server.UpdateQuota(harness.client, &apiv2.QuotaUpdateRequest{
		ClientKey: harness.client.ClientKey(),
		LeaseId:   lease.GetLeaseId(),
		Sequence:  1,
		Consumptions: []*apiv2.QuotaConsumption{
			consumption(harness.counters[0], 1),
		},
	})
	require.Equal(t, uint32(limiterapi.ExecuteSuccess), updated.GetCode())
	require.Equal(t, uint32(limiterapi.QuotaExceeded),
		harness.reserve("still-occupied", 30, reservation(harness.counters[0], 1)).GetCode())

	settled := harness.server.SettleQuota(harness.client, &apiv2.QuotaSettleRequest{
		ClientKey: harness.client.ClientKey(),
		LeaseId:   lease.GetLeaseId(),
		Sequence:  2,
	})
	require.Equal(t, uint32(limiterapi.ExecuteSuccess), settled.GetCode())
	require.Equal(t, uint32(1), settled.GetSettlements()[0].GetReturnedAmount())
	require.Equal(t, uint32(limiterapi.ExecuteSuccess),
		harness.reserve("released", 30, reservation(harness.counters[0], 1)).GetCode())
}

func TestServerExpiresLeaseAndReturnsReservation(t *testing.T) {
	harness := newLeaseHarness(t, quotaTotal(60, 5, apiv2.QuotaAccounting_CONSUMABLE))
	lease := harness.reserve("expire", 1, reservation(harness.counters[0], 5))
	require.Equal(t, uint32(limiterapi.ExecuteSuccess), lease.GetCode())

	require.Equal(t, 1, harness.server.ExpireQuotaLeases(time.Now().Add(2*time.Second)))
	require.Equal(t, uint32(limiterapi.ExecuteSuccess),
		harness.reserve("after-expire", 30, reservation(harness.counters[0], 5)).GetCode())

	updated := harness.server.UpdateQuota(harness.client, &apiv2.QuotaUpdateRequest{
		ClientKey: harness.client.ClientKey(),
		LeaseId:   lease.GetLeaseId(),
		Sequence:  1,
	})
	require.Equal(t, uint32(limiterapi.LeaseExpired), updated.GetCode())
}

func TestQuotaLeaseSnapshotShowsActiveAndSettledConsumption(t *testing.T) {
	harness := newLeaseHarness(t, quotaTotal(60, 10, apiv2.QuotaAccounting_CONSUMABLE))
	lease := harness.reserve("snapshot", 30, reservation(harness.counters[0], 10))
	require.Equal(t, uint32(limiterapi.ExecuteSuccess), lease.GetCode())

	require.Equal(t, uint32(limiterapi.ExecuteSuccess), harness.server.UpdateQuota(
		harness.client,
		&apiv2.QuotaUpdateRequest{
			ClientKey: harness.client.ClientKey(), LeaseId: lease.GetLeaseId(), Sequence: 1,
			Consumptions: []*apiv2.QuotaConsumption{consumption(harness.counters[0], 4)},
		},
	).GetCode())

	active := harness.server.QuotaLeaseSnapshot("test", "llm")
	require.Equal(t, 1, active.Summary.ActiveCount)
	require.Equal(t, uint64(6), active.Summary.ActiveReserved)
	require.Equal(t, uint64(4), active.Summary.ActiveConsumed)
	require.Len(t, active.Leases, 1)
	require.Equal(t, "ACTIVE", active.Leases[0].Status)
	require.Equal(t, uint32(6), active.Leases[0].Counters[0].Reserved)
	require.Equal(t, int64(0), active.Leases[0].Counters[0].Available)
	require.Empty(t, harness.server.QuotaLeaseSnapshot("other", "llm").Leases)

	require.Equal(t, uint32(limiterapi.ExecuteSuccess), harness.server.SettleQuota(
		harness.client,
		&apiv2.QuotaSettleRequest{ClientKey: harness.client.ClientKey(), LeaseId: lease.GetLeaseId(), Sequence: 2},
	).GetCode())
	settled := harness.server.QuotaLeaseSnapshot("test", "llm")
	require.Equal(t, 0, settled.Summary.ActiveCount)
	require.Equal(t, 1, settled.Summary.SettledCount)
	require.Equal(t, uint64(6), settled.Summary.RecentlyReturned)
	require.Equal(t, "SETTLED", settled.Leases[0].Status)
	require.Equal(t, uint32(6), settled.Leases[0].Counters[0].Returned)
}

func newLeaseHarness(t *testing.T, totals ...*apiv2.QuotaTotal) leaseHarness {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	statics := &echo.StaticsWorker{}
	require.NoError(t, statics.Initialize(nil))
	server, err := ratelimitv2.NewServer(ctx, &config.Config{
		NodeID:               1,
		MaxCounter:           100,
		MaxClient:            10,
		SlideCount:           1,
		PurgeCounterInterval: time.Hour,
	}, statics)
	require.NoError(t, err)
	client := ratelimitv2.NewClient(
		1,
		utils.NewIPAddress("127.0.0.1"),
		"lease-test-client",
		ratelimitv2.NewStreamContext(testStream{}),
		statics,
	)
	response, _ := server.InitializeQuota(ctx, client, &apiv2.RateLimitInitRequest{
		Target: &apiv2.LimitTarget{
			Namespace: "test",
			Service:   "llm",
		},
		ClientId:   client.ClientId(),
		Totals:     totals,
		SlideCount: 1,
		Mode:       apiv2.Mode_BATCH_OCCUPY,
	})
	require.Equal(t, uint32(limiterapi.ExecuteSuccess), response.GetCode())
	counters := make([]uint32, 0, len(response.GetCounters()))
	for _, counter := range response.GetCounters() {
		counters = append(counters, counter.GetCounterKey())
	}
	return leaseHarness{server: server, client: client, counters: counters}
}

func (h leaseHarness) reserve(
	idempotencyKey string, ttlSeconds uint32, reservations ...*apiv2.QuotaReservation,
) *apiv2.QuotaReserveResponse {
	return h.server.ReserveQuota(h.client, &apiv2.QuotaReserveRequest{
		ClientKey:      h.client.ClientKey(),
		IdempotencyKey: idempotencyKey,
		Reservations:   reservations,
		TtlSeconds:     ttlSeconds,
	})
}

func quotaTotal(duration, maxAmount uint32, accounting apiv2.QuotaAccounting) *apiv2.QuotaTotal {
	return &apiv2.QuotaTotal{
		Mode:       apiv2.QuotaMode_WHOLE,
		Duration:   duration,
		MaxAmount:  maxAmount,
		Accounting: accounting,
	}
}

func reservation(counterKey, amount uint32) *apiv2.QuotaReservation {
	return &apiv2.QuotaReservation{CounterKey: counterKey, Amount: amount}
}

func consumption(counterKey, consumedTotal uint32) *apiv2.QuotaConsumption {
	return &apiv2.QuotaConsumption{CounterKey: counterKey, ConsumedTotal: consumedTotal}
}
