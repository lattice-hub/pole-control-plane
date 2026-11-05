package unified

import (
	"strings"
	"testing"

	configv1 "github.com/pole-io/specification/source/go/api/v1/config"
)

func TestAudienceConditionMatching(t *testing.T) {
	cases := []struct {
		name           string
		typ            configv1.MatchType
		value, yes, no string
	}{
		{"exact", configv1.MatchType_MATCH_TYPE_EXACT, "*", "*", "anything"},
		{"regex", configv1.MatchType_MATCH_TYPE_REGEX, "ab+", "zabbz", "ac"},
		{"not equals", configv1.MatchType_MATCH_TYPE_NOT_EQUALS, "", "x", ""},
		{"in", configv1.MatchType_MATCH_TYPE_IN, "a,, b", "", "b"},
		{"not in", configv1.MatchType_MATCH_TYPE_NOT_IN, "a,, b", "b", ""},
		{"range", configv1.MatchType_MATCH_TYPE_RANGE, "-2~4", "-2", "5"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := &configv1.Audience{Conditions: []*configv1.LabelCondition{{Key: "k", Type: tc.typ, Value: tc.value}}}
			if err := validateAudience(a); err != nil {
				t.Fatal(err)
			}
			if !matches(a, map[string]string{"k": tc.yes}) || matches(a, map[string]string{"k": tc.no}) || matches(a, nil) {
				t.Fatal("unexpected match")
			}
		})
	}
	legacy := &configv1.Audience{Labels: map[string]string{"x": "", "y": "*"}}
	if !matches(legacy, map[string]string{"x": "", "y": "*"}) || matches(legacy, map[string]string{"x": ""}) {
		t.Fatal("legacy exact AND")
	}
}

func TestAudienceValidation(t *testing.T) {
	good := func(typ configv1.MatchType, value string) *configv1.LabelCondition {
		return &configv1.LabelCondition{Key: "k", Type: typ, Value: value}
	}
	for _, c := range []*configv1.LabelCondition{nil, good(0, ""), good(99, ""), good(configv1.MatchType_MATCH_TYPE_REGEX, "["), good(configv1.MatchType_MATCH_TYPE_RANGE, "3~2"), good(configv1.MatchType_MATCH_TYPE_RANGE, "0~9223372036854775808"), good(configv1.MatchType_MATCH_TYPE_EXACT, strings.Repeat("x", 4097)), good(configv1.MatchType_MATCH_TYPE_EXACT, "\n")} {
		if validateAudience(&configv1.Audience{Conditions: []*configv1.LabelCondition{c}}) == nil {
			t.Fatalf("accepted invalid condition: %v", c)
		}
	}
	c := good(configv1.MatchType_MATCH_TYPE_EXACT, "")
	for _, a := range []*configv1.Audience{{Labels: map[string]string{"x": "y"}, Conditions: []*configv1.LabelCondition{c}}, {Conditions: []*configv1.LabelCondition{c, c}}, {Conditions: make([]*configv1.LabelCondition, 65)}} {
		if validateAudience(a) == nil {
			t.Fatal("accepted invalid audience")
		}
	}
	if validateRollout(configv1.ReleaseKind_RELEASE_KIND_NORMAL, &configv1.Audience{Conditions: []*configv1.LabelCondition{c}}, 0) == nil {
		t.Fatal("normal accepted conditions")
	}
}

func TestAudienceOverlapConservative(t *testing.T) {
	a := func(typ configv1.MatchType, value string) *configv1.Audience {
		return &configv1.Audience{Conditions: []*configv1.LabelCondition{{Key: "k", Type: typ, Value: value}}}
	}
	cases := []struct {
		x, y    *configv1.Audience
		overlap bool
	}{
		{a(configv1.MatchType_MATCH_TYPE_IN, "a,b"), a(configv1.MatchType_MATCH_TYPE_REGEX, "^c$"), false},
		{a(configv1.MatchType_MATCH_TYPE_EXACT, "a"), a(configv1.MatchType_MATCH_TYPE_NOT_EQUALS, "a"), false},
		{a(configv1.MatchType_MATCH_TYPE_RANGE, "1~2"), a(configv1.MatchType_MATCH_TYPE_RANGE, "3~4"), false},
		{a(configv1.MatchType_MATCH_TYPE_RANGE, "1~3"), a(configv1.MatchType_MATCH_TYPE_RANGE, "3~4"), true},
		{a(configv1.MatchType_MATCH_TYPE_REGEX, "^a"), a(configv1.MatchType_MATCH_TYPE_REGEX, "^b"), true},
	}
	for _, tc := range cases {
		if audiencesOverlap(tc.x, tc.y) != tc.overlap || audiencesOverlap(tc.y, tc.x) != tc.overlap {
			t.Fatalf("unexpected overlap: %v %v", tc.x, tc.y)
		}
	}
}
