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
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	restful "github.com/emicklei/go-restful/v3"
	"github.com/pkg/errors"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.uber.org/zap"
	"google.golang.org/protobuf/encoding/protojson"

	specai "github.com/pole-io/specification/source/go/api/v1/ai"
	apimodel "github.com/pole-io/specification/source/go/api/v1/model"
	apiservice "github.com/pole-io/specification/source/go/api/v1/service_manage"

	authapi "github.com/pole-io/pole-server/apis/access_control/auth"
	"github.com/pole-io/pole-server/apis/access_control/ratelimit"
	"github.com/pole-io/pole-server/apis/access_control/whitelist"
	"github.com/pole-io/pole-server/apis/apiserver"
	"github.com/pole-io/pole-server/apis/observability/statis"
	"github.com/pole-io/pole-server/apis/pkg/types"
	"github.com/pole-io/pole-server/apis/pkg/types/metrics"
	storeapi "github.com/pole-io/pole-server/apis/store"
	"github.com/pole-io/pole-server/pkg/admin"
	"github.com/pole-io/pole-server/pkg/cache"
	api "github.com/pole-io/pole-server/pkg/common/api/v1"
	"github.com/pole-io/pole-server/pkg/common/conn/keepalive"
	connlimit "github.com/pole-io/pole-server/pkg/common/conn/limit"
	commonlog "github.com/pole-io/pole-server/pkg/common/log"
	commonotel "github.com/pole-io/pole-server/pkg/common/otel"
	"github.com/pole-io/pole-server/pkg/common/secure"
	"github.com/pole-io/pole-server/pkg/common/utils"
	"github.com/pole-io/pole-server/pkg/goverrule"
	"github.com/pole-io/pole-server/pkg/namespace"
	"github.com/pole-io/pole-server/pkg/service"
	"github.com/pole-io/pole-server/pkg/service/healthcheck"
	"github.com/pole-io/pole-server/pkg/systemconfig"
	"github.com/pole-io/pole-server/plugin/apiserver/httpserver/aia2a"
	"github.com/pole-io/pole-server/plugin/apiserver/httpserver/aimcp"
	"github.com/pole-io/pole-server/plugin/apiserver/httpserver/auth"
	confighttp "github.com/pole-io/pole-server/plugin/apiserver/httpserver/config"
	discovery "github.com/pole-io/pole-server/plugin/apiserver/httpserver/discover"
	skillhttp "github.com/pole-io/pole-server/plugin/apiserver/httpserver/skillmarketplace"
	httpcommon "github.com/pole-io/pole-server/plugin/apiserver/httpserver/utils"
)

var (
	cacheTypes = map[string]struct{}{
		apiservice.DiscoverResponse_INSTANCE.String():              {},
		apiservice.DiscoverResponse_CUSTOM_ROUTE_RULE.String():     {},
		apiservice.DiscoverResponse_NEARBY_ROUTE_RULE.String():     {},
		apiservice.DiscoverResponse_RATE_LIMIT.String():            {},
		apiservice.DiscoverResponse_CIRCUIT_BREAKER.String():       {},
		apiservice.DiscoverResponse_FAULT_DETECTOR.String():        {},
		apiservice.DiscoverResponse_SERVICES.String():              {},
		apiservice.DiscoverResponse_TRAFFIC_SECURITY_RULE.String(): {},
		apiservice.DiscoverResponse_TRAFFIC_MIRROR_RULE.String():   {},
		apiservice.DiscoverResponse_TRAFFIC_MOCK_RULE.String():     {},
	}
)

// HTTPServer HTTP API服务器
type HTTPServer struct {
	listenIP        string
	listenPort      uint32
	connLimitConfig *connlimit.Config
	tlsInfo         *secure.TLSInfo
	option          map[string]interface{}
	openAPI         map[string]apiserver.APIConfig
	start           bool
	restart         bool
	exitCh          chan struct{}
	ready           chan struct{}
	ctx             context.Context

	enablePprof   *atomic.Bool
	enableSwagger bool

	server                     *http.Server
	maintainServer             admin.AdminOperateServer
	namespaceServer            namespace.NamespaceOperateServer
	environmentPromotionServer *namespace.Server
	namingServer               service.DiscoverServer
	ruleServer                 goverrule.GoverRuleServer
	healthCheckServer          *healthcheck.Server
	rateLimit                  ratelimit.Ratelimit
	statis                     statis.Statis
	whitelist                  whitelist.Whitelist
	systemSettings             systemconfig.EffectiveProvider
	systemConfigUser           credentialChecker
	systemConfigAuth           consolePermissionChecker

	discoverSvr *discovery.HTTPServer
	configSvr   *confighttp.HTTPServer
	aimcpSvr    *aimcp.HTTPServer
	aia2aSvr    *aia2a.HTTPServer
	skillSvr    *skillhttp.HTTPServer
	authSvr     *auth.HTTPServer

	// apiserverSlots
	apiserverSlots map[string]apiserver.Apiserver
}

