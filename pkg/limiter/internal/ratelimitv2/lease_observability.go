package ratelimitv2

import (
	"sort"
	"strings"

	apiv2 "github.com/pole-io/specification/source/go/api/v1/traffic_manage/ratelimiter"

	"github.com/pole-io/pole-server/pkg/limiter/internal/utils"
)

type QuotaLeaseSnapshot struct {
	GeneratedAt int64                   `json:"generatedAt"`
	Summary     QuotaLeaseSnapshotStats `json:"summary"`
	Leases      []QuotaLeaseView        `json:"leases"`
}

type QuotaLeaseSnapshotStats struct {
	ActiveCount      int    `json:"activeCount"`
	SettledCount     int    `json:"settledCount"`
	ExpiredCount     int    `json:"expiredCount"`
	ActiveReserved   uint64 `json:"activeReserved"`
	ActiveConsumed   uint64 `json:"activeConsumed"`
	RecentlyReturned uint64 `json:"recentlyReturned"`
}

type QuotaLeaseView struct {
	ID         string                  `json:"id"`
	ClientKey  uint32                  `json:"clientKey"`
	Status     string                  `json:"status"`
	Sequence   uint64                  `json:"sequence"`
	ExpiresAt  int64                   `json:"expiresAt"`
	TerminalAt int64                   `json:"terminalAt,omitempty"`
	Namespace  string                  `json:"namespace"`
	Service    string                  `json:"service"`
	Counters   []QuotaLeaseCounterView `json:"counters"`
}

type QuotaLeaseCounterView struct {
	CounterKey uint32 `json:"counterKey"`
	Labels     string `json:"labels,omitempty"`
	DurationMs int64  `json:"durationMs"`
	Accounting string `json:"accounting"`
	Limit      uint32 `json:"limit"`
	Reserved   uint32 `json:"reserved"`
	Consumed   uint32 `json:"consumed"`
	Returned   uint32 `json:"returned"`
	Available  int64  `json:"available"`
}

func (s *Server) QuotaLeaseSnapshot(namespace, service string) QuotaLeaseSnapshot {
	nowMs := utils.CurrentMillisecond()
	s.leaseMutex.Lock()
	defer s.leaseMutex.Unlock()
	s.cleanupQuotaLeasesLocked(nowMs)

	snapshot := QuotaLeaseSnapshot{GeneratedAt: nowMs, Leases: make([]QuotaLeaseView, 0, len(s.quotaLeases))}
	for _, lease := range s.quotaLeases {
		view, matches := s.quotaLeaseViewLocked(lease, nowMs, namespace, service)
		if !matches {
			continue
		}
		snapshot.Leases = append(snapshot.Leases, view)
		switch lease.status {
		case leaseActive:
			snapshot.Summary.ActiveCount++
			for _, counter := range view.Counters {
				snapshot.Summary.ActiveReserved += uint64(counter.Reserved)
				snapshot.Summary.ActiveConsumed += uint64(counter.Consumed)
			}
		case leaseSettled:
			snapshot.Summary.SettledCount++
			for _, counter := range view.Counters {
				snapshot.Summary.RecentlyReturned += uint64(counter.Returned)
			}
		case leaseExpired:
			snapshot.Summary.ExpiredCount++
			for _, counter := range view.Counters {
				snapshot.Summary.RecentlyReturned += uint64(counter.Returned)
			}
		}
	}
	sort.Slice(snapshot.Leases, func(i, j int) bool {
		if snapshot.Leases[i].Status != snapshot.Leases[j].Status {
			return snapshot.Leases[i].Status < snapshot.Leases[j].Status
		}
		return maxInt64(snapshot.Leases[i].TerminalAt, snapshot.Leases[i].ExpiresAt) >
			maxInt64(snapshot.Leases[j].TerminalAt, snapshot.Leases[j].ExpiresAt)
	})
	return snapshot
}

func (s *Server) quotaLeaseViewLocked(lease *quotaLease, nowMs int64, namespace, service string) (QuotaLeaseView, bool) {
	view := QuotaLeaseView{
		ID: lease.id, ClientKey: lease.clientKey, Status: lease.status.String(), Sequence: lease.sequence,
		ExpiresAt: lease.expiresAt, TerminalAt: lease.terminalAt, Counters: make([]QuotaLeaseCounterView, 0, len(lease.counterKeys)),
	}
	for _, counterKey := range lease.counterKeys {
		entry := lease.counters[counterKey]
		identifier := entry.counter.Identifier()
		if view.Namespace == "" {
			view.Namespace = identifier.Namespace
			view.Service = identifier.Service
		}
		returned := uint32(0)
		reserved := uint32(0)
		if lease.status == leaseActive {
			reserved = entry.amount
			if entry.accounting == apiv2.QuotaAccounting_CONSUMABLE {
				reserved -= entry.consumed
			}
		} else {
			returned = entry.amount
			if entry.accounting == apiv2.QuotaAccounting_CONSUMABLE {
				returned -= entry.consumed
			}
		}
		left := entry.counter.SumQuota(nowMs).GetLeft() - int64(s.reservedByCounter[counterKey])
		view.Counters = append(view.Counters, QuotaLeaseCounterView{
			CounterKey: s.boxCounterKey(counterKey), Labels: identifier.Labels,
			DurationMs: identifier.Duration.Milliseconds(), Accounting: entry.accounting.String(),
			Limit: entry.counter.MaxAmount(), Reserved: reserved, Consumed: entry.consumed,
			Returned: returned, Available: maxInt64(left, 0),
		})
	}
	return view, (strings.TrimSpace(namespace) == "" || view.Namespace == namespace) &&
		(strings.TrimSpace(service) == "" || view.Service == service)
}

func (status leaseStatus) String() string {
	switch status {
	case leaseActive:
		return "ACTIVE"
	case leaseSettled:
		return "SETTLED"
	case leaseExpired:
		return "EXPIRED"
	default:
		return "UNKNOWN"
	}
}
