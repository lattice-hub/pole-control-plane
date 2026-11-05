# Pole Helm Chart

此 Chart 以拆分 workload 部署 Pole Control Plane、Console、Limiter，并在 Kubernetes 内部署 MySQL、GreptimeDB 与 OpenTelemetry Collector。

- 通用安装使用 `values.yaml`。
- 阿里云 ACK 生产基线叠加 `values-aliyun-prod.yaml`。
- 默认引用预先创建的 Kubernetes Secret `pole-runtime-secrets`。
- 生产必须设置 Pole 镜像 SemVer 与 digest。

完整安装、升级、回滚、备份恢复和验收步骤见
`context-kg/technical/Environment/kubernetes-production-sop.md`。
