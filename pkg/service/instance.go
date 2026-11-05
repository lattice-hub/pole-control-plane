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

package service

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	apimodel "github.com/pole-io/specification/source/go/api/v1/model"
	apiservice "github.com/pole-io/specification/source/go/api/v1/service_manage"

	"github.com/pole-io/pole-server/apis/cmdb"
	"github.com/pole-io/pole-server/apis/pkg/types"
	svctypes "github.com/pole-io/pole-server/apis/pkg/types/service"
	"github.com/pole-io/pole-server/apis/service/healthcheck"
	storeapi "github.com/pole-io/pole-server/apis/store"
	api "github.com/pole-io/pole-server/pkg/common/api/v1"
	"github.com/pole-io/pole-server/pkg/common/batchctrl"
	"github.com/pole-io/pole-server/pkg/common/eventhub"
	"github.com/pole-io/pole-server/pkg/common/utils"
	commontime "github.com/pole-io/pole-server/pkg/common/utils/time"
	"github.com/pole-io/pole-server/pkg/common/utils/valid"
)

var (
	// InstanceFilterAttributes 查询实例支持的过滤字段
	InstanceFilterAttributes = map[string]bool{
		"id":            true, // 实例ID
		"service":       true, // 服务name
		"namespace":     true, // 服务namespace
		"host":          true,
		"port":          true,
		"keys":          true,
		"values":        true,
		"protocol":      true,
		"version":       true,
		"health_status": true,
		"healthy":       true, // health_status, healthy都有，以healthy为准
		"isolate":       true,
		"weight":        true,
		"logic_set":     true,
		"cmdb_region":   true,
		"cmdb_zone":     true,
		"cmdb_idc":      true,
		"priority":      true,
		"offset":        true,
		"limit":         true,
	}
	// InsFilter2toreAttr 查询字段转为存储层的属性值，映射表
	InsFilter2toreAttr = map[string]string{
		"service": "name",
		"healthy": "health_status",
	}
	// NotInsFilterAttr 不属于 instance 表属性的字段
	NotInsFilterAttr = map[string]bool{
		"keys":   true,
		"values": true,
	}
)

// CreateInstances 批量创建服务实例
func (s *Server) CreateInstances(ctx context.Context, reqs []*apiservice.Instance) *apimodel.BatchWriteResponse {
	return batchOperateInstances(ctx, reqs, s.CreateInstance)
}

// CreateInstance create a single service instance
func (s *Server) CreateInstance(ctx context.Context, req *apiservice.Instance) *apimodel.Response {
	start := time.Now()

	// 注释：实例创建改动 - 由于API规范变更，字段从指针类型改为基础类型，需要重新构造实例而非简单拷贝
	// Create a new instance instead of copying to avoid lock value issues
	ins := &apiservice.Instance{
		Id:                req.GetId(),
		Service:           req.GetService(),
		Namespace:         req.GetNamespace(),
		Host:              req.GetHost(),
		Port:              req.GetPort(),
		Protocol:          req.GetProtocol(),
		Version:           req.GetVersion(),
		Priority:          req.GetPriority(),
		Weight:            req.GetWeight(),
		EnableHealthCheck: req.GetEnableHealthCheck(),
		HealthCheck:       req.GetHealthCheck(),
		Healthy:           req.GetHealthy(),
		Isolate:           req.GetIsolate(),
		Location:          req.GetLocation(),
		Metadata:          req.GetMetadata(),
	}
	data, resp := s.createInstance(ctx, req, ins)
	if resp != nil {
		return resp
	}

	msg := fmt.Sprintf("create instance: id=%v, namespace=%v, service=%v, host=%v, port=%v",
		ins.GetId(), req.GetNamespace(), req.GetService(),
		req.GetHost(), req.GetPort())
	log.Info(msg, utils.RequestID(ctx), zap.Duration("cost", time.Since(start)))
	svc := &svctypes.Service{
		Name:      req.GetService(),
		Namespace: req.GetNamespace(),
	}
	instanceProto := data.Proto
	event := &svctypes.InstanceEvent{
		Id:         req.GetId(),
		Namespace:  svc.Namespace,
		Service:    svc.Name,
		Instance:   instanceProto,
		EType:      svctypes.EventInstanceOnline,
		CreateTime: time.Now(),
	}
	event.InjectMetadata(ctx)
	s.sendDiscoverEvent(event)
	s.RecordHistory(ctx, instanceRecordEntry(ctx, req, svc, data, types.OCreate))
	out := &apiservice.Instance{
		Id:        ins.GetId(),
		Service:   svc.Name,
		Namespace: svc.Namespace,
		Host:      instanceProto.GetHost(),
		Port:      instanceProto.GetPort(),
	}
	return api.NewInstanceResponse(apimodel.Code_ExecuteSuccess, out)
}

// createInstance store operate
func (s *Server) createInstance(ctx context.Context, req *apiservice.Instance, ins *apiservice.Instance) (
	*svctypes.Instance, *apimodel.Response) {
	// create service if absent
	svcId, errResp := s.createWrapServiceIfAbsent(ctx, req)
	if errResp != nil {
		log.Errorf("[Instance] create service if absent fail : %+v, req : %+v", errResp.String(), req)
		return nil, errResp
	}
	if len(svcId) == 0 {
		log.Errorf("[Instance] create service if absent return service id is empty : %+v", req)
		return nil, api.NewResponseWithMsg(apimodel.Code_BadRequest, "service id is empty")
	}

	// fill instance location info
	s.packCmdb(ins)

	if namingServer.bc == nil || !namingServer.bc.CreateInstanceOpen() {
		return s.serialCreateInstance(ctx, svcId, req, ins) // 单个同步
	}
	return s.asyncCreateInstance(ctx, svcId, req, ins) // 批量异步
}

