---
type: Protocol Compatibility Baseline
title: 配置中心外部协议兼容基线
description: 核实 Apollo、Nacos 与 Consul 的读取、通知、写入和身份边界，为统一模板与 values 提出分级兼容范围。
tags: [config, compatibility, apollo, nacos, consul, template]
status: draft
updated: 2026-10-02
links: [api-servers, adr-config-template-client-rendering, adr-config-template-values-unification]
sources:
  - id: apollo-http
    resource: https://github.com/apolloconfig/apollo/blob/ce54e20b5263d6b157bafcb9d7d816fcc11f146e/docs/zh/client/other-language-client-user-guide.md
    title: Apollo 其他语言客户端指南
  - id: apollo-env
    resource: https://github.com/apolloconfig/apollo/blob/ce54e20b5263d6b157bafcb9d7d816fcc11f146e/docs/zh/client/java-sdk-user-guide.md
    title: Apollo Java SDK 环境与 Meta Server
  - id: apollo-openapi
    resource: https://github.com/apolloconfig/apollo/blob/ce54e20b5263d6b157bafcb9d7d816fcc11f146e/docs/zh/portal/apollo-open-api-platform.md
    title: Apollo 开放平台接入指南
  - id: apollo-configfile
    resource: https://github.com/apolloconfig/apollo/blob/ce54e20b5263d6b157bafcb9d7d816fcc11f146e/apollo-configservice/src/main/java/com/ctrip/framework/apollo/configservice/controller/ConfigFileController.java
    title: Apollo 配置文件输出实现
  - id: nacos-v1
    resource: https://nacos.io/docs/v1/open-api/
    title: Nacos 1.x Open API 指南
  - id: nacos-grpc-listen
    resource: https://github.com/alibaba/nacos/blob/2.4.3/config/src/main/java/com/alibaba/nacos/config/server/remote/ConfigChangeBatchListenRequestHandler.java
    title: Nacos 2.4.3 gRPC 配置监听处理
  - id: nacos-grpc-query
    resource: https://github.com/alibaba/nacos/blob/2.4.3/config/src/main/java/com/alibaba/nacos/config/server/remote/ConfigQueryRequestHandler.java
    title: Nacos 2.4.3 gRPC 查询处理
  - id: nacos-grpc-publish
    resource: https://github.com/alibaba/nacos/blob/2.4.3/config/src/main/java/com/alibaba/nacos/config/server/remote/ConfigPublishRequestHandler.java
    title: Nacos 2.4.3 gRPC 发布处理
  - id: nacos-client
    resource: https://github.com/alibaba/nacos/blob/2.4.3/client/src/main/java/com/alibaba/nacos/client/config/impl/ClientWorker.java
    title: Nacos 2.4.3 客户端通知与查询
  - id: nacos-validation
    resource: https://github.com/alibaba/nacos/blob/2.4.3/config/src/main/java/com/alibaba/nacos/config/server/utils/ParamUtils.java
    title: Nacos 2.4.3 空内容校验
  - id: nacos-v3-client
    resource: https://nacos.io/docs/v3.0/manual/user/open-api/
    title: Nacos 3.0 客户端 HTTP API
  - id: nacos-v3-admin
    resource: https://nacos.io/docs/v3.0/manual/admin/admin-api/
    title: Nacos 3.0 运维 HTTP API
  - id: nacos-v3-upgrade
    resource: https://nacos.io/docs/v3.0/manual/admin/upgrading/
    title: Nacos 3.0.1 升级兼容性
  - id: nacos-latest-upgrade
    resource: https://nacos.io/docs/latest/manual/admin/upgrading/
    title: Nacos 3.2 升级兼容性
  - id: nacos-auth
    resource: https://nacos.io/docs/v3.0/manual/user/auth/
    title: Nacos 默认鉴权与访问凭据
  - id: nacos-gray-mse
    resource: https://help.aliyun.com/zh/mse/user-guide/configure-canary-release
    title: MSE Nacos 标签灰度的版本限制
  - id: nacos-gray-labels
    resource: https://nacos.io/blog/nacos-gvr7dx_awbbpb_ir01zzd3q20fa3xq/
    title: Nacos 官方介绍 MSE 自定义标签灰度
  - id: nacos-cas-state
    resource: https://github.com/alibaba/nacos/blob/2.4.3/config/src/main/java/com/alibaba/nacos/config/server/service/repository/extrnal/ExternalConfigInfoPersistServiceImpl.java
    title: Nacos 内容 CAS 与不存在时创建分支
  - id: consul-idempotent-write
    resource: https://github.com/hashicorp/consul/blob/v1.22.0/agent/consul/state/kvs.go
    title: Consul 相同条目保持 ModifyIndex
  - id: consul-entry-equality
    resource: https://github.com/hashicorp/consul/blob/v1.22.0/agent/structs/structs.go
    title: Consul 条目相等比较字段
  - id: consul-kv
    resource: https://developer.hashicorp.com/consul/api-docs/kv
    title: Consul KV HTTP API
  - id: consul-blocking
    resource: https://developer.hashicorp.com/consul/api-docs/features/blocking
    title: Consul blocking query 契约
  - id: consul-txn
    resource: https://developer.hashicorp.com/consul/api-docs/txn
    title: Consul 事务 HTTP API
  - id: consul-session
    resource: https://developer.hashicorp.com/consul/api-docs/session
    title: Consul Session HTTP API
  - id: consul-auth
    resource: https://developer.hashicorp.com/consul/api-docs/api-structure
    title: Consul HTTP 身份与路径
  - id: consul-opaque
    resource: https://developer.hashicorp.com/consul/docs/automate/kv
    title: Consul KV 的不透明对象与大小约束
