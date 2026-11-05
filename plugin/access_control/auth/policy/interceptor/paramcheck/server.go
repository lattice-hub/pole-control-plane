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

package paramcheck

import (
	"context"
	"strconv"

	"go.uber.org/zap"

	apimodel "github.com/pole-io/specification/source/go/api/v1/model"
	apisecurity "github.com/pole-io/specification/source/go/api/v1/security"

	authapi "github.com/pole-io/pole-server/apis/access_control/auth"
	cachetypes "github.com/pole-io/pole-server/apis/cache"
	authtypes "github.com/pole-io/pole-server/apis/pkg/types/auth"
	"github.com/pole-io/pole-server/apis/store"
	storeapi "github.com/pole-io/pole-server/apis/store"
	api "github.com/pole-io/pole-server/pkg/common/api/v1"
	v1 "github.com/pole-io/pole-server/pkg/common/api/v1"
	"github.com/pole-io/pole-server/pkg/common/log"
	"github.com/pole-io/pole-server/pkg/common/utils"
	"github.com/pole-io/pole-server/pkg/common/utils/valid"
)

var (
	// StrategyFilterAttributes strategy filter attributes
	StrategyFilterAttributes = map[string]bool{
		"id":             true,
		"name":           true,
		"owner":          true,
		"offset":         true,
		"limit":          true,
		"principal_id":   true,
		"principal_type": true,
		"res_id":         true,
		"res_type":       true,
		"default":        true,
		"show_detail":    true,
		"action":         true,
	}
)

func NewServer(nextSvr authapi.StrategyServer) authapi.StrategyServer {
	return &Server{
		nextSvr: nextSvr,
	}
}

type Server struct {
	storage  store.Store
	cacheMgr cachetypes.CacheManager
	nextSvr  authapi.StrategyServer
	userSvr  authapi.UserServer
}

// PolicyHelper implements authapi.StrategyServer.
func (svr *Server) PolicyHelper() authapi.PolicyHelper {
	return svr.nextSvr.PolicyHelper()
}

// Initialize 执行初始化动作
func (svr *Server) Initialize(options *authapi.Config, storage store.Store, cacheMgr cachetypes.CacheManager, userSvr authapi.UserServer) error {
	svr.userSvr = userSvr
	svr.cacheMgr = cacheMgr
	svr.storage = storage
	return svr.nextSvr.Initialize(options, storage, cacheMgr, userSvr)
}

// Name 策略管理server名称
func (svr *Server) Name() string {
	return svr.nextSvr.Name()
}

// CreateStrategy 创建策略
func (svr *Server) CreatePolicies(ctx context.Context, reqs []*apisecurity.AuthStrategy) *apimodel.BatchWriteResponse {
	for i := range reqs {
		rsp := svr.checkCreateStrategy(reqs[i])
		if rsp != nil {
			log.Error("[Auth][Strategy] check create strategy", utils.RequestID(ctx), zap.String("msg", rsp.GetInfo()))
			return api.NewBatchWriteResponseWithMsg(apimodel.Code(rsp.GetCode()), rsp.GetInfo())
		}
	}
	return svr.nextSvr.CreatePolicies(ctx, reqs)
}

// UpdateStrategies 批量更新策略
func (svr *Server) UpdatePolicies(ctx context.Context, reqs []*apisecurity.AuthStrategy) *apimodel.BatchWriteResponse {
	batchResp := api.NewBatchWriteResponse(apimodel.Code_ExecuteSuccess)
	for i := range reqs {
		var rsp *apimodel.Response
		strategy, err := svr.storage.GetStrategyDetail(reqs[i].GetId())
		if err != nil {
			log.Error("[Auth][Strategy] get strategy from store", utils.RequestID(ctx), zap.Error(err))
			rsp = api.NewAuthStrategyResponse(storeapi.StoreCode2APICode(err), reqs[i])
		} else if strategy == nil {
			rsp = api.NewAuthStrategyResponse(apimodel.Code_NotFoundAuthStrategyRule, reqs[i])
		} else {
			rsp = svr.checkUpdateStrategy(ctx, reqs[i], strategy)
		}
		api.Collect(batchResp, rsp)
	}
	if !api.IsSuccess(batchResp) {
		return batchResp
	}
	return svr.nextSvr.UpdatePolicies(ctx, reqs)
}

