---
title: ADR：OTel 可观测性平台与 Kubernetes 部署方案
tags: [adr, observability, otel, kubernetes]
links: [architecture, common-infra, configuration, api-servers, adr-pole-rust-client-observability, adr-local-pebble-protobuf-value-cache]
updated: 2026-08-05
sources: 47
---

# ADR：OTel 可观测性平台与 Kubernetes 部署方案

## 状态

Proposed。

## 背景

Pole 平台需要同时覆盖两类可观测性：

- 系统内部可观测性：`pole-control-plane` 自身的内部事件、metrics、平台 trace、操作审计。
- 业务服务调用可观测性：业务服务之间的调用 trace、指标和结构化 event，由 sidecar 与 Rust SDK 上报。

两类数据都需要遵循 OTel/OTLP 协议，避免形成私有采集协议。但平台明确不采集、不存储、不分析业务普通日志；只有结构化 event 可以通过 OTel Logs 上报。

当前仓库已有三类内部观测插件 chain：

- `history.entries`：操作历史与审计记录。
- `discoverEvent.entries`：服务发现相关事件。
- `statis.entries`：统计指标，当前可配置 `local` 与 `prometheus`。

因此，观测平台设计应复用现有 chain 扩展点，而不是新增并列的私有 openobserver 插件体系。

## 决策

采用 `OpenTelemetry Collector Contrib + GreptimeDB + pole-control-plane Console` 的长期默认组合，OpenObserve 保留为 quickstart / 可选 provider：

```text
sidecar / rust-sdk
  -> OTLP metrics / traces / event logs
  -> OpenTelemetry Collector Contrib
  -> GreptimeDB
  -> pole-control-plane Console 查询分析

pole-control-plane
  -> OTLP internal metrics / platform traces / internal event logs / audit logs
  -> OpenTelemetry Collector Contrib
  -> GreptimeDB
  -> pole-control-plane Console 查询分析
```

快速体验或临时验证可以替换为：

```text
sidecar / rust-sdk / pole-control-plane
  -> OpenTelemetry Collector Contrib
  -> OpenObserve
  -> pole-control-plane Console 查询分析 / OpenObserve 内置 UI 辅助排查
```

组件职责：

| 组件 | 职责 |
|---|---|
| sidecar / Rust SDK | 业务侧 metrics、traces、结构化 event 上报；不负责业务普通日志采集 |
| pole-control-plane | server 侧上报平台内部 metrics、trace、event、audit；console 模块读取观测后端并做领域化分析 |
| OpenTelemetry Collector Contrib | 统一 OTLP 入口、processor、过滤、补标签、批量、重试、路由和后端 exporter |
| GreptimeDB | 长期默认 observability database，承载 metrics、event logs、audit logs、traces 的统一存储与 SQL/PromQL 查询 |
| OpenObserve | quickstart / 可选观测平台后端，适合快速体验和用内置 UI 辅助排查 |
| Console | 结合 Pole 的 namespace、service、instance、治理规则、配置版本、用户身份做分析展示 |

Collector 是采集管道，不是查询后端；GreptimeDB 是长期默认存储查询后端，不替代 Collector 的通用 pipeline 能力。OpenObserve 不作为长期默认后端，但可以作为 quickstart provider。小规模体验环境可以允许组件直接写 OpenObserve 或 GreptimeDB，但生产标准路径必须经过 Collector。

## 信号模型

| 数据类型 | OTel 信号 | 来源 | 说明 |
|---|---|---|---|
| 系统内部 metrics | Metrics | pole-control-plane `statis` / `pkg/common/otel/metrics` | API、Store、Cache、插件、任务、SDK 接入量等 |
| 系统内部 trace | Traces | pole-control-plane HTTP/gRPC/Nacos/xDS/Console API | 控制面请求在鉴权、参数校验、业务服务、Store、缓存之间的处理链路 |
| 内部事件 | Logs | pole-control-plane `EventHub` / `discoverEvent` | 服务、实例、配置、治理规则等领域事件，必须结构化 |
| 操作审计 | Logs | pole-control-plane `history` | 用户、资源、动作、结果、时间、请求上下文；保持审计语义，不混入普通日志 |
| 业务 metrics | Metrics | sidecar / Rust SDK | QPS、错误率、延迟、治理命中、熔断、限流、路由结果 |
| 业务 trace | Traces | sidecar / Rust SDK | 服务 A 到服务 B 的调用链、span、错误、耗时和治理上下文 |
| 业务 event | Logs | sidecar / Rust SDK | 实例上下线、配置感知、治理命中、异常事件等结构化事件 |
| 业务普通日志 | 不接入 | 不适用 | stdout/file/INFO/ERROR 文本日志不属于本平台范围 |

OTel Logs 在本方案中只承载结构化 event 和 audit，不等同于通用日志平台。

## OTel Event 本地队列

`history/otel` 和 `discoverEvent/otel` entry 写 Collector 前必须先进入本地可靠队列，避免 Collector、网络或 GreptimeDB 短时异常影响控制面业务链路。

本地队列使用 Pebble：

- 默认路径：`./.pole_data/observability/otel-events/otel-events.pebble`。
- `.pole_data` 是进程本地运行时数据的统一根目录，必须整体排除出 Git；观测队列不再单独写入顶层 `data/`。
- history 与 discover_event 共享 Pebble DB，通过 `otel-queue/{bucket}/` key prefix 隔离。
- 写入语义是 append；Collector 成功确认后再按 key 删除；导出失败保留等待下一轮重试。
- 队列达到容量上限时按配置丢弃并记录 drop metrics，不能阻塞 history/discoverEvent chain。
- 该队列和 [[adr-local-pebble-protobuf-value-cache]] 中的客户端 value cache 都使用 Pebble，但抽象不同：event queue 是 append/ack/delete，value cache 是 key/value hit/miss。

## Metrics 与 Event 命名规范

### 总体原则

- 优先使用 OTel 标准语义约定。HTTP、RPC 等通用请求指标使用 `http.server.request.duration`、`rpc.server.call.duration` 等标准名称或当前 OTel Go instrumentation 输出名称，不另造 Pole 私有名称。
- Pole 自定义领域指标统一使用 `pole.` 前缀，按 `pole.<domain>.<object>.<measure>` 分层，例如 `pole.discovery.service.count`。
- 指标名称使用小写点分层，不把单位写进名称；单位通过 OTel instrument unit 表达，例如 `s`、`ms`、`By`、`{request}`、`{event}`。
- Event 使用 OTel LogRecord 表达，必须设置低基数 EventName；动态对象 ID、资源名、错误详情放入 attributes。后端查询层如需要兼容旧字段，可以把 EventName 映射为 `event.name` 查询列，但上报规范不把 `event.name` 作为主字段。
- metrics label 必须低基数；高基数字段只允许进入 event 或 trace attributes。
- 业务普通日志不进入本规范。即使后端能接收 logs，也只允许结构化业务 event 和平台 audit/event。

### 通用属性

通用属性分为 Resource attributes 和 Signal attributes。

Resource attributes 在 SDK、sidecar、pole-control-plane 进程级设置：

| 属性 | 示例 | 说明 |
|---|---|---|
| `service.name` | `pole-control-plane` / `order-service` | OTel 标准服务名 |
| `service.version` | `1.2.0` | 服务版本 |
| `service.instance.id` | `pod-uid` / instance id | 服务实例 ID |
| `deployment.environment.name` | `dev` / `prod` | 环境名 |
| `k8s.namespace.name` | `pole-system` | Kubernetes namespace |
| `k8s.pod.name` | `pole-control-plane-0` | Pod 名称 |
| `k8s.cluster.name` | `local-kind` | 集群名 |
| `pole.cluster` | `default` | Pole 逻辑集群 |
| `pole.node.role` | `control-plane` / `sidecar` / `sdk` | 上报组件角色 |
| `pole.runtime.language` | `java` / `go` / `rust` | Pole 归一化语言体系，用于 Console 选择运行时面板 |
| `process.runtime.name` | `OpenJDK Runtime Environment` / `go` | OTel 标准运行时名称 |
| `process.runtime.version` | `17.0.10` / `go1.22.5` | OTel 标准运行时版本 |
| `process.runtime.description` | `OpenJDK 64-Bit Server VM ...` | OTel 标准运行时描述 |
| `telemetry.sdk.language` | `java` / `go` / `rust` | OTel SDK 语言；用于辅助判断，不替代 `pole.runtime.language` |

Signal attributes 随单条 metric、event 或 span 设置：

