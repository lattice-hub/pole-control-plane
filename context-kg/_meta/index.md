---
title: pole-control-plane 知识库
tags: [meta, index]
links: [schema, log]
updated: 2026-10-10
sources: 0
---

# pole-control-plane 知识库

本目录是 `context-kg/` wiki 的入口。所有页面均使用 [[schema]] 中定义的格式规范。操作历史见 [[log]]。

知识库按三大知识域组织：业务知识域、技术知识域、质量保障知识域。

---

## 任务过程记录（tasks/）

正式统一v1已恢复六种客户端标签匹配，合同与真实验证边界见 [[adr-config-template-values-unification]]、[[config-six-match]] 和 [[todo]]。

- [[todo]] — 当前任务计划、进度、review、部署、布局验收和验证记录；含统一配置 v1 契约、管理 HTTP/客户端 Discover 请求响应边界与核心首批实现验证、配置管理 HTML 交互原型、模板正文优先与按需变量/设置、现有 Fluent 设计对齐及浏览器验证、创建服务抽屉修整、真实操作页、治理P1/三项P2修复、OrbStack revision13验收与临时资源清理、配置中心独立交互原型、发布/客户端查看闭环、发布Step与原式标签条件行、灰度标签在类型下内联、原型同步六匹配、正式页面/共享正文与环境取值落地及OrbStack验收 | tasks, todo
- [[lessons]] — 用户纠正后的命名、交付、抽屉与配置交互；含独立选组预览、后缀格式、编辑区视口填充与发布/消费路径完整性规则 | tasks, lessons

## 业务知识域（business/）

- [[terminology]] — 共享配置模板、变量声明、环境取值与完整发布，以及协议兼容、服务治理术语 | business, terminology
- [[domain-models]] — 环境配置版本、治理 Bundle 与核心关系 | business, domain-model
- [[business-rules]] — 跨域、访问控制、服务发现、配置和治理规则 | business, rules

### 竞品情报（business/competitive-intel/）

- [[kmesh-product-research]] — Kmesh 数据平面定位、架构、能力与边界 | business, competitive-intel, kmesh, service-mesh
- [[pole-product-comparison-research]] — Pole 与五类云原生产品的分层比较 | business, competitive-intel, pole, nacos, apollo, polarismesh, kmesh, istio
- [[pole-product-capability-matrix-research]] — 注册配置与服务治理 Mesh 双矩阵、状态图例和证据边界 | business, competitive-intel, pole, nacos, apollo, consul, polarismesh, istio, kmesh, service-mesh, config

### 功能档案（business/feature-registry/）

- [[namespace]] — 运行环境、全局晋升 DAG 与系统环境保护 | business, feature, namespace
- [[service-discovery]] — 服务发现、逻辑/环境服务生命周期、双选择器删除、健康检查与四协议服务契约 | business, feature, service, healthcheck
- [[config-center]] — 配置中心：六种客户端标签条件、完整定义发布、多灰度、Discover与本地渲染；新增条件组复用/逐key可选取值目标及原型边界 | business, feature, config
- [[governance-rules]] — 治理规则：路由、限流、熔断、故障探测、泳道、无损规则 | business, feature, governance
- [[admin]] — 管理后台：AdminOperateServer 接口与跨域运维操作 | business, feature, admin
- [[skill-marketplace]] — 多源 Skill 发布、Git 批量导入与分发 | business, feature, ai, skill, marketplace

## 技术知识域（technical/）

### 架构（technical/arch/）

- [[overview]] — 项目介绍、技术选型、顶层目录结构与启动流程 | overview
- [[architecture]] — 四层架构设计、插件系统、Store 接口、CacheManager、拦截器链与请求流程 | architecture, design

### 接口契约（technical/apis/）

- [配置协议兼容核查基线](../technical/apis/config-protocol-compatibility-baseline.md) - Apollo、Nacos、Consul 官方协议与支持边界 | api, config, compatibility

- [[api-servers]] — HTTP、gRPC、xDS、Nacos、Apollo、Eureka 六类协议服务端，含 xDS 节点级治理快照与能力边界 | api, http, grpc, xds, nacos

### 环境部署（technical/Environment/）

- [[configuration]] — YAML、部署目录与 .pole_data 运行时数据约定 | config, yaml, deploy
- [[kubernetes-production-sop]] — Helm、ACK、GHCR、备份恢复与生产验收操作手册 | kubernetes, helm, ack, operations, backup

### 模块说明（technical/modules/）

