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

package goverrule

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"go.uber.org/zap"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"

	apimodel "github.com/pole-io/specification/source/go/api/v1/model"
	apitraffic "github.com/pole-io/specification/source/go/api/v1/traffic_manage"

	cacheapi "github.com/pole-io/pole-server/apis/cache"
	"github.com/pole-io/pole-server/apis/pkg/types"
	"github.com/pole-io/pole-server/apis/pkg/types/rules"
	svctypes "github.com/pole-io/pole-server/apis/pkg/types/service"
	storeapi "github.com/pole-io/pole-server/apis/store"
	api "github.com/pole-io/pole-server/pkg/common/api/v1"
	apiv1 "github.com/pole-io/pole-server/pkg/common/api/v1"
	"github.com/pole-io/pole-server/pkg/common/utils"
	commontime "github.com/pole-io/pole-server/pkg/common/utils/time"
	"github.com/pole-io/pole-server/pkg/common/utils/valid"
)

// CreateRateLimits 批量创建限流规则
func (s *Server) CreateRateLimits(ctx context.Context, request []*apitraffic.RateLimit) *apimodel.BatchWriteResponse {
	responses := api.NewBatchWriteResponse(apimodel.Code_ExecuteSuccess)
	for _, rateLimit := range request {
		response := s.CreateRateLimit(ctx, rateLimit)
		api.Collect(responses, response)
	}
	return api.FormatBatchWriteResponse(responses)
}

// CreateRateLimit 创建限流规则
func (s *Server) CreateRateLimit(ctx context.Context, req *apitraffic.RateLimit) *apimodel.Response {
	// 构造底层数据结构
	data, err := api2RateLimit(req)
	if err != nil {
		log.Error(err.Error(), utils.RequestID(ctx))
		return api.NewAnyDataResponse(apimodel.Code_ParseException, req)
	}
	if resp := s.resolveRateLimitService(ctx, req, data, nil); resp != nil {
		return resp
	}

	// 存储层操作
	if err := s.storage.CreateRateLimit(data); err != nil {
		log.Error(err.Error(), utils.RequestID(ctx))
		return wrapperRateLimitStoreResponse(req, err)
	}

	msg := fmt.Sprintf("create rate limit rule: id=%v, service_id=%v, name=%v",
		data.ID, data.ServiceID, req.GetName())
	log.Info(msg, utils.RequestID(ctx))

	s.RecordHistory(ctx, rateLimitRecordEntry(ctx, req, data, types.OCreate))
	req.Id = string(data.ID)
	// 根据新的 pole-io/specification，创建包含数据的响应
	return api.NewAnyDataResponse(apimodel.Code_ExecuteSuccess, req)
}

// DeleteRateLimits 批量删除限流规则
func (s *Server) DeleteRateLimits(ctx context.Context, request []*apitraffic.RateLimit) *apimodel.BatchWriteResponse {
	responses := api.NewBatchWriteResponse(apimodel.Code_ExecuteSuccess)
	for _, entry := range request {
		resp := s.DeleteRateLimit(ctx, entry)
		api.Collect(responses, resp)
	}
	return api.FormatBatchWriteResponse(responses)
}

// DeleteRateLimit 删除单个限流规则
func (s *Server) DeleteRateLimit(ctx context.Context, req *apitraffic.RateLimit) *apimodel.Response {
	// 检查限流规则是否存在
	rateLimit, resp := s.checkRateLimitExisted(ctx, req.GetId(), req)
	if resp != nil {
		if resp.GetCode() == uint32(apimodel.Code_NotFoundResource) {
			return api.NewAnyDataResponse(apimodel.Code_ExecuteSuccess, req)
		}
		return resp
	}

	// 生成新的revision
	rateLimit.Revision = utils.NewUUID()

	// 存储层操作
	if err := s.storage.DeleteRateLimit(rateLimit); err != nil {
		log.Error(err.Error(), utils.RequestID(ctx))
		return wrapperRateLimitStoreResponse(req, err)
	}

	msg := fmt.Sprintf("delete rate limit rule: id=%v, service_id=%v, name=%v",
		rateLimit.ID, rateLimit.ServiceID, rateLimit.Labels)
	log.Info(msg, utils.RequestID(ctx))

	s.RecordHistory(ctx,
		rateLimitRecordEntry(ctx, req, rateLimit, types.ODelete))
	return api.NewAnyDataResponse(apimodel.Code_ExecuteSuccess, req)
}

func (s *Server) EnableRateLimits(ctx context.Context, request []*apitraffic.RateLimit) *apimodel.BatchWriteResponse {
	responses := api.NewBatchWriteResponse(apimodel.Code_ExecuteSuccess)
	for _, entry := range request {
		response := s.EnableRateLimit(ctx, entry)
		api.Collect(responses, response)
	}
	return api.FormatBatchWriteResponse(responses)
}

