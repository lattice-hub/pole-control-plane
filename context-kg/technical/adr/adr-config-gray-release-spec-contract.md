---
type: Architecture Decision
status: deprecated
title: ADR：配置中心多灰度发布与 spec 契约适配
tags: [adr, config, api, gray-release]
links: [config-center, api-servers, cache-layer, adr-config-template-values-unification]
updated: 2026-10-03
sources:
  - resource: adr-config-template-values-unification.md
    title: 当前统一配置 v1 决策与实现

---

# ADR：配置中心多灰度发布与 spec 契约适配

## 状态

Deprecated。本文保留旧配置模式的历史设计与证据；旧代码已移除，不能作为当前实现或 SDK 验收状态。现行定义、发布、Discover、本地渲染与兼容转换边界见[统一配置 v1 ADR](adr-config-template-values-unification.md)。

## 背景

配置中心采用方案 B 后，产品语义从“灰度发布是全局发布锁”调整为：

- 正式发布和灰度发布互不阻塞。
- 同一配置文件允许多条 active gray release 同时存在。
- 某条灰度验证通过后，可以提交为正式草稿，再由用户执行正式发布。
- 停止灰度应能指定单条 gray release；未指定 releaseName 时保留停止全部灰度的兼容语义。

当前仓库已在 control-plane 内补齐业务实现和 HTTP route，但 `github.com/pole-io/specification` 仍未声明完整契约。若不更新 specification，其它客户端、生成代码和 OpenAPI 文档无法稳定感知这些能力。

## 决策

配置中心方案 B 需要同步适配 specification。control-plane 内的本地 HTTP route 只能作为过渡实现，不能替代长期协议契约。

必须补齐的 spec 契约：

| 能力 | spec 适配 |
|---|---|
| 灰度转正式草稿 | 在 config manage service / OpenAPI 中声明 `PromoteGrayConfigFileReleaseToDraft` 或等价操作，HTTP 端点语义为把指定 active gray 内容写回正式草稿，不自动正式发布，不停止灰度 |
| 多 active gray | 明确 `ConfigFileRelease` 的 active 唯一性按 release type 区分：normal active 每个文件最多一个，gray active 可按 release name 多个并存 |
| 停止灰度 | `StopGrayConfigFileRelease` 请求携带 `release_name` 时只停止该 gray release；为空时兼容停止该文件全部 active gray |
| 灰度优先级 | 在 `ConfigFileRelease` 或灰度规则消息中新增 `gray_priority` / `priority` 字段；客户端命中多条灰度时先按 priority 数字小者优先，再按创建时间新者优先 |
| 发布记录展示 | 返回模型应包含 release type、release status、active、release name、version、gray labels 和 gray priority，Console 才能稳定分流正式发布、灰度发布、正式草稿和历史记录 |

## 当前过渡状态

- control-plane 已支持多 active gray 的 store/cache key 维度和灰度转正式草稿 HTTP route。
- 当前实际多灰度命中排序仍使用 version/mtime 兜底，因为 specification 尚无 priority 字段。
- Console 已预留灰度优先级入口和展示字段；在 spec 未发版前，该字段会被当前后端 proto JSON parser 作为未知字段忽略，不影响已有发布流程。

## 后续实施顺序

1. 在 specification 中补 `PromoteGrayConfigFileReleaseToDraft` 操作和灰度优先级字段。
2. 重新生成 Go 代码并在 control-plane 中移除本地临时契约假设。
3. MySQL store 持久化 gray priority，并在 cache / client / watch 的灰度选择函数中按 priority、ctime 排序。
4. Console service 将 `grayPriority` 稳定映射为 spec 字段，发布记录和订阅查询使用后端返回的真实优先级。
5. 更新配置中心接口烟测，覆盖 priority 低值优先生效和同优先级按创建时间选择。

## 相关页面

- [[config-center]]
- [[api-servers]]
- [[cache-layer]]

- [统一模板与 values 及多灰度方案](adr-config-template-values-unification.md)
