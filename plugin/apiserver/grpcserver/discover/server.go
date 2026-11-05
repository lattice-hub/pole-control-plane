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

package discover

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	apiconfig "github.com/pole-io/specification/source/go/api/v1/config_manage"
	apimodel "github.com/pole-io/specification/source/go/api/v1/model"
	apisecurity "github.com/pole-io/specification/source/go/api/v1/security"
	apiservice "github.com/pole-io/specification/source/go/api/v1/service_manage"

	"github.com/pole-io/pole-server/apis/apiserver"
	authcommon "github.com/pole-io/pole-server/apis/pkg/types/auth"
	commonlog "github.com/pole-io/pole-server/pkg/common/log"
	"github.com/pole-io/pole-server/pkg/config"
	"github.com/pole-io/pole-server/pkg/goverrule"
	"github.com/pole-io/pole-server/pkg/service"
	"github.com/pole-io/pole-server/pkg/service/healthcheck"
	"github.com/pole-io/pole-server/pkg/workloadcredential"
	"github.com/pole-io/pole-server/plugin/apiserver/grpcserver"
	v1 "github.com/pole-io/pole-server/plugin/apiserver/grpcserver/discover/v1"
	"github.com/pole-io/pole-server/plugin/apiserver/grpcserver/utils"
)

var (
	namingLog = commonlog.GetScopeOrDefaultByName(commonlog.NamingLoggerName)

	cacheTypes = map[string]struct{}{
		apiservice.DiscoverResponse_INSTANCE.String():              {},
		apiservice.DiscoverResponse_CUSTOM_ROUTE_RULE.String():     {},
		apiservice.DiscoverResponse_RATE_LIMIT.String():            {},
		apiservice.DiscoverResponse_CIRCUIT_BREAKER.String():       {},
		apiservice.DiscoverResponse_FAULT_DETECTOR.String():        {},
		apiservice.DiscoverResponse_SERVICES.String():              {},
		apiservice.DiscoverResponse_TRAFFIC_SECURITY_RULE.String(): {},
		apiservice.DiscoverResponse_TRAFFIC_MIRROR_RULE.String():   {},
		apiservice.DiscoverResponse_TRAFFIC_MOCK_RULE.String():     {},
	}
)

// GRPCServer GRPC API服务器
type GRPCServer struct {
	grpcserver.BaseGrpcServer
	namingServer      service.DiscoverServer
	ruleServer        goverrule.GoverRuleServer
	healthCheckServer *healthcheck.Server
	openAPI           map[string]apiserver.APIConfig

	dsvr *v1.DiscoverGRPCServer
	csvr *v1.ConfigGRPCServer
}

// GetPort 获取端口
func (g *GRPCServer) GetPort() uint32 {
	return g.BaseGrpcServer.GetPort()
}

// GetProtocol 获取Server的协议
func (g *GRPCServer) GetProtocol() string {
	return "grpc"
}

// Initialize 初始化GRPC API服务器
func (g *GRPCServer) Initialize(ctx context.Context, option map[string]interface{},
	apiConf map[string]apiserver.APIConfig) error {
	g.openAPI = apiConf
	if err := g.BaseGrpcServer.Initialize(ctx, option, g.buildInitOptions(option)...); err != nil {
		return err
	}

	// 引入功能模块和插件
	var err error
	if g.namingServer, err = service.GetServer(); err != nil {
		namingLog.Errorf("%v", err)
		return err
	}

	if g.healthCheckServer, err = healthcheck.GetServer(); err != nil {
		namingLog.Errorf("%v", err)
		return err
	}

	if g.ruleServer, err = goverrule.GetServer(); err != nil {
		namingLog.Errorf("%v", err)
		return err
	}

	g.dsvr = v1.NewDiscoverGRPCServer(
		v1.WithDAllowAccess(g.allowAccess),
		v1.WithDEnterRateLimit(g.enterRateLimit),
		v1.WithHealthCheckerServer(g.healthCheckServer),
		v1.WithGoverRuleServer(g.ruleServer),
		v1.WithNamingServer(g.namingServer),
		v1.WithWorkloadCredentialServer(workloadcredential.GetServer()),
	)
	unifiedService, _ := config.GetUnifiedService()
	g.csvr = v1.NewConfigGRPCServer(
		v1.WithUnifiedConfigServer(unifiedService),
		v1.WithCAllowAccess(g.allowAccess),
		v1.WithCEnterRateLimit(g.enterRateLimit),
	)
	return nil
}

