package config

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	configv1 "github.com/pole-io/specification/source/go/api/v1/config"
	apimodel "github.com/pole-io/specification/source/go/api/v1/model"
	apisecurity "github.com/pole-io/specification/source/go/api/v1/security"

	"github.com/pole-io/pole-server/apis/pkg/types"
	authtypes "github.com/pole-io/pole-server/apis/pkg/types/auth"
	cachemock "github.com/pole-io/pole-server/pkg/cache/mock"
	"github.com/pole-io/pole-server/pkg/config/unified"
	authmock "github.com/pole-io/pole-server/plugin/access_control/auth/mock"
	"github.com/pole-io/pole-server/plugin/access_control/auth/policy"
)

type authFixture struct {
	caches     *cachemock.MockCacheManager
	namespaces *cachemock.MockNamespaceCache
	users      *authmock.MockUserServer
	checker    *authmock.MockAuthChecker
	authorizer unified.Authorizer
	key        *configv1.ConfigKey
}

func newAuthFixture(t *testing.T) *authFixture {
	t.Helper()
	ctrl := gomock.NewController(t)
	f := &authFixture{caches: cachemock.NewMockCacheManager(ctrl), namespaces: cachemock.NewMockNamespaceCache(ctrl), users: authmock.NewMockUserServer(ctrl), checker: authmock.NewMockAuthChecker(ctrl), key: &configv1.ConfigKey{Environment: "prod", Group: "app", Name: "settings"}}
	var err error
	f.authorizer, err = NewUnifiedAuthorizer(f.caches, f.users, f.checker)
	require.NoError(t, err)
	return f
}
func (f *authFixture) environment(value *types.Namespace) {
	f.caches.EXPECT().Namespace().Return(f.namespaces)
	f.namespaces.EXPECT().GetNamespace(f.key.Environment).Return(value)
}
func businessNamespace() *types.Namespace {
	return &types.Namespace{Name: "prod", Valid: true, Kind: apimodel.NamespaceKind_NAMESPACE_KIND_BUSINESS, Owner: "owner", Metadata: map[string]string{"department": "platform"}}
}

var verifiedPrincipal = authtypes.Principal{PrincipalID: "credential-user", PrincipalType: authtypes.PrincipalUser}

func authenticated(acquired *authtypes.AcquireContext) error {
	acquired.SetAttachment(authtypes.PrincipalKey, verifiedPrincipal)
	return nil
}

func TestUnifiedAuthorizerRoutesPublicPermissionMethodsAfterCredential(t *testing.T) {
	for _, test := range []struct {
		action    unified.Action
		operation authtypes.ResourceOperation
		method    authtypes.ServerFunctionName
		client    bool
	}{{unified.ActionRead, authtypes.Read, authtypes.DiscoverConfig, true}, {unified.ActionInspect, authtypes.Read, authtypes.InspectConfig, false}, {unified.ActionManage, authtypes.Modify, authtypes.ManageConfig, false}, {unified.ActionPreview, authtypes.Modify, authtypes.PreviewConfig, false}} {
		t.Run(string(test.action), func(t *testing.T) {
			f := newAuthFixture(t)
			environment := businessNamespace()
			f.environment(environment)
			ctx := context.WithValue(context.Background(), types.ContextOperator, "forged-staff")
			ctx = context.WithValue(ctx, types.ContextAuthTokenKey, "credential-token")
			var captured *authtypes.AcquireContext
			credential := f.users.EXPECT().CheckCredential(gomock.Any()).DoAndReturn(func(acquired *authtypes.AcquireContext) error {
				captured = acquired
				require.Equal(t, ctx, acquired.GetRequestContext())
				require.Empty(t, acquired.GetAttachments())
				return authenticated(acquired)
			})
			check := func(acquired *authtypes.AcquireContext) (bool, error) {
				require.Same(t, captured, acquired)
				require.Equal(t, authtypes.ConfigModule, acquired.GetModule())
				require.Equal(t, test.operation, acquired.GetOperation())
				require.Equal(t, []authtypes.ServerFunctionName{test.method}, acquired.GetMethods())
				require.Equal(t, test.client, acquired.IsFromClient())
				require.False(t, acquired.IsAllowAnonymous())
				require.Equal(t, map[apisecurity.ResourceType][]authtypes.ResourceEntry{apisecurity.ResourceType_Namespaces: {{Type: apisecurity.ResourceType_Namespaces, ID: "prod", Owner: "owner", Metadata: environment.Metadata}}}, acquired.GetAccessResources())
				return true, nil
			}
			if test.client {
				f.checker.EXPECT().CheckClientPermission(gomock.Any()).After(credential).DoAndReturn(check)
			} else {
				f.checker.EXPECT().CheckConsolePermission(gomock.Any()).After(credential).DoAndReturn(check)
			}
			actor, err := f.authorizer.Authorize(ctx, f.key, test.action)
			require.NoError(t, err)
			require.Equal(t, verifiedPrincipal.String(), actor)
		})
	}
}

