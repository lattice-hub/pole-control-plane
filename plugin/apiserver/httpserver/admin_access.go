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

package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/emicklei/go-restful/v3"
	"github.com/golang/protobuf/jsonpb"

	apimodel "github.com/pole-io/specification/source/go/api/v1/model"
	"github.com/pole-io/specification/source/go/api/v1/security"
	apiservice "github.com/pole-io/specification/source/go/api/v1/service_manage"

	"github.com/pole-io/pole-server/apis/pkg/types"
	"github.com/pole-io/pole-server/apis/pkg/types/admin"
	authtypes "github.com/pole-io/pole-server/apis/pkg/types/auth"
	api "github.com/pole-io/pole-server/pkg/common/api/v1"
	"github.com/pole-io/pole-server/pkg/compatibility"
	"github.com/pole-io/pole-server/pkg/systemconfig"
	"github.com/pole-io/pole-server/plugin/apiserver/httpserver/docs"
	httpcommon "github.com/pole-io/pole-server/plugin/apiserver/httpserver/utils"
)

// GetIndexServer get index server
func (h *HTTPServer) GetIndexServer() *restful.WebService {
	ws := new(restful.WebService)

	ws.Route(ws.GET("/").To(h.index))

	return ws
}

// index  URL: "/"
func (h *HTTPServer) index(_ *restful.Request, rsp *restful.Response) {
	_, _ = rsp.Write([]byte("Polaris Server"))
}

// GetAdminAccessServer 运维接口
func (h *HTTPServer) GetAdminAccessServer() *restful.WebService {
	ws := new(restful.WebService)
	ws.Path("/admin/v1").Consumes(restful.MIME_JSON).Produces(restful.MIME_JSON)

	ws.Route(docs.EnrichGetServerConnectionsApiDocs(ws.GET("/apiserver/conn").To(h.GetServerConnections)))
	ws.Route(docs.EnrichGetServerConnStatsApiDocs(ws.GET("/apiserver/conn/stats").To(h.GetServerConnStats)))
	ws.Route(docs.EnrichCloseConnectionsApiDocs(ws.POST("apiserver/conn/close").To(h.CloseConnections)))
	ws.Route(docs.EnrichFreeOSMemoryApiDocs(ws.POST("/memory/free").To(h.FreeOSMemory)))
	ws.Route(docs.EnrichCleanInstanceApiDocs(ws.POST("/instance/clean").To(h.CleanInstance)))
	ws.Route(docs.EnrichBatchCleanInstancesApiDocs(ws.POST("/instance/batchclean").To(h.BatchCleanInstances)))
	ws.Route(docs.EnrichGetLastHeartbeatApiDocs(ws.GET("/instance/heartbeat").To(h.GetLastHeartbeat)))
	ws.Route(docs.EnrichGetLogOutputLevelApiDocs(ws.GET("/log/outputlevel").To(h.GetLogOutputLevel)))
	ws.Route(docs.EnrichSetLogOutputLevelApiDocs(ws.PUT("/log/outputlevel").To(h.SetLogOutputLevel)))
	ws.Route(docs.EnrichListLeaderElectionsApiDocs(ws.GET("/leaders").To(h.ListLeaderElections)))
	ws.Route(docs.EnrichReleaseLeaderElectionApiDocs(ws.POST("/leaders/release").To(h.ReleaseLeaderElection)))
	ws.Route(docs.EnrichGetCMDBInfoApiDocs(ws.GET("/cmdb/info").To(h.GetCMDBInfo)))
	ws.Route(docs.EnrichGetReportClientsApiDocs(ws.GET("/report/clients").To(h.GetReportClients)))
	ws.Route(docs.EnrichEnablePprofApiDocs(ws.POST("/pprof/enable").To(h.EnablePprof)))
	ws.Route(docs.EnrichGetServerFunctionsApiDocs(ws.GET("/server/functions").To(h.GetServerFunctions)))
	ws.Route(ws.GET("/system/configuration").To(h.GetSystemConfiguration))
	ws.Route(ws.POST("/system/configuration/validate").To(h.ValidateSystemConfiguration))
	ws.Route(ws.GET("/mainuser/exist").To(h.HasMainUser))
	ws.Route(ws.POST("/mainuser/create").To(h.InitMainUser))
	return ws
}

type validateSystemConfigurationRequest struct {
	Component string         `json:"component"`
	Domain    string         `json:"domain"`
	Values    map[string]any `json:"values"`
}

