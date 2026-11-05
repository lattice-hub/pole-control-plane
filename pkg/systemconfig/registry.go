package systemconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"time"
)

type Registration struct {
	Definition SettingDefinition
	SourcePath string
	Value      func() any
	Source     func() SourceDescriptor
	Status     func() string
}

type Registry struct {
	registrations []Registration
	sources       SourceIndex
}

func NewRegistry(registrations []Registration, sources SourceIndex) (*Registry, error) {
	seen := make(map[string]struct{}, len(registrations))
	cloned := append([]Registration(nil), registrations...)
	for _, registration := range cloned {
		definition := registration.Definition
		if definition.Key == "" || definition.Component == "" || definition.Domain == "" ||
			definition.Label == "" || definition.ValueType == "" || definition.ApplyMode == "" ||
			definition.Sensitivity == "" || definition.Owner == "" || registration.Value == nil {
			return nil, fmt.Errorf("invalid system setting registration %q", definition.Key)
		}
		if _, ok := seen[definition.Key]; ok {
			return nil, fmt.Errorf("duplicate system setting key %q", definition.Key)
		}
		seen[definition.Key] = struct{}{}
	}
	sort.Slice(cloned, func(i, j int) bool { return cloned[i].Definition.Key < cloned[j].Definition.Key })
	return &Registry{registrations: cloned, sources: sources}, nil
}

func (r *Registry) Describe(_ context.Context, scope Scope) ([]SettingDefinition, error) {
	definitions := make([]SettingDefinition, 0, len(r.registrations))
	for _, registration := range r.registrations {
		if !matchesScope(registration.Definition, scope) {
			continue
		}
		definitions = append(definitions, registration.Definition)
	}
	return definitions, nil
}

func (r *Registry) Effective(_ context.Context, scope Scope) (*EffectiveSnapshot, error) {
	snapshot := &EffectiveSnapshot{Component: scope.Component, Settings: make([]EffectiveSetting, 0, len(r.registrations))}
	for _, registration := range r.registrations {
		definition := registration.Definition
		if !matchesScope(definition, scope) {
			continue
		}
		value := registration.Value()
		configured := isConfigured(value)
		source := r.sources.Source(registration.SourcePath)
		if registration.Source != nil {
			source = registration.Source()
		}
		setting := EffectiveSetting{
			SettingDefinition: definition,
			Configured:        configured,
			Source:            source,
		}
		if registration.Status != nil {
			setting.ApplyStatus = registration.Status()
		}
		if definition.Sensitivity == SensitivitySecret {
			setting.Redacted = true
			if configured {
				setting.DisplayValue = "已配置"
			} else {
				setting.DisplayValue = "未配置"
			}
		} else {
			setting.Value = normalizeValue(value)
			setting.DisplayValue = formatValue(value)
		}
		snapshot.Settings = append(snapshot.Settings, setting)
	}
	return snapshot, nil
}

func matchesScope(definition SettingDefinition, scope Scope) bool {
	if scope.Component != "" && definition.Component != scope.Component {
		return false
	}
	return scope.Domain == "" || definition.Domain == scope.Domain
}

func isConfigured(value any) bool {
	if value == nil {
		return false
	}
	ref := reflect.ValueOf(value)
	for ref.IsValid() && (ref.Kind() == reflect.Pointer || ref.Kind() == reflect.Interface) {
		if ref.IsNil() {
			return false
		}
		ref = ref.Elem()
	}
	if !ref.IsValid() {
		return false
	}
	switch ref.Kind() {
	case reflect.String, reflect.Array, reflect.Slice, reflect.Map:
		return ref.Len() > 0
	default:
		return true
	}
}

func normalizeValue(value any) any {
	if duration, ok := value.(time.Duration); ok {
		return duration.String()
	}
	return value
}

func formatValue(value any) string {
	if !isConfigured(value) {
		return "未配置"
	}
	if duration, ok := value.(time.Duration); ok {
		return duration.String()
	}
	if text, ok := value.(string); ok {
		return text
	}
	encoded, err := json.Marshal(value)
	if err == nil {
		return string(encoded)
	}
	return fmt.Sprint(value)
}

func LookupValue(value any, path ...string) any {
	current := value
	for _, segment := range path {
		switch typed := current.(type) {
		case map[string]interface{}:
			current = typed[segment]
		case map[interface{}]interface{}:
			current = typed[segment]
		default:
			return nil
		}
	}
	return current
}
