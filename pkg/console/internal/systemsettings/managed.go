package systemsettings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"net/url"
	"reflect"
	"strings"
	"time"

	store "github.com/pole-io/pole-server/pkg/console/internal/observer"
	"github.com/pole-io/pole-server/pkg/systemconfig"
)

func (m *Manager) ManagedDomain(ctx context.Context, component, domain string,
	effectiveSettings ...[]systemconfig.EffectiveSetting) (*ManagedDomainView, error) {
	record, err := m.repo.GetSystemConfigDomain(ctx, component, domain)
	if err != nil {
		return nil, err
	}
	view := &ManagedDomainView{
		Component: component, Domain: domain, ApplyStatus: "static",
		ApplyMessage: "当前使用启动配置",
	}
	if record.ActiveRevision != nil {
		view.Active, err = managedRevisionView(record.ActiveRevision)
		if err != nil {
			return nil, err
		}
		view.ApplyStatus = "pending_restart"
		view.ApplyMessage = "目标版本已发布；当前实例仍使用启动配置，等待重启级配置适配器收敛"
		if len(effectiveSettings) > 0 {
			view.ApplyStatus, view.ApplyMessage = managedApplyState(view.Active, effectiveSettings[0])
		}
	}
	if record.DraftRevision != nil {
		view.Draft, err = managedRevisionView(record.DraftRevision)
		if err != nil {
			return nil, err
		}
	}
	return view, nil
}

func managedApplyState(active *ManagedRevisionView,
	settings []systemconfig.EffectiveSetting) (string, string) {
	if active == nil {
		return "static", "当前使用启动配置"
	}
	guarded, drifted := false, false
	for _, setting := range settings {
		desired, ok := active.Values[setting.Key]
		if !ok {
			continue
		}
		if setting.ApplyStatus == "rejected" {
			return "rejected", "目标版本校验失败；实例继续使用上一次有效配置"
		}
		if setting.ApplyMode == systemconfig.HotReload || setting.ApplyMode == systemconfig.GuardedHotReload {
			guarded = true
			drifted = drifted || !reflect.DeepEqual(normalizeComparable(setting.Value), normalizeComparable(desired))
		}
	}
	if guarded && !drifted {
		return "applied", "目标版本已在当前实例生效"
	}
	if guarded {
		return "applying", "目标版本已发布，等待运行实例收敛"
	}
	return "pending_restart", "目标版本已发布；当前实例仍使用启动配置，等待重启级配置适配器收敛"
}

func (m *Manager) SaveManagedDraft(ctx context.Context, actor, component, domain string,
	settings []systemconfig.EffectiveSetting, request SaveManagedDraftRequest) (*ManagedDomainView, error) {
	values, err := validateManagedValues(component, domain, settings, request.Values)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(values)
	if err != nil {
		return nil, err
	}
	if _, err = m.repo.SaveSystemConfigDraft(ctx, component, domain,
		request.ExpectedDraftRevision, payload, 0, actor); err != nil {
		return nil, err
	}
	return m.ManagedDomain(ctx, component, domain)
}

func (m *Manager) PublishManaged(ctx context.Context, actor, component, domain string,
	request PublishRequest) (*ManagedDomainView, error) {
	record, err := m.repo.GetSystemConfigDomain(ctx, component, domain)
	if err != nil {
		return nil, err
	}
	if record.DraftRevision == nil || record.DraftRevision.ID != request.DraftRevision {
		return nil, errors.New("草稿版本已变化，请刷新后重试")
	}
	if _, err = m.repo.PublishSystemConfigDraft(ctx, component, domain, request.DraftRevision, actor); err != nil {
		return nil, err
	}
	return m.ManagedDomain(ctx, component, domain)
}

func (m *Manager) ManagedReleases(ctx context.Context, component, domain string) ([]*ManagedRevisionView, error) {
	revisions, err := m.repo.ListSystemConfigReleases(ctx, component, domain, 20)
	if err != nil {
		return nil, err
	}
	result := make([]*ManagedRevisionView, 0, len(revisions))
	for _, revision := range revisions {
		view, viewErr := managedRevisionView(revision)
		if viewErr != nil {
			return nil, viewErr
		}
		result = append(result, view)
	}
	return result, nil
}