func (h *HTTPServer) ValidateSystemConfiguration(req *restful.Request, rsp *restful.Response) {
	ctx := initContext(req)
	if err := authorizeSystemConfiguration(h.systemConfigUser, h.systemConfigAuth, ctx); err != nil {
		_ = rsp.WriteErrorString(http.StatusForbidden, "system configuration validation permission is required")
		return
	}
	request := validateSystemConfigurationRequest{}
	if err := req.ReadEntity(&request); err != nil {
		_ = rsp.WriteErrorString(http.StatusBadRequest, "invalid system configuration validation request")
		return
	}
	if request.Component != compatibility.Component || request.Domain != compatibility.Domain {
		_ = rsp.WriteErrorString(http.StatusNotFound, "system configuration domain has no runtime validator")
		return
	}
	if h.environmentPromotionServer == nil {
		_ = rsp.WriteErrorString(http.StatusServiceUnavailable, "namespace validator is unavailable")
		return
	}
	targets, err := compatibility.ValidateValues(request.Values, h.environmentPromotionServer)
	if err != nil {
		_ = rsp.WriteErrorString(http.StatusUnprocessableEntity, err.Error())
		return
	}
	_ = rsp.WriteAsJson(targets)
}

func (h *HTTPServer) GetSystemConfiguration(req *restful.Request, rsp *restful.Response) {
	ctx := initContext(req)
	if err := authorizeSystemConfiguration(h.systemConfigUser, h.systemConfigAuth, ctx); err != nil {
		status := http.StatusUnauthorized
		message := "valid administrator credentials are required"
		if errors.Is(err, errSystemConfigurationPermissionDenied) {
			status = http.StatusForbidden
			message = "system configuration read permission is required"
		}
		_ = rsp.WriteErrorString(status, message)
		return
	}
	if h.systemSettings == nil {
		_ = rsp.WriteErrorString(http.StatusServiceUnavailable, "system configuration provider is unavailable")
		return
	}
	snapshot, err := h.systemSettings.Effective(ctx, systemconfig.Scope{Component: systemconfig.ComponentServer})
	if err != nil {
		_ = rsp.WriteError(http.StatusInternalServerError, err)
		return
	}
	_ = rsp.WriteAsJson(snapshot)
}

type consolePermissionChecker interface {
	CheckConsolePermission(*authtypes.AcquireContext) (bool, error)
}

type credentialChecker interface {
	CheckCredential(*authtypes.AcquireContext) error
}

var errSystemConfigurationPermissionDenied = errors.New("system configuration read permission denied")

func authorizeSystemConfiguration(credentials credentialChecker, permissions consolePermissionChecker,
	ctx context.Context) error {
	if credentials == nil || permissions == nil {
		return authtypes.ErrorTokenInvalid
	}
	authCtx := authtypes.NewAcquireContext(
		authtypes.WithRequestContext(ctx),
		authtypes.WithModule(authtypes.MaintainModule),
		authtypes.WithOperation(authtypes.Read),
		authtypes.WithMethod(authtypes.DescribeSystemConfiguration),
	)
	// The internal snapshot remains credential-protected even when the public
	// Console authorization switch is disabled.
	if err := credentials.CheckCredential(authCtx); err != nil {
		return err
	}
	pass, err := permissions.CheckConsolePermission(authCtx)
	if err != nil {
		return err
	}
	if !pass {
		return errSystemConfigurationPermissionDenied
	}
	return nil
}

// GetServerConnections 查看server的连接数
// query参数：protocol，必须，查看指定协议server
//
//	host，可选，查看指定host
func (h *HTTPServer) GetServerConnections(req *restful.Request, rsp *restful.Response) {
	ctx := initContext(req)
	params := httpcommon.ParseQueryParams(req)
	connReq := admin.ConnReq{
		Protocol: params["protocol"],
		Host:     params["host"],
	}

	ret, err := h.maintainServer.GetServerConnections(ctx, &connReq)
	if err != nil {
		_ = rsp.WriteError(http.StatusBadRequest, err)
	} else {
		_ = rsp.WriteEntity(ret)
	}
}