| 属性 | metrics | event | trace | 说明 |
|---|---|---|---|---|
| `pole.namespace` | 是 | 是 | 是 | Pole 命名空间，低基数 |
| `pole.service.name` | 是 | 是 | 是 | Pole 服务名，低基数到中基数，允许作为主要聚合维度 |
| `pole.service.instance.id` | 否 | 是 | 是 | 实例 ID，高基数，不进入 metrics |
| `pole.instance.host` | 否 | 是 | 是 | 实例 IP/host，高基数，不进入 metrics |
| `pole.instance.port` | 否 | 是 | 是 | 实例端口 |
| `pole.protocol` | 是 | 是 | 是 | `http` / `grpc` / `nacos` / `xds` / `apollo` / `eureka` |
| `pole.api.name` | 是 | 是 | 是 | 归一化 API 名称，不能放原始 path |
| `pole.component` | 是 | 是 | 是 | `apiserver` / `store` / `cache` / `config` / `governance` 等 |
| `pole.resource.type` | 是 | 是 | 是 | `namespace` / `service` / `instance` / `config_group` / `route_rule` 等 |
| `pole.resource.name` | 否 | 是 | 是 | 资源名，高基数，不进入 metrics |
| `pole.operation` | 是 | 是 | 是 | `create` / `update` / `delete` / `publish` / `rollback` 等 |
| `pole.result` | 是 | 是 | 是 | `success` / `failure` |
| `pole.error.code` | 是 | 是 | 是 | 归一化错误码，不能使用错误文本 |
| `pole.rule.type` | 是 | 是 | 是 | `route` / `rate_limit` / `circuit_breaker` / `fault_detect` / `lane` / `lossless` / `security` / `mirror` / `mock` |
| `pole.rule.id` | 否 | 是 | 是 | 规则 ID，高基数，不进入 metrics |
| `pole.config.group` | 是 | 是 | 是 | 配置分组，低基数到中基数 |
| `pole.config.file` | 否 | 是 | 是 | 配置文件名，高基数，不进入 metrics |
| `pole.audit.user.id` | 否 | 是 | 是 | 操作人 ID，高基数，不进入 metrics |
| `pole.audit.user.name` | 否 | 是 | 是 | 操作人名称，不进入 metrics |
| `pole.governance.decision.id` | 否 | 是 | 是 | 单次治理决策 ID，高基数，不进入 metrics |
| `trace_id` / `span_id` | 否 | 是 | 是 | 链路关联字段，不进入 metrics |

### Metrics 名称

通用请求类如果可以由 OTel instrumentation 直接生成，优先使用标准指标；Pole 自定义只补充领域聚合指标。

#### 平台内部 metrics

| 指标名 | 类型 | 单位 | 主要属性 | 说明 |
|---|---|---|---|---|
| `pole.control_plane.request.count` | Counter | `{request}` | `pole.protocol`, `pole.api.name`, `pole.result`, `pole.error.code` | 控制面 API 请求数；可由现有 `ServerCallMetric` 映射 |
| `pole.control_plane.request.duration` | Histogram | `s` | `pole.protocol`, `pole.api.name`, `pole.result`, `pole.error.code` | 控制面 API 耗时；可由现有 `CallMetric.Duration` 转换映射 |
| `pole.control_plane.store.request.count` | Counter | `{request}` | `pole.component=store`, `pole.operation`, `pole.result`, `pole.error.code` | Store 调用次数 |
| `pole.control_plane.store.request.duration` | Histogram | `s` | `pole.operation`, `pole.result`, `pole.error.code` | Store 调用耗时 |
| `pole.control_plane.cache.update.duration` | Histogram | `s` | `pole.cache.type`, `pole.result` | 缓存刷新耗时；替代旧 `cache_update_cost` 的 OTel entry 名称 |
| `pole.control_plane.cache.update.count` | Counter | `{update}` | `pole.cache.type`, `pole.result` | 缓存刷新次数 |
| `pole.control_plane.batch.job.pending` | UpDownCounter 或 Gauge | `{job}` | `pole.batch.name` | 批处理未完成任务数；替代旧 `batch_job_unfinish` 的 OTel entry 名称 |
| `pole.control_plane.instance.register.duration` | Histogram | `s` | `pole.result` | 实例异步注册任务耗时；替代旧 `instance_regis_cost_time` 的 OTel entry 名称 |
| `pole.control_plane.instance.register.dropped` | Counter | `{task}` | `pole.drop.reason` | 注册任务丢弃数；替代旧 `instance_regis_task_expire` 的 OTel entry 名称 |
| `pole.control_plane.client.connection.count` | Gauge | `{connection}` | `pole.client.kind` | SDK/配置/发现连接数；`pole.client.kind=discovery/config/sdk` |
| `pole.discovery.service.count` | Gauge | `{service}` | `pole.namespace`, `pole.service.status` | 服务数量；状态为 `total/online/offline/abnormal` |
| `pole.discovery.instance.count` | Gauge | `{instance}` | `pole.namespace`, `pole.service.name`, `pole.instance.status` | 实例数量；状态为 `total/online/offline/abnormal/isolate` |
| `pole.config.group.count` | Gauge | `{group}` | `pole.namespace` | 配置分组数量 |
| `pole.config.file.count` | Gauge | `{file}` | `pole.namespace`, `pole.config.group` | 配置文件数量 |
| `pole.config.file.release.count` | Gauge | `{release}` | `pole.namespace`, `pole.config.group`, `pole.release.type` | 配置发布数量 |
| `pole.control_plane.process.uptime` | Gauge | `s` | `pole.node.role` | control-plane 进程运行时间 |
| `pole.control_plane.build.info` | Gauge | `{build}` | `service.version`, `git.commit`, `build.time` | 版本信息，值固定为 1 |
| `pole.control_plane.node.ready` | Gauge | `{state}` | `pole.node.role`, `pole.ready.state` | 节点是否可服务 |
| `pole.control_plane.plugin.ready` | Gauge | `{state}` | `pole.plugin.type`, `pole.plugin.name`, `pole.ready.state` | 插件初始化和运行状态 |
| `pole.control_plane.startup.duration` | Histogram | `s` | `pole.startup.phase`, `pole.result` | 启动阶段耗时，例如 config/store/cache/plugin/server |
| `pole.control_plane.config.load.count` | Counter | `{load}` | `pole.config.source`, `pole.result`, `pole.error.code` | 配置加载或重载次数 |
| `pole.control_plane.eventhub.publish.count` | Counter | `{event}` | `pole.event.topic`, `pole.result` | EventHub 发布次数 |
| `pole.control_plane.eventhub.dispatch.duration` | Histogram | `s` | `pole.event.topic`, `pole.result` | EventHub handler 分发耗时 |
| `pole.control_plane.queue.depth` | Gauge | `{item}` | `pole.queue.name`, `pole.component` | 异步队列深度 |
| `pole.control_plane.queue.dropped.count` | Counter | `{item}` | `pole.queue.name`, `pole.drop.reason` | 队列丢弃数量 |
| `pole.control_plane.healthcheck.probe.count` | Counter | `{probe}` | `pole.probe.type`, `pole.result` | 健康检查探测次数 |
| `pole.control_plane.healthcheck.probe.duration` | Histogram | `s` | `pole.probe.type`, `pole.result` | 健康检查探测耗时 |
| `pole.control_plane.healthcheck.status.change.count` | Counter | `{event}` | `pole.namespace`, `pole.service.name`, `pole.instance.status` | 健康状态变化聚合，不带实例 ID |
| `pole.config.watch.connection.count` | Gauge | `{connection}` | `pole.protocol`, `pole.namespace` | 配置 watch 长连接数量 |
| `pole.config.watch.notify.count` | Counter | `{notification}` | `pole.protocol`, `pole.namespace`, `pole.config.group`, `pole.result` | 配置变更通知次数 |
| `pole.config.publish.duration` | Histogram | `s` | `pole.namespace`, `pole.config.group`, `pole.release.type`, `pole.result` | 配置发布耗时 |
| `pole.config.publish.count` | Counter | `{publish}` | `pole.namespace`, `pole.config.group`, `pole.release.type`, `pole.result` | 配置发布次数 |
| `pole.governance.rule.count` | Gauge | `{rule}` | `pole.namespace`, `pole.rule.type`, `pole.rule.status` | 治理规则数量 |
| `pole.governance.rule.publish.count` | Counter | `{publish}` | `pole.namespace`, `pole.rule.type`, `pole.result` | 治理规则发布次数 |
| `pole.governance.rule.publish.duration` | Histogram | `s` | `pole.namespace`, `pole.rule.type`, `pole.result` | 治理规则发布耗时 |
| `pole.governance.rule.cache.update.count` | Counter | `{update}` | `pole.rule.type`, `pole.result` | 治理规则缓存刷新次数 |
| `pole.control_plane.push.count` | Counter | `{push}` | `pole.protocol`, `pole.resource.type`, `pole.result` | xDS、Nacos、配置 watch 等推送次数 |
| `pole.control_plane.push.duration` | Histogram | `s` | `pole.protocol`, `pole.resource.type`, `pole.result` | 推送耗时 |
| `pole.control_plane.push.failure.count` | Counter | `{push}` | `pole.protocol`, `pole.resource.type`, `pole.error.code` | 推送失败次数 |
| `pole.control_plane.protocol.connection.count` | Gauge | `{connection}` | `pole.protocol`, `pole.connection.state` | 协议服务连接数 |
| `pole.auth.request.count` | Counter | `{request}` | `pole.auth.kind`, `pole.resource.type`, `pole.result`, `pole.error.code` | 认证/授权请求次数 |
| `pole.auth.request.duration` | Histogram | `s` | `pole.auth.kind`, `pole.resource.type`, `pole.result` | 认证/授权耗时 |
| `pole.auth.decision.count` | Counter | `{decision}` | `pole.auth.action`, `pole.resource.type`, `pole.result` | 授权决策次数 |
| `pole.auth.token.validation.count` | Counter | `{validation}` | `pole.token.kind`, `pole.result`, `pole.error.code` | Token 校验次数 |
| `pole.control_plane.store.connection.count` | Gauge | `{connection}` | `pole.store.name`, `pole.connection.state` | DB 连接池连接数，state 为 active/idle/open |
| `pole.control_plane.store.connection.wait.duration` | Histogram | `s` | `pole.store.name` | 等待 DB 连接耗时 |
| `pole.control_plane.store.slow_query.count` | Counter | `{query}` | `pole.store.name`, `pole.operation` | 慢查询次数，不带 SQL 文本 |
| `pole.telemetry.export.count` | Counter | `{export}` | `otel.signal`, `pole.result` | OTel 导出次数 |
| `pole.telemetry.export.failure.count` | Counter | `{export}` | `otel.signal`, `pole.error.code` | OTel 导出失败次数 |
| `pole.telemetry.export.queue.depth` | Gauge | `{item}` | `otel.signal` | telemetry exporter 队列深度 |