func (m *Manager) EnrichManagedDesired(ctx context.Context, settings []systemconfig.EffectiveSetting) {
	scopes := map[string][]int{}
	for index := range settings {
		if settings[index].Domain == AgentDomain {
			continue
		}
		key := string(settings[index].Component) + "\x00" + settings[index].Domain
		scopes[key] = append(scopes[key], index)
	}
	for scope, indexes := range scopes {
		parts := strings.SplitN(scope, "\x00", 2)
		record, err := m.repo.GetSystemConfigDomain(ctx, parts[0], parts[1])
		if err != nil || record.ActiveRevision == nil {
			continue
		}
		view, err := managedRevisionView(record.ActiveRevision)
		if err != nil {
			continue
		}
		for _, index := range indexes {
			value, ok := view.Values[settings[index].Key]
			if !ok {
				continue
			}
			settings[index].DesiredValue = value
			settings[index].DesiredDisplayValue = displayManagedValue(value)
			settings[index].DesiredRevision = view.Revision
			settings[index].Drifted = !reflect.DeepEqual(normalizeComparable(settings[index].Value), normalizeComparable(value))
			switch settings[index].ApplyMode {
			case systemconfig.HotReload, systemconfig.GuardedHotReload:
				if settings[index].ApplyStatus != "rejected" {
					if settings[index].Drifted {
						settings[index].ApplyStatus = "applying"
					} else {
						settings[index].ApplyStatus = "applied"
					}
				}
			default:
				settings[index].ApplyStatus = "pending_restart"
			}
		}
	}
}

func managedRevisionView(revision *store.SystemConfigRevision) (*ManagedRevisionView, error) {
	values := map[string]any{}
	if err := json.Unmarshal(revision.Payload, &values); err != nil {
		return nil, fmt.Errorf("解析系统配置版本 %d: %w", revision.ID, err)
	}
	return &ManagedRevisionView{
		Revision: revision.ID, State: revision.State, Values: values,
		CreatedBy: revision.CreatedBy, PublishedBy: revision.PublishedBy,
		CreatedAt: revision.CreatedAt, PublishedAt: revision.PublishedAt,
	}, nil
}

func validateManagedValues(component, domain string, settings []systemconfig.EffectiveSetting,
	input map[string]any) (map[string]any, error) {
	if component == AgentComponent && domain == AgentDomain {
		return nil, errors.New("Agent 领域必须使用受控编辑器")
	}
	definitions := make(map[string]systemconfig.SettingDefinition, len(settings))
	for _, setting := range settings {
		if string(setting.Component) == component && setting.Domain == domain {
			definitions[setting.Key] = setting.SettingDefinition
		}
	}
	if len(definitions) == 0 {
		return nil, errors.New("配置领域不存在")
	}
	result := make(map[string]any)
	for key := range input {
		definition, ok := definitions[key]
		if !ok {
			return nil, fmt.Errorf("配置项 %s 不属于当前领域", key)
		}
		if !definition.Editable {
			return nil, fmt.Errorf("配置项 %s 只能通过部署配置修改", key)
		}
	}
	for key, definition := range definitions {
		if !definition.Editable {
			continue
		}
		value, ok := input[key]
		if !ok {
			return nil, fmt.Errorf("配置项 %s 不能为空", key)
		}
		normalized, err := normalizeManagedValue(definition, value)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", definition.Label, err)
		}
		result[key] = normalized
	}
	if domain == "naming" {
		minimum, minOK := durationValue(result["server.naming.healthcheck.min_check_interval"])
		maximum, maxOK := durationValue(result["server.naming.healthcheck.max_check_interval"])
		if minOK && maxOK && minimum > maximum {
			return nil, errors.New("最小检查周期不能大于最大检查周期")
		}
	}
	return result, nil
}