// asyncCreateInstance 异步新建实例
// 底层函数会合并create请求，增加并发创建的吞吐
// req 原始请求
// ins 包含了req数据与instanceID，serviceToken
func (s *Server) asyncCreateInstance(
	ctx context.Context, svcId string, req *apiservice.Instance, ins *apiservice.Instance) (
	*svctypes.Instance, *apimodel.Response) {
	allowAsyncRegis, _ := ctx.Value(types.ContextOpenAsyncRegis).(bool)
	future := s.bc.AsyncCreateInstance(svcId, ins, !allowAsyncRegis)

	rsp, err := future.Done()
	if err != nil {
		if rsp.(apimodel.Code) == apimodel.Code_ExistedResource {
			req.Id = ins.GetId()
		}
		return nil, api.NewInstanceResponse(rsp.(apimodel.Code), req)
	}

	return svctypes.CreateInstanceModel(svcId, req), nil
}

// 同步串行创建实例
// req为原始的请求体
// ins包括了req的内容，并且填充了instanceID与serviceToken
func (s *Server) serialCreateInstance(
	ctx context.Context, svcId string, req *apiservice.Instance, ins *apiservice.Instance) (
	*svctypes.Instance, *apimodel.Response) {

	instance, err := s.storage.GetInstance(ins.GetId())
	if err != nil {
		log.Error("[Instance] get instance from store",
			utils.RequestID(ctx), zap.Error(err))
		return nil, api.NewInstanceResponse(storeapi.StoreCode2APICode(err), req)
	}
	// 如果存在，则替换实例的属性数据，但是需要保留用户设置的隔离状态，以免出现关键状态丢失
	// 注释：隔离状态检查改动 - Isolate从*wrapperspb.BoolValue改为bool，直接比较而非检查nil
	if instance != nil && !ins.Isolate {
		ins.Isolate = instance.Proto.Isolate
	}
	// 直接同步创建服务实例
	data := svctypes.CreateInstanceModel(svcId, ins)
	if err := s.storage.AddInstance(data); err != nil {
		log.Error(err.Error(), utils.RequestID(ctx))
		return nil, wrapperInstanceStoreResponse(req, err)
	}

	return data, nil
}

// DeleteInstances 批量删除服务实例
func (s *Server) DeleteInstances(ctx context.Context, req []*apiservice.Instance) *apimodel.BatchWriteResponse {
	return batchOperateInstances(ctx, req, s.DeleteInstance)
}

// DeleteInstance 删除单个服务实例
func (s *Server) DeleteInstance(ctx context.Context, req *apiservice.Instance) *apimodel.Response {
	return s.deleteInstance(ctx, req, cloneInstanceRequest(req))
}

// ShutdownDeregisterInstance synchronously removes a server-owned instance during
// shutdown. It intentionally bypasses batch controllers because their parent
// context may already have been cancelled by the process supervisor.
func (s *Server) ShutdownDeregisterInstance(ctx context.Context, req *apiservice.Instance) *apimodel.Response {
	return s.serialDeleteInstance(ctx, req, cloneInstanceRequest(req))
}

func cloneInstanceRequest(req *apiservice.Instance) *apiservice.Instance {
	return &apiservice.Instance{
		Id:                req.GetId(),
		Service:           req.GetService(),
		Namespace:         req.GetNamespace(),
		Host:              req.GetHost(),
		Port:              req.GetPort(),
		Protocol:          req.GetProtocol(),
		Version:           req.GetVersion(),
		Priority:          req.GetPriority(),
		Weight:            req.GetWeight(),
		EnableHealthCheck: req.GetEnableHealthCheck(),
		HealthCheck:       req.GetHealthCheck(),
		Healthy:           req.GetHealthy(),
		Isolate:           req.GetIsolate(),
		Location:          req.GetLocation(),
		Metadata:          req.GetMetadata(),
	}
}

// 删除实例的store操作
// req 原始请求
// ins 填充了instanceID与serviceToken
func (s *Server) deleteInstance(
	ctx context.Context, req *apiservice.Instance, ins *apiservice.Instance) *apimodel.Response {
	if s.bc == nil || !s.bc.DeleteInstanceOpen() {
		return s.serialDeleteInstance(ctx, req, ins)
	}

	return s.asyncDeleteInstance(ctx, req, ins)
}

// 串行删除实例
// 返回实例所属的服务和resp
func (s *Server) serialDeleteInstance(
	ctx context.Context, req *apiservice.Instance, ins *apiservice.Instance) *apimodel.Response {
	start := time.Now()
	// 检查服务实例是否存在
	instance, err := s.storage.GetInstance(ins.GetId())
	if err != nil {
		log.Error(err.Error(), utils.RequestID(ctx))
		return api.NewInstanceResponse(storeapi.StoreCode2APICode(err), req)
	}
	if instance == nil {
		// 实例不存在，则返回成功
		return api.NewInstanceResponse(apimodel.Code_ExecuteSuccess, req)
	}
	// 鉴权
	service, resp := s.instanceAuth(ctx, req, instance.ServiceID)
	if resp != nil {
		return resp
	}

	// 存储层操作
	if err := s.storage.DeleteInstance(instance.ID()); err != nil {
		log.Error(err.Error(), utils.RequestID(ctx))
		return wrapperInstanceStoreResponse(req, err)
	}

	msg := fmt.Sprintf("delete instance: id=%v, namespace=%v, service=%v, host=%v, port=%v",
		instance.ID(), service.Namespace, service.Name, instance.Host(), instance.Port())
	log.Info(msg, utils.RequestID(ctx), zap.Duration("cost", time.Since(start)))
	s.RecordHistory(ctx, instanceRecordEntry(ctx, req, service, instance, types.ODelete))
	event := &svctypes.InstanceEvent{
		Id:         instance.ID(),
		Namespace:  service.Namespace,
		Service:    service.Name,
		Instance:   instance.Proto,
		EType:      svctypes.EventInstanceOffline,
		CreateTime: time.Now(),
	}
	event.InjectMetadata(ctx)
	s.sendDiscoverEvent(event)

	return api.NewInstanceResponse(apimodel.Code_ExecuteSuccess, req)
}