const (
	// Discover discover string
	Discover               string = "Discover"
	defaultAlivePeriodTime        = 3 * time.Minute
)

// GetPort 获取端口
func (h *HTTPServer) GetPort() uint32 {
	return h.listenPort
}

// Ready 在 handler 完成构造且 listener 已绑定后关闭。
func (h *HTTPServer) Ready() <-chan struct{} {
	return h.ready
}

// SnapshotMCPTools exposes the in-process tools/list projection to the
// bootstrap self-manager without widening the public MCP management surface.
func (h *HTTPServer) SnapshotMCPTools(ctx context.Context) ([]*specai.MCPServerTool, error) {
	if h.aimcpSvr == nil || !h.isAPIEnabled("aimcp") {
		return nil, errors.New("ai-mcp API server is not enabled")
	}
	return h.aimcpSvr.SnapshotTools(ctx)
}

// GetProtocol 获取Server的协议
func (h *HTTPServer) GetProtocol() string {
	return "http"
}

// Initialize 初始化HTTP API服务器
func (h *HTTPServer) Initialize(ctx context.Context, option map[string]interface{},
	apiConf map[string]apiserver.APIConfig) error {
	h.ready = make(chan struct{})
	h.option = option
	h.ctx = ctx
	h.openAPI = apiConf
	h.listenIP = option["listenIP"].(string)
	h.listenPort = uint32(option["listenPort"].(int))
	h.enablePprof = &atomic.Bool{}
	if val, _ := option["enablePprof"].(bool); val {
		h.enablePprof.Store(true)
	}
	h.enableSwagger, _ = option["enableSwagger"].(bool)
	h.apiserverSlots, _ = ctx.Value(utils.ContextAPIServerSlot{}).(map[string]apiserver.Apiserver)
	h.systemSettings, _ = systemconfig.ProviderFromContext(ctx, systemconfig.ComponentServer)
	// 连接数限制的配置
	if raw, _ := option["connLimit"].(map[interface{}]interface{}); raw != nil {
		connLimitConfig, err := connlimit.ParseConnLimitConfig(raw)
		if err != nil {
			return err
		}
		h.connLimitConfig = connLimitConfig
	}
	if rateLimit := ratelimit.GetRatelimit(); rateLimit != nil {
		log.Infof("http server open the ratelimit")
		h.rateLimit = rateLimit
	}

	if whitelist := whitelist.GetWhitelist(); whitelist != nil {
		log.Infof("http server open the whitelist")
		h.whitelist = whitelist
	}

	if raw, _ := option["enableCacheProto"].(bool); raw {
		types := make([]string, 0, len(cacheTypes))
		for k := range cacheTypes {
			types = append(types, k)
		}
		httpcommon.InitProtoCache(option, types, discoverCacheConvert)
	}

	// tls 配置信息
	if raw, _ := option["tls"].(map[interface{}]interface{}); raw != nil {
		tlsConfig, err := secure.ParseTLSConfig(raw)
		if err != nil {
			return err
		}
		h.tlsInfo = &secure.TLSInfo{
			CertFile:      tlsConfig.CertFile,
			KeyFile:       tlsConfig.KeyFile,
			TrustedCAFile: tlsConfig.TrustedCAFile,
		}
	}
	return nil
}

