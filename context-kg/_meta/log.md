---
title: 操作日志
tags: [meta, log]
links: [index, schema]
updated: 2026-10-10
sources: 0
---

# 操作日志

追加式记录，只增不改。最新记录在文件末尾。格式规范见 [[schema]]。

---

## [2026-05-14] ingest | pole-control-plane 全量初始化

- 扫描 656 个 Go 文件（模块：`github.com/pole-io/pole-server`）
- 生成 12 个知识页面，覆盖以下领域：
  - 概览（overview）：项目介绍、技术选型、目录结构、启动流程
  - 架构（architecture）：四层架构、插件系统、Store/Cache 接口、拦截器链
  - 业务领域（domain-components）：命名空间、服务发现、配置中心、治理规则、管理后台
  - API 服务端（api-servers）：HTTP、gRPC、xDS v3、Nacos v1/v2、Apollo、Eureka、MCP 集成
  - 存储层（storage）：接口层次、MySQL 实现、软删除、增量查询、分布式锁
  - 缓存层（cache-layer）：缓存子类型、1 秒刷新循环、-5 秒安全窗口
  - 认证系统（auth-system）：UserServer、StrategyServer、拦截器链、Token 流程
  - AI 功能（ai-features）：MCP Registry、领域模型、HTTP API、缓存
  - 公共基础设施（common-infra）：日志、EventHub、Batch Controller、OTel、同步原语
  - 配置参考（configuration）：YAML 结构、插件配置、API 服务端配置、部署目录
  - 模式与约定（patterns）：11 种关键代码模式
  - 测试（testing）：Mock Store/Auth、集成套件、测试规范
- 首次建立 wiki 结构，新建元文件：schema.md、index.md、log.md
- 替换旧 README.md 为 index.md

## [2026-05-14] restructure | wiki 目录结构重组为方案 C

- 新建子目录：_meta/, overview/, domains/, infra/, ai/, guides/
- 拆分 domain-components.md → 5 个独立域文件：
  - domains/namespace.md
  - domains/service-discovery.md
  - domains/config-center.md
  - domains/governance-rules.md
  - domains/admin.md
- 移动 12 个内容页面至对应子目录：
  - overview/: overview.md, architecture.md, api-servers.md, configuration.md
  - infra/: storage.md, cache-layer.md, auth-system.md, common-infra.md
  - ai/: ai-features.md
  - guides/: patterns.md, testing.md
- 更新 _meta/index.md 为分类视图（五大类别：overview/domains/infra/ai/guides）
- 更新 _meta/schema.md 目录结构约定为新分层结构
- 删除根目录下所有旧文件（domain-components.md 及已移动的 12 个页面）

## [2026-06-08] ingest | 治理规则统一存储与缓存更新方案归档

- 扫描 0 个 Go 文件，本次为技术方案归档。
- 更新页面：governance-rules, index
- 变更摘要：
  - 将治理规则统一表 `governance_rule` / `governance_rule_release` 的设计归入治理规则域。
  - 记录 `GovernanceRuleRepository` 统一 DB 操作原则。
  - 记录 `GovernanceRuleUpdateCache` 按 `rule_type` fan-out 给 watcher 的缓存更新模型。
  - 记录泳道方案 A：`LaneGroup` 是聚合根，`LaneRule` 是 group JSON 内最多 20 个的子对象。
  - 清理重组前残留的 `domain-components` 失效链接，改为当前知识库入口 `index`。

## [2026-06-09] restructure | context-kg 重组为 business / technical / quality 三域结构

- 按业务知识域、技术知识域、质量保障知识域重组目录。
- 新增业务基础页面：
  - business/terminology.md
  - business/domain-models.md
  - business/business-rules.md
- 移动功能档案到 business/feature-registry/。
- 移动架构、模块、接口、环境、约定页面到 technical/。
- 移动测试知识到 quality/automation/。
- 将治理规则统一存储与缓存更新方案拆为 `technical/adr/adr-governance-rule-unified-storage.md`，业务功能页只保留摘要和链接。
- 更新 _meta/schema.md 为三域结构规则，并明确长期技术方案归档到 `technical/adr/`。
- 更新 _meta/index.md 为三域目录入口。

## [2026-06-09] restructure | 任务记录迁移与 AGENTS 主文件反转

- 将任务记录从 `docs/tasks/` 迁移到 `context-kg/tasks/`。
- 删除旧 `docs/design/` 长期设计文档，长期知识统一归档到 `context-kg/`。
- 将仓库内指导文件反转为 `AGENTS.md` 保存实际内容，`CLAUDE.md` 使用相对软链接指向 `AGENTS.md`。
- 更新 schema 与 index，将 `todo`、`lessons` 纳入任务过程记录入口。

## [2026-06-10] ingest | A2A Agent Registry 功能设计归档

- 扫描 8 个相关资料与源码入口，本次为技术方案归档。
- 新增页面：technical/adr/adr-a2a-agent-registry.md。
- 更新页面：ai-features, index。
- 变更摘要：
  - 明确 A2A 与 MCP 的功能边界：MCP 管工具/资源，A2A 管 Agent 间任务协作。
  - 决定首期做 A2A Agent Card 注册、发现、缓存和 MCP 工具暴露，不接管完整 A2A task runtime。
  - 记录 A2AAgent、A2AAgentInterface、A2AAgentSkill 的建议数据模型。
  - 记录 Store、Cache、HTTP 插件、Console、安全和测试落点。

## [2026-06-10] refine | A2A Agent Registry 控制面范围收敛

- 按用户纠正收敛 A2A 方案边界。
- 更新页面：adr-a2a-agent-registry, ai-features, todo, lessons。
- 变更摘要：
  - 明确 pole-control-plane 只负责 A2A Agent Registry。
  - 将 A2A task proxy、SSE streaming 转发、push notification broker、task 状态机和 artifact 存储归类为数据面/网关/agent runtime 能力。
  - 将该边界沉淀到 lessons，避免后续把数据面能力写成 control-plane 二期规划。

## [2026-06-10] implement | A2A Agent Registry 实现集成

- 将 A2A Agent Registry 从独立 worktree 集成到当前 develop 工作区。
- 更新页面：ai-features, adr-a2a-agent-registry, todo, lessons。
- 变更摘要：
  - 新增 `A2AAgent`、`A2AAgentInterface`、`A2AAgentSkill` 类型、Store 接口、MySQL 实现和 SQL schema。
  - 新增 `A2AAgentCache`，接入 CacheManager 并支持按 namespace/name、skill、协议绑定和后端信息查询。
  - 新增 `/ai/a2a/v1` REST API 子包，提供 Agent 注册、更新、删除、查询、skill 查询和 Agent Card 查询。
  - 新增 Console A2A Agents 页面，包含列表筛选、抽屉新建/编辑、Agent Card 查看和技能详情查看。
  - 保持控制面边界，不提供 A2A task proxy、SSE 转发或 push broker。

## [2026-06-15] ingest | Console API、Client 与权限接口 E2E 测试用例设计

- 扫描 12 个相关测试、Console router、HTTP access、接口 service 和知识库入口。
- 新增页面：quality/testcases/console-client-auth-e2e-testcases.md。
- 更新页面：testing, index, schema, todo, lessons。
- 变更摘要：
  - 将 E2E 测试收敛为 Console API 和 Client/Auth 两层接口套件，不纳入前端交互测试能力。
  - 设计命名空间、MCP、A2A、服务、别名、实例、9 类治理规则、用户、用户组、角色、权限策略的读写覆盖矩阵。
  - 设计通过 Console 写入后由 8090 Client API 轮询验证服务发现和治理规则发布态的用例。
  - 设计 `consoleOpen` / `clientOpen` 四象限配置开关和 Console/Client 权限策略生效用例。

## [2026-06-15] lint | context-kg skill 与标题契约校验

- 新增本地 Codex skill：`context-kg-maintainer`，用于快速更新、生成和校验 context-kg 知识库。
- 使用 skill 自带 lint 脚本检查当前知识库。
- 修正若干页面 frontmatter `title` 与首个 H1 不一致的问题。
- 更新页面：index, schema, namespace, service-discovery, config-center, governance-rules, admin, todo。

## [2026-06-15] refine | 接口 E2E 测试用例边界收敛

- 按用户要求移除前端页面和 Playwright 测试能力口径。
- 更新页面：console-client-auth-e2e-testcases, testing, todo, lessons。
- 变更摘要：
  - 在测试用例文档中明确 E2E 只验证 HTTP/API 链路。
  - 增加非目标清单，排除 Playwright、浏览器、DOM、截图、页面布局和前端路由。
  - 将少量 UI 口径描述改为接口字段、接口响应和接口可用性断言。
  - 保留 8080 Console API、8090 Client API、权限策略和 `consoleOpen/clientOpen` 开关矩阵作为接口验收边界。

## [2026-06-16] implement | 接口 E2E 测试套件补齐

- 新增测试目录：test/e2e/internal/e2e, test/e2e/console_api, test/e2e/client, test/e2e/auth。
- 更新页面：testing, todo。
- 变更摘要：
  - 新增基于 testcontainers 的 MySQL 环境、30000 起始端口分配和临时 all 模式配置。
  - 新增 Console API 资源读写、Client Discover 传播和权限策略接口 E2E 测试代码。
  - 使用 `//go:build e2e` 隔离 E2E，默认测试命令不启动 Docker。
  - 记录 macOS 下 testcontainers 间接依赖 `go-m1cpu` 的 cgo 崩溃规避方式，推荐 `CGO_ENABLED=0`。

## [2026-06-16] refine | 接口 E2E 测试文档边界强化

- 按用户要求再次收敛测试用例文档，只保留接口维度测试能力。
- 更新页面：console-client-auth-e2e-testcases, testing, todo。
- 变更摘要：
  - 新增“能力边界”说明，明确自动化入口统一为 Go test。
  - 明确 8080 console proxy 属于接口链路验证，不等同于控制台页面测试。
  - 明确不建设 `console/web/e2e`、Playwright、浏览器脚本、DOM 断言、截图比对或页面交互流程。

## [2026-06-16] implement | 接口 E2E 测试覆盖补齐

- 补齐设计文档中仍缺失的接口 E2E 覆盖点。
- 更新页面：todo。
- 变更摘要：
  - Console API E2E 增加资源删除后不可见、服务 update/delete、auth 资源 delete 和治理规则 gray/stopbeta/rollback/release delete 覆盖。
  - Client E2E 增加实例删除和治理规则删除后的 Discover 响应不包含目标资源断言。
  - Auth E2E 增加 RegisterInstance、Heartbeat 在 `clientOpen` 开关和授权策略下的放行/拒绝用例。
  - 继续保持接口维度，不引入前端页面或 Playwright 测试。

## [2026-06-16] refine | 接口 E2E 测试用例文档二次整理

- 按用户要求继续收敛测试用例文档，只保留接口维度测试能力。
- 更新页面：console-client-auth-e2e-testcases, testing, todo。
- 变更摘要：
  - 将主测试用例文档中的前端自动化表述收敛为非目标边界。
  - 保留 8080 console proxy 作为接口链路验证，不纳入控制台页面验收。
  - 测试能力入口继续统一为 Go test。

## [2026-06-16] implement | 接口 E2E 鉴权与治理覆盖补齐

- 补齐权限策略、Client 鉴权传播和治理规则类型隔离的接口 E2E 表达。
- 更新页面：console-client-auth-e2e-testcases, todo。
- 变更摘要：
  - Auth E2E 使用真实 policy principals/resources/functions 结构，并覆盖用户组继承、角色函数变更、Token 禁用/刷新、策略删除和策略更新传播。
  - 治理规则权限增加老规则路由与新增流量治理规则的授权/未授权分支。
  - 测试用例文档将灰度、路由、泳道的客户端 label 断言收敛到当前 HTTP/API 可验证的发布记录、规则内容和 Discover 发布态。
  - 继续保持接口维度，不引入前端页面、浏览器或 Playwright 测试能力。

## [2026-06-21] refine | 治理规则编辑抽屉双栏滚动交互约定

- 将治理规则编辑抽屉的左侧内部滚动、右侧 Spec 高度和 StickyTool 布局要求沉淀为长期技术约定。
- 更新页面：patterns, index, todo, lessons。
- 变更摘要：
  - 明确编辑器正文固定高度、shell hidden、左侧 pane 内部滚动、右侧 Spec 跟随 shell 高度。
  - 明确 column flex 下直接子 section 必须禁止 shrink，避免共享 section 的 overflow hidden 裁切内容。
  - 明确真实验收必须验证 scrollHeight/clientHeight 和 scrollTop，而不是只看 CSS overflow。

## [2026-06-22] ingest | Console OIDC 用户来源与企业目录同步方案归档

- 新增页面：technical/adr/adr-console-oidc-identity-source.md。
- 更新页面：auth-system, index, todo。
- 变更摘要：
  - 明确 OIDC 登录和企业目录同步都是 Console 扩展点，不进入 pole-server 核心鉴权链。
  - 明确 native / oidc 登录模式启动期互斥，运行期间不支持切换。
  - 明确 OIDC 只改变 Pole User 来源，业务请求仍使用 Pole token 和现有资源授权逻辑。
  - 设计飞书、Lark、钉钉 DirectoryProvider 同步企业账号为 `source=oidc` 的 Pole User，解决未登录用户无法提前授权的问题。

## [2026-07-02] implement | 全仓问题修复记录

- 更新页面：todo。
- 变更摘要：
  - 记录 batchctrl graceful stop 卡死、HDS unary 未实现、服务删除订阅图清理、前端标准脚本和 lint 入口修复。
  - 补充本轮 Go、前端和 context-kg 验证命令结果。

## [2026-07-03] implement | 前端构建 warning 清理

- 更新页面：todo。
- 变更摘要：
  - 更新 Browserslist/caniuse 锁文件数据，消除 Browserslist 过期 warning。
  - 恢复 Vite CSS code split，并按依赖族配置 Rollup manualChunks，消除大 JS/CSS chunk warning。
  - 新增 Vite build 包装脚本，为 Node 22+ 提供有效 localStorage 文件路径，消除 `--localstorage-file` warning。

## [2026-07-05] implement | 配置分组详情设计适配与多灰度 spec 契约归档

- 新增页面：technical/adr/adr-config-gray-release-spec-contract.md。
- 更新页面：index, todo, lessons。
- 变更摘要：
  - 配置分组详情页按设计交接调整为文件树 + 中央业务画布，并补配置编辑、发布记录、订阅查询反回归脚本。
  - 明确方案 B 的多 active gray、灰度转正式草稿和停止单个灰度需要同步更新 specification。
  - 记录构建后必须重启 console/all-mode 并验证入口 hash，避免浏览器或运行进程持有旧静态资源。

## [2026-07-05] ingest | OTel 可观测性平台与 Kubernetes 部署方案归档

- 新增页面：technical/adr/adr-otel-observability-platform.md。
- 更新页面：index, todo。
- 变更摘要：
  - 明确系统内部可观测性与业务服务调用可观测性两个平面。
  - 选择 OpenTelemetry Collector Contrib 作为统一采集管道，OpenObserve 作为默认存储查询后端。
  - 明确业务普通日志不进入平台，只有结构化 event 与 audit 走 OTel Logs。
  - 设计 Kubernetes 快速体验路径，由 Collector、OpenObserve、pole-control-plane 和样例业务服务组成。

## [2026-07-05] refine | OTel metrics 与 event 命名规范归档

- 更新页面：technical/adr/adr-otel-observability-platform.md, todo。
- 变更摘要：
  - 定义 Pole 自定义 metrics 使用 `pole.` 前缀，HTTP/RPC 优先复用 OTel 标准语义指标。
  - 明确 metrics label 只放低基数字段，`rule.id`、`config.file`、`instance.id`、`trace_id` 等高基数字段只进入 event 或 trace。
  - 定义平台内部 event、审计 event 和业务 event 的 EventName 与关键 attributes。
  - 明确现有 `prometheus` entry 保持旧指标名，新增 `otel` entry 才输出新 `pole.*` 名称。

## [2026-07-05] refine | OTel metrics 覆盖与联动设计补充

- 更新页面：technical/adr/adr-otel-observability-platform.md, todo。
- 变更摘要：
  - 补齐平台内部 metrics 场景：启动、ready、插件、EventHub、队列、健康检查、配置 Watch、治理发布、推送、鉴权、DB 连接池和 telemetry export。
  - 明确业务 CPU/Mem 由 Kubernetes/Collector 资源指标采集，并通过 `service.instance.id`、`k8s.pod.uid`、`pole.service.name` 与业务请求指标联动。
  - 明确治理 metrics 只做低基数聚合，治理 event 通过 `pole.governance.decision.id`、`rule.id`、`trace_id/span_id` 定位具体规则和调用链。
  - 补充 Console 同时间窗联动展示请求、资源、治理事件和审计信息的分析规则。

## [2026-07-05] refine | OTel 服务绑定标签与语言体系补充

- 更新页面：technical/adr/adr-otel-observability-platform.md, todo。
- 变更摘要：
  - 增加 `pole.io/*` 服务/实例保留标签，映射到 Pole、Kubernetes、OTel runtime 资源属性。
  - 增加 `pole.io/runtime-language` / `pole.runtime.language`，用于 Console 选择 Java、Go、Rust、Node.js、Python、.NET 等运行时面板。
  - 明确 JVM、Go runtime 指标使用 OTel 标准 metrics name，Pole 只定义绑定属性和展示规则。
  - 明确服务调用、CPU/Mem、runtime metrics、trace 和 event 的绑定优先级。

## [2026-07-05] restructure | 可观测性 ADR 子目录与 Rust SDK 职责归档

- 新增目录：technical/adr/observability/。
- 移动页面：technical/adr/adr-otel-observability-platform.md → technical/adr/observability/adr-otel-observability-platform.md。
- 新增页面：technical/adr/observability/adr-pole-rust-client-observability.md。
- 更新页面：index, todo。
- 变更摘要：
  - 将可观测性相关 ADR 单独收纳到 observability 子目录。
  - 明确 `pole-rust-client` 负责 Resource attributes 注入、SDK metrics/events、治理决策关联和 tracing hooks。
  - 明确 Rust SDK 不采集业务普通日志、不采 Kubernetes CPU/Mem、不替代业务框架 instrumentation。
  - 明确治理 metrics、event、trace 通过 `pole.governance.decision.id` 关联。

## [2026-07-07] refine | 可观测性后端选型调整为 GreptimeDB 优先

- 更新页面：technical/adr/observability/adr-otel-observability-platform.md, technical/adr/observability/adr-pole-rust-client-observability.md, index, todo。
- 变更摘要：
  - 将长期默认观测后端从 OpenObserve 调整为 GreptimeDB。
  - 保留 OpenObserve 作为 quickstart / 可选 provider，用于快速体验和内置 UI 辅助排查。
  - 明确 Collector 仍是统一采集/处理/路由入口，GreptimeDB/OpenObserve 都不替代 Collector。
  - 明确 Console 通过 `observability-query` provider 查询 GreptimeDB，不允许前端直连观测后端。

## [2026-07-12] ingest | 实例 TCP/HTTP 主动健康检查设计

- 新增页面：technical/adr/adr-instance-active-healthcheck.md。
- 更新页面：business/feature-registry/service-discovery.md, index, todo, lessons。
- 变更摘要：
  - 明确 Console 手工实例禁用心跳，开放 TCP/HTTP 控制面主动探测。
  - 明确 specification、checker 插件、调度器、MySQL 映射与 Console 必须端到端适配。
  - 明确复用 `health_check.ttl` 保存主动探测间隔，HTTP 路径通过实例保留元数据持久化。

## [2026-07-12] implement | 实例 TCP/HTTP 主动健康检查落地

