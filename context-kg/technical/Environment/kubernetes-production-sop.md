---
title: Kubernetes 生产部署 SOP
tags: [kubernetes, helm, ack, operations, backup]
links: [configuration, adr-kubernetes-production-deployment]
updated: 2026-08-18
sources: 0
---

# Kubernetes 生产部署 SOP

本手册是 [[adr-kubernetes-production-deployment]] 的执行入口。命令中的域名、区域、bucket、版本和 digest 都是占位符，必须由环境负责人替换。

## 1. 交付物

- Chart：`deploy/charts/pole/`
- 通用默认值：`deploy/charts/pole/values.yaml`
- ACK 覆盖值：`deploy/charts/pole/values-aliyun-prod.yaml`
- ACK demo 合并模式覆盖值：`deploy/charts/pole/values-aliyun-demo.yaml`
- 发布镜像：`ghcr.io/lattice-hub/pole-control-plane:<semver>@sha256:<digest>`
- 发布流水线：`.github/workflows/docker.yml`

## 2. 上线前置门禁

目标集群应为受支持的 Kubernetes 版本并具备 Helm 3、CoreDNS、Metrics、可执行 NetworkPolicy 的 CNI 和动态 PVC。ACK 生产环境额外检查：

```bash
kubectl version
helm version
kubectl get nodes -o wide
kubectl get ingressclass alb
kubectl get storageclass alicloud-disk-topology-alltype
kubectl -n kube-system get pods | grep -E 'alb|csi|terway|flannel'
```

必须事先准备：公网域名、ACK ALB HTTPS 证书、OSS bucket 与最小权限 RAM 用户、备份生命周期策略、GHCR 公网拉取连通性。ACK ALB 的 `Ingress -> IngressClass -> AlbConfig -> ALB` 由平台侧维护，应用安装不新建 ALB。

若集群使用 Flannel，先按 ACK ALB Controller 当前限制确认后端 Service；需要 NodePort 时在环境 values 中设置：

```yaml
console:
  service:
    type: NodePort
```

## 3. 发布 GHCR 镜像

1. 在 GitHub 创建并发布 SemVer Release，例如 `v1.0.0`。
2. 等待 `Publish GHCR image` 成功；确认多架构、SBOM 与 attestation。
3. 在 GitHub Packages 将容器包可见性设为 Public，首次发布后用匿名环境验证拉取。
4. 解析不可变 digest：

```bash
docker buildx imagetools inspect ghcr.io/lattice-hub/pole-control-plane:v1.0.0
```

复制 manifest list 的 `sha256:...` 到生产 values；禁止部署 `latest`、空 tag 或仅 tag 无 digest 的镜像。

## 4. 创建 namespace 与原生 Secret

先创建 namespace：

```bash
kubectl create namespace pole-system --dry-run=client -o yaml | kubectl apply -f -
```

在受控终端建立权限为 `0600` 的本地 env 文件，不要提交到 Git：

```dotenv
MYSQL_ROOT_PASSWORD=<随机强密码>
MYSQL_USER=pole
MYSQL_PWD=<随机强密码>
CONSOLE_JWT_SECRET=<至少32字节随机值>
AUTH_SALT=<16、24或32字节随机值>
SYSTEM_SECRET_MASTER_KEY=<32随机字节的base64文本>
SELF_MANAGER_PROBE_KEY=<随机强密码>
OSS_ACCESS_KEY_ID=<最小权限RAM AccessKey ID>
OSS_ACCESS_KEY_SECRET=<最小权限RAM AccessKey Secret>
```

创建或更新 Secret：

```bash
kubectl -n pole-system create secret generic pole-runtime-secrets \
  --from-env-file=./pole-runtime-secrets.env \
  --dry-run=client -o yaml | kubectl apply -f -
kubectl -n pole-system get secret pole-runtime-secrets
```

随后安全删除本地文件并确认 shell history、CI 日志和工单没有明文。Secret 必须包含上述九个 key；不要用 `kubectl get secret -o yaml` 作为验收截图。

## 5. 生成环境 values

复制 ACK 示例为不入库的环境文件，至少修改：

```yaml
global:
  clusterName: pole-ack-production
image:
  repository: ghcr.io/lattice-hub/pole-control-plane
  tag: v1.0.0
  digest: sha256:<实际manifest-list-digest>
ingress:
  host: pole.example.com
backup:
  oss:
    bucket: pole-production-backup
    endpoint: https://oss-cn-hangzhou-internal.aliyuncs.com
    region: cn-hangzhou
    prefix: pole/production
```

同时根据容量评估调整 PVC、requests/limits、备份时间和 TTL。依赖镜像经验证后也填入 digest。域名证书在平台管理的 AlbConfig/ALB 中绑定，不在 Secret 中存放私钥。

