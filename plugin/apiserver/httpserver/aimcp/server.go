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

package aimcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/emicklei/go-restful/v3"
	"github.com/mark3labs/mcp-go/server"
	"go.uber.org/zap"

	authapi "github.com/pole-io/pole-server/apis/access_control/auth"
	cacheapi "github.com/pole-io/pole-server/apis/cache"
	"github.com/pole-io/pole-server/apis/pkg/types"
	"github.com/pole-io/pole-server/apis/store"
	"github.com/pole-io/pole-server/pkg/admin"
	"github.com/pole-io/pole-server/pkg/cache"
	commonlog "github.com/pole-io/pole-server/pkg/common/log"
	"github.com/pole-io/pole-server/pkg/common/version"
	"github.com/pole-io/pole-server/pkg/config"
	"github.com/pole-io/pole-server/pkg/namespace"
	"github.com/pole-io/pole-server/pkg/selfmanager"
	"github.com/pole-io/pole-server/pkg/service"
)

const (
	defaultReadAccess string = "default-read"
	defaultAccess     string = "default"
	aimcpAccess       string = "aimcp"

	basePath string = "/ai/mcp/v1"

	sseEp string = "/sse"
	msgEp string = "/message"
)

var (
	selfProbeKeyMu sync.RWMutex
	selfProbeKey   string
)

// HTTPServer
type HTTPServer struct {
	maintainServer  admin.AdminOperateServer
	namespaceServer namespace.NamespaceOperateServer
	configServer    configInspector
	discoverySvr    service.DiscoverServer
	policySvr       authapi.StrategyServer
	storage         store.Store
	cacheMgr        cacheapi.CacheManager
	mcpSvr          *server.MCPServer
	sseSvr          *server.SSEServer
	selfProbeKey    string
}

func ConfigureSelfCapabilityProbeKey(probeKey string) error {
	if err := selfmanager.ValidateCapabilityProbeKey(probeKey); err != nil {
		return err
	}
	selfProbeKeyMu.Lock()
	selfProbeKey = strings.TrimSpace(probeKey)
	selfProbeKeyMu.Unlock()
	return nil
}

func configuredSelfCapabilityProbeKey() string {
	selfProbeKeyMu.RLock()
	defer selfProbeKeyMu.RUnlock()
	return selfProbeKey
}

// NewServer 创建配置中心的 HttpServer
func NewServer(
	ctx context.Context,
	maintainServer admin.AdminOperateServer,
	namespaceServer namespace.NamespaceOperateServer,
	storage store.Store) (*HTTPServer, error) {
	// 初始化配置中心模块
	configServer, err := config.GetUnifiedService()
	if err != nil {
		commonlog.Errorf("set config server to http server error. %v", err)
		return nil, err
	}
	// 初始化服务发现模块
	discoverySvr, err := service.GetServer()
	if err != nil {
		commonlog.Errorf("set discovery server to http server error. %v", err)
		return nil, err
	}

	cacheMgr, err := cache.GetCacheManager()
	if err != nil {
		commonlog.Errorf("set cache manager to ai-mcp server error. %v", err)
		return nil, err
	}
	if err := cacheMgr.OpenResourceCache(cacheapi.ConfigEntry{
		Name: cacheapi.MCPServerName,
	}); err != nil {
		commonlog.Errorf("open mcp-server cache error. %v", err)
		return nil, err
	}
	if err := startMCPServerCache(ctx, cacheMgr.MCPServer(), cacheMgr.GetUpdateCacheInterval()); err != nil {
		commonlog.Errorf("start mcp-server cache error. %v", err)
		return nil, err
	}
	policySvr, err := authapi.GetStrategyServer()
	if err != nil {
		commonlog.Errorf("set policy server to ai-mcp server error. %v", err)
		return nil, err
	}

	mcpSvr := server.NewMCPServer("pole.io", version.Get(),
		server.WithResourceCapabilities(true, true),
		server.WithPromptCapabilities(true),
		server.WithLogging(),
	)
	sseSvr := server.NewSSEServer(mcpSvr,
		server.WithBasePath(basePath),
		server.WithSSEContextFunc(func(ctx context.Context, r *http.Request) context.Context {
			newheaders := make(http.Header)
			for k, v := range r.Header {
				if len(v) == 0 {
					continue
				}
				if k == types.HeaderAuthorizationKey {
					v[0] = strings.TrimPrefix(v[0], "Bearer ")
				}
				newheaders[k] = v
			}
			if log.DebugEnabled() {
				log.Debug("[apiserver][ai-mcp] sse rewrite header", zap.Any("origin", r.Header), zap.Any("new", newheaders))
			}
			return types.AppendRequestHeader(ctx, newheaders)
		}),
		server.WithSSEEndpoint(sseEp),
		server.WithMessageEndpoint(msgEp),
	)

	return &HTTPServer{
		maintainServer:  maintainServer,
		namespaceServer: namespaceServer,
		configServer:    configServer,
		discoverySvr:    discoverySvr,
		policySvr:       policySvr,
		storage:         storage,
		cacheMgr:        cacheMgr,
		mcpSvr:          mcpSvr,
		sseSvr:          sseSvr,
		selfProbeKey:    configuredSelfCapabilityProbeKey(),
	}, nil
}

