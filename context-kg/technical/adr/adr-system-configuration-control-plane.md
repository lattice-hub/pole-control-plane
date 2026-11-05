---
title: ADR：统一系统配置与动态生效
tags: [adr, config, runtime, console, operations]
links: [configuration, architecture, config-center, adr-console-agent-resource-workbench, auth-system, storage, adr-pole-self-management-control-loop]
updated: 2026-08-10
sources: 47
---

# ADR：统一系统配置与动态生效

## 状态

Accepted，分阶段实施中。Phase 1 已落地 Pole Server/Console 的 64 项显式配置目录；Phase 2 已落地全领域字段级编辑策略、通用草稿/发布和 Agent 受控热更新。Phase 3 首个 Server `GuardedHotReload` 切片已覆盖 Nacos/Eureka 默认环境映射；其它重启级字段仍保存 desired revision 并回执 `pending_restart`。

## 背景

Pole Server 与 Console 当前都以静态 YAML 为主要配置来源。静态文件适合自举、部署和安全基线，但日志级别、维护任务、观测查询、AgentDefinition、模型和 MCP 等运行参数需要页面化管理、版本、发布、回滚和多实例收敛。

不能把整份 `pole-server.yaml` 或 Console YAML 原样放进页面并反射式热更新：监听器、Store、插件拓扑和密钥根等配置在系统到达配置中心之前就必须生效；当前多数模块也在初始化时复制配置或通过 `sync.Once` 构造，没有运行时 apply 能力。

现有 `server_setting` 只有 MySQL DDL，没有 Store、接口、权限、版本、缓存或 Watch 链，不能作为已存在的动态配置能力。Pole 配置中心已经提供草稿、发布、历史、回滚和 Watch，应作为系统配置发布层复用。

## 决策

建立独立的 **System Configuration** 模块：

- 编译默认值和静态 YAML 继续构成可独立启动的安全基线。
- System Configuration facade 隔离存储实现：Console 首期使用自身 Pole MySQL 中的专用系统配置表，Pole Server 后续可接入配置中心保留空间；页面和运行时不感知具体 adapter。
- Console 提供统一的类型化“系统设置”页面，不暴露原始系统 YAML。
- Pole Server 与 Console 分别实现自己的 `ConfigApplier`，不能由 Console 直接修改进程内对象或核心数据库。
- Pole 默认通过独立 `SystemSecretStore` 托管业务密钥；配置正文、页面、diff 和审计只出现 `SecretReference`。Kubernetes Secret、环境变量或 Vault 只保存解密根密钥，或作为可选外部 Provider。
- 没有显式 `Validate + Apply` 的字段不得标记为热更新。

## 领域模型

| 概念 | 含义 |
|---|---|
| `SettingDefinition` | 一个配置项的强类型定义：key、component、schema、默认值、校验、敏感级别、apply mode 和兼容版本 |
| `ConfigurationDocument` | 同一组件或子域的一组强类型设置，不是一整份万能 YAML |
| `ConfigurationDraft` | 尚未影响实例的编辑态内容 |
| `ConfigurationRelease` | 不可变发布快照，包含 revision、schema version、hash、操作者和说明 |
| `DesiredSnapshot` | 控制面希望目标实例采用的已发布 revision |
| `EffectiveSnapshot` | 某个实例当前实际使用的最终配置、字段来源和 revision |
| `ApplyReceipt` | 实例对 revision 的 `applied/rejected/pending_restart` 回执 |
| `SystemSecret` | Pole 托管的版本化密文、用途、所有者、轮换状态与审计元数据，接口永不返回明文 |
| `SecretReference` | 默认指向 `pole-secret://` 的不可变 Secret 版本；也可指向显式启用的外部 Provider，不是密钥正文 |

需要跨文档原子切换时再增加 `ConfigurationBundle`：子文档先各自发布不可变版本，最后发布只引用 release ID 的 manifest。首期不引入跨文档 bundle。

## 配置分级

每个 `SettingDefinition` 必须声明 apply mode：

| Apply mode | 语义 | 代表配置 |
|---|---|---|
| `BootstrapOnly` | 只能通过部署配置修改 | mode、Store/DSN、配置源、Secret Provider、认证/加密根 |
| `RestartRequired` | 可页面发布目标值，但实例只回执待重启 | Listener/TLS、API include、插件类型与链、缓存拓扑 |
| `HotReload` | 校验通过后可原子替换 | AgentDefinition、Console 观测查询 timeout、普通 feature flag |
| `GuardedHotReload` | 需要预热、双版本窗口或 drain 后切换 | 鉴权开关、健康检查参数、OTel exporter、workload credential 轮换 |

