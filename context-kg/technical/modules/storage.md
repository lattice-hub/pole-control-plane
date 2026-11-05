---
type: Module
title: 存储层
tags: [storage, mysql, database]
links: [architecture, cache-layer, index, adr-service-contract-reporting-and-visualization]
updated: 2026-10-03
legacy_source_count: 2
sources:
  - resource: ../../../apis/store/api.go
    title: 当前模块接口
---

# 存储层

存储层完全以接口方式定义，实现细节不会向上泄露。整体架构见 [[architecture]]，缓存层如何使用存储接口见 [[cache-layer]]，各业务域入口见 [[index]]。

## 接口层次结构

```
Store (apis/store/api.go)
├── DiscoverStore
│   ├── ServiceStore           — 服务增删改查
│   ├── InstanceStore          — 实例增删改查 + 批量操作
│   ├── RateLimitStore         — 限流规则
│   ├── CircuitBreakerStore    — 熔断规则
│   ├── RouterRuleConfigStore  — 路由规则
│   ├── FaultDetectRuleStore   — 故障探测规则
│   ├── LaneStore              — 泳道规则
│   └── ServiceContractStore   — 服务契约
├── UnifiedConfigInventory      — 环境持久配置数量和删除保护
├── UnifiedConfigStore          — 配置一致快照、事务更新与持久游标（独立能力接口）
├── AdminStore                 — 分布式锁、维护操作
├── AuthStore
│   ├── UserStore
│   ├── UserGroupStore
│   ├── StrategyStore          — 认证策略
│   └── RoleStore
└── AIStore
    └── MCPServerStore
```

## MySQL 实现（`plugin/store/mysql/`）

生产环境的存储后端。关键特性如下：

**软删除**：普通实体使用 `flag` 字段（0 = 活跃，1 = 已删除）。查询时通过 `flag = 0` 过滤。

**增量查询**：普通缓存实体有 `mtime`（修改时间）列。[[cache-layer]] 使用 `WHERE mtime > ?` 查询，获取自上次轮询以来的变更数据。

```go
// 所有 Store 用于缓存同步的通用模式
GetMoreServices(mtime time.Time, firstUpdate bool) ([]*svctypes.Service, error)
GetMoreInstances(ctx context.Context, mtime time.Time, firstUpdate bool, ...) (map[string]*svctypes.Instance, error)
GetMoreMCPServers(mtime time.Time, firstUpdate bool) ([]*ai.MCPServer, error)
// 等等
```

**分布式锁**（通过 `AdminStore`）：
- `StartLeaderElection(key string)` — 获取领导权
- `IsLeader() bool` — 检查是否为主节点
- 由 bootstrap 使用，用于串行化多服务器启动过程

**事务支持**：
- `Transaction` 接口封装了 `Begin()`、`Commit()`、`Rollback()`
- 批量操作使用事务保证原子性

**注册模式**（`plugin/store/mysql/init.go`）：
```go
func init() {
    s := &mysqlStore{}
    storeapi.RegisterStore("defaultStore", s)
}
```

统一配置的聚合头、发布正文、事件和时钟由专用表管理，不使用旧 config_file/config_template 表或 mtime 增量缓存；具体事务与 namespace→clock→state 锁序见[配置 ADR](../adr/adr-config-template-values-unification.md)。

## Mock Store（`plugin/store/mock/`）

用于单元测试和集成测试的 Mock 实现。使用 `golang/mock` 生成代码。`test/suit/` 中的测试使用该 Mock Store。

## Store 注册表（`apis/store/store.go`）

```go
var storeSlots = &StoreSlots{}

func RegisterStore(name string, s Store) { ... }
func GetStore() Store { ... }  // 返回活跃的 Store（通过 SetStoreConfig 设置）
func SetStoreConfig(c *Config) { ... }
```

配置通过名称选择使用哪个 Store：
```yaml
store:
  name: defaultStore  # 或 "mock"（用于测试）
  option:
    master:
      dbType: mysql
      dbName: pole_server
      dbUser: root
      dbPassword: ...
      dbAddr: 127.0.0.1:3306
```

## 相关页面

- [[architecture]]
- [[cache-layer]]
- [[index]]
- [[adr-service-contract-reporting-and-visualization]]
