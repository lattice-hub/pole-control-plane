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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadConfigDefaultsConsoleLoggerToLogsDirectory(t *testing.T) {
	t.Setenv("TEST_POLE_AGENT_LLM_API_KEY", "test-agent-secret")
	cfgPath := filepath.Join(t.TempDir(), "pole-console.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(`
webServer:
  listenPort: 8080
poleServer:
  address: 127.0.0.1:8090
agent:
  model:
    baseURL: https://llm-gateway.example.test
    apiKey: ${TEST_POLE_AGENT_LLM_API_KEY}
`), 0644))

	cfg, err := LoadConfig(cfgPath)
	require.NoError(t, err)
	require.Equal(t, "logs/runtime/pole-console.log", cfg.Logger.RotateOutputPath)
	require.Contains(t, cfg.Logger.ErrorOutputPaths, "logs/runtime/pole-console-error.log")
	require.Equal(t, 7200, cfg.WebServer.JWT.Expired)
	require.Equal(t, "https://llm-gateway.example.test", cfg.Agent.Model.BaseURL)
	require.Equal(t, "test-agent-secret", cfg.Agent.Model.APIKey)
	require.Equal(t, "env:TEST_POLE_AGENT_LLM_API_KEY", cfg.SystemConfigSources["agent.model.apiKey"].Reference)
}