- 更新页面：technical/adr/adr-instance-active-healthcheck.md, business/feature-registry/service-discovery.md, todo, lessons。
- 变更摘要：
  - 完成 TCP/HTTP specification、服务端探测插件、调度、MySQL、Console 创建与编辑链路。
  - 修复无健康 checker 时主动检查无法调度的问题，并增加非隔离节点兜底。
  - 修复 MySQL 先构造健康检查后解析 metadata 导致 HTTP 路径回退的问题。
  - 通过真实 HTTP 服务验证实例健康到不健康状态切换，并完成测试数据清理。

## [2026-07-12] ingest | Console Fluent UI v9 设计系统迁移

- 新增页面：technical/adr/adr-console-fluent-ui-design-system.md。
- 更新页面：index, todo, lessons。
- 变更摘要：
  - 将 Console 全站组件与视觉体系确定为 Microsoft Fluent UI React v9。
  - 建立仓库级 Fluent 适配边界，保留复杂表单状态契约，统一用户可见控件。
  - 明确蓝、灰、白企业软件视觉规范和 TDesign 直接 import 禁止规则。

## [2026-07-12] implement | Console Fluent UI v9 全站落地

- 更新页面：technical/adr/adr-console-fluent-ui-design-system.md, todo, lessons。
- 变更摘要：
  - 完成 Fluent Provider、品牌主题、导航、表格、表单可见控件、弹层、通知和统一图标迁移。
  - 将旧 Form 状态控制器与少数复杂控件限制在 `components/Fluent` 适配边界，页面禁止直接导入 TDesign。
  - 修复 Layout 静态子组件、定位枚举、受控输入、Portal 点击拦截和 UI 手工拆包循环问题。
  - 通过全部前端验证脚本、生产构建、all-mode 重启以及主要页面桌面/移动端真实浏览器验证。

## [2026-07-12] refine | Fluent 侧边栏视觉纠偏

- 更新页面：todo, lessons。
- 变更摘要：
  - 移除 Fluent Nav 普通菜单项常驻灰色 surface，恢复白色连续企业控制台导航面。
  - 统一一级模块、二级缩进、hover、浅蓝选中态和左侧品牌色指示条。
  - 修正折叠态图标与箭头，并增加 Error Toast 内容归一化和静态回归检查。
  - 补齐 FluentProvider 根 DOM 的满高链路，确保侧边栏和 Footer 在长视口下占满页面。
  - 完成展开、折叠、窄屏、全部菜单跳转和 8080 生产环境验证。

## [2026-07-12] refine | 可观测性 Console 查询接口与 Rust SDK 动态配置设计

- 更新页面：technical/adr/observability/adr-otel-observability-platform.md, technical/adr/observability/adr-pole-rust-client-observability.md, index, todo。
- 变更摘要：
  - 新增 `/observability/v1` Console 查询接口设计，覆盖服务概览、拓扑、metrics、runtime、events、traces、治理、平台、审计和跨信号关联。
  - 明确 `observability-query` provider 屏蔽 GreptimeDB/OpenObserve 查询语法，Console 前端只传 Pole 领域条件和时间窗。
  - 设计 `pole-rust-client` 通过 Pole 服务发现获取 `pole-otel-collector` OTLP 上报地址，减少用户显式配置。
  - 设计 `pole-rust-client` 通过配置中心 `pole-sdk/observability.yaml` remote 下发采样、开关、batch/export 和 event 策略。
  - 明确 SDK remote config 最终映射为标准 OTel exporter 配置，不改变 OTLP 协议，也不绕过 Collector。

## [2026-07-17] implement | 系统命名空间删除保护与 Console 标识

- 更新页面：business/feature-registry/namespace.md, index, todo。
- 变更摘要：
  - 明确 `default` 与 `pole-system` 是不可删除的系统自带命名空间。
  - 明确后端删除入口与列表 `deleteable` 字段共同表达系统保护，不依赖 Console 单侧限制。
  - 明确 Console 对 `pole-system` 展示内部系统空间标识，并为两个系统空间提供专属禁删提示。

## [2026-07-17] implement | Console 完成零 TDesign Fluent UI 迁移

- 更新页面：technical/adr/adr-console-fluent-ui-design-system.md, todo。
- 变更摘要：
  - 以本地 Fluent 适配实现替换 Form、Popup、Tree、Transfer、日期时间等最后一批旧控件。
  - 从依赖声明、锁文件和生产依赖树移除 `tdesign-react` 与 `tdesign-icons-react`。
  - 清理旧 CSS 类名和主题变量，并增加覆盖依赖、源码、样式及原生交互标签的零残留门禁。
  - 保持页面配置抽屉和主要资源页面的迁移前布局与视觉，完成真实浏览器交互验证。

## [2026-07-18] refine | 可观测性数据体系首轮对接计划

- 更新页面：technical/adr/observability/adr-otel-observability-platform.md, technical/adr/observability/adr-pole-rust-client-observability.md, index, todo。
- 变更摘要：
  - 明确首轮先打通 control-plane、Rust SDK、sidecar 到 Collector、GreptimeDB、`/observability/v1`、Console 的真实数据闭环。
  - 补充 SDK 与 sidecar 共存时的上报分工，避免服务请求和治理 metrics 重复计数。
  - 明确 control-plane 侧优先补 `history/discoverEvent/statis` 的 `otel` entry，不绕开现有 chain。
  - 记录当前实现差距：Rust SDK 已有观测语义模型但缺真实 exporter 和服务发现/remote config 生命周期，sidecar 尚缺 OTel 上报模块。

## [2026-07-18] refine | 明确 observability 查询接口归属 console 模块

- 更新页面：technical/adr/observability/adr-otel-observability-platform.md, todo, lessons。
- 变更摘要：
  - 明确 `/observability/v1` 由 `pole-control-plane` 内的 console 模块提供，属于 Console 查询分析后端。
  - 区分 control-plane server 自身 OTel 上报职责与 console router/handler/provider 查询职责。
  - 修正首轮对接计划，避免把 `/observability/v1` 写成 core apiserver 职责。

## [2026-07-18] implement | 可观测性本地栈与 Console 查询骨架

- 更新页面：technical/adr/observability/adr-otel-observability-platform.md, todo。
- 变更摘要：
  - 新增 `deploy/observability` Docker Compose，本地拉起 GreptimeDB standalone 与 OpenTelemetry Collector Contrib。
  - Collector 统一接收 OTLP gRPC/HTTP，并将 traces、metrics、结构化 event/audit logs 写入 GreptimeDB `/v1/otlp`。
  - console 模块新增 `/observability/v1/platform/overview` router、handler、provider skeleton 和 `observabilityQuery` 配置入口。
  - 已用 Collector health、GreptimeDB health、OTLP smoke event 写入 `pole_events` 和 Go 回归测试验证首轮骨架。

## [2026-07-18] implement | control-plane statis OTel metrics entry

- 更新页面：technical/adr/observability/adr-otel-observability-platform.md, todo。
- 变更摘要：
  - 新增 `plugin/observability/statis/otel`，作为 `statis` chain 的 OTel metrics entry。
  - 将 API、Store、内部组件、缓存、服务发现、配置中心和客户端发现调用映射为 `pole.*` metrics。
  - 默认部署配置启用 `local + otel + prometheus`，并在启动阶段先初始化 statis chain，确保 OTel MeterProvider 先于旧 helper 指标注册。
  - 已通过定向 Go 测试、本地 Collector 导出 smoke 和 GreptimeDB 指标表查询验证。

## [2026-07-18] implement | 系统监控页接入 GreptimeDB 真实查询

- 更新页面：technical/adr/observability/adr-otel-observability-platform.md, todo。
- 变更摘要：
  - `observability-query` provider 从静态 DTO 改为 GreptimeDB SQL 查询，读取 `pole_control_plane_request_count_total` 与 duration sum/count 指标表。
  - `/observability/v1/platform/overview` 返回 provider 状态、摘要 stats、时间序列和系统监控页组件行。
  - 系统监控页优先调用 console 后端接口展示“实时数据”，请求失败或无数据时保留 mock 预览。
  - 已通过 console Go 测试、前端静态检查/lint/build、本地 all-mode API 和真实浏览器页面验证。

## [2026-07-19] implement | 系统监控资源看板与 Go runtime 看板拆分

- 更新页面：technical/adr/observability/adr-otel-observability-platform.md, todo。
- 变更摘要：
  - 将系统监控接口明细中的 CPU/MEM 移出，独立为组件资源看板；真实模式下只使用 `overview.resources`，暂无 kubeletstats 表时显示空态。
  - `/observability/v1/platform/overview` 增加 `runtime` 和 `resources` 字段，Go runtime 从 GreptimeDB `process_runtime_go_*` 表查询。
  - `pole-control-plane` Go 看板展示 goroutine、heap alloc/inuse/sys、GC count、GC pause，并预留 Go scheduler 调度延迟查询。
  - 已通过 console Go 测试、前端静态检查/lint/build、本地 all-mode API 和真实浏览器页面验证。

## [2026-07-19] fix | 系统监控 Go runtime 看板布局修正

- 更新页面：todo, lessons。
- 变更摘要：
  - 将系统监控页的 Go runtime 看板调整为整行面板，避免在双列 dashboard grid 中只占左列并留下右侧空白。
  - 将 runtime 指标卡改为更紧凑的桌面网格，并增加前端静态约束防止整行布局回归。
  - 用真实浏览器验证 1280 与 1920 宽度下 runtime 面板宽度等于所在 grid 宽度，截图归档到 `output/playwright/system-monitor-runtime-layout-wide.png`。

## [2026-07-19] fix | 系统监控 runtime 小图 hover 数据补齐

- 更新页面：todo, lessons。
- 变更摘要：
  - 将系统监控页 Go runtime 小图从纯静态 SVG 折线升级为可 hover/focus 的轻量趋势图。
  - hover 数据展示指标名、采样点序号和值，并按 runtime 指标 unit 进行格式化。
  - 用真实浏览器 hover `Heap Alloc` 小图验证 tooltip 显示 `Heap Alloc / 采样点 6 / 23.28MiB`，截图归档到 `output/playwright/system-monitor-runtime-hover-tooltip.png`。

## [2026-07-19] refine | 系统监控 Grafana-like 看板视图

- 更新页面：todo。
- 变更摘要：
  - 将系统监控页调整为 dashboard header、variables、stat panels、query panels 的 Grafana-like 信息结构。
  - 顶部展示 `Last 1 hour`、`Step 1m`、`GreptimeDB` 和实时数据状态；variables 区展示 `$category`、`$api`、`$component`。
  - 每个 panel 标题栏展示 query 元信息，主接口延迟趋势补齐 hover tooltip，可查看采样时间和值。
  - 用真实浏览器验证新版 `/metrics/system` 视图和主趋势图 hover，截图归档到 `output/playwright/system-monitor-grafana-dashboard.png`。

## [2026-07-19] fix | 系统监控图表坐标轴补齐

- 更新页面：todo, lessons。
- 变更摘要：
  - 为接口延迟趋势补充 Y 轴刻度和底部时间线。
  - 为延迟热力图补充顶部时间轴和左侧接口/组件维度。
  - 为每个 Go runtime 小图补充简化 Y 轴刻度和底部时间线。
  - 用真实浏览器验证新版 `/metrics/system` 图表坐标轴，截图归档到 `output/playwright/system-monitor-dashboard-axes.png`。

## [2026-07-19] refine | 系统监控 runtime 对齐 Grafana Stat panel

- 更新页面：todo, lessons。
- 变更摘要：
  - 按 Grafana 官方口径区分 Time series 与 Stat panel：主趋势图保留完整 x/y 轴，runtime 单值指标使用 Stat + sparkline。
  - 移除 runtime 小卡片内显眼坐标刻度和大气泡标签，改成弱化 sparkline、底部时间范围和紧凑 hover tooltip。
  - 用真实浏览器验证 `GC Pause Avg` hover 显示 `GC Pause Avg / 12:25 / 1.88ms`，截图归档到 `output/playwright/system-monitor-grafana-stat-sparkline-hover.png`。

## [2026-07-19] refine | 系统监控 variables 顶部变量行优化

- 更新页面：todo, lessons。
- 变更摘要：
  - 将系统监控页 variables 区从带阴影的大卡片收敛为 dashboard 顶部轻量变量行。
  - 去掉 `$category`、`$api`、`$component` 的厚重前缀盒，改为变量 Label + 变量名小标签。
  - 保留变量顺序、筛选联动和重置动作，并补充静态约束防止视觉结构回退。

## [2026-07-19] fix | 系统监控 runtime Stat sparkline 比例修正

- 更新页面：todo, lessons。
- 变更摘要：
  - 扩大 Go runtime Stat 卡片内 sparkline 的逻辑画布和可视高度，避免趋势线缩成短横线。
  - 增加 runtime 卡片高度，让当前值与趋势图形成更均衡的上下结构。
  - 保留 Stat panel、时间范围和 hover tooltip，不回退为完整坐标轴小图。

## [2026-07-19] refine | 系统监控 runtime Stat 对齐 Grafana 风格

- 更新页面：todo, lessons。
- 变更摘要：
  - 将 Go runtime Stat 卡片收敛为 Grafana panel 风格：薄边框、低圆角、平面背景和紧凑标题。
  - 将 sparkline 调整为下半区 area sparkline，保留当前值作为主视觉。
  - 弱化 runtime 分类标签和说明文案，减少 Fluent 卡片装饰感。

## [2026-07-19] fix | 系统监控时间轴按本地时区展示

- 更新页面：todo, lessons。
- 变更摘要：
  - 移除系统监控页固定 `TIME_LABELS`，按当前查询窗口动态生成时间标签。
  - 图表、热力图和 runtime sparkline 的时间轴与 tooltip 统一使用浏览器本地时区格式化。
  - Dashboard header 增加本地时区标识，例如 `UTC+08:00`，避免误解为 UTC 展示。

## [2026-07-19] feature | 操作审计与服务事件接入 OTel logs

- 更新页面：todo, lessons。
- 变更摘要：
  - 在 `history` 与 `discoverEvent` chain 增加 `otel` entry，将操作审计和服务事件映射为 OTel logs 并写入 Collector logs pipeline。
  - 新增 fail-open logs exporter：业务链路只入本地 bounded queue / bbolt spool，Collector 异常不阻塞接口，也不影响旧 `logger/rds` entry；Collector 恢复后补发未确认记录；默认 history/discover-event 共享一个 bbolt 文件并用 bucket 隔离。
  - Console 新增 `/observability/v1/events` 与 `/observability/v1/operations`，从 GreptimeDB `pole_events` 查询结构化 event/audit，并让页面优先读取新接口、旧 `/metrics/v1` 兜底。
  - 已通过 Go/前端测试、本地 Collector/GreptimeDB smoke 和 all-mode 真实接口验证。

## [2026-07-19] ingest | Pebble 本地 protobuf value cache 设计

- 新增页面：adr-local-pebble-protobuf-value-cache。
- 更新页面：cache-layer, index, todo, lessons。
- 变更摘要：
  - 确认 Pebble 用于存放可重建的 protobuf marshal bytes，优先覆盖实例列表、配置文件、服务契约和治理规则发布快照。
  - 明确 MySQL、注册内存态和治理 cache 仍是事实来源；实例心跳继续留在内存热路径。
  - 明确 key 必须包含 revision、release_id 或过滤/权限可见性 hash，避免不同请求上下文误命中同一 value。

## [2026-07-19] refine | Pebble value cache 开关与性能硬约束

- 更新页面：adr-local-pebble-protobuf-value-cache, cache-layer, todo, lessons。
- 变更摘要：
  - 明确本地 Pebble value cache 必须默认关闭，开关启用后才使用“内存索引 + Pebble value”模式卸载客户端大数据。
  - 将适用范围扩大为所有面向客户端返回、可重建且 value 较大的数据，包括服务发现、配置、服务契约和治理下发。
  - 将“开启后性能不能低于当前路径”写入验收标准，并要求补齐服务发现、配置发现、服务契约和治理下发基准。

## [2026-07-19] implement | 配置契约与 OTel event 队列切换 Pebble

- 更新页面：adr-otel-observability-platform, adr-local-pebble-protobuf-value-cache, cache-layer, todo, lessons。
- 变更摘要：
  - 配置文件 active release content 与服务契约本地 value cache 从 bbolt 切换为 Pebble，内存继续只保留轻量索引。
  - OTel history/discoverEvent 本地可靠队列从 bbolt spool 切换为 Pebble queue，按 key prefix 区分 audit 与 service event。
  - 保持 Collector 异常时 fail-open：业务链路只写本地 queue，导出失败保留待重试，导出成功后删除已确认 key。

## [2026-07-19] cleanup | 移除测试套件 boltdb 残留

- 更新页面：overview, todo, lessons。
- 变更摘要：
  - 将 `test/data/service_test.yaml` 的默认 store 从旧 `boltdbStore` 切换为 `defaultStore`。
  - 删除 `test/suit` 中 `bolt-data.yaml` 注入、`polaris.bolt` 清理和 bbolt helper。
  - 删除已无引用的 `test/data/bolt-data.yaml` fixture，并清理 bbolt 依赖与 checksum。

## [2026-07-19] decision | 服务调用鉴权采用托管服务身份与自定义 Header 双模式

- 新增页面：adr-managed-service-identity-authentication。
- 更新页面：auth-system, index, todo, lessons。
- 变更摘要：
  - 将服务调用鉴权从管理面用户 Token/资源授权中拆开，定义为数据面服务间信任。
  - 默认由 control-plane 管理不可见、不可修改的服务身份，并向合法 workload 签发可轮换短期凭证；被调方入站 SDK/sidecar 本地验证，control-plane 不进入业务请求热路径。
  - 保留用户自定义 Header/value 作为显式兼容模式，并要求密文/摘要存储、脱敏读取和精确匹配。

## [2026-07-19] refine | 明确服务身份领取与 Discover 边界

- 更新页面：adr-managed-service-identity-authentication, todo。
- 变更摘要：
  - 明确 service token 只用于 SDK 与 control-plane 认证，ServiceIdentity 用于服务间数据面身份，两者不能互相替代。
  - 新增 `SERVICE_IDENTITY` Discover 方向：token 校验通过后下发稳定身份描述，允许按服务 revision 缓存。
  - 实例级 workload credential 仍通过独立 Issue/Renew RPC 定向返回，禁止进入普通 Discover/failover cache。
  - 身份 Discover 强制使用 gRPC metadata token、服务端 principal 推导和 fail-closed 校验；不信任请求体自报服务，也不复用含 token 的 Service message。

## [2026-07-20] implement | 托管服务身份第一阶段闭环

- 更新页面：adr-managed-service-identity-authentication, todo。
- 变更摘要：
  - specification 增加 managed/custom 认证模式、caller selector 与 `SERVICE_IDENTITY` Discover descriptor。
  - control-plane 为真实服务创建隐藏身份，历史服务按 metadata service token 惰性补齐，并拒绝 body token 覆盖、重复 token 和服务声明错配。
  - Rust SDK 使用独立 `controlPlaneToken` 建立身份订阅，descriptor 仅保存在 Engine 内部；Console 默认展示托管身份并明确凭证闭环仍未完成。
  - Custom Header 写入值在持久化前转为 SHA-256 摘要并清空明文，管理读取、预览和 Discover 不返回原值。

## [2026-07-20] implement | WorkloadCredential 数据面凭证闭环

- 更新页面：adr-managed-service-identity-authentication, index, todo。
- 变更摘要：
  - specification 增加 Ed25519 JWT Issue/Renew、SERVICE_TOKEN binding 与 Trust Bundle Discover 契约。
  - control-plane 通过 metadata service token 推导主体并签发短期凭证，私钥仅从文件引用加载，支持 ACTIVE/VERIFY_ONLY 滚动轮换。
  - Rust SDK 在内存中领取和续期凭证，提供 HTTP/tonic 显式注入与入站验签，并仅从验签结果生成 `AuthenticatedCaller`。
  - 托管身份启用时 SDK 强制使用 grpcs，control-plane 身份入口拒绝非 TLS transport；同时补齐 descriptor 周期刷新与同版本 bundle 安全续租，保证密钥轮换和长期运行不中断。
  - 明确 V1 bearer 的重放风险、服务级而非实例级绑定，以及 SDK 不拥有业务网络栈时必须由业务显式挂载适配器。