核心日志实现目前不是并发安全的原子配置，必须先改造再开放；Console 日志级别可作为较早的热更新候选。没有 applier 的现有 YAML 字段默认归入 `BootstrapOnly` 或 `RestartRequired`，不能依据 YAML tag 自动开放。

## 存储布局

Console Agent 首期使用独立系统设置表，不与普通配置文件、观测明细或核心 `server_setting` 混用：

```text
console_system_config_domain
console_system_config_revision
console_system_secret_version
```

三张表分别保存领域活动指针、不可变配置版本和加密 Secret 版本。配置 payload 只保存 `pole-secret://` 引用；密文使用随机 DEK + 根 KEK 信封加密。Console 只通过 `SystemSettingsRepository` 访问，页面、Agent 和普通配置管理页面均不能直接操作表。

Pole Server 后续优先实现配置中心 adapter，使用 `pole-system/system-configuration` 保留空间按组件与爆炸半径拆文档；若采用 sys config DB，也必须复用同一领域接口和发布语义。Console 不直连 Pole Server 核心数据库，现有 `server_setting` 仍不启用。实例 apply receipt 使用独立 TTL Store，不混入配置正文。

## 来源优先级与自举

正常优先级：

```text
编译默认值 < 静态 YAML < 已发布动态 overlay < 紧急覆盖白名单
```

- 动态 overlay 是稀疏 patch；删除字段表示恢复静态值，不是写入零值。
- 紧急环境变量只允许覆盖明确的止损字段，不能让任意 env 隐式压过页面配置。
- `last-known-good` 是远端不可用时的故障回退，不是第四个可编辑来源。
- 页面逐字段显示 effective value、来源、desired/effective revision 和 apply mode。
- 首次部署不自动把 YAML 写入配置中心；页面提供“从当前有效值创建首个草稿”，由管理员显式发布。

Pole Server 启动顺序：

1. 加载编译默认值和静态 YAML。
2. 初始化 bootstrap island：Store、Secret Provider、基础认证和配置源。
3. 通过选定的 `SystemSettingsRepository` 读取 active system releases，或使用 last-known-good。
4. 构建并校验 `EffectiveSnapshot`。
5. 用该快照初始化其余组件和 API Server。
6. 启动配置发布 Watch 与周期性 reconcile。

Console 使用静态 `poleServer.address`、数据库连接和 Secret 根密钥完成最小自举；自己的 active documents 由 Console System Settings repository 读取，Pole Server 快照仍通过网络 facade 聚合。这保证 `all` 与 `console-only` 使用同一 seam。

## 模块 interface

```go
type SystemSettings interface {
    Describe(ctx context.Context, scope Scope) ([]SettingDefinition, error)
    Effective(ctx context.Context, scope Scope) (*EffectiveSnapshot, error)
    SaveDraft(ctx context.Context, req SaveDraftRequest) (*ConfigurationDraft, error)
    ValidateDraft(ctx context.Context, draftID string) (*ValidationReport, error)
    Publish(ctx context.Context, req PublishRequest) (*ConfigurationRelease, error)
    Rollback(ctx context.Context, req RollbackRequest) (*ConfigurationRelease, error)
    ApplyStatus(ctx context.Context, revision string) ([]ApplyReceipt, error)
}

type ConfigApplier interface {
    Validate(ctx context.Context, current, desired *EffectiveSnapshot) error
    Apply(ctx context.Context, desired *EffectiveSnapshot) ApplyReceipt
}
```

页面和测试只跨 `SystemSettings` interface；Pole Server 与 Console 的 applier 是内部 adapter。回滚不会倒退 revision 指针，而是用历史 payload 创建新的单调递增 release。

## 动态发布与多实例一致性

- 发布只原子切换 `DesiredSnapshot`，不宣称所有实例同时生效。
- 每个实例完整执行 parse、schema migration、Secret 解析、依赖探活和 `Validate` 后，才一次性替换不可变快照。
- 失败实例保留 last-known-good 并上报 `rejected`；重启级配置上报 `pending_restart`。
- 实例回执包含 instance ID、build version、desired/effective revision、status、error 和 applied time。
- 页面展示收敛率、失败实例、待重启实例和版本漂移。
- 本地发布事件用于快速通知，周期性 full reconcile 用于防止丢事件；跨节点不把进程内 EventHub 当成广播总线。
- 混合版本部署必须按 SettingDefinition 的兼容范围预检，不能静默忽略未知字段。

## 权限、安全与审计