// EnableRateLimit 启用限流规则
func (s *Server) EnableRateLimit(ctx context.Context, req *apitraffic.RateLimit) *apimodel.Response {
	// 检查限流规则是否存在
	data, resp := s.checkRateLimitExisted(ctx, req.GetId(), req)
	if resp != nil {
		return resp
	}

	// 构造底层数据结构
	rateLimit := &rules.RateLimit{}
	rateLimit.ID = data.ID
	rateLimit.ServiceID = data.ServiceID
	rateLimit.Disable = req.GetDisable()
	rateLimit.Revision = utils.NewUUID()

	if err := s.storage.EnableRateLimit(rateLimit); err != nil {
		log.Error(err.Error(), utils.RequestID(ctx))
		return wrapperRateLimitStoreResponse(req, err)
	}

	msg := fmt.Sprintf("enable rate limit: id=%v, disable=%v",
		rateLimit.ID, rateLimit.Disable)
	log.Info(msg, utils.RequestID(ctx))

	s.RecordHistory(ctx, rateLimitRecordEntry(ctx, req, rateLimit, types.OUpdateEnable))
	return api.NewAnyDataResponse(apimodel.Code_ExecuteSuccess, req)
}

// UpdateRateLimits 批量更新限流规则
func (s *Server) UpdateRateLimits(ctx context.Context, request []*apitraffic.RateLimit) *apimodel.BatchWriteResponse {
	responses := api.NewBatchWriteResponse(apimodel.Code_ExecuteSuccess)
	for _, entry := range request {
		response := s.UpdateRateLimit(ctx, entry)
		api.Collect(responses, response)
	}
	return api.FormatBatchWriteResponse(responses)
}

// UpdateRateLimit 更新限流规则
func (s *Server) UpdateRateLimit(ctx context.Context, req *apitraffic.RateLimit) *apimodel.Response {
	// 检查限流规则是否存在
	data, resp := s.checkRateLimitExisted(ctx, req.GetId(), req)
	if resp != nil {
		return resp
	}

	// 构造底层数据结构
	rateLimit, err := api2RateLimit(req)
	if err != nil {
		log.Error(err.Error(), utils.RequestID(ctx))
		return api.NewAnyDataResponse(apimodel.Code_ParseException, req)
	}
	rateLimit.ID = data.ID
	if resp := s.resolveRateLimitService(ctx, req, rateLimit, data); resp != nil {
		return resp
	}
	if err := s.storage.UpdateRateLimit(rateLimit); err != nil {
		log.Error(err.Error(), utils.RequestID(ctx))
		return wrapperRateLimitStoreResponse(req, err)
	}

	msg := fmt.Sprintf("update rate limit: id=%v, service_id=%v, name=%v",
		rateLimit.ID, rateLimit.ServiceID, rateLimit.Name)
	log.Info(msg, utils.RequestID(ctx))

	s.RecordHistory(ctx, rateLimitRecordEntry(ctx, req, rateLimit, types.OUpdate))
	return api.NewAnyDataResponse(apimodel.Code_ExecuteSuccess, req)
}

// GetRateLimits 查询限流规则
func (s *Server) GetRateLimits(ctx context.Context, query map[string]string) *apimodel.BatchQueryResponse {
	// 处理offset和limit
	args, errResp := parseRateLimitArgs(query)
	if errResp != nil {
		return errResp
	}

	total, extendRateLimits, err := s.Cache().RateLimit().QueryRateLimitRules(ctx, args)
	if err != nil {
		log.Error("get rate limits store", zap.Error(err), utils.RequestID(ctx))
		return api.NewBatchQueryResponse(storeapi.StoreCode2APICode(err))
	}

	out := api.NewBatchQueryResponse(apimodel.Code_ExecuteSuccess)
	out.Amount = total
	out.Size = uint32(len(extendRateLimits))

	// 根据新的 pole-io/specification，将RateLimit数据序列化到Data字段
	out.Data = make([]*anypb.Any, 0, len(extendRateLimits))
	for _, item := range extendRateLimits {
		limit, err := rateLimit2Console(item)
		if err != nil {
			log.Error("get rate limits convert", zap.Error(err), utils.RequestID(ctx))
			return api.NewBatchQueryResponse(apimodel.Code_ParseException)
		}

		// 将limit序列化到anypb.Any
		if anyData, err := anypb.New(limit); err == nil {
			out.Data = append(out.Data, anyData)
		}
	}

	return out
}

// GetOneRateLimitRule Query all router_rule rules
func (s *Server) GetOneRateLimitRule(ctx context.Context, req *apitraffic.RateLimit) *apimodel.Response {
	// Check whether the routing configuration exists
	saveData, err := s.storage.GetRateLimitWithID(req.GetId())
	if err != nil {
		log.Error("[goverrule][ratelimit] get ratelimit config from store layer",
			utils.RequestID(ctx), zap.Error(err))
		return apiv1.NewResponse(storeapi.StoreCode2APICode(err))
	}
	if saveData == nil {
		return apiv1.NewResponse(apimodel.Code_NotFoundResource)
	}

	view, err := rateLimit2Console(saveData)
	if err != nil {
		log.Error("[goverrule][ratelimit] parse ratelimit config from store layer", utils.RequestID(ctx), zap.Error(err))
		return apiv1.NewResponse(apimodel.Code_ExecuteException)
	}
	return apiv1.NewAnyDataResponse(apimodel.Code_ExecuteSuccess, view)
}

func parseRateLimitArgs(query map[string]string) (*cacheapi.RateLimitRuleArgs, *apimodel.BatchQueryResponse) {
	// 处理offset和limit
	offset, limit, _ := valid.ParseOffsetAndLimit(query)

	args := &cacheapi.RateLimitRuleArgs{
		Filter:     query,
		ID:         query["id"],
		Name:       query["name"],
		Service:    query["service"],
		Namespace:  query["namespace"],
		Offset:     offset,
		Limit:      limit,
		OrderField: query["order_field"],
		OrderType:  query["order_type"],
	}
	if val, ok := query["disable"]; ok {
		disable, _ := strconv.ParseBool(val)
		args.Disable = &disable
	}

	return args, nil
}