// Run 启动GRPC API服务器
func (g *GRPCServer) Run(errCh chan error) {
	g.BaseGrpcServer.Run(errCh, g.GetProtocol(), func(server *grpc.Server) error {
		g.registerServices(server)
		for name, config := range g.openAPI {
			switch name {
			case "client":
				if config.Enable {
					// 注册 v1 版本的 spec discover server
					openMethod, getErr := utils.GetDiscoverClientOpenMethod(config.Include, g.GetProtocol())
					if getErr != nil {
						return getErr
					}
					g.BaseGrpcServer.OpenMethod = openMethod
				}
			default:
				namingLog.Errorf("[Grpc][Discover] api %s does not exist in grpcserver", name)
				return fmt.Errorf("api %s does not exist in grpcserver", name)
			}
		}
		return nil
	})
}

func (g *GRPCServer) registerServices(server *grpc.Server) {
	apiservice.RegisterDiscoverGRPCServer(server, g.dsvr)
	apiservice.RegisterPoleHeartbeatGRPCServer(server, g.dsvr)
	apisecurity.RegisterWorkloadCredentialServiceServer(server, g.dsvr)
	if g.configClientEnabled() {
		apiconfig.RegisterConfigGRPCServer(server, g.csvr)
	}
}

// configClientEnabled 在未开启客户端配置 API 时不注册 ConfigGRPC，避免空方法表默认放行。
func (g *GRPCServer) configClientEnabled() bool {
	client := g.openAPI["client"]
	if !client.Enable {
		return false
	}
	if len(client.Include) == 0 {
		return true
	}
	for _, item := range client.Include {
		if item == apiserver.ConfigAccess {
			return true
		}
	}
	return false
}

// Stop 关闭GRPC
func (g *GRPCServer) Stop() {
	g.BaseGrpcServer.Stop(g.GetProtocol())
}

// Restart 重启Server
func (g *GRPCServer) Restart(option map[string]interface{}, api map[string]apiserver.APIConfig,
	errCh chan error) error {
	initFunc := func() error {
		return g.Initialize(context.Background(), option, api)
	}
	runFunc := func() {
		g.Run(errCh)
	}
	return g.BaseGrpcServer.Restart(initFunc, runFunc, g.GetProtocol(), option)
}

// enterRateLimit 限流
func (g *GRPCServer) enterRateLimit(ip string, method string) uint32 {
	return g.BaseGrpcServer.EnterRatelimit(ip, method)
}

// allowAccess 限制访问
func (g *GRPCServer) allowAccess(method string) bool {
	return g.BaseGrpcServer.AllowAccess(method)
}

func (g *GRPCServer) buildInitOptions(option map[string]interface{}) []grpcserver.InitOption {
	initOptions := []grpcserver.InitOption{
		grpcserver.WithModule(authcommon.DiscoverModule),
		grpcserver.WithProtocol(g.GetProtocol()),
		grpcserver.WithLogger(commonlog.FindScope(commonlog.APIServerLoggerName)),
		grpcserver.WithMessageToCacheObject(discoverCacheConvert),
	}

	types := make([]string, 0, len(cacheTypes))
	for k := range cacheTypes {
		types = append(types, k)
	}

	cache, err := grpcserver.NewCache(option, types)
	if err != nil {
		namingLog.Warn("[Grpc][Discover] new protobuf cache", zap.Error(err))
	}

	if cache != nil {
		initOptions = append(initOptions, grpcserver.WithProtobufCache(cache))
	}

	return initOptions
}

// discoverCacheConvert 将 DiscoverResponse 转换为 grpcserver.CacheObject
func discoverCacheConvert(m interface{}) *grpcserver.CacheObject {
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

	return &grpcserver.CacheObject{
		OriginVal: resp,
		CacheType: resp.Type.String(),
		Key:       keyProto,
	}
}
