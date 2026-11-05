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

package v1

import (
	"github.com/pole-io/pole-server/pkg/config/unified"
	"github.com/pole-io/pole-server/pkg/goverrule"
	"github.com/pole-io/pole-server/pkg/service"
	"github.com/pole-io/pole-server/pkg/service/healthcheck"
	"github.com/pole-io/pole-server/pkg/workloadcredential"
)

type DOption func(s *DiscoverGRPCServer)

func WithNamingServer(svr service.DiscoverServer) DOption {
	return func(s *DiscoverGRPCServer) {
		s.namingServer = svr
	}
}

func WithWorkloadCredentialServer(svr *workloadcredential.Server) DOption {
	return func(s *DiscoverGRPCServer) {
		s.workloadCredentialServer = svr
	}
}

func WithHealthCheckerServer(svr *healthcheck.Server) DOption {
	return func(s *DiscoverGRPCServer) {
		s.healthCheckServer = svr
	}
}

func WithGoverRuleServer(svr goverrule.GoverRuleServer) DOption {
	return func(s *DiscoverGRPCServer) {
		s.ruleServer = svr
	}
}

func WithDEnterRateLimit(f func(ip string, method string) uint32) DOption {
	return func(s *DiscoverGRPCServer) {
		s.enterRateLimit = f
	}
}

func WithDAllowAccess(f func(method string) bool) DOption {
	return func(s *DiscoverGRPCServer) {
		s.allowAccess = f
	}
}

type DiscoverGRPCServer struct {
	namingServer             service.DiscoverServer
	ruleServer               goverrule.GoverRuleServer
	healthCheckServer        *healthcheck.Server
	workloadCredentialServer *workloadcredential.Server
	enterRateLimit           func(ip string, method string) uint32
	allowAccess              func(method string) bool
}

func NewDiscoverGRPCServer(options ...DOption) *DiscoverGRPCServer {
	s := &DiscoverGRPCServer{}

	for i := range options {
		options[i](s)
	}

	return s
}

type COption func(s *ConfigGRPCServer)

func WithCEnterRateLimit(f func(ip string, method string) uint32) COption {
	return func(s *ConfigGRPCServer) {
		s.enterRateLimit = f
	}
}

func WithCAllowAccess(f func(method string) bool) COption {
	return func(s *ConfigGRPCServer) {
		s.allowAccess = f
	}
}

// WithUnifiedConfigServer 为现有客户端 ConfigGRPC 注入统一读取核心；不提供管理 RPC。
func WithUnifiedConfigServer(service *unified.Service) COption {
	return func(server *ConfigGRPCServer) { server.unifiedService = service }
}

type ConfigGRPCServer struct {
	unifiedService *unified.Service
	enterRateLimit func(ip string, method string) uint32
	allowAccess    func(method string) bool
}

func NewConfigGRPCServer(options ...COption) *ConfigGRPCServer {
	s := &ConfigGRPCServer{}

	for i := range options {
		options[i](s)
	}

	return s
}