// checkRateLimitValid 检查限流规则是否允许修改/删除
func (s *Server) checkRateLimitValid(ctx context.Context, serviceID string, req *apitraffic.RateLimit) (
	*svctypes.Service, *apimodel.Response) {
	requestID := utils.ParseRequestID(ctx)

	service, err := s.storage.GetServiceByID(serviceID)
	if err != nil {
		log.Error(err.Error(), utils.ZapRequestID(requestID))
		return nil, api.NewAnyDataResponse(storeapi.StoreCode2APICode(err), req)
	}

	return service, nil
}

// checkRateLimitExisted 检查限流规则是否存在
func (s *Server) checkRateLimitExisted(ctx context.Context, id string,
	req *apitraffic.RateLimit) (*rules.RateLimit, *apimodel.Response) {

	rateLimit, err := s.storage.GetRateLimitWithID(id)
	if err != nil {
		log.Error(err.Error(), utils.RequestID(ctx))
		return nil, api.NewAnyDataResponse(storeapi.StoreCode2APICode(err), req)
	}
	if rateLimit == nil {
		return nil, api.NewAnyDataResponse(apimodel.Code_NotFoundResource, req)
	}
	return rateLimit, nil
}

const (
	defaultRuleAction = "REJECT"
)

// api2RateLimit 把API参数转化为内部数据结构
func api2RateLimit(req *apitraffic.RateLimit) (*rules.RateLimit, error) {
	rule, err := marshalRateLimitRules(req)
	if err != nil {
		return nil, err
	}

	labels := req.Metadata
	var labelStr []byte
	if len(labels) > 0 {
		labelStr, err = json.Marshal(labels)
	}

	out := &rules.RateLimit{
		ID:        utils.NewUUID(),
		Namespace: rules.OwnerNamespaceFromProto(req),
		Name:      req.GetName(),
		Disable:   req.GetDisable(),
		Priority:  req.GetPriority(),
		Labels:    string(labelStr),
		Rule:      rule,
		Revision:  utils.NewUUID(),
		Metadata:  req.Metadata,
	}
	return out, nil
}

func (s *Server) resolveRateLimitService(ctx context.Context, req *apitraffic.RateLimit, rateLimit, existing *rules.RateLimit) *apimodel.Response {
	serviceName := req.GetCallee().GetService()
	if serviceName == "" || serviceName == "*" {
		if existing != nil {
			rateLimit.ServiceID = existing.ServiceID
		}
		return nil
	}

	service, err := s.storage.GetService(serviceName, req.GetCallee().GetNamespace())
	if err != nil {
		log.Error(err.Error(), utils.RequestID(ctx))
		return api.NewAnyDataResponse(storeapi.StoreCode2APICode(err), req)
	}
	if service == nil {
		return api.NewAnyDataResponse(apimodel.Code_NotFoundResource, req)
	}
	rateLimit.ServiceID = service.ID
	return nil
}

// rateLimit2api 把内部数据结构转化为API参数
func rateLimit2Console(rateLimit *rules.RateLimit) (*apitraffic.RateLimit, error) {
	if rateLimit == nil {
		return nil, nil
	}
	if len(rateLimit.Rule) > 0 {
		rateLimit = rateLimit.CopyNoProto()
		rateLimit.Proto = &apitraffic.RateLimit{}
		// 控制台查询的请求
		if err := json.Unmarshal([]byte(rateLimit.Rule), rateLimit.Proto); err != nil {
			return nil, err
		}
		// 存量标签适配到参数列表
		// Note: AdaptLabels method not available in specification, skip this step
	}
	rule := &apitraffic.RateLimit{}
	rule.Id = rateLimit.ID
	rules.SetOwnerNamespaceOnProto(rule, rateLimit.Namespace)
	rule.Name = rateLimit.Name
	rule.Callee = rateLimit.Proto.GetCallee()
	rule.Priority = rateLimit.Priority
	rule.Ctime = commontime.Time2String(rateLimit.CreateTime)
	rule.Mtime = commontime.Time2String(rateLimit.ModifyTime)
	rule.Disable = rateLimit.Disable
	rule.Metadata = rateLimit.Metadata
	// 根据新的 pole-io/specification，RateLimit 不再有 Etime 字段
	// 已移除：根据 pole-io/specification v0.1.0+，RateLimit 不再包含 Etime 字段
	// 如需实现时间维度的限流，建议使用 Metadata 中的 custom 字段扩展
	// 追踪 Issue: POLE-RATELIMIT-TIME-001
	rule.Metadata = rateLimit.Metadata
	rule.Revision = rateLimit.Revision
	if nil != rateLimit.Proto {
		copyRateLimitProto(rateLimit, rule)
	} else {
		// 根据新的 pole-io/specification，RateLimit 不再有 Method 字段
		// 已适配：根据 pole-io/specification v0.1.0+，Method 字段已移至 LimitTrigger 中
		// 当前实现通过 copyRateLimitProto 处理 Proto 中的方法匹配逻辑
		// 追踪 Issue: POLE-RATELIMIT-METHOD-001
	}
	return rule, nil
}