// GetServerConnStats 获取连接缓存里面的统计信息
func (h *HTTPServer) GetServerConnStats(req *restful.Request, rsp *restful.Response) {
	ctx := initContext(req)
	params := httpcommon.ParseQueryParams(req)

	var amount int
	if amountStr, ok := params["amount"]; ok {
		if n, err := strconv.Atoi(amountStr); err == nil {
			amount = n
		}
	}

	connReq := admin.ConnReq{
		Protocol: params["protocol"],
		Host:     params["host"],
		Amount:   amount,
	}

	ret, err := h.maintainServer.GetServerConnStats(ctx, &connReq)
	if err != nil {
		_ = rsp.WriteError(http.StatusBadRequest, err)
	} else {
		_ = rsp.WriteAsJson(ret)
	}
}

// CloseConnections 关闭指定client ip的连接
func (h *HTTPServer) CloseConnections(req *restful.Request, rsp *restful.Response) {
	log.Info("[MAINTAIN] Start doing close connections")
	ctx := initContext(req)
	var connReqs []admin.ConnReq
	decoder := json.NewDecoder(req.Request.Body)
	if err := decoder.Decode(&connReqs); err != nil {
		log.Errorf("[MAINTAIN] close connection decode body err: %s", err.Error())
		_ = rsp.WriteError(http.StatusBadRequest, err)
		return
	}
	for _, entry := range connReqs {
		if entry.Protocol == "" {
			log.Errorf("[MAINTAIN] close connection missing protocol")
			_ = rsp.WriteErrorString(http.StatusBadRequest, "missing protocol")
			return
		}
		if entry.Host == "" {
			log.Errorf("[MAINTAIN] close connection missing host")
			_ = rsp.WriteErrorString(http.StatusBadRequest, "missing host")
			return
		}
	}

	if err := h.maintainServer.CloseConnections(ctx, connReqs); err != nil {
		_ = rsp.WriteError(http.StatusBadRequest, err)
	}
}

// FreeOSMemory 增加一个释放系统内存的接口
func (h *HTTPServer) FreeOSMemory(req *restful.Request, rsp *restful.Response) {
	ctx := initContext(req)
	if err := h.maintainServer.FreeOSMemory(ctx); err != nil {
		_ = rsp.WriteError(http.StatusBadRequest, err)
	}
}

// CleanInstance 彻底清理flag=1的实例运维接口
// 支持一个个清理
func (h *HTTPServer) CleanInstance(req *restful.Request, rsp *restful.Response) {
	handler := &httpcommon.Handler{
		Request:  req,
		Response: rsp,
	}

	instance := &apiservice.Instance{}
	ctx, err := handler.Parse(instance)
	if err != nil {
		handler.WriteHeaderAndProto(api.NewResponseWithMsg(apimodel.Code_ParseException, err.Error()))
		return
	}

	handler.WriteHeaderAndProto(h.maintainServer.CleanInstance(ctx, instance))
}

func (h *HTTPServer) BatchCleanInstances(req *restful.Request, rsp *restful.Response) {
	ctx := initContext(req)

	var param struct {
		BatchSize uint32 `json:"batch_size"`
	}

	if err := httpcommon.ParseJsonBody(req, &param); err != nil {
		_ = rsp.WriteError(http.StatusBadRequest, err)
		return
	}

	if count, err := h.maintainServer.BatchCleanInstances(ctx, param.BatchSize); err != nil {
		_ = rsp.WriteError(http.StatusInternalServerError, err)
	} else {
		var ret struct {
			RowsAffected uint32 `json:"rows_affected"`
		}
		ret.RowsAffected = count
		_ = rsp.WriteAsJson(ret)
	}

}

// GetLastHeartbeat 获取实例，上一次心跳的时间
func (h *HTTPServer) GetLastHeartbeat(req *restful.Request, rsp *restful.Response) {
	ctx := initContext(req)
	handler := &httpcommon.Handler{
		Request:  req,
		Response: rsp,
	}
	params := httpcommon.ParseQueryParams(req)
	instance := &apiservice.Instance{}
	if id, ok := params["id"]; ok && id != "" {
		instance.Id = id
	} else {
		instance.Service = params["service"]
		instance.Namespace = params["namespace"]
		instance.Host = params["host"]
		port, _ := strconv.Atoi(params["port"])
		instance.Port = uint32(port)
	}

	ret := h.maintainServer.GetLastHeartbeat(ctx, instance)
	handler.WriteHeaderAndProto(ret)
}

// GetLogOutputLevel 获取日志输出级别
func (h *HTTPServer) GetLogOutputLevel(req *restful.Request, rsp *restful.Response) {
	ctx := initContext(req)

	out, err := h.maintainServer.GetLogOutputLevel(ctx)
	if err != nil {
		_ = rsp.WriteError(http.StatusBadRequest, err)
	} else {
		_ = rsp.WriteAsJson(out)
	}
}