---

# 配置中心外部协议兼容基线

核查日期：2026-10-02。本文是官方文档及官方源码的研究结论与实现建议，不证明 Pole 当前已经实现或通过三种协议的客户端验收。Apollo 来源固定到本次核查的 master 提交 `ce54e20b5263d6b157bafcb9d7d816fcc11f146e`（提交日期 2026-09-30），Nacos gRPC 源码固定为 tag `2.4.3`；Nacos 3.0、核查时 latest 3.2 和 MSE 的能力分别标注。Nacos 官网自定义标签文章发表于 2024-11-12；官网 latest、云产品帮助页和 Consul 在线文档会更新，下述结论以核查日期所见为准。上线支持声明必须进一步固定服务端与 SDK 版本。

## 总体结论

统一 Template + Values 可以成为内部发布模型，但外部适配器必须输出客户端原本期待的最终配置表示。Apollo、Nacos、Consul 既有客户端不会因服务端内部改用模板而自动获得模板渲染能力。对这些协议，应在服务端选定环境发布版本，以该协议真实可提供的客户端上下文解析有效 values 并完成渲染，再执行协议特定编码。按用户后续澄清，同一发布版本可按标签生成不同结果；该内部取值规则属于主 ADR 的新增设计，不是上游协议普遍具备任意标签输入的证明。

外部写入不宜直接解释为修改渲染结果。一般模板存在条件、循环、重复引用和格式变换，给定渲染结果无法唯一反推 values。主 ADR 选定无参数 `literal` 常量模板承载兼容写入，`values = {}`，参数化资源默认只读并要求显式接管。以下档位是拟议验收契约，不是全兼容承诺。

用户已取消 Pole 自身的历史兼容约束；主 ADR 按新 spec/control-plane/SDK 设计，原生 SDK 在本地渲染，服务端转换只发生在显式开启的外部适配层。本文的兼容要求仅指外部 Apollo/Nacos/Consul 所选协议，不要求保留旧 Pole API、数据模型或客户端渲染合同。

## Apollo

### 资源、输出与环境

