package auth

import (
	"testing"

	"github.com/stretchr/testify/require"

	apisecurity "github.com/pole-io/specification/source/go/api/v1/security"
)

func TestParsePolicyRuleHandlesStringResourceIDs(t *testing.T) {
	require.NotPanics(t, func() {
		detail := ParsePolicyRule(&apisecurity.AuthStrategy{
			Name:   "policy-string-resource",
			Action: apisecurity.AuthAction_ALLOW,
			Resources: &apisecurity.StrategyResources{
				Namespaces: []*apisecurity.StrategyResourceEntry{
					{Id: "namespace-a"},
					{Id: "*"},
				},
			},
			Principals: &apisecurity.Principals{},
		})

		require.Len(t, detail.Resources, 1)
		require.Equal(t, "*", detail.Resources[0].ResID)
	})
}
