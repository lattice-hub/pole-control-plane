---
type: Module
title: 缓存层
tags: [cache, performance]
links: [storage, architecture, ai-features, adr-local-pebble-protobuf-value-cache, adr-service-contract-reporting-and-visualization]
updated: 2026-10-03
legacy_source_count: 2
sources:
  - resource: ../../../apis/cache/types.go
    title: 当前模块接口
---

# 缓存层

## 设计理念

缓存层是一个内存读透缓存，位于业务逻辑层和数据库之间。它使服务发现和规则评估能够以低延迟方式运行，无需在每次请求时都访问 MySQL。存储层接口见 [[storage]]，整体架构见 [[architecture]]，AI 缓存子包见 [[ai-features]]。

**位置：** `pkg/cache/`（实现），`apis/cache/`（接口）

统一配置直接从主库读取一致聚合及观察游标；旧配置文件、分组和 values 缓存已移除，见[配置 ADR](../adr/adr-config-template-values-unification.md)。

## 缓存管理器

`apis/cache/types.go` 中的 `CacheManager` 由若干缓存子接口组合而成：

| 缓存名称 | 常量 | 用途 |
|---------|------|------|
| ServiceCache | `ServiceName` | 服务及别名 |
| InstanceCache | `InstanceName` | 服务实例 |
| RoutingConfigCache | `RoutingConfigName` | 路由规则 |
| RateLimitCache | `RateLimitConfigName` | 限流规则 |
| CircuitBreakerCache | `CircuitBreakerName` | 熔断规则 |
| FaultDetectorCache | `FaultDetectRuleName` | 故障探测规则 |
| NamespaceCache | `NamespaceName` | 命名空间 |
| ClientCache | `ClientName` | 已注册的客户端 |
| UserCache | `UserName` | 认证用户 |
| AuthStrategyCache | `StrategyName` | 认证策略/策略 |
| RoleCache | `RoleName` | 认证角色 |
| GrayCache | `GrayName` | 治理规则灰度发布 |
| MCPServerCache | `MCPServerName` | MCP 服务器 |
| LaneRuleCache | `LaneRuleName` | 泳道规则 |

## 实现（`pkg/cache/cache.go`）

```go
type CacheManager struct {
    storage  store.Store
    caches   []Cache          // 所有缓存实例的有序列表
    needLoad *SyncSet         // 需要初始化的缓存集合
}
```

**刷新循环：**
```go
func (c *CacheManager) Run() {
    // 每隔 1 秒执行一次
    ticker := time.NewTicker(time.Second)
    for range ticker.C {
        for _, cache := range c.caches {
            cache.Update()  // 增量数据库查询
        }
    }
}
```

**每个缓存的增量更新模式：**
```go
func (c *mcpServerCache) Update() error {
    // 查询数据库中自（lastMtime - 5 秒）以来被修改的记录
    servers, err := c.storage.GetMoreMCPServers(c.lastMtime.Add(-5*time.Second), c.firstUpdate)
    // 应用到内存 map
    for _, s := range servers {
        if s.Flag == 1 { // 软删除
            delete(c.servers, s.ID)
        } else {
            c.servers[s.ID] = s
        }
    }
    // 更新 lastMtime
}
```

**-5 秒**时间窗口用于防范应用服务器与数据库之间的时钟偏差。

## AI 缓存子包

```
pkg/cache/ai/
├── default.go         # NewAICaches 工厂
└── mcp_server.go      # MCPServer 缓存
```

详细的 AI 功能说明见 [[ai-features]]。

## 其他重要缓存

```
pkg/cache/service/    # ServiceCache + InstanceCache（数据量最大、最复杂）
pkg/cache/config/     # ConfigFileCache、ConfigGroupCache、GrayConfigCache
pkg/cache/auth/       # UserCache、StrategyCache、RoleCache
pkg/cache/rules/      # RoutingConfig、RateLimit、CircuitBreaker、FaultDetect、Lane
pkg/cache/namespace/  # NamespaceCache
pkg/cache/client/     # ClientCache
pkg/cache/gray/       # 灰度发布元数据
```

## 本地 value cache 方向

内存缓存继续负责热点索引、revision、mtime 和必要热状态；对于实例列表、服务契约和治理规则发布快照这类面向客户端返回、且可由事实来源重建的大 value，后续采用 [[adr-local-pebble-protobuf-value-cache]] 中的 Pebble 本地 protobuf bytes 缓存。

这个方向的核心边界：

- 默认仍使用当前内存模式；只有打开 `localValueCache.enabled` 并选择 Pebble backend 后，才启用“内存索引 + Pebble value”。
- Pebble 保存 protobuf marshal 后的 bytes，不取代 MySQL、注册内存态或治理规则 cache 的事实来源地位。
- key 必须包含 revision、release_id 或过滤条件 hash，避免不同调用方可见性的服务实例列表误命中。
- 实例心跳继续留在内存热路径，不按心跳频率同步写磁盘。
- 启用 Pebble 后必须通过性能基准，客户端发现、配置发现、服务契约和治理下发的稳态性能不能低于当前路径。
- OTel 操作审计和服务事件的本地可靠队列也使用 Pebble，但通过 queue 抽象和独立 key prefix / DB 路径隔离，不和读多写少的 value cache 混用。

## OpenResourceCache

业务组件可以按需开启所需的缓存：

```go
// 在服务发现初始化中
cacheMgr.OpenResourceCache(
    cacheapi.ResourceWithCacheName(cacheapi.ServiceName),
    cacheapi.ResourceWithCacheName(cacheapi.InstanceName),
    cacheapi.ResourceWithCacheName(cacheapi.RoutingConfigName),
    // ...
)
```

这样可以避免在未使用某些功能时加载不必要的数据。

## 相关页面

- [[storage]]
- [[architecture]]
- [[ai-features]]
- [[adr-local-pebble-protobuf-value-cache]]
- [[adr-service-contract-reporting-and-visualization]]
