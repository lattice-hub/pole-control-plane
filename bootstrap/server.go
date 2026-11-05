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

package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"

	apiservice "github.com/pole-io/specification/source/go/api/v1/service_manage"

	"github.com/pole-io/pole-server/apis/access_control/auth"
	"github.com/pole-io/pole-server/apis/apiserver"
	cacheapi "github.com/pole-io/pole-server/apis/cache"
	"github.com/pole-io/pole-server/apis/pkg/types"
	authtypes "github.com/pole-io/pole-server/apis/pkg/types/auth"
	svctypes "github.com/pole-io/pole-server/apis/pkg/types/service"
	storeapi "github.com/pole-io/pole-server/apis/store"
	boot_config "github.com/pole-io/pole-server/bootstrap/config"
	"github.com/pole-io/pole-server/pkg/admin"
	"github.com/pole-io/pole-server/pkg/cache"
	api "github.com/pole-io/pole-server/pkg/common/api/v1"
	"github.com/pole-io/pole-server/pkg/common/log"
	"github.com/pole-io/pole-server/pkg/common/utils"
	"github.com/pole-io/pole-server/pkg/common/utils/valid"
	"github.com/pole-io/pole-server/pkg/common/version"
	config_center "github.com/pole-io/pole-server/pkg/config"
	"github.com/pole-io/pole-server/pkg/goverrule"
	"github.com/pole-io/pole-server/pkg/namespace"
	"github.com/pole-io/pole-server/pkg/service"
	"github.com/pole-io/pole-server/pkg/service/batch"
	"github.com/pole-io/pole-server/pkg/service/healthcheck"
	"github.com/pole-io/pole-server/pkg/workloadcredential"
)

var (
	SelfServiceInstance = make([]*apiservice.Instance, 0)
	ConfigFilePath      = ""
	selfHeathChecker    *SelfHeathChecker
)

