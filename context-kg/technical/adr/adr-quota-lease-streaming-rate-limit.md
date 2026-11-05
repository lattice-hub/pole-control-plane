---
title: 通用配额租约与流式限流
tags: [adr, governance, ratelimit, limiter, sdk, streaming, llm]
links: [governance-rules, architecture, adr-unified-process-mode-and-limiter-integration, adr-governance-request-parameter-capture]
updated: 2026-08-04
sources: 8
---

# 通用配额租约与流式限流

## 状态

已接受，按未正式发布的破坏性变更原地实施。协议、Limiter 与 Rust SDK 不保留旧限流调用方式，也不增加 `V2` 服务名或双协议兼容层。

## 背景

LLM 请求通常以流式响应持续较长时间。若只在每个 token 到达时调用普通限流接口，配额不足可能在响应过程中才暴露为 429；此时业务响应已经开始，客户端无法可靠回退。仅给旧 `QuotaSum` 增加 `used` 字段也不能表达预留、归还、请求占用、重复上报与进程失联。

Limiter Server 已作为 `pkg/limiter` 内置到 control-plane，并由 `bootstrap` 的 `limiter-server` 与 `full` profile 装配。本决策是在现有集成模块内替换运行协议和账本，不新增独立 Limiter 仓库或二次集成层。

目标是提供与业务类型无关的配额生命周期，使 RPM、TPM 和并发限制共用协议与账本，同时只处理配额单位，不处理金额或计费价格。

## 决策

### 使用无版本后缀的单一协议

gRPC 服务统一命名为 `RateLimitGRPC`。客户端在调用业务服务前通过 `RESERVE` 获取租约，流式过程中通过 `UPDATE` 上报累计消费，结束时通过 `SETTLE` 结算。旧 report/return 语义直接删除，不维护兼容分支。

租约命令和职责如下：

| 命令 | 职责 | 关键约束 |
|---|---|---|
| `RESERVE` | 原子预留一个请求涉及的全部 counter 配额 | 使用幂等键；任一 counter 不足则全部失败 |
| `UPDATE` | 将租约中的预留量转为已消费量 | `consumed_total` 为累计值；`sequence` 单调递增 |
| `SETTLE` | 固化最终消费并归还未使用量 | 幂等；租约完成后不可再次增加消费 |

`INIT` 与 `BATCH_INIT` 继续负责初始化分布式 counter，不参与租约状态迁移。

### 区分消耗型与占用型配额

每个 counter 显式声明计量方式：

- `CONSUMABLE`：消费后计入当前限流窗口，未使用的预留量在结算时归还。RPM 与 TPM 使用该方式。
- `OCCUPANCY`：租约存续期间占用槽位，结算或过期时整体释放。并发限制使用该方式。

限流规则增加 `TOKEN` 资源。典型映射为：

- RPM：请求前预留并消费 1 个 `REQUEST/QPS` 单位；
- TPM：请求前按 `prompt_tokens + max_output_tokens` 预留 `TOKEN`，流中更新实际累计 token，结束后归还未生成部分；
- concurrency：请求前预留 1 个 `CONCURRENCY` 槽位，直到结束或租约过期才释放。

### 账本不变量

Limiter 是租约状态的唯一事实源。每个 counter 的可用量满足：

```text
available = limit - committed_in_window - active_reserved
```

多 counter 预留必须在同一临界区内检查并提交，禁止部分成功。`UPDATE` 只接受不超过预留量的累计消费；重复或乱序 sequence 不得重复扣减。流中超出预留量属于调用方协议错误，不尝试追加配额，也不把它转换为业务流中的 429。

租约携带 TTL。客户端失联时，Limiter 到期回收尚未使用的预留量，并释放占用型配额；已转为 consumable committed 的数量继续遵守原 counter 窗口。正常 `SETTLE` 与 TTL 回收都必须幂等。

### SDK 生命周期

Rust SDK 公开 `reserve_quota` 并返回 `QuotaLease`。`QuotaLease` 封装 lease id、sequence 与累计消费，调用方只负责更新已消费总量并在结束时 finish；SDK 不再公开旧的 get/return 组合。

业务调用只有 reserve 阶段可能因配额不足而被拒绝。reserve 成功后，流式输出不能再出现由本地配额检查产生的 429；update/finish 失败作为租约同步或协议错误处理，不回写已经开始的业务响应。

### Console 产品闭环

Console 不直接暴露 protobuf 枚举，而以 QPS、TPM 和并发三个计量场景配置规则；QPS 对应请求数
资源并默认 1 秒窗口，TPM 对应 Token 数资源并默认 60 秒窗口，两者都允许用户直接修改统计窗口。
RPM 只是请求数资源使用 60 秒窗口的配置结果，不作为独立场景重复暴露。TPM 编辑态必须直接解释
请求前预留、流中累计更新和结束归还，不能把它描述为普通响应中途触发的 429。
场景卡是资源类型的唯一选择入口；选中后下方只配置对应的窗口、阈值或并发上限，不再重复展示
请求数、Token 数、并发数单选组。

限流规则以顶层 `namespace` 表示规则归属环境，以 `callee.namespace + callee.service` 表示运行态
目标服务。该契约由 specification `v0.1.0-ALPHA.45` 正式发布；控制面列表按归属环境过滤，数据面
取规则时按 callee 目标服务匹配，禁止再次把两个 namespace 合并成同一个字段。

Limiter 的本地 HTTP 运维面提供只读租约快照，Console 使用独立 `limiterServer` 地址并在完成
登录态校验后代理 `/limiter/v1/quota-leases`。前端不得直接连接 8102。快照按服务过滤并展示活跃
租约、当前预留、流中已使用、最近归还、TTL 回收、counter 上限与可用量；终态记录只保留 5 分钟，
该接口用于运行态诊断，不承担长期审计或计费。

## 取舍

- 预留上界会短暂降低池子的可利用率，但换取业务请求开始后不再因配额不足中断。
- TTL 可恢复崩溃客户端遗留的预留和并发槽位，但 TTL 必须覆盖正常最长请求时长；后续如需长请求续租，应新增显式 keepalive，而不是复用消费更新隐式延长。
- 本方案只管理配额单位和窗口，不计算价格、不记录账单，也不承担模型 tokenizer 的估算职责。

## 验收标准

- specification 只生成 `RateLimitGRPC`，并包含 reserve/update/settle、计量方式和 token 资源。
- Limiter 测试覆盖原子预留、幂等重试、乱序更新、超额消费、结算归还、并发释放和 TTL 回收。
- Rust SDK 测试证明预留失败发生在业务调用前，流中累计更新不重新做配额准入，finish 会归还未使用量。
- RPM、TPM、concurrency 使用同一租约状态机，不引入 LLM 或金额专用协议。
- Console 以场景模板完成规则配置，并能经认证代理观察租约预留、消费、归还和 TTL 回收。

## 相关页面

- [[governance-rules]]
- [[architecture]]
- [[adr-unified-process-mode-and-limiter-integration]]
- [[adr-governance-request-parameter-capture]]