## [2026-07-20] refine | 托管身份 TLS 改为按需启用

- 更新页面：adr-managed-service-identity-authentication, todo, lessons。
- 变更摘要：
  - Rust SDK 不再要求启用托管身份时必须使用 `grpcs`，继续同时支持 `grpc` 与 `grpcs` endpoint。
  - control-plane 不再拒绝身份 Discover、Issue/Renew 的非 TLS transport，请求是否启用 TLS 由部署方决定。
  - 文档保留生产环境启用 TLS 的安全建议，并明确明文 bearer 的链路窃听风险，但不把建议升级为功能前置条件。
## [2026-07-20] implement | 命名空间工作台滚动与配置文件统计

- 更新页面：namespace、todo、lessons。
- 变更摘要：
  - 命名空间列表的配置文件数改为后端批量聚合的真实文件数量，并通过协议字段返回到摘要与行级表格。
  - 固定页头、指标、工具栏和分页，仅让表格内容区滚动；浏览器验收确认滚轮不会再驱动页面滚动。
## [2026-07-20] fix | 配置分组删除保护与失败通知

- 更新页面：todo、lessons。
- 变更摘要：
  - 修复 Console 遗漏 `file_count` 的响应归一化，配置文件占用的分组不再显示为零文件或错误开放删除。
  - 后端继续拒绝删除含有效配置或活跃发布的分组；Toast 将 RequestId 从正文收纳为 Fluent 复制操作。

## [2026-07-20] refine | Toast 自动与手动关闭

- 更新页面：todo、lessons。
- 变更摘要：
  - Toast 统一使用独立 ID 和 Fluent 关闭按钮，错误与成功通知均在 10 秒后自动关闭。
  - 浏览器验证覆盖错误通知手动关闭与超时自动移除，以及成功通知关闭入口。

## [2026-07-20] refine | 紧凑错误 Toast 复制动作

- 更新页面：todo、lessons。
- 变更摘要：
  - 将底部 Request ID / 错误 JSON 双复制按钮合并为标题右上角单一“复制”按钮，仍复制完整 JSON。
  - 复制动作保留图标、短文案与可访问标签，关闭按钮独立保留。

## [2026-07-20] refactor | Console 治理鉴权凭证交互收尾

- 更新页面：todo。
- 变更摘要：
  - 将 Custom Header 明文从规则 state 拆为一次性 write-only draft，并在加载、关闭、撤销、模式切换和保存后清空。
  - Console 加载详情时主动丢弃服务端摘要，提交时按认证模式重新构造最小 authentication 载荷，避免摘要或旧凭证回写。
  - 已保存的兼容凭证进入编辑态时明确提示必须重新输入才能轮换，详情和实时 Spec 始终脱敏。
  - 使用真实 Console 完成托管默认值、模式切换、首次保存、轮换、回显和测试数据清理验收。

## [2026-07-20] fix | 错误 Toast 正文宽度

- 更新页面：todo、lessons。
- 变更摘要：
  - 将 Fluent Toaster 容器设为受视口约束的 420px 桌面宽度，避免 Toast 子项溢出容器。
  - 错误正文跨越 Fluent 默认网格的右侧空列；真实 400203 通知已确认中文信息不再过早换行，复制与关闭操作完整可见。

## [2026-07-20] refactor | 治理规则编辑器移除实时 Spec

- 更新页面：governance-rules、patterns、todo、lessons。
- 变更摘要：
  - 路由、泳道、限流、熔断、探测、无损、调用鉴权、流量镜像与 Mock 统一不展示动态 Spec、YAML/JSON 预览。
  - 移除调用鉴权实际 Spec 面板及全套编辑器遗留预览样式，新增静态门禁防止预览列回归。
  - 治理编辑器长期约定改为单栏表单和内部滚动，校验由字段校验、保存和错误提示承担。

## [2026-07-20] refine | 治理鉴权规则编辑布局与来源服务选择

- 更新页面：governance-rules、patterns、todo、lessons。
- 变更摘要：
  - 受保护接口行按真实内容区收缩，路径列弹性扩展，操作列不再溢出。
  - Fluent Select 的可搜索模式会实际过滤候选项；来源服务多选器限定为紧凑宽度。
  - 编辑器 shell 使用自然高度，长规则由详情 Tab 内容区承担滚动。

## [2026-07-20] refine | 治理规则类型即选即建

- 更新页面：governance-rules、todo、lessons。
- 变更摘要：
  - 新建规则类型卡片改为直接创建入口，移除步骤条和“进入创建”二次确认。
  - 每张卡片保留明确的可访问名称，鼠标与键盘均可直接打开对应规则编辑器。

## [2026-07-20] refactor | 治理规则独立创建页

- 更新页面：governance-rules、todo、lessons。
- 变更摘要：
  - 新建规则由工作台抽屉改为独立创建路由，并复用规则详情页面框架。
  - 类型卡片直接导航；服务详情上下文通过 URL 传递给创建表单，保存或取消后回到治理工作台。

## [2026-07-20] fix | 治理独立创建页滚动与遗留浮动控件

- 更新页面：governance-rules、todo、lessons。
- 变更摘要：
  - 创建页建立有限高度 flex 链，仅由表单内容区处理长内容滚动，避免滚动落到外层工作区。
  - 页头已接管保存/撤销操作时，隐藏遗留 `StickyTool` 固定容器，消除右侧空白方块。
  - 真实鉴权创建页验证内容区可用滚动且外层不随滚轮移动；8080/8090 均正常返回。

## [2026-07-20] fix | 鉴权规则 API 路径匹配语义

- 更新页面：governance-rules、todo、lessons。
- 变更摘要：
  - 受保护接口移除“值类型”控件，仅保留协议、方法、路径匹配类型、路径和操作。
  - 鉴权 API 读写适配层清理遗留 `value_type`，参数匹配条件继续保留该字段。
  - 更新专项回归脚本，覆盖页面结构及 API 路径提交载荷。

## [2026-07-20] fix | 治理 API 协议化编辑交互

- 更新页面：governance-rules、todo、lessons。
- 变更摘要：
  - HTTP、gRPC、Dubbo 共用存储结构但使用各自的表单语义：HTTP 方法/URI、gRPC 方法名/服务名、Dubbo 方法名/接口名。
  - 切换协议清空旧 `method/path` 值，HTTP 重新使用 `GET` 默认值，避免将 HTTP 默认条件误保存到 RPC 规则。
  - 鉴权、镜像和 Mock 的 API 编辑行均复用相同协议展示与重置逻辑。

## [2026-07-20] fix | RPC 接口优先编辑与可选方法

- 更新页面：governance-rules、todo、lessons。
- 变更摘要：
  - gRPC/Dubbo 将服务名或接口名作为主匹配对象，置于方法之前；方法明确为可选的细化条件。
  - HTTP 保持方法、匹配类型、路径的既有顺序，混合协议表头明确说明两种字段语义。
  - 专项回归覆盖 gRPC 服务名匹配且方法为空时的校验与提交行为。

## [2026-07-20] fix | 鉴权服务信息垂直可折叠布局

- 更新页面：governance-rules、todo、lessons。
- 变更摘要：
  - 命名空间和服务名称调整为上下顺序，保留紧凑且受限宽度的选择器布局。
  - 服务信息接入统一折叠分段；收起时显示已选择的命名空间和服务名称摘要。
  - 专项回归、构建、重建与浏览器交互验收均已完成。

## [2026-07-20] fix | RPC 接口字段语义与宽度

- 更新页面：governance-rules、todo、lessons。
- 变更摘要：
  - 明确 RPC 的 `path` 为 service/interface、`method` 为可选方法，修正混合表头的歧义。
  - RPC 行为 interface 分配更宽输入列，method 使用较短的可选输入列。
  - 专项回归先验证旧表头失败，再通过构建、重建和 gRPC 实页切换验收。

## [2026-07-20] fix | 鉴权认证方式折叠

- 更新页面：governance-rules、todo、lessons。
- 变更摘要：
  - 认证方式改为统一可折叠分段，默认展开并在关闭或切换规则时复位。
  - 收起状态只显示当前认证模式摘要，不展示 Header 值或其它凭证编辑内容。
- 专项回归、构建、重建和真实浏览器交互验收均已完成。

## [2026-07-20] ingest | Console Agent 资源变更工作台需求调研

- 新增页面：adr-console-agent-resource-workbench。
- 更新页面：ai-features、index、todo。
- 变更摘要：
  - 明确 Agent 工作台与 A2A Agent Registry、pole-server 领域核心的边界。
  - 确认配置文件和治理规则才具备真实的编辑态与发布态分离，其他直接生效资源首期只读。
  - 采用 Console 会话深模块与内部强类型资源 adapter，模型只生成提案，确认只保存草稿，发布仍由用户在现有页面完成。
  - 固化 proposal hash、幂等、stale baseline、二次鉴权、敏感字段和审计要求，并给出分期与验收标准。

## [2026-07-20] fix | 鉴权兼容模式恢复子规则 Header 匹配

- 更新页面：governance-rules、adr-managed-service-identity-authentication、todo、lessons。
- 变更摘要：
  - Console 的“自定义 Header（兼容模式）”统一写入 `LEGACY_REQUEST_MATCH`，不再创建规则级 `CUSTOM_HEADER` 凭证。
  - Header 匹配条件随受保护接口保存在每条 `TrafficSecurityPolicy.traffic_match_rule`，各子规则相互独立。
  - 服务端继续解析历史 `CUSTOM_HEADER`，以维持存量资源读取兼容；Console 编辑保存时回归原有子规则请求匹配模型。

## [2026-07-20] ingest | 治理请求参数采集与动态消费

- 新增页面：adr-governance-request-parameter-capture。
- 更新页面：governance-rules、patterns、index、todo、lessons。
- 变更摘要：
  - 统一 `TEXT/PARAMETER/VARIABLE` 为固定匹配、当前键请求值采集和运行变量引用。
  - Proxyless Rust SDK 的本地限流按采集值隔离计数器，路由目标标签可消费同名采集参数。
  - Console 横向保留值来源并明确 xDS 当前不消费动态语义，避免把可配置误报为全数据面支持。

## [2026-07-20] release | specification ALPHA.34 与依赖联动

- 更新页面：todo。
- 变更摘要：
  - 发布 specification `v0.1.0-ALPHA.34`，并完成 Go/Rust 生成代码与 crates.io 产物验证。
  - control-plane 与 Rust SDK 均切换到 ALPHA.34，以显式暂存隔离各仓库已有的其它未提交改动。
  - 记录 specification、control-plane 与 Rust SDK 的提交、推送和全量测试结果。

## [2026-07-20] refactor | 移除治理运行变量值来源

- 更新页面：governance-rules、adr-governance-request-parameter-capture、patterns、todo、lessons。
- 变更摘要：
  - 治理匹配值来源收敛为固定值和请求参数，不再支持读取运行机器环境变量。
  - specification 保留历史枚举号与名称不可复用；Rust SDK 对未知旧值 fail closed。
  - Console 所有共享治理条件编辑器统一移除运行变量选项和相关提示。

## [2026-07-20] refactor | 治理匹配条件 Fluent UI 交互与容器布局

- 更新页面：todo、lessons。
- 变更摘要：
  - 共享条件编辑器改用 Fluent `RadioGroup`、`Button`、`Tooltip`、`Select` 与主题 token，并补齐语义化表格和可访问名称。
  - 固定值与请求参数采用明确的状态化展示，移除导致选中值聚焦后清空的筛选式枚举下拉。
  - 响应式改用组件容器查询，使嵌套治理子规则按自身可用宽度从六列表格切换为两列或单列字段卡，避免操作列裁切。
  - 已完成亮色、暗色、下拉切换和请求参数采集的真实浏览器验收。

## [2026-07-21] feat | Console Agent 配置草稿纵向闭环

- 更新页面：adr-console-agent-resource-workbench、ai-features、todo。
- 变更摘要：
  - 新增 `/ai/agent` 确定性工作台与 Console proposal API，支持已有配置文件的查询、临时 diff、确认和待发布回执。
  - 确认链路绑定 actor、baseline/preview hash、TTL 与幂等键，只保存配置草稿，完全不调用发布接口。
  - 增加状态机、HTTP adapter、路由、前端契约和真实 smoke 验证；明确底层配置 API 缺少原子 CAS 的剩余并发边界。

## [2026-07-21] fix | Console Agent 页面持久后台恢复

- 更新页面：todo、lessons。
- 变更摘要：
  - 确认连接拒绝源于临时工具会话退出，不是 Agent 路由或页面代码故障。
  - 改用独立 tmux 会话分别守护 8080 Console 与 8090 pole-server。
  - 固化回合结束后的可访问性验收：后台会话、端口、深链和静态资源必须同时有效。

## [2026-07-21] refactor | Pole Agent 独立对话入口

- 更新页面：adr-console-agent-resource-workbench、ai-features、todo、lessons、index。
- 变更摘要：
  - 将 Agent 从“AI 工具”子菜单迁移为独立一级 `/agent` 入口；A2A 与 MCP Registry 继续留在 AI 工具。
  - 用聊天时间线替换资源表单，在消息流中展示工具调用轨迹，并在写操作时按需打开临时 diff 检查器。
  - 保持 previewHash、幂等、stale 检查和发布隔离；确认后只保存草稿并停在待发布。
  - 明确 Phase 0 仍是确定性 planner + HTTP tool port，真实 ModelPort 与 MCP client transport 是下一阶段硬缺口。

## [2026-07-21] feat | 普通控制台与 Agent 双工作模式

- 更新页面：adr-console-agent-resource-workbench、ai-features、todo、lessons。
- 变更摘要：
  - 默认侧边布局在侧栏底部、版本信息上方新增工作区切换；普通模式进入 Agent，Agent 模式在同一位置返回普通控制台。
  - Agent 模式隐藏普通资源导航但保留侧栏骨架和返回入口；无侧栏的顶部布局保留页头切换兜底。
  - 切换 Agent 前记录最近普通页面；返回时使用安全 `returnTo`，并对外部或循环路径 fail closed。
  - 配置文件详情新增“交给 Agent”，只传资源自然键和返回地址；Agent 首屏接入上下文并生成可继续编辑的对话起点。
  - 保持登录态、权限、API、审计和正式发布链路共享，不复制第二套资源管理实现。

## [2026-07-21] ingest | Console Agent Runtime 与 LLM Gateway 接缝

- 更新页面：adr-console-agent-resource-workbench、ai-features、todo。
- 变更摘要：
  - 确认当前 Phase 0 是前端正则 planner + Console 强类型 REST adapter，不包含真实 LLM 或 MCP client。
  - 明确 LLM Gateway 地址、模型和凭证属于 Console 后端配置，浏览器不得直接持有密钥。
  - 将 Console Agent Runtime 分为 ModelPort、ToolExecutor、现有确认内核和 SessionRepository 四个内部 seam。
  - 记录 Pole MCP 当前缺少配置/治理资源工具，Phase 1 需先补工具能力再宣称 MCP 已连接。

## [2026-07-21] refactor | Pole Agent 一等运行主体

- 更新页面：adr-console-agent-resource-workbench、ai-features、todo、lessons。
- 变更摘要：
  - 根据用户纠正，将 Pole Agent 明确为页面唯一对话对象，而不是 ModelPort、MCP 与 Workbench 的松散组合。
  - Pole Agent 内部拥有版本化 System Prompt、LLM、MCP 工具导入、model-tool loop、会话和 approval resume。
  - 现有 Workbench 重新定位为 Agent 内部的 ChangeApprovalKernel，页面不再承担意图解析、Prompt 拼装或工具选择。

## [2026-07-21] ingest | Console Agent 系统配置来源

- 更新页面：adr-console-agent-resource-workbench、terminology、todo、index。
- 变更摘要：
  - 采用启动 YAML/Secret 自举、Pole 配置中心保存非敏感 AgentDefinition、Console 类型化系统设置页面的三层方案。
  - 配置中心只发布非敏感动态设置，模型密钥和后台凭证只保存 Secret 引用。
  - Agent 对已发布 revision 做完整校验和原子热加载，失败时保留 last-known-good，现有会话固定原 revision。
  - 拒绝 Console 直连核心数据库，避免破坏 console-only 模式并重复配置中心的发布、历史、Watch 和审计能力。

## [2026-07-21] ingest | Server 与 Console 统一系统配置

- 新增页面：adr-system-configuration-control-plane。
- 更新页面：configuration、adr-console-agent-resource-workbench、terminology、todo、index。
- 变更摘要：
  - 将 Agent 配置方案提升为 Pole Server 与 Console 共用的 System Configuration 领域。
  - 静态 YAML 保留自举和安全基线，配置中心承载已发布动态覆盖，Secret Provider 管理密钥。
  - 配置按 BootstrapOnly、RestartRequired、HotReload、GuardedHotReload 分级，没有 applier 的字段禁止热更新。
  - 引入 DesiredSnapshot、EffectiveSnapshot 和 ApplyReceipt，页面可观察实例生效、拒绝、待重启和漂移。
  - 给出只读目录、Console 低风险热更、Server 热更、重启配置与高级治理的分阶段路径。

## [2026-07-21] feat | Pole 本地 Kubernetes 一体化部署

- 更新页面：adr-otel-observability-platform、todo。
- 变更摘要：
  - 新增 `pole-system` namespace 下 Pole Deployment、Collector Deployment、GreptimeDB StatefulSet/PVC 的一体化编排。
  - 使用 ExternalName Service 继续访问宿主机 MySQL `3306`，凭证通过 Secret 注入，不把 MySQL 迁入集群。
  - 修复本地 all-mode 镜像内容，包含 Console 静态资源、运行配置和 Kubernetes 配置渲染入口。
  - OrbStack 实测三个工作负载 Ready，Console/Agent 页面可访问，OTLP event 经 Collector 写入 GreptimeDB 并由 SQL 查回。

## [2026-07-21] feat | Pole Console 接入共享 Gateway

- 更新页面：adr-otel-observability-platform、todo。
- 变更摘要：
  - 复用 `tidemind/tidemind-gateway`，为 Console 增加无需修改 hosts 的 `pole.localhost` 域名。
  - HTTPRoute 留在 Gateway namespace，通过最小 ReferenceGrant 跨 namespace 引用 `pole-control-plane:8080`。
  - GreptimeDB、OTel Collector 与宿主机 MySQL 继续保持内部访问边界。

## [2026-07-22] refactor | Console 登录入口视觉收敛

- 更新页面：adr-console-fluent-ui-design-system、todo、lessons。
- 变更摘要：
  - 将超大宣传标题与悬空表单重构为克制的产品说明区和聚焦登录面板。
  - 移除默认账号明文和没有后端 API 的伪注册入口，强化主按钮、输入焦点、密码可见性和提交状态。
  - 管理员检查异常不再误跳初始化页；保留认证 API 和登录后跳转，并补齐桌面与窄视口真实浏览器验收。

## [2026-07-22] feat | System Configuration Phase 1 只读目录

- 更新页面：adr-system-configuration-control-plane、todo、lessons、index。
- 变更摘要：
  - 新增 `pkg/systemconfig` 定义注册表、有效快照和 default/YAML/env/CLI 来源索引。
  - Pole Server 与 Console 分别注册首批核心设置，Console 通过独立只读接口跨进程聚合 52 项配置。
  - Secret 在服务端清空值；Server 内部快照接口执行独立 `DescribeSystemConfiguration` 策略授权，Console 页面不提供任何编辑或发布动作。
  - 新增独立“系统配置”侧栏入口、来源概览、搜索和组件/子域/来源筛选，并部署到 `pole.localhost`。

## [2026-07-22] refactor | System Configuration 按领域组织

