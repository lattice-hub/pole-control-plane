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
	"context"

	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/anypb"

	apimodel "github.com/pole-io/specification/source/go/api/v1/model"
	apisecurity "github.com/pole-io/specification/source/go/api/v1/security"

	"github.com/pole-io/pole-server/apis/access_control/auth"
	cacheapi "github.com/pole-io/pole-server/apis/cache"
	"github.com/pole-io/pole-server/apis/pkg/types"
	authtypes "github.com/pole-io/pole-server/apis/pkg/types/auth"
	api "github.com/pole-io/pole-server/pkg/common/api/v1"
	"github.com/pole-io/pole-server/pkg/common/syncs/container"
	"github.com/pole-io/pole-server/pkg/common/utils"
	"github.com/pole-io/pole-server/pkg/namespace"
)

var _ namespace.NamespaceOperateServer = (*Server)(nil)

// Server 带有鉴权能力的 NamespaceOperateServer
// 该层会对请求参数做一些调整，根据具体的请求发起人，设置为数据对应的 owner，不可为为别人进行创建资源
type Server struct {
	nextSvr   namespace.NamespaceOperateServer
	userSvr   auth.UserServer
	policySvr auth.StrategyServer
	cacheSvr  cacheapi.CacheManager
}

func NewServer(nextSvr namespace.NamespaceOperateServer, userSvr auth.UserServer,
	policySvr auth.StrategyServer, cacheSvr cacheapi.CacheManager) namespace.NamespaceOperateServer {
	proxy := &Server{
		nextSvr:   nextSvr,
		userSvr:   userSvr,
		policySvr: policySvr,
		cacheSvr:  cacheSvr,
	}
	return proxy
}

// CreateNamespaceIfAbsent Create a single name space，基于新API规范重新设计
func (svr *Server) CreateNamespaceIfAbsent(ctx context.Context,
	req *apimodel.Namespace) (string, *apimodel.Response) {
	n, rsp := svr.nextSvr.CreateNamespaceIfAbsent(ctx, req)
	if api.IsSuccess(rsp) {
		// 注释：资源回调改动 - 根据pole-io/specification，Response不再有Namespace字段，直接使用请求参数
		_ = svr.afterNamespaceResource(ctx, req, false)

		// 注释：响应数据改动 - 根据新的 pole-io/specification，在Response的data字段中包含namespace信息
		if rsp.Data == nil {
			if anyData, err := anypb.New(req); err == nil {
				rsp.Data = anyData
			}
		}
	}
	return n, rsp
}

// CreateNamespace 创建命名空间，基于新API规范重新设计
func (svr *Server) CreateNamespace(ctx context.Context, req *apimodel.Namespace) *apimodel.Response {
	authCtx := svr.collectNamespaceAuthContext(
		ctx, []*apimodel.Namespace{req}, authtypes.Create, authtypes.CreateNamespace)
	// 验证 token 信息
	if _, err := svr.policySvr.GetAuthChecker().CheckConsolePermission(authCtx); err != nil {
		return api.NewResponse(authtypes.ConvertToErrCode(err))
	}

	ctx = authCtx.GetRequestContext()
	ctx = context.WithValue(ctx, types.ContextAuthContextKey, authCtx)

	// 填充 ownerId 信息数据
	if ownerId := utils.ParseOwnerID(ctx); len(ownerId) > 0 {
		// 注释：所有者字段改动 - Owners字段从*wrapperspb.StringValue改为string，直接赋值
		req.Owners = string(ownerId)
	}

	resp := svr.nextSvr.CreateNamespace(ctx, req)

	// 注释：成功处理改动 - 根据新API规范，Response不再有Namespace字段，改为使用请求参数处理回调
	if api.IsSuccess(resp) {
		_ = svr.afterNamespaceResource(ctx, req, false)

		// 注释：数据封装改动 - 根据新的 pole-io/specification，在Response的data字段中包含namespace信息
		if resp.Data == nil {
			if anyData, err := anypb.New(req); err == nil {
				resp.Data = anyData
			}
		}
	}

	return resp
}