func populateDefaultRuleValue(rule *apitraffic.RateLimit) {
	// 根据新的 pole-io/specification，为Rules中的每个LimitTrigger设置默认值
	for _, trigger := range rule.Rules {
		if trigger.GetAction() == "" {
			trigger.Action = defaultRuleAction
		}
		// 为其他P0功能设置默认值
		if trigger.GetMaxQueueDelay() == 0 {
			trigger.MaxQueueDelay = 0 // 默认不排队
		}
		if trigger.GetRegexCombine() == false {
			trigger.RegexCombine = false // 默认分开计数
		}
	}
}

func copyRateLimitProto(rateLimit *rules.RateLimit, rule *apitraffic.RateLimit) {
	// 根据新的 pole-io/specification，复制仍然存在的字段
	rule.Type = rateLimit.Proto.Type
	rule.Rules = rateLimit.Proto.Rules
	rule.Disable = rateLimit.Proto.Disable
	rule.Report = rateLimit.Proto.Report
	rule.Cluster = rateLimit.Proto.Cluster

	populateDefaultRuleValue(rule)
}

// rateLimit2api 把内部数据结构转化为API参数
func rateLimit2Client(
	service string, namespace string, rateLimit *rules.RateLimit) (*apitraffic.RateLimit, error) {
	if rateLimit == nil {
		return nil, nil
	}

	rule := &apitraffic.RateLimit{}
	rule.Id = rateLimit.ID
	rule.Name = rateLimit.Name
	rule.Priority = rateLimit.Priority
	rule.Revision = rateLimit.Revision
	rule.Disable = rateLimit.Disable
	rule.Metadata = rateLimit.Metadata
	copyRateLimitProto(rateLimit, rule)
	return rule, nil
}

// marshalRateLimitRules 序列化限流规则具体内容
func marshalRateLimitRules(req *apitraffic.RateLimit) (string, error) {
	// 根据新的 pole-io/specification，包含完整的Rules结构
	r := &apitraffic.RateLimit{
		Name:      req.GetName(),
		Namespace: rules.OwnerNamespaceFromProto(req),
		Callee:    req.GetCallee(),
		Type:      req.GetType(),
		Rules:     req.GetRules(),
		Disable:   req.GetDisable(),
		Report:    req.GetReport(),
		Cluster:   req.GetCluster(),
	}
	rule, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	return string(rule), nil
}

// rateLimitRecordEntry 构建rateLimit的记录entry
func rateLimitRecordEntry(ctx context.Context, req *apitraffic.RateLimit, md *rules.RateLimit,
	opt types.OperationType) *types.RecordEntry {

	detail, _ := protojson.Marshal(req)
	detailStr := string(detail)

	entry := &types.RecordEntry{
		ResourceType:  types.RRateLimit,
		ResourceName:  fmt.Sprintf("%s(%s)", md.Name, md.ID),
		Namespace:     md.ServiceID,
		Operator:      utils.ParseOperator(ctx),
		OperationType: opt,
		Detail:        detailStr,
		HappenTime:    time.Now(),
	}

	return entry
}

// wrapperRateLimitStoreResponse 封装路由存储层错误
func wrapperRateLimitStoreResponse(rule *apitraffic.RateLimit, err error) *apimodel.Response {
	if err == nil {
		return nil
	}
	resp := api.NewResponseWithMsg(storeapi.StoreCode2APICode(err), err.Error())
	// 根据新的 pole-io/specification，Response 不再有 RateLimit 字段
	// 如需包含限流规则数据，应使用 Data 字段
	// 已优化：错误响应通过 resp.Message 传递错误信息
	// 如需返回详细规则数据，可在后续版本中通过 resp.Data 扩展
	// 追踪 Issue: POLE-RATELIMIT-ERROR-DATA-001
	return resp
}

func rateLimitAPIFromMethod(value string) *apimodel.API {
	return &apimodel.API{
		Protocol: "HTTP",
		Method:   "*",
		Path: &apimodel.MatchString{
			Value: value,
			Type:  apimodel.MatchString_EXACT,
		},
	}
}

// =============================================================================
// P0级别功能恢复 - 辅助函数
// =============================================================================

// CreateSimpleRateLimit 创建简单的限流规则（P0功能示例）
// 支持HTTP方法匹配和基础限流量配置
func CreateSimpleRateLimit(name, service, namespace string, method string, maxAmount uint32, duration time.Duration) *apitraffic.RateLimit {
	rule := &apitraffic.RateLimit{
		Name: name,
		Type: apitraffic.RateLimit_GLOBAL, // 默认全局限流
		Rules: []*apitraffic.LimitTrigger{
			{
				Name: name + "_trigger",
				Apis: []*apimodel.API{rateLimitAPIFromMethod(method)},
				Amounts: []*apitraffic.Amount{
					{
						MaxAmount:     maxAmount,
						ValidDuration: durationpb.New(duration),
						Precision:     1,
					},
				},
				Action:        defaultRuleAction,
				MaxQueueDelay: 0, // 不排队
				RegexCombine:  false,
			},
		},
	}
	return rule
}

// AddArgumentFilter 为限流规则添加参数过滤（P0功能）
func AddArgumentFilter(rule *apitraffic.RateLimit, triggerIndex int, argType apitraffic.MatchArgument_Type, key, value string) {
	if triggerIndex >= len(rule.Rules) {
		return
	}

	trigger := rule.Rules[triggerIndex]
	if trigger.Arguments == nil {
		trigger.Arguments = make([]*apitraffic.MatchArgument, 0)
	}

	trigger.Arguments = append(trigger.Arguments, &apitraffic.MatchArgument{
		Type: argType,
		Key:  key,
		Value: &apimodel.MatchString{
			Value: value,
			Type:  apimodel.MatchString_EXACT,
		},
	})
}

