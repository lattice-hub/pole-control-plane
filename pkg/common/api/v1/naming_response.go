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

package v1

import (
	"github.com/golang/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	apifault "github.com/pole-io/specification/source/go/api/v1/fault_tolerance"
	apimodel "github.com/pole-io/specification/source/go/api/v1/model"
	apiservice "github.com/pole-io/specification/source/go/api/v1/service_manage"
	apitraffic "github.com/pole-io/specification/source/go/api/v1/traffic_manage"

	"github.com/pole-io/pole-server/apis/pkg/types"
	"github.com/pole-io/pole-server/apis/pkg/types/protobuf"
)

type Rsp interface {
	GetCode() uint32
	GetInfo() string
}

/**
 * @brief 回复消息接口
 */
type ResponseMessage interface {
	proto.Message
	GetCode() uint32
	GetInfo() string
}

type ResponseMessageV2 interface {
	proto.Message
	GetCode() uint32
	GetInfo() string
}

/**
 * @brief 获取返回码前三位
 * @note 返回码前三位和HTTP返回码定义一致
 */
func CalcCode(rm ResponseMessage) int {
	return int(rm.GetCode() / 1000)
}

/**
 * @brief 获取返回码前三位
 * @note 返回码前三位和HTTP返回码定义一致
 */
func CalcCodeV2(rm ResponseMessageV2) int {
	return int(rm.GetCode() / 1000)
}

// IsSuccess .
func IsSuccessCommon(rsp *types.CommonResponse) bool {
	return rsp.Code == uint32(apimodel.Code_ExecuteSuccess)
}

/**
 * @brief 获取返回码前三位
 * @note 返回码前三位和HTTP返回码定义一致
 */
func CalcCodeCommon(rm Rsp) int {
	return int(rm.GetCode() / 1000)
}

// IsSuccess .
func IsSuccess(rsp ResponseMessage) bool {
	if rsp == nil {
		return true
	}
	return rsp.GetCode() == uint32(apimodel.Code_ExecuteSuccess)
}

/**
 * @brief BatchWriteResponse添加Response
 */
func Collect(batchWriteResponse *apimodel.BatchWriteResponse, response *apimodel.Response) {
	// 非200的code，都归为异常
	if CalcCode(response) != 200 {
		if response.GetCode() >= batchWriteResponse.GetCode() {
			batchWriteResponse.Code = response.GetCode()
			batchWriteResponse.Info = code2info[batchWriteResponse.GetCode()]
		}
	}

	batchWriteResponse.Size++
	batchWriteResponse.Responses = append(batchWriteResponse.Responses, response)
}

/**
 * @brief BatchWriteResponse添加Response
 */
func QueryCollect(resp *apimodel.BatchQueryResponse, response *apimodel.Response) {
	// 非200的code，都归为异常
	if CalcCode(response) != 200 {
		if response.GetCode() >= resp.GetCode() {
			resp.Code = response.GetCode()
			resp.Info = code2info[resp.GetCode()]
		}
	}
}

// AddNamespace BatchQueryResponse添加命名空间
func AddNamespace(b *apimodel.BatchQueryResponse, namespace *apimodel.Namespace) error {
	if b == nil || namespace == nil {
		return nil
	}
	data, err := anypb.New(proto.MessageV2(namespace))
	if err != nil {
		return nil
	}
	b.Data = append(b.Data, data)
	return nil
}

// NewResponse 创建回复
func NewResponse(code apimodel.Code) *apimodel.Response {
	return &apimodel.Response{
		Code: uint32(code),
		Info: code2info[uint32(code)],
	}
}

// NewResponseWithMsg 带上具体的错误信息
func NewResponseWithMsg(code apimodel.Code, msg string) *apimodel.Response {
	resp := NewResponse(code)
	resp.Info += ": " + msg
	return resp
}

/**
 * @brief 创建回复带客户端信息
 */
func NewClientResponse(code apimodel.Code, client *apiservice.Client) *apimodel.Response {
	return &apimodel.Response{
		Code: uint32(code),
		Info: code2info[uint32(code)],
		Data: protobuf.MarshalAny(client),
	}
}

/**
 * @brief 创建回复带命名空间信息
 */
func NewNamespaceResponse(code apimodel.Code, namespace *apimodel.Namespace) *apimodel.Response {
	return &apimodel.Response{
		Code: uint32(code),
		Info: code2info[uint32(code)],
		Data: protobuf.MarshalAny(namespace),
	}
}

/**
 * @brief 创建回复带服务信息
 */
func NewServiceResponse(code apimodel.Code, service *apiservice.Service) *apimodel.Response {
	return &apimodel.Response{
		Code: uint32(code),
		Info: code2info[uint32(code)],
		Data: protobuf.MarshalAny(service),
	}
}

// 创建带别名信息的答复
func NewServiceAliasResponse(code apimodel.Code, alias *apiservice.ServiceAlias) *apimodel.Response {
	resp := NewResponse(code)
	resp.Data = protobuf.MarshalAny(alias)
	return resp
}

/**
 * @brief 创建回复带服务实例信息
 */
