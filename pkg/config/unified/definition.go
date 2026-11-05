package unified

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"google.golang.org/protobuf/proto"

	configv1 "github.com/pole-io/specification/source/go/api/v1/config"
)

const (
	maxContentBytes    = 512 * 1024
	maxDefinitionBytes = 1024 * 1024
	maxRules           = 128
	maxParameters      = 128
	maxLabels          = 64
)

func validateLabels(labels map[string]string) error {
	if len(labels) > maxLabels {
		return invalid("at most 64 labels are supported")
	}
	for key, value := range labels {
		if key == "" || len(key) > 128 || len(value) > 1024 || !utf8.ValidString(key) || !utf8.ValidString(value) || strings.ContainsAny(key+value, "\x00\r\n") {
			return invalid("invalid label key or value")
		}
	}
	return nil
}

func validateAudience(audience *configv1.Audience) error {
	if len(audience.GetLabels()) > 0 && len(audience.GetConditions()) > 0 {
		return invalid("audience labels and conditions cannot be combined")
	}
	if len(audience.GetLabels()) > 0 {
		return validateLabels(audience.GetLabels())
	}
	conditions := audience.GetConditions()
	if len(conditions) == 0 || len(conditions) > maxLabels {
		return invalid("audience requires between 1 and 64 conditions")
	}
	seen := make(map[string]bool, len(conditions))
	for _, c := range conditions {
		if c == nil || c.Key == "" || len(c.Key) > 128 || len(c.Value) > 4096 || !utf8.ValidString(c.Key) || !utf8.ValidString(c.Value) || strings.ContainsAny(c.Key+c.Value, "\x00\r\n") || seen[c.Key] {
			return invalid("invalid or duplicate audience condition")
		}
		seen[c.Key] = true
		switch c.Type {
		case configv1.MatchType_MATCH_TYPE_EXACT, configv1.MatchType_MATCH_TYPE_NOT_EQUALS, configv1.MatchType_MATCH_TYPE_IN, configv1.MatchType_MATCH_TYPE_NOT_IN:
		case configv1.MatchType_MATCH_TYPE_REGEX:
			if _, err := regexp.Compile(c.Value); err != nil {
				return invalid("invalid audience regular expression")
			}
		case configv1.MatchType_MATCH_TYPE_RANGE:
			if _, _, ok := audienceRange(c.Value); !ok {
				return invalid("invalid audience range")
			}
		default:
			return invalid("invalid audience match type")
		}
	}
	return nil
}

func validateRollout(kind configv1.ReleaseKind, audience *configv1.Audience, priority int32) error {
	if priority < 0 {
		return invalid("priority must be nonnegative")
	}
	switch kind {
	case configv1.ReleaseKind_RELEASE_KIND_NORMAL:
		if len(audience.GetLabels()) != 0 || len(audience.GetConditions()) != 0 || priority != 0 {
			return invalid("normal releases do not have audience or priority")
		}
	case configv1.ReleaseKind_RELEASE_KIND_GRAY:
		return validateAudience(audience)
	default:
		return invalid("release kind must be NORMAL or GRAY")
	}
	return nil
}

func audienceRange(value string) (int64, int64, bool) {
	parts := strings.Split(value, "~")
	if len(parts) != 2 {
		return 0, 0, false
	}
	left, err1 := strconv.ParseInt(parts[0], 10, 64)
	right, err2 := strconv.ParseInt(parts[1], 10, 64)
	return left, right, err1 == nil && err2 == nil && left <= right
}

func conditionMatches(c *configv1.LabelCondition, value string) bool {
	switch c.GetType() {
	case configv1.MatchType_MATCH_TYPE_EXACT:
		return value == c.Value
	case configv1.MatchType_MATCH_TYPE_NOT_EQUALS:
		return value != c.Value
	case configv1.MatchType_MATCH_TYPE_REGEX:
		re, err := regexp.Compile(c.Value)
		return err == nil && re.MatchString(value)
	case configv1.MatchType_MATCH_TYPE_IN, configv1.MatchType_MATCH_TYPE_NOT_IN:
		found := false
		for _, item := range strings.Split(c.Value, ",") {
			if value == item {
				found = true
				break
			}
		}
		if c.Type == configv1.MatchType_MATCH_TYPE_NOT_IN {
			return !found
		}
		return found
	case configv1.MatchType_MATCH_TYPE_RANGE:
		left, right, ok := audienceRange(c.Value)
		n, err := strconv.ParseInt(value, 10, 64)
		return ok && err == nil && n >= left && n <= right
	default:
		return false
	}
}

