---
type: Architecture Decision
status: deprecated
title: ADR：配置模板与环境组合版本客户端渲染
tags: [adr, config, template, namespace, sdk, gray-release]
links: [config-center, namespace, cache-layer, api-servers, adr-config-template-labels-sensitive-values, adr-environment-promotion-topology, adr-config-template-values-unification]
updated: 2026-10-03
legacy_source_count: 20
sources:
  - resource: adr-config-template-values-unification.md
    title: 当前统一配置 v1 决策与实现

---

# ADR：配置模板与环境组合版本客户端渲染

## 状态

Deprecated。本文保留旧配置模式的历史设计与证据；旧代码已移除，不能作为当前实现或 SDK 验收状态。现行定义、发布、Discover、本地渲染与兼容转换边界见[统一配置 v1 ADR](adr-config-template-values-unification.md)。

## 背景

配置中心已经存在 `ConfigFileTemplate`、`ConfigFileTemplateStore`、MySQL 表和管理接口，但当前模板只是一个全局按名称唯一的静态内容记录：

- 没有模板版本和不可变发布快照。
- 没有参数 Schema。
- 没有 Namespace 归属或 Namespace Value。
- 没有模板与配置文件的绑定关系。
- 配置发布、客户端查询和 Watch 链路都不会执行模板渲染。

现有配置文件发布会把草稿内容原样复制到 `ConfigFileRelease.Content`，客户端通过 active normal/gray release 获取最终内容。该模型无法表达“同一个模板在不同 Namespace 使用不同 Value”，也无法让 Value 按客户端标签独立灰度。

## 决策

### 1. 环境配置版本是唯一可生效发布对象

配置模板跨环境共享稳定逻辑身份（ID、名称、说明和标签），但可执行的内容、格式、参数 Schema 和引擎草稿必须按 Namespace 隔离。模板定义本身不作为用户可见的独立发布对象；每次发布以“当前环境模板草稿 + 当前环境 Value 草稿”为输入，生成一个
不可变的 `EnvironmentConfigRelease`：

```text
EnvironmentConfigRelease(namespace, template_id, version)
  ├── TemplateSnapshot
  ├── ValueSnapshot
  ├── normal | gray + audience rules
  └── active state + audit metadata
```

`TemplateSnapshot` 是内部不可变组成和审计证据。发布时当前 Namespace 模板定义未变化就复用已有快照，内容、格式、Schema
或引擎变化时才在同一事务中创建新快照。每次环境发布都会新增环境版本；全量和灰度都绑定完整的模板与
Value 快照，不能只灰度 Value。

Value 按 `Namespace + Template` 建立独立聚合：

```text
NamespaceTemplateValues(namespace, template_id)
```

每个聚合维护 Value 草稿、一个生效全量环境版本、多个生效灰度环境版本、发布历史和回滚来源。不同模板
之间不共享无边界的环境全局变量池；后续如需复用公共值，应通过显式、受控的 Value Set 引用另行设计。

### 2. 配置文件绑定逻辑模板，环境版本决定实际快照

配置文件可以保持现有纯文本模式，也可以选择模板模式。模板模式只要求绑定全局逻辑 `template_id`；运行时
根据配置文件所属环境选择该模板当前命中的完整环境版本，再从版本中读取绑定的 TemplateSnapshot 与
ValueSnapshot。

旧 `template_release_id` 字段继续保留为创建时快照与兼容审计信息，但不再覆盖当前环境版本的生效选择。
因此模板更新只能通过目标环境的一次组合发布生效，不需要逐个配置文件重新固定模板快照，也不会自动影响
其它环境。

因此：

- 同一环境内绑定该逻辑模板的配置文件看到一致的环境组合版本。
- 环境版本可以准确回答“当时使用了哪个模板快照和 Value 快照”。
- 回滚通过创建一条引用历史快照的新环境版本完成，不重新激活或改写旧记录。

### 3. 服务端匹配灰度，客户端只负责渲染

客户端继续提交 Namespace、配置分组、文件名和客户端标签。服务端负责：

1. 查找配置文件当前生效的逻辑模板绑定。
2. 按环境、客户端标签、灰度优先级和全量回退规则，选择唯一命中的 `EnvironmentConfigRelease`。
3. 读取该环境版本绑定的 TemplateSnapshot 与 ValueSnapshot。
4. 返回一个完整、可校验的渲染快照。

SDK 不接收全部灰度规则，不实现灰度优先级或冲突处理。灰度匹配保持在控制面，避免 Go、Java、Rust 和其它语言 SDK 出现不一致的规则解释。