- [[storage]] — 存储接口层次结构、MySQL 实现特性（软删除、增量查询、分布式锁）与 Mock Store | storage, mysql, database
- [[cache-layer]] — 缓存子类型、增量刷新循环、AI 缓存子包与按需开启机制 | cache, performance
- [[auth-system]] — 认证授权插件、Token 与 Console Session 边界 | auth, security
- [[common-infra]] — 日志、EventHub、Batch Controller、OTel 指标、同步原语与通用工具 | infra, logging, eventhub
- [[ai-features]] — MCP、A2A、Pole Agent 与 Skill Marketplace | ai, mcp, a2a, skill, marketplace

### 技术约定（technical/conventions/）

- [[patterns]] — 代码库中的关键模式与约定（插件注册、单例、拦截器、缓存、治理规则编辑抽屉交互等） | patterns, conventions

### 架构决策（technical/adr/）

- [统一模板与 values 及多协议兼容](../technical/adr/adr-config-template-values-unification.md) - 完整环境发布与本地渲染；含共享模板/环境取值/选择环境发布设计修正及正式管理协议待补边界 | adr, config, template, compatibility, apollo, nacos, consul

- [[adr-kubernetes-production-deployment]] — 通用 Helm、ACK 适配、原生 Secret、GHCR 与 OSS 备份基线 | adr, kubernetes, helm, ack, ghcr, backup
- [[adr-governance-rule-unified-storage]] — 治理规则按环境统一存储、发布与授权 | adr, governance, storage, cache, namespace
- [[adr-service-contract-reporting-and-visualization]] — 四协议契约与 Dubbo 原生元数据适配 | adr, service-contract, openapi, grpc, dubbo, thrift
- [[adr-rpc-first-governance-scope]] — RPC-first 治理范围与消息、存储、任务集成边界 | adr, governance, rpc, scope, integration
- [[adr-governance-request-parameter-capture]] — 治理请求参数采集与动态限流、路由消费边界 | adr, governance, routing, ratelimit, sdk
- [[adr-quota-lease-streaming-rate-limit]] — 请求数/Token/并发场景配置、可调窗口、请求前预留、流中累计消费、结算归还与 Console 运行态 | adr, governance, ratelimit, limiter, sdk, streaming, llm
- [[adr-managed-service-identity-authentication]] — 托管身份、短期凭证与 Header 兼容模式 | adr, governance, auth, identity
- [[adr-a2a-agent-registry]] — A2A Agent Registry 功能设计与分期 | adr, ai, a2a, mcp, registry
- [[adr-console-oidc-identity-source]] — Console OIDC 用户来源与目录同步 | adr, auth, console, oidc, identity
- [[adr-config-gray-release-spec-contract]] — 配置中心多灰度发布与 specification 契约适配 | adr, config, api, gray-release
- [[adr-config-template-client-rendering]] — 模板与 Value 原子环境版本及客户端渲染 | adr, config, template, namespace, sdk, gray-release
- [[adr-config-template-labels-sensitive-values]] — 模板目录标签与逐参数 Value 加密存储 | adr, config, template, labels, encryption, security
- [[adr-instance-active-healthcheck]] — Console 实例 TCP/HTTP 主动健康检查 | adr, service, healthcheck, tcp, http
- [[adr-console-fluent-ui-design-system]] — Console Fluent UI v9 设计系统与复合查询交互规范 | adr, console, frontend, fluent-ui, design-system
- [[adr-console-agent-resource-workbench]] — Pole Agent 真实 LLM/MCP 运行时与资源确认待发布 | adr, ai, agent, console, governance, config
- [[adr-system-configuration-control-plane]] — 64 项字段级配置策略、协议默认环境热切换、desired/effective 状态与实例回执 | adr, config, runtime, console, operations
- [[adr-pole-self-management-control-loop]] — Pole 自身 MCP/A2A 注册、Prompt 自动应用与系统身份协调 | adr, ai, mcp, a2a, agent, prompt, automation
- [[adr-local-pebble-protobuf-value-cache]] — Pebble 本地缓存与 .pole_data 布局 | adr, cache, pebble, protobuf, performance
- [[adr-unified-process-mode-and-limiter-integration]] — 单一制品、多进程模式与 Limiter 独立部署边界 | adr, runtime, limiter, deploy, process
- [[adr-console-limiter-source-layout-and-embedded-web]] — Console/Limiter 源码归属与前端嵌入制品 | adr, architecture, console, limiter, frontend, embed, build
- [[adr-logical-service-environment-binding]] — 控制面逻辑服务与环境服务显式关联 | adr, service, namespace, domain-model, console
- [[adr-system-namespace-kind]] — 业务环境与 Pole 内部系统空间类型化 | adr, namespace, environment, system, console
- [[adr-environment-promotion-topology]] — 全局晋升 DAG、lane 回归与跨资源 Bundle | adr, namespace, environment, promotion, lane, release, console
- [[adr-ai-resource-environment-binding]] — Agent/MCP 逻辑定义、环境实例与调用作用域 | adr, ai, agent, mcp, namespace, domain-model
- [[adr-plugin-extension-registry]] — 官方插件装配、隔离与销毁契约 | adr, plugin, registry, extension, runtime
- [[adr-skill-marketplace-federation]] — 多源 Skill、Bundle 与 CLI 边界 | adr, ai, skill, marketplace, registry, supply-chain, cli