func matches(audience *configv1.Audience, labels map[string]string) bool {
	for key, value := range audience.GetLabels() {
		actual, exists := labels[key]
		if !exists || actual != value {
			return false
		}
	}
	for _, c := range audience.GetConditions() {
		if c == nil {
			return false
		}
		actual, exists := labels[c.Key]
		if !exists || !conditionMatches(c, actual) {
			return false
		}
	}
	return true
}

func audienceConditions(a *configv1.Audience) map[string]*configv1.LabelCondition {
	result := make(map[string]*configv1.LabelCondition, len(a.GetLabels())+len(a.GetConditions()))
	for key, value := range a.GetLabels() {
		result[key] = &configv1.LabelCondition{Key: key, Type: configv1.MatchType_MATCH_TYPE_EXACT, Value: value}
	}
	for _, c := range a.GetConditions() {
		if c != nil {
			result[c.Key] = c
		}
	}
	return result
}

func finiteCandidates(c *configv1.LabelCondition) ([]string, bool) {
	switch c.Type {
	case configv1.MatchType_MATCH_TYPE_EXACT:
		return []string{c.Value}, true
	case configv1.MatchType_MATCH_TYPE_IN:
		return strings.Split(c.Value, ","), true
	default:
		return nil, false
	}
}

// audiencesOverlap 只在可以证明同一键上的条件互斥时返回 false；未知交集保守视为重叠。
func audiencesOverlap(a, b *configv1.Audience) bool {
	other := audienceConditions(b)
	for key, x := range audienceConditions(a) {
		y := other[key]
		if y == nil {
			continue
		}
		if candidates, finite := finiteCandidates(x); finite {
			intersects := false
			for _, value := range candidates {
				if conditionMatches(y, value) {
					intersects = true
					break
				}
			}
			if !intersects {
				return false
			}
		} else if candidates, finite := finiteCandidates(y); finite {
			intersects := false
			for _, value := range candidates {
				if conditionMatches(x, value) {
					intersects = true
					break
				}
			}
			if !intersects {
				return false
			}
		} else if x.Type == configv1.MatchType_MATCH_TYPE_RANGE && y.Type == configv1.MatchType_MATCH_TYPE_RANGE {
			xl, xr, xok := audienceRange(x.Value)
			yl, yr, yok := audienceRange(y.Value)
			if xok && yok && (xr < yl || yr < xl) {
				return false
			}
		}
	}
	return true
}

func mergedValues(definition *configv1.ConfigDefinition, rule *configv1.ValueRule) map[string]*configv1.Scalar {
	values := make(map[string]*configv1.Scalar, len(definition.GetDefaultValues()))
	for name, value := range definition.GetDefaultValues() {
		values[name] = value
	}
	for name, value := range rule.GetValues() {
		values[name] = value
	}
	return values
}

func effectiveValues(definition *configv1.ConfigDefinition, labels map[string]string) (map[string]*configv1.Scalar, error) {
	if len(definition.GetConditionGroups()) > 0 || len(definition.GetParameterBindings()) > 0 {
		groups := make(map[string]*configv1.Audience, len(definition.GetConditionGroups()))
		for _, group := range definition.GetConditionGroups() {
			groups[group.GetId()] = group.GetAudience()
		}
		values := make(map[string]*configv1.Scalar)
		for _, parameter := range definition.GetParameterBindings() {
			for _, binding := range parameter.GetBindings() {
				if audience, ok := groups[binding.GetGroupId()]; ok && matches(audience, labels) {
					values[parameter.GetName()] = binding.GetValue()
					break
				}
			}
			if _, ok := values[parameter.GetName()]; !ok && parameter.GetFallback() != nil {
				values[parameter.GetName()] = parameter.GetFallback()
			}
		}
		return values, nil
	}
	var selected *configv1.ValueRule
	ambiguous := false
	for _, rule := range definition.GetValueRules() {
		if rule == nil || !matches(rule.Audience, labels) {
			continue
		}
		if selected == nil || rule.Priority < selected.Priority {
			selected, ambiguous = rule, false
		} else if rule.Priority == selected.Priority {
			ambiguous = true
		}
	}
	if ambiguous {
		return nil, invalid("multiple value rules match at the highest priority")
	}
	return mergedValues(definition, selected), nil
}