// Run 启动HTTP API服务器
func (h *HTTPServer) Run(errCh chan error) {
	log.Infof("start httpserver")
	h.exitCh = make(chan struct{}, 1)
	h.start = true
	defer func() {
		close(h.exitCh)
		h.start = false
	}()

	var err error

	h.maintainServer, err = admin.GetServer()
	if err != nil {
		log.Errorf("%v", err)
		errCh <- err
		return
	}

	// 引入命名空间模块
	h.namespaceServer, err = namespace.GetServer()
	if err != nil {
		log.Errorf("%v", err)
		errCh <- err
		return
	}
	h.environmentPromotionServer, err = namespace.GetOriginServer()
	if err != nil {
		log.Errorf("%v", err)
		errCh <- err
		return
	}

	// 引入功能模块和插件
	h.namingServer, err = service.GetServer()
	if err != nil {
		log.Errorf("%v", err)
		errCh <- err
		return
	}

	h.ruleServer, err = goverrule.GetServer()
	if err != nil {
		log.Error("", zap.Error(err))
		errCh <- err
		return
	}

	userSvr, err := authapi.GetUserServer()
	if err != nil {
		log.Errorf("%v", err)
		errCh <- err
		return
	}
	h.systemConfigUser = userSvr

	policySvr, err := authapi.GetStrategyServer()
	if err != nil {
		log.Errorf("%v", err)
		errCh <- err
		return
	}
	h.systemConfigAuth = policySvr.GetAuthChecker()

	h.authSvr = auth.NewServer(userSvr, policySvr)

	h.healthCheckServer, err = healthcheck.GetServer()
	if err != nil {
		log.Errorf("%v", err)
		errCh <- err
		return
	}
	h.statis = statis.GetStatis()

	h.discoverSvr = discovery.NewServer(h.namingServer, h.ruleServer, h.healthCheckServer)
	h.configSvr, err = confighttp.NewServer()
	if err != nil {
		errCh <- err
		return
	}

	storage, err := storeapi.GetStore()
	if err != nil {
		log.Errorf("%v", err)
		errCh <- err
		return
	}

	aimcpSvr, err := aimcp.NewServer(h.ctx, h.maintainServer, h.namespaceServer, storage)
	if err != nil {
		errCh <- err
		return
	}
	h.aimcpSvr = aimcpSvr
	if h.isAPIEnabled("console") {
		cacheMgr, cacheErr := cache.GetCacheManager()
		if cacheErr != nil {
			errCh <- cacheErr
			return
		}
		skillSvr, skillErr := skillhttp.NewServer(h.ctx, storage, h.systemConfigUser, h.systemConfigAuth, cacheMgr)
		if skillErr != nil {
			errCh <- skillErr
			return
		}
		h.skillSvr = skillSvr
	}
	if h.isAPIEnabled("aia2a") {
		aia2aSvr, err := aia2a.NewServer(h.ctx, storage)
		if err != nil {
			errCh <- err
			return
		}
		h.aia2aSvr = aia2aSvr
	}

	// 初始化http server
	address := fmt.Sprintf("%v:%v", h.listenIP, h.listenPort)

	var wsContainer *restful.Container
	wsContainer, err = h.createRestfulContainer()
	if err != nil {
		errCh <- err
		return
	}

	server := http.Server{
		Addr: address,
		Handler: otelhttp.NewHandler(wsContainer, "pole-control-plane-http",
			otelhttp.WithSpanNameFormatter(func(_ string, request *http.Request) string {
				path := strings.TrimSuffix(request.URL.Path, "/")
				if path == "" {
					path = "/"
				}
				return request.Method + ":" + path
			}),
		),
		WriteTimeout: 1 * time.Minute,
	}
	var ln net.Listener
	ln, err = net.Listen("tcp", address)
	if err != nil {
		log.Errorf("net listen(%s) err: %s", address, err.Error())
		errCh <- err
		return
	}

	ln = keepalive.NewTcpKeepAliveListener(defaultAlivePeriodTime, ln.(*net.TCPListener))
	// 开启最大连接数限制
	if h.connLimitConfig != nil && h.connLimitConfig.OpenConnLimit {
		log.Infof("http server use max connection limit per ip: %d, http max limit: %d",
			h.connLimitConfig.MaxConnPerHost, h.connLimitConfig.MaxConnLimit)
		ln, err = connlimit.NewListener(ln, h.GetProtocol(), h.connLimitConfig)
		if err != nil {
			log.Errorf("conn limit init err: %s", err.Error())
			errCh <- err
			return
		}
	}
	h.server = &server
	close(h.ready)

	// 开始对外服务
	if h.tlsInfo.IsEmpty() {
		err = server.Serve(ln)
	} else {
		err = server.ServeTLS(ln, h.tlsInfo.CertFile, h.tlsInfo.KeyFile)
	}
	if err != nil {
		log.Errorf("%+v", err)
		if !h.restart {
			log.Infof("not in restart progress, broadcast error")
			errCh <- err
		}

		return
	}

	log.Infof("httpserver stop")
}

