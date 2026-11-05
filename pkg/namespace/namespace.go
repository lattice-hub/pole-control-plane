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

package namespace

import (
	"context"
	"strings"
	"time"

	"github.com/golang/protobuf/jsonpb"
	"go.uber.org/zap"

	apimodel "github.com/pole-io/specification/source/go/api/v1/model"

	cacheapi "github.com/pole-io/pole-server/apis/cache"
	"github.com/pole-io/pole-server/apis/pkg/types"
	storeapi "github.com/pole-io/pole-server/apis/store"
	api "github.com/pole-io/pole-server/pkg/common/api/v1"
	commonotel "github.com/pole-io/pole-server/pkg/common/otel"
	"github.com/pole-io/pole-server/pkg/common/utils"
	commontime "github.com/pole-io/pole-server/pkg/common/utils/time"
	"github.com/pole-io/pole-server/pkg/common/utils/valid"
)

var _ NamespaceOperateServer = (*Server)(nil)

func (s *Server) allowAutoCreate() bool {
	return s.cfg.AutoCreate
}

func AllowAutoCreate(ctx context.Context) context.Context {
	ctx = context.WithValue(ctx, utils.ContextKeyAutoCreateNamespace{}, true)
	return ctx
}

// CreateNamespaces 批量创建命名空间
func (s *Server) CreateNamespaces(ctx context.Context, req []*apimodel.Namespace) *apimodel.BatchWriteResponse {
	if checkError := checkBatchNamespace(req); checkError != nil {
		return checkError
	}

	responses := api.NewBatchWriteResponse(apimodel.Code_ExecuteSuccess)
	for _, namespace := range req {
		response := s.CreateNamespace(ctx, namespace)
		api.Collect(responses, response)
	}

	return responses
}

// CreateNamespaceIfAbsent 创建命名空间，如果不存在
func (s *Server) CreateNamespaceIfAbsent(ctx context.Context, req *apimodel.Namespace) (string, *apimodel.Response) {
	if resp := checkNamespaceRequest(req); resp != nil {
		return "", resp
	}
	// 注释：字段访问改动 - GetName()直接返回string而非*wrapperspb.StringValue，去掉.GetValue()调用
	name := req.GetName()
	val, err := s.loadNamespace(name)
	if err != nil {
		return "", api.NewNamespaceResponse(storeapi.StoreCode2APICode(err), req)
	}
	if val != "" {
		return name, api.NewNamespaceResponse(apimodel.Code_ExecuteSuccess, req)
	}
	if resp := checkCreateNamespace(req); resp != nil {
		return "", resp
	}
	if !s.allowAutoCreate() {
		ctxVal := ctx.Value(utils.ContextKeyAutoCreateNamespace{})
		if ctxVal == nil || ctxVal.(bool) != true {
			// 注释：错误码改动 - Code_NotFoundNamespace已被移除，使用通用的Code_NotFoundResource
			return "", api.NewResponse(apimodel.Code_NotFoundResource)
		}
	}
	ret, err, _ := s.createNamespaceSingle.Do(name, func() (interface{}, error) {
		return s.CreateNamespace(ctx, req), nil
	})
	if err != nil {
		return "", api.NewResponseWithMsg(apimodel.Code_ExecuteException, err.Error())
	}
	var (
		resp = ret.(*apimodel.Response)
		// 注释：响应码访问改动 - GetCode()直接返回uint32而非*wrapperspb.UInt32Value
		code = resp.GetCode()
	)
	if code == uint32(apimodel.Code_ExecuteSuccess) || code == uint32(apimodel.Code_ExistedResource) {
		return name, api.NewNamespaceResponse(apimodel.Code_ExecuteSuccess, req)
	}
	return "", resp
}

