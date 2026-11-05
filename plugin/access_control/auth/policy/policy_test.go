package policy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	apisecurity "github.com/pole-io/specification/source/go/api/v1/security"

	authtypes "github.com/pole-io/pole-server/apis/pkg/types/auth"
)

func TestEnrichResourceDetailHandlesDefaultPolicyResources(t *testing.T) {
	svr := &Server{}
	resp := &apisecurity.AuthStrategy{}

	require.NotPanics(t, func() {
		svr.enrichResourceInfo(context.Background(), resp, &authtypes.StrategyDetail{
			ID: "default-policy",
			Resources: []authtypes.StrategyResource{
				{StrategyID: "default-policy", ResType: apisecurity.ResourceType_LosslessRules, ResID: "*"},
				{StrategyID: "default-policy", ResType: apisecurity.ResourceType_MirrorRules, ResID: "*"},
				{StrategyID: "default-policy", ResType: apisecurity.ResourceType_SecurityRules, ResID: "*"},
				{StrategyID: "default-policy", ResType: apisecurity.ResourceType_MockRules, ResID: "*"},
			},
		})
	})

	require.Equal(t, "*", resp.Resources.GetLosslessRules()[0].GetId())
	require.Equal(t, "*", resp.Resources.GetMirrorRules()[0].GetId())
	require.Equal(t, "*", resp.Resources.GetSecurityRules()[0].GetId())
	require.Equal(t, "*", resp.Resources.GetMockRules()[0].GetId())
}

func TestResourceConvertIncludesTrafficGovernanceRules(t *testing.T) {
	require.Contains(t, resourceConvert, apisecurity.ResourceType_SecurityRules)
	require.Contains(t, resourceConvert, apisecurity.ResourceType_MirrorRules)
	require.Contains(t, resourceConvert, apisecurity.ResourceType_MockRules)
}

func TestResourceConvertIncludesAIRegistryResources(t *testing.T) {
	require.Contains(t, resourceConvert, apisecurity.ResourceType_MCPServerResources)
	require.Contains(t, resourceConvert, apisecurity.ResourceType_A2AAgentResources)
}

func TestAIRegistryResourceTypeFilter(t *testing.T) {
	require.Equal(t, "30", resTypeFilter["mcpserverresources"])
	require.Equal(t, "30", resTypeFilter["mcp_servers"])
	require.Equal(t, "31", resTypeFilter["a2aagentresources"])
	require.Equal(t, "31", resTypeFilter["a2a_agents"])
}

func TestEnrichResourceDetailSkipsUnsupportedResourceType(t *testing.T) {
	svr := &Server{}

	require.NotPanics(t, func() {
		svr.enrichResourceDetial(context.Background(), authtypes.StrategyResource{
			StrategyID: "unknown-policy",
			ResType:    apisecurity.ResourceType(999),
			ResID:      "*",
		}, map[apisecurity.ResourceType]struct{}{}, &apisecurity.AuthStrategy{
			Resources: &apisecurity.StrategyResources{},
		})
	})
}

func TestNormalizeResourceHandlesStringIDsAndCollapsesWildcard(t *testing.T) {
	svr := &Server{}
	resources := &apisecurity.StrategyResources{
		Namespaces: []*apisecurity.StrategyResourceEntry{
			{Id: "namespace-a"},
			{Id: "*", Name: "ignored-after-wildcard"},
		},
		Services: []*apisecurity.StrategyResourceEntry{{Id: "service-a"}},
	}

	require.NotPanics(t, func() {
		resources = svr.normalizeResource(resources)
	})
	require.Equal(t, []*apisecurity.StrategyResourceEntry{{Id: "*"}}, resources.Namespaces)
	require.Equal(t, "service-a", resources.Services[0].GetId())
}
