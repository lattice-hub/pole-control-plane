package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pole-io/pole-server/apis"
	"github.com/pole-io/pole-server/apis/apiserver"
	"github.com/pole-io/pole-server/apis/observability/statis"
	storeapi "github.com/pole-io/pole-server/apis/store"
	bootconfig "github.com/pole-io/pole-server/bootstrap/config"
	"github.com/pole-io/pole-server/internal/runtime/supervisor"
	"github.com/pole-io/pole-server/pkg/common/eventhub"
	"github.com/pole-io/pole-server/pkg/common/log"
	"github.com/pole-io/pole-server/pkg/common/otel/metrics"
	"github.com/pole-io/pole-server/pkg/common/utils"
	"github.com/pole-io/pole-server/pkg/compatibility"
	"github.com/pole-io/pole-server/pkg/console"
	"github.com/pole-io/pole-server/pkg/limiter"
	"github.com/pole-io/pole-server/pkg/selfmanager"
	"github.com/pole-io/pole-server/pkg/systemconfig"
	aimcpserver "github.com/pole-io/pole-server/plugin/apiserver/httpserver/aimcp"
	"github.com/pole-io/pole-server/pluginapi"
)

// Options 是统一进程入口的启动参数。
type Options struct {
	ConfigPath     string
	ModeOverride   string
	PluginRegistry *pluginapi.Registry
}

func compatibilityTargetsFromAPIEntries(entries []apiserver.Config) (string, string) {
	nacosTarget, eurekaTarget := "dev", "dev"
	for _, entry := range entries {
		switch entry.Name {
		case "service-nacos":
			if value, ok := entry.Option["defaultNamespace"].(string); ok && strings.TrimSpace(value) != "" {
				nacosTarget = strings.TrimSpace(value)
			}
		case "service-eureka":
			if value, ok := entry.Option["namespace"].(string); ok && strings.TrimSpace(value) != "" {
				eurekaTarget = strings.TrimSpace(value)
			}
		}
	}
	return nacosTarget, eurekaTarget
}

