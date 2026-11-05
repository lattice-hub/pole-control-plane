---
type: Architecture Decision
title: ADR：配置中心统一模板与 values 及多协议兼容
description: 统一发布完整配置定义，原生 Discover 下发模板与有效值，SDK 和控制台本地渲染，外部协议按发布开关转换。
status: stable
tags: [adr, config, template, compatibility, apollo, nacos, consul]
links: [config-six-match, config-center, config-protocol-compatibility-baseline, terminology]
updated: 2026-11-05
sources:
  - resource: ../../../../specification/CONFIG.md
    title: 最新统一配置 v1 合同
  - resource: ../../../../specification/CONFIG_TEMPLATE.md
    title: 本地渲染和静态检查合同
  - resource: ../../../pkg/config/unified/service.go
    title: 草稿 CAS 与原子完整发布
  - resource: ../../../pkg/config/unified/discover.go
    title: 原生选版和有效 values 解析
  - resource: ../../../apis/store/unified_config.go
    title: 独立持久聚合与一致快照接口
  - resource: ../../../plugin/store/mysql/unified_config.go
    title: 主库事务、不可变发布行与持久游标
  - resource: ../../../pkg/config/unified_access.go
    title: 真实身份和业务环境授权
  - resource: ../../../pkg/compatibility/config/read.go
    title: 兼容转换边界与逐次重新授权
  - resource: ../../../plugin/apiserver/consulserver/kv.go
    title: 只读 Consul KV 配置消费 profile
---

# ADR：配置中心统一模板与 values 及多协议兼容

## 决策

配置尚未完整对外发布，直接收敛 v1。用户已取消旧配置模式和历史兼容约束；旧文件、独立模板发布、Value/Binding 发布及其缓存、监听、存储接口不再作为实现入口。不建设 v2、旧消息转发或旧数据迁移。

一份 `ConfigDefinition` 包含模板正文、引擎、输出格式、参数、默认 values、标签取值规则与协议转换开关。`ConfigRelease` 原子冻结完整定义及投放条件。发布的是定义，客户端请求时先选唯一发布，再选择该发布内的有效 values；同一发布可服务不同标签结果。

原生 control-plane 只做静态检查、选版和选值，`Discover` 及 `Preview` 返回 `RenderSnapshot`，正文仍是原模板。SDK 与控制台本地渲染并校验最终格式。Apollo/Nacos/Consul 适配器在部署和选中发布同时启用协议转换后，才在适配层渲染最终字节。配置运行时只有 `Discover`，没有独立原生 `Watch` 或 `Resolve`。

## 跨环境管理源修正（2026-10-03，设计已纠正，协议与正式实现待补）

用户审阅 HTML 原型时纠正了管理归属：模板正文、引擎、输出格式与变量声明跨环境共享；变量的具体 values、标签取值规则归属各环境。变量声明中的名称、类型及必填约束不属于某个环境的 values。模板允许只发布到选择的环境，不能由共享草稿的修改自动更新所有环境。

管理层应维护共享模板草稿及各环境取值草稿。发布指定模板修订时，分别结合目标环境的 values/规则，冻结该环境的完整 `ConfigRelease`；未选择环境继续使用原来不可变的完整发布。每个环境独立拥有正式版、多灰度、版本序号及生效集合。模板修订与环境取值修订是编辑来源的版本证据，不恢复独立对外生效的 TemplateRelease、ValueRelease 或 BindingRelease。

多环境操作应明确逐环境结果，不能仅凭一次 UI 提交宣称跨环境事务原子性；需要重试时仅处理失败目标，并固定原候选内容。共享模板编辑权限与各目标环境编辑/发布权限必须分别检查，标签仍不替代授权。历史完整版本的回滚仅作用于目标环境；复制历史环境 values 到草稿不覆盖共享模板。

当前 specification 的 `ConfigDraft` 和已实现 HTTP 管理仍按 `ConfigKey(environment, group, name)` 保存整份定义，缺少共享模板与环境取值的独立管理协议、目录及相应 CAS/候选冻结合同。下面的协议与持久化章节描述的是当前实现，不作为新管理源设计已经完成的证据。本轮只修正独立 HTML 原型和设计记录；正式协议、后端及 React 控制台尚未调整。运行时唯一完整发布、Discover 逐请求选版选值和本地渲染合同保持成立。

