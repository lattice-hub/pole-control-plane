---
title: 配置参考
tags: [config, yaml, deploy]
links: [overview, architecture, adr-system-configuration-control-plane, adr-console-agent-resource-workbench, adr-otel-observability-platform, adr-unified-process-mode-and-limiter-integration, adr-kubernetes-production-deployment, kubernetes-production-sop]
updated: 2026-08-19
sources: 10
---

# 配置参考

本文覆盖 YAML 配置文件的完整结构。项目的启动流程见 [[overview]]，配置如何影响架构分层见 [[architecture]]。

YAML 仍是系统自举和安全基线，但并非所有运行参数都必须永久静态化。统一动态覆盖、字段 apply mode、配置发布、回滚和实例生效状态由 [[adr-system-configuration-control-plane]] 定义；没有显式 applier 的字段仍按启动或重启配置处理。

## 配置文件位置

默认路径：`conf/pole-server.yaml`（可通过 `-c` 参数覆盖）

`utils.ConfDir` 被设置为配置文件所在的目录。

Control Plane、Console 与 Limiter 的目标 mode、兼容发布和顶层 `limiter:` 配置边界见 [[adr-unified-process-mode-and-limiter-integration]]。

## 进程模式

CLI `--mode` 优先于 `bootstrap.mode`。当前兼容期 Profile 为：

| mode | 启动模块 |
|---|---|
| `console` | Console |
| `control-plane` | Control Plane |
| `limiter-server` | Limiter |
| `all` | Control Plane + Console |
| `full` | Control Plane + Limiter + Console |
| `server` | `control-plane` 的弃用别名 |

生产 manifest 应显式指定 mode。推荐三个模块使用同一镜像、不同 workload；`full` 只用于本地、演示和轻量部署。

Limiter 最小配置：

```yaml
limiter:
  registry:
    enable: true
    control-plane-address: 127.0.0.1:8091
    advertised-host: 127.0.0.1
    name: pole-limiter
    namespace: pole-system
    health-check-enable: true
  api-servers:
    - name: grpc
      option:
        ip: 0.0.0.0
        port: 8101
  limit:
    node-id: 1
    max-counter: 1000000
    max-client: 20000
  plugin:
    statistics:
      name: echo
```

每个 Limiter 副本必须配置唯一的 `node-id`。HTTP `8100` 包含运维能力，示例默认不启用；生产应只向内部网络暴露 gRPC `8101`，并正确配置可被 SDK 访问的 `advertised-host`。

## 配置结构（`bootstrap/config/`）

顶层结构：

```yaml
bootstrap:
  logger:
    # 日志配置
  polarisService:
    # 自注册配置
    
apiServers:
  - name: httpserver
    option:
      listenPort: 8080
    api:
      admin:
        enable: true
        include: [...]
      client:
        enable: true
        include: [...]
        
store:
  name: defaultStore
  option:
    master:
      dbType: mysql
      dbName: pole_server
      dbUser: root
      dbPassword: ""
      dbAddr: 127.0.0.1:3306
      maxOpenConns: 100
      maxIdleConns: 10

cache:
  open: true
  resources:
    - name: service
    - name: instance
    - name: routingConfig
    # ... 其他资源缓存

namespace:
  autoCreate: true
  interceptors:
    - auth

service:
  interceptors:
    - paramcheck
    - auth
  batch:
    register:
      open: true
      queueSize: 10000
      waitTime: 32ms
      maxBatchCount: 128
      concurrency: 128
    deregister:
      open: true
      # 类似字段
    heartbeat:
      open: true
      # 类似字段

healthcheck:
  open: true
  service: polaris.checker
  slotNum: 30
  minCheckInterval: 10s
  maxCheckInterval: 60s
  checkers:
    - name: heartbeatChecker

config:
  interceptors:
    - paramcheck
    - auth

goverrule:
  interceptors:
    - paramcheck  
    - auth

auth:
  user:
    name: defaultUser
  strategy:
    name: defaultStrategy

plugin:
  statis:
    name: local
    option:
      interval: 60
  history:
    entries:
      - name: logger
  discoverEvent:
    entries:
      - name: local
  ratelimit:
    name: token-bucket
  whitelist:
    name: whitelist
  crypto:
    entries:
      - name: aes
```

## 插件配置

`apis.Config` 结构体映射到 YAML 中的 `plugin:` 节：

```go
type Config struct {
    CMDB                 ConfigEntry
    RateLimit            ConfigEntry
    History              PluginChanConfig   // 支持多个条目（链式）
    Statis               PluginChanConfig
    DiscoverStatis       ConfigEntry
    ParsePassword        ConfigEntry
    Whitelist            ConfigEntry
    MeshResourceValidate ConfigEntry
    DiscoverEvent        PluginChanConfig
    Crypto               PluginChanConfig
}
```

`PluginChanConfig` 支持 `entries` 数组，用于链式配置多个插件实现。

## API 服务端配置

`apiServers` 列表中每个 API 服务端的配置：
```yaml
- name: httpserver         # 必须与插件 Name() 返回值一致
  option:                  # 传递给 Initialize()
    listenPort: 8080
    enableSwagger: true
  api:
    admin:
      enable: true
      include:
        - defaultAccess
    client:
      enable: true
      include:
        - defaultAccess
```