- 更新页面：adr-system-configuration-control-plane、todo、lessons。
- 变更摘要：
  - 将 52 项配置的单一总表改为“组件 → 领域 → 配置项”三级信息架构。
  - Pole Server 与 Console 分别提供稳定的领域顺序，主表只展示当前领域并移除重复的组件/领域列。
  - 跨领域搜索保留在当前组件范围内，领域导航显示命中数并自动将命中领域滚入视野。
  - 完成桌面与 390px 真实页面验收并部署到 Kubernetes；领域导航和配置表在窄屏各自独立滚动。

## [2026-07-22] feat | Console AgentDefinition 配置目录

- 更新页面：adr-console-agent-resource-workbench、adr-system-configuration-control-plane、todo、lessons、index。
- 变更摘要：
  - Console Agent 领域由 2 项扩展为 13 项，补齐 Agent ID、Prompt、LLM Gateway/model/API key 引用、MCP endpoint/工具白名单和运行限制。
  - API key 仅由环境变量或 Kubernetes Secret 注入，系统配置响应删除原值并只返回配置状态与引用。
  - 现有确定性 Workbench 使用配置化 proposal TTL 和上游 timeout，同时明确真实 ModelPort 尚未接通。
  - 修正列表型设置的来源聚合，完成 Go、前端、Kubernetes 与真实登录页面验收。

## [2026-07-22] decision | Pole 托管 Agent Secret

- 更新页面：adr-system-configuration-control-plane、adr-console-agent-resource-workbench、todo、lessons、index。
- 变更摘要：
  - 纠正将 Kubernetes Secret/环境变量作为 LLM API key 最终入口的设计，明确 Pole 默认托管业务 Secret。
  - 定义 `SystemSecretStore` write-only interface、envelope encryption、版本化 `pole-secret://` 引用和运行时受控解析。
  - Kubernetes Secret/环境变量只保存 KEK 等自举材料，Vault 等保留为可选外部 adapter。
  - 现有配置文件加密链不满足系统 Secret 的密钥隔离、轮换和用途约束，不能直接复用。

## [2026-07-22] design | System Configuration 编辑交互与 Admin 门禁

- 更新页面：adr-system-configuration-control-plane、todo、lessons。
- 变更摘要：
  - 编辑采用单字段右侧工作面板，保存只产生领域草稿，审阅和发布保持独立动作。
  - Secret 采用 write-only 输入和版本轮换语义，diff 只显示“将轮换”，不回显新旧值。
  - System Configuration 首期只允许 `main`/admin；前端隐藏入口并守卫直达，后端统一保护全部 `/system-config/v1/*` 接口。
  - Console 签名 JWT 增加角色 claim 并设为 HttpOnly，旧会话仅在用户 ID 匹配 canonical main account 时兼容。
## [2026-07-22] feat | 三个内置系统角色与成员绑定

- 更新页面：terminology、domain-models、business-rules、auth-system、adr-system-configuration-control-plane、todo、lessons、index。
- 变更摘要：
  - 固定 `admin`、`resource-reader`、`resource-writer` 三个系统角色及稳定权限边界。
  - 启动和角色查询时幂等补齐角色与固定策略；服务端禁止角色定义和系统策略的普通增删改。
  - Console 角色页当时移除通用 CRUD，只保留用户与用户组成员绑定；后续由下方纠正记录恢复自定义角色 CRUD。
  - 内置 admin 成员签发 `admin` 会话角色，可与主账号 `main` 一同访问 admin-only 系统配置页。

## [2026-07-22] fix | 恢复自定义角色并收敛内置角色保护边界

- 影响页面：[[terminology]]、[[domain-models]]、[[business-rules]]、[[auth-system]]
- 变更摘要：
  - 角色聚合恢复自定义角色完整 CRUD、成员及权限管理。
  - 三个内置角色保持定义与资源/API 权限不可变，只开放用户和用户组绑定。

## [2026-07-22] fix | 系统配置请求失败不再伪装为空目录

- 更新页面：adr-system-configuration-control-plane、todo、lessons。
- 变更摘要：
  - 修复系统配置请求失败时清空已有设置、将错误渲染成 `0 / 0` 和全零统计的问题。
  - 首次加载失败改为独立错误态和重试入口；刷新失败保留 last-known-good 快照并显示告警。
  - 增加前端错误态契约，完成 Console/配置注册相关测试、Kubernetes 发布和真实 admin 页面复验。

## [2026-07-22] fix | 非 Admin 完全隐藏系统配置页面

- 更新页面：adr-system-configuration-control-plane、todo、lessons。
- 变更摘要：
  - 新增 `/auth/v1/user/session`，由 Console HttpOnly 签名会话返回权威角色和 admin 判定。
  - 前端不再从 localStorage 恢复角色；会话未解析时 fail-closed，非 admin 不显示菜单且直达系统配置页重定向到普通控制台。
  - 本地 Kubernetes 作为唯一运行验收环境；修复一次误将 macOS Mach-O 放入 Linux/arm64 镜像导致的 CrashLoop，并以新 Pod Ready 为发布门槛。
  - K8s 实测 main 访问系统配置为 200、临时 sub 为 403；临时用户验收后已删除。

## [2026-07-23] design | Namespace 统一为运行环境

- 更新页面：terminology、domain-models、business-rules、namespace、service-discovery、config-center、governance-rules、overview、todo、lessons、index。
- 变更摘要：
  - 明确 Namespace 是 Pole 的运行环境边界，不再定义为泛化的多租户容器。
  - 服务名是全局逻辑标识，同名服务在不同 Namespace 中表示同一服务的环境实例。
  - 配置分组、配置文件和治理规则建立一致的跨环境逻辑身份与环境实例坐标。
  - Console 命名空间、服务、配置和治理入口说明同步采用环境语义。
  - 本地 OrbStack 仅更新 `pole-system/pole-control-plane`，部署镜像为 `pole-control-plane:local-20260723-namespace-environment-copy-v2`，运行入口与新静态资源完成验证。

## [2026-07-23] feat | Namespace 环境模型全链路实现

- 更新页面：adr-governance-rule-unified-storage、todo、lessons、index。
- 变更摘要：
  - specification 九类治理聚合根增加顶层 namespace，明确区分规则归属环境与 caller、callee、target 等运行时作用域。
  - 统一治理存储的名称查询、锁、发布与删除保护按 owner namespace 隔离，并增加治理规则占用业务码。
  - Service、Config Group、Config File 详情增加同一逻辑资源的跨环境摘要与切换，服务端逐条过滤无权环境。
  - Console 规则工作台按环境查询和创建；配置文件跨环境查询仅返回摘要，不泄露正文。

## [2026-07-23] fix | Console 暗色主题全站一致性

- 更新页面：adr-console-fluent-ui-design-system、todo、lessons。
- 变更摘要：
  - 修复服务详情局部浅色变量覆盖全局暗色主题，并将治理、AI、认证、监控等页面的浅色背景和边框统一到双主题语义 token。
  - 暗色静态门禁扩展到背景、边框与局部主题变量；前端检查、ESLint 和 production build 均在 Kubernetes Linux/arm64 Pod 内执行。
  - 仅发布 `pole-control-plane` 新镜像，并通过 Pod、Gateway 和 20 个真实页面路由完成暗色表面及控制台回归。

## [2026-07-23] feat | Pole Agent 本地会话与会话记忆

- 更新页面：adr-console-agent-resource-workbench、todo、lessons。
- 变更摘要：
  - Agent 工作台采用 ChatGPT 式本地会话导航、连续消息流、浮动输入框和按需变更检查器。
  - 浏览器 IndexedDB 持久化会话、消息、未发送草稿、资源上下文、当前会话和每会话记忆设置，不作为服务端提案或审计事实来源。
  - 记忆支持关闭及 4/10/20 轮窗口，当前指令优先，历史只补全缺失上下文；preview、确认保存草稿和用户发布边界保持不变。
  - K8s Pod 内完成契约、暗色、ESLint 和生产构建，仅滚动更新 control-plane，并通过 Gateway 真实浏览器验证切换与刷新恢复。

## [2026-07-23] refactor | Pole Agent 单侧栏会话工作台

- 更新页面：adr-console-agent-resource-workbench、todo、lessons。
- 变更摘要：
  - Agent 模式复用最左侧产品导航直接承载本地会话，不再在内容区嵌套第二条会话栏。
  - 新建、切换、重命名和删除统一留在侧栏；主画布移除重复新会话入口和空态建议按钮。
  - 输入区去除双描边，同时保留 IndexedDB 会话记忆、MCP 工具轨迹、临时视图和等待用户发布边界。
  - K8s 容器检查与 Gateway 真实浏览器验证通过，并仅滚动更新 control-plane。

## [2026-07-23] refactor | Pole Agent 参考稿布局对齐

- 更新页面：adr-console-agent-resource-workbench、ai-features、todo、lessons、index。
- 变更摘要：
  - Agent 工作台按指定 HTML 参考稿收敛为 280px 会话侧栏、52px 全局栏、62px 对话栏、底部浮动输入器与 294px 可收起上下文面板。
  - 本地会话增加搜索、日期分组和快捷新建；资源上下文、连接状态、记忆设置与权限边界集中到右侧面板。
  - 临时 diff 改为消息流内工具结果，继续强制预览、确认保存草稿、用户另行发布的安全边界。
  - K8s 容器检查、Gateway 探测和真实浏览器验收通过，仅滚动发布 `pole-control-plane:local-20260723-agent-reference-layout-v1`。

## [2026-07-23] feat | Pole Agent 真实 LLM 与 MCP 最小闭环

- 更新页面：adr-console-agent-resource-workbench、ai-features、configuration、todo、lessons、index。
- 变更摘要：
  - Console 内部新增 OpenAI-compatible ModelPort、版本化 System Prompt、有上限的 model-tool loop 和 actor-bound Pole MCP client。
  - Pole MCP 增加配置文件只读工具；配置 update 仅生成受控临时视图，模型不能确认、发布或删除资源。
  - Agent 页面改为真实会话 API，保留 IndexedDB 本地会话与记忆；运行时未配置时 fail closed，不显示虚假连接。
  - K8s 容器内完成测试、构建、模型/MCP/提案/确认浏览器闭环，并发布 `pole-control-plane:local-20260723-agent-runtime-v5`；临时模型服务和测试改动已清理。

## [2026-07-23] feat | Pole 内部 Agent 配置与 Secret 闭环

- 更新页面：adr-system-configuration-control-plane、ai-features、configuration、todo、lessons、index。
- 变更摘要：
  - Console Agent 的 Gateway、模型、Prompt、MCP 策略与 API key 迁入 Pole 内部 System Settings repository，不再由 Kubernetes 日常环境变量托管。
  - 新增类型化草稿、不可变发布版本、Pole Secret 信封加密、连接测试、发布前强制探活、单实例原子切换和周期 reconcile。
  - 修复普通控制面请求刷新会话时丢失 admin 角色的问题，并兼容升级旧的无 role 主账号 Cookie。
  - K8s 仅保留数据库连接与 `POLE_SYSTEM_SECRET_MASTER_KEY`，所有测试、构建、部署和真实浏览器验收均在容器运行时完成。

## [2026-07-23] refactor | System Configuration Agent 专用工作区

- 更新页面：adr-system-configuration-control-plane、todo、lessons、index。
- 变更摘要：
  - Agent 领域不再复用通用配置表格，改为运行就绪状态、模型连接、Pole Secret、MCP 能力、指令和发布流程组成的专用工作区。
  - 编辑抽屉按模型与凭证、MCP 工具、Agent 指令分组；任意配置变化会使连接测试结果失效，保存草稿和发布保持独立动作。
  - 发布审阅展示 effective 到 draft 的字段级差异、Secret 版本引用和原子热更新保护；移除会生成不可发布草稿的“禁用 API Key”入口。
  - Kubernetes 容器内通过目标 ESLint、Admin gate、错误态、暗色约束和生产构建；真实 Gateway 在 1280、1180、900 三档视口无横向溢出。

## [2026-07-23] feat | System Configuration 全领域字段级编辑

- 更新页面：adr-system-configuration-control-plane、todo、lessons、index。
- 变更摘要：
  - 对 Pole Server 29 项和 Console 34 项逐字段登记编辑性、锁定原因、类型、范围、枚举、敏感策略和生效方式；缺少策略的新字段默认锁定。
  - 非 Agent 领域复用统一 repository 完成草稿、并发控制、发布、历史与 desired/effective 差异；重启级发布明确回执 `pending_restart`。
  - 通用页面增加类型化编辑抽屉、字段差异和发布影响；Agent 补齐提案 TTL、资源工具超时并接入真实运行时更新。
  - Kubernetes 内通过 Go 测试、Vite release build、Pod/Gateway 与真实浏览器验收，发布镜像 `pole-control-plane:local-20260723-system-config-all-v3`。

## [2026-07-24] refactor | Pole 复用 tidemind GreptimeDB

- 更新页面：adr-otel-observability-platform、todo、lessons、index。
- 变更摘要：
  - 移除 `pole-system` 重复的 GreptimeDB standalone 工作负载，通过 `pole-greptimedb` ExternalName 适配 `tidemind/maas-greptimedb-frontend`。
  - Collector 三类 signal 与 Console 查询统一使用 `pole_observability` 逻辑库，保持 Pole 与 MaaS 数据模型隔离。
  - 部署脚本幂等建库，并在新链路 rollout 成功后删除旧 StatefulSet、保留旧 PVC 作为历史数据导出和回退边界。

## [2026-07-25] ingest | 最新 Console 质量与本地部署知识

- 新增页面：console-ui-quality-gates。
- 更新页面：testing、adr-console-fluent-ui-design-system、configuration、todo、index。
- 变更摘要：
  - 将近期全站体验整改提炼为共享表格、表单重置、响应式弹层、可访问性和多视口发布验收契约。
  - 区分接口 E2E、Node 源码契约、构建回归与 Kubernetes 真实浏览器证据，避免用单层检查替代完整 UI 验收。
  - 固化本地 Kubernetes 对共享 GreptimeDB、外部 MySQL、逻辑库隔离、旧 PVC 保留和依赖恢复的运行边界。

## [2026-07-25] refine | Handoff 本地协作资产规范

- 更新页面：todo、lessons。
- 变更摘要：
  - Handoff 统一保存在项目 `.handoff/`，不再写入系统临时目录。
  - 使用 `.git/info/exclude` 做本地排除，禁止 handoff 文件进入暂存、提交或推送。
  - 交付前同时检查 ignore 来源与完整 Git 状态，避免被默认隐藏的未跟踪文件误入版本控制。

## [2026-07-25] refactor | Console 复合查询交互统一

- 更新页面：adr-console-fluent-ui-design-system、todo、lessons、index。
- 变更摘要：
  - 新增应用级 `QueryComposer`，统一主搜索、自动补全、条件 Tag、高级筛选浮层，以及远程显式查询和本地即时过滤两种模式。
  - 命名空间、服务、配置分组、治理工作台、MCP、A2A、系统配置与观测多条件页完成迁移；单字段和详情局部搜索继续保持轻量。
  - 事件指标与操作审计继续使用独立 DateRangePicker，不把时间范围放入高级条件或 Tag。
  - 新增专项契约并通过 ESLint、观测/日期/暗色回归、test build 和 1440/720 浏览器交互验收。

## [2026-07-26] ingest | 多资源域统一治理平台技术方案

- 新增页面：adr-multi-resource-governance-platform。
- 更新页面：terminology、domain-models、governance-rules、todo、lessons、index。
- 变更摘要：
  - 将服务、消息、存储和任务定义为资源域，将路由、限流、运行时鉴权、镜像、Mock 等定义为治理能力。
  - 采用公共规则信封、单效果类型化 Spec、Policy Bundle、能力协商、编译 Adapter、原子 Bundle 和应用回执。
  - 明确 Kafka/RocketMQ、Redis/MySQL、Job 的资源模型、执行点、安全不变量和 fail-open/fail-closed 边界。
  - 给出兼容现有九类服务规则的 Phase 0—5 演进路线、契约测试和完整纳管验收标准。

## [2026-07-26] feat | Pole 自身能力自动注册与自管理闭环

- 新增页面：adr-pole-self-management-control-loop。
- 更新页面：ai-features、adr-console-agent-resource-workbench、adr-system-configuration-control-plane、auth-system、architecture、todo、index。
- 变更摘要：
  - 新增隔离执行身份 `pole-self-manager`，启动和周期性收敛 Control Plane 自身 MCP、真实工具快照与 Pole Agent A2A Card。
  - Pole Agent 发布公开 Card 和受认证的 JSON-RPC `message/send`；Console Agent 按 Registry 自然键解析并消费自身 MCP。
  - System Settings 保存管理员 desired revision 后自动探测和应用健康 Prompt/模型/工具策略，失败时保留 rejected 草稿和 last-known-good。
  - Registry 写入使用稳定 ID、revision、软删除复活和子项 upsert，重复 reconcile 保持幂等；配置主体与系统执行主体分别审计。

## [2026-07-26] refactor | 治理范围收敛为 RPC-first

- 新增页面：adr-rpc-first-governance-scope。
- 撤下页面：adr-multi-resource-governance-platform。
- 更新页面：terminology、domain-models、governance-rules、todo、lessons、index。
- 变更摘要：
  - 核心治理明确聚焦 HTTP、gRPC、Dubbo 服务调用及其路由、限流、鉴权、镜像、Mock、熔断和 A/B Test。
  - Kafka、RocketMQ、Redis、MySQL 暂不进入运行时灰度和请求级治理，只保留资源目录、健康、指标与原生管理集成。
  - 不向 specification 预埋多资源域万能策略；Capability Profile、原子 Bundle 和 Apply Receipt 只围绕真实服务数据面深化。
  - Job 只有在 Pole 拥有统一 Worker/Agent、租约和执行协议后再独立评估。

## [2026-07-26] feat | 四协议服务契约上报与可视化闭环

- 新增页面：adr-service-contract-reporting-and-visualization。
- 更新页面：service-discovery、api-servers、storage、cache-layer、todo、index。
- 变更摘要：
  - 统一 HTTP/OpenAPI、gRPC、Dubbo、Thrift 的 SDK gRPC 与 Agent/CI HTTP 推送模型。
  - OpenAPI 3.x 由服务端抽取接口，其余 RPC 协议由构建侧提交结构化接口；不让控制面主动扫描生产服务。
  - Client 与 Manual 按来源独立全量替换，修复缓存 miss、软删除回流、字段兼容和统一 Discover 分发。
  - 服务详情增加“服务契约”页签，展示四协议版本、接口来源和原始契约。

## [2026-07-26] refactor | Envoy xDS v3 对齐统一治理发布

- 更新页面：api-servers、governance-rules、todo、index。
- 变更摘要：
  - Envoy Node 通过显式治理 metadata 标签复用 normal/gray release 选择，RDS、VHDS、CDS 改为 node-scoped 稳定快照，EDS 继续按 namespace 共享。
  - 路由转换改用顶层 caller → callee 服务范围，保留 DestinationGroup 标签、权重和多目标，不再从目标组反推服务身份。
  - Gateway 接入真实节点策略生成链；LDS 与策略资源先完整构建再原子替换，相同 namespace 与治理标签的节点复用规则选择结果。
  - OR、动态请求参数、非基础 QPS 限流字段及无等价 Envoy 执行模型的能力不再静默降级，并在知识库固化当前支持矩阵。

## [2026-07-27] refactor | Dependabot 安全告警清理

- 更新页面：todo、lessons、index。
- 变更摘要：
  - 清理 Go 与 Console 的 Dependabot 告警依赖，移除无安全修复版本的前端 Mock 与路由依赖。
  - Go 漏洞扫描无受影响符号或导入包，Console npm 审计无开放漏洞。
  - 增加面向 `develop` 的 Go 与 Console npm Dependabot 周更配置，并将远端开放告警归零作为发布门禁。

## [2026-07-27] ingest | Dependabot 修复版本更新至本地 Kubernetes

