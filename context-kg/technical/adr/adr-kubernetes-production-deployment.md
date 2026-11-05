---
title: Kubernetes 生产部署基线
tags: [adr, kubernetes, helm, ack, ghcr, backup]
links: [configuration, kubernetes-production-sop]
updated: 2026-08-18
sources: 18
---

# Kubernetes 生产部署基线

## 状态

已接受。正式交付物位于 `deploy/charts/pole/`，ACK 生产覆盖文件为
`values-aliyun-prod.yaml`，操作步骤见 [[kubernetes-production-sop]]。

## 决策

1. 使用一个 Helm Chart 管理 Pole、MySQL、GreptimeDB 与 OpenTelemetry Collector；云厂商能力仅放入覆盖 values，不进入通用模板。
2. Pole 使用同一镜像。生产默认 `deploymentMode=split`，拆分为 Control Plane StatefulSet、Console Deployment、
   Limiter Deployment；demo/轻量环境可用 `deploymentMode=full`，在 Control Plane Pod 中同时启动三个模块。
3. MySQL 8.4 单副本 StatefulSet 保存 `pole_server` 与 `pole_observability`；GreptimeDB Standalone StatefulSet保存遥测；两者各自使用 PVC。
4. Console 是唯一公网入口。通用 Chart 默认不开 Ingress；ACK profile 复用平台提供的 `alb` IngressClass，并要求 HTTPS。其它 HTTP/gRPC、协议、数据库和遥测端口均为 ClusterIP。
5. 初始阶段仅使用 Kubernetes 原生 Secret。Chart 默认引用安装前创建的 `pole-runtime-secrets`，不把明文密码写入 values 或 release history。
6. GitHub Release 触发 Actions 构建 `linux/amd64` 与 `linux/arm64`，推送公开 GHCR 包并生成 SBOM 与 provenance attestation。生产必须同时固定 SemVer 和 manifest digest。
7. MySQL 与 GreptimeDB 的定时备份由 CronJob 直接上传 OSS；凭据放入同一原生 Secret。恢复演练是上线门禁，不把“对象已上传”视为“可恢复”。
8. Helm fullname 遵循标准语义：release 名已经包含 chart 名时直接使用 release 名。因此 `helm ... pole
   deploy/charts/pole` 生成 `pole-*`，不生成 `pole-pole-*`。已有 StatefulSet 改名必须通过 `existingClaim`
   复用 PVC，禁止为了名称整洁重建数据盘。

## 运行时边界

| 组件 | workload | 数据 | 公开入口 |
|---|---|---|---|
| Control Plane | StatefulSet | `/app/.pole_data` PVC | 否 |
| Console | Deployment，默认 2 副本 | 临时目录 | 仅 HTTPS Ingress |
| Limiter | Deployment | 无持久数据 | 否 |
| MySQL | StatefulSet，1 副本 | `/var/lib/mysql` PVC | 否 |
| GreptimeDB | StatefulSet，Standalone | `/greptimedb_data` PVC | 否 |
| OTel Collector | Deployment | 无持久数据 | 否 |

Control Plane 以 Pod IP 自注册；Service DNS 只作为稳定客户端入口。拆分部署时 Console 指向 Control Plane Service，Limiter 指向 Control Plane gRPC Service，Limiter 广播 Pod IP，不能使用 `127.0.0.1`。

`full` 模式不额外创建 Console/Limiter Deployment。Console 与 Limiter Service 保留稳定名称，但 selector 指向
Control Plane Pod，并分别映射到 8080、8101、8102。进程内依赖使用 loopback；该拓扑共享副本数、资源限制、
故障域和滚动重启，仅适用于已明确接受这些取舍的 demo/轻量环境。ACK demo 覆盖文件为
`values-aliyun-demo.yaml`。

## 安全与供应链

- workload 默认禁用 ServiceAccount token、启用 RuntimeDefault seccomp、丢弃 Linux capabilities，并以非 root 运行 Pole 容器。
- Secret 不进入 Git、Helm values、命令历史或日志。原生 Secret 只解决分发问题，不提供云 KMS、轮换审计或静态加密保证；集群必须启用 etcd encryption at rest 并限制 namespace RBAC。
- 所有生产镜像都保留 digest 字段。Pole digest 是强制上线输入；依赖镜像也应在环境验收后补齐 digest。
- NetworkPolicy 仅在 CNI 执行网络策略时生效；当前策略用于约束入口，不等同于完整零信任或默认拒绝 egress。
- Swagger、pprof 和内部管理端点不得通过 ALB 或额外 LoadBalancer 暴露。

## ACK 适配

- 应用 Chart 不创建 `AlbConfig` 或 ALB 实例；平台团队统一维护公网 ALB、证书、WAF/ACL 与 IngressClass。
- PVC 显式使用 `alicloud-disk-topology-alltype`，依赖 ACK CSI 的 `WaitForFirstConsumer` 调度语义。
- Terway 场景可使用 ClusterIP 后端；Flannel 场景部署前必须按 ACK ALB Controller 约束确认后端服务类型，必要时将 `console.service.type` 覆盖为 `NodePort`。
- OSS 使用内网 endpoint；CronJob 通过 S3 兼容 API上传 MySQL dump，GreptimeDB 使用其原生 OSS 导出命令。

## 已知限制

- MySQL 单主和 GreptimeDB Standalone 都是单点故障，不宣称跨可用区高可用；HA operator/profile 是后续演进项。
- Control Plane 默认 1 副本；扩容前必须完成自注册、缓存一致性、`.pole_data` 独立 PVC 和滚动升级演练。
- 本交付提供离线渲染、客户端 schema、容器构建和专项测试证据；真实 ACK 的 ALB、云盘、OSS、域名与证书验收只能在目标账号内完成。

## 相关页面

- [[configuration]]
- [[kubernetes-production-sop]]
