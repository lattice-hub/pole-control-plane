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
	"time"

	"github.com/golang/protobuf/ptypes"

	apimodel "github.com/pole-io/specification/source/go/api/v1/model"
	"github.com/pole-io/specification/source/go/api/v1/traffic_manage"
	apitraffic "github.com/pole-io/specification/source/go/api/v1/traffic_manage"

	api "github.com/pole-io/pole-server/pkg/common/api/v1"
	"github.com/pole-io/pole-server/pkg/common/log"
	"github.com/pole-io/pole-server/pkg/common/utils"
	"github.com/pole-io/pole-server/pkg/common/utils/valid"
)

var (
	// _allowRateLimitFilters rate limit filters
	_allowRateLimitFilters = map[string]struct{}{
		"id":         {},
		"name":       {},
		"service":    {},
		"namespace":  {},
		"brief":      {},
		"method":     {},
		"labels":     {},
		"disable":    {},
		"offset":     {},
		"limit":      {},
		"limit_type": {},
	}
)

// CreateRateLimits implements service.DiscoverServer.
func (svr *Server) CreateRateLimits(ctx context.Context,
	reqs []*apitraffic.RateLimit) *apimodel.BatchWriteResponse {
	if err := checkBatchRateLimits(reqs); err != nil {
		return err
	}

	batchRsp := api.NewBatchWriteResponse(apimodel.Code_ExecuteSuccess)
	for i := range reqs {
		// 参数校验
		if resp := checkRateLimitParams(reqs[i]); resp != nil {
			api.Collect(batchRsp, resp)
			continue
		}
		if resp := checkRateLimitRuleParams(ctx, reqs[i]); resp != nil {
			api.Collect(batchRsp, resp)
			continue
		}
	}
	if !api.IsSuccess(batchRsp) {
		return batchRsp
	}

	return svr.nextSvr.CreateRateLimits(ctx, reqs)
}

// DeleteRateLimits implements service.DiscoverServer.
func (svr *Server) DeleteRateLimits(ctx context.Context, reqs []*traffic_manage.RateLimit) *apimodel.BatchWriteResponse {
	if err := checkBatchRateLimits(reqs); err != nil {
		return err
	}
	batchRsp := api.NewBatchWriteResponse(apimodel.Code_ExecuteSuccess)
	for i := range reqs {
		// 参数校验
		resp := checkRevisedRateLimitParams(reqs[i])
		api.Collect(batchRsp, resp)
	}
	if !api.IsSuccess(batchRsp) {
		return batchRsp
	}
	return svr.nextSvr.DeleteRateLimits(ctx, reqs)
}

// GetRateLimits implements service.DiscoverServer.
func (svr *Server) GetRateLimits(ctx context.Context,
	query map[string]string) *apimodel.BatchQueryResponse {
	for key := range query {
		if _, ok := _allowRateLimitFilters[key]; !ok {
			log.Errorf("params %s is not allowed in querying rate limits", key)
			return api.NewBatchQueryResponse(apimodel.Code_InvalidParameter)
		}
	}
	// 处理offset和limit
	offset, limit, err := valid.ParseOffsetAndLimit(query)
	if err != nil {
		return api.NewBatchQueryResponse(apimodel.Code_InvalidParameter)
	}
	query["offset"] = strconv.Itoa(int(offset))
	query["limit"] = strconv.Itoa(int(limit))

	return svr.nextSvr.GetRateLimits(ctx, query)
}

func (svr *Server) GetOneRateLimitRule(ctx context.Context, req *traffic_manage.RateLimit) *apimodel.Response {
	return svr.nextSvr.GetOneRateLimitRule(ctx, req)
}

// UpdateRateLimits implements service.DiscoverServer.
func (svr *Server) UpdateRateLimits(ctx context.Context, reqs []*traffic_manage.RateLimit) *apimodel.BatchWriteResponse {
	if err := checkBatchRateLimits(reqs); err != nil {
		return err
	}
	batchRsp := api.NewBatchWriteResponse(apimodel.Code_ExecuteSuccess)
	for i := range reqs {
		// 参数校验
		if resp := checkRevisedRateLimitParams(reqs[i]); resp != nil {
			api.Collect(batchRsp, resp)
			continue
		}
		if resp := checkRateLimitRuleParams(ctx, reqs[i]); resp != nil {
			api.Collect(batchRsp, resp)
			continue
		}
		if resp := checkRateLimitParamsDbLen(reqs[i]); resp != nil {
			api.Collect(batchRsp, resp)
			continue
		}
	}
	if !api.IsSuccess(batchRsp) {
		return batchRsp
	}

	return svr.nextSvr.UpdateRateLimits(ctx, reqs)
}

// checkBatchRateLimits 检查批量请求的限流规则
func checkBatchRateLimits(req []*apitraffic.RateLimit) *apimodel.BatchWriteResponse {
	if len(req) == 0 {
		return api.NewBatchWriteResponse(apimodel.Code_EmptyRequest)
	}

	if len(req) > valid.MaxBatchSize {
		return api.NewBatchWriteResponse(apimodel.Code_BatchSizeOverLimit)
	}

	return nil
}

// checkRateLimitParams 检查限流规则基础参数
func checkRateLimitParams(req *apitraffic.RateLimit) *apimodel.Response {
	if req == nil {
		return api.NewRateLimitResponse(apimodel.Code_EmptyRequest, req)
	}
	if err := valid.CheckResourceName(req.GetName()); err != nil {
		return api.NewRateLimitResponse(apimodel.Code_InvalidParameter, req)
	}
	if req.GetNamespace() == "" || req.GetCallee().GetNamespace() == "" || req.GetCallee().GetService() == "" {
		return api.NewRateLimitResponse(apimodel.Code_InvalidParameter, req)
	}
	if resp := checkRateLimitParamsDbLen(req); nil != resp {
		return resp
	}
	return nil
}

// checkRateLimitParams 检查限流规则基础参数
func checkRateLimitParamsDbLen(req *apitraffic.RateLimit) *apimodel.Response {
	if err := valid.CheckDbStrFieldLen(req.GetName(), valid.MaxDbRateLimitName); err != nil {
		return api.NewRateLimitResponse(apimodel.Code_InvalidParameter, req)
	}
	return nil
}

// checkRateLimitRuleParams 检查限流规则其他参数
func checkRateLimitRuleParams(ctx context.Context, req *apitraffic.RateLimit) *apimodel.Response {
	// 检查rules中的amounts是否有重复周期
	rules := req.GetRules()
	for _, rule := range rules {
		amounts := rule.GetAmounts()
		durations := make(map[time.Duration]bool)
		for _, amount := range amounts {
			d := amount.GetValidDuration()
			duration, err := ptypes.Duration(d)
			if err != nil {
				log.Error(err.Error(), utils.RequestID(ctx))
				return api.NewRateLimitResponse(apimodel.Code_InvalidParameter, req)
			}
			durations[duration] = true
		}
		if len(amounts) != len(durations) {
			return api.NewRateLimitResponse(apimodel.Code_InvalidParameter, req)
		}
	}
	return nil
}

// checkRevisedRateLimitParams 检查修改/删除限流规则基础参数
func checkRevisedRateLimitParams(req *apitraffic.RateLimit) *apimodel.Response {
	if len(req.GetId()) == 0 {
		log.Error("[RateLimit] revision is empty")
		return api.NewRateLimitResponse(apimodel.Code_InvalidParameter, req)
	}

	return nil
}