func validateDefinition(ctx context.Context, definition *configv1.ConfigDefinition) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if definition == nil {
		return invalid("configuration definition is required")
	}
	if proto.Size(definition) > maxDefinitionBytes {
		return invalid("complete configuration definition exceeds 1 MiB")
	}
	if len(definition.Content) > maxContentBytes {
		return invalid("configuration content exceeds 512 KiB")
	}
	if len(definition.Parameters) > maxParameters || len(definition.DefaultValues) > maxParameters || len(definition.ValueRules) > maxRules {
		return invalid("configuration exceeds parameter or rule limits")
	}
	if len(definition.Format) > 32 || !utf8.ValidString(definition.Format) {
		return invalid("invalid output format")
	}
	if definition.Engine == "literal" {
		if len(definition.Parameters)+len(definition.DefaultValues)+len(definition.ValueRules)+len(definition.ConditionGroups)+len(definition.ParameterBindings) != 0 {
			return invalid("literal definitions cannot contain parameters, values or value rules")
		}
		return nil
	}
	if definition.Engine != "pole-mustache-v1" {
		return invalid("unsupported template engine")
	}
	if !utf8.Valid(definition.Content) {
		return invalid("parameterized template content must be UTF-8")
	}
	if len(definition.ConditionGroups) > 0 || len(definition.ParameterBindings) > 0 {
		return validateParameterBindings(definition)
	}
	ids := make(map[string]bool, len(definition.ValueRules))
	for i, rule := range definition.ValueRules {
		if rule == nil || rule.Id == "" || len(rule.Id) > 128 || ids[rule.Id] || rule.Priority < 0 || len(rule.Values) > maxParameters {
			return invalid("value rules require unique IDs, nonnegative priorities and bounded values")
		}
		ids[rule.Id] = true
		if err := validateAudience(rule.Audience); err != nil {
			return err
		}
		for _, previous := range definition.ValueRules[:i] {
			if previous.Priority == rule.Priority && audiencesOverlap(previous.Audience, rule.Audience) {
				return invalid("overlapping value rules must use different priorities")
			}
		}
	}
	// 静态检查所有有限选值分支，不替换占位符、不检查输出文档。
	references, err := validateTemplate(definition)
	if err != nil {
		return err
	}
	for _, rule := range append([]*configv1.ValueRule{nil}, definition.ValueRules...) {
		if err := validateValues(definition, mergedValues(definition, rule), references); err != nil {
			return err
		}
	}
	return nil
}

var conditionGroupID = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

func validateParameterBindings(definition *configv1.ConfigDefinition) error {
	if len(definition.DefaultValues) != 0 || len(definition.ValueRules) != 0 {
		return invalid("parameter bindings cannot be mixed with default values or value rules")
	}
	if len(definition.ConditionGroups) > maxRules || len(definition.ParameterBindings) > maxParameters {
		return invalid("configuration exceeds parameter or condition group limits")
	}
	references, err := validateTemplate(definition)
	if err != nil {
		return err
	}
	groups, names := make(map[string]bool), make(map[string]bool)
	for _, group := range definition.ConditionGroups {
		if group == nil || len(group.Id) > 128 || !conditionGroupID.MatchString(group.Id) || groups[group.Id] || strings.TrimSpace(group.Name) == "" || len(group.Name) > 128 || !utf8.ValidString(group.Name) || names[group.Name] {
			return invalid("condition groups require unique valid IDs and names")
		}
		if err := validateAudience(group.Audience); err != nil {
			return err
		}
		groups[group.Id], names[group.Name] = true, true
	}
	declared := make(map[string]bool, len(definition.Parameters))
	for _, p := range definition.Parameters {
		declared[p.Name] = true
	}
	seen := make(map[string]bool)
	fallbacks := make(map[string]*configv1.Scalar)
	for _, parameter := range definition.ParameterBindings {
		if parameter == nil || !declared[parameter.Name] || seen[parameter.Name] || len(parameter.Bindings) > maxRules {
			return invalid("parameter bindings require a unique declared parameter and bounded bindings")
		}
		seen[parameter.Name] = true
		if parameter.Fallback != nil {
			fallbacks[parameter.Name] = parameter.Fallback
		}
		ids, groupRefs := make(map[string]bool), make(map[string]bool)
		for _, binding := range parameter.Bindings {
			if binding == nil || binding.Id == "" || len(binding.Id) > 128 || !utf8.ValidString(binding.Id) || ids[binding.Id] || !groups[binding.GroupId] || groupRefs[binding.GroupId] || binding.Value == nil {
				return invalid("binding requires a unique ID, existing unique group and value")
			}
			ids[binding.Id], groupRefs[binding.GroupId] = true, true
		}
	}
	if err := validateValues(definition, fallbacks, references); err != nil {
		return err
	}
	maximum := make(map[string]*configv1.Scalar, len(fallbacks))
	for key, value := range fallbacks {
		maximum[key] = value
	}
	for _, parameter := range definition.ParameterBindings {
		for _, binding := range parameter.Bindings {
			if maximum[parameter.Name] == nil || len(binding.Value.Value) > len(maximum[parameter.Name].Value) {
				maximum[parameter.Name] = binding.Value
			}
			branch := make(map[string]*configv1.Scalar, len(fallbacks)+1)
			for key, value := range fallbacks {
				branch[key] = value
			}
			branch[parameter.Name] = binding.Value
			if err := validateValues(definition, branch, references); err != nil {
				return err
			}
		}
	}
	return validateValues(definition, maximum, references)
}

