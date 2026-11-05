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
	"encoding/json"
	"strings"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	apisecurity "github.com/pole-io/specification/source/go/api/v1/security"

	authapi "github.com/pole-io/pole-server/apis/access_control/auth"
	cachetypes "github.com/pole-io/pole-server/apis/cache"
	"github.com/pole-io/pole-server/apis/pkg/types"
	authtypes "github.com/pole-io/pole-server/apis/pkg/types/auth"
	"github.com/pole-io/pole-server/apis/store"
	api "github.com/pole-io/pole-server/pkg/common/api/v1"
	commonotel "github.com/pole-io/pole-server/pkg/common/otel"
	"github.com/pole-io/pole-server/pkg/common/utils"
	matchs "github.com/pole-io/pole-server/pkg/common/utils/match"
)

var (
	// ErrorNotAllowedAccess 鉴权失败
	ErrorNotAllowedAccess error = errors.New(api.Code2Info(api.NotAllowedAccess))
	// ErrorInvalidParameter 不合法的参数
	ErrorInvalidParameter error = errors.New(api.Code2Info(api.InvalidParameter))
	// ErrorNotPermission .
	ErrorNotPermission = errors.New("no permission")
)

// DefaultAuthChecker pole-server自带的默认鉴权中心
type DefaultAuthChecker struct {
	conf      *AuthConfig
	cacheMgr  cachetypes.CacheManager
	userSvr   authapi.UserServer
	policyMgr *Server
}

// Initialize 执行初始化动作
func (d *DefaultAuthChecker) Initialize(conf *AuthConfig, s store.Store,
	cacheMgr cachetypes.CacheManager, userSvr authapi.UserServer) error {
	d.conf = conf
	d.cacheMgr = cacheMgr
	d.userSvr = userSvr
	return nil
}

func (d *DefaultAuthChecker) SetCacheMgr(mgr cachetypes.CacheManager) {
	d.cacheMgr = mgr
}

func (d *DefaultAuthChecker) GetConfig() *AuthConfig {
	return d.conf
}

func (d *DefaultAuthChecker) SetConfig(conf *AuthConfig) {
	d.conf = conf
}

// Cache 获取缓存统一管理
func (d *DefaultAuthChecker) Cache() cachetypes.CacheManager {
	return d.cacheMgr
}

// IsOpenConsoleAuth 针对控制台是否开启了操作鉴权
func (d *DefaultAuthChecker) IsOpenConsoleAuth() bool {
	return d.conf.ConsoleOpen
}

// IsOpenClientAuth 针对客户端是否开启了操作鉴权
func (d *DefaultAuthChecker) IsOpenClientAuth() bool {
	return d.conf.ClientOpen
}

// IsOpenAuth 返回对于控制台/客户端任意其中的一个是否开启了操作鉴权
func (d *DefaultAuthChecker) IsOpenAuth() bool {
	return d.IsOpenConsoleAuth() || d.IsOpenClientAuth()
}

// AllowResourceOperate 是否允许资源的操作
func (d *DefaultAuthChecker) ResourcePredicate(ctx *authtypes.AcquireContext, res *authtypes.ResourceEntry) bool {
	// 如果是客户端请求，并且鉴权能力没有开启，那就默认都可以进行操作
	if ctx.IsFromClient() && !d.IsOpenClientAuth() {
		return true
	}
	// 如果是控制台请求，并且鉴权能力没有开启，那就默认都可以进行操作
	if ctx.IsFromConsole() && !d.IsOpenConsoleAuth() {
		return true
	}
	if authtypes.ParseUserRole(ctx.GetRequestContext()) == authtypes.OwnerUserRole {
		return true
	}

	p, ok := ctx.GetAttachment(authtypes.PrincipalKey)
	if !ok {
		return false
	}
	policyCache := d.cacheMgr.AuthStrategy()

	principals := d.listAllPrincipals(p.(authtypes.Principal))
	for i := range principals {
		ret := policyCache.Hint(ctx.GetRequestContext(), principals[i], res)
		if ret != apisecurity.AuthAction_DENY {
			return true
		}
	}
	return false
}

