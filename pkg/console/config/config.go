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
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pole-io/pole-server/pkg/console/internal/common/log"
	"github.com/pole-io/pole-server/pkg/console/internal/observabilityquery"
	store "github.com/pole-io/pole-server/pkg/console/internal/observer"
	"github.com/pole-io/pole-server/pkg/systemconfig"
	"gopkg.in/yaml.v2"
)

var (
	_globalConfig *Config
)

type LoggerOptions = log.Options

const (
	DefaultAgentProposalTTL     = 30 * time.Minute
	DefaultAgentUpstreamTimeout = 10 * time.Second
)

// PoleServer polaris server配置
type PoleServer struct {
	Address string `yaml:"address"`
}

type MonitorServer struct {
	Address string `yaml:"address"`
}

// Config 配置
type Config struct {
	Logger               log.Options                           `yaml:"logger"`
	WebServer            WebServer                             `yaml:"webServer"`
	PoleServer           PoleServer                            `yaml:"poleServer"`
	LimiterServer        PoleServer                            `yaml:"limiterServer"`
	MonitorServer        MonitorServer                         `yaml:"monitorServer"`
	Futures              string                                `yaml:"futures"`
	Store                store.Config                          `yaml:"store"`
	ObservabilityQuery   observabilityquery.Config             `yaml:"observabilityQuery"`
	Agent                AgentConfig                           `yaml:"agent"`
	SystemSecrets        SystemSecretsConfig                   `yaml:"systemSecrets"`
	SystemConfigSources  systemconfig.SourceIndex              `yaml:"-" json:"-"`
	AgentCapabilityProbe func(context.Context, []string) error `yaml:"-" json:"-"`
}

func (c *Config) HasFutures(s string) bool {
	return strings.Contains(c.Futures, s)
}

// WebServer web server配置
type WebServer struct {
	Mode       string `yaml:"mode"`
	ListenIP   string `yaml:"listenIP"`
	ListenPort int    `yaml:"listenPort"`
	CoreURL    string `yaml:"coreURL"`
	NamingURL  string `yaml:"namingURL"`
	AuthURL    string `yaml:"authURL"`
	MonitorURL string `yaml:"monitorURL"`
	ConfigURL  string `yaml:"configURL"`
	LogURL     string `yaml:"logURL"`
	JWT        JWT    `yaml:"jwt"`
	MainUser   string `yaml:"mainUser"`
}

// JWT jwtToken 相关的配置
type JWT struct {
	// 参与jwt运算的key
	SecretKey string `yaml:"secretKey"`
	// 过期时间, 单位为秒
	Expired int `yaml:"expired"`
}

type AgentConfig struct {
	RuntimeMode            string                `yaml:"runtimeMode" json:"runtimeMode"`
	SelfManagementProbeKey string                `yaml:"selfManagementProbeKey" json:"-"`
	Definition             AgentDefinitionConfig `yaml:"definition" json:"definition"`
	Model                  AgentModelConfig      `yaml:"model" json:"model"`
	MCP                    AgentMCPConfig        `yaml:"mcp" json:"mcp"`
	A2A                    AgentA2AConfig        `yaml:"a2a" json:"a2a"`
	ProposalTTL            time.Duration         `yaml:"proposalTTL" json:"proposalTTL"`
	UpstreamTimeout        time.Duration         `yaml:"upstreamTimeout" json:"upstreamTimeout"`
}

type AgentDefinitionConfig struct {
	ID           string                  `yaml:"id" json:"id"`
	SystemPrompt AgentSystemPromptConfig `yaml:"systemPrompt" json:"systemPrompt"`
}

type AgentSystemPromptConfig struct {
	BuiltinVersion       string `yaml:"builtinVersion" json:"builtinVersion"`
	OperatorInstructions string `yaml:"operatorInstructions" json:"operatorInstructions"`
}

type AgentModelConfig struct {
	Provider string        `yaml:"provider" json:"provider"`
	BaseURL  string        `yaml:"baseURL" json:"baseURL"`
	Model    string        `yaml:"model" json:"model"`
	APIKey   string        `yaml:"apiKey" json:"-"`
	Timeout  time.Duration `yaml:"timeout" json:"timeout"`
}

type AgentMCPConfig struct {
	Endpoint      string   `yaml:"endpoint" json:"endpoint"`
	ToolAllowlist []string `yaml:"toolAllowlist" json:"toolAllowlist"`
}

type AgentA2AConfig struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description" json:"description"`
	Endpoint    string `yaml:"endpoint" json:"endpoint"`
}

// SystemSecrets contains only the irreducible bootstrap root used to unwrap
// Pole-managed product secrets. Individual product credentials never belong
// here.
type SystemSecretsConfig struct {
	MasterKey    string        `yaml:"masterKey" json:"-"`
	PollInterval time.Duration `yaml:"pollInterval" json:"pollInterval"`
}

func (c AgentConfig) Normalize() AgentConfig {
	if c.RuntimeMode == "" {
		c.RuntimeMode = "llm"
	}
	if c.Definition.ID == "" {
		c.Definition.ID = "pole-control-plane"
	}
	if c.Definition.SystemPrompt.BuiltinVersion == "" {
		c.Definition.SystemPrompt.BuiltinVersion = "v1"
	}
	if c.Model.Provider == "" {
		c.Model.Provider = "openai-compatible"
	}
	if c.Model.Model == "" {
		c.Model.Model = "auto"
	}
	if c.Model.Timeout <= 0 {
		c.Model.Timeout = 60 * time.Second
	}
	if c.MCP.Endpoint == "" {
		c.MCP.Endpoint = "http://127.0.0.1:8090/ai/mcp/v1/sse"
	}
	if c.A2A.Name == "" {
		c.A2A.Name = "Pole Agent"
	}
	if c.A2A.Description == "" {
		c.A2A.Description = "通过 MCP 发现并管理 Pole 控制面资源"
	}
	if c.ProposalTTL <= 0 {
		c.ProposalTTL = DefaultAgentProposalTTL
	}
	if c.UpstreamTimeout <= 0 {
		c.UpstreamTimeout = DefaultAgentUpstreamTimeout
	}
	return c
}

// LoadConfig 加载配置文件
func LoadConfig(filePath string) (*Config, error) {
	if filePath == "" {
		err := errors.New("invalid config file path")
		fmt.Printf("[ERR0R] %v\n", err)
		return nil, err
	}

	fmt.Printf("[INFO] load config from %v\n", filePath)

	content, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Printf("[ERROR] %v\n", err)
		return nil, err
	}

	finalContent := os.ExpandEnv(string(content))
	sources, sourceErr := systemconfig.SourceIndexFromYAML(content)
	if sourceErr != nil {
		return nil, sourceErr
	}

	config := &Config{
		Logger: DefaultLoggerOptions(),
	}
	config.WebServer.JWT.Expired = 7200 // 默认2小时
	config.WebServer.JWT.SecretKey = "polarismesh@2021"
	err = yaml.NewDecoder(bytes.NewBuffer([]byte(finalContent))).Decode(config)
	if err != nil {
		fmt.Printf("[ERROR] %v\n", err)
	}

	_globalConfig = config
	config.SystemConfigSources = sources
	return config, nil
}

func GetConfig() *Config {
	return _globalConfig
}

func DefaultLoggerOptions() log.Options {
	return log.Options{
		ErrorOutputPaths:   []string{"logs/runtime/pole-console-error.log"},
		RotateOutputPath:   "logs/runtime/pole-console.log",
		RotationMaxSize:    500,
		RotationMaxAge:     30,
		RotationMaxBackups: 100,
		Level:              "info",
	}
}
