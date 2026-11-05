---
title: AI 原生功能：MCP、A2A、Pole Agent 与 Skill Marketplace
tags: [ai, mcp, a2a, skill, marketplace]
links: [storage, cache-layer, api-servers, adr-a2a-agent-registry, adr-console-agent-resource-workbench, adr-pole-self-management-control-loop, adr-ai-resource-environment-binding, skill-marketplace, adr-skill-marketplace-federation]
updated: 2026-08-12
sources: 24
---

# AI 原生功能：MCP、A2A、Pole Agent 与 Skill Marketplace

## 概览

AI 原生功能使 Pole 的服务注册中心和治理能力对 AI 智能体（LLM）可见，目前包含：

- **MCP Registry** — 注册和发现 MCP（模型上下文协议）服务器
- **A2A Agent Registry** — 注册和发现 A2A Agent Card，使 Agent 能按能力、skill 和协议端点发现其它 Agent
- **Pole Agent** — 通过受控工具调用帮助用户查询和修改 Pole 资源
- **Skill Marketplace** — 发布、审核、同步和分发不可变 Agent Skills Bundle

MCP Server 与 A2A Agent Registry 记录是 Namespace 中的环境实例。跨环境身份由控制面稳定
逻辑定义表达，不使用同名自动聚合；完整模型见 [[adr-ai-resource-environment-binding]]。
逻辑定义与环境绑定通过 `/ai/mcp/v1/definitions` 和 `/ai/a2a/v1/definitions` 管理；详情页
可显式关联旧环境实例，并在同一逻辑定义的可访问环境之间切换。逻辑定义首期直接读取 Store，
环境实例继续使用原有 Registry Cache。

Console 的 A2A 与 MCP 一级页面已使用同一两层信息架构：无环境上下文时默认展示全局逻辑资源
目录，并显示环境覆盖与显式绑定入口；从环境空间携带 `namespace` 进入时默认展示该环境的实际
实例。环境实例创建表单锁定入口环境，MCP 的 Pole 服务候选也限定在同一环境。列表中的环境空间
可返回环境资源工作区；`pole-system` 不进入业务逻辑资源聚合。

存储层接口见 [[storage]]，缓存机制见 [[cache-layer]]，HTTP API 端点见 [[api-servers]]。

---

## MCP 服务器注册中心

### 是什么

MCP（Model Context Protocol，模型上下文协议）是一个开放标准，使 AI 智能体能够发现并调用外部工具。Pole 的 MCP Registry 支持：
- 注册 MCP 服务器（地址、工具、能力）
- 查询已注册的 MCP 服务器（按名称、命名空间、类型过滤）
- 让 AI 智能体通过 Pole 发现可用的 MCP 服务器

### 领域模型（`apis/pkg/types/ai/mcp.go`）

```go
type MCPServer struct {
    ID          string
    Name        string       // 命名空间内唯一
    Namespace   string
    Description string
    Protocol    string       // "http"、"stdio"、"sse"
    Endpoint    string       // URL 或命令
    Type        string       // 服务器分类
    Metadata    map[string]string
    Flag        int          // 0=活跃，1=已删除
    CTime, MTime time.Time
}

type MCPServerTool struct {
    ID          string
    ServerID    string       // 外键，关联 MCPServer
    Name        string
    Description string
    InputSchema string       // JSON Schema
}
```

### 存储（`apis/store/ai.go → MCPServerStore`）

关键方法：
- `CreateMCPServer`、`UpdateMCPServer`、`DeleteMCPServer`
- `GetMCPServer(id)`、`GetMCPServerByName(name, namespace)`
- `GetMoreMCPServers(mtime, firstUpdate)` — 增量缓存同步
- `QueryMCPServers(filter, offset, limit)` — 分页搜索
- 工具管理：`CreateMCPServerTool`、`GetMCPServerToolsByServerID`

### HTTP API（`plugin/apiserver/httpserver/aimcp/`）

- `GET /mcp/v1/servers` — 列出/过滤 MCP 服务器
- `POST /mcp/v1/servers` — 批量创建 MCP 服务器

### MCP 协议暴露（`plugin/apiserver/httpserver/aimcp/mcp_server.go`）

Pole 将自身的管理功能*以* MCP 工具的形式暴露，供 AI 智能体使用：

```
工具：list_mcp_servers
  输入：{namespace?, name?, type?, offset, limit}
  输出：MCPServer 对象数组

工具：create_mcp_servers
  输入：MCPServer 规格数组
  输出：批量写入响应
```

这些工具通过 `mark3labs/mcp-go` 注册，并通过 HTTP（SSE 或流式传输）提供服务。

### 缓存（`pkg/cache/ai/`）

