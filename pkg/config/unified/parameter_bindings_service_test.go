package unified_test

import (
	"context"
	"testing"

	configv1 "github.com/pole-io/specification/source/go/api/v1/config"
	"google.golang.org/protobuf/proto"
)

func TestParameterBindingsServiceLifecycle(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	scalar := func(v string) *configv1.Scalar {
		return &configv1.Scalar{Type: configv1.ValueType_VALUE_TYPE_STRING, Value: v}
	}
	d := &configv1.ConfigDefinition{Engine: "pole-mustache-v1", Content: []byte("{{{a}}}"), Parameters: []*configv1.Parameter{{Name: "a", Type: configv1.ValueType_VALUE_TYPE_STRING}, {Name: "b", Type: configv1.ValueType_VALUE_TYPE_STRING}, {Name: "c", Type: configv1.ValueType_VALUE_TYPE_STRING}}, ConditionGroups: []*configv1.NamedConditionGroup{{Id: "region", Name: "Region", Audience: &configv1.Audience{Labels: map[string]string{"region": "east"}}}, {Id: "tier", Name: "Tier", Audience: &configv1.Audience{Labels: map[string]string{"tier": "gold"}}}}, ParameterBindings: []*configv1.ParameterBindings{{Name: "a", Fallback: scalar(""), Bindings: []*configv1.ParameterBinding{{Id: "a1", GroupId: "region", Value: scalar("east")}}}, {Name: "b", Bindings: []*configv1.ParameterBinding{{Id: "b1", GroupId: "tier", Value: scalar("gold")}}}, {Name: "c"}}}
	saved := f.save(t, d)
	if !proto.Equal(saved.Draft.Definition, d) {
		t.Fatal("save lost definition")
	}
	check := func(snapshot *configv1.RenderSnapshot, a, b string, wantB bool) {
		t.Helper()
		if snapshot.Values["a"] == nil || snapshot.Values["a"].Value != a {
			t.Fatalf("unexpected a: %v", snapshot)
		}
		v, ok := snapshot.Values["b"]
		if ok != wantB || (ok && v.Value != b) {
			t.Fatalf("unexpected b: %v", snapshot)
		}
		if _, ok := snapshot.Values["c"]; ok {
			t.Fatal("optional c unexpectedly present")
		}
	}
	labels := map[string]string{"region": "east", "tier": "gold"}
	preview := &configv1.RenderSnapshot{}
	unpack(t, f.service.Preview(ctx, &configv1.PreviewRequest{Key: f.key, Definition: d, Labels: labels}), preview)
	check(preview, "east", "gold", true)
	normal := f.publish(t, configv1.ReleaseKind_RELEASE_KIND_NORMAL, 0, nil)
	check(f.discover(t, labels).Snapshot, "east", "gold", true)
	check(f.discover(t, nil).Snapshot, "", "", false)
	changed := proto.Clone(d).(*configv1.ConfigDefinition)
	changed.ParameterBindings[0].Bindings[0].Value.Value = "changed"
	f.save(t, changed)
	current := f.publish(t, configv1.ReleaseKind_RELEASE_KIND_NORMAL, 0, nil)
	check(f.discover(t, labels).Snapshot, "changed", "gold", true)
	history := &configv1.ConfigState{}
	unpack(t, f.service.GetState(ctx, &configv1.GetStateRequest{Key: f.key, ReleaseId: normal.Id}), history)
	if !proto.Equal(history.SelectedRelease.Definition, d) {
		t.Fatal("history mutated")
	}
	state := f.get(t)
	unpack(t, f.service.Rollback(ctx, &configv1.RollbackRequest{Key: f.key, SourceReleaseId: normal.Id, ReplaceReleaseId: current.Id, ExpectedReleaseSetRevision: state.ReleaseSetRevision}), &configv1.ConfigState{})
	check(f.discover(t, labels).Snapshot, "east", "gold", true)
}
