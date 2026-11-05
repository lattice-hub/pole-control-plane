/**
 * Tencent is pleased to support the open source community by making Polaris available.
 *
 * Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
 *
 * Licensed under the BSD 3-Clause License (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * https://opensource.org/licenses/BSD-3-Clause
 *
 * Unless required by applicable law or agreed to in writing, software distributed
 * under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
 * CONDITIONS OF ANY KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 */

package auth

import (
	"testing"

	"github.com/stretchr/testify/require"

	apisecurity "github.com/pole-io/specification/source/go/api/v1/security"

	authtypes "github.com/pole-io/pole-server/apis/pkg/types/auth"
)

func TestPolicyCacheColdLoadSupportsRolePrincipals(t *testing.T) {
	cache := &policyCache{}
	cache.initContainers()
	rule := NewPolicyDetailCache(&authtypes.StrategyDetail{
		ID: "role-policy", Action: apisecurity.AuthAction_ALLOW.String(), Valid: true,
		Principals: []authtypes.Principal{{
			PrincipalType: authtypes.PrincipalRole,
			PrincipalID:   "resource-reader",
		}},
	})

	require.NotPanics(t, func() { cache.handlePrincipalPolicies(rule) })
	links, ok := cache.allowPolicies[authtypes.PrincipalRole].Load("resource-reader")
	require.True(t, ok)
	require.True(t, links.Contains("role-policy"))
	_, ok = cache.principalResources[authtypes.PrincipalRole].Load("resource-reader")
	require.True(t, ok)
}