- 更新页面：todo、index。
- 变更摘要：
  - 为 `develop@69be9be7` 构建 Linux ARM64 镜像 `pole-control-plane:local-20260727-dependabot-69be9be7`，滚动更新 OrbStack `pole-system` Control Plane。
  - 通过 Pod imageID、Pod 内静态资源、Gateway 返回哈希和 HTTPRoute 状态确认实际入口已切换到新容器产物。
  - Pod 内 Console、8090 Control Plane 与 functions 健康接口均返回 200；真实浏览器在 Gateway 打开 `demo-governance/demo-order` 契约 Tab，四协议真实数据可见且无浏览器错误。
  - Deployment 补齐 source revision、image ID、change-cause 和部署时间注解；Dependabot 开放告警保持为 0。

## [2026-07-27] feat | 服务契约详情与 Dubbo Metadata 适配

- 更新页面：terminology、domain-models、service-discovery、adr-service-contract-reporting-and-visualization、todo、lessons、index。
- 变更摘要：
  - 修复接口清单只有静态摘要、无法进入详情的问题，新增可点击和键盘访问的接口详情抽屉。
  - 详情展示协议字段、方法签名、来源、修订、接口定义和全部 metadata；Dubbo 增加 Metadata Center 专属投影。
  - 统一上报入口可直接识别 Dubbo provider/consumer 运维定义和 application revision 快照，保留原始 JSON 并自动抽取接口。
  - Dubbo 原始 metadata 键不丢失，同时生成稳定的 `dubbo.*` 查询键和 Service Key。
  - 运行时 HTTP client 上报路径明确为 `/v1/ReportServiceContract`；`/naming/v1` 只承载 Console 管理 API。
  - 运行时代码提交 `015cbdbd` 构建并更新到本地 Kubernetes；真实 Dubbo 原生样例上报、鼠标/键盘详情交互与 Metadata Center 投影均通过浏览器验证。

## [2026-07-28] ingest | 配置模板与 Namespace Value 客户端渲染方案

- 新增页面：adr-config-template-client-rendering。
- 更新页面：config-center、todo、lessons、index。
- 变更摘要：
  - 模板与 Value 分别版本化，Value 按 `Namespace + Template` 建立聚合并支持正式、灰度发布。
  - 服务端根据客户端标签选择唯一命中的 Value Release，SDK 不接收或解释灰度规则。
  - SDK 获取模板与 Value 的原子组合快照，本地缓存并确定性渲染；失败时保留 last-known-good。
  - 配置文件显式固定和切换模板发布版本，模板升级不会自动影响已有绑定。
  - 首期模板语法只支持类型化变量替换，不允许任意函数、脚本、环境变量或外部 I/O。

## [2026-07-28] refactor | 配置模板跨语言引擎与服务端预览职责

- 更新页面：adr-config-template-client-rendering、config-center、todo、lessons。
- 变更摘要：
  - 服务端渲染结果只用于预览、格式诊断和跨语言一致性参考，不作为 SDK 运行时权威配置。
  - SDK 继续对服务端选中的 Template Release 与 Value Release 执行本地渲染，并校验服务端参考哈希。
  - 不采用完整 Go template，定义基于 Mustache 1.3 core 的 `pole-mustache-v1` 严格 Profile。
  - 首期只允许 triple-mustache dotted scalar 变量，禁用 section、partial、lambda、helper、动态 delimiter 和外部 I/O。
  - 服务端参考实现与所有语言 SDK 必须运行同一份语言无关测试向量。

## [2026-07-28] ingest | 配置模板 specification 与服务端参考实现

- 更新页面：adr-config-template-client-rendering、config-center、todo、lessons。
- 变更摘要：
  - specification 保留现有配置类型 wire 布局，新增模板/Value release、显式 binding、RenderSnapshot、预览 RPC 和客户端能力协商契约。
  - ConfigFile 只有纯文本与模板两种类型；SDK 的渲染结果不是第三种文件类型。
  - control-plane 增加模板与 Namespace Value 持久化、严格参考渲染、格式校验、发布时 binding 事务激活和回滚恢复。
  - Discover 从已经命中的普通或灰度 ConfigFileRelease 读取固化 binding，只向 SDK 返回匹配后的 Value 快照和参考哈希。
  - 预览响应通过统一 `code/info` 表达鉴权、参数和系统错误，渲染 diagnostics 不承担 API 错误。
  - SDK 本地渲染/last-known-good、组合 revision Watch 与 Console 管理入口仍待后续接入。

## [2026-07-28] feat | 配置模板 Console 管理入口

- 更新页面：adr-config-template-client-rendering、config-center、todo。
- 变更摘要：
  - Console 增加模板草稿、参数 Schema、Template Release、Namespace Value 正式/灰度发布与参考预览工作台。
  - ConfigFile 详情显式选择普通文本或模板渲染，并固定明确的 Template Release；发布详情展示固化 binding。
  - 新增模板、Value 与 binding 管理查询接口，前端可在刷新后恢复草稿和发布历史。
  - 模板 binding 与文件草稿字段在同一服务端事务内更新，失败不会留下不完整模板草稿。
  - 模板发布只从已保存的模板草稿生成不可变快照；不支持的预览引擎使用统一参数错误码。

## [2026-07-28] ingest | 统一进程模式与 Pole Limiter 集成方案

- 新增页面：adr-unified-process-mode-and-limiter-integration。
- 更新页面：architecture、configuration、index、todo。
- 变更摘要：
  - 决定以同一源码、二进制和镜像提供 Console、Control Plane、Limiter 与 all Profile，生产仍保持独立 workload。
  - 定义可嵌入 Module、Running 与 Supervisor seam，以及 readiness、失败回滚和逆序优雅停机不变量。
  - 明确 Limiter 是有状态数据面，`all` 只适用于 quickstart、演示和轻量部署。
  - 记录现有 `all=control-plane+console` 的破坏性兼容问题，并给出跨 breaking release 的迁移方案。
  - 记录 Limiter node-id、advertised endpoint、长流、内存状态以及 8100/8101 网络安全约束。

## [2026-07-28] ingest | 逻辑服务与环境服务显式关联

- 新增页面：adr-logical-service-environment-binding。
- 更新页面：terminology、domain-models、business-rules、namespace、service-discovery、index、todo。
- 变更摘要：
  - 不再使用服务名作为跨环境权威身份；环境服务继续以 `namespace + runtimeServiceName` 运行。
  - 引入仅属于控制面的 Logical Service 稳定 ID，SDK、注册发现和数据面治理协议均不感知。
  - 不同 Namespace 下的环境服务由管理员在 Console 显式关联，控制面只提供候选建议，不自动强绑定。

## [2026-07-28] ingest | 业务环境与 Pole 系统空间类型化

- 新增页面：adr-system-namespace-kind。
- 更新页面：namespace、terminology、domain-models、business-rules、index、todo、lessons。
- 变更摘要：
  - Namespace 增加 `BUSINESS | SYSTEM` 一等类型，旧 payload 缺省值保持为 BUSINESS。
  - `pole-system` 定义为当前 Pole 安装实例的内部管理面空间，不再解释为普通业务环境。
  - 公共 API 禁止创建或删除系统空间，MySQL 启动迁移幂等回填系统类型。
  - Logical Service、配置分组与配置文件的跨环境聚合排除系统空间，显式系统管理能力仍保留。
  - Console 将业务环境和“Pole 系统空间（当前控制面）”分区展示。
  - 未关联的环境服务仍可正常注册、发现和治理，但不参与跨环境聚合。

## [2026-07-28] feat | 统一进程模式与 Pole Limiter 集成

- 更新页面：adr-unified-process-mode-and-limiter-integration、architecture、configuration、todo。
- 变更摘要：
  - 将 Limiter 核心、协议服务、注册与统计实现纳入统一源码和二进制。
  - 增加 `control-plane`、`limiter-server`、`full` Profile，保留 `server` 别名和原 `all` 兼容语义。
  - 建立统一 Supervisor，提供同步 listener readiness、部分启动回滚、运行期 fail-fast 和逆序优雅停机。
  - 示例默认只开放内部 gRPC `8101`，HTTP 运维端口保持关闭；生产仍推荐同镜像、分 workload 部署。

## [2026-07-29] ingest | Console、Limiter 源码归属与前端嵌入制品

- 新增页面：adr-console-limiter-source-layout-and-embedded-web。
- 更新页面：architecture、adr-unified-process-mode-and-limiter-integration、index、todo、lessons。
- 变更摘要：
  - Console Go 网关与 Limiter 统一归入 `pkg/console`、`pkg/limiter`，内部实现由局部 `internal` 保护。
  - Console 前端源码迁入 `web/console`，不建立独立版本或发布生命周期。
  - Release/Test 由统一构建流程生成前端产物并复制到 Go 包生成目录，通过 `go:embed` 进入单文件二进制。
  - 本地开发保留 Vite dev server 与 HMR，正式制品不再依赖外部 `webPath`。
  - bootstrap 只依赖两个 Module 的根 Interface；共享 A2A/Agent 类型上提到稳定契约层。

## [2026-07-29] implement | Console、Limiter 目录迁移与 go:embed

- 更新页面：overview、adr-console-limiter-source-layout-and-embedded-web、adr-unified-process-mode-and-limiter-integration、ai-features、console-ui-quality-gates、testing、todo。
- 变更摘要：
  - Console 与 Limiter Go Module 已迁入 `pkg/console`、`pkg/limiter`，实现细节收敛到局部 `internal`。
  - Console 前端源码已迁入 `web/console`，发布、测试和本地重建脚本统一生成嵌入资源。
  - Console 首页、hash 资源和 SPA fallback 改为读取 `go:embed` 文件系统，移除运行时 `webPath`。
  - Docker 镜像和发布包只携带 Go 二进制，不再复制独立 Console dist。

## [2026-07-29] ingest | Agent 与 MCP 跨环境逻辑身份

- 新增页面：adr-ai-resource-environment-binding。
- 更新页面：terminology、domain-models、business-rules、ai-features、index、todo。
- 变更摘要：
  - 将 MCP Server 与 A2A Agent Registry 记录定义为 Namespace 环境实例。
  - 引入控制面稳定逻辑定义和显式环境绑定，同名只作为候选建议，不自动聚合。
  - `pole-system` 自身投影不参与 BUSINESS 跨环境聚合。
  - 后端服务使用稳定 ID 作为权威引用，默认只允许同 Namespace 绑定。
  - Pole Agent 每次 Turn 使用强类型 Namespace scope；跨环境模式只读，写操作必须单环境。
  - 落地 Definition/环境绑定 Store 与管理 API、稳定服务引用迁移、Namespace/Service 删除保护。
  - Console 支持显式关联逻辑定义和详情环境切换；Pole Agent 支持单环境与显式跨环境只读范围。
  - Agent scope 由当前 Actor 的 BUSINESS Namespace 目录在服务端复核，不信任客户端或 Prompt。

## [2026-07-29] fix | 服务删除入口与双选择器

- 更新页面：service-discovery、business-rules、todo、lessons。
- 变更摘要：
  - 环境服务删除同时接受稳定 ID 或完整 `namespace + name`，ID 路径先解析真实资源坐标。
  - 两条路径复用相同的鉴权、依赖检查、审计和软删除流程。
  - Console 恢复逻辑服务、未关联环境服务、环境服务详情和系统空间服务删除入口。
  - 逻辑服务删除与环境服务删除保持独立语义，并统一使用权限状态和确认操作。

## [2026-07-29] refine | 服务数据页面纵向密度

- 更新页面：index、todo、lessons。
- 变更摘要：
  - 对照桌面截图审计服务首页、逻辑详情、系统服务及环境服务各页签的纵向空间。
  - 将标题与工具栏的紧凑密度作为显式可选能力，保持其它资源页面默认布局不变。
  - 收敛服务摘要、页签空内容、表格行和分页间距，并补齐详情与实例表格的剩余高度分配链。
  - 将页面路径、资源名称和说明压缩进全局顶部导航，操作按钮保留在正文工具栏，并重新校准顶栏文字层级。
  - 增加服务页面密度契约、响应式检查和 Console 嵌入资源验证记录。

## [2026-07-31] refactor | 统一观测运行时数据目录

- 更新页面：configuration、adr-local-pebble-protobuf-value-cache、adr-otel-observability-platform、index、todo。
- 变更摘要：
  - 将 OTel event/audit 本地 Pebble 队列迁移到 `./.pole_data/observability/otel-events`。
  - 明确 `.pole_data` 为配置缓存和观测可靠队列共用的进程本地运行时数据根。
  - Kubernetes 收敛为单一 `runtime-local-db` 数据卷，`/app/.pole_data` 为规范挂载点，`/app/data` 仅保留为同卷兼容别名。
  - 删除误提交到顶层 `data/` 的 Pebble 运行时数据库制品。

## [2026-07-31] ingest | 官方内置插件与用户扩展 Registry

- 新增页面：adr-plugin-extension-registry。
- 更新页面：architecture、patterns、index、todo。
- 变更摘要：
  - 新增公共 Plugin Registry、Descriptor、Factory、Kind 与冻结/激活契约。
  - 官方实现通过 `builtinplugins` 显式组合，用户插件由独立 Go Module 注册到同一 Registry。
  - 通用插件、Store、API Server 与 Auth 统一从当前运行 Registry 解析实例。
  - 保留旧注册函数、默认 Registry 与 API Server Slots 运行投影作为兼容层，官方集合不再依赖 init。
  - 进程内用户插件采用自定义静态发行版；动态扩展后续使用版本化 RPC，不采用 Go plugin。

## [2026-07-31] refine | 内置插件运行生命周期收口

- 更新页面：adr-plugin-extension-registry、architecture、patterns、index、todo、lessons。
- 变更摘要：
  - Registry 统一跟踪已解析实例，Bootstrap 退出或启动失败时按逆序调用 Destroy。
  - Auth 移除旧内部 slot map，Store 与 API Server 兼容投影不再参与注册冲突判断。
  - RateLimit 与 Whitelist 从 Active Registry 解析、初始化并按运行 Registry 隔离。
  - 外部示例工程不进入本轮范围，优先完成官方内置插件闭环。

## [2026-08-01] research | Pole 与 Kmesh 产品差异

- 新增页面：kmesh-product-research。
- 更新页面：index、todo。
- 变更摘要：
  - 基于 Kmesh 官方网站、官方文档和官方 GitHub 仓库核验其数据平面定位。
  - 区分节点 eBPF、Waypoint 高级 L7 与 Istiod 控制面的职责边界。
  - 记录已证实的治理、观测、部署前提，以及不能从路线图直接推导的能力。
  - 明确 Pole 与 Kmesh 分属控制面和数据面层次，组合接入仍需协议适配与 E2E 验证。

## [2026-08-01] research | Website 产品对比分层

- 新增页面：pole-product-comparison-research。
- 更新页面：kmesh-product-research、index、todo、lessons。
- 变更摘要：
  - 核验 Nacos、Apollo、PolarisMesh、Kmesh 与 Istio 的官方当前定位和部署边界。
  - 将比较关系分为兼容迁移、同层控制面竞争、完整 Mesh 生态和数据面组合。
  - 明确 Website 应在首页提供关系摘要，在产品专题页提供完整矩阵，不扩张顶部主导航。
  - 记录 xDS、mTLS、协议兼容和完全替代等容易误导的文案护栏。

## [2026-08-01] deliver | Website 产品对比体验上线

- 更新页面：todo。
- 变更摘要：
  - Website 新增 `/compare` 专题页，并在首页和产品页提供对比入口。
  - 新增中英文对称的技术对比文档，明确兼容迁移、同层竞争、Mesh 分工与数据面组合边界。
  - 补充路由、内容、事实边界和双语对称测试，通过桌面端与移动端浏览器验收。
  - Website 提交 `bea8301` 已部署至 GitHub Pages；公开首页、对比页及中英文文档均返回 HTTP 200。

## [2026-08-01] fix | Website 产品对比文档侧栏可见性

- 更新页面：todo、lessons。
- 变更摘要：
  - 修复 Fumadocs 精选侧栏树遗漏产品对比页面的问题。
  - 中文侧栏新增“产品对比”，英文侧栏新增“Product comparison”，并补充标题与 URL 回归断言。
  - Website 提交 `68b30aa` 已通过 GitHub Pages workflow `30695915460` 部署。
  - 本地和线上浏览器均确认中英文侧栏目录项可见且可达。

## [2026-08-01] research | Pole 产品选型能力双矩阵

- 新增页面：pole-product-capability-matrix-research。
- 更新页面：pole-product-comparison-research、index、todo、lessons。
- 变更摘要：
  - 参考 PolarisMesh 官方注册中心与服务网格对比页，将选型信息拆成注册配置和服务治理 Mesh 两个领域矩阵。
  - 为已实现、部分或协议兼容、未验证和非同层能力定义明确状态，避免单一勾选掩盖运行时边界。
  - 记录 Pole xDS 当前可映射的治理子集，以及 Istio、ambient、Waypoint 和 Kmesh 尚未验证的互操作边界。
  - 明确不采用缺少同版本、同拓扑和同负载方法的 TPS 与架构排名。

## [2026-08-01] deliver | Website 产品选型能力双矩阵上线

- 更新页面：todo。
- 变更摘要：
  - Website 产品对比升级为可展开的文档目录，新增中英文“注册与配置中心对比”和“服务治理与 Mesh 对比”。
  - 矩阵逐项展示状态、运行时边界、迁移判断和官方资料，不以无统一测试条件的 TPS 数字作产品排名。
  - 20 项测试、lint、185 页生产构建与桌面/移动端浏览器验收通过。
  - Website 提交 `10ac139` 已由 GitHub Pages workflow `30696829943` 成功部署，线上中英文目录和页面已验证。

## [2026-08-01] refine | 注册发现与配置中心竞品集合纠正

- 更新页面：pole-product-capability-matrix-research、index、todo、lessons。
- 变更摘要：
  - 注册发现矩阵固定为 Pole、Nacos、PolarisMesh、Consul、Istio，配置中心矩阵固定为 Pole、Nacos、Apollo、Consul、PolarisMesh。
  - 依据 HashiCorp 与 Istio 官方资料补充 Consul Catalog、健康检查、DNS/HTTP、基础 KV，以及 Istio 平台发现源、内部注册表和 ServiceEntry 边界。
  - Website 总览与中英文矩阵同步增加 Consul，并明确 Pole 当前没有 Consul 协议直连、Consul KV 不等价于完整配置发布中心。
  - Website 提交 `b6fa221` 已由 GitHub Pages workflow `30705935415` 成功部署，线上中英文表头已验证。

## [2026-08-02] decide | 通用配额租约与流式限流

- 新增页面：adr-quota-lease-streaming-rate-limit。
- 更新页面：governance-rules、index、todo、lessons。
- 变更摘要：
  - 未正式发布阶段直接原地收敛协议，只保留无版本后缀的 `RateLimitGRPC`。
  - 统一 reserve/update/settle 生命周期，以累计消费、sequence、幂等键和 TTL 管理租约。
  - RPM、TPM 使用消耗型配额，并发使用占用型配额；不引入金额或计费语义。
  - TPM 在调用前按输入与最大输出预留，流中只调整账本，结束归还未使用量，避免响应开始后再返回配额 429。

## [2026-08-02] release | Quota Lease specification ALPHA.40

- 更新页面：todo、lessons。
- 变更摘要：
  - specification PR #13 合入 develop，发布 annotated tag 与 prerelease `v0.1.0-ALPHA.40`。
  - 合并提交 Go/Rust CI 与 Release-Rust workflow 成功，Rust crate 已发布。
  - control-plane 删除 `go.work` 本地覆盖并升级 `go.mod`；Rust SDK 根 crate 与 e2e crate 同步升级 tag。
  - 两个消费者均在无本地 replace/patch 条件下完成全量测试。

## [2026-08-04] deliver | Streaming TPM 限流产品闭环