### 4. SDK 本地缓存并确定性渲染

服务端返回：

```text
RenderSnapshot {
  template_release_id
  value_release_id
  template_content
  parameter_schema
  values
  format
  engine_version
  revision
}
```

其中：

```text
revision = hash(
  config_binding_release_id,
  template_release_id,
  value_release_id,
  engine_version
)
```

SDK 只在收到完整快照并通过校验后执行本地渲染，成功后原子替换本地缓存。没有新 revision 时直接复用已渲染结果，不在每次业务读取时重新请求或重新渲染。新快照下载、校验或渲染失败时，继续使用 last-known-good，并上报可观测错误。

服务端渲染结果不是客户端运行时的权威配置。SDK 必须根据快照独立渲染，不能直接把服务端预览内容作为配置应用。

### 5. 服务端提供预览与参考校验

服务端提供独立的模板渲染预览接口，供 Console、发布校验和跨语言一致性检查使用：

```text
RenderPreview {
  rendered_content
  format
  template_release_id
  value_release_id
  engine
  engine_version
  rendered_sha256
  diagnostics[]
}
```

预览接口使用与 SDK 相同的 `pole-mustache-v1` 规范，但其输出只承担以下职责：

- 在模板、Value 或绑定发布前展示最终格式。
- 校验缺失参数、参数类型和目标配置格式。
- 生成给定模板与 Value 组合的参考 `rendered_sha256`。
- 作为跨语言 SDK 契约测试的参考结果。

`RenderPreview.code/info` 使用项目统一 API 错误码表达鉴权、请求参数和系统失败。
`diagnostics` 仅表达模板语法、参数 Schema、Value 类型和目标格式诊断；权限失败不能伪装成
渲染诊断字符串。

运行时 `RenderSnapshot` 可以携带服务端为同一组合计算的 `expected_rendered_sha256`。SDK 本地渲染后必须比较哈希；不一致表示模板引擎实现或序列化行为偏离规范，应拒绝应用新快照并继续使用 last-known-good。

服务端可以按以下组合缓存预览和参考哈希，不需要为每个客户端重复渲染：

```text
template_release_id
+ value_release_id
+ engine_version
```

#### Console 工作区信息架构

Console 以两个稳定层次表达配置模板，不能用流程步骤条与页签重复切换同一组内容：

1. 摘要头只保留模板身份、状态、资源路径和主操作，不持续展开完整元信息。
2. 模板内容、参数 Schema、环境 Value、版本管理和基本信息使用同一套一级页签；名称、目标格式、说明和
   标签统一归入最后的基本信息页签，`TemplateRelease`、`ValueRelease` 和组合预览归入版本管理。

配置分组只保留一个配置清单，不再以同级 Tab 把“配置文件”和“配置模板”拆成两个资源目录。纯文本与
模板是 `ConfigFile` 的两种内容来源：创建配置时先选择“直接文本”或“配置模板”，模板类型再显式选择
Template 与不可变 Template Release；创建完成后两类文件统一出现在同一棵文件树中，并以类型标识区分。
当前 Namespace 的逻辑模板也必须作为同级叶子节点直接出现在这棵清单中，并用尾部“环境模板”标签区分；不能再增加模板
目录层级。点击后保持当前配置分组路由与左侧清单不变，只将右侧画布切换为原模板的内容、Schema、环境
Value 和版本管理任务；完整模板路由仅保留为历史深链兼容入口。合并的是发现入口与页面工作区，不是存储
模型：模板不能被复制到当前分组，Namespace/分组文件也不能被提升为全局资产。存量文件继续按
`config_type` 归入统一清单，存量模板直接读取原 `config_file_template` 数据，不做自动迁移或重建。

模板嵌入态与配置文件详情使用同一套右侧资源详情视觉骨架：摘要头统一承载名称、轻量状态标签、结构化资源
路径和主操作；查看态模板信息使用两列字段网格，不以禁用表单控件模拟只读信息；一级页签与正文编辑器使用
相同间距、扁平内容画布和底部状态栏。模板使用内容、Schema、环境 Value、版本管理、基本信息五个一级
任务；完整元信息不再固定占用每个任务的纵向空间。新建模板默认进入基本信息，确保名称和格式等必填项
仍是明确的首要入口。视觉一致性不改变领域职责和数据模型。