// Start 是保留给旧调用方的兼容入口。新调用方应使用 Run 并传入进程 Context。
func Start(configFilePath string, modeOverride ...string) {
	override := ""
	if len(modeOverride) > 0 {
		override = modeOverride[0]
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := Run(ctx, Options{
		ConfigPath:   configFilePath,
		ModeOverride: override,
	}); err != nil {
		fmt.Printf("[ERROR] start process: %v\n", err)
	}
}

func ensureA2AAdvertisedEndpoint(cfg *boot_config.Config, host string) {
	if cfg == nil || strings.TrimSpace(cfg.Bootstrap.Console.Agent.A2A.Endpoint) != "" {
		return
	}
	host = strings.TrimSpace(host)
	if host == "" {
		host = "127.0.0.1"
	}
	port := cfg.Bootstrap.Console.WebServer.ListenPort
	if port <= 0 {
		port = 8080
	}
	cfg.Bootstrap.Console.Agent.A2A.Endpoint = "http://" +
		net.JoinHostPort(host, strconv.Itoa(port)) + "/ai/agent/a2a/v1"
}

// StartComponents start health check and naming components
func StartComponents(ctx context.Context, cfg *boot_config.Config) error {
	var err error

	// 获取存储层对象
	s, err := storeapi.GetStore()
	if err != nil {
		log.Errorf("[Naming][Server] can not get store, err: %s", err.Error())
		return errors.New("can not get store")
	}

	// 初始化缓存模块
	if err := cache.Initialize(ctx, &cfg.Cache, s); err != nil {
		return err
	}

	cacheMgn, err := cache.GetCacheManager()
	if err != nil {
		return err
	}

	// 开启灰度规则缓存
	_ = cacheMgn.OpenResourceCache(cacheapi.ConfigEntry{
		Name: cacheapi.GrayName,
	})

	// 初始化鉴权层
	if err = auth.Initialize(ctx, &cfg.Auth, s, cacheMgn); err != nil {
		return err
	}

	userMgn, err := auth.GetUserServer()
	if err != nil {
		return err
	}

	strategyMgn, err := auth.GetStrategyServer()
	if err != nil {
		return err
	}

	// 初始化命名空间模块
	if err := namespace.Initialize(ctx, &cfg.Namespace, s, cacheMgn); err != nil {
		return err
	}

	// 初始化注册发现模块相关功能
	if err := StartDiscoverComponents(ctx, cfg, s, cacheMgn); err != nil {
		return err
	}

	// 初始化治理规则模块相关功能
	if err := StartGoverRuleComponents(ctx, cfg, s, cacheMgn); err != nil {
		return err
	}

	// 初始化配置中心模块相关功能
	if err := StartConfigCenterComponents(ctx, cfg, s, cacheMgn, userMgn, strategyMgn); err != nil {
		return err
	}

	namingSvr, err := service.GetServer()
	if err != nil {
		return err
	}
	healthCheckServer, err := healthcheck.GetServer()
	if err != nil {
		return err
	}

	// 初始化运维操作模块
	if err := admin.Initialize(ctx, &cfg.Maintain, namingSvr, healthCheckServer, cacheMgn, s,
		userMgn, strategyMgn); err != nil {
		return err
	}

	// 最后启动 cache
	if err := cache.Run(cacheMgn, ctx); err != nil {
		return err
	}
	return nil
}

func StartDiscoverComponents(ctx context.Context, cfg *boot_config.Config, s storeapi.Store,
	cacheMgn *cache.CacheManager) error {
	if err := workloadcredential.Initialize(cfg.WorkloadCredential, s); err != nil {
		return fmt.Errorf("initialize workload credential issuer: %w", err)
	}
	// 批量控制器
	namingBatchConfig, err := batch.ParseBatchConfig(cfg.Naming.Batch)
	if err != nil {
		return err
	}
	healthBatchConfig, err := batch.ParseBatchConfig(cfg.Naming.HealthChecks.Batch)
	if err != nil {
		return err
	}

	batchConfig := &batch.Config{
		Register:         namingBatchConfig.Register,
		Deregister:       namingBatchConfig.Deregister,
		ClientRegister:   namingBatchConfig.ClientRegister,
		ClientDeregister: namingBatchConfig.ClientDeregister,
		Heartbeat:        healthBatchConfig.Heartbeat,
	}

	bc, err := batch.NewBatchCtrlWithConfig(ctx, s, cacheMgn, batchConfig)
	if err != nil {
		log.Errorf("new batch ctrl with config err: %s", err.Error())
		return err
	}

	if len(cfg.Naming.HealthChecks.LocalHost) == 0 {
		cfg.Naming.HealthChecks.LocalHost = utils.LocalHost // 补充healthCheck的配置
	}
	if err = healthcheck.Initialize(ctx, &cfg.Naming.HealthChecks, bc); err != nil {
		return err
	}
	healthCheckServer, err := healthcheck.GetServer()
	if err != nil {
		return err
	}
	if cfg.Naming.HealthChecks.IsOpen() {
		healthCheckServer.SetServiceCache(cacheMgn.Service())
		healthCheckServer.SetInstanceCache(cacheMgn.Instance())
	}

	namespaceSvr, err := namespace.GetServer()
	if err != nil {
		return err
	}

	opts := []service.InitOption{
		service.WithBatchController(bc),
		service.WithStorage(s),
		service.WithCacheManager(&cfg.Cache, cacheMgn),
		service.WithHealthCheckSvr(healthCheckServer),
		service.WithNamespaceSvr(namespaceSvr),
	}

	cfg.Naming.Interceptors = service.GetChainOrder()
	// 初始化服务模块
	if err = service.Initialize(ctx, &cfg.Naming, opts...); err != nil {
		return err
	}

	if _, err = service.GetServer(); err != nil {
		return err
	}

	return nil
}

// StartGoverRuleComponents
func StartGoverRuleComponents(ctx context.Context, cfg *boot_config.Config, s storeapi.Store,
	cacheMgn *cache.CacheManager) error {
	// 批量控制器
	namespaceSvr, err := namespace.GetServer()
	if err != nil {
		return err
	}

	opts := []goverrule.InitOption{
		goverrule.WithStorage(s),
		goverrule.WithCacheManager(&cfg.Cache, cacheMgn),
		goverrule.WithNamespaceSvr(namespaceSvr),
	}

	cfg.GoverRule.Interceptors = goverrule.GetChainOrder()
	// 初始化治理规则模块
	if err = goverrule.Initialize(ctx, &cfg.GoverRule, opts...); err != nil {
		return err
	}

	if _, err = goverrule.GetServer(); err != nil {
		return err
	}
	return nil
}

func parseConfDir(path string) string {
	slashIndex := strings.LastIndex(path, "/")
	if slashIndex == -1 {
		return "./"
	}
	return path[0 : slashIndex+1]
}

// StartConfigCenterComponents 启动配置中心模块
func StartConfigCenterComponents(ctx context.Context, cfg *boot_config.Config, s storeapi.Store,
	cacheMgn *cache.CacheManager, userMgn auth.UserServer, strategyMgn auth.StrategyServer) error {
	return config_center.Initialize(ctx, cfg.Config, s, cacheMgn, userMgn, strategyMgn.GetAuthChecker())
}

// StartServers 启动server
func StartServers(ctx context.Context, apientries []apiserver.Config, errCh chan error) (
	[]apiserver.Apiserver, error) {
	runtimeSlots := make(map[string]apiserver.Apiserver, len(apientries))
	for _, protocol := range apientries {
		if !apiserver.Registered(protocol.Name) {
			continue
		}
		server, err := apiserver.Resolve(protocol.Name)
		if err != nil {
			return nil, fmt.Errorf("resolve api server plugin %s: %w", protocol.Name, err)
		}
		runtimeSlots[protocol.Name] = server
	}
	apiserver.ReplaceRuntimeSlots(runtimeSlots)

	var servers []apiserver.Apiserver

	for _, protocol := range apientries {
		slot, exist := runtimeSlots[protocol.Name]
		if !exist {
			log.Warn("[ERROR] apiserver slot not exists", zap.String("name", protocol.Name))
			continue
		}
		// 如果是 http server, 注入所有的 apiserver 实例
		if protocol.Name == "api-http" {
			ctx = context.WithValue(ctx, utils.ContextAPIServerSlot{}, runtimeSlots)
		}

		err := slot.Initialize(ctx, protocol.Option, protocol.API)
		if err != nil {
			StopServers(servers)
			fmt.Printf("[ERROR] %v\n", err)
			return nil, fmt.Errorf("apiserver %s initialize err: %s", protocol.Name, err.Error())
		}

		servers = append(servers, slot)
		go slot.Run(errCh)
	}
	return servers, nil
}

// RestartServers 重启server
func RestartServers(errCh chan error) error {
	// 重新加载配置
	cfg, err := boot_config.Load(ConfigFilePath)
	if err != nil {
		log.Infof("restart servers, reload config")
		return err
	}
	log.Infof("new config: %+v", cfg)
	// TODO: 配置的动态加载后续统一设计
	return nil
}

// StopServers 接受外部信号，停止server
func StopServers(servers []apiserver.Apiserver) {
	// stop health checkers
	if nil != selfHeathChecker {
		selfHeathChecker.Stop()
	}
	// Deregistration uses the naming service batch controllers. Keep the API and
	// service components alive until the asynchronous delete has completed.
	SelfDeregister()
	// sync stop servers
	wg := &sync.WaitGroup{}
	for _, s := range servers {
		wg.Add(1)
		go func(s apiserver.Apiserver, wg *sync.WaitGroup) {
			defer wg.Done()
			log.Infof("start stop server protocol: %s", s.GetProtocol())
			s.Stop()
			log.Infof("complete stop server protocol: %s", s.GetProtocol())
		}(s, wg)
	}
	wg.Wait()
}

// StartBootstrapInOrder 开始进入启动加锁
// 原因：Server启动的时候会从数据库拉取大量数据，防止同时启动把DB压死
// 还有一种场景，server全部宕机批量重启，导致数据库被压死，导致雪崩
func StartBootstrapInOrder(s storeapi.Store, c *boot_config.Config) (storeapi.Transaction, error) {
	order := c.Bootstrap.StartInOrder
	log.Infof("[Bootstrap] get bootstrap order config: %+v", order)
	open, _ := order["open"].(bool)
	key, _ := order["key"].(string)
	if !open || key == "" {
		log.Infof("[Bootstrap] start in order config is not open or key is null")
		return nil, nil
	}

	log.Infof("bootstrap start in order with key: %s", key)

	// 启动一个日志协程，当等锁的时候，可以看到server正在等待锁
	stopCh := make(chan struct{})
	defer close(stopCh) // 函数退出的时候，关闭stopCh
	go func() {
		ticker := time.NewTicker(time.Second * 10)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				log.Infof("bootstrap waiting the lock")
			case <-stopCh:
				return
			}
		}
	}()

	// 重试多次
	maxTimes := 10
	for i := 0; i < maxTimes; i++ {
		tx, err := s.CreateTransaction()
		if err != nil {
			log.Errorf("create transaction err: %v", err)
			return nil, err
		}
		// 这里可能会出现锁超时，超时则重试
		if err := tx.LockBootstrap(key, utils.LocalHost); err != nil {
			log.Errorf("lock bootstrap err: %s", err.Error())
			_ = tx.Commit()
			continue
		}
		// 加锁成功，直接返回
		log.Infof("lock bootstrap success")
		return tx, nil
	}

	return nil, errors.New("lock bootstrap error")
}

