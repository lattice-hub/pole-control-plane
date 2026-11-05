package poleagent

import (
	"fmt"
	"strings"
)

func namespaceScopedRemoteTools(tools []ToolDefinition) []ToolDefinition {
	scoped := make([]ToolDefinition, 0, len(tools))
	for _, tool := range tools {
		if toolScopeArgument(tool) != "" {
			scoped = append(scoped, tool)
		}
	}
	return scoped
}

// enforceToolNamespace 根据实际schema的唯一环境字段校验和注入，trace与调用使用同一份参数。
// 单环境缺值时注入；跨环境必须显式选择范围内的环境，不能借另一字段绕过约束。
func enforceToolNamespace(scope NamespaceScope, tool ToolDefinition, arguments map[string]any) error {
	field := toolScopeArgument(tool)
	if field == "" {
		return fmt.Errorf("tool must declare exactly one environment scope argument")
	}
	alternate := "namespace"
	if field == "namespace" {
		alternate = "environment"
	}
	if _, exists := arguments[alternate]; exists {
		return fmt.Errorf("tool scope must use the declared %s argument", field)
	}
	raw, exists := arguments[field]
	if exists {
		namespace, ok := raw.(string)
		if !ok {
			return fmt.Errorf("tool %s is outside the turn scope", field)
		}
		if strings.TrimSpace(namespace) != "" {
			if !scope.Contains(namespace) {
				return fmt.Errorf("tool %s is outside the turn scope", field)
			}
			arguments[field] = strings.TrimSpace(namespace)
			return nil
		}
	}
	if !scope.IsSingle() {
		return fmt.Errorf("cross-environment tool calls must select one %s", field)
	}
	arguments[field] = scope.SingleNamespace()
	return nil
}

// toolScopeArgument 只接受schema显式声明的单一string字段，避免未知或双字段工具拥有隐式全scope。
func toolScopeArgument(tool ToolDefinition) string {
	properties, ok := tool.InputSchema["properties"].(map[string]any)
	if !ok {
		return ""
	}
	selected := ""
	for _, field := range []string{"environment", "namespace"} {
		property, exists := properties[field]
		if !exists {
			continue
		}
		definition, ok := property.(map[string]any)
		if !ok || definition["type"] != "string" || selected != "" {
			return ""
		}
		selected = field
	}
	return selected
}
