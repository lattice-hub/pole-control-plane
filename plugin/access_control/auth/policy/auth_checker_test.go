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

package policy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	authapi "github.com/pole-io/pole-server/apis/access_control/auth"
	"github.com/pole-io/pole-server/apis/pkg/types"
	authtypes "github.com/pole-io/pole-server/apis/pkg/types/auth"
)

func TestCheckConsolePermission_OwnerUserBypassesPolicyCache(t *testing.T) {
	checker := &DefaultAuthChecker{
		conf:    &AuthConfig{ConsoleOpen: true, ConsoleStrict: true},
		userSvr: ownerCredentialUserServer{},
	}

	ok, err := checker.CheckConsolePermission(authtypes.NewAcquireContext(
		authtypes.WithRequestContext(context.Background()),
		authtypes.WithMethod(authtypes.DescribeNamespaces),
		authtypes.WithOperation(authtypes.Read),
	))

	require.NoError(t, err)
	require.True(t, ok)
}

func TestCheckConsolePermissionRetainsCredentialContext(t *testing.T) {
	checker := &DefaultAuthChecker{
		conf:    &AuthConfig{ConsoleOpen: true, ConsoleStrict: true},
		userSvr: ownerCredentialUserServer{},
	}
	authCtx := authtypes.NewAcquireContext(
		authtypes.WithRequestContext(context.Background()),
		authtypes.WithMethod(authtypes.DescribeAuthPolicyDetail),
		authtypes.WithOperation(authtypes.Read),
	)

	ok, err := checker.CheckConsolePermission(authCtx)

	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, authtypes.OwnerUserRole, authtypes.ParseUserRole(authCtx.GetRequestContext()))
}

func TestResourcePredicate_OwnerUserBypassesPolicyCache(t *testing.T) {
	checker := &DefaultAuthChecker{
		conf: &AuthConfig{ConsoleOpen: true, ConsoleStrict: true},
	}
	ctx := context.WithValue(context.Background(), types.ContextUserRoleIDKey, authtypes.OwnerUserRole)

	ok := checker.ResourcePredicate(authtypes.NewAcquireContext(
		authtypes.WithRequestContext(ctx),
	), &authtypes.ResourceEntry{})

	require.True(t, ok)
}

func TestPermissionSpanPrefixDistinguishesAuthorizationAndCapabilityChecks(t *testing.T) {
	authCtx := authtypes.NewAcquireContext(authtypes.WithMethod(authtypes.DescribeNamespaces))
	require.Equal(t, "auth.console.authorize.DescribeNamespaces",
		permissionSpanName(permissionSpanPrefix("auth.console", authCtx), authCtx.GetMethods()))

	authCtx.SetMethod([]authtypes.ServerFunctionName{authtypes.UpdateNamespaces})
	authCtx.SetAttachment(authtypes.PermissionCheckPurposeKey, authtypes.PermissionCheckPurposeCapability)
	require.Equal(t, "auth.console.capability.UpdateNamespaces",
		permissionSpanName(permissionSpanPrefix("auth.console", authCtx), authCtx.GetMethods()))
}

type ownerCredentialUserServer struct {
	authapi.UserServer
}

func (ownerCredentialUserServer) CheckCredential(authCtx *authtypes.AcquireContext) error {
	ctx := context.WithValue(authCtx.GetRequestContext(), types.ContextUserRoleIDKey, authtypes.OwnerUserRole)
	authCtx.SetRequestContext(ctx)
	return nil
}