- 更新页面：adr-quota-lease-streaming-rate-limit、index、todo、lessons。
- 变更摘要：
  - Console 将底层资源枚举提升为 QPS、RPM、TPM 和并发四个场景模板。
  - TPM 编辑器明确请求前预留、流中累计用量和结束归还，不再使用普通请求中途 429 的描述。
  - Limiter 增加按服务过滤的租约快照，展示活跃、结算、TTL 回收以及预留、消费、归还和可用量。
  - Console 通过认证代理访问内部 Limiter HTTP 运维面，前端不直接暴露 8102。

## [2026-08-04] release | RateLimit callee ALPHA.45 与完整产品验收

- 更新页面：adr-quota-lease-streaming-rate-limit、todo。
- 变更摘要：
  - specification `v0.1.0-ALPHA.45` 正式增加 `RateLimit.callee`，Java、Rust、C++、npm/PyPI 发布工作流全部成功。
  - control-plane 通过正式 tag 升级依赖，以 namespace 表达归属环境、callee 表达目标服务。
  - 修复限流列表 owner/callee 混用和公共数字输入多位值回退问题。
  - OrbStack full 模式和真实 8080 浏览器完成 `120000 Token / 60s` TPM 规则创建、回读与 Quota Lease 运行态验收。

## [2026-08-04] refine | 限流计量场景收敛

- 更新页面：adr-quota-lease-streaming-rate-limit、governance-rules、index、todo、lessons。
- 变更摘要：
  - 移除与可配置统计窗口重复的 RPM 场景卡片。
  - Console 只保留 QPS 请求数、TPM Token 数和并发三类资源场景，QPS/TPM 分别提供 1 秒和 60 秒默认窗口。
  - 请求数使用 60 秒窗口即表达 RPM，修改窗口不会改变资源场景选中状态。
  - 前端契约、构建和真实 `pole.localhost` 浏览器验收通过，并已更新 OrbStack 集群。

## [2026-08-04] refine | 限流资源交互去重

- 更新页面：adr-quota-lease-streaming-rate-limit、todo、lessons。
- 变更摘要：
  - QPS、TPM、并发场景卡成为资源类型的唯一选择入口。
  - 移除卡片下方重复的请求数、Token 数、并发数单选组。
  - 场景选择后直接展示阈值窗口或并发上限，前端契约、构建、OrbStack 部署和真实页面验收通过。

## [2026-08-04] fix | 限流接口编辑行布局

- 更新页面：todo、lessons。
- 变更摘要：
  - 修复 Fluent Combobox 默认最小宽度撑破接口网格、覆盖方法和匹配类型列的问题。
  - 约束 grid item、Combobox 根节点和内部 input 可收缩，并为窄容器保留可滚动的最小网格宽度。
  - 生产构建、OrbStack 部署、截图和浏览器边界测量均通过，接口行控件无交叠。

## [2026-08-04] refine | 准入策略提示视觉

- 更新页面：todo、lessons。
- 变更摘要：
  - 移除把“快速失败”配置误表现为当前错误的大面积红色结果卡。
  - 改为中性、紧凑的策略说明条，按 TPM、快速失败和排队模式解释实际执行行为。
  - 前端契约、生产构建、OrbStack 部署和真实页面截图验收通过。

## [2026-08-05] deliver | 系统接口指标与 Trace 下钻闭环

- 更新页面：adr-otel-observability-platform、index、todo。
- 变更摘要：
  - 系统监控接口明细改为请求量、时间窗 QPS 和 Histogram P95/P99 的准确口径。
  - control-plane HTTP/gRPC 入口生成 OTel server spans，Collector 将 Trace 写入带 TTL 的 GreptimeDB `pole_traces`。
  - Console 增加平台 Trace 列表与详情 API，接口聚合行可下钻同时间窗 Trace 和 Span 瀑布。

## [2026-08-05] fix | control-plane 内部 Span 层级补齐

- 更新页面：adr-otel-observability-platform、todo、lessons。
- 变更摘要：
  - 保留 HTTP 请求父 Context，并由 Console 反向代理 CLIENT Span 传播 W3C TraceContext。
  - 在 handler、限流、鉴权、namespace service、cache 和 store 稳定边界生成 INTERNAL Span。
  - 鉴权 Span 按动作在单次请求内去重，避免资源列表逐项权限判断导致调用树刷屏。
  - OrbStack 真实 `GET:/core/v1/namespaces` Trace 回读为 9 个 Span，父子层级和状态均正确。

## [2026-08-05] refine | Trace 调用树可视化重构

- 更新页面：adr-otel-observability-platform、todo、lessons。
- 变更摘要：
  - Trace 详情保留微秒级开始和结束时间，避免短 Span 全部挤在同一毫秒刻度。
  - Span 由开始时间平铺改为父先子后的深度优先调用树，并提供连接线与展开折叠。
  - 增加全局时间概览、类型图例、选中态和 Span 检查器，明确调用层级与真实时间顺序。

## [2026-08-05] fix | 鉴权 Span 写操作语义澄清

- 更新页面：adr-otel-observability-platform、todo、lessons。
- 变更摘要：
  - 确认命名空间查询中的 Update/Delete 是 `editable/deleteable` 权限能力探测，不是业务写操作。
  - 请求授权 Span 增加 `authorize` 层级，列表操作能力检查增加 `capability` 层级。
  - OrbStack 新 Trace 已从 GreptimeDB 回读验证三类鉴权 Span 命名。

## [2026-08-07] implement | A2A 与 MCP 环境化资源视图

- 更新页面：ai-features、index、todo。
- 变更摘要：
  - A2A 与 MCP 一级页面统一为“全局逻辑资源 + 环境实例”两层视图。
  - 逻辑资源目录展示环境覆盖，并提供创建逻辑定义和显式绑定已有环境实例的入口。
  - 从环境空间进入时默认按当前 Namespace 查询实例，创建表单锁定当前环境，MCP 服务候选同环境过滤。
  - 环境实例列表显式展示环境空间，`pole-system` 不进入业务逻辑资源聚合。

## [2026-08-08] implement | 配置模板标签与敏感 Value 加密

- 更新页面：adr-config-template-labels-sensitive-values、adr-config-template-client-rendering、index、todo、lessons。
- 变更摘要：
  - 模板标签作为独立目录元数据持久化和授权读写，不进入渲染快照。
  - Schema 敏感参数改为真实逐值 AES 加密落库，草稿和发布读取时受控解密。
  - 模板说明回归单行输入，与名称和格式控件保持一致高度。

## [2026-08-08] refine | 配置模板 Schema 与 Value 入口显性化

- 更新页面：todo、lessons。
- 变更摘要：
  - 模板工作区新增可点击的三步任务引导，明确模板、Schema 与环境 Value 的先后关系。
  - Schema 查看态、空态及 Value 前置空态提供就地动作，并自动切换到所需页签和编辑状态。
  - 补充 Console 防回退契约、响应式样式，并完成 OrbStack 与真实浏览器验收。

## [2026-08-08] refactor | 配置模板工作区信息架构与双版本预览

- 更新页面：adr-config-template-client-rendering、index、todo、lessons。
- 变更摘要：
  - 去除流程条与页签的重复导航，将模板元信息提取为固定上层。
  - 一级任务收敛为模板内容、参数 Schema、环境 Value 和版本管理。
  - 版本管理分别展示 Template Release 与环境 Value Release，并支持选择两者进行格式化渲染预览。
  - 中等宽度自动切换单列布局，完成契约、构建、OrbStack 和真实浏览器验收。

## [2026-08-09] refine | 配置模板版本历史行直接选择

- 更新页面：adr-config-template-client-rendering、index、todo、lessons。
- 变更摘要：
  - 移除与版本历史表重复的模板版本和 Value 版本下拉框。
  - 两张历史表改为整行单选，顶部只汇总已选组合并触发渲染预览。
  - 未选齐两个版本时禁用预览，避免默认组合掩盖用户的显式选择。

## [2026-08-09] refine | 配置模板版本预览动作收敛

- 更新页面：adr-config-template-client-rendering、todo、lessons。
- 变更摘要：
  - 删除版本表上方重复的组合标题、状态标签和已选版本摘要。
  - 选择与反馈完全由两张历史表承担，操作区只保留“渲染预览”按钮。
  - 两侧未选齐时按钮保持禁用，渲染结果与 diagnostics 继续在结果区展示。

## [2026-08-09] fix | 配置模板 Schema 连续输入失焦复发

- 新增页面：console-editable-row-focus-loss。
- 更新页面：console-ui-quality-gates、index、todo、lessons。
- 变更摘要：
  - 真实浏览器复现参数名逐字符输入只保留首字符，确认可编辑参数名参与 React key 导致整行重挂载。
  - Schema 参数行改用编辑期间稳定 key，并纳入连续输入专项门禁和配置模板契约。
  - 将第二次同类回归归因于门禁组件白名单覆盖不足，新增动态编辑器必须同步注册静态与浏览器验收。

## [2026-08-09] fix | 配置模板环境 Value 前端类型校验

- 更新页面：adr-config-template-client-rendering、console-ui-quality-gates、index、todo、lessons。
- 变更摘要：
  - 环境 Value 根据 Schema 即时校验必填项、规范整数和规范小数，并提供可访问内联反馈。
  - 保存草稿和发布复用同一校验结果，字段无效时同时禁用动作并保留函数级拦截。
  - 前端镜像服务端 canonical 标量规则，服务端继续作为最终契约权威。

## [2026-08-09] refine | 配置模板环境 Value 就地渲染预览

- 更新页面：adr-config-template-client-rendering、console-ui-quality-gates、index、todo、lessons。
- 变更摘要：
  - 环境 Value 编辑页使用固定模板版本与当前未保存 Value 直接执行服务端参考预览。
  - 草稿预览和历史版本组合预览复用结果面板，但隔离状态与入口语义。
  - Value 或固定模板版本变化后清除旧草稿预览，避免展示过期结果。

## [2026-08-09] refactor | 配置文件清单统一文本与模板类型

- 更新页面：config-center、adr-config-template-client-rendering、index、todo、lessons。
- 变更摘要：
  - 配置分组移除“配置文件 / 配置模板”同级切换，只保留统一配置文件清单。
  - 创建配置时先选择直接文本或配置模板，模板类型继续固定不可变 Template Release。
  - 文件树为存量和新建文件展示文本或模板来源标识；模板资产管理改为任务内入口。

## [2026-08-09] fix | 统一配置清单恢复历史模板可见性

- 更新页面：config-center、adr-config-template-client-rendering、index、todo、lessons。
- 变更摘要：
  - 确认历史模板、模板 Release 和环境 Value Release 数据仍在，缺陷来自前端统一清单只查询配置文件。
  - 统一清单新增展开的全局模板目录，直接读取并展示原模板数据，点击进入原模板工作区。
  - 保持 ConfigFile 的 Namespace/分组归属与 ConfigFileTemplate 的全局归属，不复制、不迁移、不重建数据。

## [2026-08-09] refine | 配置模板节点扁平化

- 更新页面：config-center、adr-config-template-client-rendering、todo、lessons。
- 变更摘要：
  - 删除统一配置清单中的“配置模板”虚拟目录，模板与配置文件改为同级叶子节点。
  - 继续使用尾部“文本 / 模板 / 全局模板”标签区分资源类型与作用域。
  - 模板节点与普通文件保持同级，不改变数据归属和版本历史。

## [2026-08-09] refine | 配置模板详情改为右侧就地展示

- 更新页面：config-center、adr-config-template-client-rendering、todo、lessons。
- 变更摘要：
  - 点击统一清单中的全局模板时保持当前配置分组路由、左侧清单和选中状态。
  - 右侧画布嵌入原模板内容、Schema、环境 Value 和版本管理，隐藏重复的模板目录与页头。
  - 独立模板工作区路由只保留为已有深链兼容入口。

## [2026-08-09] refine | 模板详情对齐配置文件视觉骨架

- 更新页面：config-center、adr-config-template-client-rendering、todo、lessons。
- 变更摘要：
  - 模板嵌入态与配置文件详情统一摘要头、资源路径、操作区、页签节奏和扁平内容画布。
  - 模板信息查看态改为两列字段网格，移除禁用输入框式只读展示。
  - 保留模板固定信息层与内容、Schema、环境 Value、版本管理任务，不改变领域边界。

## [2026-08-09] fix | 配置清单环境上下文常驻

- 更新页面：config-center、todo、lessons。
- 变更摘要：
  - 环境切换器从资源未选中条件中移出，文件与模板选择态继续固定显示。
  - 删除文件详情内部重复环境切换器，统一由清单顶部承载跨环境导航。
  - 删除重复“配置分组工作区”卡片，顶部仅保留配置中心与分组面包屑。

## [2026-08-09] refine | 模板基本信息归入独立页签

- 更新页面：config-center、adr-config-template-client-rendering、index、todo、lessons。
- 变更摘要：
  - 模板名称、目标格式、说明与标签从页签上方固定信息层移入最后的“基本信息”页签。
  - 模板一级任务统一为内容、Schema、环境 Value、版本管理和基本信息。
  - 新建模板默认进入基本信息页签，保留必填元信息的明确入口。

## [2026-08-09] refine | 模板与 Value 发布对象分区

- 更新页面：config-center、adr-config-template-client-rendering、index、todo、lessons。
- 变更摘要：
  - 模板定义页签只显示模板编辑与模板版本发布，Value 与版本管理页移除全局模板操作。
  - 模板版本和环境 Value 版本分别通过确认弹窗审阅对象、范围、说明及灰度参数。
  - 模板 Release 支持独立发布说明，旧请求未提供时兼容回退模板说明。

## [2026-08-09] refine | 配置模板草稿组合预览

- 更新页面：config-center、adr-config-template-client-rendering、index、todo、lessons。
- 变更摘要：
  - 版本管理的模板与 Value 历史表各增加一个明确的当前草稿选择行，并默认选择草稿。
  - 唯一渲染动作支持草稿与 Release 的四种组合，不再要求双方先发布。
- 草稿组合预览只读取页面状态；草稿变化后清除旧结果，不保存数据或创建 Release。

## [2026-08-10] refactor | 模板与 Value 收敛为原子环境版本

- 更新页面：terminology、domain-models、config-center、adr-config-template-client-rendering、index、todo、lessons。
- 变更摘要：
  - 环境配置版本成为唯一可生效发布对象，原子绑定 Template Snapshot 与 Value Snapshot。
  - 模板快照仅作为内部不可变证据，内容未变化时跨环境版本复用；旧独立发布接口只保留兼容。
  - 配置文件绑定逻辑模板，运行时按环境和灰度标签选择完整组合版本；Console 使用单一发布入口和单表历史。

## [2026-08-10] design | 全局环境晋升拓扑

- 更新页面：terminology、domain-models、namespace、index、todo、lessons。
- 变更摘要：
  - 环境晋升收敛为 Namespace 域的全局用户可配置 DAG，不属于配置中心私有能力。
  - 晋升边只授权资源流转方向，目标环境仍需候选变更、校验和发布。

## [2026-08-10] design | lane 资源变更集回归 base

- 更新页面：terminology、domain-models、namespace、todo。
- 变更摘要：
  - lane 回归以带分叉基线的可选资源变更集为单位，不用整环境快照覆盖 base。
  - 每项资源三方比较并进入 base 候选，冲突解决和 base 发布完成后才能继续晋升。

## [2026-08-10] design | 晋升边门禁策略

- 更新页面：terminology、domain-models、namespace、todo。
- 变更摘要：每条晋升边同时携带关系类型、正式来源版本、审批、验证、资源域和冲突策略；晋升只生成目标候选。

## [2026-08-10] design | 晋升拓扑版本化

- 更新页面：terminology、domain-models、namespace、todo。
- 变更摘要：晋升拓扑使用草稿与不可变 Revision；晋升单固定创建时策略，新 Revision 不篡改已有流程，紧急停止通过显式撤销。

## [2026-08-10] design | 可晋升资源与适配器边界

- 更新页面：terminology、domain-models、namespace、todo。
- 变更摘要：全局晋升仅包含已发布、可版本化期望状态；运行时事实与环境专属值排除，各资源域通过 Promotion Adapter 接入。

## [2026-08-10] design | 晋升资源身份对齐与目标新增

- 更新页面：terminology、domain-models、namespace、todo。
- 变更摘要：Adapter 按稳定逻辑 ID 预检 CREATE/UPDATE/CONFLICT/UNSUPPORTED；目标不存在时可创建新增候选，同名异 ID 必须阻断。

## [2026-08-10] design | 跨资源晋升 Bundle 原子边界

- 更新页面：terminology、domain-models、namespace、todo。
- 变更摘要：晋升候选必须全量预检；发布形成不可拆分 Bundle 并原子写入控制面期望状态，执行点回执实现可观测最终收敛。

## [2026-08-10] design | 全局环境晋升完整 ADR

- 新增页面：adr-environment-promotion-topology。
- 更新页面：terminology、domain-models、namespace、config-center、adr-config-template-client-rendering、index、todo。
- 变更摘要：
  - 一次性收敛全局 baseline DAG、lane/base 分叉绑定、变更集与 Bundle 状态机。
  - 定义 Adapter、三方合并、目标专属值、审批/门禁、并发、灰度、回退、存储/API 和 Console 契约。
  - 配置模板拆分为全局逻辑身份和 Namespace 隔离草稿，补充旧全局草稿迁移与分期实施边界。

## [2026-08-10] implementation | 环境晋升拓扑与模板环境草稿纵向切片

- 更新页面：adr-environment-promotion-topology、todo。
- 变更摘要：
  - 落地全局拓扑草稿、DAG/lane binding 校验、不可变 Revision、MySQL/API 与 Console 编辑页。
  - 配置模板内容、格式、Schema 和引擎改为 Namespace 草稿，组合发布读取环境定义；全局只保留逻辑身份信息。
  - 完成旧环境发布快照兼容初始化、全量测试、OrbStack 部署与真实浏览器验收；Adapter、Bundle 和晋升工作台仍待实现。

## [2026-08-10] design implementation | Nacos Eureka 默认环境动态兼容

- 更新页面：terminology、adr-system-configuration-control-plane、index、todo、lessons。
- 变更摘要：
  - Nacos `public`/空 Namespace 与 Eureka 缺省 `x-namespace` 分别使用独立的 GuardedHotReload 配置，初值均为 `dev`。
  - 协议别名不创建公共环境；实例从共享发布版本校验并原子切换，失败保留 last-known-good 并写应用回执。
  - 当前 effective 目标受 Namespace 删除保护；存量 Nacos 订阅固定创建时环境，动态切换只影响新请求与新订阅。

## [2026-08-10] fix | 环境晋升拓扑暗黑模式适配

- 更新页面：todo、lessons。
- 变更摘要：
  - 将晋升拓扑私有样式中的浅色硬编码替换为应用主题 token。
  - 覆盖页面画布、摘要、拓扑节点、编辑卡片、行背景、文字、边框和发布栏。
  - 通过现有暗色主题静态门禁、双主题 computed style 和真实 OrbStack 页面验证。

## [2026-08-11] design implementation | Console 侧边编辑抽屉统一规范

- 更新页面：adr-console-fluent-ui-design-system、todo。
- 变更摘要：
  - 统一 compact、standard、wide、workspace 四档宽度以及标题说明、遮罩、内容滚动和固定底栏边界。
  - 新增 `components/EditorDrawer` 作为创建/短表单编辑的共享框架，配置分组成为首个迁移样板。
  - 暗黑主题、窄视口、提交防重和动态标签连续输入纳入抽屉验收矩阵。

## [2026-08-11] design | Skill Marketplace 与 pole-ai-cli 分工边界

- 新增页面：skill-marketplace、adr-skill-marketplace-federation。
- 更新页面：terminology、domain-models、index、todo。
- 变更摘要：
  - 定义 `publisher/name`、不可变 SemVer Release、Agent Skills Bundle、发布审核和多源 Registry 联邦。
  - 确立 `BundleStore` 深模块与默认 MySQL BLOB Adapter，以及大小、路径、摘要、签名和回收规则。
  - Control Plane 只管理与分发，独立 `pole-ai-cli` 拥有安装、升级、lockfile、回滚和 Agent Adapter。