版本管理使用一张“环境配置版本”表展示当前草稿组合与真实发布历史。每个历史行同时显示环境版本、模板
快照、Value 快照、全量/灰度类型、生效状态和发布说明；不能再用左右两张表暗示已发布组件可以任意拼装。
草稿组合作为首行明确展示并默认选中。Console 通过唯一的“渲染预览”动作调用 `RenderPreview`，预览当前
模板草稿 + 当前环境 Value 草稿，或某个历史环境版本原本绑定的两个快照，展示格式化结果、
`rendered_sha256` 和 diagnostics。预览只读取页面状态，不保存草稿、不创建版本，也不能替代运行时
`RenderSnapshot`。

环境 Value 表单必须根据参数 Schema 即时校验必填项和标量类型。INTEGER、DECIMAL 的前端规则与服务端
canonical 文本规则保持一致；字段错误以内联方式反馈，并同时阻止保存草稿与发布。前端校验用于缩短反馈
路径，服务端仍是最终契约权威，不能因为 Console 已校验而移除 API 校验。

环境 Value 编辑页还必须支持发布前就地预览：使用当前模板草稿与页面内尚未保存的 Value 调用服务端参考
预览，展示格式化结果、哈希和 diagnostics。模板或 Value 变化后立即废弃旧预览，避免把过期结果误认为
当前输入。该入口解决当前编辑任务；版本管理负责当前草稿组合与历史环境版本检查，两者共享结果组件但不
共享状态。

模板定义页只提供模板草稿编辑和保存，不能再独立发布。环境 Value 页提供 Value 草稿保存、当前组合预览和
唯一的“发布环境配置”入口。确认弹窗同时审阅环境、模板草稿、Value 参数数、全量/灰度类型、灰度规则和
发布说明。服务端必须从当前 Namespace 持久化模板草稿生成或复用 TemplateSnapshot，并与请求中的当前 Value 在同一事务中
创建 EnvironmentConfigRelease；不能信任客户端提交的模板内容、格式或 Schema 副本。

### 6. Watch 面向组合 Revision

模板绑定切换、正式 Value 发布、客户端命中的灰度 Value 变化或模板引擎版本变化，都会产生新的组合 revision。不同标签客户端可能命中不同 Value Release，因此同一配置文件可以观察到不同 revision。

Watch 通知只表示该客户端可见的组合快照发生变化；SDK 收到通知后拉取完整 `RenderSnapshot`。不能把模板和 Value 作为两个无原子关系的 Watch 事件分别应用，否则客户端可能短暂组合新模板与旧 Value。

### 7. 使用 `pole-mustache-v1` 严格 Profile

不采用完整 Go `text/template` 作为多语言协议。Go template 的 dot、pipeline、函数映射、方法调用、变量作用域和控制结构具有 Go 特有语义，且安全模型假定模板作者可信，难以由不同语言 SDK 完整、稳定地重现。

首期模板引擎标识固定为：

```text
engine = pole-mustache
engine_version = v1
```

`pole-mustache-v1` 基于 Mustache 1.3 core 的变量查找语义，但只开放确定性的配置变量替换：

```text
{{{database.host}}}
{{{database.port}}}
```

Profile 规则：

- 只允许 triple-mustache 原始变量和 dotted name。
- delimiter 固定为 `{{{` 与 `}}}`，不允许动态修改。
- 缺失参数、未知参数、类型不匹配一律报错，不允许替换为空字符串。
- 首期 Value 只支持 string、boolean、integer 和 decimal 标量。
- boolean 固定输出 `true` / `false`，integer 使用十进制，decimal 使用规范化十进制文本，不允许科学计数法。
- UTF-8 输入输出，保留模板原有换行，不执行空白裁剪。
- `rendered_sha256` 对最终 UTF-8 字节直接计算 SHA-256，不执行换行符或 Unicode 归一化。
- SDK 渲染后必须执行目标格式校验，并与 `expected_rendered_sha256` 比较。

明确禁用：

- double-mustache HTML escape。
- section、inverted section 和循环。
- partial、parent、block 和继承。
- lambda、helper、函数、pipeline 和方法调用。
- delimiter change、动态模板引用和递归。
- 文件读取、网络请求、系统时间、随机数和进程环境变量。

缺失必填 Value、类型错误、未知引擎版本或渲染后格式校验失败时，SDK 必须拒绝应用新快照并保留 last-known-good，不能以空字符串静默替换。

所有 SDK 必须运行同一份语言无关测试向量，覆盖变量查找、标量序列化、Unicode、换行、错误输入、格式校验和哈希。跨语言一致性由规范与测试向量保证，不能只依赖各语言选择了名字相同的 Mustache 库。

