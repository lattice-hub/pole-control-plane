---
title: 治理规则（`pkg/goverrule/`）
tags: [business, feature, governance, routing, ratelimit]
links: [namespace, architecture, storage, cache-layer, service-discovery, adr-governance-rule-unified-storage, adr-governance-request-parameter-capture, adr-quota-lease-streaming-rate-limit, adr-rpc-first-governance-scope, patterns, pole-product-capability-matrix-research]
updated: 2026-08-02
sources: 10
---

# 治理规则（`pkg/goverrule/`）

治理规则归属于明确的命名空间，并遵循相同的模式：增删改查 + 版本控制 + 灰度发布。相同类型和名称的规则表示同一逻辑规则在不同环境中的独立实例；规则归属环境与 caller、callee、target service 等运行时作用范围分别表达。现有 specification、统一存储和 Console 已为九类服务治理聚合根提供 owner namespace，但匹配、目标索引和数据面协议仍主要面向服务调用。整体架构见 [[architecture]]，命名空间依赖见 [[namespace]]，存储接口见 [[storage]]，缓存层见 [[cache-layer]]，与服务发现的关联见 [[service-discovery]]。

## 规则类型

| 规则 | 用途 |
|------|------|
| **路由规则** | 基于标签/元数据的流量路由 |
| **限流规则** | 针对服务或实例的速率限制 |
| **熔断规则** | 自动故障隔离 |
| **故障探测规则** | 主动健康探测 |
| **泳道规则** | 多泳道（环境隔离）路由 |
| **无损规则** | 优雅关闭/启动 |

所有规则支持：
- 版本控制（版本号追踪）
- 灰度发布（向部分用户发布）
- 细粒度权限（控制谁可以编辑哪些规则）

## 技术决策

治理规则底层存储与缓存更新的完整方案见 [[adr-governance-rule-unified-storage]]。
请求参数值来源以及动态限流、动态路由的消费边界见 [[adr-governance-request-parameter-capture]]。
请求前预留、流中累计消费、结束归还，以及请求数、Token 数、并发的统一限流生命周期见 [[adr-quota-lease-streaming-rate-limit]]；RPM 通过请求数的 60 秒窗口表达。

目标业务结论：

- 所有治理规则统一按 `rule_type` 区分类型。
- 每条规则必须归属命名空间；规则环境实例以 `namespace + rule_type + name` 定位。
- 泳道以 LaneGroup 为聚合根，LaneRule 是 group JSON 内的子对象。
- 普通规则和泳道规则都支持版本控制、灰度发布和细粒度权限。

## RPC-first 范围

Pole 核心治理聚焦 HTTP、gRPC、Dubbo 等同步服务调用。现有路由、限流、熔断、故障探测、无损、泳道、调用鉴权、流量镜像和流量 Mock 继续围绕服务调用做深，并补充数据面能力协商、原子 Bundle、应用回执和 RPC A/B Test。

Kafka、RocketMQ、Redis、MySQL 暂不进入运行时灰度、路由、镜像、Mock 或请求级治理。它们可以提供非侵入式资源目录、健康、指标和经过单独评审的原生管理集成，但不得把 Topic、DataSource 或 Shard 伪装成 Service/Instance，也不得展示没有真实执行点的治理入口。

任务调度只有在 Pole 拥有统一 Worker/Agent、租约和执行协议后再独立评估，不复用 RPC RouteRule。完整范围决策见 [[adr-rpc-first-governance-scope]]。

## Console 编辑交互

- 路由、泳道、限流、熔断、故障探测、无损、调用鉴权、流量镜像和流量 Mock 均以分段表单完成查看与编辑。
- 新建规则先展示类型卡片；点击某一类型即直接进入对应的独立创建页面，不使用二次“进入创建”确认或创建抽屉。
- 编辑器不展示动态实时 Spec、YAML 或 JSON 预览；保存前由字段校验和业务错误提示反馈问题。
- 鉴权规则的服务信息按“命名空间、服务名称”的上下依赖顺序选择，并使用可折叠分段；收起时显示已选择的服务摘要。
- 鉴权规则的认证方式同样使用可折叠分段；收起时显示当前认证模式。选择“自定义 Header（兼容模式）”后，不设置规则级共享 Header 凭证，而是在每个鉴权子规则内、与受保护接口一起配置独立的请求 Header 匹配条件。
- API 资源仅配置协议、方法、路径匹配类型和路径值；值类型属于请求参数匹配条件，不作为 API 资源字段展示或提交。编辑语义随协议变化：HTTP 使用 `method` 与 URI `path`；gRPC 使用 `path` 承载 service/interface、`method` 承载可选方法名；Dubbo 使用 `path` 承载 interface、`method` 承载可选方法名。RPC 接口输入应比可选方法输入更宽。切换协议会清空旧协议的值，避免误用 HTTP 默认值。
- 请求参数匹配条件只区分固定值和请求参数；“请求参数”采集当前键的实际值，Proxyless SDK 可用于按值拆分本地限流器或匹配同名路由标签，xDS 当前不消费该动态语义。运行机器环境变量不属于治理规则值来源。
- 分段表单按内容自然收缩；独立创建页的内容区承接长规则滚动，页头操作固定且不保留空白浮动操作按钮，避免编辑器在内容较少时仍撑满整页。具体布局约定见 [[patterns]]。

## Envoy xDS v3 执行边界

- Envoy 以显式 node metadata 治理标签参与 normal/gray release 选择，策略快照按 node 隔离；同 namespace 不同标签节点可以得到不同 RDS、VHDS 和 CDS。
- 路由执行以顶层 caller/callee 为服务范围，目标组继续承载实例标签和权重。xDS 不从目标组反推调用方或被调方。
- xDS 当前只声明可等价翻译的路由、基础 QPS 限流、实例级熔断和故障探测能力。OR 条件、动态请求参数，以及排队、自定义响应、并发/系统资源限流等未被转换的字段会整条跳过；尚无 Envoy 执行模型的泳道、无损、鉴权、镜像和 Mock 同样不做静默降级。
- `random_percent` 映射为 Envoy RouteMatch 的运行时比例；目标组 weight 映射为 WeightedCluster。当前仍未提供基于用户键的稳定 HashPolicy，因此不能把随机比例或权重路由宣传为稳定 A/B 分桶。

## `Server` 结构体关键字段

```go
type Server struct {
    config    Config
    storage   store.Store
    namespaceSvr    namespace.NamespaceOperateServer
    caches    CacheManager
    cmdb      plugin.CMDB              // 校验资源标签
    subCtxs   []*eventhub.Subscription
    emptyPushProtectSvs sync.Map
}
```

## 相关页面

- [[namespace]]
- [[architecture]]
- [[storage]]
- [[cache-layer]]
- [[service-discovery]]
- [[adr-governance-rule-unified-storage]]
- [[adr-governance-request-parameter-capture]]
- [[adr-quota-lease-streaming-rate-limit]]
- [[adr-rpc-first-governance-scope]]
- [[patterns]]
- [[pole-product-capability-matrix-research]]