func NewInstanceResponse(code apimodel.Code, instance *apiservice.Instance) *apimodel.Response {
	return &apimodel.Response{
		Code: uint32(code),
		Info: code2info[uint32(code)],
		Data: protobuf.MarshalAny(instance),
	}
}

// 创建带自定义error的服务实例response
func NewInstanceRespWithError(code apimodel.Code, err error, instance *apiservice.Instance) *apimodel.Response {
	resp := NewInstanceResponse(code, instance)
	resp.Info += " : " + err.Error()

	return resp
}

// NewServiceContractResponse create the response with data with any type
func NewServiceContractResponse(code apimodel.Code, contract *apiservice.ServiceContract) *apimodel.Response {
	return &apimodel.Response{
		Code: uint32(code),
		Info: code2info[uint32(code)],
		Data: protobuf.MarshalAny(contract),
	}
}

// NewAnyDataResponse create the response with data with any type
func NewAnyDataResponse(code apimodel.Code, msg proto.Message) *apimodel.Response {
	ret, err := anypb.New(proto.MessageV2(msg))
	if err != nil {
		return NewResponse(code)
	}
	return &apimodel.Response{
		Code: uint32(code),
		Info: code2info[uint32(code)],
		Data: ret,
	}
}

// NewRouterResponse 创建带新版本路由的返回
func NewRouterResponse(code apimodel.Code, router *apitraffic.RouteRule) *apimodel.Response {
	return NewAnyDataResponse(code, router)
}

// NewRateLimitResponse 创建回复带限流规则信息
func NewRateLimitResponse(code apimodel.Code, rule *apitraffic.RateLimit) *apimodel.Response {
	return &apimodel.Response{
		Code: uint32(code),
		Info: code2info[uint32(code)],
		Data: protobuf.MarshalAny(rule),
	}
}

/**
 * @brief 创建回复带熔断规则信息
 */
func NewCircuitBreakerResponse(code apimodel.Code, circuitBreaker *apifault.CircuitBreakerRule) *apimodel.Response {
	return &apimodel.Response{
		Code: uint32(code),
		Info: code2info[uint32(code)],
		Data: protobuf.MarshalAny(circuitBreaker),
	}
}

/**
 * @brief 创建批量回复
 */
func NewBatchWriteResponse(code apimodel.Code) *apimodel.BatchWriteResponse {
	return &apimodel.BatchWriteResponse{
		Code: uint32(code),
		Info: code2info[uint32(code)],
		Size: 0,
	}
}

/**
 * @brief 创建带详细信息的批量回复
 */
func NewBatchWriteResponseWithMsg(code apimodel.Code, msg string) *apimodel.BatchWriteResponse {
	resp := NewBatchWriteResponse(code)
	resp.Info += ": " + msg
	return resp
}

// NewBatchQueryResponse create the batch query responses
func NewBatchQueryResponse(code apimodel.Code) *apimodel.BatchQueryResponse {
	return &apimodel.BatchQueryResponse{
		Code:   uint32(code),
		Info:   code2info[uint32(code)],
		Amount: 0,
		Size:   0,
	}
}

// NewBatchQueryResponseWithMsg create the batch query responses with message
func NewBatchQueryResponseWithMsg(code apimodel.Code, msg string) *apimodel.BatchQueryResponse {
	resp := NewBatchQueryResponse(code)
	resp.Info += ": " + msg
	return resp
}

// AddAnyDataIntoBatchQuery add message as any data array
func AddAnyDataIntoBatchQuery(resp *apimodel.BatchQueryResponse, message proto.Message) error {
	ret, err := anypb.New(proto.MessageV2(message))
	if err != nil {
		return err
	}
	resp.Data = append(resp.Data, ret)
	return nil
}

// 创建一个空白的discoverResponse
func NewDiscoverResponse(code apimodel.Code) *apiservice.DiscoverResponse {
	return &apiservice.DiscoverResponse{
		Code: uint32(code),
		Info: code2info[uint32(code)],
	}
}

// NewDiscoverServiceIdentityResponse creates a response for the authenticated
// SDK's internal service identity. It intentionally does not carry Service.token.
func NewDiscoverServiceIdentityResponse(code apimodel.Code) *apiservice.DiscoverResponse {
	return &apiservice.DiscoverResponse{
		Code: uint32(code),
		Info: code2info[uint32(code)],
		Type: apiservice.DiscoverResponse_SERVICE_IDENTITY,
	}
}

/**
 * @brief 创建查询服务回复
 */
func NewDiscoverServiceResponse(code apimodel.Code, service *apiservice.Service) *apiservice.DiscoverResponse {
	return &apiservice.DiscoverResponse{
		Code:    uint32(code),
		Info:    code2info[uint32(code)],
		Type:    apiservice.DiscoverResponse_SERVICES,
		Service: service,
	}
}

/**
 * @brief 创建查询服务实例回复
 */
