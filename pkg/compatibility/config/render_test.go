package config

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	configv1 "github.com/pole-io/specification/source/go/api/v1/config"
)

// 固定共享向量副本来自 sibling specification/CONFIG_TEMPLATE_TEST_VECTORS.json，保证独立checkout可运行。
func TestSharedVectors(t *testing.T) {
	data, err := os.ReadFile("testdata/CONFIG_TEMPLATE_TEST_VECTORS.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Vectors []struct {
			Name            string
			Request         json.RawMessage
			Valid           bool
			ExpectedContent []byte
			ExpectedSha256  string
		}
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Vectors) != 12 {
		t.Fatalf("expected 12 shared vectors, got %d", len(fixture.Vectors))
	}
	for _, vector := range fixture.Vectors {
		t.Run(vector.Name, func(t *testing.T) {
			request := &configv1.PreviewRequest{}
			if err := protojson.Unmarshal(vector.Request, request); err != nil {
				t.Fatal(err)
			}
			definition := request.Definition
			values := map[string]*configv1.Scalar{}
			for key, value := range definition.DefaultValues {
				values[key] = value
			}
			var selected *configv1.ValueRule
			ambiguous := false
			for _, rule := range definition.ValueRules {
				match := true
				for key, value := range rule.GetAudience().GetLabels() {
					actual, ok := request.Labels[key]
					if !ok || actual != value {
						match = false
					}
				}
				if !match {
					continue
				}
				if selected == nil || rule.Priority < selected.Priority {
					selected = rule
					ambiguous = false
				} else if rule.Priority == selected.Priority {
					ambiguous = true
				}
			}
			var got []byte
			var err error
			if ambiguous {
				err = errRender
			} else {
				for key, value := range selected.GetValues() {
					values[key] = value
				}
				got, err = Render(context.Background(), &configv1.RenderSnapshot{Engine: definition.Engine, Content: definition.Content, Format: definition.Format, Parameters: definition.Parameters, Values: values})
			}
			if !vector.Valid {
				if err == nil {
					t.Fatal("invalid vector accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, vector.ExpectedContent) {
				t.Fatalf("unexpected bytes %q", got)
			}
			sum := sha256.Sum256(got)
			if hex.EncodeToString(sum[:]) != vector.ExpectedSha256 {
				t.Fatal("sha256 differs")
			}
		})
	}
}

func textSnapshot(content, value string) *configv1.RenderSnapshot {
	return &configv1.RenderSnapshot{Engine: "pole-mustache-v1", Content: []byte(content), Parameters: []*configv1.Parameter{{Name: "x", Type: configv1.ValueType_VALUE_TYPE_STRING}}, Values: map[string]*configv1.Scalar{"x": {Type: configv1.ValueType_VALUE_TYPE_STRING, Value: value}}}
}

func TestRenderBoundaries(t *testing.T) {
	for _, test := range []struct {
		name, body, value, format string
		valid                     bool
	}{
		{"bom-crlf", "\ufeff {{{x}}}\r\n", "你好", "text", true},
		{"double", "{{x}}", "x", "text", false}, {"space", "{{{ x}}}", "x", "text", false}, {"trailing-braces", "{{{x}}}}}", "x", "text", false},
		{"undeclared", "{{{missing}}}", "x", "text", false}, {"json-duplicate", `{"a":1,"a":2}`, "x", "json", false},
		{"json-nested-duplicate", `{"a":{"b":1,"b":2}}`, "x", "json", false}, {"json-unescaped", `{"x":"{{{x}}}"}`, `"`, "json", false},
		{"json-huge-number", `{"n":123456789012345678901234567890}`, "", "JSON", true},
		{"xml-two-roots", "<a/><b/>", "", "xml", false}, {"xml-one-root", "<a>{{{x}}}</a>", "ok", "xml", true},
		{"yaml-duplicate", "a: 1\na: 2", "", "yaml", false}, {"unknown-format", "x", "", "toml", false},
		{"output-limit", strings.Repeat("{{{x}}}", 128), strings.Repeat("x", 4097), "text", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			snapshot := textSnapshot(test.body, test.value)
			snapshot.Format = test.format
			_, err := Render(context.Background(), snapshot)
			if (err == nil) != test.valid {
				t.Fatalf("valid=%v err=%v", test.valid, err)
			}
		})
	}
	snapshot := textSnapshot("{{{x}}}", strings.Repeat("x", MaxBytes-8))
	if _, err := Render(context.Background(), snapshot); err != nil {
		t.Fatal("exact input limit", err)
	}
	snapshot.Values["x"].Value += "x"
	if _, err := Render(context.Background(), snapshot); err == nil {
		t.Fatal("input limit accepted")
	}
	binary := []byte{0, 255, 239, 187, 191, 13, 10}
	got, err := Render(context.Background(), &configv1.RenderSnapshot{Engine: "literal", Content: binary, Format: "anything"})
	if err != nil || !bytes.Equal(got, binary) {
		t.Fatal("binary changed", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Render(ctx, textSnapshot("", "")); err == nil {
		t.Fatal("cancellation ignored")
	}
}

func TestScalarCanonicalText(t *testing.T) {
	for _, test := range []struct {
		kind           configv1.ValueType
		valid, invalid []string
	}{
		{configv1.ValueType_VALUE_TYPE_INTEGER, []string{"0", "-1", "18446744073709551616"}, []string{"-0", "01", "+1", "1.0", "1e3"}},
		{configv1.ValueType_VALUE_TYPE_DECIMAL, []string{"0", "1.25", "-0.01", "18446744073709551616.001"}, []string{"-0", "1.250", "01.25", ".25", "1e3"}},
		{configv1.ValueType_VALUE_TYPE_BOOLEAN, []string{"true", "false"}, []string{"True", "0", ""}},
	} {
		for _, value := range test.valid {
			if !validScalar(&configv1.Scalar{Type: test.kind, Value: value}) {
				t.Errorf("rejected %q", value)
			}
		}
		for _, value := range test.invalid {
			if validScalar(&configv1.Scalar{Type: test.kind, Value: value}) {
				t.Errorf("accepted %q", value)
			}
		}
	}
}

func TestSnapshotDeclarationFailures(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*configv1.RenderSnapshot)
	}{
		{"duplicate", func(s *configv1.RenderSnapshot) { s.Parameters = append(s.Parameters, s.Parameters[0]) }},
		{"unknown-type", func(s *configv1.RenderSnapshot) { s.Parameters[0].Type = 99 }},
		{"unspecified-type", func(s *configv1.RenderSnapshot) { s.Parameters[0].Type = 0 }},
		{"type-mismatch", func(s *configv1.RenderSnapshot) { s.Values["x"].Type = configv1.ValueType_VALUE_TYPE_INTEGER }},
		{"undeclared-value", func(s *configv1.RenderSnapshot) {
			s.Values["other"] = &configv1.Scalar{Type: configv1.ValueType_VALUE_TYPE_STRING}
		}},
		{"invalid-utf8", func(s *configv1.RenderSnapshot) { s.Values["x"].Value = string([]byte{255}) }},
		{"nil-parameter", func(s *configv1.RenderSnapshot) { s.Parameters[0] = nil }},
		{"nil-value", func(s *configv1.RenderSnapshot) { s.Values["x"] = nil }},
	} {
		t.Run(test.name, func(t *testing.T) {
			snapshot := textSnapshot("{{{x}}}", "value")
			test.mutate(snapshot)
			if _, err := Render(context.Background(), snapshot); err == nil {
				t.Fatal("malformed snapshot accepted")
			}
		})
	}
}