// DeleteStrategies 删除策略
func (svr *Server) DeletePolicies(ctx context.Context, reqs []*apisecurity.AuthStrategy) *apimodel.BatchWriteResponse {
	return svr.nextSvr.DeletePolicies(ctx, reqs)
}

// GetPolicies 获取资源列表
// support 1. 支持按照 principal-id + principal-role 进行查询
// support 2. 支持普通的鉴权策略查询
func (svr *Server) GetPolicies(ctx context.Context, query map[string]string) *apimodel.BatchQueryResponse {
	log.Debug("[Auth][Strategy] origin get strategies query params", utils.RequestID(ctx), zap.Any("query", query))

	searchFilters := make(map[string]string, len(query))
	for key, value := range query {
		if _, ok := StrategyFilterAttributes[key]; !ok {
			log.Errorf("[Auth][Strategy] get strategies attribute(%s) it not allowed", key)
			return api.NewAuthBatchQueryResponseWithMsg(apimodel.Code_InvalidParameter, key+" is not allowed")
		}
		// 过滤掉空值
		if value == "" {
			continue
		}
		searchFilters[key] = value
	}

	offset, limit, err := valid.ParseOffsetAndLimit(searchFilters)

	if err != nil {
		return api.NewAuthBatchQueryResponse(apimodel.Code_InvalidParameter)
	}
	searchFilters["offset"] = strconv.FormatUint(uint64(offset), 10)
	searchFilters["limit"] = strconv.FormatUint(uint64(limit), 10)
	return svr.nextSvr.GetPolicies(ctx, searchFilters)
}

// GetPolicy 获取策略详细
func (svr *Server) GetPolicy(ctx context.Context, strategy *apisecurity.AuthStrategy) *apimodel.Response {
	return svr.nextSvr.GetPolicy(ctx, strategy)
}

// GetPrincipalResources 获取某个 principal 的所有可操作资源列表
func (svr *Server) GetPrincipalResources(ctx context.Context, query map[string]string) *apimodel.Response {
	return svr.nextSvr.GetPrincipalResources(ctx, query)
}

func (svr *Server) GetResourcePrincipals(ctx context.Context, query map[string]string) *apimodel.Response {
	if len(query) == 0 {
		return api.NewResponse(apimodel.Code_EmptyRequest)
	}
	log.Debug("[Auth][Strategy] origin get resource principals query params", utils.RequestID(ctx), zap.Any("query", query))

	searchFilters := make(map[string]string, len(query))
	for key, value := range query {
		if _, ok := StrategyFilterAttributes[key]; !ok {
			log.Errorf("[Auth][Strategy] get resource principals attribute(%s) it not allowed", key)
			return api.NewResponseWithMsg(apimodel.Code_InvalidParameter, key+" is not allowed")
		}
		// 过滤掉空值
		if value == "" {
			continue
		}
		searchFilters[key] = value
	}
	return svr.nextSvr.GetResourcePrincipals(ctx, searchFilters)
}

// AuthorizeResources 授权资源
func (svr *Server) AuthorizeResources(ctx context.Context, reqs []*v1.AuthorizeResources) *apimodel.Response {
	return svr.nextSvr.AuthorizeResources(ctx, reqs)
}

// GetAuthChecker 获取鉴权检查器
func (svr *Server) GetAuthChecker() authapi.AuthChecker {
	return svr.nextSvr.GetAuthChecker()
}