## 领域模型

```text
ConfigTemplateIdentity (global)
  └── NamespaceConfigTemplateDraft[]
        └── TemplateSnapshot[] (immutable, content-reused)
ConfigFileTemplateBinding
  └── template_id (logical binding)
NamespaceTemplateValues (namespace + template)
  └── ValueDraft

EnvironmentConfigRelease (immutable)
  ├── TemplateSnapshot
  ├── ValueSnapshot
  ├── Normal (at most one active)
  └── Gray[] (matched by server)

matched EnvironmentConfigRelease
  └── RenderSnapshot
          └── SDK local rendered config
```

建议的存储资源：

| 资源 | 关键身份 | 说明 |
|---|---|---|
| `config_template` | `template_id`、全局逻辑名称 | 只保存模板身份、说明和标签 |
| `namespace_config_template_draft` | `namespace + template_id` | Namespace 隔离的内容、格式、Schema 和引擎草稿 |
| `config_template_release` | `template_release_id` | 内部不可变模板快照，内容一致时跨环境版本复用 |
| `namespace_template_values` | `namespace + template_id` | Namespace 下的 Value 草稿聚合 |
| `namespace_template_value_release` | `value_release_id` | 环境组合版本；绑定模板快照、Value 快照和发布状态 |
| `config_file_template_binding_release` | 配置文件身份 + binding release | 逻辑模板绑定；旧快照字段用于兼容审计 |

具体 DDL 和 specification 字段在实现阶段确定，但身份和不可变性不得退化。

## 模块接口

客户端查询链路应通过一个深模块集中隐藏绑定解析、灰度匹配、快照装配和 revision 计算：

```go
type ConfigSnapshotResolver interface {
    Resolve(ctx context.Context, req ResolveRequest) (*RenderSnapshot, error)
}
```

调用方只需提供配置文件身份、客户端标签和已持有 revision，不需要了解模板表、Value 表、灰度规则或缓存布局。管理端的模板、Value 和绑定 CRUD/发布接口与该解析接口分离。

服务端预览使用独立接口：

```go
type ConfigTemplatePreviewer interface {
    Preview(ctx context.Context, req PreviewRequest) (*RenderPreview, error)
}
```

`Preview` 是服务端参考实现 seam，不改变 `Resolve` 返回快照、SDK 本地渲染的运行时职责。

## 安全与审计

- 参数 Schema 中标记为敏感的 Value 必须复用或扩展配置中心加密存储能力。
- 日志、Watch 事件、错误详情和审计摘要不得记录敏感明文。
- SDK 本地缓存敏感渲染结果时应支持进程内缓存和可选加密落盘策略。
- 每次绑定和环境组合发布都记录操作者、原因、源 revision 和目标 release；回滚新增记录并保存
  `source_release_id`，不重新激活旧记录。
- 管理端预览必须按目标 Namespace 和选定 Value Release 执行，且遵守对应资源权限。

## 兼容策略

- 现有纯文本配置文件保持原发布、灰度、Watch 和客户端读取行为。
- 模板模式按配置文件显式启用，不对存量文件自动转换。
- 现有静态 `ConfigFileTemplate` 在迁移时保留为全局逻辑身份和迁移证据；Namespace 草稿优先从该环境当前 active/last TemplateSnapshot 初始化，只在没有环境快照时使用旧全局草稿。详细迁移规则见 [[adr-environment-promotion-topology]]。
- 旧 `TemplateRelease` 与 `ValueRelease` 数据无需重建：已有 Value Release 已持有 `template_release_id`，可直接
  投影为历史 EnvironmentConfigRelease；旧模板发布接口保留兼容但 Console 不再暴露。
- 存量配置文件绑定继续读取原 `template_release_id` 作为审计信息；运行时改为按 `template_id + namespace`
  选择当前环境组合版本。
- 不支持模板渲染的新 SDK 请求模板配置时应收到明确的不兼容错误；服务端不能偷偷降级为未渲染模板文本。
- 新能力需要先在 specification 声明快照、引擎版本、组合 revision 和能力协商字段，再更新各语言 SDK。

## 被否决的方案

### 模板与 Value 分两次独立发布

运行时只消费两者组合后的完整快照，独立发布会暴露无法运行的中间状态，并要求用户手工维护先后顺序。
模板快照与 Value 快照可以独立存储和审计，但只能通过一次环境组合发布原子创建或复用并切换生效状态。