// Run 加载配置、解析 Profile，并把选中的运行模块交给 Supervisor。
func Run(ctx context.Context, options Options) (runErr error) {
	runtimeRegistry, err := preparePluginRegistry(options.PluginRegistry)
	if err != nil {
		return fmt.Errorf("prepare plugin registry: %w", err)
	}
	restoreRegistry, err := pluginapi.Activate(runtimeRegistry)
	if err != nil {
		return fmt.Errorf("activate plugin registry: %w", err)
	}
	defer restoreRegistry()
	defer func() {
		if err := runtimeRegistry.Close(); err != nil {
			runErr = errors.Join(runErr, fmt.Errorf("close plugin registry: %w", err))
		}
	}()

	ConfigFilePath = options.ConfigPath
	utils.ConfDir = parseConfDir(options.ConfigPath)
	cfg, err := bootconfig.Load(options.ConfigPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	selectedMode := strings.TrimSpace(options.ModeOverride)
	if selectedMode == "" {
		selectedMode = strings.TrimSpace(cfg.Bootstrap.Mode)
	}
	if strings.EqualFold(selectedMode, bootconfig.StartModeServer) {
		fmt.Println("[WARN] start mode \"server\" is deprecated; use \"control-plane\"")
	}
	profile, err := bootconfig.ResolveStartProfile(cfg.Bootstrap.Mode, options.ModeOverride)
	if err != nil {
		return err
	}
	cfg.Bootstrap.Mode = profile.Mode
	var controlPlaneEntries []apiserver.Config
	if profile.ControlPlane {
		controlPlaneEntries, err = bootconfig.LoadAPIEntries(cfg.APIServers)
		if err != nil {
			return fmt.Errorf("load api entries: %w", err)
		}
	}
	if profile.LimiterServer {
		if err := cfg.Limiter.Validate(); err != nil {
			return fmt.Errorf("validate limiter config: %w", err)
		}
	}
	if err := validateProfileListeners(profile, cfg, controlPlaneEntries); err != nil {
		return err
	}
	if strings.TrimSpace(options.ModeOverride) != "" {
		cfg.SystemConfigSources["bootstrap.mode"] = systemconfig.SourceDescriptor{
			Kind: systemconfig.SourceCommandLine, Reference: "--mode",
		}
	}
	fmt.Printf("[INFO] resolved start mode: %s\n", profile.Mode)

	if err := log.ConfigureFile(cfg.Bootstrap.Logger); err != nil {
		return fmt.Errorf("configure logger: %w", err)
	}
	nacosTarget, eurekaTarget := compatibilityTargetsFromAPIEntries(controlPlaneEntries)
	compatibilityResolver := compatibility.NewNamespaceResolver(nacosTarget, eurekaTarget)
	ctx = compatibility.WithResolver(ctx, compatibilityResolver)
	serverSettings, err := bootconfig.NewSystemSettingsProvider(cfg, compatibilityResolver)
	if err != nil {
		return fmt.Errorf("initialize system settings registry: %w", err)
	}
	ctx = systemconfig.WithProvider(ctx, systemconfig.ComponentServer, serverSettings)

	if profile.Console && !profile.ControlPlane {
		configureRemoteConsoleAgentCapabilityProbe(&cfg.Bootstrap.Console)
	}

	modules := make([]supervisor.Module, 0, 3)
	if profile.ControlPlane {
		modules = append(modules, &controlPlaneModule{
			cfg:         cfg,
			withConsole: profile.Console,
			apiEntries:  controlPlaneEntries,
		})
	}
	if profile.LimiterServer {
		modules = append(modules, &limiterModule{cfg: cfg.Limiter})
	}
	if profile.Console {
		modules = append(modules, &consoleModule{cfg: &cfg.Bootstrap.Console})
	}
	return (supervisor.Supervisor{
		Modules:         modules,
		ShutdownTimeout: 15 * time.Second,
	}).Run(ctx)
}

func preparePluginRegistry(registry *pluginapi.Registry) (*pluginapi.Registry, error) {
	if registry == nil {
		registry = pluginapi.DefaultRegistry()
	}
	runtimeRegistry := registry.Clone()
	if registry != pluginapi.DefaultRegistry() {
		if err := runtimeRegistry.Merge(pluginapi.DefaultRegistry()); err != nil {
			return nil, err
		}
	}
	runtimeRegistry.Freeze()
	return runtimeRegistry, nil
}

type controlPlaneModule struct {
	cfg         *bootconfig.Config
	withConsole bool
	apiEntries  []apiserver.Config
}

func (m *controlPlaneModule) Name() string {
	return bootconfig.StartModeControlPlane
}

func (m *controlPlaneModule) Start(ctx context.Context) (supervisor.Running, error) {
	return startControlPlane(ctx, m.cfg, m.apiEntries, m.withConsole)
}

type controlPlaneRunning struct {
	cancel   context.CancelFunc
	servers  []apiserver.Apiserver
	errCh    <-chan error
	stopOnce sync.Once
}

func (r *controlPlaneRunning) Wait() error {
	return <-r.errCh
}

func (r *controlPlaneRunning) Stop(context.Context) error {
	r.stopOnce.Do(func() {
		StopServers(r.servers)
		r.cancel()
	})
	return nil
}

func startControlPlane(parent context.Context, cfg *bootconfig.Config, apiEntries []apiserver.Config,
	withConsole bool) (*controlPlaneRunning, error) {
	ctx, cancel := context.WithCancel(parent)
	var servers []apiserver.Apiserver
	var tx storeapi.Transaction
	var err error
	rollback := func() {
		if len(servers) > 0 {
			StopServers(servers)
		}
		cancel()
		_ = FinishBootstrapOrder(tx)
	}

	ctx, err = acquireLocalhost(ctx, &cfg.Bootstrap.PolarisService)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("acquire localhost: %w", err)
	}
	acquireLocalPort(ctx, apiEntries)
	apis.SetPluginConfig(&cfg.Plugin)

	probeFallbackMasterKey := ""
	if withConsole {
		probeFallbackMasterKey = cfg.Bootstrap.Console.SystemSecrets.MasterKey
	}
	probeKey, probeKeyErr := selfmanager.ResolveCapabilityProbeKey(
		cfg.Bootstrap.Console.Agent.SelfManagementProbeKey,
		probeFallbackMasterKey)
	if probeKeyErr == nil {
		probeKeyErr = aimcpserver.ConfigureSelfCapabilityProbeKey(probeKey)
	}
	if probeKeyErr != nil {
		log.Warnf("Pole MCP split-mode capability probe is disabled: %v", probeKeyErr)
	}
	statis.GetStatis()
	metrics.InitMetrics()
	eventhub.InitEventHub()

	storeapi.SetStoreConfig(&cfg.Store)
	store, err := storeapi.GetStore()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("get store: %w", err)
	}
	tx, err = StartBootstrapInOrder(store, cfg)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("bootstrap in order: %w", err)
	}
	if err := StartComponents(ctx, cfg); err != nil {
		rollback()
		return nil, fmt.Errorf("start components: %w", err)
	}
	if resolver, ok := compatibility.ResolverFromContext(ctx); ok {
		if runtimeStore, supported := store.(storeapi.SystemConfigRuntimeStore); supported {
			runtime := compatibility.NewRuntime(resolver, runtimeStore, store)
			if reconcileErr := runtime.Reconcile(); reconcileErr != nil {
				log.Warnf("compatibility system configuration uses last-known-good: %v", reconcileErr)
			}
			go runtime.Run(ctx, time.Second)
		}
	}
	errCh := make(chan error, len(apiEntries)+1)
	servers, err = StartServers(ctx, apiEntries, errCh)
	if err != nil {
		rollback()
		return nil, fmt.Errorf("start api servers: %w", err)
	}
	if err := waitForReadyServers(ctx, servers, apiEntries, errCh); err != nil {
		rollback()
		return nil, err
	}
	if withConsole {
		ensureA2AAdvertisedEndpoint(cfg, utils.LocalHost)
		configureConsoleAgentCapabilityProbe(cfg, store, servers)
	}
	if err := StartSelfManagement(ctx, cfg, store, servers); err != nil {
		rollback()
		return nil, fmt.Errorf("start pole self manager: %w", err)
	}
	if err := polarisServiceRegister(&cfg.Bootstrap.PolarisService, apiEntries); err != nil {
		rollback()
		return nil, fmt.Errorf("register polaris service: %w", err)
	}
	finishErr := FinishBootstrapOrder(tx)
	tx = nil
	if finishErr != nil {
		rollback()
		return nil, fmt.Errorf("finish bootstrap order: %w", finishErr)
	}
	fmt.Println("finish starting control plane")
	return &controlPlaneRunning{
		cancel:  cancel,
		servers: servers,
		errCh:   errCh,
	}, nil
}