// 异步删除实例
// 返回实例所属的服务和resp
func (s *Server) asyncDeleteInstance(
	ctx context.Context, req *apiservice.Instance, ins *apiservice.Instance) *apimodel.Response {
	start := time.Now()
	allowAsyncRegis, _ := ctx.Value(types.ContextOpenAsyncRegis).(bool)
	future := s.bc.AsyncDeleteInstance(ins, !allowAsyncRegis)
	rsp, err := future.Done()
	if err != nil {
		if errors.Is(err, batchctrl.ErrorBatchControllerStopped) {
			return s.serialDeleteInstance(ctx, req, ins)
		}
		code, ok := rsp.(apimodel.Code)
		if !ok {
			log.Error(err.Error(), utils.RequestID(ctx))
			return api.NewInstanceResponse(apimodel.Code_ExecuteException, req)
		}
		// 如果发现不存在资源，意味着实例已经被删除，直接返回成功
		if code == apimodel.Code_NotFoundResource {
			return api.NewInstanceResponse(apimodel.Code_ExecuteSuccess, req)
		}
		log.Error(err.Error(), utils.RequestID(ctx))
		return api.NewInstanceResponse(code, req)
	}
	instance := future.Detach("instance_val").(*svctypes.Instance)

	// 打印本地日志与操作记录
	msg := fmt.Sprintf("delete instance: id=%v, namespace=%v, service=%v, host=%v, port=%v",
		instance.ID(), instance.Namespace(), instance.Service(), instance.Host(), instance.Port())
	log.Info(msg, utils.RequestID(ctx), zap.Duration("cost", time.Since(start)))
	service := &svctypes.Service{Name: instance.Service(), Namespace: instance.Namespace()}
	s.RecordHistory(ctx, instanceRecordEntry(ctx, req, service, instance, types.ODelete))
	event := &svctypes.InstanceEvent{
		Id:         instance.ID(),
		Namespace:  service.Namespace,
		Service:    service.Name,
		Instance:   instance.Proto,
		EType:      svctypes.EventInstanceOffline,
		CreateTime: time.Now(),
	}
	event.InjectMetadata(ctx)
	s.sendDiscoverEvent(event)

	return api.NewInstanceResponse(apimodel.Code_ExecuteSuccess, req)
}

// DeleteInstancesByHost 根据host批量删除服务实例
func (s *Server) DeleteInstancesByHost(
	ctx context.Context, req []*apiservice.Instance) *apimodel.BatchWriteResponse {
	return batchOperateInstances(ctx, req, s.DeleteInstanceByHost)
}

// DeleteInstanceByHost 根据host删除服务实例
func (s *Server) DeleteInstanceByHost(ctx context.Context, req *apiservice.Instance) *apimodel.Response {
	// 获取实例
	instances, service, err := s.getInstancesMainByService(ctx, req)
	if err != nil {
		return err
	}

	if instances == nil {
		return api.NewInstanceResponse(apimodel.Code_ExecuteSuccess, req)
	}

	ids := make([]interface{}, 0, len(instances))
	for _, instance := range instances {
		ids = append(ids, instance.ID())
	}

	if err := s.storage.BatchDeleteInstances(ids); err != nil {
		log.Error(err.Error(), utils.RequestID(ctx))
		return wrapperInstanceStoreResponse(req, err)
	}

	for _, instance := range instances {
		msg := fmt.Sprintf("delete instance: id=%v, namespace=%v, service=%v, host=%v, port=%v",
			instance.ID(), service.Namespace, service.Name, instance.Host(), instance.Port())
		log.Info(msg, utils.RequestID(ctx))
		s.RecordHistory(ctx, instanceRecordEntry(ctx, req, service, instance, types.ODelete))
		s.sendDiscoverEvent(&svctypes.InstanceEvent{
			Id:         instance.ID(),
			Namespace:  service.Namespace,
			Service:    service.Name,
			Instance:   instance.Proto,
			EType:      svctypes.EventInstanceOffline,
			CreateTime: time.Now(),
		})
	}
	return api.NewInstanceResponse(apimodel.Code_ExecuteSuccess, req)
}

// UpdateInstances 批量修改服务实例
func (s *Server) UpdateInstances(ctx context.Context, req []*apiservice.Instance) *apimodel.BatchWriteResponse {
	return batchOperateInstances(ctx, req, s.UpdateInstance)
}