func normalizeManagedValue(definition systemconfig.SettingDefinition, value any) (any, error) {
	rule := definition.Validation
	switch definition.ValueType {
	case systemconfig.ValueTypeBoolean:
		typed, ok := value.(bool)
		if !ok {
			return nil, errors.New("必须是布尔值")
		}
		return typed, nil
	case systemconfig.ValueTypeInteger:
		number, ok := numberValue(value)
		if !ok {
			return nil, errors.New("必须是整数")
		}
		if rule.MinInteger != nil && number < *rule.MinInteger {
			return nil, fmt.Errorf("不能小于 %d", *rule.MinInteger)
		}
		if rule.MaxInteger != nil && number > *rule.MaxInteger {
			return nil, fmt.Errorf("不能大于 %d", *rule.MaxInteger)
		}
		return number, nil
	case systemconfig.ValueTypeDuration:
		text, ok := value.(string)
		if !ok {
			return nil, errors.New("必须是 duration，例如 30s")
		}
		parsed, err := time.ParseDuration(strings.TrimSpace(text))
		if err != nil {
			return nil, errors.New("duration 格式无效")
		}
		if rule.MinDuration != "" {
			minimum, _ := time.ParseDuration(rule.MinDuration)
			if parsed < minimum {
				return nil, fmt.Errorf("不能小于 %s", rule.MinDuration)
			}
		}
		if rule.MaxDuration != "" {
			maximum, _ := time.ParseDuration(rule.MaxDuration)
			if parsed > maximum {
				return nil, fmt.Errorf("不能大于 %s", rule.MaxDuration)
			}
		}
		return parsed.String(), nil
	case systemconfig.ValueTypeList:
		items, ok := value.([]any)
		if !ok {
			if stringsValue, stringsOK := value.([]string); stringsOK {
				return stringsValue, nil
			}
			return nil, errors.New("必须是字符串列表")
		}
		result := make([]string, 0, len(items))
		for _, item := range items {
			text, ok := item.(string)
			if !ok {
				return nil, errors.New("列表项必须是字符串")
			}
			text = strings.TrimSpace(text)
			if text != "" {
				result = append(result, text)
			}
		}
		return result, nil
	case systemconfig.ValueTypeString:
		text, ok := value.(string)
		if !ok {
			return nil, errors.New("必须是字符串")
		}
		text = strings.TrimSpace(text)
		if rule.Required && text == "" {
			return nil, errors.New("不能为空")
		}
		if len(rule.Options) > 0 && !contains(rule.Options, text) {
			return nil, fmt.Errorf("必须是 %s 之一", strings.Join(rule.Options, "、"))
		}
		if text != "" {
			switch rule.Format {
			case "url":
				parsed, err := url.ParseRequestURI(text)
				if err != nil || parsed.Scheme == "" || parsed.Host == "" {
					return nil, errors.New("必须是完整 URL")
				}
			case "host-port":
				if _, _, err := net.SplitHostPort(text); err != nil {
					return nil, errors.New("必须是 host:port")
				}
			}
		}
		return text, nil
	default:
		return nil, errors.New("该类型不支持通用编辑")
	}
}

func numberValue(value any) (int64, bool) {
	switch typed := value.(type) {
	case float64:
		if math.Trunc(typed) != typed || typed > math.MaxInt64 || typed < math.MinInt64 {
			return 0, false
		}
		return int64(typed), true
	case int:
		return int64(typed), true
	case int64:
		return typed, true
	default:
		return 0, false
	}
}

func durationValue(value any) (time.Duration, bool) {
	text, ok := value.(string)
	if !ok {
		return 0, false
	}
	parsed, err := time.ParseDuration(text)
	return parsed, err == nil
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func displayManagedValue(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	encoded, err := json.Marshal(value)
	if err == nil {
		return string(encoded)
	}
	return fmt.Sprint(value)
}

func normalizeComparable(value any) any {
	if number, ok := numberValue(value); ok {
		return number
	}
	return value
}