#### 业务服务调用 metrics

业务侧 metrics 由 sidecar 和 Rust SDK 上报，control-plane 只读取分析。

| 指标名 | 类型 | 单位 | 主要属性 | 说明 |
|---|---|---|---|---|
| `pole.service.request.count` | Counter | `{request}` | `pole.namespace`, `pole.service.name`, `pole.callee.service.name`, `pole.protocol`, `pole.result`, `pole.error.code` | 服务间调用请求数 |
| `pole.service.request.duration` | Histogram | `s` | `pole.namespace`, `pole.service.name`, `pole.callee.service.name`, `pole.protocol`, `pole.result` | 服务间调用耗时 |
| `pole.service.request.inflight` | UpDownCounter 或 Gauge | `{request}` | `pole.namespace`, `pole.service.name`, `pole.callee.service.name` | 进行中的调用数 |
| `pole.service.governance.route.match.count` | Counter | `{match}` | `pole.namespace`, `pole.service.name`, `pole.callee.service.name`, `pole.rule.type`, `pole.result` | 路由/泳道/镜像/Mock 等治理规则命中数；不带 `rule.id` |
| `pole.service.governance.rate_limit.count` | Counter | `{request}` | `pole.namespace`, `pole.service.name`, `pole.callee.service.name`, `pole.result` | 限流通过/拒绝数 |
| `pole.service.governance.circuit_breaker.count` | Counter | `{request}` | `pole.namespace`, `pole.service.name`, `pole.callee.service.name`, `pole.result` | 熔断通过/拒绝/半开等结果数 |
| `pole.service.config.watch.count` | Counter | `{event}` | `pole.namespace`, `pole.service.name`, `pole.config.group`, `pole.result` | SDK 感知配置变更次数；不带具体文件名 |
| `pole.service.discovery.refresh.count` | Counter | `{refresh}` | `pole.namespace`, `pole.service.name`, `pole.result` | SDK/sidecar 服务发现刷新次数 |
| `pole.service.cpu.utilization` | Gauge | `1` | `pole.namespace`, `pole.service.name`, `pole.workload.name`, `k8s.namespace.name` | 业务服务 CPU 使用率，由 Kubernetes/Collector 资源指标聚合得到 |
| `pole.service.cpu.usage` | Gauge | `s` | `pole.namespace`, `pole.service.name`, `pole.workload.name`, `k8s.namespace.name` | 业务服务 CPU 累计使用时间或速率视图 |
| `pole.service.memory.usage` | Gauge | `By` | `pole.namespace`, `pole.service.name`, `pole.workload.name`, `k8s.namespace.name` | 业务服务内存使用量 |
| `pole.service.memory.limit` | Gauge | `By` | `pole.namespace`, `pole.service.name`, `pole.workload.name`, `k8s.namespace.name` | 业务服务内存限制 |
| `pole.service.pod.ready.count` | Gauge | `{pod}` | `pole.namespace`, `pole.service.name`, `pole.workload.name`, `k8s.namespace.name` | Ready Pod 数量 |

业务调用 metrics 中：

- `pole.service.name` 表示调用方服务。
- `pole.callee.service.name` 表示被调方服务。
- 不允许使用 `instance.id`、`host`、`path`、`rule.id`、`config.file`、`trace_id`、`request_id` 作为 metrics label。
- 需要按实例或具体规则排查时，通过 trace 和 event 关联。

### 业务 CPU/Mem 与请求监控联动

业务 CPU/Mem 指标不由 `pole-control-plane` 采集，也不由 SDK 强行读取宿主机资源。Kubernetes 部署模式下，资源指标由 OpenTelemetry Collector 的 Kubernetes/kubelet 相关 receiver 采集，再通过 Kubernetes resource attributes 与 Pole 业务维度关联。

#### 服务绑定标签与语言体系

Pole 服务和实例需要预留一组绑定标签，用来把业务请求 metrics、runtime metrics、Kubernetes CPU/Mem metrics、trace 和 event 归属到同一个服务。标签建议由 sidecar、SDK、部署模板或 control-plane 自动注入，不能完全依赖用户手填。

保留标签：

| Pole 服务/实例标签 | OTel attribute | 说明 |
|---|---|---|
| `pole.io/namespace` | `pole.namespace` | Pole 命名空间 |
| `pole.io/service` | `pole.service.name` | Pole 服务名 |
| `pole.io/instance-id` | `pole.service.instance.id` / `service.instance.id` | Pole 实例 ID |
| `pole.io/workload-name` | `k8s.workload.name` | Kubernetes workload 名称 |
| `pole.io/workload-kind` | `k8s.workload.kind` | Deployment/StatefulSet/DaemonSet 等 |
| `pole.io/k8s-namespace` | `k8s.namespace.name` | Kubernetes namespace |
| `pole.io/pod-uid` | `k8s.pod.uid` | Pod UID |
| `pole.io/pod-name` | `k8s.pod.name` | Pod 名称 |
| `pole.io/container-name` | `container.name` | 容器名 |
| `pole.io/cluster` | `k8s.cluster.name` / `pole.cluster` | 集群名 |
| `pole.io/runtime-language` | `pole.runtime.language` | 归一化语言体系，取值如 `java/go/rust/nodejs/python/dotnet` |
| `pole.io/runtime-name` | `process.runtime.name` | 运行时名称 |
| `pole.io/runtime-version` | `process.runtime.version` | 运行时版本 |

绑定优先级：

1. Pod/实例级：`k8s.pod.uid` + `container.name`，用于实例详情和调用链详情。
2. Pole 实例级：`pole.service.instance.id` / `service.instance.id`，用于实例事件、trace、runtime metrics 关联。
3. Workload 级：`pole.namespace` + `pole.service.name` + `k8s.workload.name`，用于资源指标聚合。
4. 服务级：`pole.namespace` + `pole.service.name`，用于服务概览。

#### 语言运行时 metrics 展示

语言运行时 metrics 使用 OTel 或语言 instrumentation 的标准名称，不新增 `pole.*` runtime 指标名。`pole.runtime.language` 只决定 Console 默认展示哪些指标组，以及查询时追加哪些绑定属性。

| `pole.runtime.language` | 默认 runtime 指标 | Console 展示重点 |
|---|---|---|
| `java` | `jvm.memory.used`, `jvm.memory.committed`, `jvm.memory.limit`, `jvm.memory.used_after_last_gc`, `jvm.gc.duration`, `jvm.thread.count`, `jvm.class.loaded`, `jvm.class.unloaded`, `jvm.class.count`, `jvm.cpu.time`, `jvm.cpu.count`, `jvm.cpu.recent_utilization` | Heap/Non-Heap、GC 次数和耗时、线程、类加载、JVM CPU |
| `go` | `go.memory.used`, `go.memory.limit`, `go.memory.allocated`, `go.memory.allocations`, `go.memory.gc.goal`, `go.memory.gc.cycles`, `go.memory.gc.pause.duration`, `go.cpu.time`, `go.goroutine.count`, `go.processor.limit`, `go.schedule.duration`, `go.config.gogc` | 内存、GC pause、goroutine、processor、scheduler |
| `rust` | 暂不定义统一 runtime 标准指标；优先展示进程/容器 CPU/Mem、请求指标、trace/event | 请求、资源、panic/error event、线程/自定义指标 |
| `nodejs` | 按 Node.js OTel instrumentation 输出的标准 runtime/process 指标 | Event loop、heap、GC、资源指标 |
| `python` | 按 Python OTel instrumentation 输出的标准 runtime/process 指标 | 进程资源、GC、线程/协程视 instrumentation 能力展示 |
| `dotnet` | 按 .NET OTel instrumentation 输出的 runtime/process 指标 | GC、线程池、异常、资源指标 |

如果服务没有 `pole.runtime.language`：

- Console 先尝试用 `telemetry.sdk.language`、`process.runtime.name` 推断。
- 推断失败时只展示通用请求指标、Kubernetes CPU/Mem、trace 和 event。
- 不允许通过 metric name 猜测并回写服务语言标签；语言标签应由服务元数据或 instrumentation resource 明确声明。

推荐链路：

```text
kubelet / Kubernetes API
  -> OpenTelemetry Collector kubeletstats / k8s receivers
  -> resource processor 补充 k8s.pod.uid、k8s.namespace.name、k8s.workload.name
  -> Pole enrichment：由 pod label / env / SDK resource 补 pole.namespace、pole.service.name、pole.service.instance.id
  -> GreptimeDB
  -> Console 同屏展示资源指标与请求指标
```

关联键优先级：

1. `service.instance.id` 与 `k8s.pod.uid` 一致时，按实例精确关联。
2. 如果业务进程和 sidecar 共 Pod，按 `k8s.pod.uid` + `container.name` 关联。
3. 如果只有工作负载级数据，按 `pole.namespace` + `pole.service.name` + `k8s.workload.name` 聚合关联。
4. 如果无法拿到 Pole 服务标签，Console 只能展示 Kubernetes workload 资源指标，不能强行归属到 Pole 服务。

Console 联动视图：

- 服务概览：同一时间窗展示 `pole.service.request.count`、`pole.service.request.duration`、`pole.service.cpu.utilization`、`pole.service.memory.usage`。
- 调用链详情：span 上携带 `service.instance.id` / `k8s.pod.uid` 时，展示该实例在 span 时间窗前后的 CPU/Mem 曲线。
- 异常分析：当错误率或 P99 延迟上升时，自动叠加 CPU throttling、内存接近 limit、Pod restart、Ready 数变化。
- Runtime 分析：根据 `pole.runtime.language` 展示 JVM/Go/Node/Python/.NET 等语言特定面板；runtime 指标与请求指标通过同一组服务绑定属性关联。
- 实例详情：实例事件、资源指标、请求指标、治理命中事件按时间线合并。