System Configuration 页面和 `/system-config/v1/*` Console facade 首期只允许主账号会话角色 `main`，或内置 `admin` 系统角色成员签发的 `admin` 会话访问。前端启动时通过 `/auth/v1/user/session` 从 HttpOnly 签名会话恢复权威角色；解析完成前 fail-closed，不展示菜单且不渲染 admin-only 路由。非 admin 直达 `/system-configuration` 会重定向到普通控制台，后端仍从同一签名会话读取角色并执行最终 gate。浏览器 localStorage 不保存也不参与角色授权。后续开放细粒度委派时，再把 admin gate 扩展为下列独立权限，不能直接取消服务端门禁。

新增独立权限：

- `system-config.read`
- `system-config.edit`
- `system-config.validate`
- `system-config.publish`
- `system-config.rollback`
- `system-config.apply-status.read`

权限可按 component/domain 收窄；认证、加密、Agent tool policy 等高风险设置可要求双人审批。审计记录 actor、document、draft/release revision、before/after hash、脱敏字段差异、原因和 Request ID。

业务 Secret 默认由 Pole 自己托管。页面通过独立的 write-only `SystemSecretStore` interface 创建或轮换 Secret；服务端用随机 DEK 加密值，再以部署期 KEK 包装 DEK，只保存密文、wrapped DEK、key ID、版本和审计元数据。Kubernetes Secret/环境变量默认只承载 `POLE_SYSTEM_SECRET_MASTER_KEY` 这类 KEK 自举材料，不承载每个 LLM API key。配置草稿只写入版本化 `pole-secret://console/agent/llm-gateway/{version}` 引用，发布与回滚不会复制密钥正文。

Console 页面只提供“设置新值、保留当前版本、替换、连接测试”操作；读取接口永远只返回 configured、last rotated、version 和引用。当前 Agent Runtime 要求发布版本包含可用模型凭证，因此页面不提供“禁用 API Key”这一会制造不可发布草稿的伪操作；未来只有在领域模型增加独立的 `AgentDisabled` 有效状态后，才可提供停用入口。Agent Runtime 通过受控内部 resolve interface 获取明文并只保留于进程内存，不能让浏览器、配置查询、日志或模型上下文获得它。Vault/Kubernetes External Secrets 可作为企业部署的可选 adapter，但不是默认产品体验。

现有配置文件加密链不能直接充当 `SystemSecretStore`：它把数据密钥与密文放在同一配置 metadata 中，并会对有权限的配置读取者解密正文，缺少 write-only interface、KEK 包装、独立权限、轮换状态和用途约束。JWT/签名密钥只有建立 old/new 双 key 窗口后才能进入受控轮换，否则保持 `BootstrapOnly`。

Pole Agent 不获得任何 System Configuration 写工具，也不能读取 Secret。系统管理员保存 Agent desired revision 后，由隔离的确定性执行身份 `pole-self-manager` 自动完成候选探测与发布；这不是 Agent/LLM 自修改 Prompt、模型和工具权限。详细执行边界见 [[adr-pole-self-management-control-loop]]。

## 页面信息架构

侧栏新增独立“系统设置”，不混入普通“配置管理”：

- 页面先以 Pole Server / Console 组件页签切分，再以领域导航进入单个配置工作区；领域是后续草稿、发布、回滚和生效状态的最小页面单元。
- 单领域表格不重复展示组件和领域列；搜索覆盖当前组件全部领域，领域导航同步展示命中数并自动定位首个命中领域。
- 窄视口下领域导航转为横向滚动入口，选中领域必须自动滚入视野，配置表仅在自身容器内横向滚动。
- **概览**：Server/Console desired 与 effective revision、漂移、失败和待重启。
- **Pole Server**：按 naming、cache、maintain、observability、auth 等子域展示类型化字段。
- **Console**：runtime、observability、Agent 等子域。
- **发布中心**：草稿 diff、校验、影响分析、发布说明、历史和回滚。
- **实例状态**：逐实例 applied/rejected/pending_restart。

每个字段都展示来源和 apply mode。`BootstrapOnly` 字段只读展示，并给出部署修改提示；不能伪装成页面发布后会生效。

### 编辑与保存交互

普通领域保持“导航 + 配置表”的浏览结构；Agent 领域是完整配置任务，使用独立工作区展示运行就绪状态、模型连接、Pole Secret、MCP 能力、Agent 指令和发布流程，不再把 13 项原始定义、搜索、编辑与发布堆在同一工具栏。原始配置定义仍可作为审计元数据，但不承担 Agent 的主操作路径。

