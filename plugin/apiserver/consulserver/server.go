// Package consulserver 提供配置消费专用的 Consul KV 读取适配；不提供 KV 写入、Session 或锁。
package consulserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/pole-io/pole-server/apis/apiserver"
	compatconfig "github.com/pole-io/pole-server/pkg/compatibility/config"
	"github.com/pole-io/pole-server/pkg/config"
	"github.com/pole-io/pole-server/pluginapi"
)

const Protocol = "service-consul"

// Register 注册可选适配器；只有部署列出该插件并启用 client/config 才开放配置入口。
func Register(registry *pluginapi.Registry) error {
	return apiserver.RegisterFactory(registry, pluginapi.Descriptor{Kind: pluginapi.KindAPIServer, Name: Protocol, Origin: pluginapi.OriginBuiltin},
		func() (apiserver.Apiserver, error) { return &Server{}, nil })
}

// Server 拥有可选 HTTP listener；配置权威和逐请求权限始终由统一领域服务提供。
type Server struct {
	server *http.Server
	port   uint32
}

func (s *Server) GetProtocol() string { return Protocol }
func (s *Server) GetPort() uint32     { return s.port }

// Initialize 只装配入口，不打开监听；environment 是缺省 dc 对应的业务环境，必须显式配置。
func (s *Server) Initialize(ctx context.Context, options map[string]interface{}, api map[string]apiserver.APIConfig) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	environment, _ := options["environment"].(string)
	if environment == "" {
		return errors.New("Consul configuration requires a default environment")
	}
	ip, _ := options["listenIP"].(string)
	if ip == "" {
		ip = "0.0.0.0"
	}
	port := 8500
	if value, ok := options["listenPort"]; ok {
		var valid bool
		port, valid = value.(int)
		if !valid || port < 1 || port > 65535 {
			return errors.New("Consul listenPort must be an integer between 1 and 65535")
		}
	}
	enabled := api["client"].Enable
	if include := api["client"].Include; enabled && len(include) != 0 {
		enabled = false
		for _, name := range include {
			if name == apiserver.ConfigAccess {
				enabled = true
			}
		}
	}
	var reader compatconfig.Discoverer
	if enabled {
		core, err := config.GetUnifiedService()
		if err != nil {
			return err
		}
		reader = core
	}
	s.port = uint32(port)
	s.server = &http.Server{Addr: net.JoinHostPort(ip, strconv.Itoa(port)), Handler: NewHandler(reader, environment, enabled),
		ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 90 * time.Second, WriteTimeout: 70 * time.Second, MaxHeaderBytes: 64 << 10}
	return nil
}

func (s *Server) Run(errorsChannel chan error) {
	listener := s.server
	go func() {
		if err := listener.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errorsChannel <- fmt.Errorf("Consul configuration listener: %w", err)
		}
	}()
}

func (s *Server) Stop() {
	if s.server == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.server.Shutdown(ctx); err != nil {
		_ = s.server.Close()
	}
}

func (s *Server) Restart(options map[string]interface{}, api map[string]apiserver.APIConfig, errorsChannel chan error) error {
	s.Stop()
	if err := s.Initialize(context.Background(), options, api); err != nil {
		return err
	}
	s.Run(errorsChannel)
	return nil
}