资源指标 label 仍必须控制基数：服务级资源指标默认不带 pod 名称；实例详情需要 pod 级数据时通过 event/trace 或专门详情查询读取，不作为主聚合 metrics label。

### 治理 metrics 与治理 event 联动

治理 metrics 用于低基数聚合，治理 event 用于高基数定位，两者必须成对设计。

治理 metrics 只保留可聚合维度：

| 指标 | 允许属性 | 禁止属性 |
|---|---|---|
| `pole.service.governance.route.match.count` | `pole.namespace`, `pole.service.name`, `pole.callee.service.name`, `pole.rule.type`, `pole.result` | `pole.rule.id`, `trace_id`, `request_id`, `instance.id` |
| `pole.service.governance.rate_limit.count` | `pole.namespace`, `pole.service.name`, `pole.callee.service.name`, `pole.result` | 具体规则 ID、客户端 IP、请求 path |
| `pole.service.governance.circuit_breaker.count` | `pole.namespace`, `pole.service.name`, `pole.callee.service.name`, `pole.result` | 具体实例 ID、错误文本 |
| `pole.governance.rule.publish.count` | `pole.namespace`, `pole.rule.type`, `pole.result` | 具体规则 ID |

治理 event 承载具体定位字段：

| EventName | 关联 metrics | 必带关联属性 |
|---|---|---|
| `pole.service.governance.rule.matched` | `pole.service.governance.route.match.count` | `pole.governance.decision.id`, `pole.rule.type`, `pole.rule.id`, `trace_id`, `span_id`, `pole.service.name`, `pole.callee.service.name` |
| `pole.service.governance.request.rejected` | `pole.service.governance.rate_limit.count` / `pole.service.governance.circuit_breaker.count` | `pole.governance.decision.id`, `pole.rule.type`, `pole.rule.id`, `pole.reject.reason`, `trace_id`, `span_id` |
| `pole.governance.rule.changed` | `pole.governance.rule.publish.count` | `pole.rule.type`, `pole.rule.id`, `pole.operation`, `pole.audit.user.id` |

关联规则：

- 单次治理决策生成 `pole.governance.decision.id`，写入 span attributes 和治理 event attributes；metrics 不带该字段。
- metrics 与 event 通过时间窗、`pole.namespace`、`pole.service.name`、`pole.callee.service.name`、`pole.rule.type`、`pole.result` 关联。
- 进入详情页后，再用 event 中的 `pole.rule.id`、`trace_id`、`span_id` 查询具体规则、链路和实例上下文。
- Console 的治理分析不从 metrics 反推出具体规则；必须跳转到治理 event 或 trace 做精确定位。

### Event 名称

EventName 统一使用 `pole.<domain>.<object>.<action>`。事件 body 可以为空，所有可查询字段必须放 attributes。

#### 平台内部 event

| EventName | 来源 | Severity | 关键属性 | 说明 |
|---|---|---|---|---|
| `pole.discovery.instance.online` | `InstanceOnline` | INFO | `pole.namespace`, `pole.service.name`, `pole.service.instance.id`, `pole.instance.host`, `pole.instance.port` | 实例上线 |
| `pole.discovery.instance.offline` | `InstanceOffline` | WARN | 同上，另加 `pole.reason` | 实例下线 |
| `pole.discovery.instance.health_changed` | `InstanceTurnHealth` / `InstanceTurnUnHealth` | INFO/WARN | `pole.namespace`, `pole.service.name`, `pole.service.instance.id`, `pole.instance.health.status` | 健康状态变化 |
| `pole.discovery.instance.isolate_changed` | `InstanceOpenIsolate` / `InstanceCloseIsolate` | INFO | `pole.namespace`, `pole.service.name`, `pole.service.instance.id`, `pole.instance.isolate.status` | 隔离状态变化 |
| `pole.discovery.service.empty_push_protection_changed` | `ServiceOpenEmptyPushProtect` 等 | WARN/INFO | `pole.namespace`, `pole.service.name`, `pole.protection.status` | 推空保护状态变化 |
| `pole.config.file.published` | config release | INFO | `pole.namespace`, `pole.config.group`, `pole.config.file`, `pole.release.type`, `pole.release.name`, `pole.release.version` | 配置发布 |
| `pole.config.file.rollback` | config release rollback | WARN | 同上 | 配置回滚 |
| `pole.governance.rule.changed` | governance rule change | INFO | `pole.namespace`, `pole.rule.type`, `pole.rule.id`, `pole.operation` | 治理规则变更 |
| `pole.control_plane.leader.changed` | leader election | WARN | `pole.leader.old`, `pole.leader.new` | 控制面主节点变化 |
| `pole.control_plane.cache.refresh_failed` | cache refresh | ERROR | `pole.cache.type`, `pole.error.code`, `error.message` | 缓存刷新失败 |

#### 操作审计 event

审计事件统一归一为一个低基数事件名：

| EventName | 来源 | Severity | 关键属性 | 说明 |
|---|---|---|---|---|
| `pole.audit.operation` | `history.RecordEntry` | INFO/WARN/ERROR | `pole.resource.type`, `pole.resource.name`, `pole.namespace`, `pole.operation`, `pole.audit.user.id`, `pole.audit.user.name`, `pole.result`, `pole.error.code` | 管理面操作审计 |

审计事件不按每种资源和操作拆 event name，避免事件名维度膨胀。资源类型和操作类型放 attributes。

`RecordEntry` 映射：

| `RecordEntry` 字段 | OTel attribute |
|---|---|
| `ResourceType` | `pole.resource.type` |
| `ResourceName` | `pole.resource.name` |
| `Namespace` | `pole.namespace` |
| `Operator` | `pole.audit.user.name`，后续若有用户 ID 则补 `pole.audit.user.id` |
| `OperationType` | `pole.operation` |
| `Detail` | `pole.audit.detail` |
| `Server` | `service.instance.id` 或 `pole.server.node` |
| `HappenTime` | LogRecord timestamp |

#### 业务服务 event

业务 event 由 sidecar / Rust SDK 上报，control-plane 只展示和分析。

| EventName | 来源 | Severity | 关键属性 | 说明 |
|---|---|---|---|---|
| `pole.service.instance.registered` | SDK/sidecar | INFO | `pole.namespace`, `pole.service.name`, `pole.service.instance.id` | 业务实例注册成功 |
| `pole.service.instance.deregistered` | SDK/sidecar | WARN | 同上，另加 `pole.reason` | 业务实例反注册 |
| `pole.service.discovery.updated` | SDK/sidecar | INFO | `pole.namespace`, `pole.service.name`, `pole.callee.service.name`, `pole.revision` | 服务发现结果更新 |
| `pole.service.config.changed` | SDK/sidecar | INFO | `pole.namespace`, `pole.service.name`, `pole.config.group`, `pole.config.file`, `pole.revision` | 配置变更被业务侧感知 |
| `pole.service.governance.rule.matched` | SDK/sidecar | INFO | `pole.namespace`, `pole.service.name`, `pole.callee.service.name`, `pole.rule.type`, `pole.rule.id`, `pole.rule.name`, `trace_id` | 治理规则命中；可带 `rule.id` |
| `pole.service.governance.request.rejected` | SDK/sidecar | WARN | `pole.namespace`, `pole.service.name`, `pole.callee.service.name`, `pole.rule.type`, `pole.rule.id`, `pole.reject.reason`, `trace_id` | 限流、熔断、鉴权等导致请求拒绝 |
| `pole.service.call.failed` | SDK/sidecar | ERROR | `pole.namespace`, `pole.service.name`, `pole.callee.service.name`, `pole.protocol`, `pole.error.code`, `trace_id` | 业务调用失败事件 |

业务 event 可携带 `rule.id`、`config.file`、`instance.id`、`trace_id` 等高基数字段，因为它们用于精确定位；同类字段不能进入 metrics label。

### 命名兼容策略

现有 `prometheus` entry 已经注册了 `client_total`、`service_count`、`instance_count`、`cache_update_cost` 等历史名称。本 ADR 不要求立即改名，避免破坏现有看板。

后续新增 `otel` entry 时：

- 对外输出采用本节 `pole.*` 新名称。
- 旧 `local` / `prometheus` entry 保持原行为。
- 如需要兼容老 Prometheus 看板，可以在 Collector 或后端查询层增加重命名/别名规则，不把旧名称继续扩散到 SDK/sidecar。

## Control-Plane 内部扩展方式

保持现有观测 chain：

```yaml
plugin:
  history:
    entries:
      - name: HistoryLogger
      - name: otel
  discoverEvent:
    entries:
      - name: EventLogger
      - name: otel
  statis:
    entries:
      - name: local
      - name: prometheus
      - name: otel
```

扩展原则：

- `history` 的 `otel` entry 将审计记录转换为结构化 OTel LogRecord。
- `discoverEvent` 的 `otel` entry 将领域事件转换为结构化 OTel LogRecord，设置 EventName、资源标识和 namespace/service 维度。
- `statis` 的 `otel` entry 将内部统计转换为 OTel Metrics。
- 平台 trace 通过 HTTP/gRPC/Nacos/xDS/Console API 拦截器补齐，不从 `statis` 或 `history` 派生。
- 不新增独立 `openobserver` 插件类型，避免绕开现有插件配置与 chain 扩展模型。

## Console 查询分析

Console 不直接查询 Collector。`/observability/v1` 由 `pole-control-plane` 内的 console 模块负责提供，在 console 后端增加 `observability-query` 适配层：

```text
Console 页面
  -> pole-control-plane console 模块 /observability/v1
  -> GreptimeDB / OpenObserve 查询 API
  -> Pole 元数据补充
```

