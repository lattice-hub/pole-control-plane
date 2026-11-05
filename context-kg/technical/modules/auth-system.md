---
title: 访问控制与认证系统
tags: [auth, security]
links: [business-rules, adr-console-oidc-identity-source, adr-managed-service-identity-authentication, architecture, index, patterns, adr-pole-self-management-control-loop]
updated: 2026-08-20
sources: 9
---

# 访问控制与认证系统

## 架构

认证系统包含两个维度：
1. **身份认证（Authentication）** — 验证身份（Token、用户名/密码）
2. **授权（Authorization）** — 执行资源级别的权限控制

两者均以插件方式实现，并采用拦截器链模式。整体架构见 [[architecture]]，各业务域入口见 [[index]]，拦截器模式详解见 [[patterns]]。

## 插件接口（`apis/access_control/auth/`）

```go
// 认证系统配置
type Config struct {
    User     ConfigEntry   // 例如 "defaultUser"
    Strategy ConfigEntry   // 例如 "defaultStrategy"
    Interceptors []string  // 拦截器链名称
}

// UserServer — 管理用户和用户组
type UserServer interface {
    CreateUser(ctx, req) *apimodel.Response
    UpdateUser(ctx, req) *apimodel.Response
    DeleteUsers(ctx, reqs) *apimodel.BatchWriteResponse
    GetUser(ctx, filter) *apimodel.Response
    GetUsers(ctx, query) *apimodel.BatchQueryResponse
    
    CreateGroup, UpdateGroup, DeleteGroup, GetGroup, GetGroups
    
    Login(ctx, req) *apimodel.Response
    GetUserToken, UpdateUserToken
    CheckCredential(ctx, req) *authtypes.AuthResponse
}

// StrategyServer — 管理访问策略
type StrategyServer interface {
    CreateStrategy, UpdateStrategy, DeleteStrategy
    GetStrategy, GetStrategies
    
    CreateRole, UpdateRole, DeleteRole, GetRoles
    
    // 核心授权检查
    CheckResourcesAuth(ctx, req) *apimodel.BatchWriteResponse
    CheckConsolePermission(ctx, req) *authtypes.AuthResponse
}
```

## 插件实现（`plugin/access_control/auth/`）

### 用户管理（`auth/user/`）
- 默认实现：`defaultUser`
- 将用户/用户组存储在 MySQL 中（`AuthStore`）
- 使用缓存加速查询（`UserCache`、`RoleCache`）
- 使用 `golang.org/x/crypto` 进行密码哈希

### 策略管理（`auth/policy/`）
- 默认实现：`defaultStrategy`
- 资源级访问控制（类似 AWS IAM）
- 策略将用户/用户组与资源及允许的操作绑定
- 使用 `StrategyCache` 加速策略评估

### 内置系统角色

Pole 的角色目录包含普通自定义角色，以及三个不可变的内置系统角色：

| 稳定 ID | 名称 | 权限边界 |
|---------|------|----------|
| `pole-system-role-admin` | `admin` | 控制面管理、认证授权及全部业务资源 |
| `pole-system-role-resource-reader` | `resource-reader` | 全部业务资源读取，不含认证、系统配置和运维管理 |
| `pole-system-role-resource-writer` | `resource-writer` | 全部业务资源读写，不含认证、系统配置和运维管理 |

`ensureSystemRoles()` 在策略服务初始化和角色查询时幂等补齐内置角色及固定策略，兼容已有数据库。角色 REST API 对自定义角色提供完整 CRUD；当目标是内置角色时，删除被拒绝，更新只允许 `users`、`user_groups`，并在服务端保留固定名称、描述、来源、类型、元数据和策略。系统策略禁止通过普通策略 API 修改或删除，普通策略创建、更新及直接资源授权也不得把内置角色作为授权主体。

登录时，主账号仍签发 `main` 会话角色；直接或通过用户组加入内置 `admin` 的子账号签发 `admin` 会话角色，因此可进入 System Configuration 等 admin-only Console 页面。资源读写角色仍签发普通 `sub` 会话角色，具体资源权限由策略检查链决定。

### 限流（`access_control/ratelimit/token/`）
- 令牌桶算法，用于 API 级别的速率限制
- 防止管理/运维 API 被滥用

### IP 白名单（`access_control/whitelist/ip/`）
- 将访问限制在特定 IP 范围内
- 可按部署环境配置

## 拦截器链模式

每个业务领域都有认证拦截器来包装真实的服务端：