## 6. 预检与首次安装

```bash
helm lint deploy/charts/pole -f deploy/charts/pole/values-aliyun-prod.yaml -f ./values-prod.local.yaml
helm template pole deploy/charts/pole \
  --namespace pole-system \
  -f deploy/charts/pole/values-aliyun-prod.yaml \
  -f ./values-prod.local.yaml > /tmp/pole-rendered.yaml
kubectl apply --dry-run=server -f /tmp/pole-rendered.yaml
helm upgrade --install pole deploy/charts/pole \
  --namespace pole-system \
  -f deploy/charts/pole/values-aliyun-prod.yaml \
  -f ./values-prod.local.yaml \
  --rollback-on-failure --wait --timeout 20m
```

`--dry-run=server` 必须连接目标 ACK 执行，能发现 CRD、准入策略、配额和版本差异。Helm 4 使用
`--rollback-on-failure`；它只回滚 Kubernetes release，不回滚已经执行的数据库变更。

### Demo 合并模式

Demo/轻量环境可以在 ACK 云盘基线之上追加 demo overlay：

```bash
helm lint deploy/charts/pole \
  -f deploy/charts/pole/values-aliyun-prod.yaml \
  -f deploy/charts/pole/values-aliyun-demo.yaml
helm upgrade --install pole deploy/charts/pole \
  --namespace pole-system \
  -f deploy/charts/pole/values-aliyun-prod.yaml \
  -f deploy/charts/pole/values-aliyun-demo.yaml \
  --set-string image.tag="$POLE_IMAGE_TAG" \
  --set-string image.digest="$POLE_IMAGE_DIGEST" \
  --rollback-on-failure --wait --timeout 20m
```

该 overlay 设置 `deploymentMode=full`，只保留一个 Pole StatefulSet；Console/Limiter Service 指向同一 Pod。
不要把它当作生产 HA 拓扑。迁移后必须确认独立 Console/Limiter Deployment 已删除，同时 8080、8101、8102
仍通过各自 ClusterIP Service 可达。

### 资源命名与已有 PVC 迁移

release 名为 `pole` 时，规范资源名是 `pole-control-plane`、`pole-console`、`pole-mysql` 等。首次安装前检查
渲染结果不得出现 `pole-pole-*`：

```bash
helm template pole deploy/charts/pole -n pole-system | grep '^  name:'
```

如果修正 fullname 前已经创建 StatefulSet，不得直接升级并生成空白云盘。先记录 PVC、PV、reclaimPolicy 和
数据库指纹，再在不入库的环境 overlay 中指定：

```yaml
controlPlane:
  persistence:
    existingClaim: data-legacy-control-plane-0
mysql:
  persistence:
    existingClaim: data-legacy-mysql-0
greptimedb:
  persistence:
    existingClaim: data-legacy-greptimedb-0
```

先停止写入方和 OTel，再依次缩容旧 StatefulSet；确认 RWO 云盘已卸载后，带该 overlay 执行
`helm upgrade --rollback-on-failure --wait`。升级后必须比对 PV ID、MySQL schema/table 数、GreptimeDB 行数和
OTel canary。旧 PVC 名可以作为迁移桥保留；不要仅为名称一致而删除并重建 PVC。

## 7. 安装验收

### 基础设施

```bash
kubectl -n pole-system get pods,pvc,svc,ingress,cronjob
kubectl -n pole-system get events --sort-by=.lastTimestamp
kubectl -n pole-system rollout status statefulset/pole-pole-mysql --timeout=10m
kubectl -n pole-system rollout status statefulset/pole-pole-greptimedb --timeout=10m
kubectl -n pole-system rollout status statefulset/pole-pole-control-plane --timeout=10m
kubectl -n pole-system rollout status deployment/pole-pole-console --timeout=10m
kubectl -n pole-system rollout status deployment/pole-pole-limiter --timeout=10m
```

`deploymentMode=full` 时不应存在最后两个 Deployment；改为检查 Control Plane Pod 同时监听 8080、8101、8102，
并验证 Console 根路径、Limiter maintenance 根路径和 `/maintain/quota-leases`。

所有 PVC 必须 Bound，Pod Ready 且零持续重启。检查 Service，只有 Console Ingress 应有公网地址；不得额外存在 MySQL、GreptimeDB、OTel 或协议端口的 LoadBalancer。

### 数据与 API

```bash
kubectl -n pole-system exec pole-pole-mysql-0 -- \
  mysql -upole -p -e 'SHOW DATABASES; USE pole_server; SHOW TABLES;'
kubectl -n pole-system exec pole-pole-greptimedb-0 -- \
  curl -fsS http://127.0.0.1:4000/health
kubectl -n pole-system port-forward svc/pole-pole-control-plane 18090:8090
curl -fsS http://127.0.0.1:18090/admin/v1/server/functions
```