// CheckClientPermission 执行检查客户端动作判断是否有权限，并且对 RequestContext 注入操作者数据
func (d *DefaultAuthChecker) CheckClientPermission(preCtx *authtypes.AcquireContext) (allowed bool, err error) {
	originalCtx := preCtx.GetRequestContext()
	spanName := permissionSpanName(permissionSpanPrefix("auth.client", preCtx), preCtx.GetMethods())
	ctx, end := commonotel.StartSpanOnce(originalCtx, spanName, spanName, "auth")
	preCtx.SetRequestContext(ctx)
	defer func() {
		end(err)
	}()
	preCtx.SetFromClient()
	if !d.IsOpenClientAuth() {
		return true, nil
	}
	if d.IsOpenClientAuth() && !d.conf.ClientStrict {
		preCtx.SetAllowAnonymous(true)
	}
	return d.CheckPermission(preCtx)
}

// CheckConsolePermission 执行检查控制台动作判断是否有权限，并且对 RequestContext 注入操作者数据
func (d *DefaultAuthChecker) CheckConsolePermission(preCtx *authtypes.AcquireContext) (allowed bool, err error) {
	originalCtx := preCtx.GetRequestContext()
	spanName := permissionSpanName(permissionSpanPrefix("auth.console", preCtx), preCtx.GetMethods())
	ctx, end := commonotel.StartSpanOnce(originalCtx, spanName, spanName, "auth")
	preCtx.SetRequestContext(ctx)
	defer func() {
		end(err)
	}()
	preCtx.SetFromConsole()
	if !d.IsOpenConsoleAuth() {
		return true, nil
	}
	if d.IsOpenConsoleAuth() && !d.conf.ConsoleStrict {
		preCtx.SetAllowAnonymous(true)
	}
	// 如果是初始化主用户的请求，直接放行
	if authapi.IsInitMainUser(preCtx.GetRequestContext()) {
		return true, nil
	}
	return d.CheckPermission(preCtx)
}

// CheckPermission 执行检查动作判断是否有权限
func (d *DefaultAuthChecker) CheckPermission(authCtx *authtypes.AcquireContext) (bool, error) {
	if err := d.userSvr.CheckCredential(authCtx); err != nil {
		log.Error("[Auth][Checker] check credential failed", utils.RequestID(authCtx.GetRequestContext()),
			zap.Error(err), zap.Any("method", authCtx.GetMethods()), zap.Any("module", authCtx.GetModule()))
		return false, err
	}
	if authtypes.ParseUserRole(authCtx.GetRequestContext()) == authtypes.OwnerUserRole {
		return true, nil
	}
	if log.DebugEnabled() {
		log.Debug("[Auth][Checker] check permission args", utils.RequestID(authCtx.GetRequestContext()),
			zap.Any("method", authCtx.GetMethods()), zap.Any("resources", authCtx.GetAccessResources()))
	}

	if pass, _ := d.doCheckPermission(authCtx); pass {
		return true, nil
	}

	// 触发缓存的同步，避免鉴权策略和角色信息不一致导致的权限检查失败
	if err := d.resyncData(authCtx); err != nil {
		return false, err
	}
	pass, err := d.doCheckPermission(authCtx)
	if err != nil {
		log.Error("[Auth][Checker] check permission failed", utils.RequestID(authCtx.GetRequestContext()),
			zap.Error(err), zap.Any("method", authCtx.GetMethods()), zap.Any("module", authCtx.GetModule()))
	}
	return pass, err
}

func permissionSpanName(prefix string, methods []authtypes.ServerFunctionName) string {
	if len(methods) == 0 {
		return prefix + ".check"
	}
	names := make([]string, 0, len(methods))
	for _, method := range methods {
		names = append(names, string(method))
	}
	return prefix + "." + strings.Join(names, "+")
}

func permissionSpanPrefix(prefix string, authCtx *authtypes.AcquireContext) string {
	purpose := "authorize"
	if value, ok := authCtx.GetAttachment(authtypes.PermissionCheckPurposeKey); ok &&
		value == authtypes.PermissionCheckPurposeCapability {
		purpose = authtypes.PermissionCheckPurposeCapability
	}
	return prefix + "." + purpose
}