每个 `SettingDefinition` 必须显式声明 `editable`、`edit_reason`、描述和 validation；注册但未进入策略表的字段默认锁定。当前字段评审结果：

| 组件 | 总数 | 页面可管理 | 部署锁定 | 说明 |
|---|---:|---:|---:|---|
| Pole Server | 29 | 21 | 8 | bootstrap 装配、Store/DSN、认证插件与 Token 盐锁定 |
| Console | 34 | 22 | 12 | Web 监听/静态目录、主账号、Store、JWT 根密钥和自举上游锁定 |

普通领域编辑只提交当前领域完整的可管理字段集合。服务端拒绝未知字段、跨领域字段和锁定字段，并统一验证 boolean、integer、duration、list、枚举、URL、host:port、范围以及健康检查最小/最大周期关系。保存只创建草稿；发布创建不可变 desired revision。`RestartRequired` 发布后页面同时显示 current effective 和 desired value，并标记 `pending_restart`，直到专用启动 overlay/applier 回执 applied。

1. Agent 默认页先回答“当前运行什么、还缺什么、是否有草稿、下一步是什么”，主动作随状态在“完成首次配置 / 编辑配置 / 审阅并发布”之间切换。
2. 编辑采用 720px 模态任务抽屉，按“模型与凭证 / MCP 工具 / Agent 指令”分组；窄视口回落为单列，反馈留在对应配置上下文内。
3. 普通值使用与类型对应的单字段控件；list 使用可增删 Tag 输入；duration 同时校验格式和范围。
4. Secret 只显示“已配置/未配置、版本、最近轮换”，输入框永远为空。留空代表保持当前版本；显式替换才创建新的 Pole Secret 版本。
5. 连接测试、保存草稿、发布是三个不可合并的阶段。任意普通字段或 Secret 输入改变都会使客户端测试结果失效；发布仍由服务端针对 exact draft revision 强制复验。
6. “保存草稿”只更新当前领域 draft，不改变 effective value，不触发 Agent；默认页显示草稿 revision、创建人/时间和变化项数。
7. “审阅并发布”按字段展示 effective → draft diff，Secret 仅显示版本引用变化；同时说明连接复验、Secret 安全和 last-known-good 原子切换边界。

编辑面板底部固定为“取消 / 保存草稿”，连接测试放在模型连接分组内；发布动作不出现在编辑面板，防止用户把测试、保存与生效混为一次操作。关闭有未保存输入的面板必须二次确认；保存后的草稿关闭面板不丢失。

## 分阶段落地

### Phase 1：配置目录与只读页面

- 建立 SettingDefinition registry，先显式注册 Server/Console 核心安全字段；页面明确展示注册覆盖范围，不把任意插件 `Option` 或整份 YAML 反射成配置目录。
- 标记 owner、scope、apply mode、sensitivity 和当前来源。
- 页面只读展示 effective value 和 source，不改变加载行为。
- 页面必须区分“合法空筛选结果”和“目录请求失败”：首次请求失败显示明确错误与重试，不渲染全零统计；刷新失败保留 last-known-good 快照并提示当前数据可能过期。

当前实现通过 `pkg/systemconfig` 提供 registry、source index 与 effective snapshot interface。Pole Server 在 `/admin/v1/system/configuration` 输出本组件快照，并通过 `MaintainModule + Read + DescribeSystemConfiguration` 执行凭证及策略授权；Console 在 `/system-config/v1/settings` 聚合本地 Console 与远端 Server 快照。目录共 64 项，其中 Pole Server 31 项、Console 33 项；页面已按组件和领域组织单领域配置表，新增字段必须显式注册并声明领域、敏感级别与 apply mode。

Phase 1 的来源限定为 `compiled_default`、`static_file`、`environment` 和 `command_line`。默认值归一化后必须同步回落为 `compiled_default`，复合环境模板记录全部变量引用；列表设置聚合其 YAML 子项来源，避免被误报为编译默认值。Secret 在服务端删除原值后只返回“已配置/未配置”状态和引用。

### Phase 2：Console 低风险热更新

- AgentDefinition 已接入专用 MySQL repository、Pole Secret Store、草稿/连接测试/发布历史、发布前强制探活、原子快照、周期 reconcile 和 last-known-good。
- Agent 提案 TTL 与资源工具上游超时已进入专用编辑器，并通过原子 TTL 与逐请求 context timeout 随发布热更新。
- K8s 日常不再注入 LLM Gateway 地址或 API key，只保留数据库连接和 `POLE_SYSTEM_SECRET_MASTER_KEY`。
- Console 普通领域和 Pole Server 已统一支持类型化草稿、发布与 desired/effective 差异；observability、feature flags、日志等在独立 applier 完成前仍正确回执 `pending_restart`。