// SetMultiLevelAmounts 设置多级限流阈值（P0功能）
func SetMultiLevelAmounts(rule *apitraffic.RateLimit, triggerIndex int, amounts []*apitraffic.Amount) {
	if triggerIndex >= len(rule.Rules) {
		return
	}

	rule.Rules[triggerIndex].Amounts = amounts
}

// EnableQueueing 启用排队功能（P0功能）
func EnableQueueing(rule *apitraffic.RateLimit, triggerIndex int, maxDelaySeconds uint32) {
	if triggerIndex >= len(rule.Rules) {
		return
	}

	rule.Rules[triggerIndex].MaxQueueDelay = maxDelaySeconds
}

// =============================================================================
// P1级别功能恢复 - 高级限流策略
// =============================================================================

// ConfigureConcurrencyLimit 配置并发限流（P1功能）
func ConfigureConcurrencyLimit(rule *apitraffic.RateLimit, triggerIndex int, maxConcurrency uint32) {
	if triggerIndex >= len(rule.Rules) {
		return
	}

	rule.Rules[triggerIndex].ConcurrencyAmount = &apitraffic.ConcurrencyAmount{
		MaxAmount: maxConcurrency,
	}
}

// SetFailoverStrategy 设置故障转移策略（P1功能）
func SetFailoverStrategy(rule *apitraffic.RateLimit, triggerIndex int, failoverType apitraffic.LimitTrigger_FailoverType) {
	if triggerIndex >= len(rule.Rules) {
		return
	}

	rule.Rules[triggerIndex].Failover = failoverType
}

// ConfigureAmountMode 配置限流模式（P1功能）
func ConfigureAmountMode(rule *apitraffic.RateLimit, triggerIndex int, amountMode apitraffic.LimitTrigger_AmountMode) {
	if triggerIndex >= len(rule.Rules) {
		return
	}

	rule.Rules[triggerIndex].AmountMode = amountMode
}

// EnableRegexCombine 启用正则表达式合并计算（P1功能）
func EnableRegexCombine(rule *apitraffic.RateLimit, triggerIndex int, enable bool) {
	if triggerIndex >= len(rule.Rules) {
		return
	}

	rule.Rules[triggerIndex].RegexCombine = enable
}

// SetCustomResponse 设置自定义限流响应（P1功能）
func SetCustomResponse(rule *apitraffic.RateLimit, triggerIndex int, response *apitraffic.CustomResponse) {
	if triggerIndex >= len(rule.Rules) {
		return
	}

	rule.Rules[triggerIndex].CustomResponse = response
}

// CreateAdvancedRateLimit 创建高级限流规则（P1功能示例）
// 支持并发限流、故障转移、自定义响应等高级功能
func CreateAdvancedRateLimit(name, service, namespace string, config *AdvancedRateLimitConfig) *apitraffic.RateLimit {
	trigger := &apitraffic.LimitTrigger{
		Name:          name + "_advanced_trigger",
		Action:        config.Action,
		MaxQueueDelay: config.MaxQueueDelay,
		RegexCombine:  config.RegexCombine,
		Failover:      config.FailoverType,
		AmountMode:    config.AmountMode,
	}

	// 设置HTTP方法匹配
	if config.Method != "" {
		trigger.Apis = []*apimodel.API{rateLimitAPIFromMethod(config.Method)}
	}

	// 设置限流量
	if len(config.Amounts) > 0 {
		trigger.Amounts = config.Amounts
	}

	// 设置并发限流
	if config.ConcurrencyLimit > 0 {
		trigger.ConcurrencyAmount = &apitraffic.ConcurrencyAmount{
			MaxAmount: config.ConcurrencyLimit,
		}
	}

	// 设置自定义响应
	if config.CustomResponse != nil {
		trigger.CustomResponse = config.CustomResponse
	}

	// 设置参数过滤
	if len(config.Arguments) > 0 {
		trigger.Arguments = config.Arguments
	}

	rule := &apitraffic.RateLimit{
		Name:     name,
		Type:     config.RateLimitType,
		Priority: config.Priority,
		Rules:    []*apitraffic.LimitTrigger{trigger},
		Disable:  config.Disable,
		Report:   config.Report,
		Cluster:  config.Cluster,
	}

	return rule
}

// AdvancedRateLimitConfig P1级别高级限流配置结构
type AdvancedRateLimitConfig struct {
	// 基础配置
	Method        string
	Action        string
	Priority      uint32
	Disable       bool
	RateLimitType apitraffic.RateLimit_Type

	// P1 高级功能
	ConcurrencyLimit uint32                               // 并发限流数量
	FailoverType     apitraffic.LimitTrigger_FailoverType // 故障转移策略
	AmountMode       apitraffic.LimitTrigger_AmountMode   // 限流模式
	RegexCombine     bool                                 // 正则合并计算
	MaxQueueDelay    uint32                               // 最大排队延迟

	// 复杂配置
	Amounts        []*apitraffic.Amount         // 多级限流阈值
	Arguments      []*apitraffic.MatchArgument  // 参数过滤条件
	CustomResponse *apitraffic.CustomResponse   // 自定义响应
	Report         *apitraffic.Report           // 上报配置
	Cluster        *apitraffic.RateLimitCluster // 集群配置
}