type listenerBinding struct {
	module string
	host   string
	port   uint32
}

func validateProfileListeners(
	profile bootconfig.StartProfile,
	cfg *bootconfig.Config,
	controlPlaneEntries []apiserver.Config,
) error {
	bindings := make([]listenerBinding, 0,
		len(controlPlaneEntries)+len(cfg.Limiter.APIServers)+1)
	for i, entry := range controlPlaneEntries {
		host, _ := entry.Option["listenIP"].(string)
		port, err := parseProfilePort(entry.Option["listenPort"])
		if err != nil {
			return fmt.Errorf("validate control-plane api-servers[%d] %q listener: %w",
				i, entry.Name, err)
		}
		bindings = append(bindings, listenerBinding{
			module: "control-plane/" + entry.Name,
			host:   normalizeListenHost(host),
			port:   port,
		})
	}
	if profile.LimiterServer {
		for i, entry := range cfg.Limiter.APIServers {
			host, port, err := limiter.ParseListenOption(entry.Option)
			if err != nil {
				return fmt.Errorf("validate limiter api-servers[%d] %q listener: %w",
					i, entry.Name, err)
			}
			bindings = append(bindings, listenerBinding{
				module: "limiter-server/" + entry.Name,
				host:   normalizeListenHost(host),
				port:   port,
			})
		}
	}
	if profile.Console {
		port, err := parseProfilePort(cfg.Bootstrap.Console.WebServer.ListenPort)
		if err != nil {
			return fmt.Errorf("validate console listener: %w", err)
		}
		bindings = append(bindings, listenerBinding{
			module: "console",
			host:   normalizeListenHost(cfg.Bootstrap.Console.WebServer.ListenIP),
			port:   port,
		})
	}

	for i := range bindings {
		if bindings[i].port == 0 {
			continue
		}
		for j := 0; j < i; j++ {
			if bindings[j].port != bindings[i].port ||
				!listenHostsOverlap(bindings[j].host, bindings[i].host) {
				continue
			}
			return fmt.Errorf("listener conflict: %s and %s both bind %s:%d",
				bindings[j].module, bindings[i].module,
				bindings[i].host, bindings[i].port)
		}
	}
	return nil
}