`include` 列表中的条目是各服务端实现注册的 API 分组名称。

## 环境变量

### Console Pole Agent

`bootstrap.console.agent` 定义 Agent ID、Prompt、模型、MCP endpoint、工具白名单、proposal TTL 与超时。最小真实运行时需要：

- `runtimeMode: llm`；
- OpenAI-compatible LLM Gateway `baseURL` 与 `model`；
- 可解析的 API key；
- 可由 Console 访问的 Pole MCP SSE endpoint；
- 工具白名单至少包含需要开放的只读工具。

LLM Gateway 地址、模型、Prompt、MCP 策略和 API key 现在由 Admin-only `/system-configuration` 页面写入 Pole 内部系统配置库；保存草稿不生效，连接测试成功且发布后才原子切换当前实例。API key 由 Pole SystemSecretStore 信封加密，页面、配置正文、日志和模型上下文均不返回明文。Kubernetes 不再注入每个模型地址或 API key，只保留数据库连接和 `POLE_SYSTEM_SECRET_MASTER_KEY` 根加密材料。未发布有效模型配置时 `/ai/agent/v1/runtime` 返回未就绪，前端禁止发送；这不是降级到本地规则解析。

其余未登记配置仍以 YAML 为准；配置文件路径是基础运行时参数。

## 部署目录

```
deploy/
├── conf/
│   ├── pole-server.yaml     # 示例配置
│   ├── i18n/                # 翻译文件
│   └── plugin/
│       └── ratelimit/       # 限流规则文件
├── kubernetes/              # 本地 K8s 编排、共享依赖适配与 Gateway Route
├── observability/           # 无 Kubernetes 时的独立观测 quickstart
└── tools/                   # 部署脚本
```

进程本地运行时数据统一写入 `./.pole_data/`。当前配置缓存位于
`./.pole_data/cache/`，OTel event/audit 可靠队列位于
`./.pole_data/observability/otel-events/`；该目录不属于部署静态资源，必须整体排除出
Git。Kubernetes 以 `runtime-local-db` 作为唯一运行时数据卷，规范挂载点是
`/app/.pole_data`；兼容期同时将同一卷挂载到 `/app/data`，确保旧镜像和仍覆盖旧路径的
私有配置可以回滚，但两者不再拥有独立存储。

## 本地 Kubernetes 依赖边界

`deploy/kubernetes/` 只在 `pole-system` 运行 Pole Control Plane 和 OpenTelemetry Collector，不重复拥有 MySQL 或 GreptimeDB 的物理生命周期：

- `pole-mysql` 是指向 `host.docker.internal` 的 ExternalName Service。宿主机 MySQL 中的 `pole_server` 承载核心业务数据，MySQL `pole_observability` 承载 Console history/event reader；它们与 GreptimeDB 中同名的观测逻辑库不是同一个数据库实例。
- `pole-greptimedb` 是指向 `maas-greptimedb-frontend.tidemind.svc.cluster.local` 的 ExternalName 适配服务。Collector 和 Console 只依赖 Pole namespace 内的稳定服务名。
- Collector 的 traces、metrics、logs exporter 和 Console query provider 统一使用共享 GreptimeDB 的 `pole_observability` 逻辑库；MaaS 的 `maas_logs`、`maas_metrics` 保持独立。
- 部署脚本会幂等创建 GreptimeDB 逻辑库，等待 Collector 与 Control Plane rollout 成功后再删除旧 standalone StatefulSet；旧 `data-pole-greptimedb-0` PVC 保留，直到历史数据明确导出或放弃。
- 宿主机 `pole-mysql` 容器必须配置 `unless-stopped`。Console 入口拒绝连接且 Pod CrashLoop 时，应先检查前一容器日志、MySQL 状态和 RestartPolicy，再判断是否为 SPA 问题。
- MySQL DSN 必须带 `parseTime=true`。Skill Marketplace 等新表直接把 `TIMESTAMP` 扫描到 `time.Time`；省略该参数会让
  驱动返回 `[]uint8`，表现为 Publisher 或 Release 读取 400/500，而不是 Bundle 格式错误。
- 本地资源稳定性验收依赖集群级 Metrics API。OrbStack 当前使用固定版本 Metrics Server `v0.9.0`；验收必须同时检查
  `v1beta1.metrics.k8s.io` Available 和 `kubectl --context orbstack top node/pod` 返回数值，不能只看 Deployment Ready。

共享观测后端的所有权、数据隔离和迁移理由见 [[adr-otel-observability-platform]]。

正式生产的 Helm、ACK ALB、Kubernetes Secret、GHCR 与 OSS 备份基线见
[[adr-kubernetes-production-deployment]]，逐步操作与验收命令见 [[kubernetes-production-sop]]。

## 相关页面

- [[overview]]
- [[architecture]]
- [[adr-system-configuration-control-plane]]
- [[adr-console-agent-resource-workbench]]
- [[adr-otel-observability-platform]]
- [[adr-unified-process-mode-and-limiter-integration]]
- [[adr-kubernetes-production-deployment]]
- [[kubernetes-production-sop]]