## 条件组与逐 key 取值修正（2026-11-05，目标模型，正式协议未实现）

用户确认条件组是一整组判断，例如“国内订单客户端”包含 `region=cn AND app=order`，不是可枚举匹配维度。多个key复用条件组，各key分别关联组与具体值；兜底可不配置，无命中且无兜底时，解析结果中不包含这个key。显式空字符串是存在的合法值，不等于缺失。

独立demo用 `ConditionGroup`、逐key有序 `ValueBinding[]` 和可选 `fallback` 表达。条件组不持有变量值；绑定通过稳定组ID引用，重命名保留关联，引用中的组不允许直接删除。组和取值暂按当前配置/环境管理，跨文件或跨环境共享组不在本轮范围。原型建议同key多组命中时按关联顺序取第一条，并提供上下移；该冲突策略是本轮建议，不作为用户已经确认的正式协议。

每个key独立解析，可以在同一请求中分别取不同组的值，不再用旧模型的全配置单一winning规则。环境发布冻结组定义、绑定顺序及兜底是否存在；历史复制仅恢复该环境的取值管理源，不覆盖共享正文。预览分别展示逐key值/来源/缺项和正文渲染；不得把缺项自动补为空串或删除正文行。

当前正式静态检查要求所有被正文引用或required的变量有值，客户端渲染也拒绝缺项。demo允许未引用的key无兜底且不进入结果；对正文引用且无兜底的key，发布保守阻止并解释其可能缺值。是否允许这类定义发布并按请求失败，需要后续明确调整完整性合同；不能仅凭新UI默认改变运行时错误策略。

该原型阶段正式v1仍为 `default_values + ValueRule` 单winning；现已由下述“正式页面落地”新增逐变量模型，旧winning数据保持原语义。SDK仍只接收已选values的RenderSnapshot。`RenderSnapshot.values`仍可用map缺项表达无值，但快照修订和错误策略必须一致。独立原型和模型检查不代表正式API或业务发布验收。

## 文件后缀与正文编辑（2026-11-05，原型展示约定）

用户要求前端不再手动选择输出格式，由文件名后缀识别。独立demo使用单一推断函数供编辑语言、渲染预览及发布快照元数据复用：yaml/yml→YAML，json→JSON，properties/ini、xml与文本使用对应展示；未知或无后缀按纯文本，不猜测模糊的conf语法。

正文使用本地加载的Monaco，语言随文件切换、编辑区填充剩余视口，窄/短窗口自然滚动。JSON模板未渲染前允许占位符，静态诊断不作为最终格式校验；预览仍按渲染结果检查。正式v1的显式format字段和语言/格式白名单未修改，后续落地需要统一前端推断、服务端验证与冻结版本合同；本轮高亮演示不代表新增格式的真实消费/SDK验收。

## 领域边界与发布操作

身份为 `ConfigKey(environment, group, name)`。每个身份独立拥有草稿、不可变发布历史、生效集合修订号和存在期。环境必须是有效业务 Namespace；标签只用于投放和取值，不代替凭据或资源授权。

- 草稿保存按 `expected_draft_revision` 整份 CAS，包括首次保存的零值期望；草稿不改变线上配置。
- 发布同时比较草稿修订号与生效集合修订号；一个正式版本、最多 128 个灰度版本可同时生效。
- 灰度标签支持六种结构化条件并按 AND 匹配（旧 labels map 保留精确语义）；先取最小 priority，同 priority 取最大 sequence。未命中灰度才选择正式版本。
- 值规则只在已选发布内选择，最小 priority 唯一获胜；同优先级可能重叠的规则静态拒绝，不隐式叠加多个分支。
- 正式发布保留其他灰度；停止单个版本、停止全部灰度、下线分别是明确管理动作。
- 复制历史版本到草稿只修改草稿；回滚创建新的不可变发布。灰度回滚明确替换某个 active 灰度，不能通过更新历史快照改变线上内容。
- 下线保留草稿及历史。环境内持久聚合（包括下线和仅草稿记录）阻止删除环境。