// UpdateInstance 修改单个服务实例
func (s *Server) UpdateInstance(ctx context.Context, req *apiservice.Instance) *apimodel.Response {
	service, instance, preErr := s.execInstancePreStep(ctx, req)
	if preErr != nil {
		return preErr
	}
	// 修改
	log.Info(fmt.Sprintf("old instance: %+v", instance), utils.RequestID(ctx))

	var eventTypes map[svctypes.InstanceEventType]bool
	var needUpdate bool
	// 存储层操作
	if needUpdate, eventTypes = s.updateInstanceAttribute(req, instance); !needUpdate {
		log.Info("update instance no data change, no need update",
			utils.RequestID(ctx), zap.String("instance", req.String()))
		return api.NewInstanceResponse(apimodel.Code_NoNeedUpdate, req)
	}
	if err := s.storage.UpdateInstance(instance); err != nil {
		log.Error(err.Error(), utils.RequestID(ctx))
		return wrapperInstanceStoreResponse(req, err)
	}

	msg := fmt.Sprintf("update instance: id=%v, namespace=%v, service=%v, host=%v, port=%v, healthy = %v",
		instance.ID(), service.Namespace, service.Name, instance.Host(),
		instance.Port(), instance.Healthy())
	log.Info(msg, utils.RequestID(ctx))
	s.RecordHistory(ctx, instanceRecordEntry(ctx, req, service, instance, types.OUpdate))

	for eventType := range eventTypes {
		event := &svctypes.InstanceEvent{
			Id:         instance.ID(),
			Namespace:  service.Namespace,
			Service:    service.Name,
			Instance:   instance.Proto,
			EType:      eventType,
			CreateTime: time.Now(),
		}
		event.InjectMetadata(ctx)
		s.sendDiscoverEvent(event)
	}

	for i := range s.instanceChains {
		s.instanceChains[i].AfterUpdate(ctx, instance)
	}

	return api.NewInstanceResponse(apimodel.Code_ExecuteSuccess, req)
}

// UpdateInstancesIsolate 批量修改服务实例隔离状态
// @note 必填参数为service+namespace+host
func (s *Server) UpdateInstancesIsolate(
	ctx context.Context, req []*apiservice.Instance) *apimodel.BatchWriteResponse {
	return batchOperateInstances(ctx, req, s.UpdateInstanceIsolate)
}

// UpdateInstanceIsolate 修改服务实例隔离状态
// @note 必填参数为service+namespace+ip
func (s *Server) UpdateInstanceIsolate(ctx context.Context, req *apiservice.Instance) *apimodel.Response {
	// 注释：隔离状态验证改动 - isolate field is now a bool value, no need to check for nil
	// isolate field is now a bool value, no need to check for nil

	// 获取实例
	instances, service, err := s.getInstancesMainByService(ctx, req)
	if err != nil {
		return err
	}
	if instances == nil {
		return api.NewInstanceResponse(apimodel.Code_NotFoundResource, req)
	}

	// 判断是否需要更新
	needUpdate := false
	for _, instance := range instances {
		// 注释：隔离状态比较改动 - 直接比较bool值而非通过wrapper类型
		if instance.Isolate() != req.GetIsolate() {
			needUpdate = true
			break
		}
	}
	if !needUpdate {
		return api.NewInstanceResponse(apimodel.Code_NoNeedUpdate, req)
	}

	isolate := 0
	if req.GetIsolate() {
		isolate = 1
	}

	ids := make([]interface{}, 0, len(instances))
	for _, instance := range instances {
		// 方便后续打印操作记录
		instance.Proto.Isolate = req.GetIsolate()
		ids = append(ids, instance.ID())
	}

	if err := s.storage.BatchSetInstanceIsolate(ids, isolate, utils.NewUUID()); err != nil {
		log.Error(err.Error(), utils.RequestID(ctx))
		return wrapperInstanceStoreResponse(req, err)
	}

	for _, instance := range instances {
		msg := fmt.Sprintf("update instance: id=%v, namespace=%v, service=%v, host=%v, port=%v, isolate=%v",
			instance.ID(), service.Namespace, service.Name, instance.Host(), instance.Port(), instance.Isolate())
		log.Info(msg, utils.RequestID(ctx))
		s.RecordHistory(ctx, instanceRecordEntry(ctx, req, service, instance, types.OUpdateIsolate))

		// 比对下更新前后的 isolate 状态
		// 注释：隔离状态更新逻辑改动 - 使用bool值直接比较和赋值，业务逻辑保持不变
		if instance.Isolate() != req.GetIsolate() {
			eventType := svctypes.EventInstanceCloseIsolate
			if req.GetIsolate() {
				eventType = svctypes.EventInstanceOpenIsolate
			}
			s.sendDiscoverEvent(&svctypes.InstanceEvent{
				Id:         instance.ID(),
				Namespace:  req.Namespace,
				Service:    req.Service,
				Instance:   instance.Proto,
				EType:      eventType,
				CreateTime: time.Now(),
			})
		}
		instance.Proto.Isolate = req.GetIsolate()
	}
	for i := range s.instanceChains {
		s.instanceChains[i].AfterUpdate(ctx, instances...)
	}

	return api.NewInstanceResponse(apimodel.Code_ExecuteSuccess, req)
}

/**
 * @brief 根据服务和host获取服务实例
 */
func (s *Server) getInstancesMainByService(ctx context.Context, req *apiservice.Instance) (
	[]*svctypes.Instance, *svctypes.Service, *apimodel.Response) {
	// 检查服务
	// 这里获取的是源服务的token。如果是别名,service=nil
	service, err := s.storage.GetSourceServiceToken(req.GetService(), req.GetNamespace())
	if err != nil {
		log.Error(err.Error(), utils.RequestID(ctx))
		return nil, nil, api.NewInstanceResponse(storeapi.StoreCode2APICode(err), req)
	}
	if service == nil {
		return nil, nil, api.NewInstanceResponse(apimodel.Code_NotFoundResource, req)
	}

	// 获取服务实例
	instances, err := s.storage.GetInstancesMainByService(service.ID, req.GetHost())
	if err != nil {
		log.Error(err.Error(), utils.RequestID(ctx))
		return nil, nil, api.NewInstanceResponse(storeapi.StoreCode2APICode(err), req)
	}
	return instances, service, nil
}