## [2026-08-11] implement | Skill Marketplace 首期纵向切片

- 更新页面：skill-marketplace、adr-skill-marketplace-federation、ai-features、todo、index。
- 变更摘要：
  - 实现规范化 Agent Skills ZIP、MySQL BundleStore、签名/扫描/审核、私有 grant、公共匿名读和多源持久同步。
  - Console 增加 Marketplace 目录、深链详情、Bundle manifest 安全预览、发布/审核/Registry/授权管理。
  - 新建 `pole-ai-cli`，提供七个命令、三类 Agent Adapter、精确 lock 与本地投影完整性验证。
  - 目标测试、前端构建、CLI Clippy、specification Go/Rust 契约和浏览器 fixture 通过；真实外部集成仍待验收。

## [2026-08-12] deliver | Skill Marketplace develop 与 OrbStack 交付

- 更新页面：todo、log。
- 变更摘要：
  - control-plane 与 specification 已按独立功能提交推送 `develop`；pole-ai-cli 已本地提交，等待配置 remote。
  - 干净提交树发现并修复 EditorDrawer 依赖和 Console 8080 Marketplace 反向代理缺口。
  - OrbStack 已部署提交树镜像，验证 MySQL schema、公共/认证 API、真实 Console 深链、暗色窄视口和刷新返回。
  - 空 Registry 下未伪造已发布 Release，CLI 真实安装和外部 Registry 同步保留为明确未验收边界。

## [2026-08-12] fix | Skill 市场中文产品命名

- 更新页面：todo、lessons、log。
- 变更摘要：
  - 中文 locale 统一为“Skill 市场”，英文 locale 保留 `Skill Marketplace`。
  - 列表与详情页的页头、通知、空状态和可访问名称统一跟随 locale key。
  - 干净构建、OrbStack 部署和真实浏览器中英文切换验收通过。

## [2026-08-12] implement | Git 单仓库批量 Skill 导入

- 更新页面：skill-marketplace、adr-skill-marketplace-federation、ai-features、index、todo、lessons。
- 变更摘要：
  - Tag/Release 先解析到不可变 commit，禁止分支导入与预览后静默换快照。
  - 支持仓库根单 Skill 或指定目录下一级多 Skill 发现，统一 Tag SemVer 并允许显式覆盖。
  - Console 增加发现预览和逐项结果；后端对每个子树独立规范化、扫描、冻结与发布。

## [2026-08-12] fix | Skill 市场发布表单安全留白

- 更新页面：index、todo、lessons、log。
- 变更摘要：
  - 上传与 Git 导入抽屉增加独立正文容器，使表单与页头、底栏保持一致横向基线。
  - 文件选择控件纳入主题、hover、键盘焦点和窄屏样式，并增加专项布局契约。

## [2026-08-12] fix | Skill 市场抽屉交互统一复核

- 更新页面：index、todo、lessons、log。
- 变更摘要：
  - 五个 Skill 抽屉入口统一使用局部正文容器，不影响共享 EditorDrawer 的复杂工作区调用方。
  - 审核、Registry Source、访问策略与空态补齐桌面/窄屏安全留白，并增加全入口数量契约。

## [2026-08-15] ingest | Kubernetes 生产部署基线与 SOP

- 新增页面：adr-kubernetes-production-deployment、kubernetes-production-sop。
- 更新页面：configuration、index、todo、log。
- 变更摘要：
  - 以通用 Helm Chart 交付拆分的 Pole 三 workload 与 Kubernetes 内 MySQL、GreptimeDB、OTel Collector。
  - ACK profile 仅公网暴露 Console ALB HTTPS，其余端口保持 ClusterIP，并使用 ACK CSI 云盘。
  - 原生 Secret 管理运行与 OSS 凭据；GitHub Release 发布公开 GHCR 多架构不可变镜像。
  - 补齐安装、验收、升级、回滚、OSS 备份恢复、卸载和故障排查门禁。

## [2026-08-18] validate | ACK 内部生产栈部署验收

- 更新页面：todo、log。
- 变更摘要：
  - 在用户指定 ACK context 部署 Helm revision 2，验证 7 个 Pod、3 个云盘 PVC 与内部 Service。
  - 验证公开 GHCR digest、MySQL schema、Pole 三组件入口及 OTel 到 GreptimeDB 的 canary 落库回读。
  - 记录 MySQL 最小 Linux capabilities 修复和 NetworkPolicy、ALB HTTPS、OSS 备份尚未完成的真实边界。

## [2026-08-18] investigate | ACK Pole 运行拓扑收敛

- 更新页面：todo、lessons、log。
- 变更摘要：
  - 以源码和测试确认 `all` 只包含 Control Plane 与 Console，Limiter 仅由 `full` 或独立 mode 启动。
  - 明确 Limiter 对分布式限流数据面和 quota-leases 页面负责，不是基础 Control Plane/Console 的启动依赖。
  - 记录合并 Console 后副本数、故障域、扩缩容和滚动发布耦合的待确认取舍。

## [2026-08-18] deploy | ACK demo full 合并模式

- 更新页面：adr-kubernetes-production-deployment、kubernetes-production-sop、todo、lessons、log。
- 变更摘要：
  - Chart 增加默认 split、demo 可选 full 的显式部署模式，并提供 ACK demo overlay。
  - Helm revision 3 将 Control Plane、Console、Limiter 收敛到单 Pod，保留三个稳定 ClusterIP Service。
  - 验证全部监听端口、核心 HTTP、Limiter 运行态、固定镜像 digest 和 OTel 到 GreptimeDB canary 回读。

## [2026-08-18] fix | ACK Helm 资源命名规范化

- 更新页面：adr-kubernetes-production-deployment、kubernetes-production-sop、todo、lessons、log。
- 变更摘要：
  - 修复 release 与 chart 同名时 fullname 重复拼接，运行资源从 `pole-pole-*` 规范为 `pole-*`。
  - 增加 existingClaim 迁移能力，有序停机后让新 StatefulSet 复用原有三块 ACK 云盘。
  - 通过 PV/数据库指纹、六项 HTTP smoke、固定镜像和 OTel canary 证明改名未重置数据。

## [2026-08-18] operate | ACK Demo Console 临时管理员初始化入口

- 更新页面：todo、log。
- 变更摘要：
  - 清理账号风控拒绝的临时 CLB Service，改用 ACK AlbConfig 创建公网 Basic ALB HTTP Listener。
  - ALB ACL 仅允许当前公网 IPv4 `/32`，验证允许来源 200、独立外部来源不可访问。
  - 将入口明确限定为无 TLS 的一次性管理员初始化通道，等待用户完成后删除 ALB 与 EIP。

## [2026-08-18] operate | 临时 Console 入口放开全公网

- 更新页面：todo、lessons、log。
- 变更摘要：
  - 根据用户明确选择移除 ALB HTTP Listener 的 `/32` White ACL。
  - 保留无 TLS、一次性管理员初始化和完成后立即删除的安全边界。

## [2026-08-19] fix | ACK Demo 创建用户空标签提交异常

- 更新页面：todo、lessons、log。
- 变更摘要：
  - 从 ACK 在线哈希资源确认创建用户的 `user_labels.reduce` 在 API 请求前读取未初始化字段。
  - 为共享 LabelInput 注册空数组默认值，并在 UserEditor 提交边界增加数组归一化与专项回归。
  - 发布带 SBOM/provenance 的多架构 GHCR digest，升级 ACK Helm revision 5 并验证新 SPA 哈希、Pod 和数据库保留状态。

## [2026-08-19] fix | ACK 临时 ALB ACL 与 IP 后端调谐

- 更新页面：todo、lessons、log。
- 变更摘要：
  - 区分 Pod/Service 200 与公网 ALB 静默超时，确认省略 aclConfig 没有解除旧 White ACL。
  - 以显式空 Black ACL 表达全公网放开，并让 Ingress 使用预先创建的 IP 类型临时后端 Service。
  - ACK 调谐后两个 ALB EIP 均恢复 HTTP 200，临时清理范围同步包含该 Service。

## [2026-08-19] verify | OrbStack Console 全链路浏览器验收

- 更新页面：console-orbstack-browser-acceptance、index、todo、log。
- 变更摘要：
  - 以真实浏览器完成环境、配置、注册发现、实例、认证、A2A、MCP 和关键响应式交互的增删改查回归。
  - 修复实例 metadata/删除键、服务删除键、配置删除路由、A2A 绑定、MCP 长地址和窄屏头部问题并重放。
  - 明确 Skill 外部导入、Agent 模型、系统配置发布、服务监控 Mock 与九类治理规则逐类发布的未验收边界。

## [2026-08-19] fix | OrbStack Console 验收缺口补齐

- 更新页面：console-orbstack-browser-acceptance、configuration、todo、lessons、log。
- 变更摘要：
  - 为 A2A 逻辑定义补齐带绑定保护和二次确认的 Console 删除入口，并通过真实浏览器清理遗留定义。
  - 安装固定版本 Metrics Server，使用 APIService Available 与 `kubectl top node/pod` 证明资源指标链路可用。
  - 通过 Console 上传并回读自包含 Skill ZIP；修复 MySQL DSN 缺少 `parseTime=true` 导致的时间字段扫描错误。

## [2026-08-19] deploy | ACK Demo 同步 Console 验收修复

- 更新页面：kubernetes-production-sop、todo、log。
- 变更摘要：
  - 首次升级因构建版本超过实例字段长度导致自注册失败，安全回滚后修复自注册版本边界并增加字段级错误上下文。
  - 发布带 SBOM/provenance 的 amd64/arm64 GHCR digest，以 Helm revision 8 部署到用户指定 ACK context。
  - 验证 Pod 零重启、公网 Console/API 200、PVC/MySQL 数据保留、Limiter Endpoint、OTel 写入回读和 ACK Metrics API。

## [2026-08-20] fix | Console 认证态与授权拒绝分离

- 更新页面：auth-system、index、todo、lessons、log。
- 变更摘要：
  - 修复前端把普通 HTTP 401 与 `401001` 资源无权当成 Session 失效并跳回登录页的问题。
  - Console Proxy 只对 `401003/401004` 清除 HttpOnly Cookie，并让无效 Token 使用明确的 `401004` 认证错误码。
  - 发布双架构 GHCR digest 并升级 ACK Helm revision 9；以有效普通用户、无效 Token 和受控浏览器三层回归验证边界。

## [2026-10-02] ingest | 配置中心统一模板与 values 及协议兼容方案

- 新增统一模型 ADR 与 Apollo/Nacos/Consul 官方协议核查基线，记录用户确认方向、协议 profile、原子发布、渲染、写入限制、监听游标、迁移顺序和验收矩阵。
- 对照当前源码区分原生组合 Watch 已实现与兼容订阅未接入，记录 Apollo/Nacos 模板读取及协议映射缺口；未发现 Consul server adapter，按新增能力规划。
- 保留 Pole 原生 SDK 渲染合同，为第三方协议建议服务端最终内容交付；现有多文件共享 values 和协议地址保持迁移可追溯。
- 同步配置中心、旧渲染 ADR、任务记录和渐进导航；本轮仅设计文档，无业务代码修改、数据迁移、提交、推送或部署。

## [2026-10-02] ingest | 澄清发布定义与按客户端标签渲染

- 根据用户澄清，原子发布固定模板、默认 values 与标签取值规则；同一发布版本允许产生不同客户端结果，撤回把最终渲染内容视为唯一发布产物的假设。
- 区分版本投放和版本内取值，补齐有效 values、结果 revision、缓存隔离、标签变化重新订阅/解析、输入相关校验与预览。
- 保留原生 SDK 本地渲染与第三方协议服务端内容交付的分工；Consul 基础 KV 保持统一 key 视图，协议不能提供的标签不宣称透明兼容。
- 更新方案、功能页、研究中的设计建议、任务、索引和既有 lesson；只更新文档，没有实现或运行时行为变更。

## [2026-10-02] ingest | 补齐多个灰度版本并存的统一配置方案

- 明确同一配置环境为一个正式版与多个灰度版的生效集合，按客户端标签和稳定优先级唯一选版本，再在该版本内解析有效 values；禁止跨版本混用模板、默认值或取值规则。
- 补齐定向停止/替换/回滚、灰度转正式草稿及正式发布不自动停止其他灰度的合同；增加生效集合 revision、缓存与监听重新选择、重叠受众预览及三灰度并存验收。
- 用当前源码确认普通文件与模板环境版本的排序差异，更新既有多灰度 ADR 的历史状态边界并保留迁移等价要求。
- 本轮只更新方案及相关导航/记录，未修改业务代码或执行发布部署。

## [2026-10-02] review | 完整复核统一配置模型与兼容方案

- 对固定文档快照执行 Standards / Spec 双轴审查，修正删除作用域、literal 灰度写入与 CAS 目标、Consul 固定视图事务物化、缓存完整集合切换、灰度回滚受众转移及 Apollo 草稿生效边界。
- 补明地址 tombstone、明确缺失/拒绝与 last-known-good、跨出口校验、无 CAS 请求覆盖及过期游标重查；公开 spec 仍收敛为配置草稿、统一发布和解析结果。
- 两路独立复核确认上述问题在设计层关闭；目标文档检查通过，全库旧格式 lint 基线不变。未修改业务代码、执行迁移或运行 SDK E2E，具体客户端矩阵与能力协商留在 P0 定稿。

## [2026-10-02] review | 统一配置方案第二轮边界审查

- 固定修订后快照，重新推演原生 SDK、条件写与删除重建；核实 Nacos casMd5 无法隔离同内容重建、Consul 完整条目不变时保留 ModifyIndex，两项协议合同仍需修订。
- 另确认 literal 迁移缺少旧 Pole SDK 原始内容交付例外；独立调用当前渲染器，复现 ADR double-mustache 示例语法失败，triple-mustache 对照成功。
- 将 1 项 P1、3 项 P2 记录到任务 Review 和待修订清单。本轮仅审查并记录，不修改方案正文或业务代码，不执行协议 E2E、提交或部署。

## [2026-10-02] refine | 取消 Pole 历史兼容约束并简化渲染链路

- 根据用户明确指示，取消旧 SDK、旧 wire/API、绑定发布、旧灰度排序及存量迁移的设计约束；撤销第二轮审查的旧 SDK 兼容 P1。
- 新方案建议原生 SDK 与外部客户端统一消费服务端按标签渲染的结果，spec/control-plane/Console/SDK 按单一模型实现；发布权威仍为模板、values 与规则定义。
- 保留 Apollo/Nacos/Consul 外部协议需求，修订 Nacos MD5 CAS 存在期保证和 Consul 无变化 PUT 索引行为，并修正 triple-mustache 示例；同步历史 ADR 入口、功能页、索引、任务和 lesson。
- 本轮仅更新方案，不修改业务代码、不迁移或清理数据、不提交或部署。

## [2026-10-02] review | 无历史兼容约束的新配置模型复审

- 固定服务端统一渲染方案快照，独立复核外部协议一致性和原生 SDK 上下文/响应时序；不重新引入旧 Pole 模型与迁移要求。
- 协议一致性轴未确认新增问题；Spec 轴确认 1 项 P2：原生 SDK 缺少旧请求/订阅回调失效与响应接纳顺序合同，停止灰度或切换标签后可能被迟到响应覆盖。记录最小修正建议与逆序验收场景，未增加发布对象。
- 本次只更新任务 Review、待办和导航/日志，方案正文保持被审快照，未执行实现、SDK E2E、提交或部署。

## [2026-10-02] refine | 补齐原生配置 SDK 的响应接纳合同

- 根据用户确认，将迟到响应 P2 写入统一模型 ADR 第 4.1 节：同配置串行协调、本地上下文/请求世代、观察游标读取下限及应用通知接纳规则。
- 存在/缺失结果与 observation_cursor 取自同一一致视图；拒绝按当前请求身份处理，不要求披露资源游标；通知位置与已接纳位置分开，覆盖重读失败、断线恢复、日志换代和旧回调失效。
- 同步原生读取合同、版本表、实施顺序、验收矩阵、功能页、导航与任务记录。补充停止灰度后旧响应、标签 A→B→A、旧缺失/拒绝、删除重建、通知先到但读取失败等场景；仅更新设计，未修改业务代码或执行 SDK E2E。

## 相关页面

- [[index]]
- [[schema]]


## [2026-10-02] implement | 统一配置新 spec 与核心首批落地

- 新增 pole.config.v2、Go/Rust 生成合同与统一配置核心；原子发布完整 definition、多灰度选择、标签 values、服务端渲染、生命周期 CAS 和读取/监听游标。
- 接入 MySQL 主库一致快照、不可变 release 行、事务事件及显式恢复换代；原生 HTTP/gRPC 复用资源鉴权。管理历史采用有界元数据分页与单版本详情，HTTP Watch 预留响应写入预算。
- 核心、存储与入口回归/race、定向 vet、Go/Rust 契约检查通过；真实隔离 MySQL 验证并发、事件失败回滚及 34 个大版本；默认 gRPC 接收上限下验证 9 个大版本分页读取。临时测试数据库与容器已清理。
- 同步 ADR 第 10.1 节、配置中心功能页、任务 Review 与导航；SDK、Console、地址生命周期和外部适配继续按后续阶段推进。未提交、发布、部署或迁移；临时 modfile 用于相邻 spec 联调，正式依赖版本尚待发布。
- 本机 golangci-lint 缺失；spec 旧测试全量 vet copylock 基线保留；知识库结构检查无新增诊断，未修订无关旧格式。


## [2026-10-02] refine | 配置契约直接沿用 v1

- 用户确认尚未完整对外发布，不另起配置协议版本。将此前新增的配置契约收回 pole.config.v1、api/v1/config、Rust pole::config::v1 和 /config/v1，旧本任务版本产物清理；协议文档改为 specification/CONFIG.md。上述实现日志保留当时记录，以本条为最新版本决定。
- HTTP 合并为唯一配置 WebService，新 11 个 JSON 动作与分组入口同 root；gRPC 不注册旧版本别名。生产容器分派与开关测试、生成客户端测试、六包 race、核心/存储回归、定向 vet 和本地主程序构建通过。
- 同步 ADR、功能页、任务 Review、索引和 lessons；未改变字段/编号或发布语义，也未修改第三方协议与限流自身的版本。知识库无新增 lint 诊断。
- 无提交、推送、部署；无真实业务数据操作。


## 2026-10-02

- 操作：update。
- 变更摘要：按用户纠正，配置管理仅走 HTTP；config.proto 只保留消息，现有 v1.ConfigGRPC 仅声明 Discover/Resolve/Watch，删除并行 ConfigService 与管理 RPC。同步 Go/Rust 生成代码、服务注册与 client/config 开关；将管理历史容量验证迁至 HTTP。
- 更新页面：adr-config-template-values-unification、config-center、tasks/todo、tasks/lessons、_meta/index、technical/adr/index；相邻 specification 的 CONFIG.md、README 与任务记录同步。
- 验证：Go/Rust spec 测试、定向 vet、Rust check/fmt、control-plane 六包 race、核心/存储普通回归和主程序构建通过；HTTP 管理发布后可用生成客户端读取/监听，管理 RPC 不可调用。知识库 lint 无新增诊断；既有 spec 全量 vet 与 golangci-lint 缺失限制保留。未提交或部署。


## 2026-10-02