// FinishBootstrapOrder 完成 提交锁
func FinishBootstrapOrder(tx storeapi.Transaction) error {
	if tx != nil {
		return tx.Commit()
	}

	return nil
}

func genContext() context.Context {
	ctx := context.Background()
	reqCtx := context.WithValue(context.Background(), types.ContextAuthTokenKey, "")
	ctx = context.WithValue(ctx, types.ContextRequestId, fmt.Sprintf("self-%d", time.Now().Nanosecond()))
	ctx = context.WithValue(ctx, types.ContextAuthContextKey,
		authtypes.NewAcquireContext(
			authtypes.WithOperation(authtypes.Read),
			authtypes.WithModule(authtypes.BootstrapModule),
			authtypes.WithRequestContext(reqCtx)))
	return ctx
}

// acquireLocalhost 探测获取本机IP地址
func acquireLocalhost(ctx context.Context, polarisService *boot_config.PolarisService) (context.Context, error) {
	if polarisService == nil || !polarisService.EnableRegister {
		log.Infof("[Bootstrap] polaris service config not found")
		return ctx, nil
	}
	if len(polarisService.SelfAddress) != 0 {
		utils.LocalHost = polarisService.SelfAddress
		return utils.WithLocalhost(ctx, utils.LocalHost), nil
	}
	if len(polarisService.NetworkInter) != 0 {
		netInter, err := net.InterfaceByName(polarisService.NetworkInter)
		if err != nil {
			log.Errorf("get local host by network_interface: %s err: %s", polarisService.NetworkInter, err.Error())
			return nil, err
		}
		addrs, err := netInter.Addrs()
		if err != nil {
			log.Errorf("get local host by network_interface: %s err: %s", polarisService.NetworkInter, err.Error())
			return nil, err
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					utils.LocalHost = ipnet.IP.String()
					return utils.WithLocalhost(ctx, utils.LocalHost), nil
				}
			}
		}
	}

	localHost, err := getLocalHost(polarisService.ProbeAddress)
	if err != nil {
		log.Errorf("get local host err: %s", err.Error())
		return nil, err
	}
	log.Infof("[Bootstrap] get local host: %s", localHost)
	utils.LocalHost = localHost
	return utils.WithLocalhost(ctx, utils.LocalHost), nil
}