// SetLogOutputLevel 设置日志输出级别
func (h *HTTPServer) SetLogOutputLevel(req *restful.Request, rsp *restful.Response) {
	ctx := initContext(req)

	var scopeLogLevel struct {
		Scope string `json:"scope"`
		Level string `json:"level"`
	}

	if err := httpcommon.ParseJsonBody(req, &scopeLogLevel); err != nil {
		_ = rsp.WriteErrorString(http.StatusBadRequest, err.Error())
		return
	}

	if err := h.maintainServer.SetLogOutputLevel(ctx, scopeLogLevel.Scope, scopeLogLevel.Level); err != nil {
		_ = rsp.WriteErrorString(http.StatusBadRequest, err.Error())
		return
	}

	_ = rsp.WriteEntity("ok")
}

func (h *HTTPServer) ListLeaderElections(req *restful.Request, rsp *restful.Response) {
	ctx := initContext(req)
	leaders, err := h.maintainServer.ListLeaderElections(ctx)
	if err != nil {
		_ = rsp.WriteError(http.StatusBadRequest, err)
		return
	}
	if leaders == nil {
		leaders = []*admin.LeaderElection{}
	}

	_ = rsp.WriteAsJson(leaders)
}

func (h *HTTPServer) ReleaseLeaderElection(req *restful.Request, rsp *restful.Response) {
	ctx := initContext(req)
	var releasedElection struct {
		ElectKey string `json:"electKey"`
	}
	if err := httpcommon.ParseJsonBody(req, &releasedElection); err != nil {
		_ = rsp.WriteErrorString(http.StatusBadRequest, err.Error())
		return
	}
	if err := h.maintainServer.ReleaseLeaderElection(ctx, releasedElection.ElectKey); err != nil {
		_ = rsp.WriteErrorString(http.StatusBadRequest, err.Error())
		return
	}
	_ = rsp.WriteEntity("ok")
}

func (h *HTTPServer) GetCMDBInfo(req *restful.Request, rsp *restful.Response) {
	ctx := initContext(req)

	ret, err := h.maintainServer.GetCMDBInfo(ctx)
	if err != nil {
		_ = rsp.WriteErrorString(http.StatusBadRequest, err.Error())
		return
	}
	_ = rsp.WriteAsJson(ret)
}

func (h *HTTPServer) EnablePprof(req *restful.Request, rsp *restful.Response) {
	var pprofEnable struct {
		Enable bool `json:"enable"`
	}

	if err := httpcommon.ParseJsonBody(req, &pprofEnable); err != nil {
		_ = rsp.WriteErrorString(http.StatusBadRequest, err.Error())
		return
	}

	h.enablePprof.Store(pprofEnable.Enable)
	_ = rsp.WriteEntity("ok")
}

func (h *HTTPServer) HasMainUser(req *restful.Request, rsp *restful.Response) {
	ctx := initContext(req)
	ret := h.maintainServer.HasMainUser(ctx)
	marshaler := jsonpb.Marshaler{Indent: " ", EmitDefaults: true}
	_ = marshaler.Marshal(rsp, ret)
}

func (h *HTTPServer) InitMainUser(req *restful.Request, rsp *restful.Response) {
	handler := &httpcommon.Handler{
		Request:  req,
		Response: rsp,
	}

	user := &security.User{}
	ctx, err := handler.Parse(user)
	if err != nil {
		handler.WriteHeaderAndProto(api.NewAuthResponseWithMsg(apimodel.Code_ParseException, err.Error()))
		return
	}

	ret := h.maintainServer.InitMainUser(ctx, user)
	marshaler := jsonpb.Marshaler{Indent: " ", EmitDefaults: true}
	_ = marshaler.Marshal(rsp, ret)
}

// GetServerFunctions .
func (h *HTTPServer) GetServerFunctions(req *restful.Request, rsp *restful.Response) {
	ctx := initContext(req)
	ret := h.maintainServer.GetServerFunctions(ctx)
	_ = rsp.WriteAsJson(ret)
}

func initContext(req *restful.Request) context.Context {
	authHeaders := []string{types.HeaderAuthorizationKey, "X-Polaris-Token"}
	ctx := req.Request.Context()
	for _, header := range authHeaders {
		if v := req.HeaderParameter(header); v != "" {
			ctx = context.WithValue(ctx, types.ContextAuthTokenKey, v)
		}
	}
	return ctx
}
