---
title: Console OrbStack 浏览器全链路验收用例与结果
tags: [quality, testcases, e2e, console, playwright, orbstack]
links: [console-ui-quality-gates, console-client-auth-e2e-testcases, configuration, auth-system, ai-features]
updated: 2026-08-19
sources: 21
---

# Console OrbStack 浏览器全链路验收用例与结果

## 验收口径

本页记录 2026-08-19 在 OrbStack Kubernetes `full` 模式运行态上执行的真实浏览器验收。所有集群命令显式使用
`--context orbstack`。写操作以“提交请求成功、刷新后回读、删除后不可见”为通过标准；只打开页面或静态检查不计作
业务写操作通过。

浏览器验收补充但不替代 [[console-client-auth-e2e-testcases]] 中的接口与客户端传播测试。运行态测试对象统一使用
`e2e-*` 前缀，能够通过 UI 删除的对象在验收结束后清理。

## 实际覆盖矩阵

| 领域 | 已执行的真实操作 | 结果 |
|------|------------------|------|
| 环境空间 | 创建、查看资源聚合页、删除空环境 | 通过；临时环境删除后列表由 6 恢复为 5 |
| 配置中心 | 分组和文件创建、Monaco 编辑、发布 v1/v2、版本比较、回滚生成 v3、文件与分组删除 | 通过；删除文件后 URL 清除 `file`，未再请求已删除详情 |
| 注册发现 | 逻辑服务创建、环境绑定/解绑、环境服务编辑/删除、别名创建/编辑/删除 | 通过；删除以 `namespace/name` 为真实资源键 |
| 服务实例 | 创建、编辑、详情、删除 | 通过；删除提交完整 service/namespace/host/port，详情恢复 metadata |
| 认证管理 | 用户、用户组、角色、策略创建/查看/修改/删除，Token 重置/禁用/启用，资源授权抽屉 | 通过；低权限用户写按钮禁用且未发出变更请求 |
| A2A | 逻辑定义、环境实例、更新、显式绑定、详情、授权数据加载、逻辑定义删除 | 通过；绑定回读和详情展示一致；无环境绑定的定义经确认后删除并刷新列表 |
| MCP | 短地址创建/详情/删除、授权数据加载；长地址存储专项回归 | 短地址浏览器链路通过；长地址修复通过前后端契约与 MySQL 单测，不冒充外部 MCP 调用通过 |
| Skill 市场 | 目录、Registry Source、审核工作台导航；通过“上传 Bundle”导入私有 ZIP Release 并查看详情 | 通过；上传返回 201，详情回读版本、摘要、扫描状态和 Bundle 文件树；本轮选择 ZIP，不扩张到 Git/Registry 外部依赖 |
| 监控 | 系统、Trace、事件、操作审计列表与详情 | 页面和真实数据查询通过；服务监控标注为 Mock 预览，不计作真实观测验收 |
| 系统配置 | 配置域读取、编辑抽屉与取消 | 读取/交互通过；发布会改变共享运行态并可能要求重启，本轮未提交 |
| Agent 工作台 | 环境选择、单环境/跨环境只读比较、会话重命名 | 交互通过；未配置 `agent.model`/`agent.mcp`，发送与变更链路按依赖阻塞记录 |
| 治理工作台 | 路由与空态导航、共享编辑器静态契约 | 未完成九类规则逐类发布/回滚 E2E，不能据此宣称治理数据面全通过 |

## 本轮发现并修复的缺陷

1. 实例扩展查询扫描了 metadata 列却未反序列化，导致 HTTP 主动健康检查路径丢失。
2. 实例删除只提交 `id`，而列表对象没有稳定数据库 id；改为提交完整实例四元组。
3. 环境资源页删除服务使用合成 id，接口幂等成功但对象未删除；改为使用 `namespace/name`。
4. 配置文件删除后仍保留 `file` 查询参数并请求已删除详情；删除成功后先收敛路由再刷新列表。
5. A2A 环境绑定响应被二次读取 `.data`，详情误显示未关联。
6. MCP 自定义地址同时写入长度 32 的 legacy `reference`；地址模式只持久化 `backend_address`。
7. 390×844 窄屏头部可横向滚动并遮挡设置入口；压缩语言与用户区后不再溢出。
8. A2A 逻辑定义缺少删除入口；复用共享逻辑资源目录补齐二次确认、已绑定保护、删除请求和成功后刷新。
9. Skill Release 上传读取 Publisher 时间字段时报 `[]uint8` 到 `time.Time` 扫描错误；MySQL DSN 统一增加
   `parseTime=true`，并覆盖本地、Helm 与测试配置。