查询分析能力分为四组：

- 服务观测：服务拓扑、调用链、接口延迟、错误率、治理命中情况。
- 实例观测：实例健康、上下线事件、SDK 版本、sidecar 版本、流量状态、CPU/Mem 资源曲线。
- 平台观测：控制面 API、Store、Cache、插件、任务、配置发布链路。
- 审计观测：用户操作、资源变更、失败原因、请求上下文和关联 trace。

Console 必须通过 Pole 元数据增强原始观测数据，例如将 trace/span 的 `service.name`、`namespace`、`instance.id`、`rule.id` 关联到控制台中的服务、实例、治理规则和配置版本。

Console 联动原则：

- 服务详情页默认按同一时间窗合并请求 metrics、CPU/Mem metrics、实例事件和治理事件。
- 请求异常分析先看 `pole.service.request.*`，再按 `service.instance.id` / `k8s.pod.uid` 叠加 Pod/Container CPU、内存、Ready 状态。
- 治理分析页先用 `pole.service.governance.*` metrics 找到异常时间窗和规则类型，再用治理 event 的 `pole.rule.id`、`pole.governance.decision.id`、`trace_id` 定位具体规则和调用链。
- 审计页通过 `pole.audit.operation` 关联治理规则变更 event、规则发布 metrics 和后续业务侧治理命中变化。

### Console 查询 API

现有 Console 已有 `/metrics/v1` 下的历史监控接口，例如 labels、接口描述、事件日志和操作历史。后续 OTel 方案不应继续扩大 `/metrics/v1` 的职责，而是由 console 模块新增 `/observability/v1` 作为统一观测查询分组。`/metrics/v1` 可以保留兼容旧页面，新的服务观测、平台观测、trace、event、audit 和关联分析都走 `/observability/v1`。

接口分层：

| 层级 | 职责 |
|---|---|
| Console 前端 | 只传 Pole 领域筛选条件和时间窗，不拼 GreptimeDB SQL、PromQL 或 OpenObserve 查询语句 |
| console 模块 `observability-query` API | 做鉴权、参数归一、默认时间窗、分页、字段白名单、跨信号关联和 Pole 元数据补全 |
| Provider | 适配 GreptimeDB / OpenObserve / Prometheus+Tempo+Logs 等后端查询语法 |
| Pole 元数据 | 服务、实例、命名空间、治理规则、配置版本、用户和权限上下文 |

统一请求参数：

| 参数 | 类型 | 说明 |
|---|---|---|
| `start_time` / `end_time` | int64 | 毫秒时间戳，必填；后端限制最大时间跨度 |
| `step` | string | 聚合步长，例如 `30s`、`1m`、`5m`；未传由后端按时间窗推导 |
| `namespace` | string | Pole namespace，服务侧查询必填 |
| `service` | string | 调用方或目标服务名，服务详情必填 |
| `callee_service` | string | 被调服务名，调用关系查询可选 |
| `instance_id` | string | 只允许详情查询使用，不允许主聚合接口使用 |
| `runtime_language` | string | 可选；未传时由服务元数据或 Resource attributes 推断 |
| `rule_type` | string | 治理规则类型 |
| `rule_id` | string | 只允许 event/trace/detail 查询使用 |
| `trace_id` / `span_id` | string | trace 与 event 详情关联 |
| `limit` / `cursor` | int/string | 列表分页 |

统一响应外壳沿用 Console 当前响应习惯：

```json
{
  "code": 200000,
  "info": "success",
  "data": {},
  "request_id": "optional"
}
```

时间序列统一响应：

```json
{
  "series": [
    {
      "name": "p99",
      "unit": "ms",
      "labels": {
        "pole.service.name": "order"
      },
      "points": [
        {
          "timestamp": 1720000000000,
          "value": 12.3
        }
      ]
    }
  ]
}
```

首期接口：

| 方法 | 路径 | 用途 | 后端信号 |
|---|---|---|---|
| GET | `/observability/v1/services/overview` | 服务概览：请求量、错误率、P50/P90/P99、CPU/Mem、实例数、治理命中摘要 | metrics + Pole 元数据 |
| GET | `/observability/v1/services/topology` | 服务调用拓扑，按 caller/callee 聚合请求量、错误率、延迟和治理命中 | metrics + trace optional |
| GET | `/observability/v1/services/metrics` | 服务请求与资源指标时间序列 | metrics |
| GET | `/observability/v1/services/runtime` | 按 `pole.runtime.language` 返回 JVM/Go/Rust/Node/Python/.NET 面板需要的指标组 | metrics |
| GET | `/observability/v1/services/events` | 业务结构化 event 列表：发现、配置、治理、调用失败等 | logs events |
| GET | `/observability/v1/services/traces` | 服务调用 trace 列表，支持服务、被调服务、错误、延迟阈值过滤 | traces |
| GET | `/observability/v1/traces/:trace_id` | trace 详情，返回 span 树、关键 attributes、关联事件 | traces + logs events |
| GET | `/observability/v1/governance/overview` | 治理概览：规则类型、命中、拒绝、异常趋势 | metrics |
| GET | `/observability/v1/governance/events` | 治理 event 列表，支持 `rule_id`、`decision_id`、`trace_id` 定位 | logs events |
| GET | `/observability/v1/platform/overview` | control-plane 健康概览：API、Store、Cache、插件、队列、exporter | metrics |
| GET | `/observability/v1/platform/traces` | control-plane 接口 trace 列表，支持接口、错误和延迟过滤 | traces |
| GET | `/observability/v1/platform/events` | control-plane 内部事件列表 | logs events |
| GET | `/observability/v1/audit/operations` | 操作审计列表，替代旧操作历史查询入口的 OTel 后端版本 | audit logs |
| GET | `/observability/v1/correlate` | 以 `trace_id`、`decision_id`、`rule_id` 或时间窗做跨信号关联 | metrics + logs + traces |

服务详情页推荐查询顺序：

1. 调 `/observability/v1/services/overview` 获取当前时间窗摘要和默认聚合粒度。
2. 调 `/observability/v1/services/metrics` 并行获取请求、CPU、Mem、Pod Ready 和 runtime 指标。
3. 调 `/observability/v1/services/events` 获取同时间窗关键 event。
4. 用户点开异常点后，调 `/observability/v1/correlate` 获取相关 trace、治理 event、实例事件和配置变更。

`observability-query` provider 必须遵守以下规则：

- Provider 入参使用 Pole 领域模型，不允许把 SQL、PromQL 或后端私有查询语句透传给前端。
- Provider 输出使用统一 DTO，字段名固定为 `timestamp`、`value`、`labels`、`attributes`、`trace_id`、`span_id`、`event_name`、`severity`。
- GreptimeDB provider 是默认实现，内部可用 SQL/PromQL；OpenObserve provider 只作为 quickstart / 可选实现。
- 查询白名单由接口语义决定，只允许查询本 ADR 定义的 metric name、EventName 和 trace attributes。
- 高基数字段只能用于详情过滤，不能在服务概览、拓扑、主时间序列接口中作为 group by 维度。
- 所有查询都要经过 Console 当前认证和资源权限检查：用户没有服务/命名空间/治理规则权限时，不返回对应观测数据。

关联查询契约：

| 入口字段 | 允许跳转 |
|---|---|
| `trace_id` | trace 详情、同 trace event、请求时间窗资源指标 |
| `span_id` | span 详情、span 时间窗 CPU/Mem、治理决策 |
| `pole.governance.decision.id` | 治理 event、trace/span、规则详情 |
| `pole.rule.id` | 规则详情、规则发布审计、规则命中趋势和命中 event |
| `pole.service.instance.id` | 实例详情、实例事件、实例时间窗资源指标 |
| `pole.config.group` + `pole.config.file` + `pole.revision` | 配置版本、配置发布审计、配置感知 event |

接口安全边界：

- 默认最大时间窗建议为 24 小时，超过后要求更粗 `step` 或异步导出任务。
- trace/event 详情接口允许高基数字段过滤，但必须强制 `limit`，并默认按时间倒序。
- 响应不返回原始 SQL、后端 token、后端 endpoint、Collector endpoint。
- 跨命名空间查询只允许具备全局或多命名空间权限的用户使用。
- audit 查询必须保留审计语义，不能和普通业务 event 混为一个默认列表。

## Kubernetes 部署模式

整体按 Kubernetes 模式交付，目标是快速体验和后续生产演进都走同一条路径。

推荐组件：

- OpenTelemetry Collector Contrib：使用官方 Helm Chart 部署。
- GreptimeDB：作为长期默认观测后端部署为 standalone/cluster/Operator/Helm 形态，提供 SQL/PromQL 查询。
- OpenObserve：作为 quickstart / 可选后端部署，主要用于快速体验和内置 UI 辅助排查。
- pole-control-plane server：通过现有部署方式接入 Collector endpoint，上报自身内部 signals。
- pole-control-plane console 模块：配置 observability-query provider endpoint，提供 `/observability/v1` 查询接口。

本地集群复用 `tidemind` 已有 GreptimeDB，Pole 通过本 namespace 的稳定服务适配层隔离物理拓扑：

```text
pole-system
  pole-control-plane
  pole-otel-collector
  pole-greptimedb (ExternalName adapter)
    -> maas-greptimedb-frontend.tidemind.svc.cluster.local
    -> database: pole_observability
  pole-mysql -> ExternalName host.docker.internal # 仅本地开发
```

