package config

import (
	"time"

	"github.com/pole-io/pole-server/pkg/compatibility"
	"github.com/pole-io/pole-server/pkg/systemconfig"
	"github.com/pole-io/pole-server/pkg/workloadcredential"
)

func NewSystemSettingsProvider(cfg *Config, compatibilityResolvers ...*compatibility.Resolver) (systemconfig.EffectiveProvider, error) {
	resolver := compatibility.NewNamespaceResolver("dev", "dev")
	if len(compatibilityResolvers) > 0 && compatibilityResolvers[0] != nil {
		resolver = compatibilityResolvers[0]
	}
	sources := cfg.SystemConfigSources.Clone()
	if cfg.Naming.AutoCreate == nil {
		sources.MarkCompiledDefault("naming.autoCreate")
	}
	healthcheckSourceDefaults(cfg, sources)
	if cfg.Cache.DiffTime == 0 {
		sources.MarkCompiledDefault("cache.diffTime")
	}
	if cfg.WorkloadCredential.TTL == 0 {
		sources.MarkCompiledDefault("workloadCredential.ttl")
	}
	if cfg.WorkloadCredential.ClockSkew == 0 {
		sources.MarkCompiledDefault("workloadCredential.clockSkew")
	}
	if cfg.WorkloadCredential.BundleTTL == 0 {
		sources.MarkCompiledDefault("workloadCredential.bundleTTL")
	}
	healthcheckConfig := func() healthcheckValues {
		value := cfg.Naming.HealthChecks
		value.SetDefault()
		return healthcheckValues{
			open: value.IsOpen(), service: value.Service, slotNum: value.SlotNum,
			minCheckInterval: value.MinCheckInterval, maxCheckInterval: value.MaxCheckInterval,
			clientCheckInterval: value.ClientCheckInterval, clientCheckTTL: value.ClientCheckTtl,
		}
	}
	cacheDiffTime := func() any {
		if cfg.Cache.DiffTime == 0 {
			return 5 * time.Second
		}
		return cfg.Cache.DiffTime.Abs()
	}
	workloadTTL := func() any {
		if cfg.WorkloadCredential.TTL == 0 {
			return workloadcredential.DefaultCredentialTTL
		}
		return cfg.WorkloadCredential.TTL
	}
	workloadClockSkew := func() any {
		if cfg.WorkloadCredential.ClockSkew == 0 {
			return workloadcredential.DefaultClockSkew
		}
		return cfg.WorkloadCredential.ClockSkew
	}
	workloadBundleTTL := func() any {
		if cfg.WorkloadCredential.BundleTTL == 0 {
			return workloadcredential.DefaultBundleTTL
		}
		return cfg.WorkloadCredential.BundleTTL
	}

	registrations := []systemconfig.Registration{
		serverSetting("server.bootstrap.mode", "bootstrap", "启动模式", "bootstrap.mode", systemconfig.ValueTypeString, systemconfig.BootstrapOnly, systemconfig.SensitivityPublic, func() any { return cfg.Bootstrap.Mode }),
		serverSetting("server.bootstrap.logger", "bootstrap", "日志配置文件", "bootstrap.logger", systemconfig.ValueTypeString, systemconfig.RestartRequired, systemconfig.SensitivityInternal, func() any { return cfg.Bootstrap.Logger }),
		serverSetting("server.bootstrap.api_servers", "bootstrap", "API Server 配置文件", "apiservers", systemconfig.ValueTypeString, systemconfig.RestartRequired, systemconfig.SensitivityInternal, func() any { return cfg.APIServers }),
		serverSetting("server.namespace.auto_create", "namespace", "自动创建命名空间", "namespace.autoCreate", systemconfig.ValueTypeBoolean, systemconfig.RestartRequired, systemconfig.SensitivityPublic, func() any { return cfg.Namespace.AutoCreate }),
		serverSetting("server.naming.auto_create", "naming", "自动创建服务", "naming.autoCreate", systemconfig.ValueTypeBoolean, systemconfig.RestartRequired, systemconfig.SensitivityPublic, func() any { return boolValue(cfg.Naming.AutoCreate, true) }),
		serverSetting("server.naming.healthcheck.open", "naming", "健康检查开关", "naming.healthcheck.open", systemconfig.ValueTypeBoolean, systemconfig.RestartRequired, systemconfig.SensitivityPublic, func() any { return healthcheckConfig().open }),
		serverSetting("server.naming.healthcheck.service", "naming", "健康检查服务", "naming.healthcheck.service", systemconfig.ValueTypeString, systemconfig.RestartRequired, systemconfig.SensitivityInternal, func() any { return healthcheckConfig().service }),
		serverSetting("server.naming.healthcheck.slot_num", "naming", "时间轮槽位数", "naming.healthcheck.slotNum", systemconfig.ValueTypeInteger, systemconfig.RestartRequired, systemconfig.SensitivityPublic, func() any { return healthcheckConfig().slotNum }),
		serverSetting("server.naming.healthcheck.min_check_interval", "naming", "最小检查周期", "naming.healthcheck.minCheckInterval", systemconfig.ValueTypeDuration, systemconfig.RestartRequired, systemconfig.SensitivityPublic, func() any { return healthcheckConfig().minCheckInterval }),
		serverSetting("server.naming.healthcheck.max_check_interval", "naming", "最大检查周期", "naming.healthcheck.maxCheckInterval", systemconfig.ValueTypeDuration, systemconfig.RestartRequired, systemconfig.SensitivityPublic, func() any { return healthcheckConfig().maxCheckInterval }),
		serverSetting("server.naming.healthcheck.client_check_interval", "naming", "客户端检查周期", "naming.healthcheck.clientCheckInterval", systemconfig.ValueTypeDuration, systemconfig.RestartRequired, systemconfig.SensitivityPublic, func() any { return healthcheckConfig().clientCheckInterval }),
		serverSetting("server.naming.healthcheck.client_check_ttl", "naming", "客户端检查 TTL", "naming.healthcheck.clientCheckTtl", systemconfig.ValueTypeDuration, systemconfig.RestartRequired, systemconfig.SensitivityPublic, func() any { return healthcheckConfig().clientCheckTTL }),
		serverSetting("server.config.open", "config", "配置中心开关", "config.open", systemconfig.ValueTypeBoolean, systemconfig.RestartRequired, systemconfig.SensitivityPublic, func() any { return cfg.Config.Open }),
		serverSetting("server.cache.diff_time", "cache", "增量同步回溯窗口", "cache.diffTime", systemconfig.ValueTypeDuration, systemconfig.RestartRequired, systemconfig.SensitivityPublic, cacheDiffTime),
		serverSetting("server.store.name", "storage", "存储插件", "store.name", systemconfig.ValueTypeString, systemconfig.BootstrapOnly, systemconfig.SensitivityInternal, func() any { return cfg.Store.Name }),
		serverSetting("server.store.dsn", "storage", "数据库连接", "store.option.master.dns", systemconfig.ValueTypeSecret, systemconfig.BootstrapOnly, systemconfig.SensitivitySecret, func() any { return systemconfig.LookupValue(cfg.Store.Option, "master", "dns") }),
		serverSetting("server.auth.user.plugin", "auth", "用户认证插件", "auth.user.name", systemconfig.ValueTypeString, systemconfig.BootstrapOnly, systemconfig.SensitivityInternal, func() any {
			if cfg.Auth.User == nil {
				return ""
			}
			return cfg.Auth.User.Name
		}),
		serverSetting("server.auth.user.salt", "auth", "Token 加密盐", "auth.user.option.salt", systemconfig.ValueTypeSecret, systemconfig.BootstrapOnly, systemconfig.SensitivitySecret, func() any {
			if cfg.Auth.User == nil {
				return nil
			}
			return cfg.Auth.User.Option["salt"]
		}),
		serverSetting("server.auth.strategy.plugin", "auth", "授权策略插件", "auth.strategy.name", systemconfig.ValueTypeString, systemconfig.BootstrapOnly, systemconfig.SensitivityInternal, func() any {
			if cfg.Auth.Strategy == nil {
				return ""
			}
			return cfg.Auth.Strategy.Name
		}),
		serverSetting("server.auth.console_open", "auth", "Console 鉴权", "auth.strategy.option.consoleOpen", systemconfig.ValueTypeBoolean, systemconfig.RestartRequired, systemconfig.SensitivityPublic, func() any { return authOption(cfg, "consoleOpen") }),
		serverSetting("server.auth.client_open", "auth", "客户端鉴权", "auth.strategy.option.clientOpen", systemconfig.ValueTypeBoolean, systemconfig.RestartRequired, systemconfig.SensitivityPublic, func() any { return authOption(cfg, "clientOpen") }),
		serverSetting("server.workload_credential.enabled", "workload-credential", "数据面凭证签发", "workloadCredential.enabled", systemconfig.ValueTypeBoolean, systemconfig.RestartRequired, systemconfig.SensitivityPublic, func() any { return cfg.WorkloadCredential.Enabled }),
		serverSetting("server.workload_credential.issuer", "workload-credential", "凭证签发者", "workloadCredential.issuer", systemconfig.ValueTypeString, systemconfig.RestartRequired, systemconfig.SensitivityInternal, func() any { return cfg.WorkloadCredential.Issuer }),
		serverSetting("server.workload_credential.audience", "workload-credential", "凭证受众", "workloadCredential.audience", systemconfig.ValueTypeString, systemconfig.RestartRequired, systemconfig.SensitivityInternal, func() any { return cfg.WorkloadCredential.Audience }),
		serverSetting("server.workload_credential.trust_domain", "workload-credential", "信任域", "workloadCredential.trustDomain", systemconfig.ValueTypeString, systemconfig.RestartRequired, systemconfig.SensitivityInternal, func() any { return cfg.WorkloadCredential.TrustDomain }),
		serverSetting("server.workload_credential.ttl", "workload-credential", "凭证 TTL", "workloadCredential.ttl", systemconfig.ValueTypeDuration, systemconfig.RestartRequired, systemconfig.SensitivityPublic, workloadTTL),
		serverSetting("server.workload_credential.clock_skew", "workload-credential", "时钟偏差容忍", "workloadCredential.clockSkew", systemconfig.ValueTypeDuration, systemconfig.RestartRequired, systemconfig.SensitivityPublic, workloadClockSkew),
		serverSetting("server.workload_credential.bundle_ttl", "workload-credential", "公钥包 TTL", "workloadCredential.bundleTTL", systemconfig.ValueTypeDuration, systemconfig.RestartRequired, systemconfig.SensitivityPublic, workloadBundleTTL),
		compatibilitySetting(compatibility.NacosPublicNamespaceTargetKey, "Nacos public 默认环境",
			func() any { return resolver.Current().NacosPublicTarget }, resolver),
		compatibilitySetting(compatibility.EurekaDefaultNamespaceKey, "Eureka 缺省环境",
			func() any { return resolver.Current().EurekaDefaultTarget }, resolver),
	}
	systemconfig.ApplyEditPolicies(registrations, serverSystemSettingPolicies())
	return systemconfig.NewRegistry(registrations, sources)
}

