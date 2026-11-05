package healthcheck

import (
	"errors"
	"testing"

	checkerapi "github.com/pole-io/pole-server/apis/service/healthcheck"
	commonhash "github.com/pole-io/pole-server/pkg/common/utils/hash"
)

type recordingPeerUpdater struct {
	checkerapi.HealthChecker
	calls int
	peers []checkerapi.CheckerPeer
	err   error
}

func (r *recordingPeerUpdater) SetCheckerPeers(peers []checkerapi.CheckerPeer) error {
	r.calls++
	r.peers = append([]checkerapi.CheckerPeer(nil), peers...)
	return r.err
}

func TestSelectCheckerPeersFallsBackToLocalWhenAllUnhealthy(t *testing.T) {
	healthy := map[string]checkerapi.CheckerPeer{}
	local := checkerapi.CheckerPeer{ID: "local", Host: "10.0.0.1", Port: 8091}
	fallback := map[string]checkerapi.CheckerPeer{
		local.ID: local,
	}
	selected := selectCheckerPeers(healthy, fallback)
	if len(selected) != 1 {
		t.Fatalf("selected peer count = %d", len(selected))
	}
	if selected[local.ID] != local {
		t.Fatalf("selected peers = %+v", selected)
	}
}

func TestSelectCheckerPeersPrefersHealthy(t *testing.T) {
	healthyPeer := checkerapi.CheckerPeer{ID: "healthy", Host: "10.0.0.2", Port: 8091}
	healthy := map[string]checkerapi.CheckerPeer{healthyPeer.ID: healthyPeer}
	fallback := map[string]checkerapi.CheckerPeer{
		"local": {ID: "local", Host: "10.0.0.1", Port: 8091},
	}
	selected := selectCheckerPeers(healthy, fallback)
	if len(selected) != 1 || selected[healthyPeer.ID] != healthyPeer {
		t.Fatalf("unexpected selected peers: %+v", selected)
	}
}

func TestCheckerPeerBucketsDeduplicatesPodsByHost(t *testing.T) {
	peers := map[string]checkerapi.CheckerPeer{
		"first":  {ID: "first", Host: "10.0.0.1", Port: 8091},
		"second": {ID: "second", Host: "10.0.0.1", Port: 9091},
	}
	buckets := checkerPeerBuckets(peers)
	expected := commonhash.Bucket{Host: "10.0.0.1", Weight: 100}
	if len(buckets) != 1 || !buckets[expected] {
		t.Fatalf("unexpected buckets: %+v", buckets)
	}
}

func TestDispatcherRefreshesPeerAwareCheckerOnlyWhenMembershipChanges(t *testing.T) {
	updater := &recordingPeerUpdater{}
	dispatcher := &Dispatcher{svr: &Server{checkers: map[int32]checkerapi.HealthChecker{1: updater}}}
	peers := map[string]checkerapi.CheckerPeer{
		"second": {ID: "second", Host: "10.0.0.2", Port: 8091},
		"first":  {ID: "first", Host: "10.0.0.1", Port: 8091},
	}

	dispatcher.refreshCheckerPeers(peers)
	if updater.calls != 1 {
		t.Fatalf("updater calls = %d", updater.calls)
	}
	if len(updater.peers) != 2 || updater.peers[0].ID != "first" || updater.peers[1].ID != "second" {
		t.Fatalf("unexpected ordered peers: %+v", updater.peers)
	}
	dispatcher.refreshCheckerPeers(peers)
	if updater.calls != 1 {
		t.Fatalf("unchanged membership refreshed %d times", updater.calls)
	}
}

func TestDispatcherDoesNotCommitPeerMembershipWhenUpdaterFails(t *testing.T) {
	updater := &recordingPeerUpdater{err: errors.New("peer unavailable")}
	dispatcher := &Dispatcher{svr: &Server{checkers: map[int32]checkerapi.HealthChecker{1: updater}}}
	peers := map[string]checkerapi.CheckerPeer{
		"remote": {ID: "remote", Host: "10.0.0.2", Port: 8091},
	}

	if err := dispatcher.refreshCheckerPeers(peers); err == nil {
		t.Fatal("expected refresh error")
	}
	if len(dispatcher.checkerPeers) != 0 {
		t.Fatalf("failed membership was committed: %+v", dispatcher.checkerPeers)
	}
}