`pole-greptimedb` 是 Pole 拥有的依赖接口，不是数据库工作负载。它用 ExternalName 指向 `tidemind/maas-greptimedb-frontend`，使 Collector 和 Console 查询模块继续依赖稳定的 `http://pole-greptimedb:4000`，不把跨 namespace DNS 扩散到应用配置。物理 GreptimeDB 集群生命周期由 `tidemind` 维护；Pole 只拥有 `pole_observability` 逻辑库、Collector pipeline 和查询 provider。MySQL 仍可作为部署外部依赖；本地 OrbStack 通过 `pole-mysql` ExternalName Service 访问宿主机 `3306`，凭证只进入 Kubernetes Secret。

共享物理集群不等于共享数据模型。Collector 的 traces、metrics、logs exporter 都显式携带 `x-greptime-db-name: pole_observability`，Console provider 也查询同一数据库；MaaS 的 `maas_logs`、`maas_metrics` 与 Pole 表保持逻辑隔离。若生产环境需要独立容量、故障域或合规边界，仍可把 `pole-greptimedb` 适配服务切换到专用 GreptimeDB，而无需修改数据面调用方。

Collector 部署形态：

| 模式 | 用途 |
|---|---|
| Deployment | control-plane、sidecar、SDK 通过 OTLP endpoint 上报，适合作为默认快速体验模式 |
| DaemonSet | 需要节点级采集或就近接入时使用；本方案不采业务普通日志，因此不是首选 |
| StatefulSet | 通常不用于 Collector，GreptimeDB/OpenObserve 这类存储后端才需要稳定存储身份 |

快速体验路径：

1. 准备 GreptimeDB：本地集群复用 `tidemind/maas-greptimedb-frontend` 并创建 `pole_observability`；独立环境则安装专用 GreptimeDB。
2. 安装 OpenTelemetry Collector Contrib，配置 OTLP receiver 与 GreptimeDB exporter/OTLP endpoint。
3. 部署或重启 pole-control-plane，配置 OTLP endpoint 指向 Collector。
4. 部署带 sidecar 或 Rust SDK 的样例业务服务，上报 metrics、traces、event logs。
5. 在 Console 中打开服务观测、平台观测、事件和审计视图。

quickstart 也可以把第 1、2 步替换为 OpenObserve，用 OpenObserve 内置 UI 辅助验证 ingestion 与查询；但 ADR 的长期默认后端仍是 GreptimeDB。

仓库已经提供 `deploy/kubernetes/` 本地 Kubernetes 栈：

- `pole-control-plane` LoadBalancer Service 暴露 Console、HTTP、gRPC、Nacos、Apollo、Eureka 和 xDS 端口。
- `pole-greptimedb` ExternalName Service 适配 `tidemind/maas-greptimedb-frontend`，对 Pole 暴露稳定的 `4000`—`4003` 依赖接口。
- `pole-otel-collector` Deployment 通过 ClusterIP 接收 OTLP，并经 `pole-greptimedb` 写入独立的 `pole_observability` 逻辑库。
- `pole-mysql` ExternalName Service 指向 `host.docker.internal`；Pole Pod 通过 `${MYSQL_HOST}` 使用宿主机 `3306`。
- Pole 镜像包含 all-mode 二进制、Console `dist` 和运行配置；启动入口把 Pod IP、Secret、Collector/GreptimeDB Service DNS 与本地连接池限制渲染进有效配置。
- 从旧 standalone 迁移时，部署脚本只在共享数据库建库、Collector 与 Control Plane rollout 成功后删除旧 StatefulSet；`data-pole-greptimedb-0` PVC 保留用于历史数据导出或回退。

`deploy/observability/docker-compose.yaml` 继续保留为不具备 Kubernetes 时的兼容 quickstart：

- `greptime/greptimedb:v1.1.3` standalone 暴露 HTTP/OTLP HTTP `4000`、MySQL `4002`、PostgreSQL `4003`；本机 gRPC 默认映射到 `14001`，容器内仍是 `4001`。
- `otel/opentelemetry-collector-contrib:0.156.0` 暴露 OTLP/gRPC `4317`、OTLP/HTTP `4318`、health `13133` 和 Collector metrics `8888`。
- Collector 配置接收 traces、metrics、logs 三类 OTLP signals，通过 GreptimeDB `/v1/otlp` 写入；logs 使用 `x-greptime-log-table-name: pole_events`，只用于结构化 event/audit。
- 该 compose 不再是本地默认路径，也不会由 Kubernetes 部署脚本自动删除，避免误删已有 GreptimeDB volume。

### Console Gateway 域名

本地集群复用现有 `tidemind/tidemind-gateway`，不再为 Pole 单独安装 Ingress Controller。由于 Gateway 的 HTTP listener 只接受同 namespace Route，`pole-console` HTTPRoute 放在 `tidemind`，并由 `pole-system/allow-tidemind-pole-console` ReferenceGrant 按来源 namespace 和目标 Service 收敛跨 namespace 授权。

- 本地域名：`pole.localhost`，依赖标准 `.localhost` 回环解析，无需修改 hosts。
- `/agent` 与普通 Console 共用域名、Cookie 和认证边界。
- GreptimeDB、Collector 和 MySQL 不加入 HTTPRoute，保持内部服务边界。

生产演进路径：

- Collector processor 增加资源属性标准化：cluster、namespace、service、instance、sdk、sidecar、rule。
- Collector processor 增加过滤规则：丢弃业务普通日志，只保留结构化 event/audit。
- Collector exporter 支持多后端：GreptimeDB、OpenObserve、Prometheus/Mimir、Tempo/Jaeger、Kafka 等。
- Console 只依赖 `observability-query` 接口，不直接绑定后端实现。

## 首轮对接切片

首轮目标不是一次性做完整观测平台，而是打通一条可以真实验证的数据闭环：

```text
pole-control-plane / pole-client-rust / pole-sidecar
  -> OTLP metrics + traces + structured event logs
  -> OpenTelemetry Collector Contrib
  -> GreptimeDB
  -> pole-control-plane console 模块 /observability/v1
  -> Console 系统监控、服务监控、事件、操作审计页面
```

首轮必须同时覆盖系统侧和业务侧，否则 Console 无法验证跨信号联动：

| 工作面 | 首轮范围 | 不在首轮范围 |
|---|---|---|
| 数据契约 | 固化 metrics 名称、EventName、Resource attributes、低/高基数字段边界 | 告警规则 DSL、任意 SQL 查询 |
| Collector/K8s | 提供 `pole-system` namespace、Collector、GreptimeDB 适配服务或独立部署 values；注册 `pole-otel-collector` 服务 | 多后端 fan-out、生产级多租户容量治理 |
| control-plane 上报 | `statis/history/discoverEvent` 增加 `otel` entry；HTTP/gRPC/Console API 补基础 trace；内部 metrics 使用 `pole.control_plane.*` 新命名 | 替换旧 `/metrics/v1` 或删除 logger/prometheus entry |
| Rust SDK 上报 | 接入真实 OTel exporter；从环境变量、本地配置、服务发现和配置中心合并观测配置；治理 metrics/event/trace 写入同一决策 ID | 替代业务框架完整 instrumentation |
| sidecar 上报 | 在代理请求、路由、限流、熔断、探测、鉴权、Mock、镜像执行点上报请求指标、治理指标、event 和 span attributes | 采集业务普通日志、采集 Kubernetes CPU/Mem |
| Console 查询 | 在 console 模块新增 `/observability/v1` provider 和最小 DTO；服务概览、服务详情、系统监控、事件、审计先接真实数据 | 前端直连 GreptimeDB/OpenObserve |

首轮实现顺序：

1. **契约先行**：在 console 模块定义 `observability-query` DTO、metric/event 白名单和 Resource attributes 校验规则；Rust SDK 与 sidecar 按同一份名称和标签上报。
2. **Collector 先跑通**：用 Kubernetes quickstart 接通 Collector 与 GreptimeDB，暴露 `pole-system/pole-otel-collector`，并把该服务注册进 Pole 服务发现。
3. **control-plane 自观测先接入**：新增 `otel` chain entry，优先让 `pole.audit.operation`、`pole.discovery.instance.*`、`pole.control_plane.request.*` 进入 GreptimeDB；这能在没有业务样例前验证 event、audit、metrics 三类信号。
4. **业务侧最小样例**：Rust SDK 和 sidecar 都先接一条样例调用链，至少覆盖 `pole.service.request.count`、`pole.service.request.duration`、`pole.service.governance.*`、`pole.service.governance.rule.matched` 和 trace/span 关联。
5. **Console 去 mock**：服务监控首页优先接 `/observability/v1/services/overview`，服务详情按 tab 懒加载 `/metrics`、`/events`、`/traces`；系统监控接 `/platform/overview`，事件与操作审计接 `/platform/events` 和 `/audit/operations`。
6. **联动验证**：用同一时间窗验证服务请求指标、治理 event、trace、CPU/Mem 指标能按 `pole.namespace + pole.service.name + service.instance.id/k8s.pod.uid` 关联。

### 首轮接口交付顺序

`/observability/v1` 不应一次铺满所有接口，推荐按页面可验证路径拆分：

| 顺序 | 接口 | 首轮字段 |
|---|---|---|
| 1 | `GET /observability/v1/platform/overview` | control-plane API QPS、错误率、P95/P99、Store/Cache 延迟、Exporter 状态 |
| 2 | `GET /observability/v1/platform/events` | `pole.discovery.*`、`pole.config.*`、`pole.governance.rule.changed` |
| 3 | `GET /observability/v1/audit/operations` | `pole.audit.operation`，兼容旧操作审计筛选字段 |
| 4 | `GET /observability/v1/services/overview` | 服务级请求量、错误率、P95/P99、CPU/Mem、实例数、治理能力摘要 |
| 5 | `GET /observability/v1/services/metrics` | 请求、延迟、CPU、Mem、Ready、治理命中时间序列 |
| 6 | `GET /observability/v1/services/events` | 业务发现、配置、治理、调用失败 event |
| 7 | `GET /observability/v1/services/traces` / `GET /observability/v1/traces/:trace_id` | trace 列表与详情 |
| 8 | `GET /observability/v1/correlate` | 按 `trace_id`、`decision_id`、`rule_id`、实例或时间窗返回关联信号 |