### 发布时渲染并固化唯一内容

Value 支持按客户端标签灰度，发布阶段不存在面向所有客户端的唯一渲染结果。

### 服务端在每次配置读取时渲染

会把模板引擎放到高频读取路径，增加延迟和缓存组合数量，也弱化客户端离线使用与 last-known-good 能力。

### SDK 直接使用服务端预览内容

会把预览接口变成事实上的运行时配置读取接口，削弱客户端本地渲染、离线恢复和跨语言实现校验能力。服务端预览只提供参考结果与哈希。

### 完整 Go template 作为多语言语法

Go 特有的 pipeline、函数和控制语义无法保证被其它语言 SDK 等价实现，也扩大了模板执行的安全面。

### 把全部 Value 灰度规则下发给 SDK

会迫使所有语言 SDK 复制灰度优先级、冲突处理和正式回退规则，难以保证行为一致。

### Namespace 全局变量池

不同模板会产生同名冲突、权限扩散和无法收敛的变量生命周期，也难以按模板 Schema 完成校验。

### 模板新版本自动影响所有绑定配置

一次模板发布可能跨 Namespace 批量改变客户端配置，破坏显式发布、审计和回滚边界。

### 全局共享一份可编辑模板草稿

dev/lane 中未发布的开发会改变 pre/pro 的预览和下一次发布基线。全局只能共享逻辑身份，可执行草稿必须按 Namespace 隔离并沿晋升拓扑流转。

## 分期实施

1. [x] 在 specification 定义模板、参数 Schema、Value、绑定、`RenderSnapshot` 和 `RenderPreview` 契约。
2. [x] 实现模板与 Value 的不可变 release、正式/灰度发布和存储接口。
3. [x] 发布 `pole-mustache-v1` 语法规范及 Go/Rust wire 契约测试。
4. [x] 实现 `ConfigSnapshotResolver`，复用现有服务端客户端标签匹配逻辑。
5. [x] 实现 `ConfigTemplatePreviewer` 参考渲染、格式校验和期望哈希。
6. [ ] 扩展 Watch，使其面向客户端可见的组合 revision。
7. [ ] 在首批 SDK 实现最小模板引擎、本地缓存、哈希校验、原子替换和 last-known-good。
8. [x] Console 增加模板版本、参数 Schema、Namespace Value、灰度发布、渲染预览和显式切换界面。
9. [ ] 建立服务端参考实现与各语言 SDK 共享渲染向量的契约测试矩阵。

当前后端实现将模板绑定固化在每个 `ConfigFileRelease` 中。服务端先按客户端标签选出
普通或灰度配置发布，再从该发布快照读取 binding；不能查询文件级“最新 active binding”
代替发布快照，否则普通与灰度发布固定不同模板版本时会发生串版。

## 验收标准

- 同一模板可被多个 Namespace 使用，各自维护独立 Value 和发布历史。
- 同一逻辑模板在多个 Namespace 中维护独立定义草稿，dev/lane 编辑不改变 pre/pro 的草稿、预览或发布基线。
- 同一 Namespace 下不同模板的 Value 不会互相污染。
- 服务端根据客户端标签只返回唯一命中的 Value Release。
- SDK 不包含灰度规则匹配实现。
- 服务端预览返回参考渲染内容、诊断和 `rendered_sha256`，但不是客户端运行时权威配置。
- Go、Java、Rust 等 SDK 对同一 `pole-mustache-v1` 测试向量输出完全相同。
- SDK 本地渲染哈希必须与服务端参考哈希一致。
- 模板或 Value 变化通过一个组合 revision 原子生效。
- 新快照渲染失败时客户端继续使用上一份成功结果。
- 模板升级必须通过目标 Namespace 的原子环境组合发布；只影响该环境中绑定同一逻辑模板的配置文件，不跨环境自动生效。
- 纯文本配置文件和旧客户端保持原有行为。

## 规范依据

- [Go `text/template` 官方文档](https://pkg.go.dev/text/template)
- [Mustache 1.3 官方规范](https://mustache.github.io/mustache.5.html)
- [Mustache 多语言实现列表](https://mustache.github.io/)

## 相关页面

- [[config-center]]
- [[namespace]]
- [[cache-layer]]
- [[api-servers]]
- [[adr-config-template-labels-sensitive-values]]
- [[adr-environment-promotion-topology]]

- [统一模板与 values 及多协议兼容方案](adr-config-template-values-unification.md)
