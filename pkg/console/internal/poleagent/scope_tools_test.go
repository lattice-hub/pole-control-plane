package poleagent

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestScopeRemoteToolsKeepsSchemaBoundTools(t *testing.T) {
	ambiguous := environmentTool("ambiguous")
	ambiguous.InputSchema["properties"].(map[string]any)["namespace"] = map[string]any{"type": "string"}
	invalid := environmentTool("invalid")
	invalid.InputSchema["properties"].(map[string]any)["environment"] = map[string]any{"type": "object"}
	tools := []ToolDefinition{environmentTool("get_config_state"), namespaceTool("get_service"), {Name: "list_namespaces", InputSchema: map[string]any{"properties": map[string]any{}}}, ambiguous, invalid}
	scoped := namespaceScopedRemoteTools(tools)
	require.Len(t, scoped, 2)
	require.Equal(t, "get_config_state", scoped[0].Name)
	require.Equal(t, "get_service", scoped[1].Name)
}

func TestEnforceToolNamespaceUsesSchemaField(t *testing.T) {
	for _, test := range []struct {
		field string
		tool  ToolDefinition
	}{{"environment", environmentTool("get_config_state")}, {"namespace", namespaceTool("get_service")}} {
		t.Run(test.field, func(t *testing.T) {
			arguments := map[string]any{"group": "app"}
			require.NoError(t, enforceToolNamespace(singleScope("development"), test.tool, arguments))
			require.Equal(t, "development", arguments[test.field])
			require.Len(t, arguments, 2)
			require.ErrorContains(t, enforceToolNamespace(singleScope("development"), test.tool, map[string]any{test.field: "production"}), "outside the turn scope")
			require.ErrorContains(t, enforceToolNamespace(singleScope("development"), test.tool, map[string]any{test.field: 123}), "outside the turn scope")
			scope := NamespaceScope{Mode: NamespaceScopeCrossEnvironment, Namespaces: []string{"development", "production"}}
			require.ErrorContains(t, enforceToolNamespace(scope, test.tool, map[string]any{}), "must select one "+test.field)
			explicit := map[string]any{test.field: "production"}
			require.NoError(t, enforceToolNamespace(scope, test.tool, explicit))
			require.Equal(t, "production", explicit[test.field])
		})
	}
}

func TestEnforceToolNamespaceRejectsConflictingField(t *testing.T) {
	require.ErrorContains(t, enforceToolNamespace(singleScope("development"), environmentTool("get_config_state"), map[string]any{"namespace": "production"}), "declared environment")
	require.ErrorContains(t, enforceToolNamespace(singleScope("development"), namespaceTool("get_service"), map[string]any{"environment": "production"}), "declared namespace")
}