// AfterResourceOperation 操作完资源的后置处理逻辑
func (svr *Server) AfterResourceOperation(afterCtx *authtypes.AcquireContext) error {
	return svr.nextSvr.AfterResourceOperation(afterCtx)
}

// CreateRoles 批量创建角色
func (svr *Server) CreateRoles(ctx context.Context, reqs []*apisecurity.Role) *apimodel.BatchWriteResponse {
	return svr.nextSvr.CreateRoles(ctx, reqs)
}

// UpdateRoles 批量更新角色
func (svr *Server) UpdateRoles(ctx context.Context, reqs []*apisecurity.Role) *apimodel.BatchWriteResponse {
	return svr.nextSvr.UpdateRoles(ctx, reqs)
}

// DeleteRoles 批量删除角色
func (svr *Server) DeleteRoles(ctx context.Context, reqs []*apisecurity.Role) *apimodel.BatchWriteResponse {
	return svr.nextSvr.DeleteRoles(ctx, reqs)
}

// GetRoles 查询角色列表
func (svr *Server) GetRoles(ctx context.Context, query map[string]string) *apimodel.BatchQueryResponse {
	return svr.nextSvr.GetRoles(ctx, query)
}

// checkCreateStrategy 检查创建鉴权策略的请求
func (svr *Server) checkCreateStrategy(req *apisecurity.AuthStrategy) *apimodel.Response {
	// 检查名称信息
	if err := CheckName(req.Name); err != nil {
		return api.NewAuthStrategyResponse(apimodel.Code_InvalidUserName, req)
	}
	// 检查用户是否存在
	if err := svr.checkUserExist(convertPrincipalsToUsers(req.GetPrincipals())); err != nil {
		return api.NewAuthStrategyResponse(apimodel.Code_NotFoundUser, req)
	}
	// 检查用户组是否存在
	if err := svr.checkGroupExist(convertPrincipalsToGroups(req.GetPrincipals())); err != nil {
		return api.NewAuthStrategyResponse(apimodel.Code_NotFoundUserGroup, req)
	}
	if hasBuiltInRolePrincipal(req.GetPrincipals()) {
		return api.NewAuthStrategyResponseWithMsg(apimodel.Code_BadRequest,
			"built-in role permissions are immutable", req)
	}
	// 检查资源是否存在
	if errResp := svr.checkResourceExist(req.GetResources()); errResp != nil {
		return errResp
	}
	return nil
}

// checkUpdateStrategy 检查更新鉴权策略的请求
// Case 1. 修改的是默认鉴权策略的话，只能修改资源，不能添加用户 or 用户组
// Case 2. 鉴权策略只能被自己的 owner 对应的用户修改
func (svr *Server) checkUpdateStrategy(ctx context.Context, req *apisecurity.AuthStrategy,
	saved *authtypes.StrategyDetail) *apimodel.Response {
	if authtypes.IsSystemRolePolicyID(saved.ID) {
		return api.NewAuthStrategyResponseWithMsg(apimodel.Code_BadRequest,
			"built-in role policy is immutable", req)
	}
	if saved.Default {
		if len(req.GetPrincipals().GetUsers()) != 0 ||
			len(req.GetPrincipals().GetGroups()) != 0 {
			return api.NewAuthStrategyResponse(apimodel.Code_NotAllowModifyDefaultStrategyPrincipal, req)
		}

		// 主账户的默认策略禁止编辑
		if len(saved.Principals) == 1 && saved.Principals[0].PrincipalType == authtypes.PrincipalUser {
			if saved.Principals[0].PrincipalID == utils.ParseOwnerID(ctx) {
				return api.NewAuthResponse(apimodel.Code_NotAllowModifyOwnerDefaultStrategy)
			}
		}
	}

	// 检查用户是否存在
	if err := svr.checkUserExist(convertPrincipalsToUsers(req.GetPrincipals())); err != nil {
		return api.NewAuthStrategyResponse(apimodel.Code_NotFoundUser, req)
	}
	// 检查用户组是否存在
	if err := svr.checkGroupExist(convertPrincipalsToGroups(req.GetPrincipals())); err != nil {
		return api.NewAuthStrategyResponse(apimodel.Code_NotFoundUserGroup, req)
	}
	if hasBuiltInRolePrincipal(req.GetPrincipals()) {
		return api.NewAuthStrategyResponseWithMsg(apimodel.Code_BadRequest,
			"built-in role permissions are immutable", req)
	}
	// 检查资源是否存在
	if errResp := svr.checkResourceExist(req.GetResources()); errResp != nil {
		return errResp
	}
	return nil
}