- `MCPServerCache` — 按 ID 和按 `namespace/name` 索引，使用增量更新模式（每隔 1 秒轮询 `mtime`）。

---

## 与外部 AI 智能体的集成

典型使用场景：

```
AI 智能体（Claude、GPT 等）
    ↓ （MCP 协议）
Pole MCP 端点（/mcp）
    ↓
list_mcp_servers 工具 → 返回已注册的 MCP 后端
    ↓
Pole 内部 MCP 注册表
```

这使智能体能够自主发现可用的 MCP 后端，无需硬编码配置。

## A2A Agent Registry

A2A（Agent2Agent）用于 Agent 之间的协作。它与 MCP 的边界是：

- MCP 面向工具和资源调用，适合结构化、相对无状态的能力。
- A2A 面向 Agent 与 Agent 之间的任务协作，适合多轮、长任务、可流式更新或异步回调的场景。

Pole 中的 A2A 首期只做注册与发现，不做完整任务运行时：

- 注册 A2A Agent Card，包括 identity、supported interfaces、skills、capabilities、security 和 raw card JSON。
- 复用现有 namespace、Store、Cache、HTTP 插件和 auth 模式。
- 通过 REST API 和 Console 页面暴露 A2A Agent Registry 管理能力。
- A2A task proxy、SSE streaming 转发和 push notification broker 属于数据面/网关运行时能力，不纳入 pole-control-plane 的 Registry 方案。

### HTTP API（`plugin/apiserver/httpserver/aia2a/`）

接口前缀为 `/ai/a2a/v1`：
- `GET /agents` — 从缓存查询 Agent，支持分页与过滤
- `POST /agents` — 批量创建 Agent
- `PUT /agents` — 批量更新 Agent
- `POST /agents/delete` — 批量软删除 Agent
- `GET /agent/skills` — 按 `agent_id` 或 `agent_name + agent_namespace` 查询技能
- `GET /agents/{id}/card` — 返回原始 Agent Card JSON；无原始 card 时返回 Registry 记录

### 存储与缓存

- `apis/store.A2AAgentStore` 提供 Agent 增删改查、按 `namespace/name` 唯一查询、增量同步、技能查询和分页查询。
- MySQL 表：`a2a_agent`、`a2a_agent_interface`、`a2a_agent_skill`，采用 `flag=1` 软删除和 `mtime` 增量刷新。
- `pkg/cache/ai.A2AAgentCache` 按 ID、`namespace/name` 和 namespace 建索引，并支持 skill tag、协议绑定、后端服务和能力过滤。

### Console 页面（`web/console/src/pages/AI/A2A/`）

A2A 页面与 MCP Registry 页面保持同一交互风格：逻辑资源/环境实例视图、筛选工具栏、主表格、
抽屉编辑和详情页。

页面操作流：
- 逻辑资源页展示跨环境稳定身份、环境覆盖、实例数和版本，并允许显式绑定尚未归属其它定义的环境实例。
- 环境实例页按名称前缀、环境空间、协议绑定、skill tag、后端类型、streaming 和 push 能力筛选 Agent。
- 表格展示名称、环境空间、版本、协议、能力标签、技能数、后端绑定、拉取状态和操作时间。
- 新建/编辑抽屉分为“基础信息”“接入与能力”“技能”“来源与 Card”四段，覆盖 Agent Card 元数据、访问接口、技能声明和原始 card JSON。
- 详情操作只提供“查看 Agent Card”和“查看技能”，不提供 message/task/push 执行按钮，避免把数据面能力误放到控制面。

前端数据流：
- `services/a2a.ts` 对接 `/ai/a2a/v1` REST API。
- `modules/ai/a2a.ts` 管理列表、编辑对象、skills 和 card 详情状态。
- `modules/store.ts` 将 `aiA2A` reducer 注册到全局 store。
- `router/modules/ai.ts` 将 `/ai/a2a` 作为 AI 工具菜单的一等页面暴露。

完整方案背景见 [[adr-a2a-agent-registry]]。

## Console Pole Agent

Console 已提供独立一级 `/agent` 工作模式。它不属于“AI 工具”资源管理分组：A2A Agent 与 MCP 服务页面负责注册表管理，Pole Agent 则是用户通过对话调用控制面工具的操作入口。默认侧边布局在侧栏底部、版本信息上方提供工作区切换：普通模式进入 Agent，Agent 模式在同一位置返回普通控制台；Agent 模式隐藏普通资源导航但保留返回入口，切回时恢复最近访问的普通页面。无侧栏的顶部布局保留页头切换兜底。

