# OrbStack 独立本地环境：lattice-hub.localhost

当前 Console 位于 `orbstack` context、`pole-ui-review` namespace，由 Helm release `pole` 管理。
域名入口为 **http://lattice-hub.localhost/**，流量路径：

```text
浏览器 → OrbStack localhost:80 → Envoy Gateway → HTTPRoute → pole-console:8080
```

本环境不依赖旧部署中的 `tidemind-gateway`、共享数据库或 `pole-system` namespace。
`orbstack-gateway.yaml` 与应用 Helm release 分开管理；应用升级沿用这份路由，无须重建镜像。

## 安装与应用

先检查集群现状，已有 Gateway API CRD 或控制器时应复用或核对兼容性，避免重复安装。
当前环境首次安装使用 Envoy Gateway `v1.9.3`（支持本机 Kubernetes `v1.36`）：

```bash
helm upgrade --install envoy-gateway oci://docker.io/envoyproxy/gateway-helm \
  --version v1.9.3 --kube-context orbstack \
  --namespace envoy-gateway-system --create-namespace --wait --timeout 3m

kubectl --context orbstack apply -f deploy/kubernetes/orbstack-gateway.yaml
kubectl --context orbstack -n pole-ui-review wait \
  --for=condition=Programmed gateway/lattice-hub --timeout=120s
```

该 manifest 包含：

- `GatewayClass/lattice-hub-orbstack`：使用 Envoy Gateway 控制器。
- `pole-ui-review/Gateway/lattice-hub`：HTTP 80，仅匹配 `lattice-hub.localhost`，只接受同 namespace 的路由。
- `pole-ui-review/HTTPRoute/lattice-hub-console`：保留请求路径，转发到同 namespace 的 `pole-console:8080`。

路由只引用 Console，无跨 namespace `ReferenceGrant`，不公开 MySQL、GreptimeDB 或管理协议端口。
OrbStack 将 LoadBalancer 的 80 端口映射到本机；本机和浏览器将 `.localhost` 解析到回环地址，无须修改 `/etc/hosts`。
这是本地 HTTP 入口，未配置 TLS。当前本机实测 80 端口通过 IPv4 可达，强制 IPv6 连接不可达；浏览器域名访问已验证正常。排障时可用 `curl -4`。

## 验证与排障

```bash
kubectl --context orbstack get gatewayclass lattice-hub-orbstack
kubectl --context orbstack -n pole-ui-review get gateway,httproute
kubectl --context orbstack -n envoy-gateway-system get pods,svc
curl --noproxy '*' -4 -I http://lattice-hub.localhost/
# 可选：排查 IPv6 监听
curl --noproxy '*' -6 -I http://lattice-hub.localhost/
```

检查 `Gateway` 的 `Accepted=True`、`Programmed=True`，以及 `HTTPRoute` 的
`Accepted=True`、`ResolvedRefs=True`。域名访问不依赖 `kubectl port-forward`。
若本机 80 端口被其他容器或服务占用，先查明占用归属，避免覆盖别的项目入口。

当前环境已按用户要求清空应用数据并恢复首次安装状态。打开域名后会进入“初始化管理员账户”，由使用者设置首个管理员账号和密码；旧 `pole` 测试账号及本地凭据文件已经移除。

增加域名入口本身不重建应用 Pod、数据库或 PVC。之后按用户要求执行过独立的数据重置，三个应用 PVC 已重新创建，Gateway 配置保持不变。
如需撤销本应用入口，仅删除这份 manifest；不要在其他应用共用控制器时卸载 Envoy Gateway。

```bash
kubectl --context orbstack delete -f deploy/kubernetes/orbstack-gateway.yaml
```

参考：[Envoy Gateway 安装](https://gateway.envoyproxy.io/docs/install/install-helm/)、
[版本兼容矩阵](https://gateway.envoyproxy.io/news/releases/matrix/)、
[OrbStack Kubernetes 网络](https://docs.orbstack.dev/kubernetes)。