func parseProfilePort(value any) (uint32, error) {
	var port uint64
	switch typed := value.(type) {
	case int:
		if typed < 0 {
			return 0, fmt.Errorf("port must be between 0 and %d", math.MaxUint16)
		}
		port = uint64(typed)
	case int64:
		if typed < 0 {
			return 0, fmt.Errorf("port must be between 0 and %d", math.MaxUint16)
		}
		port = uint64(typed)
	case uint32:
		port = uint64(typed)
	case uint64:
		port = typed
	default:
		return 0, fmt.Errorf("port must be an integer")
	}
	if port > math.MaxUint16 {
		return 0, fmt.Errorf("port must be between 0 and %d", math.MaxUint16)
	}
	return uint32(port), nil
}

func normalizeListenHost(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return "0.0.0.0"
	}
	return host
}

func listenHostsOverlap(left, right string) bool {
	if left == right {
		return true
	}
	leftIP := net.ParseIP(strings.Trim(left, "[]"))
	rightIP := net.ParseIP(strings.Trim(right, "[]"))
	return leftIP != nil && leftIP.IsUnspecified() ||
		rightIP != nil && rightIP.IsUnspecified()
}

type readyApiserver interface {
	Ready() <-chan struct{}
}

const apiServerReadyTimeout = 15 * time.Second

func waitForReadyServers(
	ctx context.Context,
	servers []apiserver.Apiserver,
	apiEntries []apiserver.Config,
	errCh <-chan error) error {
	for _, server := range servers {
		readyCtx, cancel := context.WithTimeout(ctx, apiServerReadyTimeout)
		readyServer, ok := server.(readyApiserver)
		if ok {
			select {
			case <-readyServer.Ready():
			case err := <-errCh:
				cancel()
				return apiServerReadyError(err)
			case <-readyCtx.Done():
				cancel()
				return fmt.Errorf("api server %s readiness: %w",
					server.GetProtocol(), context.Cause(readyCtx))
			}
			cancel()
			continue
		}
		if err := waitForTCPListenerReady(
			readyCtx,
			server.GetProtocol(),
			readinessProbeHost(configuredListenHost(server, apiEntries)),
			server.GetPort(),
			errCh,
		); err != nil {
			cancel()
			return err
		}
		cancel()
	}
	return nil
}

func waitForTCPListenerReady(
	ctx context.Context,
	protocol string,
	host string,
	port uint32,
	errCh <-chan error,
) error {
	if port == 0 {
		return fmt.Errorf("api server %s readiness: listener port is zero", protocol)
	}
	address := net.JoinHostPort(host, strconv.FormatUint(uint64(port), 10))
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	dialer := net.Dialer{Timeout: 100 * time.Millisecond}
	for {
		conn, err := dialer.DialContext(ctx, "tcp", address)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		select {
		case runErr := <-errCh:
			return apiServerReadyError(runErr)
		case <-ticker.C:
		case <-ctx.Done():
			return fmt.Errorf("api server %s readiness: %w", protocol, context.Cause(ctx))
		}
	}
}

func configuredListenHost(
	server apiserver.Apiserver,
	apiEntries []apiserver.Config,
) string {
	runtimeSlots := apiserver.RuntimeSlots()
	for _, entry := range apiEntries {
		slot, exists := runtimeSlots[entry.Name]
		if !exists || slot != server {
			continue
		}
		host, _ := entry.Option["listenIP"].(string)
		return normalizeListenHost(host)
	}
	return "127.0.0.1"
}

func readinessProbeHost(configuredHost string) string {
	host := strings.Trim(strings.TrimSpace(configuredHost), "[]")
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsUnspecified() {
		return host
	}
	if ip.To4() != nil {
		return "127.0.0.1"
	}
	return "::1"
}

func apiServerReadyError(err error) error {
	if err == nil {
		err = fmt.Errorf("exited unexpectedly")
	}
	return fmt.Errorf("api server failed before ready: %w", err)
}

type limiterModule struct {
	cfg limiter.Config
}

func (m *limiterModule) Name() string {
	return bootconfig.StartModeLimiterServer
}

func (m *limiterModule) Start(ctx context.Context) (supervisor.Running, error) {
	return limiter.Start(ctx, m.cfg)
}

type consoleModule struct {
	cfg *console.Config
}

func (m *consoleModule) Name() string {
	return bootconfig.StartModeConsole
}

func (m *consoleModule) Start(ctx context.Context) (supervisor.Running, error) {
	return console.Start(ctx, m.cfg)
}
