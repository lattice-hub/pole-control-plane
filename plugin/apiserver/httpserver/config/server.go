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

package config

import (
	"github.com/pole-io/pole-server/pkg/config"
	"github.com/pole-io/pole-server/pkg/config/unified"
)

// HTTPServer 提供统一配置管理入口。
type HTTPServer struct {
	unifiedService *unified.Service
}

// NewServer 获取具备存储和认证能力的配置服务；初始化失败时不挂载空服务。
func NewServer() (*HTTPServer, error) {
	service, err := config.GetUnifiedService()
	if err != nil {
		return nil, err
	}
	return &HTTPServer{unifiedService: service}, nil
}

const (
	defaultReadAccess   string = "default-read"
	defaultAccess       string = "default"
	configConsoleAccess string = "config"
)