func (d *DefaultAuthChecker) resyncData(authCtx *authtypes.AcquireContext) error {
	if err := d.cacheMgr.AuthStrategy().Update(); err != nil {
		log.Error("[Auth][Checker] force sync policy failed", utils.RequestID(authCtx.GetRequestContext()), zap.Error(err))
		return err
	}
	if err := d.cacheMgr.Role().Update(); err != nil {
		log.Error("[Auth][Checker] force sync role failed", utils.RequestID(authCtx.GetRequestContext()), zap.Error(err))
		return err
	}
	if err := d.cacheMgr.User().Update(); err != nil {
		log.Error("[Auth][Checker] force sync user failed", utils.RequestID(authCtx.GetRequestContext()), zap.Error(err))
		return err
	}
	return nil
}

// doCheckPermission 执行权限检查
func (d *DefaultAuthChecker) doCheckPermission(authCtx *authtypes.AcquireContext) (bool, error) {
	if d.IsCredible(authCtx) {
		return true, nil
	}
	cur := authCtx.GetAttachments()[authtypes.PrincipalKey].(authtypes.Principal)

	principals := d.listAllPrincipals(cur)

	// 遍历所有的 principal，检查是否有一个符合要求
	for i := range principals {
		principal := principals[i]
		allowPolicies := d.cacheMgr.AuthStrategy().GetPrincipalPolicies("allow", principal)
		denyPolicies := d.cacheMgr.AuthStrategy().GetPrincipalPolicies("deny", principal)

		resources := authCtx.GetAccessResources()

		// 先执行 deny 策略
		for i := range denyPolicies {
			item := denyPolicies[i]
			if d.MatchPolicy(authCtx, item, principal, resources) {
				return false, ErrorNotPermission
			}
		}

		// 处理 allow 策略，只要有一个放开，就可以认为通过
		for i := range allowPolicies {
			item := allowPolicies[i]
			if d.MatchPolicy(authCtx, item, principal, resources) {
				return true, nil
			}
		}
	}
	return false, ErrorNotPermission
}

func (d *DefaultAuthChecker) listAllPrincipals(p authtypes.Principal) []authtypes.Principal {
	principals := make([]authtypes.Principal, 0, 4)
	principals = append(principals, p)
	// 获取角色列表
	roles := d.cacheMgr.Role().GetPrincipalRoles(p)
	for i := range roles {
		principals = append(principals, authtypes.Principal{
			PrincipalID:   roles[i].ID,
			PrincipalType: authtypes.PrincipalRole,
		})
	}

	// 如果是用户，获取所在的用户组列表
	if p.PrincipalType == authtypes.PrincipalUser {
		groups := d.cacheMgr.User().GetUserLinkGroupIds(p.PrincipalID)
		for i := range groups {
			principals = append(principals, authtypes.Principal{
				PrincipalID:   groups[i],
				PrincipalType: authtypes.PrincipalGroup,
			})
		}
	}
	return principals
}

// IsCredible 检查是否是可信的请求
func (d *DefaultAuthChecker) IsCredible(authCtx *authtypes.AcquireContext) bool {
	reqHeaders := types.GetRequestHeader(authCtx.GetRequestContext())
	if len(d.conf.CredibleHeaders) == 0 {
		return false
	}
	matched := true
	for k, v := range d.conf.CredibleHeaders {
		val, exist := reqHeaders[strings.ToLower(k)]
		if !exist {
			matched = false
			break
		}
		if len(val) == 0 {
			matched = false
		}
		matched = v == val[0]
		if !matched {
			break
		}
	}
	return matched
}

// MatchPolicy 检查策略是否匹配
func (d *DefaultAuthChecker) MatchPolicy(authCtx *authtypes.AcquireContext, policy *authtypes.StrategyDetail,
	principal authtypes.Principal, resources map[apisecurity.ResourceType][]authtypes.ResourceEntry) bool {
	if !d.MatchCalleeFunctions(authCtx, principal, policy) {
		log.Error("server function match policy fail", utils.RequestID(authCtx.GetRequestContext()),
			zap.String("principal", principal.String()), zap.String("policy-id", policy.ID))
		return false
	}
	if !d.MatchResourceOperateable(authCtx, principal, policy) {
		log.Error("access resource match policy fail", utils.RequestID(authCtx.GetRequestContext()),
			zap.String("principal", principal.String()), zap.String("policy-id", policy.ID))
		return false
	}
	return true
}

