package types

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSanitizeOperationDetail(t *testing.T) {
	for _, resource := range []Resource{RUser, RUserGroup, RUserGroupRelation} {
		t.Run(string(resource), func(t *testing.T) {
			cases := map[string]string{
				"protobuf aliases":      `{"id":"visible","password":"password-sentinel","Password":"upper-sentinel","authToken":"camel-sentinel","auth_token":"snake-sentinel","Token":"model-sentinel","token_enable":true}`,
				"nested relation":       `{"id":"visible","relation":{"users":[{"name":"member","PASSWORD":"nested-sentinel","AUTH_TOKEN":"nested-token-sentinel"}]}}`,
				"escaped key":           `{"id":"visible","pass\u0077ord":"unicode-sentinel","auth-token":"separator-sentinel"}`,
				"nested JSON string":    `{"id":"visible","metadata":{"data":"{\"Password\":\"encoded-sentinel\",\"name\":\"nested-visible\"}"}}`,
				"malformed nested JSON": `{"id":"visible","metadata":{"data":"{\"auth_token\":\"malformed-sentinel\""}}`,
				"double encoded JSON":   `"{\"id\":\"visible\",\"auth_token\":\"double-sentinel\"}"`,
				"password changes":      `{"id":"visible","oldPassword":"old-sentinel","new_password":"new-sentinel","password_hash":"hash-sentinel"}`,
				"duplicate keys":        `{"id":"visible","password":"first-sentinel","password":"second-sentinel"}`,
			}
			for name, detail := range cases {
				t.Run(name, func(t *testing.T) {
					sanitized := SanitizeOperationDetail(resource, detail)
					require.NotContains(t, sanitized, "sentinel")
					require.Contains(t, sanitized, "visible")
					require.True(t, json.Valid([]byte(sanitized)))
					require.Equal(t, sanitized, SanitizeOperationDetail(resource, sanitized))
				})
			}
			for _, malformed := range []string{`{"Password":"broken-sentinel"`, `{"id":"visible"} {"password":"trailing-sentinel"}`, `password=plain-sentinel`, `"plain-sentinel"`, `123`, `null`} {
				require.Equal(t, redactedOperationDetail, SanitizeOperationDetail(resource, malformed))
			}
			require.Empty(t, SanitizeOperationDetail(resource, ""))
		})
	}
}

func TestSanitizeOperationDetailPreservesLegitimateAudit(t *testing.T) {
	detail := `{"id":"user-id","name":"visible","token_enable":false,"comment":"ordinary text","metadata":{"team":"platform","sequence":9007199254740993}}`
	require.JSONEq(t, detail, SanitizeOperationDetail(RUser, detail))
	require.Contains(t, SanitizeOperationDetail(RUser, detail), "9007199254740993")
	for _, resource := range []Resource{RService, RRouting, RAuthStrategy} {
		for _, nonUserDetail := range []string{"update route", detail, `{"password":"legitimate-non-account-detail"}`} {
			require.Equal(t, nonUserDetail, SanitizeOperationDetail(resource, nonUserDetail))
		}
	}
}

func TestSanitizeOperationDetailBoundsNestedInput(t *testing.T) {
	detail := strings.Repeat(`{"nested":`, 100) + `{"password":"deep-sentinel"}` + strings.Repeat(`}`, 100)
	sanitized := SanitizeOperationDetail(RUser, detail)
	require.NotContains(t, sanitized, "sentinel")
	require.Contains(t, sanitized, "[REDACTED]")
}

func TestRecordEntryStringExcludesCredentials(t *testing.T) {
	entry := &RecordEntry{ResourceType: RUser, ResourceName: "audit-user", Detail: `{"id":"visible","password":"logger-sentinel","authToken":"logger-token-sentinel"}`}
	require.NotContains(t, entry.String(), "sentinel")
	require.Contains(t, entry.String(), "visible")
	require.Contains(t, entry.Detail, "sentinel", "formatting must not mutate the business entry")
}