MySQL 密码交互输入，不写进命令。再从测试客户端验证 Nacos/Apollo/Eureka/xDS 与 Limiter 的内部地址；不得从公网直接访问这些端口。向 OTel Collector 写入一条 canary trace/metric/log，并从 GreptimeDB 或 Console 查询回读，避免把“Collector Ready”误判为链路验收。

### 公网 Console

```bash
curl -fsSI http://pole.example.com/
curl -fsS https://pole.example.com/admin/v1/server/functions
openssl s_client -connect pole.example.com:443 -servername pole.example.com </dev/null
```

HTTP 应跳转 HTTPS，证书链和域名应有效，登录、核心资源列表与一次受权限保护的变更应通过真实浏览器验收。仅返回静态首页不等于产品 E2E。

## 8. 备份与恢复演练

CronJob 默认每天 02:00 执行。上线前手动触发并检查 OSS 对象：

```bash
kubectl -n pole-system create job --from=cronjob/pole-pole-mysql-backup mysql-backup-drill
kubectl -n pole-system create job --from=cronjob/pole-pole-greptimedb-backup greptime-backup-drill
kubectl -n pole-system logs job/mysql-backup-drill --all-containers
kubectl -n pole-system logs job/greptime-backup-drill --all-containers
```

MySQL 恢复流程：停止写入、取得目标时间点 dump、在隔离实例导入两个数据库、执行行数/约束/核心 API 校验，再切换生产。GreptimeDB 使用同版本 `greptime cli data import` 从 OSS 导入隔离库，完成 trace/metric/log 查询校验后再切换。每季度演练并记录 RPO、RTO、对象校验和与恢复责任人；未完成恢复演练不得宣称备份可用。

## 9. 升级与回滚

升级前完成备份、阅读 release note、确认新 digest、执行 Helm diff 或模板差异审查：

```bash
helm history pole -n pole-system
helm upgrade pole deploy/charts/pole -n pole-system \
  -f deploy/charts/pole/values-aliyun-prod.yaml \
  -f ./values-prod.local.yaml \
  --rollback-on-failure --wait --timeout 20m
```

应用回滚：

```bash
helm rollback pole <REVISION> -n pole-system --wait --timeout 20m
```

回滚后重复第 7 节。若新版本执行了不向后兼容的 schema 迁移，不允许只回滚镜像；必须使用该版本专属恢复计划或数据库备份。

## 10. 卸载与数据保留

先暂停入口和写入、完成最终备份并确认 OSS，再执行 `helm uninstall pole -n pole-system`。StatefulSet 的 volumeClaimTemplate PVC 通常会保留，但不要依赖默认行为：卸载前记录 PV/PVC、reclaimPolicy、快照和 bucket 对象。PVC、PV、快照、Secret 与 OSS 对象只能依据经过审批的明确清单删除，禁止用宽泛 label 或 namespace 级联误删生产数据。

## 11. 故障排查顺序

1. `kubectl get events`：调度、云盘、配额、镜像拉取。
2. initContainer 日志：MySQL 是否可用、Control Plane 是否 Ready。
3. workload 前一容器日志：`kubectl logs -p`，确认 CrashLoop 根因。
4. DNS/Service/Endpoints：确认拆分进程未回退到 loopback。
5. ALB Controller 事件、IngressClass 与 AlbConfig：定位公网入口和证书问题。
6. CronJob、OSS endpoint 与 RAM 权限：定位备份失败。
7. 回滚前先判断是否包含数据库兼容性变化。

## 12. 官方参考

- [ACK ALB Ingress](https://www.alibabacloud.com/help/en/slb/application-load-balancer/use-an-alb-ingress-to-expose-services-in-an-ack-cluster)
- [ACK AlbConfig](https://www.alibabacloud.com/help/en/ack/serverless-kubernetes/user-guide/use-albconfigs-to-configure-alb-instances)
- [ACK 云盘动态卷](https://www.alibabacloud.com/help/en/ack/ack-managed-and-ack-dedicated/user-guide/use-dynamically-provisioned-disk-volumes/)
- [MySQL 官方容器](https://github.com/docker-library/docs/blob/master/mysql/README.md)
- [GreptimeDB Standalone](https://docs.greptime.com/getting-started/installation/greptimedb-standalone/)
- [OpenTelemetry Collector 容器安装](https://opentelemetry.io/docs/collector/install/docker/)
- [GHCR 容器发布](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry)

## 相关页面

- [[configuration]]
- [[adr-kubernetes-production-deployment]]