// CreateNamespace 创建单个命名空间
func (s *Server) CreateNamespace(ctx context.Context, req *apimodel.Namespace) *apimodel.Response {
	// 参数检查
	if checkError := checkCreateNamespace(req); checkError != nil {
		return checkError
	}

	// 注释：命名空间名称获取改动 - GetName()返回string，去掉.GetValue()方法调用
	namespaceName := req.GetName()

	// 检查是否存在
	namespace, err := s.storage.GetNamespace(namespaceName)
	if err != nil {
		log.Error(err.Error(), utils.RequestID(ctx))
		return api.NewNamespaceResponse(storeapi.StoreCode2APICode(err), req)
	}
	if namespace != nil {
		return api.NewNamespaceResponse(apimodel.Code_ExistedResource, req)
	}

	data := s.createNamespaceModel(req)

	// 存储层操作
	if err := s.storage.AddNamespace(data); err != nil {
		log.Error(err.Error(), utils.RequestID(ctx))
		return api.NewNamespaceResponse(storeapi.StoreCode2APICode(err), req)
	}

	log.Info("create namespace", utils.RequestID(ctx), zap.String("name", namespaceName))
	out := &apimodel.Namespace{
		Name: req.GetName(),
	}
	s.RecordHistory(namespaceRecordEntry(ctx, req, types.OCreate))
	return api.NewNamespaceResponse(apimodel.Code_ExecuteSuccess, out)
}

/**
 * @brief 创建存储层命名空间模型
 */
func (s *Server) createNamespaceModel(req *apimodel.Namespace) *types.Namespace {
	// 注释：模型创建改动 - 所有字段访问从wrapper类型改为基础类型，业务逻辑保持不变
	namespace := &types.Namespace{
		Name:     req.GetName(),
		Comment:  req.GetComment(),
		Owner:    req.GetOwners(),
		Token:    utils.NewUUID(),
		Kind:     apimodel.NamespaceKind_NAMESPACE_KIND_BUSINESS,
		Metadata: req.GetMetadata(),
	}
	return namespace
}

// DeleteNamespaces 批量删除命名空间
func (s *Server) DeleteNamespaces(ctx context.Context, req []*apimodel.Namespace) *apimodel.BatchWriteResponse {
	if checkError := checkBatchNamespace(req); checkError != nil {
		return checkError
	}

	responses := api.NewBatchWriteResponse(apimodel.Code_ExecuteSuccess)
	for _, namespace := range req {
		response := s.DeleteNamespace(ctx, namespace)
		api.Collect(responses, response)
	}

	return responses
}

