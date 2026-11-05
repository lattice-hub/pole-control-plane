---
type: Test Case
title: 配置六种匹配正式链路验收
description: 六种客户端标签条件的选版选值、预览、持久性及正式页面回归。
status: stable
tags: [quality, config, audience, http, grpc, mysql, console]
links: [adr-config-template-values-unification, config-center]
updated: 2026-11-05
sources:
  - resource: ../../../pkg/config/unified/audience_test.go
    title: 匹配与拒绝边界
  - resource: ../../../plugin/apiserver/httpserver/config/six_match_live_test.go
    title: 真实 HTTP 与 gRPC 网络验证
  - resource: ../../../plugin/store/mysql/unified_config_audience_integration_test.go
    title: MySQL 回连持久性与发布回滚
---

# 配置六种匹配正式链路验收

目标是恢复原有六种匹配而非只增加UI选项。需求与当前合同见[配置ADR](../../technical/adr/adr-config-template-values-unification.md)，能力摘要见[配置中心](../../business/feature-registry/config-center.md)。

## 前置条件与步骤

`CFG-MATCH-01`：以隔离测试身份/事务仓库启动正式HTTP handlers和gRPC Discover服务。对每种类型保存正式基线，再保存含同类型ValueRule的新定义并新增灰度。分别调用HTTP Preview和gRPC Discover，断言有效values及选中发布ID；命中选gray，未命中和缺键选normal。Preview仅对候选定义选值，不替代灰度选版验证。

| 类型 | 条件value | 命中标签client | 不命中标签client |
| --- | --- | --- | --- |
| EXACT | cn | cn | us |
| REGEX | ^cn-[0-9]+$ | cn-12 | us-12 |
| NOT_EQUALS | blocked | allowed | blocked |
| IN | cn,us | us | jp |
| NOT_IN | cn,us | jp | cn |
| RANGE | 10~20 | 15 | 21 |

每行另测缺少client键。自动化：`TestSixMatchHTTPGRPC`，六类型共18组选版/选值，2026-11-05通过。非法RE2 `[` 返回HTTP400且不泄露旧快照。

`CFG-MATCH-02`：Go匹配/服务测试覆盖空串、星号字面量、空集合成员和不trim、int64上下界、非法/逆序范围、重复键/未知类型、AND、旧map、重叠规则、排序及回滚。相关包race检查通过。同优先级复杂交集无法证明互斥时保守拒绝，不宣称求解任意正则交集。

`CFG-MATCH-03`：独立MySQL8.4.6，执行 `UNIFIED_CONFIG_TEST_DSN=<隔离实例DSN> go test ./plugin/store/mysql -run '^TestUnifiedAudienceMySQLPersistence$' -count=1`。测试创建唯一子库并清理；写六条件、关闭重建连接后检查历史、Discover/Preview、替换/回滚、旧map。2026-11-05通过，专用容器随后停止移除。

`CFG-MATCH-04`：浏览器挂载当前正式UnifiedConfiguration源文件，调用上述正式HTTP服务；六类型hit/miss逐一通过。范围由10~20改11~19，经保存和真实发布回包确认draft3/releaseSet3。另测空value保存回读后命中、非法正则中文错误、null条件JSON修正入口、空文件显示“输出为空（0 字节）”。390px视口无元素越界，恢复默认视口。截图及结构化结果位于仓库本地 `web/console/output/six-match-formal-verification/evidence/`；执行方法见该目录上级README。

## 覆盖与限制

- 已执行全量 `go test ./...`、相关包race、前端构建/目标lint、Audience边界、12共享向量/9管理动作检查。
- 协议Go/Rust合同、Node类型/Pythonwire检查通过；Java/C#/C++构建未执行。
- 全量tsc仍被既有共享组件等错误阻挡；本轮Unified相关源码无类型错误。
- 浏览器使用隔离身份/内存事务仓库，HTTP、领域、选值真实；SQL另由集成测试证明。尚未验证真实凭据+SQL+外部SDK的完整业务E2E，未部署OrbStack。
- 临时浏览器页、live HTTP/Vite进程和独立SQL资源已清理；测试源码及证据保留，未提交推送。

## 交互原型回归（2026-11-05）

`CFG-MATCH-05`：独立18084原型的六操作与正式实现分开验收。运行同一Go/WASM的Node边界检查与22项release-check；浏览器按上表输入正反/缺键，RANGE另测闭区间端点，共19项。三条件REGEX+IN+RANGE联合AND命中通过，非法RE2 lookahead与逆序范围不能进入确认。实际原型发布regex CLIENT_ID条件priority0后，三客户端重读分别#6/#5/#1，历史包含匹配方式；然后重置临时发布并核对#5/#4/#1预置历史。

正则表达式、集合成员、范围双边界控件及JSON匹配测试都可操作。390×600无横溢出且footer按钮可达，多输入id独立。目标类型/lint/独立构建及构建产物冷启动通过；此为原型内存发布与Go WASM匹配验证，不宣称真实业务消费。证据位于 `web/console/output/config-center-demo-2026-11-05/evidence/six-match-*`。

## 正式登录页面与真实 Preview（2026-11-06）

`CFG-MATCH-06`：真实登录 IAB 的正式页面对六种操作符逐一调用真实 Preview 接口，正反命中均通过；负条件缺 key 不命中。RANGE `100~200` 的 200 命中、201 不命中；EXACT `100` 不匹配 `0100`。该证据覆盖页面到 Preview 的选值，不单独证明外部 SDK 消费。正式多灰度发布与历史操作见[浏览器验收](console-orbstack-browser-acceptance.md)。最终镜像 `config-formal-20261106-f`/Helm revision18已完成发布与多灰度回归，临时配置和业务环境已精确清理，详见浏览器验收。

## 相关页面

- [[adr-config-template-values-unification]]
- [[config-center]]