func compatibilitySetting(key, label string, value func() any,
	resolver *compatibility.Resolver) systemconfig.Registration {
	return systemconfig.Registration{
		Definition: systemconfig.SettingDefinition{
			Key: key, Component: systemconfig.ComponentServer, Domain: compatibility.Domain,
			Label: label, ValueType: systemconfig.ValueTypeString,
			ApplyMode: systemconfig.GuardedHotReload, Sensitivity: systemconfig.SensitivityPublic,
			Owner: "pole-server",
		},
		Value:  value,
		Source: func() systemconfig.SourceDescriptor { return resolver.Current().Source },
		Status: func() string { return resolver.Current().ApplyStatus },
	}
}

func serverSystemSettingPolicies() map[string]systemconfig.EditPolicy {
	required := systemconfig.Validation{Required: true}
	positive := func(min int64) systemconfig.Validation {
		return systemconfig.Validation{Required: true, MinInteger: &min}
	}
	duration := func(min, max string) systemconfig.Validation {
		return systemconfig.Validation{Required: true, MinDuration: min, MaxDuration: max}
	}
	locked := func(reason, description string) systemconfig.EditPolicy {
		return systemconfig.EditPolicy{Reason: reason, Description: description}
	}
	restart := func(description string, validation systemconfig.Validation) systemconfig.EditPolicy {
		return systemconfig.EditPolicy{
			Editable: true, Reason: "发布后进入待重启状态", Description: description, Validation: validation,
		}
	}
	return map[string]systemconfig.EditPolicy{
		"server.bootstrap.mode":        locked("进程装配边界，只能通过部署配置修改", "决定 Control Plane、Limiter Server、Console 的进程装配方式。"),
		"server.bootstrap.logger":      locked("日志配置文件属于部署工件", "日志配置文件路径由镜像或挂载卷提供。"),
		"server.bootstrap.api_servers": locked("API Server 拓扑属于部署工件", "API Server 清单路径由部署阶段确定。"),

		"server.namespace.auto_create": restart("未找到命名空间时是否自动创建。", systemconfig.Validation{}),

		compatibility.NacosPublicNamespaceTargetKey: {
			Editable: true, Reason: "校验目标环境后受控热切换",
			Description: "Nacos 空 namespaceId 或 public 映射到的 Pole BUSINESS 环境。",
			Validation:  systemconfig.Validation{Required: true},
		},
		compatibility.EurekaDefaultNamespaceKey: {
			Editable: true, Reason: "校验目标环境后受控热切换",
			Description: "Eureka 请求未携带 x-namespace 时使用的 Pole BUSINESS 环境。",
			Validation:  systemconfig.Validation{Required: true},
		},

		"server.naming.auto_create":                       restart("未找到服务时是否自动创建。", systemconfig.Validation{}),
		"server.naming.healthcheck.open":                  restart("是否启用服务实例健康检查调度。", systemconfig.Validation{}),
		"server.naming.healthcheck.service":               restart("承担健康检查任务的内部服务名称。", required),
		"server.naming.healthcheck.slot_num":              restart("健康检查时间轮槽位数量。", positive(1)),
		"server.naming.healthcheck.min_check_interval":    restart("健康检查退避的最小周期。", duration("1s", "10m")),
		"server.naming.healthcheck.max_check_interval":    restart("健康检查退避的最大周期，必须不小于最小周期。", duration("1s", "1h")),
		"server.naming.healthcheck.client_check_interval": restart("客户端心跳健康检查周期。", duration("1s", "10m")),
		"server.naming.healthcheck.client_check_ttl":      restart("客户端心跳失效判定 TTL。", duration("1s", "1h")),

		"server.config.open":     restart("是否开放配置中心服务。", systemconfig.Validation{}),
		"server.cache.diff_time": restart("增量缓存同步时向前回溯的时间窗口。", duration("1s", "10m")),

		"server.store.name": locked("存储插件属于自举岛", "存储插件必须在读取动态配置之前完成初始化。"),
		"server.store.dsn":  locked("数据库连接属于自举 Secret", "数据库连接只通过部署期 Secret 或静态配置提供。"),

		"server.auth.user.plugin":     locked("认证插件属于自举岛", "用户认证插件在动态配置加载前初始化。"),
		"server.auth.user.salt":       locked("Token 盐属于根级 Secret", "缺少双版本轮换机制前只能通过部署配置修改。"),
		"server.auth.strategy.plugin": locked("授权插件属于自举岛", "授权策略插件在动态配置加载前初始化。"),
		"server.auth.console_open":    restart("是否要求 Console 请求通过控制面鉴权。", systemconfig.Validation{}),
		"server.auth.client_open":     restart("是否要求客户端请求通过控制面鉴权。", systemconfig.Validation{}),

		"server.workload_credential.enabled":      restart("是否签发工作负载身份凭证。", systemconfig.Validation{}),
		"server.workload_credential.issuer":       restart("工作负载凭证的 issuer。", required),
		"server.workload_credential.audience":     restart("工作负载凭证的 audience。", required),
		"server.workload_credential.trust_domain": restart("SPIFFE 风格工作负载信任域。", required),
		"server.workload_credential.ttl":          restart("单个工作负载凭证的有效期。", duration("1m", "24h")),
		"server.workload_credential.clock_skew":   restart("校验凭证时允许的时钟偏差。", duration("0s", "10m")),
		"server.workload_credential.bundle_ttl":   restart("公钥包缓存有效期。", duration("1m", "24h")),
	}
}