### 业务侧 SDK 与 sidecar 分工

SDK 和 sidecar 都可以上报业务侧数据，但职责不能重叠到互相覆盖：

| 数据 | pole-client-rust | pole-sidecar |
|---|---|---|
| Resource attributes | 注入业务进程、SDK、Pole 服务实例维度 | 注入代理进程、Pod、sidecar、被代理服务维度 |
| 服务请求 metrics | 业务使用 SDK client/hook 时上报调用方视角 | 代理经过 sidecar 的流量时上报入口/出口视角 |
| 治理 metrics | SDK 本地执行路由、限流、熔断、鉴权、Mock、镜像时上报 | sidecar 执行代理层治理时上报 |
| 结构化 event | 服务发现更新、配置感知、治理命中/拒绝、SDK 连接状态 | 代理路由失败、治理拒绝、探测变化、sidecar 连接状态 |
| trace | 注入 Pole 治理上下文到业务 span，保持 W3C TraceContext | 继续传播 trace context，并给代理 span 写入治理属性 |
| 普通业务日志 | 不采集 | 不采集 |
| CPU/Mem | 不采集 Kubernetes 资源指标 | 不采集业务容器 CPU/Mem；由 Collector kubeletstats/K8s receiver 负责 |

如果 SDK 与 sidecar 同时存在：

- 请求 metrics 需要用 `pole.node.role=sdk|sidecar` 区分来源，Console 默认按服务维度聚合时避免重复计数。
- 治理执行只由实际做决策的一侧上报 `pole.service.governance.*` metrics；另一侧只透传 trace attributes。
- 同一次治理决策如果跨 SDK 和 sidecar 协作，必须共享 `pole.governance.decision.id`，并在 event 和 span 上保留该字段。

### 当前实现差距

基于当前仓库状态，首轮对接需要补齐：

- control-plane server：已新增 `statis` 的 `otel` entry，输出 API、Store、内部组件、缓存、服务发现、配置中心和客户端发现调用等 `pole.*` metrics；`history/discoverEvent` 的 OTel event/audit entry 仍待补齐。
- control-plane：已有 OTLP metric exporter、基础 Resource attributes 和 `statis` 新 `pole.*` 指标名；logs/event exporter、trace instrumentation 和更完整的启动/插件/队列指标仍需继续补齐。
- Console 模块：已新增 `/observability/v1/platform/overview` router/handler/provider 和 `bootstrap.console.observabilityQuery` 配置；平台概览已接入 GreptimeDB 查询 `pole_control_plane_request_*` 与 `process_runtime_go_*` 指标表，服务监控接口、事件/审计接口和权限过滤仍需继续实现，不能复用旧 `/metrics/v1` 扩大职责。
- Console 系统监控页：请求/接口指标、CPU/MEM 资源指标、Go runtime 指标已经拆成独立看板。当前本地 GreptimeDB 有 Go runtime 表，因此 `pole-control-plane` Go 看板展示真实 goroutine、heap、GC 指标；本地尚无 kubeletstats 资源表，因此组件资源看板在真实模式下显示空态。
- control-plane Trace：HTTP 与 gRPC 服务端入口使用全局 OTel TracerProvider 创建 server span；Console 反向代理使用 CLIENT span 传播 W3C TraceContext；HTTP handler、限流、鉴权、业务服务、缓存和存储使用 INTERNAL span 表达稳定边界。鉴权按动作在单次请求内去重，避免资源列表逐项校验刷出重复 Span；请求入口授权使用 `auth.console.authorize.<Method>`，列表响应为回填 `editable/deleteable` 进行的操作能力探测使用 `auth.console.capability.<Method>`，后者不代表执行了对应写操作。静态元数据接口不伪造不存在的业务、缓存或存储调用。Collector 通过 `greptime_trace_v1` 写入带 7 天 TTL 的 `pole_traces`；Console 已提供 `/platform/traces` 与 `/traces/:trace_id`，系统监控从聚合接口行按接口和时间窗下钻 Trace 列表及 Span 瀑布。详情接口同时返回兼容毫秒时间戳和用于瀑布定位的微秒时间戳；Console 按父子关系深度优先排列 Span，并提供连接线、折叠、全局时间概览、同行时间轴和选中 Span 检查器。聚合 metrics 不携带 `trace_id`，不能伪装成单次调用。
- pole-client-rust：已有观测语义模型、Resource attributes、低基数 metric 过滤和 no-op recorder；还需要真实 OTel recorder/exporter、服务发现 endpoint 刷新、配置中心 remote config 拉取和热更新。
- pole-sidecar：当前没有可用 OTel 上报模块；需要先在代理请求生命周期和治理执行链路定义统一打点位置。

## 取舍

采用 OpenTelemetry Collector Contrib 的原因：

- 官方 OTel 组件，天然承载 receiver、processor、exporter pipeline。
- contrib 发行版组件更完整，适合后续接入 GreptimeDB、OpenObserve、Prometheus、Jaeger/Tempo、Kafka 或云厂商后端。
- Kubernetes 官方部署路径成熟，适合快速体验。

采用 GreptimeDB 作为长期默认后端的原因：

- GreptimeDB 定位为 observability database，而不是完整观测平台；更适合 Pole Console 自己做领域化查询和分析。
- 支持 metrics、logs、traces 的统一 OTel 后端模型，适合存储 Pole 的 metrics、结构化 event、audit 和 traces。
- SQL + PromQL 查询能力适合 `observability-query` 做跨信号关联，例如请求指标、治理 event、trace、CPU/Mem、runtime metrics 的同时间窗分析。
- 与 Pole 的产品边界更匹配：Pole Console 是主要分析界面，后端只需要提供稳定存储和查询能力。
- 后续可以将 GreptimeDB 作为长期默认 provider，同时保留其它 provider 替换能力。

保留 OpenObserve 作为 quickstart / 可选 provider 的原因：

- 同时覆盖 logs、metrics、traces 存储查询。
- 支持 OTLP 接入，能承接结构化 event logs 和 audit logs。
- 自带 UI、看板和告警能力，适合本地快速体验、调试 ingestion、辅助排障。
- 但 OpenObserve 的平台能力与 Pole Console 有部分重叠，因此不作为长期默认后端。

不采用 Prometheus 作为统一 Collector 的原因：

- Prometheus 适合作为 metrics 后端，不是完整 OTel logs/metrics/traces pipeline。
- 本方案还需要 event logs、audit logs 和 trace。

不把业务普通日志纳入本方案的原因：

- 普通日志会引入日志采集 agent、全文索引、日志成本治理和脱敏治理，显著扩大平台边界。
- 当前核心目标是事件、指标、链路和审计，不是通用日志平台。

## 后续实施分期

### Phase 1：Kubernetes 快速体验

- [x] 提供 `pole-system` namespace 下 Pole + Collector，并通过适配服务复用共享 GreptimeDB。
- [x] 支持通过 ExternalName Service 继续使用宿主机 MySQL，并以 Secret 注入凭证。
- [x] 使用真实 OTLP log smoke 验证 Collector 写入 GreptimeDB 后可由 SQL 查回。
- 可选提供 Collector + OpenObserve quickstart values，用于快速体验和调试。
- pole-control-plane server 支持配置 OTLP endpoint；console 模块支持配置 observability-query provider endpoint。
- Console 提供最小观测入口：服务指标、调用 trace、事件列表、审计列表。
- Rust SDK / sidecar 提供样例业务服务上报 event、metrics、trace。

### Phase 2：内部 chain OTel entry

- 为 `history`、`discoverEvent`、`statis` 增加 `otel` entry。
- 平台 HTTP/gRPC/Nacos/xDS/Console API 接入 trace instrumentation。
- 建立统一资源属性规范。
- 增加 Collector 过滤规则，阻止业务普通日志进入。

### Phase 3：Console 领域化分析

- 服务拓扑、调用链详情、治理命中分析。
- 平台内部健康视图：API、Store、Cache、插件、任务。
- 审计与 trace/event 关联查询。
- 告警规则与治理规则、配置版本、服务实例关联。

### Phase 4：后端可替换

- 抽象 `observability-query` provider。
- 默认 provider 为 GreptimeDB。
- OpenObserve provider 作为 quickstart / 可选实现。
- 支持 Prometheus/Mimir metrics、Tempo/Jaeger traces、Loki/OpenSearch event logs 的组合后端。

## Rust SDK 职责边界

`pole-rust-client` 是业务侧观测数据进入 Pole 观测体系的 SDK 入口之一，但它不替代 sidecar、Collector 或业务框架 instrumentation。

职责摘要见 [[adr-pole-rust-client-observability]]，核心边界如下：

- 负责给业务侧 OTel signals 注入 Pole 绑定属性，包括 `pole.namespace`、`pole.service.name`、`pole.service.instance.id`、`pole.runtime.language=rust`。
- 负责上报 SDK 自身的服务发现、配置订阅、治理决策、缓存刷新、错误和重试等 metrics/events。
- 负责在治理决策时把低基数 metrics 与高基数 event/trace 用 `pole.governance.decision.id`、`trace_id/span_id` 关联。
- 负责提供 hooks/layers，让业务 HTTP/gRPC 框架把调用 span 与 Pole 的路由、限流、熔断、配置上下文绑定。
- 不负责采集业务普通日志，不负责 Kubernetes CPU/Mem 采集，不负责替代 Collector、GreptimeDB 或 OpenObserve。