// Stop shutdown server
func (h *HTTPServer) Stop() {
	// 释放connLimit的数据，如果没有开启，也需要执行一下
	// 目的：防止restart的时候，connLimit冲突
	connlimit.RemoveLimitListener(h.GetProtocol())
	// stop http server
	if h.server != nil {
		// 延迟三秒，等待http server关闭，做到流量无损。
		// 在此之前已经建立的链接，会正常执行业务，若执行时长超过3秒，则会抛出异常。
		// 若是在3秒内提前处理完所有请求，h.server会提前关闭。
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := h.server.Shutdown(ctx); nil != err {
			log.Errorf("httpserver shutdown failed, err: %v\n", err)
		}
	}
}

// Restart restart server
func (h *HTTPServer) Restart(option map[string]interface{}, apiConf map[string]apiserver.APIConfig,
	errCh chan error) error {
	log.Infof("restart httpserver new config: %+v", option)
	// 备份一下option
	backupOption := h.option
	// 备份一下api
	backupAPI := h.openAPI

	// 设置restart标记，防止stop的时候把错误抛出
	h.restart = true
	// 关闭httpserver
	h.Stop()
	// 等待httpserver退出
	if h.start {
		<-h.exitCh
	}

	log.Infof("old httpserver has stopped, begin restart httpserver")

	ctx := context.Background()
	if err := h.Initialize(ctx, option, apiConf); err != nil {
		h.restart = false
		if initErr := h.Initialize(ctx, backupOption, backupAPI); initErr != nil {
			log.Errorf("start httpserver with backup cfg err: %s", initErr.Error())
			return initErr
		}
		go h.Run(errCh)

		log.Errorf("restart httpserver initialize err: %s", err.Error())
		return err
	}

	log.Infof("init httpserver successfully, restart it")
	h.restart = false
	go h.Run(errCh)
	return nil
}

// createRestfulContainer create handler
func (h *HTTPServer) createRestfulContainer() (*restful.Container, error) {
	wsContainer := restful.NewContainer()
	wsContainer.RecoverHandler(h.recoverFunc)

	// 增加CORS TODO
	cors := restful.CrossOriginResourceSharing{
		// ExposeHeaders:  []string{"X-My-Header"},
		AllowedHeaders: []string{"Content-Type", "Accept", types.HeaderRequestId},
		AllowedMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut},
		CookiesAllowed: false,
		Container:      wsContainer}
	wsContainer.Filter(cors.Filter)

	// Incr container filter to respond to OPTIONS
	wsContainer.Filter(wsContainer.OPTIONSFilter)

	wsContainer.Filter(h.process)

	for name, apiConfig := range h.openAPI {
		switch name {
		case "admin":
			if apiConfig.Enable {
				wsContainer.Add(h.GetIndexServer())
				wsContainer.Add(h.GetAdminAccessServer())
			}
		case "aimcp":
			if apiConfig.Enable {
				wsContainer.Add(h.aimcpSvr.GetMCPAccessServer(apiConfig.Include))
			}
		case "aia2a":
			if apiConfig.Enable {
				wsContainer.Add(h.aia2aSvr.GetA2AAccessServer(apiConfig.Include))
			}
		case "console":
			if apiConfig.Enable {
				wsContainer.Add(h.discoverSvr.GetConsoleAccessServer(apiConfig.Include))
				wsContainer.Add(h.GetCoreV1ConsoleAccessServer(apiConfig.Include))
				wsContainer.Add(h.authSvr.GetAuthServer())
				if h.skillSvr != nil {
					wsContainer.Add(h.skillSvr.GetAccessServer())
				}
			}
		case "client":
			if apiConfig.Enable {
				ws := new(restful.WebService)
				ws.Path("/v1").Consumes(restful.MIME_JSON).Produces(restful.MIME_JSON)
				h.discoverSvr.GetClientAccessServer(ws, apiConfig.Include)
				wsContainer.Add(ws)
			}
		default:
			log.Warnf("api %s does not exist in httpserver", name)
		}
	}

	if h.configSvr != nil {
		consoleAPI := h.openAPI["console"]
		wsContainer.Add(h.configSvr.GetUnifiedAccessServer(consoleAPI.Enable, consoleAPI.Include))
	}

	h.enablePprofAccess(wsContainer)
	// 收集插件的 endpoint 数据
	h.enablePluginDebugAccess(wsContainer)
	return wsContainer, nil
}

func (h *HTTPServer) isAPIEnabled(name string) bool {
	apiConfig, ok := h.openAPI[name]
	return ok && apiConfig.Enable
}