页面采用 280px 本地会话侧栏、全局栏、对话标题栏、消息流、浮动输入器和 294px 可收起上下文面板。会话支持搜索、日期分组、重命名、删除与快捷新建，消息、草稿、资源引用和 4/10/20 轮记忆窗口持久化在浏览器 IndexedDB。资源读取和草稿修改展示真实工具调用轨迹；临时 diff 作为消息流中的工具结果展示，用户确认后只保存编辑态资源并返回 `waiting_for_publish`，发布仍由用户进入现有配置分组完成。配置文件详情可通过“交给 Agent”传递 `config.file + namespace/group/name + returnTo`，不把配置正文放入 URL。

Phase 1 最小真实运行时已在 Console 后端提供一等 `PoleAgent`：页面调用 `/ai/agent/v1/turns`；Pole Agent 内部加载版本化 System Prompt，通过 OpenAI-compatible LLM Gateway 运行最多 8 轮 model-tool loop，并以当前用户身份连接 Pole MCP、导入白名单工具。Pole MCP 已提供 namespace、MCP Registry 和配置文件只读工具；配置 update 只通过内部 proposal 工具进入不可绕过的 `ChangeApprovalKernel`，承担预览、哈希、幂等、并发检查和草稿执行。运行时探针同时检查模型配置与 MCP 连接，未就绪时页面 fail closed。当前仍未覆盖流式输出、服务端会话持久化、create、治理规则写入和更多资源域。

Pole Agent 本体属于 `pole-system`，业务环境由每次 Turn 的强类型 Namespace scope 决定。
普通写操作只能作用于一个业务环境；资源上下文和工具参数必须在服务端接受同一 scope 校验，
不能只依赖 Prompt。服务端会以当前 Actor 调用 Namespace 目录，拒绝无权、缺失或 SYSTEM
Namespace；A2A 调用也必须显式传递并接受同一 scope 校验。Console 提供单环境模式和显式
跨环境只读比较模式，后者至少选择两个 BUSINESS Namespace。

Agent 运行配置已从 Kubernetes 日常环境变量迁入 Pole 内部 System Settings：Admin 在 `/system-configuration?component=pole-console&domain=agent` 编辑 Gateway、模型、Prompt、MCP 白名单和 write-only API key；MySQL 保存不可变配置版本及信封加密 Secret，发布前重新探测模型与 MCP，成功后当前实例原子切换，其他实例通过周期 reconcile 收敛。静态 YAML 仅保留首次启动基线，K8s 只保留数据库连接与 Secret 根密钥。

Pole 自身能力现已形成自动闭环：`pole-self-manager` 在启动和周期 reconcile 中把 Control Plane MCP 及真实工具快照登记为 `pole-system/pole-control-plane`，`all` 模式再从真实 Agent Card 投影并登记 Pole Agent A2A 能力。Console Agent 以 Registry 自然键解析 MCP 地址，不再把固定 endpoint 当作唯一事实来源。管理员保存 Agent Prompt/模型/工具策略后自动执行候选探测并应用；失败版本保留为 rejected 草稿，运行时继续使用 last-known-good。完整边界见 [[adr-pole-self-management-control-loop]]。

## Skill Marketplace

Skill Marketplace 是 AI 工具下独立于 MCP、A2A Card Skill 与 Pole Agent 的发布目录。控制面以
`publisher/name` 管理不可变 SemVer Release，对上传 ZIP 进行 Agent Skills manifest、路径、大小、权限、
SHA-256、Ed25519 签名和基础静态扫描校验；默认使用 MySQL BLOB `BundleStore`，并通过 Pole、HTTP Index
和 GitHub Tag/Release Adapter 将外部版本冻结到本地。Git 手工导入可从不可变 commit 快照识别仓库根 Skill，
或指定目录下的全部一级 Skill；Console 先预览发现结果，再逐项冻结 Bundle 并返回批量摘要。

HTTP 根为 `/api/skill-marketplace`。公共且已发布的目录、详情和精确 Bundle 允许匿名读取；私有 Skill
通过 User、UserGroup、Role grant 控制。Console 路由为 `/ai/skills` 与
`/ai/skills/:publisher/:name`，只管理、审核和安全预览，不执行或安装 Bundle。安装生命周期由独立
`pole-ai` CLI 的内容 Store、lockfile 及 Codex、Claude Code、通用目录 Adapter 负责。完整产品与技术
边界见 [[skill-marketplace]]、[[adr-skill-marketplace-federation]]。

## 相关页面

- [[storage]]
- [[cache-layer]]
- [[api-servers]]
- [[adr-a2a-agent-registry]]
- [[adr-console-agent-resource-workbench]]
- [[adr-pole-self-management-control-loop]]
- [[adr-ai-resource-environment-binding]]
- [[skill-marketplace]]
- [[adr-skill-marketplace-federation]]
