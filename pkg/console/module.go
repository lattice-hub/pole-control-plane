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

package console

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	consoleconfig "github.com/pole-io/pole-server/pkg/console/config"
	"github.com/pole-io/pole-server/pkg/console/internal/handlers"
	"github.com/pole-io/pole-server/pkg/console/internal/router"

	_ "github.com/pole-io/pole-server/pkg/console/internal/observer/mysql"
)

// Running 是已完成监听器绑定的 Console 实例。
type Running struct {
	server   *http.Server
	done     chan error
	stopOnce sync.Once
	stopErr  error
}

// Start 在当前进程中初始化并启动内嵌 Console 网关。
func Start(ctx context.Context, config *consoleconfig.Config) (*Running, error) {
	handlers.NewAdminGetter(config)
	if err := initialize(config); err != nil {
		return nil, err
	}
	setMode(config)

	address := fmt.Sprintf("%v:%v", config.WebServer.ListenIP, config.WebServer.ListenPort)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("listen console %s: %w", address, err)
	}
	server := &http.Server{
		Addr: address,
		Handler: otelhttp.NewHandler(router.NewRouter(config, embeddedConsoleAssets()), "pole-console-http",
			otelhttp.WithSpanNameFormatter(func(_ string, request *http.Request) string {
				path := strings.TrimSuffix(request.URL.Path, "/")
				if path == "" {
					path = "/"
				}
				return request.Method + ":" + path
			}),
		),
	}
	running := &Running{
		server: server,
		done:   make(chan error, 1),
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = running.Stop(shutdownCtx)
	}()

	go func() {
		err := server.Serve(listener)
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		select {
		case running.done <- err:
		default:
		}
	}()

	return running, nil
}

// Wait 阻塞到运行期异常或实例停止。
func (r *Running) Wait() error {
	return <-r.done
}

// Stop 幂等停止 Console HTTP 服务。
func (r *Running) Stop(ctx context.Context) error {
	r.stopOnce.Do(func() {
		r.stopErr = r.server.Shutdown(ctx)
	})
	return r.stopErr
}