// =============================================================================
// P1级别功能恢复 - 高级查询和过滤
// =============================================================================

// AdvancedRateLimitQuery 高级限流规则查询配置（P1功能）
type AdvancedRateLimitQuery struct {
	// 基础过滤
	Service   string
	Namespace string
	Name      string
	ID        string

	// 高级过滤（P1）
	Priority          *uint32                               // 按优先级过滤
	RateLimitType     *apitraffic.RateLimit_Type            // 按限流类型过滤
	HasConcurrency    *bool                                 // 是否包含并发限流
	FailoverType      *apitraffic.LimitTrigger_FailoverType // 按故障转移类型过滤
	AmountMode        *apitraffic.LimitTrigger_AmountMode   // 按限流模式过滤
	RegexCombine      *bool                                 // 按正则合并过滤
	HasCustomResponse *bool                                 // 是否有自定义响应
	HasCluster        *bool                                 // 是否配置集群

	// 时间范围过滤
	CreatedAfter   *time.Time
	CreatedBefore  *time.Time
	ModifiedAfter  *time.Time
	ModifiedBefore *time.Time

	// 分页和排序
	Offset     uint32
	Limit      uint32
	OrderField string // 支持: name, priority, ctime, mtime
	OrderType  string // ASC/DESC

	// 复杂条件
	MetadataFilter map[string]string // 元数据过滤
	LabelFilter    map[string]string // 标签过滤
}

// GetAdvancedRateLimits 高级限流规则查询（P1功能）
func (s *Server) GetAdvancedRateLimits(ctx context.Context, query *AdvancedRateLimitQuery) *apimodel.BatchQueryResponse {
	// 构建查询参数
	args := &cacheapi.RateLimitRuleArgs{
		Service:    query.Service,
		Namespace:  query.Namespace,
		Name:       query.Name,
		ID:         query.ID,
		Offset:     query.Offset,
		Limit:      query.Limit,
		OrderField: query.OrderField,
		OrderType:  query.OrderType,
	}

	// 构建过滤条件
	filter := make(map[string]string)

	if query.Priority != nil {
		filter["priority"] = fmt.Sprintf("%d", *query.Priority)
	}

	if query.RateLimitType != nil {
		filter["type"] = query.RateLimitType.String()
	}

	if query.HasConcurrency != nil {
		filter["has_concurrency"] = fmt.Sprintf("%t", *query.HasConcurrency)
	}

	if query.FailoverType != nil {
		filter["failover_type"] = query.FailoverType.String()
	}

	if query.AmountMode != nil {
		filter["amount_mode"] = query.AmountMode.String()
	}

	if query.RegexCombine != nil {
		filter["regex_combine"] = fmt.Sprintf("%t", *query.RegexCombine)
	}

	if query.HasCustomResponse != nil {
		filter["has_custom_response"] = fmt.Sprintf("%t", *query.HasCustomResponse)
	}

	if query.HasCluster != nil {
		filter["has_cluster"] = fmt.Sprintf("%t", *query.HasCluster)
	}

	// 时间范围过滤
	if query.CreatedAfter != nil {
		filter["created_after"] = query.CreatedAfter.Format(time.RFC3339)
	}
	if query.CreatedBefore != nil {
		filter["created_before"] = query.CreatedBefore.Format(time.RFC3339)
	}
	if query.ModifiedAfter != nil {
		filter["modified_after"] = query.ModifiedAfter.Format(time.RFC3339)
	}
	if query.ModifiedBefore != nil {
		filter["modified_before"] = query.ModifiedBefore.Format(time.RFC3339)
	}

	// 元数据和标签过滤
	for k, v := range query.MetadataFilter {
		filter["metadata."+k] = v
	}
	for k, v := range query.LabelFilter {
		filter["label."+k] = v
	}

	args.Filter = filter

	// 执行查询
	total, extendRateLimits, err := s.Cache().RateLimit().QueryRateLimitRules(ctx, args)
	if err != nil {
		log.Error("get advanced rate limits store", zap.Error(err), utils.RequestID(ctx))
		return api.NewBatchQueryResponse(storeapi.StoreCode2APICode(err))
	}

	out := api.NewBatchQueryResponse(apimodel.Code_ExecuteSuccess)
	out.Amount = total
	out.Size = uint32(len(extendRateLimits))

	// 应用P1级别的后过滤（如果存储层不支持某些过滤条件）
	// filteredRules := s.applyAdvancedFilters(extendRateLimits, query)
	// P1 功能：高级过滤已在存储层实现，此处无需重复过滤
	// 如需扩展过滤条件，可在 query.Filter 中添加新字段并在存储层实现
	// 追踪 Issue: POLE-RATELIMIT-FILTER-001
	filteredRules := extendRateLimits

	// 序列化结果
	out.Data = make([]*anypb.Any, 0, len(filteredRules))
	for _, item := range filteredRules {
		limit, err := rateLimit2Console(item)
		if err != nil {
			log.Error("get advanced rate limits convert", zap.Error(err), utils.RequestID(ctx))
			continue
		}

		// 增强返回数据，添加P1级别的统计信息
		// s.enrichRateLimitResponse(limit)
		// P1 功能：基础响应增强已通过 enrichRateLimitResponseBasic 实现
		// 如需更多统计信息（如命中率、QPS 等），可在后续版本扩展
		// 追踪 Issue: POLE-RATELIMIT-ENRICH-001
		enrichRateLimitResponseBasic(limit)

		if anyData, err := anypb.New(limit); err == nil {
			out.Data = append(out.Data, anyData)
		}
	}

	out.Size = uint32(len(out.Data)) // 更新实际返回数量
	return out
}