// DeleteNamespace 删除单个命名空间
func (s *Server) DeleteNamespace(ctx context.Context, req *apimodel.Namespace) *apimodel.Response {
	// 参数检查
	if checkError := checkReviseNamespace(ctx, req); checkError != nil {
		return checkError
	}
	if s.isProtectedNamespace(req.GetName()) {
		response := api.NewNamespaceResponse(apimodel.Code_InvalidParameter, req)
		response.Info += ": built-in namespace cannot be deleted"
		return response
	}

	tx, err := s.storage.CreateTransaction()
	if err != nil {
		log.Error(err.Error(), utils.RequestID(ctx))
		return api.NewNamespaceResponse(storeapi.StoreCode2APICode(err), req)
	}
	defer func() { _ = tx.Commit() }()

	// 检查是否存在
	// 注释：命名空间锁定改动 - GetName()直接返回string，删除操作逻辑保持不变
	namespace, err := tx.LockNamespace(req.GetName())
	if err != nil {
		log.Error(err.Error(), utils.RequestID(ctx))
		return api.NewNamespaceResponse(storeapi.StoreCode2APICode(err), req)
	}
	if namespace == nil {
		return api.NewNamespaceResponse(apimodel.Code_ExecuteSuccess, req)
	}
	if isSystemNamespace(namespace) {
		response := api.NewNamespaceResponse(apimodel.Code_InvalidParameter, req)
		response.Info += ": system namespace cannot be deleted"
		return response
	}

	// 判断属于该命名空间的服务是否都已经被删除
	total, err := s.getServicesCountWithNamespace(namespace.Name)
	if err != nil {
		log.Error("get services count with namespace err", utils.RequestID(ctx), zap.Error(err))
		return api.NewNamespaceResponse(storeapi.StoreCode2APICode(err), req)
	}
	if total != 0 {
		log.Error("the removed namespace has remain services", utils.RequestID(ctx))
		return api.NewNamespaceResponse(apimodel.Code_NamespaceExistedServices, req)
	}

	// MCP Server 和 A2A Agent 都是环境实例，删除 Namespace 前必须先移除这些实例。
	aiResourceTotal, err := s.storage.CountAIResourcesByNamespace(namespace.Name)
	if err != nil {
		log.Error("get ai resource count with namespace err", utils.RequestID(ctx), zap.Error(err))
		return api.NewNamespaceResponse(storeapi.StoreCode2APICode(err), req)
	}
	if aiResourceTotal != 0 {
		log.Error("the removed namespace has remain ai resources", utils.RequestID(ctx))
		response := api.NewNamespaceResponse(apimodel.Code_ExistedResource, req)
		response.Info += ": namespace still contains MCP Server or A2A Agent"
		return response
	}

	// 统一配置保留草稿及历史，任何聚合仍归属该环境时都不能删除环境。
	configurationCount, err := s.storage.CountUnifiedConfigs(ctx, namespace.Name)
	if err != nil {
		log.Error("count unified configurations in environment failed", utils.RequestID(ctx), zap.Error(err))
		return api.NewNamespaceResponse(storeapi.StoreCode2APICode(err), req)
	}
	if configurationCount != 0 {
		log.Error("the removed namespace still contains configurations", utils.RequestID(ctx))
		return api.NewNamespaceResponse(apimodel.Code_NamespaceExistedConfigGroups, req)
	}

	governanceRuleTotal, err := s.storage.CountGovernanceRules(namespace.Name)
	if err != nil {
		log.Error("get governance rule count with namespace err", utils.RequestID(ctx), zap.Error(err))
		return api.NewNamespaceResponse(storeapi.StoreCode2APICode(err), req)
	}
	if governanceRuleTotal != 0 {
		log.Error("the removed namespace has remain governance rules", utils.RequestID(ctx))
		return api.NewNamespaceResponse(apimodel.Code_NamespaceExistedGovernanceRules, req)
	}

	// 存储层操作
	if err := tx.DeleteNamespace(namespace.Name); err != nil {
		log.Error(err.Error(), utils.RequestID(ctx))
		return api.NewNamespaceResponse(storeapi.StoreCode2APICode(err), req)
	}

	s.caches.Service().CleanNamespace(namespace.Name)

	log.Info("delete namespace", utils.RequestID(ctx), zap.String("name", namespace.Name))
	s.RecordHistory(namespaceRecordEntry(ctx, req, types.ODelete))
	return api.NewNamespaceResponse(apimodel.Code_ExecuteSuccess, req)
}

// UpdateNamespaces 批量修改命名空间
func (s *Server) UpdateNamespaces(ctx context.Context, req []*apimodel.Namespace) *apimodel.BatchWriteResponse {
	if checkError := checkBatchNamespace(req); checkError != nil {
		return checkError
	}

	responses := api.NewBatchWriteResponse(apimodel.Code_ExecuteSuccess)
	for _, namespace := range req {
		response := s.UpdateNamespace(ctx, namespace)
		api.Collect(responses, response)
	}

	return responses
}

// UpdateNamespace 修改单个命名空间
func (s *Server) UpdateNamespace(ctx context.Context, req *apimodel.Namespace) *apimodel.Response {
	// 参数检查
	if resp := checkReviseNamespace(ctx, req); resp != nil {
		return resp
	}

	// 权限校验
	namespace, resp := s.checkNamespaceAuthority(ctx, req)
	if resp != nil {
		return resp
	}
	// 修改
	s.updateNamespaceAttribute(req, namespace)

	// 存储层操作
	if err := s.storage.UpdateNamespace(namespace); err != nil {
		log.Error(err.Error(), utils.RequestID(ctx))
		return api.NewNamespaceResponse(storeapi.StoreCode2APICode(err), req)
	}

	log.Info("update namespace", zap.String("name", namespace.Name), utils.RequestID(ctx))
	s.RecordHistory(namespaceRecordEntry(ctx, req, types.OUpdate))
	return api.NewNamespaceResponse(apimodel.Code_ExecuteSuccess, req)
}

/**
 * @brief 修改命名空间属性
 */
func (s *Server) updateNamespaceAttribute(req *apimodel.Namespace, namespace *types.Namespace) {
	// 注释：属性更新改动 - 字段访问从wrapper类型改为基础类型，直接赋值而非检查nil
	namespace.Comment = req.GetComment()
	namespace.Owner = req.GetOwners()

	namespace.Metadata = req.GetMetadata()
	namespace.ServiceExportTo = nil
}