```
pkg/service/interceptor/auth/server.go
pkg/config/interceptor/auth/server.go
pkg/goverrule/interceptor/auth/server.go
pkg/namespace/interceptor/auth/server.go
pkg/admin/interceptor/auth/server.go
```

模式示例（以服务认证拦截器为例）：
```go
type Server struct {
    nextSvr     service.DiscoverServer  // 包装真实服务端
    userSvr     auth.UserServer
    strategySvr auth.StrategyServer
}

func (s *Server) CreateService(ctx context.Context, req *apiservice.Service) *apimodel.Response {
    // 1. 从 ctx 中提取认证上下文（来自 HTTP 请求头的 Token/用户信息）
    // 2. 通过 strategySvr.CheckConsolePermission() 检查权限
    // 3. 授权通过后，委托给 s.nextSvr.CreateService()
}
```

## 认证 Token 流程

1. 客户端在请求中携带 `X-Polaris-Token` 或 `Authorization` 请求头
2. HTTP 服务端中间件将 Token 提取到 `context.Context` 中
3. 认证拦截器调用 `UserServer.CheckCredential()` 验证 Token
4. `StrategyServer.CheckConsolePermission()` 评估资源策略
5. 授权通过后，请求继续流向业务逻辑层

## Console Session 与资源授权边界

Console 使用签名的 HttpOnly `jwt` Cookie 保存用户 ID、Pole Token 与会话角色。该 Cookie 证明浏览器 Session，
但资源请求是否允许仍由 control-plane 的策略链独立判断，因此 HTTP 401 本身不能作为清理 Session 的充分条件。

- `401001`（`Code_NotAllowedAccess`）表示当前身份没有目标资源或函数权限；Console 必须保留 Cookie 与本地登录态，
  由菜单、路由或页面呈现无权反馈。
- `401003`（`Code_TokenDisabled`）与 `401004`（`Code_TokenNotExisted`）表示认证凭据已经不可继续使用；Console Proxy
  可以下发过期 Cookie，前端可以清理登录态并进入登录页。
- `/auth/v1/user/session` 直接返回 HTTP 401 时，说明签名 Cookie 缺失、过期或无效，可以单独判定 Console Session 失效。
- 旧 Console 代理返回的 `407` 暂时作为兼容认证失效码保留，但不能将普通 API 的任意 HTTP 401 或 `401001` 纳入该分类。

`ErrorTokenInvalid` 与 `ErrorTokenNotExist` 都映射为 `401004`；资源策略拒绝继续映射为 `401001`。代理在读取 401
响应业务码后恢复响应体，只对前两类明确认证错误清除 Cookie，避免授权拒绝破坏仍然有效的浏览器 Session。

## Console 外部用户来源

Console 可以在登录入口层扩展企业 OIDC 用户来源，但该能力不进入 pole-server 核心鉴权链。OIDC 登录只用于确认外部用户身份，并将其映射或同步为 `source=oidc` 的 Pole User；后续 Console 请求仍使用 Pole token，资源权限仍由本页描述的 User/UserGroup/Role/Policy 与拦截器链判断。完整技术方案见 [[adr-console-oidc-identity-source]]。

## 服务调用身份

治理域的服务调用鉴权与本页的管理面用户认证/资源授权是两条链。默认调用鉴权由 control-plane 托管每个服务的内部身份和短期 workload 凭证，由调用方/被调方 SDK 完成凭证携带与本地验证；自定义 Header value 只作为兼容模式。完整边界见 [[adr-managed-service-identity-authentication]]。

## 参数检查拦截器

与认证拦截器相邻，参数检查拦截器负责验证请求参数：
```
pkg/service/interceptor/paramcheck/
pkg/config/interceptor/paramcheck/
```

这些拦截器在请求到达业务逻辑之前，对必填字段、长度限制、格式约束等进行校验。

## 自管理系统身份

`pole-self-manager` 是进程内确定性协调器的固定执行身份，不是可登录用户或管理员角色，也不签发可交给 Agent 的 Token。它只能收敛 Pole 自身 MCP/A2A Registry 投影，并在管理员保存 desired state 后记录 Agent 配置的自动执行主体；管理员仍是唯一配置主体。完整边界见 [[adr-pole-self-management-control-loop]]。

## 相关页面

- [[business-rules]]
- [[adr-console-oidc-identity-source]]
- [[adr-managed-service-identity-authentication]]
- [[architecture]]
- [[index]]
- [[patterns]]
- [[adr-pole-self-management-control-loop]]