`ConfigState` 是管理视图，不是数据库聚合。内部 `UnifiedConfigAggregate` 保存完整历史成员和 active 标记；管理响应只含草稿、有界发布摘要页、完整 active IDs 和按需选中的完整发布。运行时只读取 active 发布正文，避免历史大小决定客户端响应预算。

## 协议

所有响应共用 `model.Response`、公共 `Code` 和 `Any`。失败不带旧快照或敏感模板正文。成功数据类型分别为管理 `ConfigState`、预览 `RenderSnapshot`、客户端 `ConfigResult`。

| 入口 | 行为 |
| --- | --- |
| `POST /config/v1/draft` | CAS 保存完整草稿 |
| `POST /config/v1/state` | 读取草稿、发布摘要分页及可选单版本定义 |
| `POST /config/v1/preview` | 静态检查并按 labels 选择 values，由控制台本地渲染 |
| `POST /config/v1/publish` | 发布完整定义并原子调整生效集合 |
| `POST /config/v1/stop` | 停止指定 active 发布 |
| `POST /config/v1/stop-all-gray` | 停止全部 active 灰度 |
| `POST /config/v1/copy-release-to-draft` | 将指定历史定义复制到草稿 |
| `POST /config/v1/rollback` | 创建并激活历史定义的新发布 |
| `POST /config/v1/offline` | 下线配置，保留历史 |
| `/v1.ConfigGRPC/Discover` | 双向流逐次请求、逐次完整响应，无主动 push |

HTTP 使用 protobuf JSON：bytes 为 base64，uint64 为十进制字符串。请求体上限 4 MiB。旧 `/files`、`/groups`、`/templates`、原生 `/watch`、`/resolve`、`/promote` 和旧管理 RPC 已移除。MCP 只暴露 `get_config_state` 管理检查；Workbench 使用 state/draft 并携带 CAS，保留完整定义，只替换正文。

控制面不建设配置目录枚举接口；控制台通过完整 ConfigKey 进入统一编辑和发布视图。客户端同一流可以反复发 Discover，每帧重新授权、返回完整结果；领域错误后仍可处理下一帧。

## 渲染合同

引擎只有 `literal` 和 `pole-mustache-v1`。literal 保持任意字节及空内容，不解析变量。Mustache 仅允许无空白的 `{{{name}}}`；参数名按点分段校验。标量使用带 ValueType 的规范字符串，整数与小数不能经过 float64 转换。

控制面校验每个默认分支及单规则覆盖分支，不在 Publish、Preview、Discover 中替换模板或验证最终输出格式。原生本地消费及兼容渲染使用同一规范。定义上限 1 MiB，输入和输出各 512 KiB，最多 128 个参数或值规则、64 个客户端标签。管理摘要分页默认 20、最多 100，额外受 256 KiB 元数据预算约束。

## 持久化、游标与权限

MySQL 使用 `unified_config_state`、`unified_config_release`、`unified_config_clock`、`unified_config_event`。聚合头为内部 JSON；不可变发布正文是独立 protobuf 行。`ConfigState` 的公开投影不能作为持久格式使用。

状态与观察游标从主库同一事务快照读取。写事务先对有效业务 Namespace 加共享锁，再锁 clock 和 state，与环境删除的排他锁互斥；即使授权缓存尚未刷新，也不能在已删除环境里保存配置。事务原子提交聚合、生效集合、不可变发布、审计事件及游标；任何回调或 SQL 失败整体回滚。草稿保存不推进运行时观察位置。当前配置写操作共用全局时钟锁，包括草稿保存，代价是不同配置的写入串行，规模化吞吐尚未压测。

管理响应虽然已分页和限制大小，当前管理读写仍会加载完整发布历史；长期高频发布的内存与事务耗时，以及事件表持续增长的保留策略尚未通过压力测试。运行时读取仅加载 active 发布，不受历史正文累积影响。

