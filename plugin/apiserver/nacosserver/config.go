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

package nacosserver

import (
	"github.com/mitchellh/mapstructure"

	connlimit "github.com/pole-io/pole-server/pkg/common/conn/limit"
	"github.com/pole-io/pole-server/pkg/common/secure"
	"github.com/pole-io/pole-server/plugin/apiserver/nacosserver/model"
)

type NacosConfig struct {
	ListenPort       uint32            `mapstructure:"listenPort"`
	GrpcListenPort   uint32            `mapstructure:"grpcListenPort"`
	ConnLimit        *connlimit.Config `mapstructure:"connLimit"`
	TLS              *secure.TLSConfig `mapstructure:"tls"`
	DefaultNamespace string            `mapstructure:"defaultNamespace"`
	ServerService    string            `mapstructure:"serverService"`
	ServerNamespace  string            `mapstructure:"serverNamespace"`
}

func loadNacosConfig(raw map[string]interface{}) (*NacosConfig, error) {
	defaultCfg := &NacosConfig{
		ListenPort:       model.DefaultListenPort,
		DefaultNamespace: "dev",
	}
	decodeConfig := &mapstructure.DecoderConfig{
		DecodeHook: mapstructure.StringToTimeDurationHookFunc(),
		Result:     defaultCfg,
	}
	decoder, err := mapstructure.NewDecoder(decodeConfig)
	if err != nil {
		return nil, err
	}
	if err := decoder.Decode(raw); err != nil {
		return nil, err
	}

	if defaultCfg.GrpcListenPort == 0 {
		// grpc port is not set, use http port + 1000
		defaultCfg.GrpcListenPort = defaultCfg.ListenPort + 1000
	}
	return defaultCfg, nil
}