/**
 * @brief 修改服务属性
 */
func (s *Server) updateInstanceAttribute(
	req *apiservice.Instance, instance *svctypes.Instance) (bool, map[svctypes.InstanceEventType]bool) {
	// #lizard forgives
	instance.MallocProto()
	needUpdate := false
	insProto := instance.Proto
	var updateEvents = make(map[svctypes.InstanceEventType]bool)
	if !maps.Equal(req.GetMetadata(), instance.Metadata()) {
		insProto.Metadata = req.GetMetadata()
		needUpdate = true
		updateEvents[svctypes.EventInstanceUpdate] = true
	}

	if ok := instanceLocationNeedUpdate(req.GetLocation(), instance.Proto.GetLocation()); ok {
		insProto.Location = req.Location
		needUpdate = true
		updateEvents[svctypes.EventInstanceUpdate] = true
	}

	if req.GetProtocol() != "" && req.GetProtocol() != instance.Protocol() {
		insProto.Protocol = req.GetProtocol()
		needUpdate = true
		updateEvents[svctypes.EventInstanceUpdate] = true
	}

	// 注释：版本字段检查改动 - GetVersion()返回string而非*wrapperspb.StringValue
	if req.GetVersion() != "" && req.GetVersion() != instance.Version() {
		insProto.Version = req.GetVersion()
		needUpdate = true
		updateEvents[svctypes.EventInstanceUpdate] = true
	}

	// 注释：优先级和权重字段检查改动 - For uint32 fields, 0 means "not set" or use existing value
	// Note: For uint32 fields, 0 means "not set" or use existing value
	if req.GetPriority() != 0 && req.GetPriority() != instance.Priority() {
		insProto.Priority = req.GetPriority()
		needUpdate = true
		updateEvents[svctypes.EventInstanceUpdate] = true
	}

	if req.GetWeight() != 0 && req.GetWeight() != instance.Weight() {
		insProto.Weight = req.GetWeight()
		needUpdate = true
		updateEvents[svctypes.EventInstanceUpdate] = true
	}

	// 注释：健康状态字段检查改动 - For boolean fields, we always check if the value is different from current instance
	// For boolean fields, we always check if the value is different from current instance
	if req.GetHealthy() != instance.Healthy() {
		insProto.Healthy = req.GetHealthy()
		needUpdate = true
		if req.GetHealthy() {
			updateEvents[svctypes.EventInstanceTurnHealth] = true
		} else {
			updateEvents[svctypes.EventInstanceTurnUnHealth] = true
		}
	}

	if req.GetIsolate() != instance.Isolate() {
		insProto.Isolate = req.GetIsolate()
		needUpdate = true
		if req.GetIsolate() {
			updateEvents[svctypes.EventInstanceOpenIsolate] = true
		} else {
			updateEvents[svctypes.EventInstanceCloseIsolate] = true
		}
	}

	// 注释：LogicSet字段移除 - LogicSet field has been removed from the specification
	// LogicSet field has been removed from the specification

	if ok := updateHealthCheck(req, instance); ok {
		needUpdate = true
		updateEvents[svctypes.EventInstanceUpdate] = true
	}

	// 每次更改，都要生成一个新的uuid
	if needUpdate {
		// 注释：版本号更新改动 - Revision字段直接赋值string而非*wrapperspb.StringValue
		insProto.Revision = string(utils.NewUUID())
	}

	return needUpdate, updateEvents
}

func instanceLocationNeedUpdate(req *apimodel.Location, old *apimodel.Location) bool {
	// 如果没有带上，则不进行更新
	if req == nil {
		return false
	}
	if req.GetRegion() != old.GetRegion() {
		return true
	}
	if req.GetZone() != old.GetZone() {
		return true
	}
	if req.GetCampus() != old.GetCampus() {
		return true
	}

	return false
}

// 健康检查的更新
func updateHealthCheck(req *apiservice.Instance, instance *svctypes.Instance) bool {
	needUpdate := false
	insProto := instance.Proto
	if req.GetEnableHealthCheck() {
		healthCheck := svctypes.NormalizeHealthCheck(req.GetHealthCheck())
		if healthCheck == nil {
			return false
		}
		// 如果数据库中实例原有是不打开健康检查，
		// 那么一旦打开，status需置为false，等待检查成功后才能变成true
		if !instance.EnableHealthCheck() {
			// 需要重置healthy，则认为有变更
			insProto.Healthy = false
			insProto.EnableHealthCheck = true
			needUpdate = true
		}

		if !proto.Equal(healthCheck, instance.HealthCheck()) {
			needUpdate = true
		}
		insProto.HealthCheck = healthCheck
		insProto.EnableHealthCheck = true
		insProto.Metadata = svctypes.MetadataWithHealthCheck(insProto.Metadata, healthCheck)
	}

	// update的时候，修改了enableHealthCheck的值
	if !req.GetEnableHealthCheck() {
		if req.GetEnableHealthCheck() != instance.EnableHealthCheck() {
			needUpdate = true
		}
		if insProto.GetHealthCheck() != nil {
			needUpdate = true
		}

		insProto.EnableHealthCheck = bool(false)
		insProto.HealthCheck = nil
		insProto.Metadata = svctypes.MetadataWithHealthCheck(insProto.Metadata, nil)
	}

	return needUpdate
}

