package config

import (
	"context"
	"errors"
	"fmt"

	configv1 "github.com/pole-io/specification/source/go/api/v1/config"
	apimodel "github.com/pole-io/specification/source/go/api/v1/model"
	apisecurity "github.com/pole-io/specification/source/go/api/v1/security"

	authapi "github.com/pole-io/pole-server/apis/access_control/auth"
	cacheapi "github.com/pole-io/pole-server/apis/cache"
	authtypes "github.com/pole-io/pole-server/apis/pkg/types/auth"
	"github.com/pole-io/pole-server/pkg/config/unified"
	"github.com/pole-io/pole-server/pkg/namespace"
	"github.com/pole-io/pole-server/plugin/access_control/auth/policy"
)

// NewUnifiedAuthorizer 按配置环境映射真实 Namespace 资源，所有调用均重新认证与授权。
func NewUnifiedAuthorizer(caches cacheapi.CacheManager, users authapi.UserServer, checker authapi.AuthChecker) (unified.Authorizer, error) {
	if caches == nil || users == nil || checker == nil {
		return nil, errors.New("configuration authentication is unavailable")
	}
	return &unifiedAuthorizer{caches: caches, users: users, checker: checker}, nil
}

type unifiedAuthorizer struct {
	caches  cacheapi.CacheManager
	users   authapi.UserServer
	checker authapi.AuthChecker
}

func (a *unifiedAuthorizer) Authorize(ctx context.Context, key *configv1.ConfigKey, action unified.Action) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	namespaces := a.caches.Namespace()
	if namespaces == nil {
		return "", errors.New("configuration namespace cache is unavailable")
	}
	environment := namespaces.GetNamespace(key.GetEnvironment())
	if environment == nil || !environment.Valid || environment.Name == namespace.SystemNamespace || environment.Kind != apimodel.NamespaceKind_NAMESPACE_KIND_BUSINESS {
		return "", unified.ErrAccessDenied
	}
	resource := authtypes.ResourceEntry{Type: apisecurity.ResourceType_Namespaces, ID: environment.Name, Owner: environment.Owner, Metadata: environment.Metadata}
	operation, method := authtypes.Modify, authtypes.ManageConfig
	switch action {
	case unified.ActionRead:
		operation, method = authtypes.Read, authtypes.DiscoverConfig
	case unified.ActionInspect:
		operation, method = authtypes.Read, authtypes.InspectConfig
	case unified.ActionManage:
	case unified.ActionPreview:
		method = authtypes.PreviewConfig
	default:
		return "", unified.ErrAccessDenied
	}
	acquired := authtypes.NewAcquireContext(authtypes.WithRequestContext(ctx), authtypes.WithModule(authtypes.ConfigModule), authtypes.WithOperation(operation), authtypes.WithMethod(method), authtypes.WithAccessResources(map[apisecurity.ResourceType][]authtypes.ResourceEntry{resource.Type: {resource}}))
	if action == unified.ActionRead {
		acquired.SetFromClient()
	}
	// 认证必须先于权限开关判断，审计身份不得来自 Staffname 或请求标签。
	if err := a.users.CheckCredential(acquired); err != nil {
		return "", unifiedAuthError(err)
	}
	var allowed bool
	var err error
	if action == unified.ActionRead {
		allowed, err = a.checker.CheckClientPermission(acquired)
	} else {
		allowed, err = a.checker.CheckConsolePermission(acquired)
	}
	if err != nil {
		return "", unifiedAuthError(err)
	}
	if !allowed {
		return "", unified.ErrAccessDenied
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	principal, ok := acquired.GetAttachment(authtypes.PrincipalKey)
	identity, valid := principal.(authtypes.Principal)
	if !ok || !valid || identity.PrincipalID == "" || identity.PrincipalID == authtypes.NewAnonymousPrincipal().PrincipalID {
		return "", unified.ErrAccessDenied
	}
	return identity.String(), nil
}

func unifiedAuthError(err error) error {
	for _, refusal := range []error{authtypes.ErrorNoUser, authtypes.ErrorNoUserGroup, authtypes.ErrorTokenNotExist, authtypes.ErrorTokenInvalid, authtypes.ErrorTokenDisabled, policy.ErrorNotAllowedAccess, policy.ErrorNotPermission} {
		if errors.Is(err, refusal) {
			return fmt.Errorf("%w: authentication or permission refused", unified.ErrAccessDenied)
		}
	}
	return err
}