// MatchCalleeFunctions 检查操作方法是否和策略匹配
func (d *DefaultAuthChecker) MatchCalleeFunctions(authCtx *authtypes.AcquireContext,
	principal authtypes.Principal, policy *authtypes.StrategyDetail) bool {

	// 如果开启了兼容模式，并且策略没有对可调用方法的拦截，那么就认为匹配成功
	if d.conf.Compatible && len(policy.CalleeMethods) == 0 {
		return true
	}

	functions := policy.CalleeMethods

	allMatch := 0
	for _, method := range authCtx.GetMethods() {
		curMatch := false
		for i := range functions {
			if matchs.IsMatchAll(functions[i]) {
				return true
			}
			if functions[i] == string(method) {
				curMatch = true
				break
			}
			if matchs.IsWildMatch(string(method), functions[i]) {
				curMatch = true
				break
			}
		}
		if curMatch {
			allMatch++
		}
	}
	return allMatch == len(authCtx.GetMethods())
}

type (
	compatibleChecker func(ctx context.Context, cacheSvr cachetypes.CacheManager, resource *authtypes.ResourceEntry) bool
)

var (
	compatibleResource = map[apisecurity.ResourceType]compatibleChecker{
		apisecurity.ResourceType_UserGroups: func(ctx context.Context, cacheSvr cachetypes.CacheManager,
			resource *authtypes.ResourceEntry) bool {
			saveVal := cacheSvr.User().GetGroup(resource.ID)
			if saveVal == nil {
				return false
			}
			operator := utils.ParseUserID(ctx)
			_, exist := saveVal.UserIds[operator]
			return exist
		},
		apisecurity.ResourceType_PolicyRules: func(ctx context.Context, cacheSvr cachetypes.CacheManager,
			resource *authtypes.ResourceEntry) bool {
			saveVal := cacheSvr.AuthStrategy().GetPolicyRule(resource.ID)
			if saveVal == nil {
				return false
			}
			operator := utils.ParseUserID(ctx)
			for i := range saveVal.Principals {
				if saveVal.Principals[i].PrincipalID == operator {
					return true
				}
			}
			return false
		},
		apisecurity.ResourceType_Namespaces: func(ctx context.Context, cacheSvr cachetypes.CacheManager,
			resource *authtypes.ResourceEntry) bool {
			saveVal := cacheSvr.Namespace().GetNamespace(resource.ID)
			return saveVal != nil
		},
		apisecurity.ResourceType_Services: func(ctx context.Context, cacheSvr cachetypes.CacheManager,
			resource *authtypes.ResourceEntry) bool {
			saveVal := cacheSvr.Service().GetServiceByID(resource.ID)
			return saveVal != nil
		},
		apisecurity.ResourceType_MCPServerResources: func(ctx context.Context, cacheSvr cachetypes.CacheManager,
			resource *authtypes.ResourceEntry) bool {
			saveVal := cacheSvr.MCPServer().GetMCPServerByID(resource.ID)
			return saveVal != nil
		},
		apisecurity.ResourceType_A2AAgentResources: func(ctx context.Context, cacheSvr cachetypes.CacheManager,
			resource *authtypes.ResourceEntry) bool {
			saveVal := cacheSvr.A2AAgent().GetA2AAgentByID(resource.ID)
			return saveVal != nil
		},
		apisecurity.ResourceType_Users: func(ctx context.Context, cacheSvr cachetypes.CacheManager,
			resource *authtypes.ResourceEntry) bool {
			saveVal := cacheSvr.User().GetUserByID(resource.ID)
			return saveVal != nil
		},
		apisecurity.ResourceType_Roles: func(ctx context.Context, cacheSvr cachetypes.CacheManager,
			resource *authtypes.ResourceEntry) bool {
			saveVal := cacheSvr.Role().GetRole(resource.ID)
			return saveVal != nil
		},
		apisecurity.ResourceType_RouteRules: func(ctx context.Context, cacheSvr cachetypes.CacheManager,
			resource *authtypes.ResourceEntry) bool {
			saveVal := cacheSvr.RoutingConfig().GetRule(resource.ID)
			return saveVal != nil
		},
		apisecurity.ResourceType_RateLimitRules: func(ctx context.Context, cacheSvr cachetypes.CacheManager,
			resource *authtypes.ResourceEntry) bool {
			saveVal := cacheSvr.RateLimit().GetRule(resource.ID)
			return saveVal != nil
		},
		apisecurity.ResourceType_CircuitBreakerRules: func(ctx context.Context, cacheSvr cachetypes.CacheManager,
			resource *authtypes.ResourceEntry) bool {
			saveVal := cacheSvr.CircuitBreaker().GetRule(resource.ID)
			return saveVal != nil
		},
		apisecurity.ResourceType_FaultDetectRules: func(ctx context.Context, cacheSvr cachetypes.CacheManager,
			resource *authtypes.ResourceEntry) bool {
			saveVal := cacheSvr.FaultDetector().GetRule(resource.ID)
			return saveVal != nil
		},
		apisecurity.ResourceType_LaneRules: func(ctx context.Context, cacheSvr cachetypes.CacheManager,
			resource *authtypes.ResourceEntry) bool {
			saveVal := cacheSvr.LaneRule().GetRule(resource.ID)
			return saveVal != nil
		},
		apisecurity.ResourceType_LosslessRules: func(ctx context.Context, cacheSvr cachetypes.CacheManager,
			resource *authtypes.ResourceEntry) bool {
			saveVal := cacheSvr.Lossless().GetRule(resource.ID)
			return saveVal != nil
		},
		apisecurity.ResourceType_SecurityRules: func(ctx context.Context, cacheSvr cachetypes.CacheManager,
			resource *authtypes.ResourceEntry) bool {
			saveVal := cacheSvr.TrafficSecurity().GetRule(resource.ID)
			return saveVal != nil
		},
		apisecurity.ResourceType_MirrorRules: func(ctx context.Context, cacheSvr cachetypes.CacheManager,
			resource *authtypes.ResourceEntry) bool {
			saveVal := cacheSvr.TrafficMirror().GetRule(resource.ID)
			return saveVal != nil
		},
		apisecurity.ResourceType_MockRules: func(ctx context.Context, cacheSvr cachetypes.CacheManager,
			resource *authtypes.ResourceEntry) bool {
			saveVal := cacheSvr.TrafficMock().GetRule(resource.ID)
			return saveVal != nil
		},
	}
)