每项修复均有定向静态契约或 Go 单测；有可用本地依赖的实例、服务、配置删除和窄屏问题还在最终 OrbStack
镜像上重放了对应浏览器操作。

## 最终运行态证据

- 镜像：`pole-control-plane:console-e2e-final10-20260819`。
- Control Plane 与 OTel Collector 均 Ready，Control Plane 重启数为 0。
- Console HTTPRoute `Accepted=True`、`ResolvedRefs=True`；Console 与 functions 入口均为 HTTP 200。
- Limiter 8101/8102 Endpoint 指向当前 Control Plane Pod。
- A2A 删除回归的浏览器控制台为 0 error/0 warning；删除请求返回 200，列表从 1 项刷新为 0。
- OrbStack 已安装固定版本 Metrics Server `v0.9.0`，`v1beta1.metrics.k8s.io` 为 Available；最终采样中节点为
  `310m / 5051Mi`，Control Plane Pod 为 `20m / 109Mi`，OTel Collector 为 `3m / 140Mi`。
- Skill 测试制品 `e2e/e2e-orbstack-check@0.0.1` 以私有 ZIP 导入，服务端扫描通过；详情可浏览根目录
  `SKILL.md` 与 `README.md`。该不可变测试 Release 和测试 Publisher 保留用于后续回归。

## 保留边界

- Skill Git/Registry 导入、Agent 模型调用、MCP 真实工具刷新仍需要外部仓库、Registry 或模型连接；本轮已经以
  自包含 ZIP 覆盖一条真实外部导入链路，但不据此宣称另外两类来源已通过。
- 系统配置发布/重启和九类治理规则逐类发布会改变共享运行态，本轮没有把仅打开页面表述为写操作通过。
- `npm` release build 通过；依赖审计仍报告既有高危项，安全升级不属于本轮交互缺陷修复范围。

## 统一配置真实页面验收（2026-11-06，已完成）

已登录真实 IAB 的 `lattice-hub.localhost` 正式页面，以业务 namespace `ui-audit-config-20261106`、group `ui-audit-config`、文件 `ui-audit-config-20261106.yaml` 和 `temp/default` 环境操作。YAML 正文在本地 Monaco 中编辑，编辑器填充高度；配置 `port` INTEGER（fallback 8080，region EXACT cn → 8088）和 `logLevel` STRING（fallback info，tier EXACT canary → debug）。跨环境保存先因 default 缺少 required/refvalue 局部失败，补充 9090/warn 后两环境成功。真实 Preview 命中 8088/debug、未命中 8080/info，修改输入清除旧结果。

两环境正式发布成功；temp 中 regex `CLIENT_ID ^ui-audit-.*` priority 10 的 #2 与 RANGE `build 100~200` priority 5 的 #3 同时 active，normal #1 仍 active；之后 normal #4 保留两个 gray。历史回滚 #1 生成 #5；#3 原子转正式生成 #6，#3 停止而 #2 保留。复制 #6 成功；修改共享正文后复制 #1 被明确拒绝以防覆盖。

390×600 视口 document 宽度 390，抽屉 footer 的下一步可见；只读标签内部 30px、外框 32px，与 32px select 对齐。浏览器 console error 为空。截图位于本地 `output/config-formal-20261106/`，包括 `publish-result.png`、`multiple-gray.png`、`gray-mobile.png`；最终镜像截图为 `multiple-gray-final.png`、`gray-input-final.png`、`publish-result-final.png`、`history-final.png`。

真实操作暴露并修复嵌套 `@type` 保存、条件组自动折叠、原生确认阻塞、发布刷新卸载结果、回滚空 ID 与复制静默覆盖问题。最终镜像 `config-formal-20261106-f` 已部署 Helm revision18，Pod Ready且0重启、Gateway/HTTPRoute条件True、HTTP200。最终镜像新增集合灰度 #7，与regex #2和正式 #6并存，发布结果不再重置。临时文件模板1/绑定2/状态2/发布8/事件8已精确清为0，逻辑clock不修改；UI删除空测试环境，业务环境恢复仅default，配置目录为空。没有提交推送。全仓tsc/KG lint仍有既有错误；本轮目标源码无类型错误。

## 相关页面

- [[console-ui-quality-gates]]
- [[console-client-auth-e2e-testcases]]
- [[configuration]]
- [[auth-system]]
- [[ai-features]]
