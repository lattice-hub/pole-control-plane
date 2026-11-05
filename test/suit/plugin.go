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

package testsuit

import (
	_ "github.com/pole-io/pole-server/pkg/service/interceptor"
	defaultpolicy "github.com/pole-io/pole-server/plugin/access_control/auth/policy"
	defaultuser "github.com/pole-io/pole-server/plugin/access_control/auth/user"
	ratelimittoken "github.com/pole-io/pole-server/plugin/access_control/ratelimit/token"
	cmdbmemory "github.com/pole-io/pole-server/plugin/cmdb/memory"
	cryptoaes "github.com/pole-io/pole-server/plugin/crypto/aes"
	discoverlogger "github.com/pole-io/pole-server/plugin/observability/discoverevent/logger"
	historylogger "github.com/pole-io/pole-server/plugin/observability/history/logger"
	statislogger "github.com/pole-io/pole-server/plugin/observability/statis/logger"
	statisprometheus "github.com/pole-io/pole-server/plugin/observability/statis/prometheus"
	"github.com/pole-io/pole-server/plugin/service/healthchecker/heartbeat"
	"github.com/pole-io/pole-server/plugin/service/healthchecker/probe"
	mysqlstore "github.com/pole-io/pole-server/plugin/store/mysql"
	"github.com/pole-io/pole-server/pluginapi"
)

func init() {
	registry := pluginapi.DefaultRegistry()
	registerTestPlugin(registry, defaultpolicy.Register)
	registerTestPlugin(registry, defaultuser.Register)
	registerTestPlugin(registry, ratelimittoken.Register)
	registerTestPlugin(registry, cmdbmemory.Register)
	registerTestPlugin(registry, cryptoaes.Register)
	registerTestPlugin(registry, discoverlogger.Register)
	registerTestPlugin(registry, historylogger.Register)
	registerTestPlugin(registry, statislogger.Register)
	registerTestPlugin(registry, statisprometheus.Register)
	registerTestPlugin(registry, heartbeat.Register)
	registerTestPlugin(registry, probe.Register)
	registerTestPlugin(registry, mysqlstore.Register)
}

func registerTestPlugin(registry *pluginapi.Registry, register func(*pluginapi.Registry) error) {
	if err := register(registry); err != nil {
		panic(err)
	}
}
