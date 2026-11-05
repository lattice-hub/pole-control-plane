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
	"fmt"
	"maps"
	"math"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	"google.golang.org/protobuf/encoding/protojson"

	apimodel "github.com/pole-io/specification/source/go/api/v1/model"
	apisecurity "github.com/pole-io/specification/source/go/api/v1/security"

	cachetypes "github.com/pole-io/pole-server/apis/cache"
	"github.com/pole-io/pole-server/apis/pkg/types"
	authtypes "github.com/pole-io/pole-server/apis/pkg/types/auth"
	storeapi "github.com/pole-io/pole-server/apis/store"
	api "github.com/pole-io/pole-server/pkg/common/api/v1"
	v1 "github.com/pole-io/pole-server/pkg/common/api/v1"
	"github.com/pole-io/pole-server/pkg/common/utils"
	matchs "github.com/pole-io/pole-server/pkg/common/utils/match"
	commontime "github.com/pole-io/pole-server/pkg/common/utils/time"
	"github.com/pole-io/pole-server/pkg/common/utils/valid"
)

type (
	// StrategyDetail2Api strategy detail to *apisecurity.AuthStrategy func
	StrategyDetail2Api func(ctx context.Context, user *authtypes.StrategyDetail) *apisecurity.AuthStrategy
)

// CreatePolicies 创建鉴权策略
func (svr *Server) CreatePolicies(ctx context.Context, reqs []*apisecurity.AuthStrategy) *apimodel.BatchWriteResponse {
	resp := api.NewAuthBatchWriteResponse(apimodel.Code_ExecuteSuccess)

	for index := range reqs {
		ret := svr.CreatePolicy(ctx, reqs[index])
		api.Collect(resp, ret)
	}
	return resp
}