func TestPropertiesValidationDoesNotExpandApplicationPlaceholders(t *testing.T) {
	t.Setenv("POLE_TEST_PROPERTIES_ENV", "must-not-expand")
	for _, content := range []string{
		"keyOnly\n",
		"continued=one\\\r\n  two\n",
		"escaped\\ key\\:part=value\n",
		"x=${POLE_TEST_PROPERTIES_ENV}\n",
		"x=${x}\n",
		"x=${y}\ny=${x}\n",
	} {
		t.Run(content, func(t *testing.T) {
			snapshot := &configv1.RenderSnapshot{Engine: "pole-mustache-v1", Format: "properties", Content: []byte(content)}
			actual, err := Render(context.Background(), snapshot)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(actual, []byte(content)) {
				t.Fatalf("properties bytes changed: %q", actual)
			}
		})
	}
	snapshot := textSnapshot("message={{{x}}}\r\n", "${POLE_TEST_PROPERTIES_ENV}")
	snapshot.Format = "properties"
	actual, err := Render(context.Background(), snapshot)
	if err != nil || string(actual) != "message=${POLE_TEST_PROPERTIES_ENV}\r\n" {
		t.Fatalf("unexpected second expansion: %q %v", actual, err)
	}
	invalid := []byte("x=\\uZZZZ\n")
	if _, err := Render(context.Background(), &configv1.RenderSnapshot{Engine: "pole-mustache-v1", Format: "properties", Content: invalid}); err == nil {
		t.Fatal("invalid unicode escape accepted")
	}
	actual, err = Render(context.Background(), &configv1.RenderSnapshot{Engine: "literal", Format: "properties", Content: invalid})
	if err != nil || !bytes.Equal(actual, invalid) {
		t.Fatal("literal applied properties parsing", err)
	}
}