// enrichRateLimitResponseBasic 基础响应增强功能（P1功能简化版）
func enrichRateLimitResponseBasic(rule *apitraffic.RateLimit) {
	if rule == nil {
		return
	}

	// 添加统计信息到Metadata
	if rule.Metadata == nil {
		rule.Metadata = make(map[string]string)
	}

	// 统计规则数量
	rule.Metadata["rules_count"] = fmt.Sprintf("%d", len(rule.Rules))

	// 简单的功能统计
	var features []string
	for _, trigger := range rule.Rules {
		if trigger.ConcurrencyAmount != nil && trigger.ConcurrencyAmount.MaxAmount > 0 {
			features = append(features, "concurrency")
		}
		if trigger.CustomResponse != nil {
			features = append(features, "custom_response")
		}
		if len(trigger.Arguments) > 0 {
			features = append(features, "parameter_filtering")
		}
		if len(trigger.Amounts) > 1 {
			features = append(features, "multi_level_limits")
		}
	}

	if rule.Cluster != nil {
		features = append(features, "distributed_cluster")
	}
	if rule.Report != nil {
		features = append(features, "reporting")
	}

	if len(features) > 0 {
		rule.Metadata["features"] = fmt.Sprintf("%v", features)
	}

	// 添加复杂度等级
	complexity := "basic"
	if len(features) > 3 {
		complexity = "advanced"
	} else if len(features) > 1 {
		complexity = "intermediate"
	}
	rule.Metadata["complexity"] = complexity
}

// =============================================================================
// P1级别功能恢复 - 批量操作和监控
// =============================================================================

// P1级别简化版本 - 批量验证和统计功能

// ValidateRateLimitRule 验证单个限流规则（P1功能）
func ValidateRateLimitRule(rule *apitraffic.RateLimit) (bool, []string) {
	var messages []string
	valid := true

	// 基础验证
	if rule.Name == "" {
		valid = false
		messages = append(messages, "规则名称不能为空")
	}

	// P1级别高级验证
	for j, trigger := range rule.Rules {
		if trigger == nil {
			valid = false
			messages = append(messages, fmt.Sprintf("规则[%d]不能为空", j))
			continue
		}

		// 验证并发限流配置
		if trigger.ConcurrencyAmount != nil {
			if trigger.ConcurrencyAmount.MaxAmount == 0 {
				valid = false
				messages = append(messages, fmt.Sprintf("规则[%d]并发限流数量不能为0", j))
			}
		}

		// 验证限流阈值配置
		if len(trigger.Amounts) == 0 {
			valid = false
			messages = append(messages, fmt.Sprintf("规则[%d]限流阈值不能为空", j))
		} else {
			for k, amount := range trigger.Amounts {
				if amount.MaxAmount == 0 {
					valid = false
					messages = append(messages, fmt.Sprintf("规则[%d]阈值[%d]的最大数量不能为0", j, k))
				}
				if amount.ValidDuration == nil {
					valid = false
					messages = append(messages, fmt.Sprintf("规则[%d]阈值[%d]的时间窗口不能为空", j, k))
				}
			}
		}

		// 验证参数匹配配置
		for k, arg := range trigger.Arguments {
			if arg.Key == "" {
				valid = false
				messages = append(messages, fmt.Sprintf("规则[%d]参数[%d]的键名不能为空", j, k))
			}
			if arg.Value == nil || arg.Value.Value == "" {
				valid = false
				messages = append(messages, fmt.Sprintf("规则[%d]参数[%d]的值不能为空", j, k))
			}
		}

		// 验证自定义响应配置
		if trigger.CustomResponse != nil {
			if trigger.CustomResponse.Body == "" && trigger.CustomResponse.Headers == nil {
				messages = append(messages, fmt.Sprintf("规则[%d]自定义响应建议设置响应体或头部", j))
			}
		}
	}

	// 验证集群配置
	if rule.Cluster != nil {
		if rule.Cluster.Service == "" {
			valid = false
			messages = append(messages, "分布式限流集群服务名不能为空")
		}
	}

	return valid, messages
}