func (svr *Server) CreatePolicy(ctx context.Context, req *apisecurity.AuthStrategy) *apimodel.Response {
	req.Owner = utils.ParseOwnerID(ctx)
	req.Resources = svr.normalizeResource(req.Resources)

	saveData := authtypes.ParsePolicyRule(req)

	tx, err := svr.storage.StartTx()
	if err != nil {
		log.Error("[Auth][Strategy] start tx", utils.RequestID(ctx), zap.Error(err))
		return api.NewAuthResponse(storeapi.StoreCode2APICode(err))
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if err := svr.storage.AddStrategy(tx, saveData); err != nil {
		log.Error("[Auth][Strategy] create strategy into store", utils.RequestID(ctx),
			zap.Error(err))
		return api.NewAuthResponse(storeapi.StoreCode2APICode(err))
	}
	if err := tx.Commit(); err != nil {
		log.Error("[Auth][Strategy] create strategy  commit tx", utils.RequestID(ctx), zap.Error(err))
		return api.NewAuthResponse(storeapi.StoreCode2APICode(err))
	}

	log.Info("[Auth][Strategy] create strategy", utils.RequestID(ctx), zap.String("name", req.Name))
	svr.RecordHistory(authStrategyRecordEntry(ctx, req, saveData, types.OCreate))

	return api.NewAuthStrategyResponse(apimodel.Code_ExecuteSuccess, req)
}

// UpdatePolicies 批量修改鉴权
func (svr *Server) UpdatePolicies(
	ctx context.Context, reqs []*apisecurity.AuthStrategy) *apimodel.BatchWriteResponse {
	resp := api.NewAuthBatchWriteResponse(apimodel.Code_ExecuteSuccess)

	for index := range reqs {
		ret := svr.UpdatePolicy(ctx, reqs[index])
		api.Collect(resp, ret)
	}

	return resp
}

// UpdatePolicy 实现鉴权策略的变更
// Case 1. 修改的是默认鉴权策略的话，只能修改资源，不能添加、删除用户 or 用户组
// Case 2. 鉴权策略只能被自己的 owner 对应的用户修改
// Case 3. 主账户的默认策略不得修改
func (svr *Server) UpdatePolicy(ctx context.Context, req *apisecurity.AuthStrategy) *apimodel.Response {
	saveData, err := svr.storage.GetStrategyDetail(req.GetId())
	if err != nil {
		log.Error("[Auth][Strategy] get strategy from store", utils.RequestID(ctx),
			zap.Error(err))
		return api.NewAuthStrategyResponse(storeapi.StoreCode2APICode(err), req)
	}
	if saveData == nil {
		return api.NewAuthStrategyResponse(apimodel.Code_NotFoundAuthStrategyRule, req)
	}
	if authtypes.IsSystemRolePolicyID(saveData.ID) {
		return api.NewAuthStrategyResponseWithMsg(apimodel.Code_BadRequest,
			"built-in role policy is immutable", req)
	}

	updateData := authtypes.ParsePolicyRule(req)
	data, needUpdate := svr.updateAuthPolicyAttribute(ctx, saveData, updateData)
	if !needUpdate {
		return api.NewAuthStrategyResponse(apimodel.Code_NoNeedUpdate, req)
	}

	if err := svr.storage.UpdateStrategy(data); err != nil {
		log.Error("[Auth][Strategy] update strategy into store",
			utils.RequestID(ctx), zap.Error(err))
		return api.NewAuthResponseWithMsg(storeapi.StoreCode2APICode(err), err.Error())
	}

	log.Info("[Auth][Strategy] update strategy into store", utils.RequestID(ctx),
		zap.String("name", saveData.Name))
	svr.RecordHistory(authModifyStrategyRecordEntry(ctx, req, saveData, types.OUpdate))

	return api.NewAuthStrategyResponse(apimodel.Code_ExecuteSuccess, req)
}

// DeletePolicies 批量删除鉴权策略
func (svr *Server) DeletePolicies(
	ctx context.Context, reqs []*apisecurity.AuthStrategy) *apimodel.BatchWriteResponse {
	resp := api.NewAuthBatchWriteResponse(apimodel.Code_ExecuteSuccess)
	for index := range reqs {
		ret := svr.DeletePolicy(ctx, reqs[index])
		api.Collect(resp, ret)
	}

	return resp
}

// DeleteStrategy 删除鉴权策略
// Case 1. 只有该策略的 owner 账户可以删除策略
// Case 2. 默认策略不能被删除，默认策略只能随着账户的删除而被清理
func (svr *Server) DeletePolicy(ctx context.Context, req *apisecurity.AuthStrategy) *apimodel.Response {
	strategy, err := svr.storage.GetStrategyDetail(req.GetId())
	if err != nil {
		log.Error("[Auth][Strategy] get strategy from store", utils.RequestID(ctx),
			zap.Error(err))
		return api.NewAuthStrategyResponse(storeapi.StoreCode2APICode(err), req)
	}

	if strategy == nil {
		return api.NewAuthStrategyResponse(apimodel.Code_ExecuteSuccess, req)
	}
	if authtypes.IsSystemRolePolicyID(strategy.ID) {
		return api.NewAuthStrategyResponseWithMsg(apimodel.Code_BadRequest,
			"built-in role policy cannot be deleted", req)
	}

	if strategy.Default {
		log.Error("[Auth][Strategy] delete default strategy is denied", utils.RequestID(ctx))
		return api.NewAuthStrategyResponseWithMsg(apimodel.Code_BadRequest, "default strategy can't delete", req)
	}

	if err := svr.storage.DeleteStrategy(req.GetId()); err != nil {
		log.Error("[Auth][Strategy] delete strategy from store",
			utils.RequestID(ctx), zap.Error(err))
		return api.NewAuthResponse(storeapi.StoreCode2APICode(err))
	}

	log.Info("[Auth][Strategy] delete strategy from store", utils.RequestID(ctx),
		zap.String("name", req.Name))
	svr.RecordHistory(authStrategyRecordEntry(ctx, req, strategy, types.ODelete))

	return api.NewAuthStrategyResponse(apimodel.Code_ExecuteSuccess, req)
}

// GetPolicies 批量查询鉴权策略
// Case 1. 如果是以资源视角来查询鉴权策略，那么就会忽略自动根据账户类型进行数据查看的限制
//
//	eg. 比如当前子账户A想要查看资源R的相关的策略，那么不在会自动注入 principal_id 以及 principal_type 的查询条件
//
// Case 2. 如果是以用户视角来查询鉴权策略，如果没有带上 principal_id，那么就会根据账户类型自动注入 principal_id 以
//
//	及 principal_type 的查询条件，从而限制该账户的数据查看
//	eg.
//		a. 如果当前是超级管理账户，则按照传入的 query 进行查询即可
//		b. 如果当前是主账户，则自动注入 owner 字段，即只能查看策略的 owner 是自己的策略
//		c. 如果当前是子账户，则自动注入 principal_id 以及 principal_type 字段，即稚嫩查询与自己有关的策略
func (svr *Server) GetPolicies(ctx context.Context, filters map[string]string) *apimodel.BatchQueryResponse {
	filters = ParseStrategySearchArgs(ctx, filters)
	offset, limit, _ := valid.ParseOffsetAndLimit(filters)

	// 透传兼容模式信息数据
	ctx = context.WithValue(ctx, utils.ContextKeyCompatible{}, svr.options.Compatible)
	total, strategies, err := svr.cacheMgr.AuthStrategy().Query(ctx, cachetypes.PolicySearchArgs{
		Filters: filters,
		Offset:  offset,
		Limit:   limit,
	})
	if err != nil {
		log.Error("[Auth][Strategy] get strategies from store", zap.Any("query", filters),
			utils.RequestID(ctx), zap.Error(err))
		return api.NewAuthBatchQueryResponse(storeapi.StoreCode2APICode(err))
	}

	resp := api.NewAuthBatchQueryResponse(apimodel.Code_ExecuteSuccess)
	resp.Amount = total
	resp.Size = uint32(len(strategies))

	var authStrategies []*apisecurity.AuthStrategy
	if strings.Compare(filters["berif"], "true") == 0 {
		authStrategies = enhancedAuthStrategy2Api(ctx, strategies, svr.authStrategy2Api)
	} else {
		authStrategies = enhancedAuthStrategy2Api(ctx, strategies, svr.authStrategyFull2Api)
	}

	// 将认证策略数据添加到 resp.Data 字段
	for _, strategy := range authStrategies {
		if err := api.AddAnyDataIntoBatchQuery(resp, strategy); err != nil {
			log.Error("[Auth][Strategy] add strategy to response data", utils.RequestID(ctx), zap.Error(err))
			return api.NewAuthBatchQueryResponse(apimodel.Code_ExecuteException)
		}
	}
	return resp
}

var (
	resTypeFilter = map[string]string{
		"namespace":            "0",
		"service":              "1",
		"mcpserverresources":   "30",
		"mcp_server_resources": "30",
		"mcp_servers":          "30",
		"a2aagentresources":    "31",
		"a2a_agent_resources":  "31",
		"a2a_agents":           "31",
	}

	principalTypeFilter = map[string]string{
		"user":   "1",
		"group":  "2",
		"groups": "2",
	}
)

// ParseStrategySearchArgs 处理鉴权策略的搜索参数
func ParseStrategySearchArgs(ctx context.Context, searchFilters map[string]string) map[string]string {
	if val, ok := searchFilters["res_type"]; ok {
		if v, exist := resTypeFilter[val]; exist {
			searchFilters["res_type"] = v
		} else {
			searchFilters["res_type"] = "0"
		}
	}

	if val, ok := searchFilters["principal_type"]; ok {
		if v, exist := principalTypeFilter[val]; exist {
			searchFilters["principal_type"] = v
		} else {
			searchFilters["principal_type"] = "1"
		}
	}
	return searchFilters
}

// GetPolicy 根据策略ID获取详细的鉴权策略
// Case 1 如果当前操作者是该策略 principal 中的一员，则可以查看
// Case 2 如果当前操作者是该策略的 owner，则可以查看
// Case 3 如果当前操作者是admin角色，直接查看
func (svr *Server) GetPolicy(ctx context.Context, req *apisecurity.AuthStrategy) *apimodel.Response {
	userId := utils.ParseUserID(ctx)
	isOwner := utils.ParseIsOwner(ctx)

	if req.GetId() == "" {
		return api.NewAuthResponse(apimodel.Code_EmptyQueryParameter)
	}

	ret, err := svr.storage.GetStrategyDetail(req.GetId())
	if err != nil {
		log.Error("[Auth][Strategy] get strategt from store",
			utils.RequestID(ctx), zap.Error(err))
		return api.NewAuthResponse(storeapi.StoreCode2APICode(err))
	}
	if ret == nil {
		return api.NewAuthStrategyResponse(apimodel.Code_NotFoundAuthStrategyRule, req)
	}

	var canView bool = isOwner
	// 判断是否在该策略所属的成员列表中，如果自己在某个用户组，而该用户组又在这个策略的成员中，则也是可以查看的
	if !canView {
		curUser := &apisecurity.User{
			Id: userId,
		}
		for index := range ret.Principals {
			principal := ret.Principals[index]
			if principal.PrincipalType == authtypes.PrincipalUser && principal.PrincipalID == userId {
				canView = true
				break
			}
			if principal.PrincipalType == authtypes.PrincipalGroup {
				group := &apisecurity.UserGroup{
					Id: principal.PrincipalID,
				}
				if svr.userSvr.GetUserHelper().CheckUserInGroup(ctx, group, curUser) {
					canView = true
					break
				}
			}
		}
	}

	if !canView {
		log.Error("[Auth][Strategy] get strategy detail denied",
			utils.RequestID(ctx), zap.String("user", userId), zap.String("strategy", req.Id),
			zap.Bool("is-owner", isOwner),
		)
		return api.NewAuthStrategyResponse(apimodel.Code_NotAllowedAccess, req)
	}

	return api.NewAuthStrategyResponse(apimodel.Code_ExecuteSuccess, svr.authStrategyFull2Api(ctx, ret))
}

// GetPrincipalResources 获取某个principal可以获取到的所有资源ID数据信息
func (svr *Server) GetPrincipalResources(ctx context.Context, query map[string]string) *apimodel.Response {
	if len(query) == 0 {
		return api.NewAuthResponse(apimodel.Code_EmptyRequest)
	}

	principalId := query["principal_id"]
	if principalId == "" {
		return api.NewAuthResponse(apimodel.Code_EmptyQueryParameter)
	}

	var principalType string
	if v, exist := principalTypeFilter[query["principal_type"]]; exist {
		principalType = v
	} else {
		principalType = "1"
	}

	principalRole, _ := strconv.ParseInt(principalType, 10, 64)
	if err := authtypes.CheckPrincipalType(int(principalRole)); err != nil {
		return api.NewAuthResponse(apimodel.Code_InvalidPrincipalType)
	}

	var (
		resources = make([]authtypes.StrategyResource, 0, 20)
		err       error
	)

	// 找这个用户所关联的用户组
	if authtypes.PrincipalType(principalRole) == authtypes.PrincipalUser {
		groups := svr.userSvr.GetUserHelper().GetUserOwnGroup(ctx, &apisecurity.User{
			Id: principalId,
		})
		for i := range groups {
			item := groups[i]
			res, err := svr.storage.GetStrategyResources(item.GetId(), authtypes.PrincipalGroup)
			if err != nil {
				log.Error("[Auth][Strategy] get principal link resource", utils.RequestID(ctx),
					zap.String("principal-id", principalId), zap.Any("principal-role", principalRole), zap.Error(err))
				return api.NewAuthResponse(storeapi.StoreCode2APICode(err))
			}
			resources = append(resources, res...)
		}
	}

	pResources, err := svr.storage.GetStrategyResources(principalId, authtypes.PrincipalType(principalRole))
	if err != nil {
		log.Error("[Auth][Strategy] get principal link resource", utils.RequestID(ctx),
			zap.String("principal-id", principalId), zap.Any("principal-role", principalRole), zap.Error(err))
		return api.NewAuthResponse(storeapi.StoreCode2APICode(err))
	}

	resources = append(resources, pResources...)
	tmp := &apisecurity.AuthStrategy{
		Resources: &apisecurity.StrategyResources{},
	}

	svr.enrichResourceInfo(ctx, tmp, &authtypes.StrategyDetail{
		Resources: resourceDeduplication(resources),
	})

	return api.NewStrategyResourcesResponse(apimodel.Code_ExecuteSuccess, tmp.Resources)
}

// GetResourcePrincipals 获取资源的所有关联成员
func (svr *Server) GetResourcePrincipals(ctx context.Context, query map[string]string) *apimodel.Response {
	resId := query["res_id"]
	resType := query["res_type"]
	action := query["action"]

	total, ret, err := svr.cacheMgr.AuthStrategy().Query(ctx, cachetypes.PolicySearchArgs{
		Filters: map[string]string{
			"res_id":   resId,
			"res_type": resType,
		},
		Offset: 0,
		Limit:  math.MaxUint32,
	})
	if err != nil {
		log.Error("[Auth][Strategy] get resource principals from store", utils.RequestID(ctx),
			zap.Error(err))
		return api.NewResponse(storeapi.StoreCode2APICode(err))
	}
	if total == 0 {
		return api.NewResponse(apimodel.Code_ExecuteSuccess)
	}

	principals := &apisecurity.Principals{}
	for i := range ret {
		for j := range ret[i].Principals {
			// 禁止策略不算在内
			if action != "" && ret[i].Action != action {
				continue
			}
			item := ret[i].Principals[j]
			switch item.PrincipalType {
			case authtypes.PrincipalUser:
				principals.Users = append(principals.Users, &apisecurity.Principal{
					Id:   item.PrincipalID,
					Name: item.Name,
				})
			case authtypes.PrincipalGroup:
				principals.Groups = append(principals.Groups, &apisecurity.Principal{
					Id:   item.PrincipalID,
					Name: item.Name,
				})
			case authtypes.PrincipalRole:
				principals.Roles = append(principals.Roles, &apisecurity.Principal{
					Id:   item.PrincipalID,
					Name: item.Name,
				})
			}
		}
	}
	return api.NewAnyDataResponse(apimodel.Code_ExecuteSuccess, principals)
}

// AuthorizeResources 授权资源：将指定资源授权给若干 principal，即给这些 principal 的默认策略增加对应资源关联。
func (svr *Server) AuthorizeResources(ctx context.Context, reqs []*v1.AuthorizeResources) *apimodel.Response {
	if len(reqs) == 0 {
		return api.NewAuthResponse(apimodel.Code_EmptyRequest)
	}

	for _, req := range reqs {
		// 1.1 请求校验：ResourceType、ResourceID、Principals 必填
		if strings.TrimSpace(req.ResourceType) == "" {
			return api.NewAuthResponseWithMsg(apimodel.Code_InvalidParameter, "resource_type is required")
		}
		if strings.TrimSpace(req.ResourceID) == "" {
			return api.NewAuthResponseWithMsg(apimodel.Code_InvalidParameter, "resource_id is required")
		}
		if len(req.Principals.Users) == 0 && len(req.Principals.Groups) == 0 && len(req.Principals.Roles) == 0 {
			return api.NewAuthResponseWithMsg(apimodel.Code_InvalidParameter, "principals is required and cannot be empty")
		}

		// 1.2 ResourceType 映射为 apisecurity.ResourceType
		key := strings.ToLower(strings.TrimSpace(req.ResourceType))
		num, ok := resTypeFilter[key]
		if !ok {
			return api.NewAuthResponseWithMsg(apimodel.Code_InvalidParameter, "invalid resource_type: "+req.ResourceType)
		}
		resType, ok := authtypes.SearchTypeMapping[num]
		if !ok {
			return api.NewAuthResponseWithMsg(apimodel.Code_InvalidParameter, "unsupported resource_type: "+req.ResourceType)
		}

		// 1.3 资源存在性校验（* 表示不校验）
		if req.ResourceID != "*" {
			if errResp := svr.authorizeCheckResourceExist(ctx, resType, req.ResourceID); errResp != nil {
				return errResp
			}
		}

		// 1.4 Principal 存在性校验
		if errResp := svr.authorizeCheckPrincipalsExist(ctx, &req.Principals); errResp != nil {
			return errResp
		}

		// 1.5 按 principal 写默认策略资源
		strategyResources := make([]authtypes.StrategyResource, 0)
		for _, p := range req.Principals.Users {
			if p.Id == "" {
				continue
			}
			strategy, err := svr.storage.GetDefaultStrategyDetailByPrincipal(p.Id, authtypes.PrincipalUser)
			if err != nil {
				log.Error("[Auth][Strategy] get default strategy by user", utils.RequestID(ctx), zap.String("user", p.Id), zap.Error(err))
				return api.NewAuthResponse(storeapi.StoreCode2APICode(err))
			}
			if strategy == nil {
				return api.NewAuthResponseWithMsg(apimodel.Code_NotFoundAuthStrategyRule, "default strategy not found for user: "+p.Id)
			}
			strategyResources = append(strategyResources, authtypes.StrategyResource{
				StrategyID: strategy.ID,
				ResType:    resType,
				ResID:      req.ResourceID,
			})
		}
		for _, p := range req.Principals.Groups {
			if p.Id == "" {
				continue
			}
			strategy, err := svr.storage.GetDefaultStrategyDetailByPrincipal(p.Id, authtypes.PrincipalGroup)
			if err != nil {
				log.Error("[Auth][Strategy] get default strategy by group", utils.RequestID(ctx), zap.String("group", p.Id), zap.Error(err))
				return api.NewAuthResponse(storeapi.StoreCode2APICode(err))
			}
			if strategy == nil {
				return api.NewAuthResponseWithMsg(apimodel.Code_NotFoundAuthStrategyRule, "default strategy not found for group: "+p.Id)
			}
			strategyResources = append(strategyResources, authtypes.StrategyResource{
				StrategyID: strategy.ID,
				ResType:    resType,
				ResID:      req.ResourceID,
			})
		}
		for _, p := range req.Principals.Roles {
			if p.Id == "" {
				continue
			}
			strategy, err := svr.storage.GetDefaultStrategyDetailByPrincipal(p.Id, authtypes.PrincipalRole)
			if err != nil {
				log.Error("[Auth][Strategy] get default strategy by role", utils.RequestID(ctx), zap.String("role", p.Id), zap.Error(err))
				return api.NewAuthResponse(storeapi.StoreCode2APICode(err))
			}
			if strategy == nil {
				return api.NewAuthResponseWithMsg(apimodel.Code_NotFoundAuthStrategyRule, "default strategy not found for role: "+p.Id)
			}
			strategyResources = append(strategyResources, authtypes.StrategyResource{
				StrategyID: strategy.ID,
				ResType:    resType,
				ResID:      req.ResourceID,
			})
		}
		if len(strategyResources) == 0 {
			continue
		}
		if err := svr.storage.LooseAddStrategyResources(strategyResources); err != nil {
			log.Error("[Auth][Strategy] loose add strategy resources", utils.RequestID(ctx), zap.Error(err))
			return api.NewAuthResponse(storeapi.StoreCode2APICode(err))
		}
	}

	return api.NewAuthResponse(apimodel.Code_ExecuteSuccess)
}

// authorizeCheckResourceExist 校验授权请求中的资源是否存在（* 由调用方跳过）
func (svr *Server) authorizeCheckResourceExist(ctx context.Context, resType apisecurity.ResourceType, resourceID string) *apimodel.Response {
	switch resType {
	case apisecurity.ResourceType_Namespaces:
		if svr.cacheMgr.Namespace().GetNamespace(resourceID) == nil {
			return api.NewAuthResponse(apimodel.Code_NotFoundResource)
		}
	case apisecurity.ResourceType_Services:
		if svr.cacheMgr.Service().GetServiceByID(resourceID) == nil {
			return api.NewAuthResponse(apimodel.Code_NotFoundResource)
		}
	case apisecurity.ResourceType_MCPServerResources:
		if svr.cacheMgr.MCPServer().GetMCPServerByID(resourceID) == nil {
			return api.NewAuthResponse(apimodel.Code_NotFoundResource)
		}
	case apisecurity.ResourceType_A2AAgentResources:
		if svr.cacheMgr.A2AAgent().GetA2AAgentByID(resourceID) == nil {
			return api.NewAuthResponse(apimodel.Code_NotFoundResource)
		}
	default:
		return api.NewAuthResponseWithMsg(apimodel.Code_InvalidParameter, "resource_type not supported for authorize")
	}
	return nil
}

// authorizeCheckPrincipalsExist 校验授权请求中的 users/groups/roles 是否存在
func (svr *Server) authorizeCheckPrincipalsExist(ctx context.Context, principals *v1.Principals) *apimodel.Response {
	if principals == nil {
		return nil
	}
	users := make([]*apisecurity.User, 0, len(principals.Users))
	for _, p := range principals.Users {
		if p.Id != "" {
			users = append(users, &apisecurity.User{Id: p.Id})
		}
	}
	if len(users) > 0 {
		if err := svr.userSvr.GetUserHelper().CheckUsersExist(ctx, users); err != nil {
			return api.NewAuthResponse(apimodel.Code_NotFoundUser)
		}
	}
	groups := make([]*apisecurity.UserGroup, 0, len(principals.Groups))
	for _, p := range principals.Groups {
		if p.Id != "" {
			groups = append(groups, &apisecurity.UserGroup{Id: p.Id})
		}
	}
	if len(groups) > 0 {
		if err := svr.userSvr.GetUserHelper().CheckGroupsExist(ctx, groups); err != nil {
			return api.NewAuthResponse(apimodel.Code_NotFoundUserGroup)
		}
	}
	for _, p := range principals.Roles {
		if p.Id == "" {
			continue
		}
		if _, ok := authtypes.GetSystemRoleDefinition(p.Id); ok {
			return api.NewAuthResponseWithMsg(apimodel.Code_BadRequest, "built-in role permissions are immutable")
		}
		if svr.PolicyHelper().GetRole(p.Id) == nil {
			return api.NewAuthResponseWithMsg(apimodel.Code_NotFoundResource, "role not found: "+p.Id)
		}
	}
	return nil
}

// enhancedAuthStrategy2Api
func enhancedAuthStrategy2Api(ctx context.Context, s []*authtypes.StrategyDetail,
	fn StrategyDetail2Api) []*apisecurity.AuthStrategy {
	out := make([]*apisecurity.AuthStrategy, 0, len(s))
	for k := range s {
		out = append(out, fn(ctx, s[k]))
	}
	return out
}

// authStrategy2Api
func (svr *Server) authStrategy2Api(ctx context.Context, s *authtypes.StrategyDetail) *apisecurity.AuthStrategy {
	if s == nil {
		return nil
	}

	// note: 不包括token，token比较特殊
	out := &apisecurity.AuthStrategy{
		Id:              s.ID,
		Name:            s.Name,
		Comment:         s.Comment,
		Ctime:           commontime.Time2String(s.CreateTime),
		Mtime:           commontime.Time2String(s.ModifyTime),
		Action:          apisecurity.AuthAction(apisecurity.AuthAction_value[s.Action]),
		DefaultStrategy: s.Default,
	}

	return out
}

// authStrategyFull2Api
func (svr *Server) authStrategyFull2Api(ctx context.Context, data *authtypes.StrategyDetail) *apisecurity.AuthStrategy {
	if data == nil {
		return nil
	}

	// note: 不包括token，token比较特殊
	out := &apisecurity.AuthStrategy{
		Id:              data.ID,
		Name:            data.Name,
		Comment:         data.Comment,
		Action:          apisecurity.AuthAction(apisecurity.AuthAction_value[data.Action]),
		DefaultStrategy: data.Default,
		Functions:       data.CalleeMethods,
		Metadata:        data.Metadata,
		Ctime:           commontime.Time2String(data.CreateTime),
		Mtime:           commontime.Time2String(data.ModifyTime),
	}

	svr.enrichPrincipalInfo(out, data)
	svr.enrichResourceInfo(ctx, out, data)
	return out
}

// updateAuthPolicyAttribute 更新计算鉴权策略的属性
func (svr *Server) updateAuthPolicyAttribute(ctx context.Context, saved, update *authtypes.StrategyDetail) (*authtypes.StrategyDetail, bool) {
	var needUpdate bool

	if saved.Comment != update.Comment {
		saved.Comment = update.Comment
		needUpdate = true
	}

	if saved.Action != update.Action {
		saved.Action = update.Action
		needUpdate = true
	}

	if !maps.Equal(saved.Metadata, update.Metadata) {
		saved.Metadata = update.Metadata
		needUpdate = true
	}

	if !slices.Equal(saved.CalleeMethods, update.CalleeMethods) {
		saved.CalleeMethods = update.CalleeMethods
		needUpdate = true
	}

	if !slices.EqualFunc(saved.Principals, update.Principals, func(e1, e2 authtypes.Principal) bool {
		return e1.PrincipalID == e2.PrincipalID && e1.PrincipalType == e2.PrincipalType
	}) {
		saved.Principals = update.Principals
		needUpdate = true
	}

	if !slices.EqualFunc(saved.Resources, update.Resources, func(e1, e2 authtypes.StrategyResource) bool {
		return e1.StrategyID == e2.StrategyID && e1.ResType == e2.ResType && e1.ResID == e2.ResID
	}) {
		saved.Resources = update.Resources
		needUpdate = true
	}

	if !slices.EqualFunc(saved.Conditions, update.Conditions, func(e1, e2 authtypes.Condition) bool {
		return e1.Key == e2.Key && e1.Value == e2.Value && e1.CompareFunc == e2.CompareFunc
	}) {
		saved.Conditions = update.Conditions
		needUpdate = true
	}

	return saved, needUpdate
}

// normalizeResource 对于资源进行归一化处理, 如果出现 * 的话，则该资源访问策略就是 *
func (svr *Server) normalizeResource(resources *apisecurity.StrategyResources) *apisecurity.StrategyResources {
	if resources == nil {
		return &apisecurity.StrategyResources{}
	}
	for _, ptrGetter := range resourceFieldPointerGetters {
		slicePtr := ptrGetter(resources)
		if slicePtr.Elem().IsNil() {
			continue
		}
		sliceVal := slicePtr.Elem()
		for i := 0; i < sliceVal.Len(); i++ {
			entry := sliceVal.Index(i)
			if entry.IsNil() {
				continue
			}
			item := entry.Elem()
			resID := item.FieldByName("Id")
			// specification 的资源 ID 已从 wrapperspb.StringValue 迁移为 string。
			// 通过 reflect.String 读取可避免旧类型断言在策略创建时 panic。
			if resID.IsValid() && resID.Kind() == reflect.String && resID.String() == matchs.MatchAll {
				sliceVal.Set(reflect.ValueOf([]*apisecurity.StrategyResourceEntry{{
					Id: "*",
				}}))
				break
			}
		}
	}
	return resources
}

// enrichPrincipalInfo 填充 principal 摘要信息
func (svr *Server) enrichPrincipalInfo(resp *apisecurity.AuthStrategy, data *authtypes.StrategyDetail) {
	users := make([]*apisecurity.Principal, 0, len(data.Principals))
	groups := make([]*apisecurity.Principal, 0, len(data.Principals))
	roles := make([]*apisecurity.Principal, 0, len(data.Principals))
	for index := range data.Principals {
		principal := data.Principals[index]
		switch principal.PrincipalType {
		case authtypes.PrincipalUser:
			if user := svr.userSvr.GetUserHelper().GetUser(context.TODO(), &apisecurity.User{
				Id: principal.PrincipalID,
			}); user != nil {
				users = append(users, &apisecurity.Principal{
					Id:   user.GetId(),
					Name: user.GetName(),
				})
			}
		case authtypes.PrincipalGroup:
			if group := svr.userSvr.GetUserHelper().GetGroup(context.TODO(), &apisecurity.UserGroup{
				Id: principal.PrincipalID,
			}); group != nil {
				groups = append(groups, &apisecurity.Principal{
					Id:   group.GetId(),
					Name: group.GetName(),
				})
			}
		case authtypes.PrincipalRole:
			if role := svr.PolicyHelper().GetRole(principal.PrincipalID); role != nil {
				roles = append(roles, &apisecurity.Principal{
					Id:   role.ID,
					Name: role.Name,
				})
			}
		}
	}

	resp.Principals = &apisecurity.Principals{
		Users:  users,
		Groups: groups,
		Roles:  roles,
	}
}

// enrichResourceInfo 填充资源摘要信息
func (svr *Server) enrichResourceInfo(ctx context.Context, resp *apisecurity.AuthStrategy, data *authtypes.StrategyDetail) {
	allMatch := map[apisecurity.ResourceType]struct{}{}
	resp.Resources = &apisecurity.StrategyResources{
		Namespaces:          make([]*apisecurity.StrategyResourceEntry, 0, 4),
		Services:            make([]*apisecurity.StrategyResourceEntry, 0, 4),
		RouteRules:          make([]*apisecurity.StrategyResourceEntry, 0, 4),
		RatelimitRules:      make([]*apisecurity.StrategyResourceEntry, 0, 4),
		CircuitbreakerRules: make([]*apisecurity.StrategyResourceEntry, 0, 4),
		FaultdetectRules:    make([]*apisecurity.StrategyResourceEntry, 0, 4),
		LaneRules:           make([]*apisecurity.StrategyResourceEntry, 0, 4),
		LosslessRules:       make([]*apisecurity.StrategyResourceEntry, 0, 4),
		MirrorRules:         make([]*apisecurity.StrategyResourceEntry, 0, 4),
		SecurityRules:       make([]*apisecurity.StrategyResourceEntry, 0, 4),
		MockRules:           make([]*apisecurity.StrategyResourceEntry, 0, 4),
		Users:               make([]*apisecurity.StrategyResourceEntry, 0, 4),
		UserGroups:          make([]*apisecurity.StrategyResourceEntry, 0, 4),
		Roles:               make([]*apisecurity.StrategyResourceEntry, 0, 4),
		AuthPolicies:        make([]*apisecurity.StrategyResourceEntry, 0, 4),
		McpServers:          make([]*apisecurity.StrategyResourceEntry, 0, 4),
		A2AAgents:           make([]*apisecurity.StrategyResourceEntry, 0, 4),
	}

	for index := range data.Resources {
		res := data.Resources[index]
		svr.enrichResourceDetial(ctx, res, allMatch, resp)
	}
}

func (svr *Server) enrichResourceDetial(ctx context.Context, item authtypes.StrategyResource,
	allMatch map[apisecurity.ResourceType]struct{}, resp *apisecurity.AuthStrategy) {

	resType := apisecurity.ResourceType(item.ResType)
	ptrGetter, ok := resourceFieldPointerGetters[resType]
	if !ok {
		log.Warn("[Auth][Strategy] unsupported resource type in fill-info",
			zap.String("id", item.StrategyID), zap.String("res-id", item.ResID),
			zap.String("res-type", resType.String()), utils.RequestID(ctx))
		return
	}

	slicePtr := ptrGetter(resp.Resources)
	if slicePtr.Elem().IsNil() {
		return
	}
	sliceVal := slicePtr.Elem()

	if item.ResID == "*" {
		allMatch[resType] = struct{}{}
		sliceVal.Set(reflect.ValueOf([]*apisecurity.StrategyResourceEntry{
			{
				Id:        "*",
				Namespace: "*",
				Name:      "*",
			},
		}))
		return
	}
	if _, ok := allMatch[resType]; !ok {
		convert, ok := resourceConvert[resType]
		if !ok {
			log.Warn("[Auth][Strategy] unsupported resource converter in fill-info",
				zap.String("id", item.StrategyID), zap.String("res-id", item.ResID),
				zap.String("res-type", resType.String()), utils.RequestID(ctx))
			return
		}
		if data := convert(ctx, svr, item); data != nil {
			// 创建一个新数组并把元素的值追加进去
			resArr := reflect.Append(sliceVal, reflect.ValueOf(data))
			sliceVal.Set(resArr)
		}
	}
}

// filter different types of Strategy resources
func resourceDeduplication(resources []authtypes.StrategyResource) []authtypes.StrategyResource {
	rLen := len(resources)
	ret := make([]authtypes.StrategyResource, 0, rLen)
	filters := map[apisecurity.ResourceType]map[string]struct{}{}

	est := struct{}{}
	for i := range resources {
		res := resources[i]
		filter, ok := filters[apisecurity.ResourceType(res.ResType)]
		if !ok {
			filters[apisecurity.ResourceType(res.ResType)] = map[string]struct{}{}
			filter = filters[apisecurity.ResourceType(res.ResType)]
		}
		if _, exist := filter[res.ResID]; !exist {
			filter[res.ResID] = est
			ret = append(ret, res)
		}
	}
	return ret
}

// authStrategyRecordEntry 转换为鉴权策略的记录结构体
func authStrategyRecordEntry(ctx context.Context, req *apisecurity.AuthStrategy, md *authtypes.StrategyDetail,
	operationType types.OperationType) *types.RecordEntry {

	detailBytes, _ := protojson.Marshal(req)
	detail := string(detailBytes)

	entry := &types.RecordEntry{
		ResourceType:  types.RAuthStrategy,
		ResourceName:  fmt.Sprintf("%s(%s)", md.Name, md.ID),
		OperationType: operationType,
		Operator:      utils.ParseOperator(ctx),
		Detail:        detail,
		HappenTime:    time.Now(),
	}

	return entry
}

// authModifyStrategyRecordEntry
func authModifyStrategyRecordEntry(
	ctx context.Context, req *apisecurity.AuthStrategy, md *authtypes.StrategyDetail,
	operationType types.OperationType) *types.RecordEntry {

	detailBytes2, _ := protojson.Marshal(req)
	detail := string(detailBytes2)

	entry := &types.RecordEntry{
		ResourceType:  types.RAuthStrategy,
		ResourceName:  fmt.Sprintf("%s(%s)", md.Name, md.ID),
		OperationType: operationType,
		Operator:      utils.ParseOperator(ctx),
		Detail:        detail,
		HappenTime:    time.Now(),
	}

	return entry
}

var (
	resourceFieldPointerGetters = authtypes.ResourceFieldPointerGetters

	resourceConvert = map[apisecurity.ResourceType]func(context.Context,
		*Server, authtypes.StrategyResource) *apisecurity.StrategyResourceEntry{

		// 注册、配置、治理
		apisecurity.ResourceType_Namespaces: func(ctx context.Context, svr *Server,
			item authtypes.StrategyResource) *apisecurity.StrategyResourceEntry {
			user := svr.cacheMgr.Namespace().GetNamespace(item.ResID)
			if user == nil {
				log.Warn("[Auth][Strategy] not found namespace in fill-info",
					zap.String("id", item.StrategyID), zap.String("res-id", item.ResID), utils.RequestID(ctx))
				return nil
			}
			return &apisecurity.StrategyResourceEntry{
				Id:        item.ResID,
				Namespace: user.Name,
				Name:      user.Name,
			}
		},
		apisecurity.ResourceType_Services: func(ctx context.Context, svr *Server,
			item authtypes.StrategyResource) *apisecurity.StrategyResourceEntry {
			user := svr.cacheMgr.Namespace().GetNamespace(item.ResID)
			if user == nil {
				log.Warn("[Auth][Strategy] not found namespace in fill-info",
					zap.String("id", item.StrategyID), zap.String("res-id", item.ResID), utils.RequestID(ctx))
				return nil
			}
			return &apisecurity.StrategyResourceEntry{
				Id:        item.ResID,
				Namespace: user.Name,
				Name:      user.Name,
			}
		},
		apisecurity.ResourceType_RouteRules: func(ctx context.Context, svr *Server,
			item authtypes.StrategyResource) *apisecurity.StrategyResourceEntry {
			user := svr.cacheMgr.RoutingConfig().GetRule(item.ResID)
			if user == nil {
				log.Warn("[Auth][Strategy] not found route_rule in fill-info",
					zap.String("id", item.StrategyID), zap.String("res-id", item.ResID), utils.RequestID(ctx))
				return nil
			}
			return &apisecurity.StrategyResourceEntry{
				Id:        item.ResID,
				Namespace: user.Name,
				Name:      user.Name,
			}
		},
		apisecurity.ResourceType_LaneRules: func(ctx context.Context, svr *Server,
			item authtypes.StrategyResource) *apisecurity.StrategyResourceEntry {
			user := svr.cacheMgr.LaneRule().GetRule(item.ResID)
			if user == nil {
				log.Warn("[Auth][Strategy] not found lane_rule in fill-info",
					zap.String("id", item.StrategyID), zap.String("res-id", item.ResID), utils.RequestID(ctx))
				return nil
			}
			return &apisecurity.StrategyResourceEntry{
				Id:        item.ResID,
				Namespace: user.Name,
				Name:      user.Name,
			}
		},
		apisecurity.ResourceType_LosslessRules: func(ctx context.Context, svr *Server,
			item authtypes.StrategyResource) *apisecurity.StrategyResourceEntry {
			user := svr.cacheMgr.Lossless().GetRule(item.ResID)
			if user == nil {
				log.Warn("[Auth][Strategy] not found lossless_rule in fill-info",
					zap.String("id", item.StrategyID), zap.String("res-id", item.ResID), utils.RequestID(ctx))
				return nil
			}
			return &apisecurity.StrategyResourceEntry{
				Id:        item.ResID,
				Namespace: user.Namespace,
				Name:      user.Service,
			}
		},
		// 流量控制类资源
		apisecurity.ResourceType_RateLimitRules: func(ctx context.Context, svr *Server,
			item authtypes.StrategyResource) *apisecurity.StrategyResourceEntry {
			user := svr.cacheMgr.RateLimit().GetRule(item.ResID)
			if user == nil {
				log.Warn("[Auth][Strategy] not found ratelimit_rule in fill-info",
					zap.String("id", item.StrategyID), zap.String("res-id", item.ResID), utils.RequestID(ctx))
				return nil
			}
			return &apisecurity.StrategyResourceEntry{
				Id:        item.ResID,
				Namespace: user.Name,
				Name:      user.Name,
			}
		},
		apisecurity.ResourceType_CircuitBreakerRules: func(ctx context.Context, svr *Server,
			item authtypes.StrategyResource) *apisecurity.StrategyResourceEntry {
			user := svr.cacheMgr.CircuitBreaker().GetRule(item.ResID)
			if user == nil {
				log.Warn("[Auth][Strategy] not found circuitbreaker_rule in fill-info",
					zap.String("id", item.StrategyID), zap.String("res-id", item.ResID), utils.RequestID(ctx))
				return nil
			}
			return &apisecurity.StrategyResourceEntry{
				Id:        item.ResID,
				Namespace: user.Name,
				Name:      user.Name,
			}
		},
		apisecurity.ResourceType_FaultDetectRules: func(ctx context.Context, svr *Server,
			item authtypes.StrategyResource) *apisecurity.StrategyResourceEntry {
			user := svr.cacheMgr.FaultDetector().GetRule(item.ResID)
			if user == nil {
				log.Warn("[Auth][Strategy] not found faultdetect_rule in fill-info",
					zap.String("id", item.StrategyID), zap.String("res-id", item.ResID), utils.RequestID(ctx))
				return nil
			}
			return &apisecurity.StrategyResourceEntry{
				Id:        item.ResID,
				Namespace: user.Name,
				Name:      user.Name,
			}
		},
		apisecurity.ResourceType_SecurityRules: func(ctx context.Context, svr *Server,
			item authtypes.StrategyResource) *apisecurity.StrategyResourceEntry {
			rule := svr.cacheMgr.TrafficSecurity().GetRule(item.ResID)
			if rule == nil {
				log.Warn("[Auth][Strategy] not found traffic_security_rule in fill-info",
					zap.String("id", item.StrategyID), zap.String("res-id", item.ResID), utils.RequestID(ctx))
				return nil
			}
			return &apisecurity.StrategyResourceEntry{
				Id:        item.ResID,
				Namespace: rule.Namespace,
				Name:      rule.Name,
			}
		},
		apisecurity.ResourceType_MirrorRules: func(ctx context.Context, svr *Server,
			item authtypes.StrategyResource) *apisecurity.StrategyResourceEntry {
			rule := svr.cacheMgr.TrafficMirror().GetRule(item.ResID)
			if rule == nil {
				log.Warn("[Auth][Strategy] not found traffic_mirror_rule in fill-info",
					zap.String("id", item.StrategyID), zap.String("res-id", item.ResID), utils.RequestID(ctx))
				return nil
			}
			return &apisecurity.StrategyResourceEntry{
				Id:        item.ResID,
				Namespace: rule.Namespace,
				Name:      rule.Name,
			}
		},
		apisecurity.ResourceType_MockRules: func(ctx context.Context, svr *Server,
			item authtypes.StrategyResource) *apisecurity.StrategyResourceEntry {
			rule := svr.cacheMgr.TrafficMock().GetRule(item.ResID)
			if rule == nil {
				log.Warn("[Auth][Strategy] not found traffic_mock_rule in fill-info",
					zap.String("id", item.StrategyID), zap.String("res-id", item.ResID), utils.RequestID(ctx))
				return nil
			}
			return &apisecurity.StrategyResourceEntry{
				Id:        item.ResID,
				Namespace: rule.Namespace,
				Name:      rule.Name,
			}
		},
		apisecurity.ResourceType_MCPServerResources: func(ctx context.Context, svr *Server,
			item authtypes.StrategyResource) *apisecurity.StrategyResourceEntry {
			server := svr.cacheMgr.MCPServer().GetMCPServerByID(item.ResID)
			if server == nil {
				log.Warn("[Auth][Strategy] not found mcp_server in fill-info",
					zap.String("id", item.StrategyID), zap.String("res-id", item.ResID), utils.RequestID(ctx))
				return nil
			}
			return &apisecurity.StrategyResourceEntry{
				Id:        item.ResID,
				Namespace: server.GetNamespace(),
				Name:      server.GetName(),
			}
		},
		apisecurity.ResourceType_A2AAgentResources: func(ctx context.Context, svr *Server,
			item authtypes.StrategyResource) *apisecurity.StrategyResourceEntry {
			agent := svr.cacheMgr.A2AAgent().GetA2AAgentByID(item.ResID)
			if agent == nil {
				log.Warn("[Auth][Strategy] not found a2a_agent in fill-info",
					zap.String("id", item.StrategyID), zap.String("res-id", item.ResID), utils.RequestID(ctx))
				return nil
			}
			return &apisecurity.StrategyResourceEntry{
				Id:        item.ResID,
				Namespace: agent.Namespace,
				Name:      agent.Name,
			}
		},
		// 鉴权资源
		apisecurity.ResourceType_Users: func(ctx context.Context, svr *Server,
			item authtypes.StrategyResource) *apisecurity.StrategyResourceEntry {
			user := svr.cacheMgr.User().GetUserByID(item.ResID)
			if user == nil {
				log.Warn("[Auth][Strategy] not found user in fill-info",
					zap.String("id", item.StrategyID), zap.String("res-id", item.ResID), utils.RequestID(ctx))
				return nil
			}
			return &apisecurity.StrategyResourceEntry{
				Id:   item.ResID,
				Name: user.Name,
			}
		},
		apisecurity.ResourceType_UserGroups: func(ctx context.Context, svr *Server,
			item authtypes.StrategyResource) *apisecurity.StrategyResourceEntry {
			user := svr.cacheMgr.User().GetGroup(item.ResID)
			if user == nil {
				log.Warn("[Auth][Strategy] not found user_group in fill-info",
					zap.String("id", item.StrategyID), zap.String("res-id", item.ResID), utils.RequestID(ctx))
				return nil
			}
			return &apisecurity.StrategyResourceEntry{
				Id:   item.ResID,
				Name: user.Name,
			}
		},
		apisecurity.ResourceType_Roles: func(ctx context.Context, svr *Server,
			item authtypes.StrategyResource) *apisecurity.StrategyResourceEntry {
			user := svr.cacheMgr.Role().GetRole(item.ResID)
			if user == nil {
				log.Warn("[Auth][Strategy] not found role in fill-info",
					zap.String("id", item.StrategyID), zap.String("res-id", item.ResID), utils.RequestID(ctx))
				return nil
			}
			return &apisecurity.StrategyResourceEntry{
				Id:   item.ResID,
				Name: user.Name,
			}
		},
		apisecurity.ResourceType_PolicyRules: func(ctx context.Context, svr *Server,
			item authtypes.StrategyResource) *apisecurity.StrategyResourceEntry {
			user := svr.cacheMgr.AuthStrategy().GetPolicyRule(item.ResID)
			if user == nil {
				log.Warn("[Auth][Strategy] not found auth_policy in fill-info",
					zap.String("id", item.StrategyID), zap.String("res-id", item.ResID), utils.RequestID(ctx))
				return nil
			}
			return &apisecurity.StrategyResourceEntry{
				Id:   item.ResID,
				Name: user.Name,
			}
		},
	}
)