// checkUserExist 检查用户是否存在
func (svr *Server) checkUserExist(users []*apisecurity.User) error {
	if len(users) == 0 {
		return nil
	}
	return svr.userSvr.GetUserHelper().CheckUsersExist(context.TODO(), users)
}

// checkUserGroupExist 检查用户组是否存在
func (svr *Server) checkGroupExist(groups []*apisecurity.UserGroup) error {
	if len(groups) == 0 {
		return nil
	}
	return svr.userSvr.GetUserHelper().CheckGroupsExist(context.TODO(), groups)
}

// checkResourceExist 检查资源是否存在
func (svr *Server) checkResourceExist(resources *apisecurity.StrategyResources) *apimodel.Response {
	// Namespaces
	namespaces := resources.GetNamespaces()
	nsCache := svr.cacheMgr.Namespace()
	for index := range namespaces {
		val := namespaces[index]
		if val.GetId() == "*" {
			continue
		}
		if ns := nsCache.GetNamespace(val.GetId()); ns == nil {
			return api.NewAuthResponse(apimodel.Code_NotFoundResource)
		}
	}

	// Services
	services := resources.GetServices()
	svcCache := svr.cacheMgr.Service()
	for index := range services {
		val := services[index]
		if val.GetId() == "*" {
			continue
		}
		if svc := svcCache.GetServiceByID(val.GetId()); svc == nil {
			return api.NewAuthResponse(apimodel.Code_NotFoundResource)
		}
	}

	// 统一配置按环境授权，独立旧配置分组资源已移除；拒绝创建无法生效的授权。
	if len(resources.GetConfigGroups()) != 0 {
		return api.NewAuthResponse(apimodel.Code_InvalidParameter)
	}

	// RouteRules
	routeRules := resources.GetRouteRules()
	routingCache := svr.cacheMgr.RoutingConfig()
	for index := range routeRules {
		val := routeRules[index]
		if val.GetId() == "*" {
			continue
		}
		if rule := routingCache.GetRule(val.GetId()); rule == nil {
			return api.NewAuthResponse(apimodel.Code_NotFoundResource)
		}
	}

	// RatelimitRules
	ratelimitRules := resources.GetRatelimitRules()
	ratelimitCache := svr.cacheMgr.RateLimit()
	for index := range ratelimitRules {
		val := ratelimitRules[index]
		if val.GetId() == "*" {
			continue
		}
		if rule := ratelimitCache.GetRule(val.GetId()); rule == nil {
			return api.NewAuthResponse(apimodel.Code_NotFoundResource)
		}
	}

	// CircuitBreakerRules
	circuitBreakerRules := resources.GetCircuitbreakerRules()
	cbCache := svr.cacheMgr.CircuitBreaker()
	for index := range circuitBreakerRules {
		val := circuitBreakerRules[index]
		if val.GetId() == "*" {
			continue
		}
		if rule := cbCache.GetRule(val.GetId()); rule == nil {
			return api.NewAuthResponse(apimodel.Code_NotFoundResource)
		}
	}

	// FaultdetectRules
	faultDetectRules := resources.GetFaultdetectRules()
	fdCache := svr.cacheMgr.FaultDetector()
	for index := range faultDetectRules {
		val := faultDetectRules[index]
		if val.GetId() == "*" {
			continue
		}
		if rule := fdCache.GetRule(val.GetId()); rule == nil {
			return api.NewAuthResponse(apimodel.Code_NotFoundResource)
		}
	}

	// LaneRules
	laneRules := resources.GetLaneRules()
	laneCache := svr.cacheMgr.LaneRule()
	for index := range laneRules {
		val := laneRules[index]
		if val.GetId() == "*" {
			continue
		}
		if rule := laneCache.GetRule(val.GetId()); rule == nil {
			return api.NewAuthResponse(apimodel.Code_NotFoundResource)
		}
	}

	// LosslessRules
	losslessRules := resources.GetLosslessRules()
	losslessCache := svr.cacheMgr.Lossless()
	for index := range losslessRules {
		val := losslessRules[index]
		if val.GetId() == "*" {
			continue
		}
		if rule := losslessCache.GetRule(val.GetId()); rule == nil {
			return api.NewAuthResponse(apimodel.Code_NotFoundResource)
		}
	}

	if resp := checkTrafficGovernanceResourceExist(resources.GetSecurityRules(), svr.cacheMgr.TrafficSecurity()); resp != nil {
		return resp
	}
	if resp := checkTrafficGovernanceResourceExist(resources.GetMirrorRules(), svr.cacheMgr.TrafficMirror()); resp != nil {
		return resp
	}
	if resp := checkTrafficGovernanceResourceExist(resources.GetMockRules(), svr.cacheMgr.TrafficMock()); resp != nil {
		return resp
	}
	if resp := checkResourceEntriesExist(resources.GetMcpServers(), func(id string) bool {
		return svr.cacheMgr.MCPServer().GetMCPServerByID(id) != nil
	}); resp != nil {
		return resp
	}
	if resp := checkResourceEntriesExist(resources.GetA2AAgents(), func(id string) bool {
		return svr.cacheMgr.A2AAgent().GetA2AAgentByID(id) != nil
	}); resp != nil {
		return resp
	}

	return nil
}