// CreateNamespaces 创建命名空间，基于新API规范重新设计
func (svr *Server) CreateNamespaces(
	ctx context.Context, reqs []*apimodel.Namespace) *apimodel.BatchWriteResponse {
	authCtx := svr.collectNamespaceAuthContext(ctx, reqs, authtypes.Create, authtypes.CreateNamespaces)

	// 验证 token 信息
	if _, err := svr.policySvr.GetAuthChecker().CheckConsolePermission(authCtx); err != nil {
		return api.NewBatchWriteResponse(authtypes.ConvertToErrCode(err))
	}

	ctx = authCtx.GetRequestContext()
	ctx = context.WithValue(ctx, types.ContextAuthContextKey, authCtx)

	// 填充 ownerId 信息数据
	ownerId := utils.ParseOwnerID(ctx)
	if len(ownerId) > 0 {
		for index := range reqs {
			req := reqs[index]
			// 注释：批量处理改动 - Owners字段类型从wrapper改为string
			req.Owners = string(ownerId)
		}
	}
	resp := svr.nextSvr.CreateNamespaces(ctx, reqs)

	// 注释：批量回调改动 - 使用请求数组而非响应数组进行回调处理，适应新的API规范
	for i := range reqs {
		if len(resp.Responses) > i && api.IsSuccess(resp.Responses[i]) {
			_ = svr.afterNamespaceResource(ctx, reqs[i], false)

			// 注释：批量数据封装改动 - 根据新的 pole-io/specification，在Response的data字段中包含namespace信息
			if resp.Responses[i].Data == nil {
				if anyData, err := anypb.New(reqs[i]); err == nil {
					resp.Responses[i].Data = anyData
				}
			}
		}
	}
	return resp
}

// DeleteNamespaces 删除命名空间，基于新API规范重新设计
func (svr *Server) DeleteNamespaces(
	ctx context.Context, reqs []*apimodel.Namespace) *apimodel.BatchWriteResponse {
	authCtx := svr.collectNamespaceAuthContext(ctx, reqs, authtypes.Delete, authtypes.DeleteNamespaces)
	if _, err := svr.policySvr.GetAuthChecker().CheckConsolePermission(authCtx); err != nil {
		return api.NewBatchWriteResponse(authtypes.ConvertToErrCode(err))
	}

	ctx = authCtx.GetRequestContext()
	ctx = context.WithValue(ctx, types.ContextAuthContextKey, authCtx)

	resp := svr.nextSvr.DeleteNamespaces(ctx, reqs)

	// 注释：删除回调改动 - 使用请求数组处理删除成功的namespace资源回调，适应新API规范
	for i := range reqs {
		if len(resp.Responses) > i && api.IsSuccess(resp.Responses[i]) {
			_ = svr.afterNamespaceResource(ctx, reqs[i], true)

			// 注释：删除数据封装改动 - 根据新的 pole-io/specification，在Response的data字段中包含已删除的namespace信息
			if resp.Responses[i].Data == nil {
				if anyData, err := anypb.New(reqs[i]); err == nil {
					resp.Responses[i].Data = anyData
				}
			}
		}
	}
	return resp
}

// UpdateNamespaces 更新命名空间，基于新API规范重新设计
func (svr *Server) UpdateNamespaces(
	ctx context.Context, req []*apimodel.Namespace) *apimodel.BatchWriteResponse {
	authCtx := svr.collectNamespaceAuthContext(ctx, req, authtypes.Modify, authtypes.UpdateNamespaces)
	if _, err := svr.policySvr.GetAuthChecker().CheckConsolePermission(authCtx); err != nil {
		return api.NewBatchWriteResponse(authtypes.ConvertToErrCode(err))
	}

	ctx = authCtx.GetRequestContext()
	ctx = context.WithValue(ctx, types.ContextAuthContextKey, authCtx)

	resp := svr.nextSvr.UpdateNamespaces(ctx, req)

	// 注释：更新回调改动 - 使用请求数组处理更新成功的namespace资源回调，保持功能一致性
	for i := range req {
		if len(resp.Responses) > i && api.IsSuccess(resp.Responses[i]) {
			_ = svr.afterNamespaceResource(ctx, req[i], false)

			// 注释：更新数据封装改动 - 根据新的 pole-io/specification，在Response的data字段中包含更新后的namespace信息
			if resp.Responses[i].Data == nil {
				if anyData, err := anypb.New(req[i]); err == nil {
					resp.Responses[i].Data = anyData
				}
			}
		}
	}
	return resp
}

// GetNamespaces 获取命名空间列表信息，基于新的API规范重新设计
func (svr *Server) GetNamespaces(
	ctx context.Context, query map[string][]string) *apimodel.BatchQueryResponse {
	authCtx := svr.collectNamespaceAuthContext(ctx, nil, authtypes.Read, authtypes.DescribeNamespaces)
	if _, err := svr.policySvr.GetAuthChecker().CheckConsolePermission(authCtx); err != nil {
		return api.NewBatchQueryResponse(authtypes.ConvertToErrCode(err))
	}

	ctx = authCtx.GetRequestContext()
	ctx = context.WithValue(ctx, types.ContextAuthContextKey, authCtx)

	// 注释：权限过滤改动 - 应用命名空间级别的权限过滤，保持权限控制功能不变
	ctx = cacheapi.AppendNamespacePredicate(ctx, func(ctx context.Context, n *types.Namespace) bool {
		return svr.policySvr.GetAuthChecker().ResourcePredicate(authCtx, &authtypes.ResourceEntry{
			Type:     apisecurity.ResourceType_Namespaces,
			ID:       n.Name,
			Metadata: n.Metadata,
		})
	})

	authCtx.SetRequestContext(ctx)
	resp := svr.nextSvr.GetNamespaces(ctx, query)

	// 注释：权限处理重大改动 - 根据新的 pole-io/specification，需要从 Data 字段中解析 namespaces 并设置权限信息
	return svr.processNamespacesWithPermissions(ctx, resp, authCtx)
}