### Phase 3：Server 低风险热更新

- 每个字段先实现 applier、并发测试、回滚和失效测试，再开放页面编辑。
- 优先维护任务开关/周期和普通运行阈值；鉴权、健康检查、OTel 单独评审。
- 首个落地切片是 `pole-server/compatibility`：
  - `server.compatibility.nacos.public_namespace_target` 与
    `server.compatibility.eureka.default_namespace_target` 独立发布，静态初值均为 `dev`。
  - Nacos 空 `namespaceId`/`public` 和 Eureka 缺少 `x-namespace` 时按当前 effective snapshot 解析；显式环境原样优先。
  - Pole Server 各实例从共享 active revision 读取、验证目标存在且为 BUSINESS、原子替换并写
    `system_config_apply_receipt`；失败保留 last-known-good 并回执 `rejected`。
  - 被 effective 映射引用的环境禁止删除；兼容别名不是 Namespace，不显示在环境列表，也不进入晋升拓扑。
  - Nacos 持久订阅固定创建时的内部环境与 wire alias；切换只影响新请求和新订阅。Eureka peer replication
    继续使用静态显式拓扑，不因缺省请求映射变化而迁移。

### Phase 4：重启配置与实例回执

- 支持 `RestartRequired` desired snapshot、启动期 overlay 和 apply status。
- 页面明确区分“已发布”与“已生效”。

### Phase 5：高级治理

- 按需增加 bundle manifest、实例灰度、审批策略和外部 Secret Provider adapter。

## 非目标

- 不把整份 YAML 自动映射成可编辑表单。
- 不允许 Console 或 Agent 直连核心数据库修改配置。
- 不承诺所有已发布配置都能热更新。
- 不把静态文件废弃；它始终是可独立启动的安全基线。

## 证据

- `bootstrap/config/config.go`：Pole Server 配置聚合并在启动时一次读取。
- `pkg/console/config/config.go`：Console 配置一次读取并展开环境变量，无 Watch/reload。
- `bootstrap/server.go`：Store、组件、API Server 与 Console 的启动顺序。
- `apis/store/store.go`：核心 Store 通过全局配置和一次初始化获取。
- `pkg/console/internal/observer/api.go`：Console ObserverStore 只有 history/event 查询。
- `pkg/config/watcher.go`：配置中心已有发布通知和长轮询机制。
- `pkg/cache/config/config_file.go`：active release 缓存和本地发布事件。
- `plugin/store/mysql/scripts/pole_server.sql`：`server_setting` 只有 DDL。
- `pkg/workloadcredential/server.go`：已有受控服务实例替换基础。
- `pkg/common/otel/config.go`：OTel 运行参数需要专用重建与 drain。
- `plugin/apiserver/httpserver/admin_access.go`：日志级别和 pprof 已存在局部运行时操作。
- `pkg/console/internal/observabilityquery/config.go`：Console 观测查询配置是低风险动态候选。
- `pkg/console/internal/router/agent_router.go`：Agent TTL 与 Workbench 当前在路由构造时固定。
- `pkg/systemconfig/registry.go`：定义注册、查询、稳定排序和服务端 Secret 脱敏。
- `pkg/systemconfig/source.go`：静态 YAML、环境变量与编译默认值来源索引。
- `bootstrap/config/system_settings.go`：Pole Server 首批 29 项显式配置定义。
- `pkg/console/config/config.go`：Console AgentDefinition、LLM、MCP 与运行限制的类型化启动结构。
- `pkg/console/internal/systemsettings/provider.go`：Console 首批 33 项显式配置定义，其中 Agent 领域 13 项。
- `pkg/console/internal/handlers/system_config.go`：Console 鉴权后跨进程聚合两个 effective snapshot。
- `pkg/console/internal/systemsettings/manager.go`：Agent 草稿、发布前连接校验、Secret 解析、原子运行时与周期协调。
- `pkg/console/internal/observer/mysql/system_settings.go`：Console 系统配置版本和加密 Secret 持久化。
- `web/console/src/pages/SystemConfiguration/index.tsx`：独立配置页面、来源概览、Agent 草稿审阅与发布。

## 相关页面

- [[configuration]]
- [[architecture]]
- [[config-center]]
- [[adr-console-agent-resource-workbench]]
- [[auth-system]]
- [[storage]]
- [[adr-pole-self-management-control-loop]]
