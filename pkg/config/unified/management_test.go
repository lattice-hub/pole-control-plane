package unified_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	configv1 "github.com/pole-io/specification/source/go/api/v1/config"
	"github.com/pole-io/specification/source/go/api/v1/model"

	"github.com/pole-io/pole-server/apis/store"
	"github.com/pole-io/pole-server/pkg/config/unified"
)

func page(t *testing.T, f *fixture, req *configv1.GetStateRequest) *configv1.ConfigState {
	t.Helper()
	s := &configv1.ConfigState{}
	unpack(t, f.service.GetState(context.Background(), req), s)
	return s
}
func TestManagementPaginationKeepsHistoryBoundedAndDetectsChangedSet(t *testing.T) {
	f := newFixture(t)
	f.save(t, literal(bytes.Repeat([]byte{'x'}, 512*1024)))
	var oldest string
	for i := 0; i < 24; i++ {
		r := f.publish(t, 1, 0, nil)
		if i == 0 {
			oldest = r.Id
		}
	}
	first := page(t, f, &configv1.GetStateRequest{Key: f.key, PageSize: 7, ReleaseId: oldest})
	if len(first.Releases) != 7 || first.NextPageToken == "" || len(first.ActiveReleaseIds) != 1 || len(first.SelectedRelease.Definition.Content) != 512*1024 || proto.Size(first) > 2*1024*1024 {
		t.Fatal("unbounded or incomplete history")
	}
	seen := map[uint64]bool{}
	current := first
	for {
		for _, m := range current.Releases {
			if seen[m.Sequence] {
				t.Fatal("duplicate sequence")
			}
			seen[m.Sequence] = true
		}
		if current.NextPageToken == "" {
			break
		}
		current = page(t, f, &configv1.GetStateRequest{Key: f.key, PageSize: 7, PageToken: current.NextPageToken})
		if current.SelectedRelease != nil || len(current.Releases) == 0 {
			t.Fatal("invalid next page")
		}
	}
	if len(seen) != 24 || first.Releases[0].Sequence != 24 {
		t.Fatal("history skipped")
	}
	f.save(t, literal([]byte("new draft")))
	request := &configv1.GetStateRequest{Key: f.key, PageToken: first.NextPageToken}
	if string(page(t, f, request).Draft.Definition.Content) != "new draft" {
		t.Fatal("stale draft")
	}
	f.publish(t, 1, 0, nil)
	requireCode(t, f.service.GetState(context.Background(), request), model.Code_DataConflict)
	request.Key = &configv1.ConfigKey{Environment: "prod", Group: "checkout", Name: "other"}
	requireCode(t, f.service.GetState(context.Background(), request), model.Code_InvalidParameter)
	f.auth.denied.Store(true)
	requireCode(t, f.service.GetState(context.Background(), &configv1.GetStateRequest{Key: f.key, ReleaseId: oldest}), model.Code_AuthTokenForbidden)
}
func TestManagementMetadataByteLimitDoesNotSkipEntries(t *testing.T) {
	f := newFixture(t)
	f.save(t, literal([]byte("gray")))
	labels := map[string]string{}
	for i := 0; i < 64; i++ {
		labels[fmt.Sprint(i)] = strings.Repeat("x", 1024)
	}
	for i := 0; i < 6; i++ {
		f.publish(t, 2, int32(i), labels)
	}
	current := page(t, f, &configv1.GetStateRequest{Key: f.key, PageSize: 100})
	if len(current.Releases) >= 6 || len(current.Releases) == 0 || len(current.ActiveReleaseIds) != 6 {
		t.Fatal("byte budget or active collection wrong")
	}
	seen := map[string]bool{}
	for {
		if proto.Size(current) > 260*1024 {
			t.Fatal("page too large")
		}
		for _, m := range current.Releases {
			if seen[m.Id] {
				t.Fatal("duplicate")
			}
			seen[m.Id] = true
		}
		if current.NextPageToken == "" {
			break
		}
		current = page(t, f, &configv1.GetStateRequest{Key: f.key, PageSize: 100, PageToken: current.NextPageToken})
	}
	if len(seen) != 6 {
		t.Fatal("byte truncation skipped entries")
	}
}

type runtimeOnlyRepository struct{ *memoryRepository }

func (r runtimeOnlyRepository) ReadUnifiedConfig(context.Context, *configv1.ConfigKey) (*store.UnifiedConfigSnapshot, error) {
	return nil, errors.New("full history read not allowed")
}
func (r runtimeOnlyRepository) ReadUnifiedConfigRuntime(ctx context.Context, key *configv1.ConfigKey) (*store.UnifiedConfigSnapshot, error) {
	return r.memoryRepository.ReadUnifiedConfig(ctx, key)
}
func TestDiscoverUsesRuntimeReader(t *testing.T) {
	f := newFixture(t)
	f.save(t, literal([]byte("live")))
	f.publish(t, 1, 0, nil)
	service, err := unified.New(runtimeOnlyRepository{f.repository}, f.auth)
	if err != nil {
		t.Fatal(err)
	}
	result := &configv1.ConfigResult{}
	unpack(t, service.Discover(context.Background(), &configv1.DiscoverRequest{Key: f.key}), result)
	if string(result.Snapshot.Content) != "live" {
		t.Fatal(result)
	}
}
func TestActiveGrayLimitRejectsAppendButAllowsReplacement(t *testing.T) {
	f := newFixture(t)
	f.save(t, literal([]byte("gray")))
	var target string
	for i := 0; i < 128; i++ {
		target = f.publish(t, 2, int32(i), map[string]string{"cohort": "test"}).Id
	}
	before := f.discover(t, nil)
	state := f.get(t)
	req := &configv1.PublishRequest{Key: f.key, ExpectedDraftRevision: state.Draft.Revision, ExpectedReleaseSetRevision: state.ReleaseSetRevision, Kind: 2, Audience: &configv1.Audience{Labels: map[string]string{"cohort": "other"}}}
	requireCode(t, f.service.Publish(context.Background(), req), model.Code_InvalidParameter)
	if !proto.Equal(before, f.discover(t, nil)) {
		t.Fatal("failed append changed runtime")
	}
	req.ReplaceReleaseId = target
	result := &configv1.ConfigState{}
	unpack(t, f.service.Publish(context.Background(), req), result)
	if len(result.ActiveReleaseIds) != 128 || result.ReleaseSetRevision != state.ReleaseSetRevision+1 {
		t.Fatal("replacement failed at capacity")
	}
}