func checkTrafficGovernanceResourceExist(resources []*apisecurity.StrategyResourceEntry,
	cache cachetypes.TrafficGovernanceCache) *apimodel.Response {
	return checkResourceEntriesExist(resources, func(id string) bool {
		return cache.GetRule(id) != nil
	})
}

func checkResourceEntriesExist(resources []*apisecurity.StrategyResourceEntry, exists func(string) bool) *apimodel.Response {
	for index := range resources {
		val := resources[index]
		if val.GetId() == "*" {
			continue
		}
		if !exists(val.GetId()) {
			return api.NewAuthResponse(apimodel.Code_NotFoundResource)
		}
	}
	return nil
}

func convertPrincipalsToUsers(principals *apisecurity.Principals) []*apisecurity.User {
	if principals == nil {
		return make([]*apisecurity.User, 0)
	}

	users := make([]*apisecurity.User, 0, len(principals.Users))
	for k := range principals.GetUsers() {
		user := principals.GetUsers()[k]
		users = append(users, &apisecurity.User{
			Id: user.Id,
		})
	}

	return users
}

func convertPrincipalsToGroups(principals *apisecurity.Principals) []*apisecurity.UserGroup {
	if principals == nil {
		return make([]*apisecurity.UserGroup, 0)
	}

	groups := make([]*apisecurity.UserGroup, 0, len(principals.Groups))
	for k := range principals.GetGroups() {
		group := principals.GetGroups()[k]
		groups = append(groups, &apisecurity.UserGroup{
			Id: group.Id,
		})
	}

	return groups
}

func hasBuiltInRolePrincipal(principals *apisecurity.Principals) bool {
	if principals == nil {
		return false
	}
	for _, role := range principals.GetRoles() {
		if _, ok := authtypes.GetSystemRoleDefinition(role.GetId()); ok {
			return true
		}
	}
	return false
}