// GetNamespaces 查询命名空间
func (s *Server) GetNamespaces(ctx context.Context, query map[string][]string) *apimodel.BatchQueryResponse {
	ctx, endServiceSpan := commonotel.StartSpan(ctx, "service.namespace.get", "service")
	defer endServiceSpan(nil)

	filter, offset, limit, checkError := checkGetNamespace(query)
	if checkError != nil {
		return checkError
	}

	amount, namespaces, err := s.caches.Namespace().Query(ctx, &cacheapi.NamespaceArgs{
		Filter: filter,
		Offset: offset,
		Limit:  limit,
	})
	if err != nil {
		return api.NewBatchQueryResponse(storeapi.StoreCode2APICode(err))
	}

	out := api.NewBatchQueryResponse(apimodel.Code_ExecuteSuccess)
	// 注释：响应字段改动 - Amount和Size从*wrapperspb.UInt32Value改为uint32，直接赋值
	out.Amount = uint32(amount)
	out.Size = uint32(len(namespaces))
	_, endStoreSpan := commonotel.StartSpan(ctx, "store.config.count_by_namespace", "store")
	configFilesByNamespace, err := s.storage.CountUnifiedConfigsByEnvironment(ctx)
	endStoreSpan(err)
	if err != nil {
		log.Error("count config files by namespace failed", utils.RequestID(ctx), zap.Error(err))
		return api.NewBatchQueryResponse(storeapi.StoreCode2APICode(err))
	}
	configFileCounts := configFilesByNamespace
	for _, namespace := range namespaces {
		nsCntInfo := s.caches.Service().GetNamespaceCntInfo(namespace.Name)
		// 注释：命名空间数据构造改动 - 所有字段从wrapper类型改为基础类型，数据处理逻辑保持不变
		if err := api.AddNamespace(out, &apimodel.Namespace{
			Id:                       string(namespace.Name),
			Name:                     string(namespace.Name),
			Comment:                  string(namespace.Comment),
			Owners:                   string(namespace.Owner),
			Ctime:                    string(commontime.Time2String(namespace.CreateTime)),
			Mtime:                    string(commontime.Time2String(namespace.ModifyTime)),
			TotalServiceCount:        uint32(nsCntInfo.ServiceCount),
			TotalInstanceCount:       uint32(nsCntInfo.InstanceCnt.TotalInstanceCount),
			TotalHealthInstanceCount: uint32(nsCntInfo.InstanceCnt.HealthyInstanceCount),
			TotalConfigFileCount:     uint32(min(configFileCounts[namespace.Name], uint64(1<<32-1))),
			Editable:                 true,
			Deleteable:               !s.isProtectedNamespace(namespace.Name) && !isSystemNamespace(namespace),
			Kind:                     namespace.Kind,
			Metadata:                 namespace.Metadata,
		}); err != nil {
			log.Error("add namespace to batch query response failed", zap.Error(err))
			return api.NewBatchQueryResponse(apimodel.Code_ExecuteException)
		}
	}
	return out
}

// 根据命名空间查询服务总数
func (s *Server) getServicesCountWithNamespace(namespace string) (uint32, error) {
	filter := map[string]string{"namespace": namespace}
	total, _, err := s.storage.GetServices(filter, nil, nil, 0, 1)
	if err != nil {
		return 0, err
	}
	return total, nil
}

// loadNamespace
func (s *Server) loadNamespace(name string) (string, error) {
	if val := s.caches.Namespace().GetNamespace(name); val != nil {
		return name, nil
	}
	val, err := s.storage.GetNamespace(name)
	if err != nil {
		return "", err
	}
	if val == nil {
		return "", nil
	}
	return val.Name, nil
}

// 检查namespace的权限，并且返回namespace
func (s *Server) checkNamespaceAuthority(
	ctx context.Context, req *apimodel.Namespace) (*types.Namespace, *apimodel.Response) {
	// 注释：命名空间权限检查改动 - GetName()返回string，权限验证逻辑保持不变
	namespaceName := req.GetName()
	// namespaceToken := parseNamespaceToken(ctx, req)

	// 检查是否存在
	namespace, err := s.storage.GetNamespace(namespaceName)
	if err != nil {
		log.Error(err.Error(), utils.RequestID(ctx))
		return nil, api.NewNamespaceResponse(storeapi.StoreCode2APICode(err), req)
	}
	if namespace == nil {
		return nil, api.NewNamespaceResponse(apimodel.Code_NotFoundResource, req)
	}
	return namespace, nil
}