var parameterName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*(\.[A-Za-z_][A-Za-z0-9_-]*)*$`)
var integerText = regexp.MustCompile(`^-?(0|[1-9][0-9]*)$`)
var decimalText = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]*[1-9])?$`)

func validateTemplate(definition *configv1.ConfigDefinition) (map[string]bool, error) {
	switch strings.ToLower(definition.Format) {
	case "", "text", "json", "yaml", "yml", "xml", "properties", "html":
	default:
		return nil, invalid("unsupported template format")
	}
	declared := make(map[string]bool, len(definition.Parameters))
	for _, p := range definition.Parameters {
		if p == nil || !parameterName.MatchString(p.Name) || declared[p.Name] || p.Type < configv1.ValueType_VALUE_TYPE_STRING || p.Type > configv1.ValueType_VALUE_TYPE_DECIMAL {
			return nil, invalid("invalid parameter declaration")
		}
		declared[p.Name] = true
	}
	references := make(map[string]bool)
	content := string(definition.Content)
	for len(content) > 0 {
		open, close := strings.Index(content, "{{"), strings.Index(content, "}}")
		if close >= 0 && (open < 0 || close < open) {
			return nil, invalid("invalid template syntax")
		}
		if open < 0 {
			break
		}
		content = content[open:]
		if !strings.HasPrefix(content, "{{{") {
			return nil, invalid("only triple-mustache variables are supported")
		}
		end := strings.Index(content[3:], "}}}")
		if end < 0 {
			return nil, invalid("unclosed template variable")
		}
		name := content[3 : 3+end]
		if !parameterName.MatchString(name) || !declared[name] {
			return nil, invalid("invalid or undeclared template variable")
		}
		references[name] = true
		content = content[3+end+3:]
	}
	return references, nil
}

func validateValues(definition *configv1.ConfigDefinition, values map[string]*configv1.Scalar, references map[string]bool) error {
	declared := make(map[string]*configv1.Parameter, len(definition.Parameters))
	total := len(definition.Content)
	for _, p := range definition.Parameters {
		declared[p.Name] = p
		total += len(p.Name)
	}
	if total > maxContentBytes {
		return invalid("template inputs exceed 512 KiB")
	}
	for name, value := range values {
		p := declared[name]
		if p == nil || value == nil || value.Type != p.Type {
			return invalid("value type or declaration mismatch")
		}
		total += len(value.Value)
		if total > maxContentBytes {
			return invalid("template inputs exceed 512 KiB")
		}
		valid := false
		switch value.Type {
		case configv1.ValueType_VALUE_TYPE_STRING:
			valid = utf8.ValidString(value.Value)
		case configv1.ValueType_VALUE_TYPE_BOOLEAN:
			valid = value.Value == "true" || value.Value == "false"
		case configv1.ValueType_VALUE_TYPE_INTEGER:
			valid = value.Value != "-0" && integerText.MatchString(value.Value)
		case configv1.ValueType_VALUE_TYPE_DECIMAL:
			valid = value.Value != "-0" && decimalText.MatchString(value.Value)
		}
		if !valid {
			return invalid("invalid scalar value")
		}
	}
	for name, p := range declared {
		if (p.Required || references[name]) && values[name] == nil {
			return invalid("required or referenced parameter has no value")
		}
	}
	return nil
}
