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

package http

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/emicklei/go-restful/v3"

	"github.com/pole-io/pole-server/pkg/limiter/internal/apiserver"
	"github.com/pole-io/pole-server/pkg/limiter/internal/ratelimitv2"
)

// Server http server
type Server struct {
	ip       string
	port     uint32
	server   *http.Server
	handler  *restful.Container
	core     *ratelimitv2.Server
	done     chan error
	stopOnce sync.Once
}

// Start 同步绑定 listener 后启动 HTTP 运维服务。
func Start(option map[string]interface{}, core *ratelimitv2.Server) (*Server, error) {
	ip, port, err := apiserver.ParseListenOption(option)
	if err != nil {
		return nil, err
	}
	h := &Server{
		ip:   ip,
		port: port,
		core: core,
		done: make(chan error, 1),
	}
	address := fmt.Sprintf("%s:%d", h.ip, h.port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("listen limiter http %s: %w", address, err)
	}
	h.port = uint32(listener.Addr().(*net.TCPAddr).Port)
	// handler
	h.initHandler()
	// http server
	server := http.Server{Addr: address, Handler: h.handler}
	h.server = &server
	go func() {
		err := h.server.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		h.done <- err
		close(h.done)
	}()
	return h, nil
}

// Done 返回运行期退出结果。
func (h *Server) Done() <-chan error {
	return h.done
}

// Stop server
func (h *Server) Stop(ctx context.Context) error {
	var err error
	h.stopOnce.Do(func() {
		err = h.server.Shutdown(ctx)
	})
	return err
}

// GetProtocol get protocol
func (h *Server) GetProtocol() string {
	return "http"
}

// GetPort 	get port
func (h *Server) GetPort() uint32 {
	return h.port
}
