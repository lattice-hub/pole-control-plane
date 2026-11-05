---
title: 领域术语表
tags: [business, terminology]
links: [domain-models, business-rules, auth-system, skill-marketplace, adr-skill-marketplace-federation, adr-console-agent-resource-workbench, adr-system-configuration-control-plane, adr-rpc-first-governance-scope, adr-service-contract-reporting-and-visualization, adr-logical-service-environment-binding, adr-system-namespace-kind, adr-ai-resource-environment-binding, adr-config-template-client-rendering, adr-environment-promotion-topology, adr-config-template-values-unification]
updated: 2026-10-03
sources: 0
---

# 领域术语表

| 中文术语 | English | 含义 |
|----------|---------|------|
| 命名空间 | Namespace | Pole 的顶层运行边界，先按 Kind 区分业务环境与内部系统空间 |
| 业务环境 | Business Namespace | `kind=BUSINESS` 的 Namespace；参与服务、配置与治理资源的跨环境聚合 |
| Pole 系统空间 | System Namespace | `kind=SYSTEM` 的内部管理面空间；继承当前 Pole 部署阶段，不属于业务发布环境 |
| 环境晋升拓扑 | Environment Promotion Topology | 用户在全局环境空间中定义的 BUSINESS Namespace 有向无环图；边表示允许的晋升路径，适用于所有可晋升资源域 |
| 环境晋升边 | Environment Promotion Edge | 连接两个 baseline BUSINESS Namespace 的有向关系；同时定义正式来源版本、审批、验证、资源域与冲突门禁 |
| lane 基线绑定 | Lane Base Binding | lane Namespace 到唯一 baseline Namespace 的分叉归属关系；不是晋升 DAG 边，回归时按资源分叉版本三方合并 |
| 环境晋升拓扑版本 | Environment Topology Revision | 全局晋升 DAG 及全部边策略的不可变发布快照；晋升变更集创建时固定引用，不被后续拓扑修改篡改 |
| 环境晋升变更集 | Environment Promotion Change Set | 引用来源环境已发布资源版本及其分叉基线的可审计变更集；在目标环境生成候选变更而不直接生效 |
| 环境晋升 Bundle | Environment Promotion Bundle | 通过全部预检、审批和验证后不可拆分发布的跨资源目标版本集；保证控制面期望状态原子，通过回执跟踪运行时最终收敛 |
| 晋升适配器 | Promotion Adapter | 资源域接入全局晋升流程的契约；负责导出正式版本、差异与三方合并、生成目标候选和执行资源域校验 |
| 晋升资源预检 | Promotion Resource Preflight | Adapter 按稳定逻辑资源 ID 对齐来源与目标后给出的 CREATE、UPDATE、CONFLICT 或 UNSUPPORTED 结果；决定资源能否进入目标候选 |
| 泳道回归 | Lane Merge to Base | 将 lane 选中资源相对分叉基线的变更三方合并到 base 候选变更；冲突解决并发布 base 后才可继续向下游晋升 |
| 逻辑服务 | Logical Service | 由控制面稳定 ID 标识的跨环境业务服务；聚合多个显式关联的环境服务，SDK 不感知该 ID |
| 环境服务 | Service Environment | 由 `namespace + runtimeServiceName` 标识的运行时服务记录，独立承载实例、契约和运行状态 |
| 实例 | Instance | 服务的运行节点，包含 host、port、协议、健康状态、隔离状态和元数据 |
| 服务契约 | Service Contract | 协议原始定义与可检索接口投影的组合；原文是事实来源，投影用于发现、治理和展示 |
| Dubbo 元数据快照 | Dubbo Metadata Snapshot | Dubbo 按 application 与 metadata revision 聚合的接口配置和方法定义 |
| Dubbo 服务映射 | Dubbo Service Mapping | Dubbo interface 到 provider application 的一对多关系，用于应用级服务发现 |
| 配置模板 | Config Template | 跨环境共享的配置正文、引擎、输出格式及变量声明；允许只向选择的环境发布 |
| 模板变量声明 | Template Parameter | 共享模板中变量的名称、类型与必填约束，不包含环境的具体取值 |
| 环境取值 | Environment Values | 某个环境的默认变量值和客户端标签取值规则；不同环境独立维护 |
| 模板修订 | Template Revision | 共享模板的已保存编辑版本；改变模板修订不会自动更新任何环境的生效配置 |
| 环境配置发布 | Config Release | 在某个环境冻结完整模板与该环境 values/规则的不可变版本；同一环境可同时存在正式版与多个灰度版 |
| 服务调用 | Service Invocation | 一次具有调用方、被调方、接口、请求属性和结果状态的 HTTP/RPC 交互 |
| 服务治理能力 | Service Governance Capability | 作用于服务调用的路由、限流、鉴权、镜像、Mock、熔断等能力 |
| 治理规则 | Governance Rule | 声明对哪些服务调用、在何种条件下施加一个服务治理效果的期望策略聚合根 |
| 执行点 | Enforcement Point | 真正执行服务治理规则的 SDK、Sidecar 或 Gateway |
| 能力档案 | Capability Profile | 执行点声明的服务治理规则类型、匹配能力、动作版本和可选特性集合 |
| 应用回执 | Apply Receipt | 执行点对某个服务治理 Bundle 返回的已应用、拒绝或部分应用状态 |
| 发布版本 | Release | 配置或治理规则发布时形成的快照版本 |
| 灰度发布 | Gray Release | 面向部分客户端、标签或条件生效的发布方式 |
| 泳道组 | Lane Group | 一组泳道路由规则的聚合根 |
| MCP 逻辑定义 | MCP Definition | 由控制面稳定 ID 标识的跨环境 MCP 能力身份，不进入 MCP 协议或数据面寻址 |
| MCP 环境实例 | MCP Deployment | 某个 Namespace 中可调用的 MCP Server 登记，承载 Endpoint、Tool 快照和后端绑定 |
| Agent 逻辑定义 | Agent Definition | 由控制面稳定 ID 标识的跨环境 Agent 能力身份，不等同于某个环境的 Agent Card |
| Agent 环境实例 | Agent Deployment | 某个 Namespace 中可调用的 A2A Agent 登记，承载 Endpoint、Card、技能和后端绑定 |
| Pole Agent | Pole Agent | Console 提供的一等对话主体，内部使用模型和 MCP 工具操作 Pole，写操作必须等待人工确认 |
| Pole Agent Profile | Pole Agent Profile | Pole Agent 的版本化行为配置，包含 Prompt、模型 profile、MCP 工具集和运行限制，不包含密钥明文 |
| Agent 调用作用域 | Agent Namespace Scope | 单次 Pole Agent Turn 可读取或修改的显式 Namespace 集合；普通写操作只能作用于一个环境 |
| Skill 发布者 | Skill Publisher | 拥有稳定 slug、成员授权、信任状态和版本化签名公钥的 Marketplace 发布身份；不等同于 Namespace |
| Skill | Agent Skill | 以 `publisher/name` 定位的可发布逻辑资产；不等同于 MCP Tool 或 A2A Agent Card 中的能力声明 |
| Skill Release | Skill Release | 精确绑定 SemVer、Bundle SHA-256、来源和审核证据的不可变发布版本 |
| Skill Bundle | Skill Bundle | 以根 `SKILL.md` 为必需入口、可携带 scripts/references/assets 的 Agent Skills 开放格式目录快照 |
| Registry 来源 | Registry Source | 可周期同步的 Pole、Git 或 HTTP Skill 目录来源，拥有信任等级、同步水位和错误状态 |
| Bundle 存储 | Bundle Store | 按内容摘要保存、读取、检查和回收已验证 Bundle 的存储 seam；默认 Adapter 为 MySQL BLOB |
| 启动配置 | Bootstrap Configuration | 让 Console 能够启动并定位外部依赖的最小不可热更新配置，包括配置源、凭证引用和紧急开关 |
| Console 系统设置 | Console System Settings | 面向管理员的类型化运行配置页面；底层使用 Pole 配置中心的草稿、发布、历史和回滚能力 |
| 系统配置定义 | Setting Definition | 描述系统设置的类型、默认值、校验、敏感级别、归属组件、兼容版本和生效方式 |
| 期望配置快照 | Desired Snapshot | 控制面当前希望目标实例采用的已发布系统配置版本 |
| 有效配置快照 | Effective Snapshot | 某个实例实际使用的最终配置及其默认值、静态文件或动态覆盖来源 |
| 配置应用回执 | Apply Receipt | 实例对某个系统配置版本返回的已应用、拒绝或等待重启状态 |
| 协议兼容默认环境映射 | Protocol Compatibility Namespace Mapping | 将兼容协议的缺省别名解析为当前 Pole BUSINESS Namespace 的动态配置；它不是独立环境，也不参与环境晋升拓扑 |
| 系统角色 | Built-in Role | Pole 固定提供的 `admin`、`resource-reader`、`resource-writer` 权限集合；角色定义不可增删改，只允许维护用户和用户组成员关系 |
| 自定义角色 | Custom Role | 管理员按职责创建的角色；可维护角色定义、用户/用户组成员及资源/API 权限，并可删除 |

## 相关页面

- [[domain-models]]
- [[business-rules]]
- [[auth-system]]
- [[skill-marketplace]]
- [[adr-skill-marketplace-federation]]
- [[adr-console-agent-resource-workbench]]
- [[adr-system-configuration-control-plane]]
- [[adr-rpc-first-governance-scope]]
- [[adr-service-contract-reporting-and-visualization]]
- [[adr-logical-service-environment-binding]]
- [[adr-system-namespace-kind]]
- [[adr-ai-resource-environment-binding]]
- [[adr-config-template-client-rendering]]
- [[adr-environment-promotion-topology]]
- [[adr-config-template-values-unification]]