// GetInstances 查询服务实例
func (s *Server) GetInstances(ctx context.Context, query map[string]string) *apimodel.BatchQueryResponse {
	showLastHeartbeat := query["show_last_heartbeat"] == "true"
	delete(query, "show_last_heartbeat")
	delete(query, "show_service_revision")
	// 对数据先进行提前处理一下
	filters, metaFilter, batchErr := preGetInstances(query)
	if batchErr != nil {
		return batchErr
	}
	// 分页数据
	offset, limit, _ := valid.ParseOffsetAndLimit(filters)

	total, instances, err := s.Cache().Instance().QueryInstances(filters, metaFilter, offset, limit)
	if err != nil {
		log.Errorf("[Server][Instances][Query] instances store err: %s", err.Error())
		return api.NewBatchQueryResponse(storeapi.StoreCode2APICode(err))
	}

	out := api.NewBatchQueryResponse(apimodel.Code_ExecuteSuccess)
	out.Amount = uint32(total)
	out.Size = uint32(len(instances))

	apiInstances := make([]*apiservice.Instance, 0, len(instances))
	for _, instance := range instances {
		svc, _ := s.loadServiceByID(instance.ServiceID)
		if svc == nil {
			continue
		}
		protoIns := copyOSSInstance(instance.Proto)
		protoIns.Service = svc.Name
		protoIns.Namespace = svc.Namespace
		// ServiceToken field has been removed from Instance in the specification
		s.packCmdb(protoIns)
		apiInstances = append(apiInstances, protoIns)
	}
	if showLastHeartbeat {
		s.fillLastHeartbeatTime(apiInstances)
	}
	for i := range apiInstances {
		if err := api.AddAnyDataIntoBatchQuery(out, apiInstances[i]); err != nil {
			log.Errorf("[Server][Instances][Query] add instance to response data: %s", err.Error())
			return api.NewBatchQueryResponse(apimodel.Code_ExecuteException)
		}
	}
	return out
}

func (s *Server) fillLastHeartbeatTime(instances []*apiservice.Instance) {
	checker, ok := s.healthServer.Checkers()[int32(apiservice.HealthCheck_HEARTBEAT)]
	if !ok {
		return
	}
	req := &healthcheck.BatchQueryRequest{Requests: make([]*healthcheck.QueryRequest, 0, len(instances))}
	for i := range instances {
		item := instances[i]
		req.Requests = append(req.Requests, &healthcheck.QueryRequest{
			InstanceId: item.GetId(),
		})
	}
	rsp, err := checker.BatchQuery(context.Background(), req)
	if err != nil {
		return
	}
	for i := range rsp.Responses {
		item := instances[i]
		copyMetadata := make(map[string]string, len(item.GetMetadata()))
		for k, v := range item.GetMetadata() {
			copyMetadata[k] = v
		}
		if queryRsp := rsp.Responses[i]; queryRsp.Exists {
			copyMetadata["last-heartbeat-timestamp"] = strconv.Itoa(int(queryRsp.LastHeartbeatSec))
			copyMetadata["last-heartbeat-time"] = commontime.Time2String(time.Unix(queryRsp.LastHeartbeatSec, 0))
		}
		item.Metadata = copyMetadata
	}
}

var (
	ignoreReturnOSSInstanceMetadata = map[string]struct{}{
		"version":  {},
		"protocol": {},
		"region":   {},
		"zone":     {},
		"campus":   {},
	}
)

func copyOSSInstance(instance *apiservice.Instance) *apiservice.Instance {
	copyIns := &apiservice.Instance{
		Id:                instance.Id,
		Service:           instance.Service,
		Namespace:         instance.Namespace,
		Host:              instance.Host,
		Port:              instance.Port,
		Protocol:          instance.Protocol,
		Version:           instance.Version,
		Priority:          instance.Priority,
		Weight:            instance.Weight,
		EnableHealthCheck: instance.EnableHealthCheck,
		HealthCheck:       instance.HealthCheck,
		Healthy:           instance.Healthy,
		Isolate:           instance.Isolate,
		Location:          instance.Location,
		Ctime:             instance.Ctime,
		Mtime:             instance.Mtime,
		Revision:          instance.Revision,
	}

	copym := map[string]string{}
	for k, v := range instance.Metadata {
		if _, ok := ignoreReturnOSSInstanceMetadata[k]; ok {
			continue
		}
		copym[k] = v
	}

	copyIns.Metadata = copym
	return copyIns
}

// GetInstanceLabels 获取实例标签列表
func (s *Server) GetInstanceLabels(ctx context.Context, query map[string]string) *apimodel.Response {
	var (
		serviceId string
		namespace = DefaultNamespace
	)

	if val, ok := query["namespace"]; ok {
		namespace = val
	}

	if service, ok := query["service"]; ok {
		svc := s.Cache().Service().GetServiceByName(service, namespace)
		if svc != nil {
			serviceId = svc.ID
		}
	}

	if id, ok := query["service_id"]; ok {
		serviceId = id
	}

	if serviceId == "" {
		return api.NewAnyDataResponse(apimodel.Code_ExecuteSuccess, &apiservice.InstanceLabels{})
	}

	labels := s.Cache().Instance().GetInstanceLabels(serviceId)
	if labels == nil {
		labels = &apiservice.InstanceLabels{}
	}
	return api.NewAnyDataResponse(apimodel.Code_ExecuteSuccess, labels)
}