`Cursor(scope, epoch, position)` 持久保存；连接重建不重新生成 epoch。Discover 的 `min_cursor` 不允许静默降级：scope/epoch 不一致返回 `ConfigCursorReset`，观察位置不足返回 `ConfigUnavailable`。SDK 仍需实现上下文世代、迟到响应拒绝和已应用状态协调，不能按 release ID 排序。

认证必须先取得真实 principal，再判断客户端或控制台资源权限；关闭权限检查开关仍不接受无凭据请求。读取、管理检查、编辑和预览分别走公共权限检查。Staffname、labels 和传入 principal 不能替代认证。已删除主体是明确拒绝；认证缓存或存储故障是不可用，不能伪装成拒绝或配置缺失。

## 外部协议 profile

适配器每次读取都调用同一 `DiscoverForProtocol`：先重新认证、选版，再检查该发布的 compatibility；选中灰度未启用协议时拒绝，不回落到已启用的正式版本。当前没有渲染缓存，关闭转换、下线和权限撤销不会沿用旧成功结果。单次读取预算 5 秒，外部等待最多 60 秒；超时终检可再占用一次读取预算。

| 协议 | 已实现配置消费范围 | 明确边界 |
| --- | --- | --- |
| Apollo | properties/JSON/raw 读取、releaseKey、notifications | cluster→dataCenter→default 仅在明确 ABSENT 时尝试下一个定位；releaseKey 为 snapshot_revision，通知 ID 为持久 position；单整数不能完整表达跨 epoch 重同步 |
| Nacos | v1/v2 读取与监听，按实际渲染字节计算 MD5；v2 connection 兴趣有上限并随取消/Stop 回收 | 客户端发布、导入、删除返回 405；v2 lastModified 未知为 0；JSON string 不交付非 UTF-8 |
| Consul | 只读 `GET /v1/kv/group/name`，dc→environment、raw 字节、JSON base64 Value、index 等待 | 写入、锁、Session、Flags 修改、prefix/recurse 和 stale/cached 不支持；`label.*` 是 Pole 扩展，不视为官方 SDK 标签能力 |

