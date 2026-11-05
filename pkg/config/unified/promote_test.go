package unified_test

import (
	"context"
	configv1 "github.com/pole-io/specification/source/go/api/v1/config"
	"github.com/pole-io/specification/source/go/api/v1/model"
	"google.golang.org/protobuf/proto"
	"testing"
)

func TestPromoteGrayAtomicSnapshotAndAuthorization(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.save(t, literal([]byte("normal")))
	f.publish(t, 1, 0, nil)
	d := &configv1.ConfigDefinition{Engine: "pole-mustache-v1", Content: []byte("{{{a}}}"), Parameters: []*configv1.Parameter{{Name: "a", Type: configv1.ValueType_VALUE_TYPE_STRING}}, ConditionGroups: []*configv1.NamedConditionGroup{{Id: "g", Name: "G", Audience: &configv1.Audience{Labels: map[string]string{"x": "1"}}}}, ParameterBindings: []*configv1.ParameterBindings{{Name: "a", Fallback: &configv1.Scalar{Type: configv1.ValueType_VALUE_TYPE_STRING, Value: "fallback"}, Bindings: []*configv1.ParameterBinding{{Id: "b", GroupId: "g", Value: &configv1.Scalar{Type: configv1.ValueType_VALUE_TYPE_STRING, Value: "bound"}}}}}}
	f.save(t, d)
	source := f.publish(t, 2, 0, map[string]string{"cohort": "source"})
	f.save(t, literal([]byte("other")))
	other := f.publish(t, 2, 1, map[string]string{"cohort": "other"})
	f.save(t, literal([]byte("draft")))
	before := f.get(t)
	request := &configv1.PromoteGrayRequest{Key: f.key, ReleaseId: source.Id, ExpectedReleaseSetRevision: before.ReleaseSetRevision}
	stale := proto.Clone(request).(*configv1.PromoteGrayRequest)
	stale.ExpectedReleaseSetRevision--
	requireCode(t, f.service.PromoteGray(ctx, stale), model.Code_DataConflict)
	f.auth.denied.Store(true)
	requireCode(t, f.service.PromoteGray(ctx, request), model.Code_AuthTokenForbidden)
	f.auth.denied.Store(false)
	if !proto.Equal(before, f.get(t)) {
		t.Fatal("failed promotion mutated state")
	}
	result := &configv1.ConfigState{}
	unpack(t, f.service.PromoteGray(ctx, request), result)
	if !proto.Equal(before.Draft, result.Draft) {
		t.Fatal("draft changed")
	}
	if len(result.ActiveReleaseIds) != 2 {
		t.Fatalf("active releases: %v", result.ActiveReleaseIds)
	}
	activeOther := false
	for _, id := range result.ActiveReleaseIds {
		if id == other.Id {
			activeOther = true
		}
		if id == source.Id {
			t.Fatal("source gray still active")
		}
	}
	if !activeOther {
		t.Fatal("other gray stopped")
	}
	promoted := f.discover(t, map[string]string{"x": "1"})
	if promoted.Snapshot.Values["a"].Value != "bound" || string(promoted.Snapshot.Content) != "{{{a}}}" {
		t.Fatal(promoted)
	}
	detail := &configv1.ConfigState{}
	unpack(t, f.service.GetState(ctx, &configv1.GetStateRequest{Key: f.key, ReleaseId: promoted.ReleaseId}), detail)
	r := detail.SelectedRelease
	if r.Kind != 1 || r.SourceReleaseId != source.Id || r.Audience != nil || r.Priority != 0 || !proto.Equal(r.Definition, d) {
		t.Fatal(r)
	}
	history := &configv1.ConfigState{}
	unpack(t, f.service.GetState(ctx, &configv1.GetStateRequest{Key: f.key, ReleaseId: source.Id}), history)
	if !proto.Equal(source, history.SelectedRelease) {
		t.Fatal("history changed")
	}
	request.ExpectedReleaseSetRevision = result.ReleaseSetRevision
	requireCode(t, f.service.PromoteGray(ctx, request), model.Code_NotFoundResource)
}