// checkAction 检查操作资源是否和策略匹配
func (d *DefaultAuthChecker) MatchResourceOperateable(authCtx *authtypes.AcquireContext,
	principal authtypes.Principal, policy *authtypes.StrategyDetail) bool {

	// 检查下 principal 有没有 condition 信息
	principalCondition := make([]authtypes.Condition, 0, 4)
	// 这里主要兼容一些内部特殊场景，可能在 role/user/group 关联某个策略时，会有一些额外的关系属性，这里在 extend 统一查找
	_ = json.Unmarshal([]byte(principal.Extend["condition"]), &principalCondition)

	ctx := context.Background()
	if len(principalCondition) != 0 {
		ctx = context.WithValue(context.Background(), types.ContextKeyConditions, principalCondition)
	}

	matchCheck := func(resType apisecurity.ResourceType, resources []authtypes.ResourceEntry) bool {
		for i := range resources {
			actionResult := d.cacheMgr.AuthStrategy().Hint(ctx, principal, &resources[i])
			if policy.IsMatchAction(actionResult.String()) {
				return true
			}
			// 兼容模式下，对于用户组和策略规则，走一遍兜底的检查逻辑
			if _, ok := compatibleResource[resType]; ok && d.conf.Compatible {
				return true
			}
		}
		return false
	}

	reqRes := authCtx.GetAccessResources()
	isMatch := true
	for k, v := range reqRes {
		subMatch := matchCheck(k, v)
		isMatch = isMatch && subMatch
	}
	return isMatch
}