// GetInstancesCount 查询总的服务实例，不带过滤条件的
func (s *Server) GetInstancesCount(ctx context.Context) *apimodel.BatchQueryResponse {
	count, err := s.storage.GetInstancesCount()
	if err != nil {
		log.Errorf("[Server][Instance][Count] storage get err: %s", err.Error())
		return api.NewBatchQueryResponse(storeapi.StoreCode2APICode(err))
	}

	out := api.NewBatchQueryResponse(apimodel.Code_ExecuteSuccess)
	out.Amount = uint32(count)
	// 总数通过 Amount 字段返回，符合 BatchQueryResponse 约定
	return out
}

// update/delete instance前置条件
func (s *Server) execInstancePreStep(ctx context.Context, req *apiservice.Instance) (
	*svctypes.Service, *svctypes.Instance, *apimodel.Response) {
	// 检查服务实例是否存在
	instance, err := s.storage.GetInstance(req.GetId())
	if err != nil {
		log.Error("[Instance] get instance from store", utils.RequestID(ctx), utils.ZapInstanceID(req.GetId()),
			zap.Error(err))
		return nil, nil, api.NewInstanceResponse(storeapi.StoreCode2APICode(err), req)
	}
	if instance == nil {
		return nil, nil, api.NewInstanceResponse(apimodel.Code_NotFoundResource, req)
	}

	service, resp := s.instanceAuth(ctx, req, instance.ServiceID)
	if resp != nil {
		return nil, nil, resp
	}

	return service, instance, nil
}

// 实例鉴权
func (s *Server) instanceAuth(ctx context.Context, req *apiservice.Instance, serviceID string) (
	*svctypes.Service, *apimodel.Response) {
	svc, err := s.storage.GetServiceByID(serviceID)
	if err != nil {
		log.Error(err.Error(), utils.RequestID(ctx))
		return nil, api.NewInstanceResponse(storeapi.StoreCode2APICode(err), req)
	}
	if svc == nil {
		return nil, api.NewInstanceResponse(apimodel.Code_NotFoundResource, req)
	}

	return svc, nil
}

// 获取api.instance
func (s *Server) getInstance(service *apiservice.Service, instance *apiservice.Instance) *apiservice.Instance {
	out := &apiservice.Instance{
		Id:                instance.GetId(),
		Service:           service.GetName(),
		Namespace:         service.GetNamespace(),
		Host:              instance.GetHost(),
		Port:              instance.GetPort(),
		Protocol:          instance.GetProtocol(),
		Version:           instance.GetVersion(),
		Priority:          instance.GetPriority(),
		Weight:            instance.GetWeight(),
		EnableHealthCheck: instance.GetEnableHealthCheck(),
		HealthCheck:       instance.GetHealthCheck(),
		Healthy:           instance.GetHealthy(),
		Isolate:           instance.GetIsolate(),
		Location:          instance.GetLocation(),
		Metadata:          instance.GetMetadata(),
		Ctime:             instance.GetCtime(),
		Mtime:             instance.GetMtime(),
		Revision:          instance.GetRevision(),
	}

	s.packCmdb(out)
	return out
}

// 获取cmdb
func (s *Server) packCmdb(instance *apiservice.Instance) {
	if instance == nil || !isEmptyLocation(instance.GetLocation()) {
		return
	}

	location, err := cmdb.GetCMDB().GetLocation(instance.GetHost())
	if err != nil {
		log.Error("[Instance] pack cmdb info fail",
			zap.String("namespace", instance.GetNamespace()),
			zap.String("service", instance.GetService()),
			zap.String("host", instance.GetHost()),
			zap.Uint32("port", instance.GetPort()))
		return
	}
	if location != nil {
		instance.Location = location.Proto
	}
}

func isEmptyLocation(loc *apimodel.Location) bool {
	return loc == nil || (loc.GetRegion() == "" &&
		loc.GetZone() == "" &&
		loc.GetCampus() == "")
}

func (s *Server) sendDiscoverEvent(event *svctypes.InstanceEvent) {
	if event.Instance != nil {
		// In order not to cause `panic` in cause multi-corporate data op, do deep copy
		// event.Instance = proto.Clone(event.Instance).(*apiservice.Instance)
	}
	_ = eventhub.Publish(eventhub.InstanceEventTopic, event)
}

type wrapSvcName interface {
	// GetService 获取服务名
	GetService() string
	// GetNamespace 获取命名空间
	GetNamespace() string
}

type rawSvcName interface {
	// GetService 获取服务名
	GetService() string
	// GetNamespace 获取命名空间
	GetNamespace() string
}

// createWrapServiceIfAbsent 如果服务不存在，则进行创建，并返回服务的ID信息
func (s *Server) createWrapServiceIfAbsent(ctx context.Context, instance wrapSvcName) (string, *apimodel.Response) {
	return s.createServiceIfAbsent(ctx, instance.GetNamespace(), instance.GetService())
}

