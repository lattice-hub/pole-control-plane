---
type: API Reference
title: API 服务端 — 协议实现
tags: [api, http, grpc, xds, nacos]
links: [architecture, ai-features, auth-system, adr-service-contract-reporting-and-visualization, pole-product-capability-matrix-research]
updated: 2026-07-27
legacy_source_count: 4
sources:
  - resource: ../../../apis/apiserver/apiserver.go
    title: 当前模块接口
---

# API 服务端 — 协议实现

## 概览

所有 API 服务端均实现 `apis/apiserver/Apiserver` 接口。整体分层设计见 [[architecture]]，AI MCP 集成详情见 [[ai-features]]，认证中间件见 [[auth-system]]。

```go
type Apiserver interface {
    GetProtocol() string
    GetPort() int
    Initialize(ctx context.Context, option map[string]interface{}, apiConf APIConfig) error
    Run(errCh chan error)
    Stop()
    Restart(option map[string]interface{}, apiConf APIConfig) error
}
```

每个服务端通过 `init()` 自行注册，并在配置中通过名称选择使用哪个实现。

---

## HTTP 服务端（`plugin/apiserver/httpserver/`）

基于 `go-restful/v3` 的主 REST API。负责处理：
- 服务发现 REST API
- 配置中心 REST API
- 治理规则管理
- 认证管理（用户、策略）
- 管理后台操作
- AI MCP 服务器管理
- OpenAPI/Swagger 文档

**关键子包：**
```
httpserver/
├── server.go           # 主服务端，路由注册
├── discover/           # 服务发现端点
├── config/             # 配置中心端点
├── auth/               # 认证端点（用户、用户组、策略）
├── aimcp/              # AI MCP 服务器管理端点
│   ├── server.go
│   └── mcp_server.go
├── utils/              # 共享 HTTP 工具函数
├── i18n/               # 国际化
└── docs/               # Swagger 规范生成
```

**MCP 端点**（`plugin/apiserver/httpserver/aimcp/`）：
- `GET /mcp/v1/servers` — 列出 MCP 服务器（支持过滤）
- `POST /mcp/v1/servers` — 创建 MCP 服务器
- 通过 MCP 协议暴露工具，供 AI 智能体消费

---

配置中心管理仅走九个 POST /config/v1 动作，原生客户端仅 /v1.ConfigGRPC/Discover；Apollo/Nacos/Consul 是可选只读转换入口。旧文件/分组/模板和 HTTP Watch/Resolve 已移除，详见[当前配置 ADR](../adr/adr-config-template-values-unification.md)。

## gRPC 服务端（`plugin/apiserver/grpcserver/`）

面向服务 SDK 客户端的高性能二进制协议。

**子包：**
```
grpcserver/
├── discover/
│   └── v1/         # 服务发现 gRPC 处理器
└── utils/          # gRPC 工具函数（元数据、错误映射）
```

实现了来自 `pole-io/specification` protobuf 定义的 `polaris.Discover` 和 `polaris.Heartbeat` gRPC 服务。

服务契约同时提供 gRPC `ReportServiceContract` 和 HTTP client API
`POST /v1/ReportServiceContract` 上报入口，并通过统一 Discover 的 `SERVICE_CONTRACTS`
类型下发；`/naming/v1` 仅用于 Console 管理查询。协议与载荷约定见
[[adr-service-contract-reporting-and-visualization]]。

---

## xDS 服务端 v3（`plugin/apiserver/xdsserverv3/`）

实现 Envoy xDS API v3，用于 Envoy 代理集成：
- LDS（监听器发现服务）
- CDS（集群发现服务）
- EDS（端点发现服务）
- RDS（路由发现服务）
- ADS（聚合发现服务）

以 `envoyproxy/go-control-plane` 为基础框架。

**子包：**
```
xdsserverv3/
├── cache/     # xDS 快照缓存，将 Pole 资源映射为 xDS proto
└── resource/  # xDS 资源构建器（监听器、集群、端点、路由）
```

转换关系：Pole 服务/实例/路由规则 → Envoy xDS 资源

治理策略下发遵循统一治理 release 语义：

- Envoy Node 使用 `pole.io/governance-label.<key>=<value>` metadata 显式声明治理灰度标签；TLS、端口、服务身份等 xDS 控制 metadata 不会被隐式当作业务标签。
- LDS 按 node 隔离；EDS 仍按 namespace 共享；受 caller 或灰度标签影响的 RDS、VHDS、CDS 按 node 生成稳定内容版本，避免同 namespace 节点互相污染。
- 路由按 `CustomRoute.caller → CustomRoute.callee` 选择，`DestinationGroup` 只表达 callee 实例子集、标签和权重，不再用于反推 caller/callee。
- 当前 Envoy 转换支持路由、基础 QPS 限流、实例级熔断和故障探测。路由匹配支持 HTTP path、method、header、query、AND 与随机比例；限流只转换固定 HTTP 匹配与基础 token bucket，排队、自定义响应、并发/系统资源、爬坡、均摊和自定义 failover/action 等尚未等价实现的字段会整条跳过。
- OR、动态请求参数、通配匹配和其它无法等价表达的条件会被明确跳过并记录日志，不会删除条件后扩大规则命中范围。
- 泳道、无损、调用鉴权、流量镜像和流量 Mock 尚未建立 Envoy 等价执行模型，不属于当前 xDS 能力声明；管理端可保存和发布不等于 Envoy 已执行。

---

## Nacos 服务端（`plugin/apiserver/nacosserver/`）

为 Nacos 客户端提供完整的 Nacos 协议兼容。

**子包：**
```
nacosserver/
├── core/        # Nacos 核心逻辑
├── model/       # Nacos 数据模型
├── logger/      # Nacos 专属日志
├── v1/          # Nacos v1 API
│   ├── config/  # 配置中心（v1）
│   ├── discover/# 服务发现（v1）
│   └── http/    # HTTP 处理器
└── v2/          # Nacos v2 API（基于 gRPC）
    ├── config/
    ├── discover/
    ├── pb/      # Nacos v2 protobuf
    └── remote/  # gRPC 连接管理
```

将 Nacos 格式的请求转换为 Pole 内部格式。

---

## Apollo 服务端（`plugin/apiserver/apolloserver/`）

为 Apollo 配置中心客户端提供协议兼容。将 Apollo SDK 指向该服务端后，客户端即可从 Pole 的配置中心获取配置。

---

## Eureka 服务端（`plugin/apiserver/eurekaserver/`）

Netflix Eureka 协议兼容。使用 Eureka 服务发现的 Spring Cloud 应用程序，可通过 Pole 进行注册和服务发现。

---

## AI MCP 集成（`plugin/apiserver/httpserver/aimcp/`）

将 Pole 的服务注册中心能力暴露为 MCP 工具，供 AI 智能体（Claude、GPT 等）使用。详细说明见 [[ai-features]]。

**已注册的 MCP 工具：**
- `list_mcp_servers` — 查询已注册的 MCP 服务器
- `create_mcp_servers` — 注册新的 MCP 服务器

使用 `mark3labs/mcp-go` 库处理 MCP 协议。

## 相关页面

- [[architecture]]
- [[ai-features]]
- [[auth-system]]
- [[adr-service-contract-reporting-and-visualization]]
- [[pole-product-capability-matrix-research]]
