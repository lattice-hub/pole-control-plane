# Pole 本地 Kubernetes 部署

> 当前独立 Helm 环境（`orbstack/pole-ui-review`）通过 `http://lattice-hub.localhost/` 访问，见 [OrbStack 域名入口](README.orbstack.md)。下文为依赖 `tidemind` 的既有共享部署方式。

本目录在 `pole-system` namespace 编排 Pole Control Plane（`full` profile，内含 Limiter Server）和 OpenTelemetry Collector。观测存储复用 `tidemind/maas-greptimedb-frontend`，Pole 通过本 namespace 的 `pole-greptimedb` ExternalName Service 保持稳定依赖接口，并使用独立逻辑库 `pole_observability`。MySQL 不进入 Kubernetes，Pod 通过 `pole-mysql` ExternalName Service 访问宿主机 `host.docker.internal:3306`。Console 复用 `tidemind/tidemind-gateway`，通过跨 namespace `HTTPRoute` 暴露域名。

## 部署

前置条件：

- 当前 `kubectl` context 指向本地 Kubernetes，例如 OrbStack。
- 宿主机 MySQL `3306` 可从 Pod 访问，并已创建 `pole_server`、`pole_observability` 数据库。
- 如果 MySQL 由 Docker 容器提供，应配置 `--restart unless-stopped`；既有容器可执行 `docker update --restart unless-stopped pole-mysql`，避免 OrbStack/Docker 重启后 Control Plane 因数据库不可达持续 CrashLoop。
- 已安装 Docker、Go、Node.js 和 npm。
- 集群中已有 `tidemind/tidemind-gateway`，其 `http` listener 允许同 namespace 的 Route。
- 集群中已有 Ready 的 `tidemind/maas-greptimedb-frontend` Deployment；部署脚本会幂等创建 `pole_observability` 逻辑库。

```bash
./deploy/kubernetes/build-image.sh

MYSQL_USER=root MYSQL_PWD=123456 \
POLE_CONSOLE_JWT_SECRET='replace-with-local-console-secret' \
POLE_AUTH_SALT='1234567890123456' \
  ./deploy/kubernetes/deploy-local.sh
```

密码、Console JWT Secret 和认证 Salt 通过 `pole-runtime-secrets` Secret 注入，不写入 manifests。可使用环境变量覆盖：

```bash
MYSQL_USER=root \
MYSQL_PWD='your-password' \
POLE_CONSOLE_JWT_SECRET='your-console-secret' \
POLE_AUTH_SALT='16-or-24-or-32-byte-salt' \
./deploy/kubernetes/deploy-local.sh
```

## 访问与验证

本地部署完成后优先使用域名访问：

```text
http://pole.localhost/
http://pole.localhost/agent
```

`.localhost` 由浏览器和系统解析到本机，不需要修改 `/etc/hosts`。`HTTPRoute` 位于 `tidemind` namespace；`pole-system` 中的 `ReferenceGrant` 按来源 namespace 和目标 Service 收敛授权，只允许 `tidemind` 的 HTTPRoute 引用 `pole-control-plane:8080`，不会暴露 GreptimeDB、Collector、Limiter 运维端口或 MySQL。集群内 SDK 通过 `pole-limiter:8101` 使用限流协议，Console 通过内部 `8102` 查询租约运行态。

```bash
kubectl -n pole-system get pods,svc,pvc
kubectl -n tidemind get httproute pole-console
kubectl -n pole-system logs deployment/pole-control-plane --tail=100
kubectl -n pole-system logs deployment/pole-otel-collector --tail=100
```

如果 Console 入口拒绝连接且 `pole-control-plane` 为 `CrashLoopBackOff`，先检查前一容器日志和宿主机 MySQL：

```bash
kubectl -n pole-system logs deployment/pole-control-plane --previous --tail=100
docker inspect pole-mysql --format 'status={{.State.Status}} restart={{.HostConfig.RestartPolicy.Name}}'
docker start pole-mysql
docker update --restart unless-stopped pole-mysql
kubectl -n pole-system rollout restart deployment/pole-control-plane
kubectl -n pole-system rollout status deployment/pole-control-plane --timeout=240s
```

`pole-control-plane` 默认使用 `ClusterIP`，宿主机只通过 `pole.localhost` 的 Gateway 访问 Console，
不会再把 `127.0.0.1:8080` 作为隐式入口。如需绕过 Gateway 排障或调试 Pole 的 HTTP、gRPC、
Nacos、Apollo、Eureka、xDS 等协议，显式启动临时端口转发：

```bash
kubectl --context orbstack -n pole-system port-forward service/pole-control-plane 18080:8080 18090:8090
curl -fsS http://127.0.0.1:18080/
```

## 数据边界

- Pole 的 metrics、traces 和结构化 event/audit logs 写入共享 GreptimeDB 的 `pole_observability` 逻辑库；MaaS 继续使用自己的逻辑库，双方不共享业务表。
- `pole-greptimedb` 只是 Pole namespace 内的稳定适配服务，不拥有 GreptimeDB 生命周期；物理集群由 `tidemind` 维护。
- 从旧 standalone 部署迁移时，脚本在新链路验证并完成 rollout 后删除旧 StatefulSet，但保留 `data-pole-greptimedb-0` PVC，便于历史数据导出或回退；确认不再需要前不要手工删除。
- Pole 本地 OTel 可靠队列和日志使用 `emptyDir`，Pod 重建后不保留；生产部署应替换为 PVC 或外部队列。
- 宿主机 MySQL 是部署外部依赖，删除 namespace 不会删除 MySQL 数据。
- 旧 Docker Compose 栈不会被部署脚本自动删除；完成数据迁移和验证后再停掉，避免误删原有 GreptimeDB volume。