- 操作：update。
- 变更摘要：按用户要求，原生配置 gRPC 收敛为唯一 Discover，在双向流上逐次请求/逐次响应；直接调用统一选版渲染核心，无请求不发送。删除重复 Resolve/Watch RPC 与原生旧读取转换，HTTP 管理和长轮询保持原合同。
- 更新页面：统一配置 ADR、config-center、tasks/todo、tasks/lessons 和两级索引；相邻 specification 同步 service、Go/Rust 生成代码、CONFIG.md、README 与合同测试。
- 验证：协议 Go/Rust 测试、定向 vet、Rust check/fmt、control-plane 核心/存储回归和主程序构建通过；入口六包 race 通过，包含重复请求、无主动发送、逐帧授权、灰度/下线重读、取消/deadline/EOF 和生产流包装。既有 spec 全量 vet copylock 与 golangci-lint 缺失限制保留；SDK 刷新/缓存实现仍为后续工作。未提交、推送或部署。

- 配置跨语言命名补正：grpc_config_api.proto 的 Java 外层类名改为 PoleConfigGRPCService，Rust proto 副本和 Go 描述符同步。配置 Go 契约测试、Rust 构建及临时 Java 生成检查通过；不改版权声明或其他模块命名。

- Polaris 命名只读审计：确认服务发现 Java 选项、限流真实 wire 包名、冗余 gRPC 方法路径、启动 YAML/metadata、认证头与日志路径仍有旧名。旧辅助脚本制品名不一致列为候选，未确认当前发布包使用；版权头不列为清理项。细节与证据见 tasks/todo；本轮未改运行代码。


## 2026-10-03 统一配置 v1 实现与旧链路清理

- 当前 ADR 和配置功能页以 sibling specification 最新标准为权威，明确原生 Discover/Preview 下发模板与有效 values、SDK/控制台本地渲染及显式兼容转换。
- 记录独立内部聚合、不可变发布、主库事务游标、业务环境权限和 namespace→clock→state 锁序；旧配置文件/模板/values/binding/watch/cache/store 及无生产者指标移除。
- 更新协议 profile、Console 与 Workbench 接线和验收边界；旧模板/多灰度 ADR 标为 deprecated 并指向当前决策。相关导航、存储/缓存/API 描述同步。
- 本轮验证和缺口见 tasks/todo.md；没有提交、推送、部署或共享业务数据清理。


## 2026-10-03 配置管理 HTML 交互原型

- 按用户要求先提供独立 HTML demo，将配置列表、草稿编辑、标签预览、整份发布、多灰度生效和历史回滚按管理任务组织；隐藏常规操作中的协议 JSON 和手填版本标识。
- 原型位于 web/console/output/configuration-demo/index.html，本地示例数据仅存在内存；未接真实 API，未修改正式前后端、spec，未提交、推送或部署。
- Playwright 主流程及补充状态共 31 项检查通过；验证 1440px/390px 布局并检查截图。修复窄屏 grid 被配置列表撑宽、选择替换灰度后条件与优先级未同步，以及仅灰度/无生效版本的误导文案。
- 更新 tasks/todo、tasks/lessons 与导航摘要；原型设计待用户审阅，不能代替真实服务协议或业务验收。


## 2026-10-03 配置原型对齐现有控制台设计

- 用户指出独立 HTML 与现有页面风格未对齐。按当前 AppLayout/Menu/Header、ResourceLayout、Fluent token 与 EditorDrawer 修正原型外壳、导航层级、品牌资产、图标、密度、按钮和抽屉尺寸；保留单份草稿、标签预览和多灰度交互。
- 浏览器实测 232px/64px 侧栏、24px 页面边距、32px 按钮、560px/960px 抽屉和 390px 安全边距；执行草稿/线上预览隔离、标签选版和新增灰度，桌面及窄窗口无页面横向溢出，所查日志无 error/warn。
- 当前正式控制台入口与 file: 标签不可由浏览器工具访问；采用当前源码为对齐基准，验证并刷新已有 HTTP 原型，不宣称正式控制台同屏验收。修改仍只限 HTML 原型和任务/经验/导航记录。


## 2026-10-03 共享模板与环境取值归属修正

- 用户指出模板应跨环境共享，具体 values 归属环境，并允许只发布到部分环境。核对最新 spec，确认当前环境内完整草稿模型缺少共享管理源层。
- 独立 HTML 改为共享模板/变量声明、环境取值/规则、选择目标环境发布；每个环境冻结自己的完整版本并保留独立多灰度。修复多环境发布提示、目标检查前的修改顺序与灰度表单输入保留。
- 本轮浏览器验证共享模板保存不改变线上、目标环境使用新模板及自身 values、未选环境旧快照不变、多灰度独立、空值不继承及逐环境部分失败结果。桌面/390px 无页面横向溢出，发布底栏可达，所查运行日志为空。截图和具体数据在 tasks/todo 的本轮 Review。
- 更新当前 ADR、术语、功能摘要、任务/经验及索引；共享管理协议/CAS/冻结候选和正式实现仍待补。没有修改 spec、正式前后端，没有提交、推送或部署。


## 2026-10-10 模板编辑原型简化

- 用户截图指出模板页仍复杂，按正文编辑主任务取消首屏变量大表、模式/格式选择和重复说明；变量定义与模板设置改为清楚命名、带数量的按需入口。沿用现有 Fluent 外壳。
- 保留类型/必填编辑、新增/删除声明、正文引用删除保护、草稿状态、预览与目标环境发布；修正添加返回原变量视图和关闭后的焦点恢复。共享模板与环境 values 归属未改变。
- 本轮浏览器验证展开/编辑/关闭后的正文保留、草稿与线上隔离、prod 单环境发布保留其他环境及两个灰度，1440px/390px 无页面横向溢出；JS 语法通过，所查日志无 error/warn。具体证据和截图见 tasks/todo。
- 仅更新 HTML demo 与任务/经验/索引日志；正式协议和前后端未修改，无提交、推送或部署。已恢复示例、默认视口并刷新两个已有预览标签。

## [2026-11-04] task | 创建服务抽屉 audit / polish

- 更新tasks/todo与lessons：领域正文留白、纵向提示/错误去重、readonly环境和14组合验证；OrbStack revision6且数据保留。
- 输出限定HTML/JSON报告并加入整合报告；共享抽屉焦点恢复另列P2，未扩大源码改动。

## [2026-11-04] task | 真实操作页修整（浏览器复验待完成）

- 更新tasks/todo与lessons，记录领域留白、开关状态和关闭焦点修复；OrbStack revision7数据保留，11项契约通过。
- 当前Chrome实际复验因活动窗口变化待恢复，HTML/JSON报告明确区分实现、部署与未完成验收。

## [2026-11-05] task | 真实浏览器操作页验收与修整完成

- 用户改选独立受控浏览器并亲自登录；真实域名覆盖7类工作区、9种治理创建工作面，保存71张截图、77条布局/焦点记录。创建、编辑、别名和两向关联均使用经授权临时资源，7项资源全部清理，admin及系统资源保留。
- 补齐AI/实例抽屉留白、窄屏栅格、暗色只读身份、可访问名称、步骤文字与焦点；服务创建确认后连接实例下一步。真实发现并修复用户组普通更新重启用Token与用户/组新建状态残留，保留专用凭据启停接口和权限。
- 目标契约/lint、Go包测试、release/arm64镜像通过；TypeScript仍278条存量且无新增。OrbStack revision10、4Pod Ready/0restart、PVC UID/Secret哈希保留，Gateway域名首页匹配构建。未提交、推送或重置数据库。
- 主整合报告回链限定HTML/JSON报告；新增治理字段命名P1和3项P2建议保留，明确未测外部AI/SDK、发布生效、物理触屏/完整读屏及故障注入。

## [2026-11-05] task | 治理字段P1与三项P2关闭

- 按用户授权完成治理字段可访问名称、配置/治理上手指引、通知定位及旧chunk恢复。真实浏览器验证9类创建页及动态行、配置示例与JSON恢复、390/1440浅暗色通知和键盘、11→12真实跨版本恢复；最终revision13。
- 关联修复Mock响应头空行与重复键拒绝，保留有效Record和输入焦点；没有保存规则或配置。临时逻辑服务已清理。
- 8契约、目标lint、release/arm64构建通过；类型271条存量无新增。额外旧布局脚本仍有存量过时断言，未列通过。4Pod Ready/0restart、PVC/Secret不变、域名匹配构建；未提交推送。
- 更新tasks/todo、主报告及此前报告的后续关闭状态；完整证据在web/console/output/playwright/followup-fixes-2026-11-05。未将局部实测扩称全站AA或发布生效验收。


## [2026-11-05] task | 配置中心交互原型启动

- 用户要求先看对齐注册发现的操作 demo。已核对真实配置入口与 v1 九动作，沿用既定共享模板和环境取值归属，独立原型演示目录、环境发布与历史。新增管理能力明确标注待补，不修改正式流程或部署。

## [2026-11-05] task | 配置中心交互原型交付

- 独立React/Fluent原型完成目录→正文/环境取值→标签预览→指定环境发布→历史操作，保持既定共享归属。只读复核及真实受控浏览器验证后，修正严格标签、literal原样、变量重名覆盖、模态错误反馈、键盘焦点和短窗按钮可达性。
- 目标TypeScript/oxlint/独立构建通过；1440/390和短窗实测，内存临时资源刷新重置。目录、共享管理及多环境编排仍明确为待补能力；未改正式页面、部署、提交或推送。

## [2026-11-05] task | 配置正文与变量取值分任务页签

- 根据用户“正文和key同屏拥挤”的反馈，将独立demo改为配置内容/变量与取值两个页签，移除双栏常驻，保留共用草稿和发布动作。真实浏览器验证双向草稿保留、默认宽度与390px布局，新的截图与README已更新；正式页和部署保留。

## [2026-11-05] task | 条件取值就地整合

- 用户确认条件应在变量值处配置。独立demo将默认值和条件值改为同一变量表格，条件标签/优先级就地编辑，删除独立规则管理区；保留整组条件优先级及默认回落语义。
- 真实浏览器验证增删改、留空继承、非法条件和渲染预览，390px表格内部滚动可达；目标类型/lint/独立构建通过。证据和README更新，未改正式页面、业务数据或部署。

## [2026-11-05] task | 逐key可选取值目标澄清

- 用户提出条件组复用、各key关联组/匹配值与可选兜底，未命中且无兜底时key无value。已只读复核旧单winning规则、缺失与空字符串、模板引用和required合同；条件组是匹配维度还是条件集已提问，依赖此决定的原型尚未修改。

## [2026-11-05] task | 条件组与逐key可选取值原型完成

- 用户确认条件组为整组AND判断。独立demo改用环境条件组、逐key有序关联和可选兜底，未命中且无兜底不返回key，空串保持存在。组名可改且ID稳定，引用中禁止删除；上下移提供原型建议的首个命中策略。正文引用缺值仍明确报错，发布保守保持静态完整性边界，不将新取值目标当正式运行时。
- 12项模型场景、目标类型/lint/独立构建通过；真实浏览器验证命中/缺项/空串/排序/组引用、环境隔离、部分失败重试、冻结历史和复制恢复，390/1440实际渲染无文档横溢出。独立只读复核后补具名控件。新schema统一到发布快照；ADR/功能页/README及证据同步，未改后端/正式页面/部署，示例内存重置，未提交推送。

## [2026-11-05] task | 取值工作区密度修整

- 根据用户控制区占用与留白反馈，将全部常驻表单改为紧凑key清单和单行展开编辑，条件组摘要/就地编辑，环境和管理工具栏合并，预览入口及双列结果收紧；解析模型与发布边界保留。
- 实测默认key行高171→46px，1440×1000可同时呈现条件组/取值/预览；390px表格与容器同宽，无文档横溢出。编辑切换保留草稿、组管理及键盘焦点通过；类型/lint/独立构建通过，新页error为空。README与compact-values-*证据同步，未改后端/正式页/部署或提交推送。

## [2026-11-05] task | 独立预览弹窗与条件组选取

- 按用户要求移除常驻预览，仅保留入口；弹窗以渲染为主、取值来源为次。已有组多选生成标签，自定义按需补充，冲突/失效组/非法CSV拒绝；预览环境不改编辑环境，输入变化隐藏旧快照。
- 16模型、目标类型/lint/独立构建通过；真实浏览器覆盖多选、补充冲突、空选缺项、未保存草稿、环境隔离、缺值/空内容，390×600关闭可达且焦点恢复。独立只读复核完成，README和preview-dialog-*同步；未接真实API、部署、提交或推送。

## [2026-11-05] task | 后缀推断与自适应语法编辑器

- 按用户要求移除格式选择，由单一后缀推断统一编辑语言/预览/快照；接入本地Monaco和本地workers，按需加载编辑器chunk。正文填充桌面剩余高度，窄/短窗自然滚动，格式只读展示。
- 12格式案例、目标类型/lint/独立构建和只读复核通过；实测YAML/JSON/Properties/XML多色token、输入/切tab/预览保留、JSON worker格式化及390/1440/1920/短窗布局。常规底栏距视口底17px，无横向溢出；新页error为空。编辑器chunk约3.24MB保留大包提示，正式打包优化未验收。临时内存文件已刷新清除；仅demo与记录，未改正式协议、部署、提交或推送。

## [2026-11-05] task | 发布记录与客户端查看交互补齐

- 独立原型补齐显式正式/新增灰度入口，多个灰度条件/priority/状态展示，正文/变量/草稿对比与候选可读审阅、灰度快照转正式。客户端读取示例支持环境/搜索/历史版本与渲染来源，明确当前无正式枚举API、watch或下发回执。
- 浏览器实测正式替换保留灰度、多环境失败重试、灰度转正式/停止重读/历史记录不自动改写及390×600可达；8选版+16取值+12后缀、目标类型/lint/构建通过，新页error为空。只读复核问题已修，内存操作已重置；未改正式源码/API/OrbStack，不提交推送。

## [2026-11-05] task | 灰度受众按客户端标签澄清

- 用户明确灰度范围是客户端请求标签，标签组只作快捷勾选/填写来源；独立demo增加标签勾选、自定义名值与最终AND条件，组带入后不关联组、可独立增删。变量取值条件组保持在版本内的value选择职责。
- 13模型/目标类型/lint/构建及真实浏览器验证通过；组带入独立取消、冲突拒绝、空范围阻止和tier客户端实际选版已验证。窄屏按钮可达，新页error为空，临时内存发布重置；未部署/提交推送。

## [2026-11-05] task | 发布抽屉与环境Tab审阅

- 按用户标注将发布居中弹窗改为右侧Fluent抽屉，多环境候选改环境Tab并保留正文/变量子Tab；标题操作栏固定，正文滚动，取消/空环境及冻结候选均有明确查看状态。
- 真实浏览器验证环境值隔离、Tab保留、范围取消、空范围、部分失败重试与焦点恢复；修复全局header污染和窄屏100vw裁切，390×600底栏可达。类型/lint/构建通过，新页error为空；临时内存发布重置，仅demo与记录，不部署提交推送。

## [2026-11-05] task | 发布Step与原式客户端标签条件行

- 用户要求环境→类型→灰度标签→确认，参考HEAD旧配置和当前共享组件，原型恢复IP/ID/语言、匹配方式和值条件行，当前仅完全匹配。前序无发布动作，冻结后只审阅/结果；示例请求显式提供CLIENT_*，不冒充自动采集。
- 14选版/标签模型、目标类型/lint/构建、只读复核与真实浏览器验证通过；空/重复条件拒绝、回退保留、失败重试、目标客户端选版、390×600布局与切步顶部/focus已验证。内存临时发布重置，当前第3步未确认；仅demo和记录，未部署提交推送。

## [2026-11-05] task | 灰度匹配能力回退核对

- 用户指出正则/范围原有能力，核实统一v1将Audience简化为标签map且只做相等比较；原匹配类型未迁入。记录为能力回退，不以当前实现限制代替产品要求。
- 本轮仅原因核对和待办记录，未恢复协议/匹配器、未改Demo或部署提交推送。


## [2026-11-05] task | 正式配置六种匹配恢复

- 同步协议/生成合同、统一后端匹配器和正式Console条件编辑；旧精确labels保留，六操作AND、缺项负条件、空值/RE2/集合/int64和保守歧义语义明确。补齐空预览提示与窄屏历史溢出。
- 全量Go/race、协议合同、隔离MySQL持久性、真实HTTP/gRPC及正式源浏览器命中验证通过；目标前端lint/边界/构建通过，全量tsc既有错误保留。新增config-six-match验收页并更新ADR/功能/任务/lessons/索引；临时资源清理，未部署提交推送。


## [2026-11-05] task | 灰度标签在发布类型下内联

- 按浏览器标注移除原型独立客户端标签步骤，灰度标签与优先级在类型选项下展开；进入确认前按类型校验、切换返回保留输入。同步README/ADR/tasks/lessons/索引。
- 真实浏览器桌面与390×600、空条件/正式跳过/保留输入验证通过；目标类型/lint/独立构建通过，未确认发布，未部署提交推送。


## [2026-11-05] task | 交互原型同步六种匹配

- 18084灰度编辑器补六类型、正则/集合成员/int64范围输入及JSON匹配测试；结构化条件随发布/历史/模拟读取保存，旧exact示例兼容。独立Go/WASM执行正式regexp/strconv语义，桥接/产物/重建脚本入原型目录。
- 22选版模型、WASM边界、目标类型/lint/构建及构建冷启动通过；真实浏览器六类19项、组合AND、非法拒绝、发布读取和390×600验证通过，修复复合输入id/命名。临时发布与18087服务清理，README/ADR/验收/tasks/lessons/索引同步；不提交推送部署。


## [2026-11-05] task | 正式配置页面与真实管理模型落地

- 新增共享正文独立CAS/目录/绑定环境存储与13管理动作，逐参数条件组取值保留旧winning兼容；源创建旧草稿保护与namespace资源锁/计数补齐。正式页面模块化接真实API，内联六匹配/独立预览/历史对比与原子晋升/多环境发布抽屉，标签键改同高只读Input。
- Go全量/race、真实隔离MySQL、协议与前端检查通过；candidate镜像config-formal-20261105-a部署Helm revision14，manifest仅镜像变化、域名路由仍正常。真实账号浏览器验收待重新登录后继续，未提交推送。


## [2026-11-05] task | 正式候选 revision15 与真实登录交接

- 真实MySQL并发补测修复共享正文/首次草稿间隙锁竞态，以共同clock锁序列化；三个交错与相关包回归通过，专用容器清理。
- 新镜像config-formal-20261105-b升级Helm revision15，路由健康与六表存在已核实。独立浏览器回到登录页，已请求用户重新登录；真实业务操作验收待续，没有声明完成。未读取更改密码，无新增业务临时资源，未提交推送。

## [2026-11-06] update | 统一配置真实登录页面验收进展

- 更新浏览器与六匹配验收、统一配置 ADR、任务 review 和 lessons；区分真实 Preview/发布证据与最终镜像、清理待办。


## [2026-11-06] task | 正式配置最终浏览器验收与本地部署

- 修复真实保存Any元数据回写、组展开状态、原生确认、发布刷新结果丢失；补历史回滚目标ID与共享字段复制保护，历史条件与时间可读化。
- 真实登录页面验证编辑/跨环境局部恢复/Preview/六匹配/正式与多灰度/快照/回滚/原子晋升/复制保护/390px，通过最终镜像f再次发布与截图；Helm revision18 deployed、路由True/HTTP200、Pod1/1重启0。
- 精确清理模板1/绑定2/状态2/发布8/事件8和空测试环境，保留clock及既有资源；docs/review/索引同步。目标检查通过，全仓tsc/KG lint既有问题保留；外部SDK与客户端观测API不作完成声明。未提交推送。


## [2026-11-06] task | 两仓业务代码同步 develop

- 用户明确扩大提交范围至两个仓库当前全部业务改动，排除本地产物/临时文件。通过显式清单隔离候选，不混入截图、Demo输出、缓存或本地consoleassets；保留源码占位文件。
- specification 已推送 f1d3771e58700ebf38b7e05cfa103fea664fb926；控制面将协议绑定到该远端固定伪版本，测试向量从Go实际依赖定位，避免隐式相邻工作区依赖。
