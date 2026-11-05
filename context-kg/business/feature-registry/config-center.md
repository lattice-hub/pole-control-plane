---
type: Feature
title: 配置中心（`pkg/config/`）
description: 统一配置定义、草稿、原子发布与多灰度，原生客户端本地渲染，兼容协议按发布开关转换。
status: stable
tags: [business, feature, config]
links: [namespace, adr-config-template-values-unification]
updated: 2026-11-05
sources:
  - resource: ../../../pkg/config/unified/service.go
    title: 统一配置领域核心
  - resource: ../../../pkg/config/unified/discover.go
    title: 标签选择与原生渲染快照
---

# 配置中心（`pkg/config/`）

用户管理一份完整配置定义：模板、参数、默认 values、标签取值规则和 Apollo/Nacos/Consul 转换开关。每个 environment/group/name 独立维护草稿、一个正式版本、多个灰度版本和不可变历史；模板与 values 一起发布。

2026-10-03 用户纠正管理源归属：共享模板包含正文与变量声明；具体 values 和标签规则按环境维护，发布允许只选部分环境，并在各目标环境冻结完整版本。正式实现现已新增共享正文CAS、绑定环境及目录管理；各环境独立保存完整草稿，发布冻结完整快照。具体设计与待补边界见[当前 ADR 的跨环境修正](../../technical/adr/adr-config-template-values-unification.md)。

2026-11-05 用户进一步确认目标：条件组包含整组AND判断，各key独立关联组与值，兜底可选；未命中且无兜底的key不进入取值结果，显式空字符串保持存在。正式v1现已增加逐参数条件组绑定与可选fallback；旧全配置winning规则保留兼容，新旧模型不能混用。冲突顺序、模板缺值和发布完整性边界见[当前 ADR 的逐key修正](../../technical/adr/adr-config-template-values-unification.md)。

正式v1现已恢复 EXACT、REGEX、NOT_EQUALS、IN、NOT_IN、RANGE 六种客户端标签条件；灰度选版和版本内取值共用后端匹配器，条件 AND，缺标签时负条件也不命中。旧 labels 精确数据保留，正式前端可编辑、保存、预览和查看历史条件。详见[六匹配合同与验证边界](../../technical/adr/adr-config-template-values-unification.md#正式链路恢复六种匹配2026-11-05已实现)。

control-plane 先从生效集合按标签选择唯一版本，再选择该版本内的有效 values。原生 `Discover` 和管理 `Preview` 下发原模板及有效 values，由 SDK/控制台本地渲染。Discover 在双向流上逐次请求、逐次完整响应，没有原生 Watch、Resolve 或主动 push。

控制面保留九个核心 HTTP 管理动作：草稿保存、状态读取、预览、发布、停止单版本、停止全部灰度、复制历史到草稿、回滚和下线。草稿及生效集合分别使用 CAS；管理历史使用有界摘要分页和按需单版本详情。另有目录、共享正文读写与灰度转正式四个动作，共13个。环境内仍有配置聚合（包括下线历史）或共享正文绑定时不能删除环境。

全部读取重新认证并授权到业务环境；客户端标签和 Staffname 不授予权限。适配器部署启用且选中发布明确开启转换后，才在外部协议适配层渲染。失败不能转换为空内容或缺失，literal 空值和二进制保留。

旧文件/分组/独立模板/Value/Binding 管理和 WatchCenter 已移除。当前 Console 以配置文件目录进入共享正文/环境取值编辑，再按ConfigKey发布，Agent Workbench 通过 state/draft 保存完整定义及 CAS。兼容 profile 与已验证范围见[当前 ADR](../../technical/adr/adr-config-template-values-unification.md)；真实外部 SDK、原生 SDK 接纳/缓存和部署业务验收仍需分别验证。