// 检查批量请求
func checkBatchNamespace(req []*apimodel.Namespace) *apimodel.BatchWriteResponse {
	if len(req) == 0 {
		return api.NewBatchWriteResponse(apimodel.Code_EmptyRequest)
	}

	if len(req) > valid.MaxBatchSize {
		return api.NewBatchWriteResponse(apimodel.Code_BatchSizeOverLimit)
	}

	return nil
}

// 检查创建命名空间请求参数
func checkCreateNamespace(req *apimodel.Namespace) *apimodel.Response {
	if response := checkNamespaceRequest(req); response != nil {
		return response
	}
	if req.GetName() == SystemNamespace ||
		req.GetKind() != apimodel.NamespaceKind_NAMESPACE_KIND_BUSINESS {
		response := api.NewNamespaceResponse(apimodel.Code_InvalidParameter, req)
		response.Info += ": system namespace can only be provisioned by Pole"
		return response
	}

	return nil
}

func checkNamespaceRequest(req *apimodel.Namespace) *apimodel.Response {
	if req == nil {
		return api.NewNamespaceResponse(apimodel.Code_EmptyRequest, req)
	}

	if err := valid.CheckResourceName(req.GetName()); err != nil {
		// 注释：错误码改动 - InvalidNamespaceName已被移除，使用通用的InvalidParameter错误码
		return api.NewNamespaceResponse(apimodel.Code_InvalidParameter, req)
	}
	return nil
}

// 检查删除/修改命名空间请求参数
func checkReviseNamespace(ctx context.Context, req *apimodel.Namespace) *apimodel.Response {
	if req == nil {
		return api.NewNamespaceResponse(apimodel.Code_EmptyRequest, req)
	}

	if err := valid.CheckResourceName(req.GetName()); err != nil {
		// 注释：错误码统一改动 - InvalidNamespaceName改为InvalidParameter，保持验证逻辑一致
		return api.NewNamespaceResponse(apimodel.Code_InvalidParameter, req)
	}
	return nil
}

// 检查查询命名空间请求参数
func checkGetNamespace(query map[string][]string) (map[string][]string, int, int, *apimodel.BatchQueryResponse) {
	filter := make(map[string][]string)

	if value := query["name"]; len(value) > 0 {
		filter["name"] = value
	}

	if value := query["owner"]; len(value) > 0 {
		filter["owner"] = value
	}

	if value := query["kind"]; len(value) > 0 {
		if len(value) != 1 {
			return nil, 0, 0, api.NewBatchQueryResponse(apimodel.Code_InvalidParameter)
		}
		switch strings.ToLower(value[0]) {
		case "business":
			filter["kind"] = []string{"business"}
		case "system":
			filter["kind"] = []string{"system"}
		default:
			return nil, 0, 0, api.NewBatchQueryResponse(apimodel.Code_InvalidParameter)
		}
	}

	offset, err := valid.CheckQueryOffset(query["offset"])
	if err != nil {
		return nil, 0, 0, api.NewBatchQueryResponse(apimodel.Code_InvalidParameter)
	}

	limit, err := valid.CheckQueryLimit(query["limit"])
	if err != nil {
		return nil, 0, 0, api.NewBatchQueryResponse(apimodel.Code_InvalidParameter)
	}

	return filter, offset, limit, nil
}

// 生成命名空间的记录entry
func namespaceRecordEntry(ctx context.Context, req *apimodel.Namespace, opt types.OperationType) *types.RecordEntry {
	marshaler := jsonpb.Marshaler{}
	datail, _ := marshaler.MarshalToString(req)
	return &types.RecordEntry{
		ResourceType: types.RNamespace,
		// 注释：记录条目改动 - GetName()返回string，历史记录功能保持不变
		ResourceName:  req.GetName(),
		Namespace:     req.GetName(),
		OperationType: opt,
		Operator:      utils.ParseOperator(ctx),
		Detail:        datail,
		HappenTime:    time.Now(),
	}
}