#### 可观测性（technical/adr/observability/）

- [[adr-otel-observability-platform]] — OTel 指标、Trace 下钻、可靠队列与 K8s 部署 | adr, observability, otel, kubernetes
- [[adr-pole-rust-client-observability]] — Rust SDK 观测上报与动态配置 | adr, observability, otel, rust-sdk

## 质量保障知识域（quality/）

### 自动化背景知识（quality/automation/）

- [[testing]] — 测试目录结构、Mock Store/Auth、集成测试套件与测试规范 | quality, automation, testing, mock
- [[console-ui-quality-gates]] — Console 共享组件与发布验收矩阵 | quality, automation, console, frontend, ux

### 缺陷档案（quality/defects/）

- [[console-editable-row-focus-loss]] — 动态编辑行不稳定 key 导致输入失焦的复发与门禁 | quality, defect, console, frontend, react, input

### 测试用例集（quality/testcases/）

- [[console-client-auth-e2e-testcases]] — Console API、Client 与权限接口 E2E 覆盖矩阵 | quality, testcases, e2e, api, console, client, auth
- [[console-orbstack-browser-acceptance]] — OrbStack Console 真实浏览器操作验收矩阵 | quality, testcases, e2e, console, playwright, orbstack

---

## 如何快速定位

| 我想了解… | 读哪里 |
|-----------|--------|
| 这个项目是什么，能做什么 | [[overview]] |
| 代码整体结构和层次 | [[architecture]] |
| 领域术语和核心实体 | [[terminology]]、[[domain-models]] |
| 命名空间如何管理运行环境 | [[namespace]] |
| 服务发现与健康检查 | [[service-discovery]] |
| 同一业务服务如何跨环境关联 | [[adr-logical-service-environment-binding]] |
| 配置文件发布与 Watch | [[config-center]] |
| 配置模板与 Namespace Value 如何渲染 | [[adr-config-template-client-rendering]] |
| 路由/限流/熔断等治理规则 | [[governance-rules]] |
| 治理规则统一存储方案 | [[adr-governance-rule-unified-storage]] |
| RPC-first 治理范围与外部资源边界 | [[adr-rpc-first-governance-scope]] |
| 服务间托管身份与凭证 | [[adr-managed-service-identity-authentication]] |
| 管理后台运维操作 | [[admin]] |
| MCP 如何运作 | [[ai-features]] |
| Skill Marketplace 如何发布与分发 | [[skill-marketplace]]、[[adr-skill-marketplace-federation]] |
| Pole 如何自动注册和管理自身能力 | [[adr-pole-self-management-control-loop]] |
| System Configuration 如何发布 | [[adr-system-configuration-control-plane]] |
| Control Plane、Console 与 Limiter 如何组合运行 | [[adr-unified-process-mode-and-limiter-integration]] |
| OTel 与共享 GreptimeDB 如何部署 | [[adr-otel-observability-platform]] |
| 数据库表/查询是怎样的 | [[storage]] |
| 内存缓存如何刷新 | [[cache-layer]] |
| 认证 Token 如何校验 | [[auth-system]] |
| HTTP/gRPC 端点在哪里 | [[api-servers]] |
| 公共工具包（日志、EventHub） | [[common-infra]] |
| YAML 配置项含义 | [[configuration]] |
| Kubernetes 生产部署与运维 | [[kubernetes-production-sop]]、[[adr-kubernetes-production-deployment]] |
| 代码约定和惯用模式 | [[patterns]] |
| 如何写/跑测试 | [[testing]] |
| Console 表格和页面如何验收 | [[console-ui-quality-gates]] |
| wiki 本身的维护规范 | [[schema]] |

## 相关页面

- [[schema]]
- [[log]]

- [配置六匹配验收](../quality/testcases/config-six-match.md) - HTTP/gRPC、MySQL、真实登录页面 Preview 六匹配及缺项/空值边界。

- 2026-11-06：正式配置真实登录浏览器验收、Helm revision18与测试资源清理完成，见 [[console-orbstack-browser-acceptance]]、[[config-six-match]]、[[adr-config-template-values-unification]]。

- 2026-11-06：两个仓库业务代码develop同步与固定远端协议依赖，范围/验证见 [[todo]]。