func TestUnifiedAuthorizerRejectsInvalidNamespaceBeforePermissions(t *testing.T) {
	for name, ns := range map[string]*types.Namespace{"missing": nil, "deleted": {Name: "prod", Kind: apimodel.NamespaceKind_NAMESPACE_KIND_BUSINESS}, "system kind": {Name: "prod", Valid: true, Kind: apimodel.NamespaceKind_NAMESPACE_KIND_SYSTEM}, "reserved": {Name: "pole-system", Valid: true, Kind: apimodel.NamespaceKind_NAMESPACE_KIND_BUSINESS}, "unknown kind": {Name: "prod", Valid: true, Kind: apimodel.NamespaceKind(99)}} {
		t.Run(name, func(t *testing.T) {
			f := newAuthFixture(t)
			f.environment(ns)
			actor, err := f.authorizer.Authorize(context.Background(), f.key, unified.ActionRead)
			require.ErrorIs(t, err, unified.ErrAccessDenied)
			require.Empty(t, actor)
		})
	}
}

func TestUnifiedAuthorizerNeverTrustsUnverifiedContextIdentity(t *testing.T) {
	for name, principal := range map[string]any{"absent": nil, "anonymous": authtypes.NewAnonymousPrincipal(), "empty": authtypes.Principal{}, "wrong attachment type": "trusted-user"} {
		t.Run(name, func(t *testing.T) {
			f := newAuthFixture(t)
			f.environment(businessNamespace())
			credential := f.users.EXPECT().CheckCredential(gomock.Any()).DoAndReturn(func(acquired *authtypes.AcquireContext) error {
				if principal != nil {
					acquired.SetAttachment(authtypes.PrincipalKey, principal)
				}
				return nil
			})
			f.checker.EXPECT().CheckClientPermission(gomock.Any()).After(credential).Return(true, nil)
			ctx := context.WithValue(context.Background(), types.ContextOperator, "Staffname-admin")
			ctx = context.WithValue(ctx, types.ContextUserIDKey, "forged-id")
			ctx = context.WithValue(ctx, authtypes.PrincipalKey, verifiedPrincipal)
			actor, err := f.authorizer.Authorize(ctx, f.key, unified.ActionRead)
			require.ErrorIs(t, err, unified.ErrAccessDenied)
			require.Empty(t, actor)
		})
	}
}

func TestUnifiedAuthorizerPreservesRefusalVersusDependencyFailures(t *testing.T) {
	dependency := errors.New("credential database unavailable")
	for _, stage := range []string{"credential", "permission"} {
		for _, failure := range []error{authtypes.ErrorNoUser, authtypes.ErrorNoUserGroup, authtypes.ErrorTokenNotExist, authtypes.ErrorTokenInvalid, authtypes.ErrorTokenDisabled, policy.ErrorNotAllowedAccess, policy.ErrorNotPermission, dependency} {
			t.Run(stage+"/"+failure.Error(), func(t *testing.T) {
				f := newAuthFixture(t)
				f.environment(businessNamespace())
				wrapped := fmt.Errorf("wrapped: %w", failure)
				if stage == "credential" {
					f.users.EXPECT().CheckCredential(gomock.Any()).Return(wrapped)
				} else {
					credential := f.users.EXPECT().CheckCredential(gomock.Any()).DoAndReturn(authenticated)
					f.checker.EXPECT().CheckConsolePermission(gomock.Any()).After(credential).Return(false, wrapped)
				}
				actor, err := f.authorizer.Authorize(context.Background(), f.key, unified.ActionManage)
				require.Empty(t, actor)
				if failure == dependency {
					require.ErrorIs(t, err, dependency)
					require.NotErrorIs(t, err, unified.ErrAccessDenied)
				} else {
					require.ErrorIs(t, err, unified.ErrAccessDenied)
				}
			})
		}
	}
	t.Run("explicit false", func(t *testing.T) {
		f := newAuthFixture(t)
		f.environment(businessNamespace())
		f.users.EXPECT().CheckCredential(gomock.Any()).DoAndReturn(authenticated)
		f.checker.EXPECT().CheckClientPermission(gomock.Any()).Return(false, nil)
		_, err := f.authorizer.Authorize(context.Background(), f.key, unified.ActionRead)
		require.ErrorIs(t, err, unified.ErrAccessDenied)
	})
	t.Run("cache unavailable", func(t *testing.T) {
		f := newAuthFixture(t)
		f.caches.EXPECT().Namespace().Return(nil)
		_, err := f.authorizer.Authorize(context.Background(), f.key, unified.ActionRead)
		require.Error(t, err)
		require.NotErrorIs(t, err, unified.ErrAccessDenied)
	})
}

func TestUnifiedAuthorizerStillAuthenticatesWhenPublicPermissionSwitchesAreOff(t *testing.T) {
	for _, action := range []unified.Action{unified.ActionRead, unified.ActionInspect, unified.ActionManage, unified.ActionPreview} {
		t.Run(string(action), func(t *testing.T) {
			f := newAuthFixture(t)
			checker := &policy.DefaultAuthChecker{}
			require.NoError(t, checker.Initialize(&policy.AuthConfig{ClientOpen: false, ConsoleOpen: false}, nil, f.caches, f.users))
			authorizer, err := NewUnifiedAuthorizer(f.caches, f.users, checker)
			require.NoError(t, err)
			f.environment(businessNamespace())
			f.users.EXPECT().CheckCredential(gomock.Any()).Return(authtypes.ErrorTokenInvalid)
			_, err = authorizer.Authorize(context.Background(), f.key, action)
			require.ErrorIs(t, err, unified.ErrAccessDenied)
			f.environment(businessNamespace())
			f.users.EXPECT().CheckCredential(gomock.Any()).DoAndReturn(authenticated)
			actor, err := authorizer.Authorize(context.Background(), f.key, action)
			require.NoError(t, err)
			require.Equal(t, verifiedPrincipal.String(), actor)
		})
	}
}
