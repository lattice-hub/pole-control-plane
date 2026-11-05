package limiter

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/pole-io/pole-server/pkg/limiter/internal/apiserver"
	limitergrpc "github.com/pole-io/pole-server/pkg/limiter/internal/apiserver/grpc"
	limiterhttp "github.com/pole-io/pole-server/pkg/limiter/internal/apiserver/http"
	"github.com/pole-io/pole-server/pkg/limiter/internal/ratelimitv2"
	"github.com/pole-io/pole-server/pkg/limiter/internal/statistics"
	"github.com/pole-io/pole-server/pkg/limiter/internal/statistics/echo"
	"github.com/pole-io/pole-server/pkg/limiter/internal/statistics/file"
)

const rollbackTimeout = 5 * time.Second

// Running 是已完成 listener 绑定和可选注册的 Limiter 实例。
type Running struct {
	cancel   context.CancelFunc
	servers  []apiserver.APIServer
	registry *registryClient
	statics  statistics.Statis
	done     chan error
	stopping chan struct{}
	stopOnce sync.Once
	stopErr  error
}

// Start 启动一个可嵌入的 Limiter 实例。成功返回即表示所有 listener 已绑定，
// 且启用 registry 时已经完成实例注册。
func Start(parent context.Context, cfg Config) (*Running, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(parent)
	running := &Running{
		cancel:   cancel,
		done:     make(chan error, 1),
		stopping: make(chan struct{}),
	}
	statics, err := buildStatistics(ctx, cfg)
	if err != nil {
		cancel()
		return nil, err
	}
	running.statics = statics
	core, err := ratelimitv2.NewServer(ctx, &cfg.Limit, statics)
	if err != nil {
		_ = statics.Destroy()
		cancel()
		return nil, fmt.Errorf("initialize limiter core: %w", err)
	}

	for _, entry := range cfg.APIServers {
		var server apiserver.APIServer
		switch strings.ToLower(strings.TrimSpace(entry.Name)) {
		case "grpc":
			server, err = limitergrpc.Start(entry.Option, core, statics)
		case "http":
			server, err = limiterhttp.Start(entry.Option, core)
		}
		if err != nil {
			running.rollback()
			return nil, fmt.Errorf("start limiter %s server: %w", entry.Name, err)
		}
		running.servers = append(running.servers, server)
	}

	if cfg.Registry.Enable {
		running.registry = newRegistryClient(cfg.Registry)
		if err := running.registry.Register(ctx, running.servers); err != nil {
			running.rollback()
			return nil, fmt.Errorf("register limiter: %w", err)
		}
	}
	running.watch(ctx)
	return running, nil
}

func buildStatistics(ctx context.Context, runtimeConfig Config) (statistics.Statis, error) {
	cfg := runtimeConfig.Plugin.Statistics
	if cfg == nil || strings.EqualFold(strings.TrimSpace(cfg.Name), "echo") {
		worker := &echo.StaticsWorker{}
		if err := worker.Initialize(cfg); err != nil {
			return nil, err
		}
		return worker, nil
	}
	if strings.EqualFold(strings.TrimSpace(cfg.Name), "file") {
		worker := &file.StaticsWorker{}
		serverAddress := strings.TrimSpace(runtimeConfig.Registry.AdvertisedHost)
		if serverAddress == "" {
			serverAddress = "127.0.0.1"
		}
		if err := worker.InitializeContextWithIdentity(ctx, cfg, file.RuntimeIdentity{
			ServerAddress:    serverAddress,
			LimitServiceName: runtimeConfig.Registry.Name,
		}); err != nil {
			return nil, err
		}
		return worker, nil
	}
	return nil, fmt.Errorf("unsupported limiter statistics plugin %q", cfg.Name)
}

func (r *Running) watch(ctx context.Context) {
	for _, server := range r.servers {
		server := server
		go func() {
			err, ok := <-server.Done()
			select {
			case <-r.stopping:
				return
			default:
			}
			if !ok || err == nil {
				err = fmt.Errorf("limiter %s server exited unexpectedly", server.GetProtocol())
			} else {
				err = fmt.Errorf("limiter %s server: %w", server.GetProtocol(), err)
			}
			select {
			case r.done <- err:
			default:
			}
		}()
	}
}

// Wait 阻塞到运行期异常或实例停止。
func (r *Running) Wait() error {
	select {
	case err := <-r.done:
		return err
	}
}

// Endpoints 返回实际绑定的 listener。
func (r *Running) Endpoints() []string {
	endpoints := make([]string, 0, len(r.servers))
	for _, server := range r.servers {
		endpoints = append(endpoints,
			endpointAddress("127.0.0.1", server.GetPort())+"/"+server.GetProtocol())
	}
	return endpoints
}

// Stop 按 listener、registry、statistics、worker 的顺序幂等停止实例。
func (r *Running) Stop(ctx context.Context) error {
	r.stopOnce.Do(func() {
		close(r.stopping)
		var stopErrors []error
		for i := len(r.servers) - 1; i >= 0; i-- {
			if err := r.servers[i].Stop(ctx); err != nil {
				stopErrors = append(stopErrors,
					fmt.Errorf("stop limiter %s server: %w", r.servers[i].GetProtocol(), err))
			}
		}
		if r.registry != nil {
			if err := r.registry.Deregister(ctx); err != nil {
				stopErrors = append(stopErrors, err)
			}
		}
		if r.statics != nil {
			if err := r.statics.Destroy(); err != nil {
				stopErrors = append(stopErrors, fmt.Errorf("destroy limiter statistics: %w", err))
			}
		}
		r.cancel()
		r.stopErr = errors.Join(stopErrors...)
		select {
		case r.done <- nil:
		default:
		}
	})
	return r.stopErr
}

func (r *Running) rollback() {
	ctx, cancel := context.WithTimeout(context.Background(), rollbackTimeout)
	defer cancel()
	_ = r.Stop(ctx)
}