- 客户端配置路径包含 `appId / clusterName / namespaceName`；`properties` Namespace 通常不带扩展名，其他格式须带 `.json`、`.yaml` 等后缀。`env` 不在 `/configs` 的路径中：客户端按环境选择 Meta Server / Config Service；Portal OpenAPI 则显式包含 `/envs/{env}`。适配器应建立显式环境路由映射，不能把 cluster 当作 env 的同义词。[HTTP 指南 §1.2–1.3](https://github.com/apolloconfig/apollo/blob/ce54e20b5263d6b157bafcb9d7d816fcc11f146e/docs/zh/client/other-language-client-user-guide.md#12-通过带缓存的http接口从apollo读取配置)、[Java SDK §1.2.2、§1.2.4.1](https://github.com/apolloconfig/apollo/blob/ce54e20b5263d6b157bafcb9d7d816fcc11f146e/docs/zh/client/java-sdk-user-guide.md#122-apollo-meta-server)。
- `GET /configs/{appId}/{clusterName}/{namespaceName}` 返回含 `appId`、`cluster`、`namespaceName`、`configurations` 与 `releaseKey` 的对象。`/configfiles/json/...` 直接返回 map：properties 是字符串键值，非 properties 用 `content` 装完整文件字符串。`/configfiles/raw/...` 返回文件文本；properties 的 raw 输出是由 map 重新序列化的 properties 文本，不保证保留原始注释、顺序或空白。[HTTP 指南 §1.2.2、§1.3.2](https://github.com/apolloconfig/apollo/blob/ce54e20b5263d6b157bafcb9d7d816fcc11f146e/docs/zh/client/other-language-client-user-guide.md#122-http接口返回格式)、[ConfigFileController.getRawConfigContent L262–269](https://github.com/apolloconfig/apollo/blob/ce54e20b5263d6b157bafcb9d7d816fcc11f146e/apollo-configservice/src/main/java/com/ctrip/framework/apollo/configservice/controller/ConfigFileController.java#L262)。

### 两种不同的版本标识

`/configs` 接受上次 `releaseKey`，相等时返回 HTTP 304 且无 body；它是发布状态标识，不应解释为可排序的整数。`messages` 是含通知明细的 JSON，在多实例内存缓存条件下用于使拉取侧及时刷新；不能只实现 `releaseKey` 比较而忽略该组合。[HTTP 指南 §1.3.1](https://github.com/apolloconfig/apollo/blob/ce54e20b5263d6b157bafcb9d7d816fcc11f146e/docs/zh/client/other-language-client-user-guide.md#131-http接口说明)。

`GET /notifications/v2?appId=...&cluster=...&notifications=...` 的通知数组含 `namespaceName` 与数字 `notificationId`，初始为 `-1`。官方说明：通知都最新时挂起约 60 秒，无变化返回 304；有旧游标或新变更返回 200 与变化 Namespace 的新 notificationId，客户端随后再次拉取配置。这是通知游标，不是 releaseKey，也不是内容 hash。[HTTP 指南 §1.4.1–1.4.3](https://github.com/apolloconfig/apollo/blob/ce54e20b5263d6b157bafcb9d7d816fcc11f146e/docs/zh/client/other-language-client-user-guide.md#14-应用感知配置更新)。

### 编辑、发布与鉴权

Portal OpenAPI 通过 `/openapi/v1/envs/{env}/apps/{appId}/clusters/{clusterName}/namespaces/{namespaceName}` 下的 `POST items`、`PUT items/{key}`、`DELETE items/{key}` 修改编辑态，再由 `POST releases` 发布。非 properties 的 item key 固定为 `content`，value 是整份文件。写 item 和发布必须保持两个提交时点，不能把 item 修改自动发布。[OpenAPI §3.2.10–3.2.13](https://github.com/apolloconfig/apollo/blob/ce54e20b5263d6b157bafcb9d7d816fcc11f146e/docs/zh/portal/apollo-open-api-platform.md#3210-新增配置接口)。

OpenAPI 使用 Portal 签发 Token，通过 `Authorization` header 授予目标 Namespace 的编辑/发布权限；配置消费者开启访问密钥时使用 `Authorization: Apollo {appId}:{signature}` 与毫秒 `Timestamp`，签名覆盖时间和 URL 的 path/query。这两种凭据与权限入口应分别实现。[OpenAPI 接入授权](https://github.com/apolloconfig/apollo/blob/ce54e20b5263d6b157bafcb9d7d816fcc11f146e/docs/zh/portal/apollo-open-api-platform.md#2-接入步骤)、[客户端指南 §1.5](https://github.com/apolloconfig/apollo/blob/ce54e20b5263d6b157bafcb9d7d816fcc11f146e/docs/zh/client/other-language-client-user-guide.md#15-配置访问密钥)。

## Nacos

### 1.x HTTP 配置协议

正式资源键是 `tenant / group / dataId`；tenant 表示 namespace ID，不是名称。`GET /nacos/v1/cs/configs` 读 content，`POST` 同路径发布 content，`DELETE` 同路径删除；group 缺省为 `DEFAULT_GROUP`，tenant 可省略。发布成功响应为 `true`，不是 Apollo 的独立草稿编辑动作。[1.x Open API「获取配置」「发布配置」「删除配置」](https://nacos.io/docs/v1/open-api/)。

监听为 `POST /nacos/v1/cs/configs/listener`：表单 `Listening-Configs` 每条为 `dataId\x02group\x02contentMD5\x02tenant\x01`，或省略 tenant 字段。字段分隔符是字节 `0x02`，记录分隔符是 `0x01`，URL 编码分别 `%02`、`%01`；不是可见字符 `^2` / `^1`。`Long-Pulling-Timeout: 30000` 控制长轮询，MD5 相同则等待；返回变化键列表 `dataId\x02group\x02tenant\x01`，无变化是空串。通知后客户端必须重新 GET。[1.x Open API「监听配置」](https://nacos.io/docs/v1/open-api/#监听配置)。

MD5 必须来自该客户端最终看到的 content，不能替换为发布 ID、模板 ID 或内部 SHA。监听报文的初次空 MD5 表示本地无缓存，不能据此推断配置内容就是空值。

内容 CAS 不包含资源存在期。Nacos 2.4.3 的 `casMd5` 无法区分 A→B→A 或删除后以相同内容重建；资源不存在时 `insertOrUpdateCas` 进入创建分支。Pole 不能通过内部存在期让标准客户端获得更强的防旧写保证。[2.4.3 CAS 创建分支](https://github.com/alibaba/nacos/blob/2.4.3/config/src/main/java/com/alibaba/nacos/config/server/service/repository/extrnal/ExternalConfigInfoPersistServiceImpl.java#L188-L197)、[CAS 比较字段](https://github.com/alibaba/nacos/blob/2.4.3/config/src/main/java/com/alibaba/nacos/config/server/service/repository/extrnal/ExternalConfigInfoPersistServiceImpl.java#L615-L618)。

### 2.x gRPC 配置协议

- `ConfigBatchListenRequest` 包含 `listen` 与一组 `dataId/group/tenant/md5`，用于订阅及取消订阅；服务端记录 connection ID，立即返回 MD5 已不一致的配置键。订阅 READ 鉴权作用于配置资源。[2.4.3 ConfigChangeBatchListenRequestHandler.handle L51–79](https://github.com/alibaba/nacos/blob/2.4.3/config/src/main/java/com/alibaba/nacos/config/server/remote/ConfigChangeBatchListenRequestHandler.java#L51)。
- 服务端发出 `ConfigChangeNotifyRequest` 后，客户端确认通知并使本地 cache 重新查询；消息不是完整 content。协议支持必须覆盖长连接、服务端请求确认、重连重订阅及 `ConfigQueryRequest`，单个 gRPC query handler 不等于完整 SDK 兼容。[2.4.3 ClientWorker L668–705、L1017–1040](https://github.com/alibaba/nacos/blob/2.4.3/client/src/main/java/com/alibaba/nacos/client/config/impl/ClientWorker.java#L668)。
- query 返回 content、MD5、contentType、lastModified、beta、加密数据 key 等信息；缺失配置由 `CONFIG_NOT_FOUND` 表达。客户端对 not-found 清理配置快照，不能返回“200 + 空配置”伪装存在。[ConfigQueryRequestHandler L142–170](https://github.com/alibaba/nacos/blob/2.4.3/config/src/main/java/com/alibaba/nacos/config/server/remote/ConfigQueryRequestHandler.java#L142)、[ClientWorker L1184–1187](https://github.com/alibaba/nacos/blob/2.4.3/client/src/main/java/com/alibaba/nacos/client/config/impl/ClientWorker.java#L1184)。
- `ConfigPublishRequest` 使用 WRITE 鉴权，保存正式或 beta/tag 内容并发出变更，包含 `casMd5` 条件发布入口；删除使用独立 `ConfigRemoveRequest`。不要把 publish 接成只保存草稿且等待 Console 发布。[ConfigPublishRequestHandler L73–153](https://github.com/alibaba/nacos/blob/2.4.3/config/src/main/java/com/alibaba/nacos/config/server/remote/ConfigPublishRequestHandler.java#L73)、[ClientWorker 请求类型](https://github.com/alibaba/nacos/blob/2.4.3/client/src/main/java/com/alibaba/nacos/client/config/impl/ClientWorker.java#L24)。

**空与删除的限定**：Nacos 2.4.3 v1/v2 共用校验拒绝 `StringUtils.isBlank(content)`，包括空串与纯空白；该发布失败不是删除。官方 GET 具有 not-found 分支。因此模板渲染得到空串时，应依据目标协议校验并报错，不能自动删资源或声称所有 Nacos 版本接受空配置。[ParamUtils.checkParam L82–100](https://github.com/alibaba/nacos/blob/2.4.3/config/src/main/java/com/alibaba/nacos/config/server/utils/ParamUtils.java#L82)。

### 3.x HTTP 和版本兼容分界

3.0 客户端 HTTP 路径为 `GET /nacos/v3/client/cs/config`，参数改为 `namespaceId / groupName / dataId`，响应使用统一 JSON 包装，data 内有 content、md5 等；默认 namespace ID 为 `public`，与空串兼容。该 Client HTTP API **没有 publish/delete/listener**，通过轮询 GET 比较 MD5；写入属于 Admin API：`POST`、`DELETE /nacos/v3/admin/cs/config`，需要管理员权限。Admin 的 `/config/listener` 是查看监听者状态，不是给客户端长轮询用。[3.0 Client API §1](https://nacos.io/docs/v3.0/manual/user/open-api/#1-配置管理)、[3.0 Admin API §3.2–3.5](https://nacos.io/docs/v3.0/manual/admin/admin-api/)。

官方 3.0 Client HTTP 文档说新 OpenAPI 不兼容旧 v1/v2 HTTP；但 3.0.1 升级文档仍列 1.x SDK 兼容、计划 v3.2 后停止。这描述新 API 与过渡期 legacy endpoint 的不同范围，不能据一份文档断言所有 3.x 版本行为相同。latest 3.2 明确 1.x 客户端不兼容，2.x/3.x 客户端兼容，旧 HTTP API 需自行集成 legacy adapter；3.0 的空 namespace 到 public 迁移与兼容模式也应单独考虑。[3.0 Client API 兼容提示](https://nacos.io/docs/v3.0/manual/user/open-api/)、[3.0.1 升级 §1.1](https://nacos.io/docs/v3.0/manual/admin/upgrading/#11-客户端兼容性)、[latest 升级 §1.1、§2.1.7、§3](https://nacos.io/docs/latest/manual/admin/upgrading/)。

### 鉴权与灰度能力限制

默认鉴权插件通过 username/password 登录取得 accessToken，HTTP 请求携带 accessToken；3.x 登录路径为 `/nacos/v3/auth/user/login`。这不是所有自定义鉴权插件的通用协议，具体 Token 生命周期与资源 READ/WRITE 权限须在所声明插件 profile 中测试。官方 3.0 鉴权页面的业务请求例子仍使用 v2 路径，应以对应版本 Client/Admin API 路径为准。[Nacos 配置鉴权 §2](https://nacos.io/docs/v3.0/manual/user/auth/#2-openapi如何配置鉴权信息)。

基础 Nacos 2.4.3 query/listen 使用 client IP 与 `Vipserver-Tag`，不是任意发现实例 metadata 自动参与灰度。[query L76–85、L99–139](https://github.com/alibaba/nacos/blob/2.4.3/config/src/main/java/com/alibaba/nacos/config/server/remote/ConfigQueryRequestHandler.java#L76)、[listen L58–71](https://github.com/alibaba/nacos/blob/2.4.3/config/src/main/java/com/alibaba/nacos/config/server/remote/ConfigChangeBatchListenRequestHandler.java#L58)。

官方自定义标签文章介绍的是 **MSE 服务端**：开源 Java Nacos-Client ≥2.3.2 收集自定义应用标签，环境变量注入要求 ≥2.4.2，MSE 服务端要求 ≥2.2.3.3。`nacos.config.gray.label`、`nacos.app.conn.labels` 来自 ConfigService 参数、JVM 或环境变量，优先级 properties > JVM > env；标签 key/value 字符受限于字母、数字、`_-.`，不合法项忽略。文章中的节点标签数量、表达式与灰度版本数限制属于 MSE，不能据此宣称开源 server 或 Go/Python SDK 同样支持。[阿里云 MSE 官方版本要求](https://help.aliyun.com/zh/mse/user-guide/configure-canary-release)、[Nacos 官方标签文章「版本要求」「设置应用标签」「多标签灰度」](https://nacos.io/blog/nacos-gvr7dx_awbbpb_ir01zzd3q20fa3xq/)。

## Consul KV

### 资源与字节表示

`GET /v1/kv/{key}` 默认精确匹配，返回数组与 `CreateIndex/ModifyIndex/LockIndex/Flags/Value/Session` 元数据，Value 为 base64；`raw` 返回原始值，`keys` 只返回 key 数组且隐含 recurse，`recurse` 将 key 视为前缀，`separator` 用于 keys 列举分层前缀。不存在 key 返回 404。`PUT` 保存请求 body 字节，`DELETE` 删除精确 key，`DELETE ?recurse` 删除该前缀的所有 key。前缀是字符串匹配，`/` 不构成真实目录。[KV API Read/Create/Delete](https://developer.hashicorp.com/consul/api-docs/kv)、[KV 对象与 key 说明](https://developer.hashicorp.com/consul/docs/automate/kv#using-consul-kv)。

Consul Value 是不透明对象，可包含二进制，最大 512 KB；模板系统若只支持 UTF-8 文本，应对外声明“文本配置消费子集”，明确拒绝超出范围的二进制写入，不能静默解码替换非法字节。存在但长度为 0 的值与 key 缺失必须区分，GET raw 的空 body 仍可能是存在值；启用字节安全的 KV profile 时，含 NUL 字节的载荷按原样保存。[Consul KV 对象限制](https://developer.hashicorp.com/consul/docs/automate/kv#using-consul-kv)。

### blocking query 与 CAS

KV 读取通过 `X-Consul-Index` 返回查询状态，后续传 `index=<previous>` 与 `wait` 阻塞；wait 默认 5 分钟、最多 10 分钟，并有随机额外等待。返回不保证内容已变化；index 回退时客户端需重置为 0，并避免零值导致忙循环。前缀读取也需正确感知删除，尤其最高 ModifyIndex 的成员被删导致 index 回退的情形。[Blocking Queries](https://developer.hashicorp.com/consul/api-docs/features/blocking)。

`PUT ?cas=0` 仅允许不存在时创建；非零 cas 仅在等于 key 的 ModifyIndex 时写入。`DELETE ?cas=N` 要求 N > 0 且匹配 ModifyIndex。Flags 是客户端自定义 uint64 元数据，必须保留。**内容相同也不能将 CAS 状态强行归并成同一个内容 hash**；查询游标和键级写入前提是独立状态。[KV API CAS、Flags](https://developer.hashicorp.com/consul/api-docs/kv#createupdate-key)。

完整条目相同的 PUT 不要求推进 ModifyIndex。Consul v1.22.0 的 `kvsSetTxn` 在条目相等时保留旧值，`DirEntry.Equal` 比较 key、Value、Flags、LockIndex 和 Session，不比较内部发布记录。基础 KV profile 虽不开放锁操作，也须区分正文相同与完整条目相同；Flags 改变不能被正文 hash 去重。[kvsSetTxn](https://github.com/hashicorp/consul/blob/v1.22.0/agent/consul/state/kvs.go#L148-L153)、[DirEntry.Equal](https://github.com/hashicorp/consul/blob/v1.22.0/agent/structs/structs.go#L2623-L2629)。

### 必须单独声明的 Consul 能力

| 能力 | 官方契约与边界 |
| --- | --- |
| 原子事务 | `PUT /v1/txn` 最多 64 操作，任意操作失败回滚所有修改；支持 set/cas/check-index/delete-tree 等，失败可返回 409；只读 txn 不支持 blocking。[Transactions](https://developer.hashicorp.com/consul/api-docs/txn) |
| Session lock | acquire/release 使用 Session ID、维护 Session/LockIndex；普通 PUT 即使被其他 Session 锁定仍可写，锁是协作语义。[KV acquire/release](https://developer.hashicorp.com/consul/api-docs/kv#createupdate-key)；会话创建、续租、TTL/失效行为另属 [Session API](https://developer.hashicorp.com/consul/api-docs/session) |
| ACL | key:read 与 key:write 分开，Token 使用 `X-Consul-Token` 或 `Authorization: Bearer`；列表可能被 ACL 过滤，应正确表达过滤结果。[HTTP 身份与过滤](https://developer.hashicorp.com/consul/api-docs/api-structure#authentication) |
| Datacenter | 每个 dc 有独立 KV，没有内置跨 dc KV 复制，`dc` 参数不能随意忽略并回落到默认环境。[KV API](https://developer.hashicorp.com/consul/api-docs/kv) |
| Enterprise Namespace / partition | `ns`、`X-Consul-Namespace`、ACL Token 所属 namespace 和 default 有优先级；namespace/partition 是 Enterprise 范围，不能等同于 OSS key 前缀。[KV Namespace](https://developer.hashicorp.com/consul/api-docs/kv#methods-to-specify-namespace) |

若实现不含事务、Session 或 Enterprise，收到这些参数必须返回明确的不支持结果，不能丢弃参数后执行普通写入。

## 最低合理支持 profile（建议）

| Profile | 首期应覆盖的可验收行为 | 边界 |
| --- | --- | --- |
| `apollo-config-read-v1` | 显式 app/cluster/namespace/env 映射；properties map 与 non-properties content/raw；configs releaseKey/304；notifications/v2 数字通知 ID 与重拉；访问密钥鉴权 | 公共 Namespace 继承、cluster fallback、灰度 label/IP、Portal OpenAPI 分别列附加能力 |
| `nacos-config-http-v1` | tenant/group/dataId；GET + MD5 listener 分隔符和超时；资源缺失与权限失败 | HTTP publish/delete 是可选写入扩展，开启后必须保留即时发布语义；不代表 3.x Client HTTP |
| `nacos-config-grpc-v2` | 固定 SDK 版本，query + batch listen/unlisten + server notify ACK + 重连重订阅；MD5 与 not-found；READ 鉴权 | publish/remove/casMd5、beta、tag、加密插件是独立能力，不把连接标签收集等同于完整灰度 |
| `nacos-config-http-v3-read` | GET `/v3/client/cs/config`、namespaceId/groupName/dataId、统一 envelope 与 MD5 轮询、默认鉴权插件 | 无 Client publish/delete/listener；Admin API 单独实现/授权 |
| `consul-kv-config-read-v1` | 单 key JSON/base64/raw；keys/recurse/separator；404；X-Consul-Index/index/wait；key:read；明确 dc 映射 | 文本子集必须显式声明；无 Session/事务/Enterprise 时不能声称 KV 全兼容 |

基础 profile 可以从读取与通知开始；协议写入只有在权限、`literal` 字节恒等交付及发布事务都通过验收后才启用。名称中的版本指协议 profile，不是 Pole 或上游所有产品版本的兼容保证。

## 模板 + values 下的写入设计建议

1. **显式配置绑定。** 外部定位符映射到内部环境、config_id 和一个输出地址身份；映射记录格式、编码、大小限制、写入模式与灰度可用信息。身份定位不能代替权限。Consul key 应作为完整不透明字符串保存，不能只按 `/` 拆分为 group/file 而丢失可逆性。
2. **literal 常量模板承载兼容写入。** 按主 ADR 选定方案，将外部 payload 保存为无参数 `literal` 模板，`values = {}`，交付采用原始字节恒等映射，不进入 Mustache 解析。正文中的 `{{...}}`、CRLF、BOM、末尾换行等须保留；只有存储和交付都具备字节安全表示时才启用 Consul 二进制 profile。`{{content}}` 包装和逆向映射均不是当前选定方案。
3. **参数化资源默认只读、显式接管。** 对既有参数化模板，拒绝外部整文件写入，并指向内部 Value 管理。只有经过显式、可预览的接管流程，才能将写入所有者与表示转换为 `literal` 兼容资源。不得自动逆向推导 values、自动脱离模板、创建隐藏覆盖层或把解析出的 properties 键随意当成参数。
4. **保持协议提交时点。** Apollo item 操作进入兼容草稿，release 才冻结完整发布；Nacos publish 与 Consul PUT 必须在单一成功事务内生成且激活新环境发布，否则返回失败。按主 ADR 收窄基础写入为无活跃灰度、无取值规则的 literal 正式视图；即时写入只使用本次正文，不夹带或覆写 Console 未发布草稿。该限制是 Pole 的设计选择，不表示上游不支持 beta/tag。
5. **保留删除和条件写。** 按主 ADR，外部 DELETE 仅撤下所请求输出地址并保留 tombstone，普通后续发布不能复活；不删共享模板、其他出口或其他环境。Nacos blank publish 拒绝；Consul empty Value 仍是资源；Apollo 删除 item 只改草稿。Nacos casMd5 与 Consul ModifyIndex 须在同一个写事务中比较明确的写入视图，不用灰度读取结果保护 normal 写入。无 CAS/幂等键请求沿用其覆盖语义，不承诺 exactly-once。
6. **分别生成协议状态。** 内部环境 release ID/revision 用于发布审计与一致性；客户端解析结果另带与有效 values 绑定的结果 revision，不能只按环境 release ID 缓存或通知。Apollo 有 opaque releaseKey 与可排序 notificationId；Nacos 对最终 content 计算 MD5；Consul 分配持久查询 index 与 key ModifyIndex。可复用内部变更事件，但不能把一个 release hash 填到全部字段。修改了模板但最终输出相同时，应按各协议的状态语义决定通知或 CAS 变化，而非全局规定“hash 相同就无新发布”。
7. **按受众读取后通知。** 灰度 profile 只用目标客户端真实传入且经该版本支持的 IP/label/tag/连接标签；模板和 values 的组合快照必须在同一次选择中冻结。Consul OSS KV profile 不应默默加入 per-client 灰度；主 ADR 规定固定视图的内容、存在状态、元数据与索引必须同事务物化，避免 GET 与 CAS/index 分别观察不同结果。

## 2026-10-03 实现范围

以上基础写入、条目物化与 CAS 为较早方案研究，未进入当前实现。现行 control-plane 配置适配为只读消费 profile；Nacos/Consul 写入明确拒绝，Consul 不实现 prefix、Flags、锁与完整条目创建索引。当前行为、索引映射和证据以[统一配置 v1 ADR](../adr/adr-config-template-values-unification.md)为准。本页上游协议资料不代表实现完成。

## 待验证与验收证据

本文没有运行官方客户端或核实本地兼容 handler。至少需要固定版本执行：Apollo 首次/304/通知超时、非 properties raw、public Namespace/cluster fallback、权限失败；Nacos v1 字节分隔符/初始 MD5/删除、2.x notify ACK/重连/空配置拒绝/casMd5、3.x v3 与 legacy 路由差异；Consul prefix 删除/游标回退/空值/二进制/CAS/Flags/ACL 与 dc 失败。

尤其需进一步明确：Nacos 开源目标 server 的自定义标签灰度实现与版本、非 Java SDK 的标签注入能力；Nacos 3.0 Client HTTP 文档与过渡 legacy SDK 支持的实际启动路由；Apollo 缓存 messages 的精确兼容；Consul 仅文本输出时对官方通用 KV 客户端的可用范围。当前资料只能支持限定 profile 设计，不能支持“完全替代”或“全协议兼容”的产品声明。

## 相关页面

- [API 服务端协议实现](api-servers.md)
- [配置模板与环境组合版本客户端渲染](../adr/adr-config-template-client-rendering.md)
- [配置中心统一模板与 Values 方案](../adr/adr-config-template-values-unification.md)