Consul 查询 index 为持久观察 position 加一，避免零索引；ModifyIndex 采用该观察位置，是配置消费 profile，不是完整 KV 条目修改历史。无关提交允许提前唤醒，旧 index 高于恢复后的 index 时立即返回新基线；附带 `X-Pole-Config-Epoch`。JSON CreateIndex 为未知值 0，不伪造真实创建索引。官方 KV 的 raw/base64/missing 与 blocking 接口依据见 [KV 文档](https://developer.hashicorp.com/consul/api-docs/kv) 和 [blocking 文档](https://developer.hashicorp.com/consul/api-docs/features/blocking)。

Consul 适配器默认不部署。需要显式加入 API Server 配置并打开 client/config，例如：

```yaml
- name: service-consul
  option:
    listenIP: "0.0.0.0"
    listenPort: 8500
    environment: prod
  api:
    client:
      enable: true
      include: [config]
```

外部 profile 的 handler、字节、错误、取消和规范向量验证，不等同于真实 Apollo/Nacos/Consul SDK 兼容验收。外部集群同步、三套管理控制台与全协议写入不在当前实现中。

## 清理与交付边界

旧配置领域模型、独立模板/values/binding/file 发布链、文件/分组/values 缓存、WatchCenter、旧 MySQL 实现及建表脚本、历史清理任务、旧 HTTP 管理、旧响应构造和无人上报的旧文件指标已移除。共享治理灰度缓存仍服务治理规则，不属于旧配置链。无关服务发现、认证、系统设置和 OTel 共享修改保留。

本次不执行 DROP、不迁移或清理共享业务数据库。较早实验实现把公开 protobuf ConfigState 当作聚合头，其数据不兼容新的内部 JSON 头；新实现需新的存储基线。旧配置表也不再被新代码读写。正式发布前需发布并固定对应 specification 版本，当前开发 go.mod 通过 sibling replace 使用最新协议。

验收记录在[本轮任务 Review](../../tasks/todo.md)。未提交、推送或部署，不把编译、隔离数据库回归或浏览器 fixture 当作真实凭据加数据库加外部 SDK 的业务验收。

## 配置原型的发布与客户端查看补齐（2026-11-05）

仅交互目标与示例，不改变正式协议：发布记录显式展示正式/灰度、状态、条件和priority，提供新增灰度、正式发布、停止及以选中灰度完整快照转正式的路径。该原型阶段的正式选版合同为灰度AND精确匹配（已由下述六匹配补齐）、priority升序、同级sequence降序，未命中退正式；正式发布保留灰度。转正式演示对应新建normal并停止指定gray，不将灰度快照直接改写，也不从编辑草稿取值。

历史版本和候选审阅拆为只读正文、可读变量取值与草稿对比，冻结每版本组定义/绑定/兜底。每key取值目标模型仍未进入正式v1。新增“客户端读取”是订阅查看设计示意，记录客户端标签、读取时选中版本和本地渲染。读取记录在演示中保持不变，需明确模拟重读才重新选版。版本详情从当前发布列表查询状态，不能把读取时active当作当前生效状态。

正式管理 `plugin/apiserver/httpserver/config/unified.go` 仅九个配置动作，当前没有客户端列表/读取观测API；服务发现订阅接口不适用于配置。原生Discover双向流按每帧请求返回完整结果，不是Watch/主动推送；本原型不声称在线订阅、已送达、消费确认或真实版本分布。落地客户端观测需另行明确采集、身份、留存、权限及新鲜度协议。

验证与截图参见独立demo README和任务review；正式页面/API、OrbStack不变，未提交推送。

### 用户澄清：灰度范围按客户端标签（2026-11-05）

灰度受众是客户端请求的标签条件，不是标签组。标签组可作为快捷填写来源，带入后只保存客户端标签键值对；用户可取消其中某个条件或添加自定义标签，不能留下运行时组关联。变量取值组则在选中发布后按key决定value，两类条件的操作入口、说明和快照归属保持分离。当前demo从示例读取记录给出标签勾选项，仍无真实客户端标签枚举API。

### 发布审阅的容器与导航（2026-11-05）

用户要求发布操作改为右侧抽屉，多个环境快照用Tab而非多层折叠查看。原型使用Fluent OverlayDrawer固定标题/操作栏，正文独立滚动；环境Tab只呈现所选目标，内部正文/变量页签随环境切换保留。无目标时不展示旧候选，冻结后仍可切环境审阅；不改变发布快照、逐环境结果或失败重试语义。此为独立demo交互实现，正式管理页面和协议不变。

### 发布Step与原式标签条件行（2026-11-05）

用户要求环境→类型→灰度标签→内容确认→发布结果。前序步骤只设置范围，最终确认才执行发布；回退保留输入，冻结后只查看候选/结果，失败重试保持原候选。环境Tab审阅保留，切步滚动归零并聚焦标题。

参考HEAD旧配置GrayRuleEditor/PublishForm和当前TrafficMatchConditionEditor，原型恢复客户端IP/ID/语言、匹配方式和值的条件行，并支持v1任意标签键的自定义行。当时只开放完全匹配AND；此限制已由下述正式协议/后端恢复覆盖，独立demo不作为正式链路证据。当前Discover不自动注入IP/ID/语言，CLIENT_*须由请求labels显式提供；示例请求加入这些字段用于演示，未改变真实采集/SDK/协议。

## 正式链路恢复六种匹配（2026-11-05，已实现）

`specification/api/v1/config/config.proto` 新增 `MatchType` 和 `LabelCondition(key,type,value)`；`Audience.conditions=2` 与旧 `labels=1` 按非空集合互斥。旧精确数据原样可读，新条件在灰度和 ValueRule 共用，键唯一、全部 AND。该变化不改变共享模板/逐key取值仍待落地的目标，也不增加客户端读取观测 API。

| 类型 | 语义 |
| --- | --- |
| EXACT | 字面相等，`*` 是字面值 |
| REGEX | Go/RE2 搜索；`^`/`$` 可限定全串 |
| NOT_EQUALS | 已有标签且字面不等 |
| IN / NOT_IN | 按逗号分隔的字面成员集合；不 trim，空成员和空格保留 |
| RANGE | `left~right`，signed int64 闭区间，支持正负号；请求也须能解析为 int64 |

缺标签始终不匹配，包括两种负条件；显式空字符串是存在的标签值。每个 audience 最多64条件，key/value 上限分别128/4096字节；非法 UTF-8、NUL/CR/LF、未知或零类型、重复键、非法正则/反向或溢出范围拒绝。请求标签继续使用既有128/1024字节键值限制。空 audience 不允许灰度或 ValueRule 使用。

正式匹配器 `pkg/config/unified/definition.go` 同时用于灰度选版与 Preview/Discover/兼容层选值；不存在前端用 JS 正则重做 RE2 的第二套正式算法。灰度排序保持 priority升序、sequence降序；同 priority 值规则只在可证明不相交时允许（有限 EXACT/IN 候选及不相交 RANGE），未知复杂交集保守拒绝。

正式 Console 的灰度和取值规则均可编辑六种条件，范围双边界、集合成员行、内置 CLIENT_IP/ID/LANGUAGE 与自定义键。CLIENT_* 仍要求调用方显式传入，不自动注入。后端 Preview 返回有效 values 后前端本地渲染；旧向量用的精确选择辅助函数遇到结构化条件显式拒绝，避免静默当无条件。proto3 省略的空 value 在编辑时恢复为空串；异常条件保留原 JSON 修正入口。合法空文件预览显示0字节。

完整条件随现有 protobuf 发布快照持久化，不增加 SQL 列。数据库回连、历史、替换/回滚已用隔离 MySQL 8.4.6 验证；HTTP/gRPC 六种命中/未命中/缺项、正式源页面浏览器及边界验证见[六匹配验收用例](../../quality/testcases/config-six-match.md)。浏览器身份/事务仓库使用隔离替身，HTTP/领域代码真实；不等同于真实凭据+SQL+外部SDK完整业务验收。Java/C#/C++ 构建未执行；Go/Rust/Node/Python 协议检查已执行。未部署 OrbStack、未提交推送。

### 原型修正：灰度标签在发布类型下就地配置（2026-11-05）

用户标注要求取消独立客户端标签步骤。18084独立原型现在固定环境→发布类型→内容确认→发布结果；选择灰度后在该选项下直接显示客户端标签条件和优先级。类型步骤进入确认前校验灰度范围，正式不受隐藏条件影响；切换类型与返回上一步保留输入。此修正仅调整原型任务路径，六匹配正式协议/后端能力不变。桌面/390×600浏览器与目标类型/lint/构建已验证，未部署提交推送。

### 独立原型同步六匹配（2026-11-05）

18084交互原型也已同步六种灰度条件。编辑器保持发布类型下内联，正则单表达式、集合成员行、signed-int64双边界，测试匹配接受string-map JSON。条件以key/type/value数组冻结到发布，不再把操作符编码回标签字符串；旧示例labels只作exact兼容。模拟客户端读取、灰度历史/回滚复用同一结构；正常发布和转正式不保留灰度条件。

为复用Go regexp/strconv语义，原型携带独立Go/WASM运行时及配套wasm_exec，未引入npm正则依赖；UI桥接不重做匹配。资源初始化先于示例选版，加载失败明确提示；Node检查、22项选版模型及浏览器单类型/组合/发布读取验证通过。约4.8MiB WASM仅在独立原型包，不改变正式Console或服务端部署。临时发布已重置；共享模板/逐key取值目标与真实客户端观测仍维持既有实现边界。

## 正式页面落地：共享正文、环境取值与文件目录（2026-11-05）

正式配置页迁至 `/configuration/files`，菜单为“配置中心 / 配置文件”，保留旧 `/configuration` 深链。共享管理身份为 `ConfigFileKey(group,name)`，运行时仍为 `ConfigKey(environment,group,name)`。共享正文 `ConfigFileTemplate` 只含正文、引擎、文件后缀推断格式、参数声明和兼容开关，单独revision/CAS；绑定业务环境，环境取值与发布历史仍属于各环境聚合。保存共享正文不会修改环境草稿或生效版本，Console以环境实际草稿为基线，保存时逐环境组成完整定义并执行独立CAS；共享源与环境正文不一致、缺少草稿时均显示待保存，不能冒充已发布或已同步。

共享源写入需对所有旧、新绑定环境逐次Manage授权；读取需绑定环境Inspect权限，目录按单授权环境分页。不允许移除已有绑定环境；新增绑定可由协议表达。首次建立共享源时不得覆盖同名旧草稿的不同共享字段；服务与SQL事务均检查。MySQL新增 `unified_config_file_template` 与 `unified_config_file_environment`，不DROP/迁移旧表；环境绑定写入与删除使用namespace锁并计入资源数量。

`ConfigDefinition.condition_groups=8` 与 `parameter_bindings=9` 是新环境取值模型：命名组含整组AND Audience，每参数binding按列表顺序选择第一个命中组，未命中只使用presence存在的fallback，没有fallback则该key缺项。空字符串Scalar仍存在。非空新模型禁止与 `default_values/value_rules` 混用；旧定义继续全配置winning规则，正式UI保留高级编辑，不静默改语义。被正文引用或required的参数沿用静态完整性要求：必须有兜底；未引用非required变量可无兜底。

管理HTTP在原九动作基础上增加 `/files`、`/file-template`、`/file-template-draft` 和 `/promote-gray` 共13动作。灰度转正式在同一事务内克隆已审阅active gray的完整快照，创建normal、停止源gray、保留其它gray和草稿，记录source_release_id并检查set修订；不通过前端复制到草稿冒充原子晋升。

正式Console按正文/变量/历史/接入任务分区；独立预览使用真实Preview返回values后本地渲染。正文编辑器本地打包Monaco/worker，模型路径按文件与环境隔离；内置标签键用32px只读Fluent Input与其它字段对齐。受众测试通过真实Preview的临时选值定义，不保存、不发布，不将原型Go/WASM带入正式包。发布抽屉四步：环境→类型（灰度条件内联）→逐环境候选Tab→结果。只使用真实已保存草稿；冻结草稿和set修订后逐环境双CAS提交，正常发布保留灰度，成功项不重试，冲突需重新审阅。

草稿仅在当前会话内存缓存，按登录用户和文件隔离；离开页面/后退再返回时先重新授权读取，保留编辑开始的CAS，避免缓存越权或覆盖他人修改。不同环境编辑保留，成功保存清缓存；局部失败保留待处理状态。没有真实客户端订阅/读取观测API，正式页不展示示例列表，也不宣称Watch或消费回执。

协议Go/Rust合同、Node/Python检查、新服务生命周期、隔离MySQL8.4重连/目录/CAS、Go全量/race、前端目标类型/lint/构建已执行。candidate已部署OrbStack Helm revision15（config-formal-20261105-b，含共享正文首次创建并发修复），仅镜像变更，Gateway仍 `lattice-hub.localhost`，真实账号浏览器验收进行中；最终证据见任务review，不把部署Ready等同于业务验收。未提交推送。

### 正式登录页面验收进展（2026-11-06）

此前隔离身份浏览器与 revision15 候选记录保留为当时证据。本轮已在真实登录的 `lattice-hub.localhost` 正式页面验证跨环境局部失败与恢复、真实 Preview、两环境正式发布、多个灰度并存、正常发布保留灰度、历史回滚及原子晋升；六匹配的正反和缺键边界通过真实 Preview。详见[浏览器验收](../../quality/testcases/console-orbstack-browser-acceptance.md)和[六匹配用例](../../quality/testcases/config-six-match.md)。这不证明外部 SDK 实际消费；最终镜像 `config-formal-20261106-f` 已部署Helm revision18并完成真实页面发布回归；临时文件及业务环境已精确清理。历史复制在共享字段不一致时明确拒绝，防止当前共享模板静默覆盖历史定义；相同共享字段可复制环境取值，回滚查全量active正式ID后执行CAS。

## 相关页面

- [[config-six-match]]
- [[config-center]]
- [[config-protocol-compatibility-baseline]]
- [[terminology]]