func NewDiscoverInstanceResponse(code apimodel.Code, service *apiservice.Service) *apiservice.DiscoverResponse {
	return &apiservice.DiscoverResponse{
		Code:    uint32(code),
		Info:    code2info[uint32(code)],
		Type:    apiservice.DiscoverResponse_INSTANCE,
		Service: service,
	}
}

/**
 * @brief 创建查询服务路由回复
 */
func NewDiscoverRoutingResponse(code apimodel.Code, service *apiservice.Service) *apiservice.DiscoverResponse {
	return &apiservice.DiscoverResponse{
		Code:    uint32(code),
		Info:    code2info[uint32(code)],
		Type:    apiservice.DiscoverResponse_SERVICE_CONTRACTS,
		Service: service,
	}
}

// NewDiscoverLosslessResponse create the response with data with any type
func NewDiscoverLosslessResponse(code apimodel.Code, service *apiservice.Service) *apiservice.DiscoverResponse {
	return &apiservice.DiscoverResponse{
		Code:    uint32(code),
		Info:    code2info[uint32(code)],
		Type:    apiservice.DiscoverResponse_LOSSLESS,
		Service: service,
	}
}

func NewDiscoverTrafficSecurityResponse(code apimodel.Code, service *apiservice.Service) *apiservice.DiscoverResponse {
	return &apiservice.DiscoverResponse{
		Code:    uint32(code),
		Info:    code2info[uint32(code)],
		Type:    apiservice.DiscoverResponse_TRAFFIC_SECURITY_RULE,
		Service: service,
	}
}

func NewDiscoverTrafficMirrorResponse(code apimodel.Code, service *apiservice.Service) *apiservice.DiscoverResponse {
	return &apiservice.DiscoverResponse{
		Code:    uint32(code),
		Info:    code2info[uint32(code)],
		Type:    apiservice.DiscoverResponse_TRAFFIC_MIRROR_RULE,
		Service: service,
	}
}

func NewDiscoverTrafficMockResponse(code apimodel.Code, service *apiservice.Service) *apiservice.DiscoverResponse {
	return &apiservice.DiscoverResponse{
		Code:    uint32(code),
		Info:    code2info[uint32(code)],
		Type:    apiservice.DiscoverResponse_TRAFFIC_MOCK_RULE,
		Service: service,
	}
}

/**
 * @brief 创建查询限流规则回复
 */
func NewDiscoverRateLimitResponse(code apimodel.Code, service *apiservice.Service) *apiservice.DiscoverResponse {
	return &apiservice.DiscoverResponse{
		Code:    uint32(code),
		Info:    code2info[uint32(code)],
		Type:    apiservice.DiscoverResponse_RATE_LIMIT,
		Service: service,
	}
}

/**
 * @brief 创建查询熔断规则回复
 */
func NewDiscoverCircuitBreakerResponse(code apimodel.Code, service *apiservice.Service) *apiservice.DiscoverResponse {
	return &apiservice.DiscoverResponse{
		Code:    uint32(code),
		Info:    code2info[uint32(code)],
		Type:    apiservice.DiscoverResponse_CIRCUIT_BREAKER,
		Service: service,
	}
}

// NewDiscoverLaneResponse .
func NewDiscoverLaneResponse(code apimodel.Code, service *apiservice.Service) *apiservice.DiscoverResponse {
	return &apiservice.DiscoverResponse{
		Code:    uint32(code),
		Info:    code2info[uint32(code)],
		Type:    apiservice.DiscoverResponse_LANE,
		Service: service,
	}
}

/**
 * @brief 创建查询探测规则回复
 */
func NewDiscoverFaultDetectorResponse(code apimodel.Code, service *apiservice.Service) *apiservice.DiscoverResponse {
	return &apiservice.DiscoverResponse{
		Code:    uint32(code),
		Info:    code2info[uint32(code)],
		Type:    apiservice.DiscoverResponse_FAULT_DETECTOR,
		Service: service,
	}
}

// 格式化responses
// batch操作
// 如果所有子错误码一致，那么使用子错误码
// 如果包含任意一个5xx，那么返回500
func FormatBatchWriteResponse(response *apimodel.BatchWriteResponse) *apimodel.BatchWriteResponse {
	var code uint32
	for _, resp := range response.Responses {
		if code == 0 {
			code = resp.GetCode()
			continue
		}
		if code == resp.GetCode() {
			continue
		}
		// 发现不一样
		code = 0
		break
	}
	// code不等于0，意味着所有的resp都是一样的错误码，则合并为同一个错误码
	if code != 0 {
		response.Code = code
		response.Info = code2info[code]
		return response
	}

	// 错误都不一致
	// 存在5XX，则返回500
	// 不存在5XX，但是存在4XX，则返回4XX
	// 除去以上两个情况，不修改返回值
	hasBadRequest := false
	for _, resp := range response.Responses {
		httpStatus := CalcCode(resp)
		if httpStatus >= 500 {
			response.Code = ExecuteException
			response.Info = code2info[response.Code]
			return response
		} else if httpStatus >= 400 {
			hasBadRequest = true
		}
	}

	if hasBadRequest {
		response.Code = BadRequest
		response.Info = code2info[response.Code]
	}
	return response
}