func (s *Server) createServiceIfAbsent(
	ctx context.Context, namespace string, svcName string) (string, *apimodel.Response) {
	svc, errResp := s.loadService(namespace, svcName)
	if errResp != nil {
		return "", errResp
	}
	if svc != nil {
		return svc.ID, nil
	}
	// if auto_create_service config is false, return service not found
	if !s.allowAutoCreate() {
		return "", api.NewResponse(apimodel.Code_NotFoundResource)
	}
	simpleService := &apiservice.Service{
		Name:      string(svcName),
		Namespace: string(namespace),
		Owners: func() string {
			owner := utils.ParseOwnerID(ctx)
			if owner == "" {
				return string("pole")
			}
			return string(owner)
		}(),
	}
	key := fmt.Sprintf("%s:%s", simpleService.Namespace, simpleService.Name)
	ret, err, _ := s.createServiceSingle.Do(key, func() (interface{}, error) {
		resp := s.CreateService(ctx, simpleService)
		return resp, nil
	})
	if err != nil {
		return "", api.NewResponseWithMsg(apimodel.Code_ExecuteException, err.Error())
	}
	resp := ret.(*apimodel.Response)
	retCode := apimodel.Code(resp.GetCode())
	if retCode != apimodel.Code_ExecuteSuccess && retCode != apimodel.Code_ExistedResource {
		return "", resp
	}
	// 从 resp.Data 中提取服务数据
	if resp.Data == nil {
		return "", api.NewResponseWithMsg(apimodel.Code_ExecuteException, "service data not found in response")
	}
	service := &apiservice.Service{}
	if err := resp.Data.UnmarshalTo(service); err != nil {
		return "", api.NewResponseWithMsg(apimodel.Code_ExecuteException, fmt.Sprintf("failed to unmarshal service: %v", err))
	}
	svcId := service.GetId()
	return svcId, nil
}

func (s *Server) loadService(namespace string, svcName string) (*svctypes.Service, *apimodel.Response) {
	svc := s.caches.Service().GetServiceByName(svcName, namespace)
	if svc != nil {
		if svc.IsAlias() {
			return nil, api.NewResponseWithMsg(apimodel.Code_BadRequest, "service is alias")
		}
		return svc, nil
	}
	// 再走数据库查询一遍
	svc, err := s.storage.GetService(svcName, namespace)
	if err != nil {
		return nil, api.NewResponseWithMsg(storeapi.StoreCode2APICode(err), err.Error())
	}
	if svc != nil && svc.IsAlias() {
		return nil, api.NewResponseWithMsg(apimodel.Code_BadRequest, "service is alias")
	}
	return svc, nil
}

func (s *Server) loadServiceByID(svcID string) (*svctypes.Service, error) {
	svc := s.caches.Service().GetServiceByID(svcID)
	if svc != nil {
		if svc.IsAlias() {
			return nil, errors.New("service is alias")
		}
		return svc, nil
	}

	// 再走数据库查询一遍
	svc, err := s.storage.GetServiceByID(svcID)
	if err != nil {
		return nil, err
	}

	if svc != nil && svc.IsAlias() {
		return nil, errors.New("service is alias")
	}

	return svc, nil
}

// 获取instance请求的token信息
func parseInstanceReqToken(ctx context.Context, req *apiservice.Instance) string {
	// 根据 pole-io/specification，Instance 不再有 ServiceToken 字段
	// 直接从上下文中获取 token
	return utils.ParseToken(ctx)
}

// 实例查询前置处理
func preGetInstances(query map[string]string) (map[string]string, map[string]string, *apimodel.BatchQueryResponse) {
	var metaFilter map[string]string
	metaKey, metaKeyAvail := query["keys"]
	if metaKeyAvail {
		metaFilter = map[string]string{}
		keys := strings.Split(metaKey, ",")
		values := strings.Split(query["values"], ",")
		for i := range keys {
			metaFilter[keys[i]] = values[i]
		}
	}

	// 以healthy为准
	_, lhs := query["health_status"]
	_, rhs := query["healthy"]
	if lhs && rhs {
		delete(query, "health_status")
	}

	filters := make(map[string]string)
	for key, value := range query {
		if attr, ok := InsFilter2toreAttr[key]; ok {
			key = attr
		}
		if !NotInsFilterAttr[key] {
			filters[key] = value
		}
	}

	return filters, metaFilter, nil
}

// 批量操作实例
func batchOperateInstances(ctx context.Context, reqs []*apiservice.Instance,
	handler func(ctx context.Context, req *apiservice.Instance) *apimodel.Response) *apimodel.BatchWriteResponse {
	responses := api.NewBatchWriteResponse(apimodel.Code_ExecuteSuccess)

	chs := make([]chan *apimodel.Response, 0, len(reqs))
	for i, instance := range reqs {
		chs = append(chs, make(chan *apimodel.Response))
		go func(index int, ins *apiservice.Instance) {
			chs[index] <- handler(ctx, ins)
		}(i, instance)
	}

	for _, ch := range chs {
		resp := <-ch
		api.Collect(responses, resp)
	}

	return api.FormatBatchWriteResponse(responses)
}

// wrapper instance store response
func wrapperInstanceStoreResponse(instance *apiservice.Instance, err error) *apimodel.Response {
	if err == nil {
		return nil
	}
	resp := api.NewInstanceResponse(storeapi.StoreCode2APICode(err), instance)
	resp.Info += ": " + err.Error()
	return resp
}

// 生成instance的记录entry
func instanceRecordEntry(ctx context.Context, req *apiservice.Instance, svc *svctypes.Service, ins *svctypes.Instance,
	opt types.OperationType) *types.RecordEntry {
	if svc == nil || ins == nil {
		return nil
	}
	datail, _ := protojson.MarshalOptions{EmitUnpopulated: true}.Marshal(req)
	entry := &types.RecordEntry{
		ResourceType:  types.RInstance,
		ResourceName:  fmt.Sprintf("%s(%s:%d)", svc.Name, ins.Host(), ins.Port()),
		Namespace:     svc.Namespace,
		OperationType: opt,
		Operator:      utils.ParseOperator(ctx),
		Detail:        string(datail),
		HappenTime:    time.Now(),
	}
	return entry
}

type InstanceChain interface {
	// AfterUpdate .
	AfterUpdate(ctx context.Context, instances ...*svctypes.Instance)
}