func acquireLocalPort(ctx context.Context, apientries []apiserver.Config) {
	for i := range apientries {
		entry := apientries[i]
		if entry.Name != "service-grpc" {
			continue
		}
		port, _ := entry.Option["listenPort"].(int)
		utils.LocalPort = port
		break
	}
}

// polarisServiceRegister 自注册主函数
func polarisServiceRegister(polarisService *boot_config.PolarisService, apiServers []apiserver.Config) error {
	if polarisService == nil || !polarisService.EnableRegister {
		log.Infof("[Bootstrap] not enable register the polaris service")
		return nil
	}

	apiServerNames := make(map[string]bool)
	for _, server := range apiServers {
		apiServerNames[server.Name] = true
	}
	runtimeSlots := apiserver.RuntimeSlots()
	hbInterval := boot_config.DefaultHeartbeatInterval
	if polarisService.HeartbeatInterval > 0 {
		hbInterval = polarisService.HeartbeatInterval
	}
	// 开始注册每个服务
	for _, svc := range polarisService.Services {
		protocols := svc.Protocols
		// 如果service.Protocols为空，默认采用apiServers的protocols注册，实际为配置中的Name字段,
		// 如：grpcserver, httpserver, xdsserverv3，也隐式表达了协议的意思
		if len(protocols) == 0 {
			for _, server := range apiServers {
				protocols = append(protocols, server.Name)
			}
		}
		for _, name := range protocols {
			if _, exist := apiServerNames[name]; !exist {
				return fmt.Errorf("server(%s) not registered", name)
			}
			slot, exist := runtimeSlots[name]
			if !exist {
				return fmt.Errorf("server(%s) not supported", name)
			}
			host := utils.LocalHost
			port := slot.GetPort()
			protocol := slot.GetProtocol()
			if err := selfRegister(host, port, protocol, polarisService.Isolated, svc, hbInterval); err != nil {
				log.Errorf("self register err: %s", err.Error())
				return err
			}

			log.Infof("self register success. host = %s, port = %d, protocol = %s, service = %s",
				host, port, protocol, svc)
		}
	}
	if len(SelfServiceInstance) > 0 && !polarisService.DisableHeartbeat {
		log.Infof("start self health checker")
		var err error
		if selfHeathChecker, err = NewSelfHeathChecker(SelfServiceInstance, hbInterval); nil != err {
			log.Errorf("self health checker err: %s", err.Error())
			return err
		}
		go selfHeathChecker.Start()
	}
	return nil
}