// enablePluginDebugAccess .
func (h *HTTPServer) enablePluginDebugAccess(wsContainer *restful.Container) {
	log.Infof("open http access for plugin")

	wsContainer.Handle("/debug/endpoints", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		endpints := make([]map[string]string, 0, 8)
		for _, checker := range h.healthCheckServer.Checkers() {
			handlers := checker.DebugHandlers()
			for i := range handlers {
				endpints = append(endpints, map[string]string{
					"path": handlers[i].Path,
					"desc": handlers[i].Desc,
				})
			}
		}

		for _, item := range h.apiserverSlots {
			if val, ok := item.(apiserver.EnrichApiserver); ok {
				handlers := val.DebugHandlers()
				for i := range handlers {
					endpints = append(endpints, map[string]string{
						"path": handlers[i].Path,
						"desc": handlers[i].Desc,
					})
				}
			}
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(utils.MustJson(endpints)))
	}))

	for _, checker := range h.healthCheckServer.Checkers() {
		handlers := checker.DebugHandlers()
		for i := range handlers {
			wsContainer.Handle(handlers[i].Path, handlers[i].Handler)
		}
	}

	for _, item := range h.apiserverSlots {
		if val, ok := item.(apiserver.EnrichApiserver); ok {
			handlers := val.DebugHandlers()
			for i := range handlers {
				wsContainer.Handle(handlers[i].Path, handlers[i].Handler)
			}
		}
	}
}

// process 在接收和回复时统一处理请求
func (h *HTTPServer) process(req *restful.Request, rsp *restful.Response, chain *restful.FilterChain) {
	path := strings.TrimSuffix(req.Request.URL.Path, "/")
	if path == "" {
		path = "/"
	}
	requestCtx := commonotel.WithRequestSpanState(req.Request.Context())
	ctx, end := commonotel.StartSpan(requestCtx, "handler "+req.Request.Method+":"+path, "handler")
	req.Request = req.Request.WithContext(ctx)
	var processErr error
	func() {
		if err := h.preprocess(req, rsp); err != nil {
			processErr = err
			return
		}

		chain.ProcessFilter(req, rsp)
	}()

	h.postProcess(req, rsp)
	end(processErr)
}

// preprocess 请求预处理
func (h *HTTPServer) preprocess(req *restful.Request, rsp *restful.Response) error {
	// 设置开始时间
	req.SetAttribute("start-time", time.Now())

	// 处理请求ID
	requestID := req.HeaderParameter(types.HeaderRequestId)
	if requestID == "" {
		// TODO: 设置请求ID
	}

	platformID := req.HeaderParameter("Platform-Id")
	requestURL := req.Request.URL.String()
	if !strings.Contains(requestURL, Discover) {
		// 打印请求
		log.Info("receive request",
			zap.String("client-address", req.Request.RemoteAddr),
			zap.String("user-agent", req.HeaderParameter("User-Agent")),
			utils.ZapRequestID(requestID),
			zap.String("platform-id", platformID),
			zap.String("method", req.Request.Method),
			zap.String("url", requestURL),
		)
	}

	// 管理端接口访问鉴权
	if strings.Contains(requestURL, "naming") {
		if err := h.enterAuth(req, rsp); err != nil {
			return err
		}
	}

	// 限流
	if err := h.enterRateLimit(req, rsp); err != nil {
		return err
	}

	return nil
}

// postProcess 请求后处理：统计
func (h *HTTPServer) postProcess(req *restful.Request, rsp *restful.Response) {
	now := time.Now()

	// 接口调用统计
	path := req.Request.URL.Path
	if path != "/" {
		// 去掉最后一个"/"
		path = strings.TrimSuffix(path, "/")
	}
	method := req.Request.Method + ":" + path
	startTime := req.Attribute("start-time").(time.Time)
	code, ok := req.Attribute(utils.PolarisCode).(uint32)

	recordApiCall := true
	if !ok {
		code = uint32(rsp.StatusCode())
		recordApiCall = code != http.StatusNotFound && code != http.StatusMethodNotAllowed
	}

	diff := now.Sub(startTime)
	// 打印耗时超过1s的请求
	if diff > time.Second {
		var scope *commonlog.Scope
		if strings.Contains(path, "naming") {
			scope = namingLog
		} else {
			scope = configLog
		}

		scope.Info("handling time > 1s",
			zap.String("client-address", req.Request.RemoteAddr),
			zap.String("user-agent", req.HeaderParameter("User-Agent")),
			utils.ZapRequestID(req.HeaderParameter(types.HeaderRequestId)),
			zap.String("method", req.Request.Method),
			zap.String("url", req.Request.URL.String()),
			zap.Duration("handling-time", diff),
		)
	}

	if recordApiCall {
		h.statis.ReportCallMetrics(metrics.CallMetric{
			Type:     metrics.ServerCallMetric,
			API:      method,
			Protocol: "HTTP",
			Code:     int(code),
			Duration: diff,
		})
	}
}