// collectNamespaceAuthContext 对于命名空间的处理，收集所有的与鉴权的相关信息
func (svr *Server) collectNamespaceAuthContext(ctx context.Context, req []*apimodel.Namespace,
	resourceOp authtypes.ResourceOperation, methodName authtypes.ServerFunctionName) *authtypes.AcquireContext {
	return authtypes.NewAcquireContext(
		authtypes.WithRequestContext(ctx),
		authtypes.WithOperation(resourceOp),
		authtypes.WithModule(authtypes.CoreModule),
		authtypes.WithMethod(methodName),
		authtypes.WithAccessResources(svr.queryNamespaceResource(req)),
	)
}

// queryNamespaceResource 根据所给的 namespace 信息，收集对应的 ResourceEntry 列表
func (svr *Server) queryNamespaceResource(
	req []*apimodel.Namespace) map[apisecurity.ResourceType][]authtypes.ResourceEntry {
	if len(req) == 0 {
		return map[apisecurity.ResourceType][]authtypes.ResourceEntry{}
	}

	names := container.NewSet[string]()
	for index := range req {
		// 注释：命名空间名称提取改动 - Name字段从wrapper类型改为string，直接访问
		names.Add(req[index].Name)
	}
	param := names.ToSlice()
	nsArr := svr.cacheSvr.Namespace().GetNamespacesByName(param)

	temp := make([]authtypes.ResourceEntry, 0, len(nsArr))

	for index := range nsArr {
		ns := nsArr[index]
		temp = append(temp, authtypes.ResourceEntry{
			Type:  apisecurity.ResourceType_Namespaces,
			ID:    ns.Name,
			Owner: ns.Owner,
		})
	}

	ret := map[apisecurity.ResourceType][]authtypes.ResourceEntry{
		apisecurity.ResourceType_Namespaces: temp,
	}
	authLog.Debug("[Auth][Server] collect namespace access res", zap.Any("res", ret))
	return ret
}

// processNamespacesWithPermissions 基于新API规范处理命名空间权限
// 注释：权限处理重大重构 - 从BatchQueryResponse的Data字段解析namespace并设置权限信息
func (svr *Server) processNamespacesWithPermissions(
	ctx context.Context, resp *apimodel.BatchQueryResponse, authCtx *authtypes.AcquireContext) *apimodel.BatchQueryResponse {

	// 如果响应失败或没有数据，直接返回
	if resp == nil || resp.Code != uint32(apimodel.Code_ExecuteSuccess) || len(resp.Data) == 0 {
		return resp
	}
	authCtx.SetAttachment(authtypes.PermissionCheckPurposeKey, authtypes.PermissionCheckPurposeCapability)

	// 注释：Data字段处理 - 遍历Data中的每个namespace，设置权限信息，适应新的API结构
	for _, anyData := range resp.Data {
		if anyData == nil {
			continue
		}

		// 尝试将Any类型解析为Namespace
		namespace := &apimodel.Namespace{}
		if err := anyData.UnmarshalTo(namespace); err != nil {
			// 如果解析失败，跳过此项
			continue
		}

		// 设置访问资源信息
		authCtx.SetAccessResources(map[apisecurity.ResourceType][]authtypes.ResourceEntry{
			apisecurity.ResourceType_Namespaces: {
				{
					Type:     apisecurity.ResourceType_Namespaces,
					ID:       namespace.GetId(),
					Metadata: namespace.GetMetadata(),
				},
			},
		})

		// 注释：权限检查保持不变 - 检查写操作权限
		authCtx.SetMethod([]authtypes.ServerFunctionName{authtypes.UpdateNamespaces})
		if _, err := svr.policySvr.GetAuthChecker().CheckConsolePermission(authCtx); err != nil {
			namespace.Editable = false
		}

		// 注释：删除权限检查 - 检查删除操作权限，逻辑保持一致
		authCtx.SetMethod([]authtypes.ServerFunctionName{authtypes.DeleteNamespaces})
		if _, err := svr.policySvr.GetAuthChecker().CheckConsolePermission(authCtx); err != nil {
			namespace.Deleteable = false
		}

		// 注释：数据更新改动 - 将修改后的namespace重新序列化到anyData中，避免直接赋值造成锁复制
		if newAnyData, err := anypb.New(namespace); err == nil {
			// 更新Data中的内容 - 避免直接赋值造成锁复制
			anyData.TypeUrl = newAnyData.TypeUrl
			anyData.Value = newAnyData.Value
		}
	}

	return resp
}