func healthcheckSourceDefaults(cfg *Config, sources systemconfig.SourceIndex) {
	value := cfg.Naming.HealthChecks
	if value.Open == nil {
		sources.MarkCompiledDefault("naming.healthcheck.open")
	}
	if value.Service == "" {
		sources.MarkCompiledDefault("naming.healthcheck.service")
	}
	if value.SlotNum == 0 {
		sources.MarkCompiledDefault("naming.healthcheck.slotNum")
	}
	if value.MinCheckInterval == 0 || value.MaxCheckInterval == 0 || value.MinCheckInterval > value.MaxCheckInterval {
		sources.MarkCompiledDefault("naming.healthcheck.minCheckInterval", "naming.healthcheck.maxCheckInterval")
	}
	if value.ClientCheckInterval == 0 {
		sources.MarkCompiledDefault("naming.healthcheck.clientCheckInterval")
	}
	if value.ClientCheckTtl == 0 {
		sources.MarkCompiledDefault("naming.healthcheck.clientCheckTtl")
	}
}

type healthcheckValues struct {
	open                bool
	service             string
	slotNum             int
	minCheckInterval    time.Duration
	maxCheckInterval    time.Duration
	clientCheckInterval time.Duration
	clientCheckTTL      time.Duration
}

func serverSetting(key, domain, label, sourcePath string, valueType systemconfig.ValueType,
	applyMode systemconfig.ApplyMode, sensitivity systemconfig.Sensitivity, value func() any) systemconfig.Registration {
	return systemconfig.Registration{
		Definition: systemconfig.SettingDefinition{
			Key: key, Component: systemconfig.ComponentServer, Domain: domain, Label: label,
			ValueType: valueType, ApplyMode: applyMode, Sensitivity: sensitivity, Owner: "pole-server",
		},
		SourcePath: sourcePath,
		Value:      value,
	}
}

func boolValue(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func authOption(cfg *Config, key string) any {
	if cfg.Auth.Strategy == nil {
		return false
	}
	value, ok := cfg.Auth.Strategy.Option[key]
	if !ok {
		return false
	}
	return value
}