// selfRegister 服务自注册
func selfRegister(
	host string, port uint32, protocol string, isolated bool, polarisService *boot_config.Service, hbInterval int) error {
	server, err := service.GetServer()
	if err != nil {
		return err
	}

	name := boot_config.DefaultPolarisName
	polarisNamespace := boot_config.DefaultPolarisNamespace
	if polarisService.Name != "" {
		name = polarisService.Name
	}

	if polarisService.Namespace != "" {
		polarisNamespace = polarisService.Namespace
	}

	metadata := polarisService.Metadata
	if len(metadata) == 0 {
		metadata = make(map[string]string)
	}
	metadata[types.MetaKeyBuildRevision] = version.GetRevision()
	metadata[types.MetaKeyPolarisService] = name

	req := &apiservice.Instance{
		Service:           name,
		Namespace:         polarisNamespace,
		Host:              host,
		Port:              port,
		Protocol:          protocol,
		Version:           selfRegistrationVersion(version.Get()),
		EnableHealthCheck: true,
		Isolate:           isolated,
		HealthCheck: &apiservice.HealthCheck{
			Type: apiservice.HealthCheck_HEARTBEAT,
			Heartbeat: &apiservice.HeartbeatHealthCheck{
				Ttl: uint32(hbInterval),
			},
		},
		Metadata: metadata,
	}

	resp := server.RegisterInstance(genContext(), req)
	if api.CalcCode(resp) != 200 {
		// 如果self之前注册过，那么可以忽略
		if resp.GetCode() != api.ExistedResource {
			return fmt.Errorf(
				"register instance service=%q namespace=%q host=%q port=%d protocol=%q: %s",
				req.GetService(), req.GetNamespace(), req.GetHost(), req.GetPort(), req.GetProtocol(), resp.GetInfo())
		}

		resp = server.UpdateInstance(genContext(), req)
		if api.CalcCode(resp) != 200 {
			return fmt.Errorf(
				"update instance service=%q namespace=%q host=%q port=%d protocol=%q: %s",
				req.GetService(), req.GetNamespace(), req.GetHost(), req.GetPort(), req.GetProtocol(), resp.GetInfo())
		}
	}
	SelfServiceInstance = append(SelfServiceInstance, req)

	return nil
}

func selfRegistrationVersion(value string) string {
	runes := []rune(value)
	if len(runes) <= valid.MaxDbInsVersionLength {
		return value
	}
	return string(runes[:valid.MaxDbInsVersionLength])
}

// SelfDeregister Server退出的时候，自动反注册
func SelfDeregister() {
	namingServer, err := service.GetOriginServer()
	if err != nil {
		log.Errorf("get naming server obj err: %s", err.Error())
		return
	}
	for _, req := range SelfServiceInstance {
		log.Infof("Deregister the instance(%+v)", req)
		if resp := namingServer.ShutdownDeregisterInstance(genContext(), req); api.CalcCode(resp) != 200 {
			// 遇到失败，继续反注册其他的实例
			log.Errorf("Deregister instance error: %s", resp.GetInfo())
		}
	}
	// wait the async event handler to finish
	time.Sleep(5 * time.Second)
}

// getLocalHost 获取本地IP地址
func getLocalHost(addr string) (string, error) {
	if len(addr) == 0 {
		return "127.0.0.1", nil
	}
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = conn.Close()
	}()

	localAddr := conn.LocalAddr().String() // ip:port
	segs := strings.Split(localAddr, ":")
	if len(segs) != 2 {
		return "", errors.New("get local address format is invalid")
	}

	return segs[0], nil
}

// getSelfRegisterPolarsServiceKeySet 获取自注册的系统服务集合
func getSelfRegisterPolarsServiceKeySet(polarisServiceCfg *boot_config.PolarisService) map[svctypes.ServiceKey]struct{} {
	if polarisServiceCfg == nil {
		return nil
	}
	polarisServiceSet := make(map[svctypes.ServiceKey]struct{})
	for _, svc := range polarisServiceCfg.Services {
		ns, n := svc.Namespace, svc.Name
		if ns == "" {
			ns = boot_config.DefaultPolarisNamespace
		}
		polarisServiceSet[svctypes.ServiceKey{Namespace: ns, Name: n}] = struct{}{}
	}
	return polarisServiceSet
}