## 验证要求

- 独立 Kubernetes 快速体验必须能通过一组 manifest/Helm values 启动 Collector、GreptimeDB、pole-control-plane 和样例服务；本地组合环境允许复用现有共享 GreptimeDB，但必须使用 Pole 独立逻辑库。
- OpenObserve quickstart 作为可选 values 验证路径。
- Collector 配置必须包含 OTLP receiver、batch processor、resource processor、GreptimeDB exporter/OTLP endpoint。
- 业务普通日志过滤规则必须有测试或配置验证。
- Console 查询必须通过 pole-control-plane 的 console 模块后端适配层，不允许前端直接调用 GreptimeDB 或 OpenObserve。
- `history`、`discoverEvent`、`statis` 的 `otel` entry 必须保留原有 logger/prometheus/local entry 的兼容行为。

## 证据

- `deploy/conf/pole-server.yaml` 当前已有 `history`、`discoverEvent`、`statis` chain 配置。
- `deploy/kubernetes/` 已包含本地镜像构建、部署脚本、Pole Deployment、Collector Deployment、GreptimeDB ExternalName 适配服务、MySQL ExternalName Service、Console HTTPRoute 和跨 namespace ReferenceGrant。
- OrbStack 实测 Pole 与 Collector Pod Ready，共享 `tidemind/maas-greptimedb-frontend` Ready；Console `/`、`/namespace`、`/agent`、`/login` 和入口静态资源返回 200。
- Pod 内通过 `pole-mysql:3306` 读取宿主机 `pole_server`、`pole_observability`；OTLP smoke event 经 Collector 写入共享 GreptimeDB 的 `pole_observability.pole_events` 并由 SQL 查回。
- `plugin/observability/` 当前已有 history、discoverevent、statis 三类插件实现。
- `apis/observability/` 当前已有 event、history、statis 三类接口定义。
- `plugin/observability/statis/otel` 已实现 `statis.entries[].name=otel`，将 `CallMetric`、`DiscoveryMetric`、`ConfigMetrics` 和 `ClientDiscoverMetric` 映射为 `pole.*` OTel metrics。
- `pkg/console/internal/observabilityquery` 已实现 GreptimeDB provider；`/observability/v1/platform/overview` 返回系统监控页可消费的 provider 状态、摘要 stats、时间序列、组件行、资源 metrics 占位和 Go runtime metrics。
- `pkg/console/internal/observabilityquery` 已查询 `process_runtime_go_goroutines`、`process_runtime_go_mem_heap_alloc_bytes`、`process_runtime_go_mem_heap_inuse_bytes`、`process_runtime_go_mem_heap_sys_bytes`、`process_runtime_go_gc_count_total`、`process_runtime_go_gc_pause_ns_*`，并预留 `go_schedule_duration_seconds_*` 调度延迟查询。
- `web/console/src/pages/Metrics/SystemMonitor` 已优先调用 `/observability/v1/platform/overview` 展示真实数据；接口明细表不再展示 CPU/MEM，CPU/MEM 独立进入组件资源看板，Go runtime 独立进入 `pole-control-plane` 服务端看板。
- `apis/pkg/types/metrics/types.go` 当前已有 CallMetric、DiscoveryMetric、ConfigMetrics 和 ClientDiscoverMetric，可映射为 `pole.*` 指标。
- `apis/pkg/types/operation.go` 当前已有 RecordEntry，可映射为 `pole.audit.operation`。
- `apis/pkg/types/service/instance.go` 当前已有 InstanceEvent 与 ServiceEvent，可映射为 discovery event。
- OTel semantic conventions 要求 metrics 命名表达可聚合动作，单位由 instrument unit 表达；event 语义由 LogRecord EventName 和 attributes 承载。
- OTel process runtime resource 语义定义了 `process.runtime.name`、`process.runtime.version`、`process.runtime.description`，并说明可结合 `telemetry.sdk.language` 判断运行时类型。
- OTel JVM metrics 语义定义了 `jvm.memory.*`、`jvm.gc.duration`、`jvm.thread.count`、`jvm.class.*`、`jvm.cpu.*` 等标准指标。
- OTel Go runtime metrics 语义定义了 `go.memory.*`、`go.memory.gc.*`、`go.goroutine.count`、`go.processor.limit`、`go.schedule.duration` 等标准指标。
- OTel telemetry attributes 定义了 `telemetry.sdk.language`、`telemetry.sdk.name`、`telemetry.sdk.version` 等 SDK 属性。
- OTel Kubernetes components 文档说明 Kubernetes Attributes Processor 可把 Kubernetes metadata 加到 traces、metrics、logs 资源属性上，用于关联应用遥测与 Pod 指标。
- Collector Contrib `kubeletstats` receiver 可从 kubelet 拉取 node、pod、container、volume metrics，覆盖业务 CPU/Mem 资源指标来源。
- GreptimeDB 官方文档说明其是 unified observability database，覆盖 metrics、logs、traces，并支持 OTLP、SQL、PromQL。
- GreptimeDB OTLP 文档说明可作为 OpenTelemetry Collector 目标后端，并提供统一 OTLP endpoint 接收多信号。
- GreptimeDB standalone Docker 文档说明本地 standalone 默认可暴露 HTTP `4000`、gRPC `4001`、MySQL `4002`、PostgreSQL `4003`。
- GreptimeDB OTel Collector 文档说明 Collector 可以通过 OTLP receiver `4317/4318` 接收 signals，并通过 `http://greptimedb:4000/v1/otlp` exporter 写入 traces、metrics、logs。
- OpenTelemetry Collector 官方文档定义了 Collector 的接收、处理和导出管道能力。
- OpenTelemetry Collector Docker 文档说明 contrib 镜像可通过自定义 config 挂载启动，并暴露 OTLP `4317/4318`、health `13133`、metrics `8888`。
- GreptimeDB HTTP API 文档说明 `/health` 和 `/v1/sql` 可用于本地健康检查与 SQL smoke 查询。
- OpenTelemetry Collector Helm Chart 支持 Kubernetes 部署。
- OTel exporter 配置规范定义了 `OTEL_EXPORTER_OTLP_ENDPOINT`、按 signal endpoint、protocol、timeout、headers、compression 等配置，并说明 OTLP/HTTP 会基于 base endpoint 拼接 `/v1/traces`、`/v1/metrics`、`/v1/logs`。
- OpenObserve 文档说明其支持 OTLP 接入 logs、metrics、traces，适合作为 quickstart / 可选 provider。
- 当前 Console 已有 `/metrics/v1` 历史监控路由，包括 labels、server interfaces、server events、server operations；新的 OTel 查询接口应独立在 `/observability/v1` 下演进。

## 参考

- [OpenTelemetry Collector](https://opentelemetry.io/docs/collector/)
- [OpenTelemetry Collector configuration](https://opentelemetry.io/docs/collector/configuration/)
- [OpenTelemetry Collector Docker deployment](https://opentelemetry.io/docs/collector/install/docker/)
- [OpenTelemetry Collector distributions](https://opentelemetry.io/docs/collector/distributions/)
- [OpenTelemetry Collector Helm Chart](https://opentelemetry.io/docs/platforms/kubernetes/helm/collector/)
- [OpenTelemetry Protocol Exporter](https://opentelemetry.io/docs/specs/otel/protocol/exporter/)
- [OpenTelemetry Metrics semantic conventions](https://opentelemetry.io/docs/specs/semconv/general/metrics/)
- [OpenTelemetry Event semantic conventions](https://opentelemetry.io/docs/specs/semconv/general/events/)
- [OpenTelemetry HTTP metrics semantic conventions](https://opentelemetry.io/docs/specs/semconv/http/http-metrics/)
- [OpenTelemetry RPC metrics semantic conventions](https://opentelemetry.io/docs/specs/semconv/rpc/rpc-metrics/)
- [OpenTelemetry process runtime resource semantic conventions](https://opentelemetry.io/docs/specs/semconv/resource/process/)
- [OpenTelemetry JVM metrics semantic conventions](https://opentelemetry.io/docs/specs/semconv/runtime/jvm-metrics/)
- [OpenTelemetry Go runtime metrics semantic conventions](https://opentelemetry.io/docs/specs/semconv/runtime/go-metrics/)
- [OpenTelemetry telemetry attributes](https://opentelemetry.io/docs/specs/semconv/registry/attributes/telemetry/)
- [OpenTelemetry Kubernetes Collector components](https://opentelemetry.io/docs/platforms/kubernetes/collector/components/)
- [OpenTelemetry Collector Contrib kubeletstats receiver](https://pkg.go.dev/github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver)
- [GreptimeDB documentation](https://docs.greptime.com/)
- [GreptimeDB OpenTelemetry Protocol ingestion](https://docs.greptime.com/user-guide/ingest-data/for-observability/opentelemetry/)
- [GreptimeDB standalone Docker installation](https://docs.greptime.com/getting-started/installation/greptimedb-standalone/)
- [GreptimeDB OTel Collector integration](https://docs.greptime.com/user-guide/ingest-data/for-observability/otel-collector/)
- [GreptimeDB HTTP API](https://docs.greptime.com/user-guide/protocols/http/)
- [GreptimeDB product overview](https://greptime.com/product/db)
- [OpenObserve ingestion](https://openobserve.ai/docs/ingestion/)
- [OpenObserve OTLP ingestion](https://openobserve.ai/docs/ingestion/logs/otlp/)

## 相关页面

- [[architecture]]
- [[common-infra]]
- [[configuration]]
- [[api-servers]]
- [[adr-pole-rust-client-observability]]
- [[adr-local-pebble-protobuf-value-cache]]