// enterAuth 访问鉴权
func (h *HTTPServer) enterAuth(req *restful.Request, rsp *restful.Response) error {
	// 判断白名单插件是否开启
	if h.whitelist == nil {
		return nil
	}
	_, end := commonotel.StartSpan(req.Request.Context(), "middleware.network_acl", "middleware")
	var retErr error
	defer func() { end(retErr) }()

	rid := req.HeaderParameter(types.HeaderRequestId)

	address := req.Request.RemoteAddr
	segments := strings.Split(address, ":")
	if len(segments) != 2 {
		return nil
	}
	if !h.whitelist.Contain(segments[0]) {
		log.Error("http access is not allowed",
			zap.String("client", address),
			utils.ZapRequestID(rid))
		httpcommon.HTTPResponse(req, rsp, api.NotAllowedAccess)
		retErr = errors.New("http access is not allowed")
		return retErr
	}
	return nil
}

// enterRateLimit 访问限制
func (h *HTTPServer) enterRateLimit(req *restful.Request, rsp *restful.Response) error {
	// 检查限流插件是否开启
	if h.rateLimit == nil {
		return nil
	}
	_, end := commonotel.StartSpan(req.Request.Context(), "middleware.rate_limit", "middleware")
	var retErr error
	defer func() { end(retErr) }()

	rid := req.HeaderParameter(types.HeaderRequestId)
	// IP级限流
	// 先获取当前请求的address
	address := req.Request.RemoteAddr
	segments := strings.Split(address, ":")
	if len(segments) != 2 {
		return nil
	}
	if ok := h.rateLimit.Allow(ratelimit.IPRatelimit, segments[0]); !ok {
		log.Error("ip ratelimit is not allow", zap.String("client", address),
			utils.ZapRequestID(rid))
		httpcommon.HTTPResponse(req, rsp, api.IPRateLimit)
		retErr = errors.New("ip ratelimit is not allow")
		return retErr
	}

	// 接口级限流
	apiName := fmt.Sprintf("%s:%s", req.Request.Method,
		strings.TrimSuffix(req.Request.URL.Path, "/"))
	if ok := h.rateLimit.Allow(ratelimit.APIRatelimit, apiName); !ok {
		log.Error("api ratelimit is not allow", zap.String("client", address),
			utils.ZapRequestID(rid), zap.String("api", apiName))
		httpcommon.HTTPResponse(req, rsp, api.APIRateLimit)
		retErr = errors.New("api ratelimit is not allow")
		return retErr
	}

	return nil
}

func (h *HTTPServer) recoverFunc(i interface{}, w http.ResponseWriter) {
	log.Errorf("panic %+v", i)
	obj := &apimodel.Response{}

	status := api.CalcCode(obj)
	if code := obj.GetCode(); code != api.ExecuteSuccess {
		w.Header().Add(utils.PolarisCode, fmt.Sprintf("%d", code))
		w.Header().Add(utils.PolarisMessage, api.Code2Info(code))
	}
	w.WriteHeader(status)

	data, _ := protojson.MarshalOptions{Indent: " ", EmitUnpopulated: true}.Marshal(obj)
	_, _ = w.Write(data)
}

func discoverCacheConvert(m interface{}) *httpcommon.CacheObject {
	resp, ok := m.(*apiservice.DiscoverResponse)
	if !ok {
		return nil
	}
	if resp.Code != uint32(apimodel.Code_ExecuteSuccess) {
		return nil
	}
	if resp.GetService().GetRevision() == "" {
		return nil
	}
	if _, ok := cacheTypes[resp.GetType().String()]; !ok {
		return nil
	}
	keyProto := fmt.Sprintf("%s-%s-%s", resp.GetService().GetNamespace(),
		resp.GetService().GetName(), resp.GetService().GetRevision())

	return &httpcommon.CacheObject{
		OriginVal: resp,
		CacheType: resp.Type.String(),
		Key:       keyProto,
	}
}