func startMCPServerCache(ctx context.Context, mcpCache cacheapi.MCPServerCache, interval time.Duration) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if interval <= 0 {
		interval = time.Second
	}

	if err := mcpCache.Update(); err != nil {
		return err
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := mcpCache.Update(); err != nil {
					commonlog.Warnf("update mcp-server cache error. %v", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

// GetConfigAccessServer 获取配置中心接口
func (h *HTTPServer) GetMCPAccessServer(include []string) *restful.WebService {
	commonlog.Info("enable ai-mcp access server")

	h.addMcpTools()
	consoleAccess := []string{defaultAccess}

	ws := new(restful.WebService)
	ws.Path(basePath).Consumes(restful.MIME_JSON, "multipart/form-data", "text/event-stream").Produces(restful.MIME_JSON, "text/event-stream", "application/zip")

	if len(include) == 0 {
		include = consoleAccess
	}

	for _, item := range include {
		switch item {
		case aimcpAccess, defaultAccess:
			h.addDefaultAccess(ws)
		}
	}

	return ws
}

func (h *HTTPServer) addMcpTools() {
	h.addToolsNamespace(h.mcpSvr)
	h.addToolsMCPServer(h.mcpSvr)
	h.addToolsConfigState(h.mcpSvr)
}

func (h *HTTPServer) addDefaultAccess(ws *restful.WebService) {
	ws.Route(ws.POST("/self-capabilities/probe").To(h.ProbeSelfCapabilitiesEndpoint))

	// MCP registry console handlers
	ws.Route(ws.GET("/servers").To(h.ListMCPServers))
	ws.Route(ws.POST("/servers").To(h.CreateMCPServers))
	ws.Route(ws.PUT("/servers").To(h.UpdateMCPServers))
	ws.Route(ws.POST("/servers/delete").To(h.DeleteMCPServers))
	ws.Route(ws.GET("/server/tools").To(h.ListMCPServerTools))
	ws.Route(ws.GET("/definitions").To(h.ListMCPServerDefinitions))
	ws.Route(ws.POST("/definitions").To(h.CreateMCPServerDefinitions))
	ws.Route(ws.PUT("/definitions").To(h.UpdateMCPServerDefinitions))
	ws.Route(ws.POST("/definitions/delete").To(h.DeleteMCPServerDefinitions))
	ws.Route(ws.GET("/definitions/environments").To(h.ListMCPServerDefinitionEnvironments))
	ws.Route(ws.GET("/definitions/environment-binding").To(h.GetMCPServerDefinitionBinding))
	ws.Route(ws.POST("/definitions/environment-bindings").To(h.BindMCPServerDefinitionEnvironment))
	ws.Route(ws.POST("/definitions/environment-bindings/delete").To(h.UnbindMCPServerDefinitionEnvironment))

	// MCP sse handler
	ws.Route(ws.GET(sseEp).To(func(req *restful.Request, rsp *restful.Response) {
		h.sseSvr.ServeHTTP(rsp, req.Request)
	}))
	ws.Route(ws.POST(sseEp).To(func(req *restful.Request, rsp *restful.Response) {
		h.sseSvr.ServeHTTP(rsp, req.Request)
	}))

	// MCP message handler
	ws.Route(ws.GET(msgEp).To(func(req *restful.Request, rsp *restful.Response) {
		h.sseSvr.ServeHTTP(rsp, req.Request)
	}))
	ws.Route(ws.POST(msgEp).To(func(req *restful.Request, rsp *restful.Response) {
		h.sseSvr.ServeHTTP(rsp, req.Request)
	}))
}

func (h *HTTPServer) ProbeSelfCapabilitiesEndpoint(req *restful.Request, rsp *restful.Response) {
	timestamp := req.Request.Header.Get(selfmanager.ProbeTimestampHeader)
	signature := req.Request.Header.Get(selfmanager.ProbeSignatureHeader)
	if timestamp == "" || signature == "" {
		_ = rsp.WriteErrorString(http.StatusUnauthorized, "machine identity is required")
		return
	}
	req.Request.Body = http.MaxBytesReader(rsp, req.Request.Body, 64<<10)
	bodyBytes, err := io.ReadAll(req.Request.Body)
	if err != nil {
		_ = rsp.WriteErrorString(http.StatusRequestEntityTooLarge, "capability probe request is too large")
		return
	}
	if err := selfmanager.VerifyCapabilityProbe(h.selfProbeKey, req.Request.Method,
		req.Request.URL.Path, bodyBytes, timestamp, signature, time.Now()); err != nil {
		_ = rsp.WriteErrorString(http.StatusForbidden, "invalid machine identity")
		return
	}
	var body struct {
		Allowlist []string `json:"allowlist"`
	}
	if err := json.Unmarshal(bodyBytes, &body); err != nil || len(body.Allowlist) > 256 {
		_ = rsp.WriteErrorString(http.StatusBadRequest, "invalid capability probe request")
		return
	}
	if err := h.ProbeSelfCapabilities(req.Request.Context(), body.Allowlist); err != nil {
		_ = rsp.WriteErrorString(http.StatusServiceUnavailable, "Pole MCP self-capabilities are not ready")
		return
	}
	rsp.WriteHeader(http.StatusNoContent)
}