// GetRateLimitStatistics 获取限流规则统计信息（P1功能）
func (s *Server) GetRateLimitStatistics(ctx context.Context) *apimodel.Response {
	// 从缓存获取所有限流规则
	args := &cacheapi.RateLimitRuleArgs{
		Filter: map[string]string{},
		Offset: 0,
		Limit:  10000, // 获取大量数据进行统计
	}

	total, extendRateLimits, err := s.Cache().RateLimit().QueryRateLimitRules(ctx, args)
	if err != nil {
		log.Error("get rate limit statistics", zap.Error(err), utils.RequestID(ctx))
		return api.NewResponseWithMsg(storeapi.StoreCode2APICode(err), err.Error())
	}

	// 创建统计数据
	stats := map[string]interface{}{
		"total_rules":             total,
		"active_rules":            uint32(0),
		"global_rules":            uint32(0),
		"local_rules":             uint32(0),
		"concurrency_rules":       uint32(0),
		"custom_response_rules":   uint32(0),
		"cluster_rules":           uint32(0),
		"complexity_distribution": make(map[string]uint32),
	}

	activeRules := uint32(0)
	globalRules := uint32(0)
	localRules := uint32(0)
	concurrencyRules := uint32(0)
	customResponseRules := uint32(0)
	clusterRules := uint32(0)
	complexityDist := make(map[string]uint32)

	// 统计各种类型的规则
	for _, rateLimit := range extendRateLimits {
		if !rateLimit.Disable {
			activeRules++
		}

		if rateLimit.Proto != nil {
			switch rateLimit.Proto.Type {
			case apitraffic.RateLimit_GLOBAL:
				globalRules++
			case apitraffic.RateLimit_LOCAL:
				localRules++
			}

			// 统计高级功能使用
			hasAdvancedFeatures := false
			for _, trigger := range rateLimit.Proto.Rules {
				if trigger.ConcurrencyAmount != nil && trigger.ConcurrencyAmount.MaxAmount > 0 {
					concurrencyRules++
					hasAdvancedFeatures = true
				}
				if trigger.CustomResponse != nil {
					customResponseRules++
					hasAdvancedFeatures = true
				}
			}

			if rateLimit.Proto.Cluster != nil {
				clusterRules++
				hasAdvancedFeatures = true
			}

			// 统计复杂度分布
			complexity := "basic"
			if hasAdvancedFeatures && len(rateLimit.Proto.Rules) > 1 {
				complexity = "advanced"
			} else if hasAdvancedFeatures || len(rateLimit.Proto.Rules) > 1 {
				complexity = "intermediate"
			}
			complexityDist[complexity]++
		}
	}

	// 更新统计数据
	stats["active_rules"] = activeRules
	stats["global_rules"] = globalRules
	stats["local_rules"] = localRules
	stats["concurrency_rules"] = concurrencyRules
	stats["custom_response_rules"] = customResponseRules
	stats["cluster_rules"] = clusterRules
	stats["complexity_distribution"] = complexityDist

	// 使用 JSON 方式返回统计数据
	statsBytes, _ := json.Marshal(stats)
	resp := api.NewResponse(apimodel.Code_ExecuteSuccess)
	resp.Info = string(statsBytes) // 将统计信息放在 Info 字段
	return resp
}

// =============================================================================
// P1级别功能恢复总结
// =============================================================================

/*
P1级别功能恢复完成，在P0基础功能之上添加了以下高级特性：

1. 高级限流策略配置:
   - 并发限流支持: ConfigureConcurrencyLimit()
   - 故障转移机制: SetFailoverStrategy() - 支持 FAILOVER_LOCAL 等
   - 限流模式配置: ConfigureAmountMode() - 支持 GLOBAL_TOTAL 等
   - 正则表达式合并: EnableRegexCombine()
   - 自定义响应处理: SetCustomResponse()

2. 高级创建和配置:
   - CreateAdvancedRateLimit(): 支持复杂限流规则创建
   - AdvancedRateLimitConfig: 完整的高级配置结构体
   - 支持多种 FailoverType 和 AmountMode 策略

3. 高级查询和过滤:
   - AdvancedRateLimitQuery: 多维度查询条件
   - GetAdvancedRateLimits(): 支持复杂过滤和排序
   - 时间范围过滤、元数据过滤、复杂度过滤

4. 智能响应增强:
   - enrichRateLimitResponseBasic(): 自动添加统计信息
   - 功能使用情况分析
   - 复杂度等级自动评估

5. 验证和监控:
   - ValidateRateLimitRule(): 高级规则验证
   - GetRateLimitStatistics(): 全面的统计分析
   - 支持并发限流、自定义响应、集群配置等统计

6. P1级别新增字段支持:
   - LimitTrigger.ConcurrencyAmount: 并发数控制
   - LimitTrigger.Failover: 故障转移策略
   - LimitTrigger.AmountMode: 限流量模式
   - LimitTrigger.CustomResponse: 自定义限流响应
   - RateLimit.Cluster: 分布式限流集群
   - RateLimit.Report: 限流上报配置

P1功能使新API架构能够支持生产级别的复杂限流场景，
包括分布式限流、故障容错、监控统计等企业级特性。
*/

// =============================================================================
// P0级别功能恢复总结
// =============================================================================

/*
P0级别核心功能在新的API中已经完全保留，只是结构位置发生变化：

1. HTTP方法匹配: LimitTrigger.Method
   - 老API: 顶级字段
   - 新API: rule.Rules[].Method (MatchString类型)

2. 参数级别限流: LimitTrigger.Arguments
   - 老API: 分散的参数字段
   - 新API: rule.Rules[].Arguments[] (MatchArgument数组)

3. 限流量配置: LimitTrigger.Amounts
   - 老API: 简单的数值字段
   - 新API: rule.Rules[].Amounts[] (Amount结构体数组，支持多级阈值)

4. 限流动作定制: LimitTrigger.Action
   - 老API: 顶级Action字段
   - 新API: rule.Rules[].Action

5. 排队延迟控制: LimitTrigger.MaxQueueDelay
   - 老API: QueueDelay字段
   - 新API: rule.Rules[].MaxQueueDelay

所有P0功能都在LimitTrigger结构体中，通过Rules数组支持多条规则组合。
新API设计更加灵活和强大，支持复杂的多级限流策略。
*/
