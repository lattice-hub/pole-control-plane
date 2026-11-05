package unified

import (
	"context"
	"testing"

	configv1 "github.com/pole-io/specification/source/go/api/v1/config"
	"google.golang.org/protobuf/proto"
)

func TestParameterBindingsResolutionAndValidation(t *testing.T) {
	scalar := func(v string) *configv1.Scalar {
		return &configv1.Scalar{Type: configv1.ValueType_VALUE_TYPE_STRING, Value: v}
	}
	def := &configv1.ConfigDefinition{Engine: "pole-mustache-v1", Content: []byte("{{{a}}}"), Parameters: []*configv1.Parameter{{Name: "a", Type: configv1.ValueType_VALUE_TYPE_STRING}, {Name: "b", Type: configv1.ValueType_VALUE_TYPE_STRING}, {Name: "c", Type: configv1.ValueType_VALUE_TYPE_STRING}},
		ConditionGroups:   []*configv1.NamedConditionGroup{{Id: "one", Name: "One", Audience: &configv1.Audience{Labels: map[string]string{"region": "east"}}}, {Id: "two", Name: "Two", Audience: &configv1.Audience{Labels: map[string]string{"tier": "gold"}}}},
		ParameterBindings: []*configv1.ParameterBindings{{Name: "a", Fallback: scalar(""), Bindings: []*configv1.ParameterBinding{{Id: "a1", GroupId: "one", Value: scalar("first")}, {Id: "a2", GroupId: "two", Value: scalar("second")}}}, {Name: "b", Bindings: []*configv1.ParameterBinding{{Id: "b1", GroupId: "two", Value: scalar("other")}}}, {Name: "c"}}}
	if err := validateDefinition(context.Background(), def); err != nil {
		t.Fatal(err)
	}
	values, err := effectiveValues(def, map[string]string{"region": "east", "tier": "gold"})
	if err != nil || values["a"].GetValue() != "first" || values["b"].GetValue() != "other" {
		t.Fatalf("unexpected independent resolution: %v %v", values, err)
	}
	if _, ok := values["c"]; ok {
		t.Fatal("missing optional key returned")
	}
	values, err = effectiveValues(def, nil)
	if err != nil || values["a"] == nil || values["a"].Value != "" {
		t.Fatalf("empty fallback lost: %v %v", values, err)
	}
	if _, ok := values["b"]; ok {
		t.Fatal("missing fallback returned")
	}
	cases := map[string]func(*configv1.ConfigDefinition){"mixed": func(d *configv1.ConfigDefinition) { d.DefaultValues = map[string]*configv1.Scalar{"a": scalar("x")} }, "missing group": func(d *configv1.ConfigDefinition) { d.ParameterBindings[0].Bindings[0].GroupId = "missing" }, "missing required fallback": func(d *configv1.ConfigDefinition) { d.ParameterBindings[0].Fallback = nil }, "duplicate group": func(d *configv1.ConfigDefinition) { d.ConditionGroups[1].Id = "one" }, "wrong type": func(d *configv1.ConfigDefinition) {
		d.ParameterBindings[0].Bindings[0].Value.Type = configv1.ValueType_VALUE_TYPE_INTEGER
	}}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			copy := proto.Clone(def).(*configv1.ConfigDefinition)
			mutate(copy)
			if validateDefinition(context.Background(), copy) == nil {
				t.Fatal("expected rejection")
			}
		})
	}
}

func TestLegacyValueRuleStillChoosesSingleWinner(t *testing.T) {
	s := func(v string) *configv1.Scalar {
		return &configv1.Scalar{Type: configv1.ValueType_VALUE_TYPE_STRING, Value: v}
	}
	d := &configv1.ConfigDefinition{DefaultValues: map[string]*configv1.Scalar{"b": s("fallback")}, ValueRules: []*configv1.ValueRule{{Id: "one", Priority: 0, Audience: &configv1.Audience{Labels: map[string]string{"x": "1"}}, Values: map[string]*configv1.Scalar{"a": s("winner")}}, {Id: "two", Priority: 1, Audience: &configv1.Audience{Labels: map[string]string{"y": "1"}}, Values: map[string]*configv1.Scalar{"b": s("loser")}}}}
	values, err := effectiveValues(d, map[string]string{"x": "1", "y": "1"})
	if err != nil || values["a"].Value != "winner" || values["b"].Value != "fallback" {
		t.Fatalf("legacy winner changed: %v %v", values, err)
	}
}
