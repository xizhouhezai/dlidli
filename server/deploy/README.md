# 本地 Docker 与 Kubernetes 环境（M3-ENG-01/02）

> 本机（Windows + WSL2）已安装 **Docker Desktop 4.91.0**，安装目录
> `D:\Program Files (x86)\docker`，镜像数据目录 `D:\DockerData`。

## 1. 安装位置与数据目录

| 项 | 路径 |
| --- | --- |
| Docker Desktop 程序 | `D:\Program Files (x86)\docker` |
| Docker CLI / Compose / kubectl | `D:\Program Files (x86)\docker\resources\bin` |
| 镜像与容器数据（vhdx） | `D:\DockerData` |

设置文件：`%APPDATA%\Docker\settings-store.json`（Docker Desktop 4.91 的主存储）
与 `%APPDATA%\Docker\settings.json`。

> **重要**：`settings.json` / `settings-store.json` 必须包含 `settingsVersion`（当前 45）
> 且**不能带 UTF-8 BOM**。两者任一不满足，`com.docker.backend` 会在启动时崩溃
> （日志报 `no settingsVersion found` 或 `invalid character 'ï'`）。
> 用 PowerShell 写这两个文件时应使用 `[System.IO.File]::WriteAllText($p, $json, (New-Object System.Text.UTF8Encoding($false)))`。

## 2. 网络：为什么必须配代理

本机直连 Docker Hub 不通（`registry-1.docker.io` 超时/重置），但本地 Clash
（`127.0.0.1:7890`）可以访问。因此 Docker Desktop 配置为手动代理指向 Clash：

```json
"proxyHttpMode": "manual",
"overrideProxyHttp": "http://127.0.0.1:7890",
"overrideProxyHttps": "http://127.0.0.1:7890"
```

这样镜像拉取统一走代理。**Clash 必须先启动**，否则 `docker pull` 会失败。

> WSL 内的 distro 无法直接访问宿主 `127.0.0.1:7890`（Clash 只监听回环地址）。
> 若需要在 WSL 内直接 `docker pull`，需在 Windows 侧做端口转发：
> `netsh interface portproxy add v4tov4 listenaddress=0.0.0.0 listenport=7891 connectaddress=127.0.0.1 connectport=7890`
> 并把 Clash 的「允许局域网连接」打开。当前 compose/kubectl 均走 Windows 侧 CLI，不需要该步骤。

## 3. 启动 compose 环境

```powershell
$env:PATH = "D:\Program Files (x86)\docker\resources\bin;$env:PATH"
cd server
docker compose -f deploy/docker-compose.yaml up -d
docker compose -f deploy/docker-compose.yaml ps
```

**端口冲突注意**：本机已有原生 MySQL(`3307`) 与 Redis(`6379`) 在跑。
`deploy/docker-compose.yaml` 中的 `redis` 映射到 `6379`，与原生实例冲突；
只起 MySQL 时可指定服务名：

```powershell
docker compose -f deploy/docker-compose.yaml up -d mysql
```

## 4. 构建并运行 API 镜像

```powershell
$env:PATH = "D:\Program Files (x86)\docker\resources\bin;$env:PATH"
cd server
docker build -f deploy/Dockerfile -t dlidli/api:local .
```

## 5. 本地 Kubernetes（可选，用于 M3-ENG-02 验证）

Docker Desktop 自带单节点 Kubernetes（kind 模式）。启用方式：

```json
// %APPDATA%\Docker\settings-store.json
"KubernetesEnabled": true,
"KubernetesVersion": "v1.36.1"
```

启用后 `kubectl` 上下文为 `docker-desktop`，首次启动需数分钟拉取控制面镜像。

### 5.1 HPA 需要 metrics-server

Docker Desktop 的 Kubernetes **默认不安装 metrics-server**，因此
`deploy/k8s/04-api-hpa.yaml` 部署后 `kubectl get hpa` 会显示
`<unknown>/70%`。这是预期现象，不是清单错误。安装后即可看到真实指标：

```powershell
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
# kind/kubeadm 自签证书环境需要放宽 TLS 校验：
kubectl -n kube-system patch deployment metrics-server --type=json `
  -p '[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--kubelet-insecure-tls"}]'
```

### 5.2 部署清单

```powershell
kubectl apply -f deploy/k8s/00-namespace.yaml
kubectl apply -f deploy/k8s/01-configmap.yaml
kubectl apply -f deploy/k8s/02-secret.yaml
kubectl apply -f deploy/k8s/06-data.yaml
kubectl apply -f deploy/k8s/03-api.yaml
kubectl apply -f deploy/k8s/04-api-hpa.yaml
kubectl apply -f deploy/k8s/05-api-pdb.yaml
kubectl -n dlidli get pods,svc,hpa
```

**本地验证的边界**（务必如实记录，不要当成生产验收）：

- `06-data.yaml` 的 MySQL/Redis 使用 `emptyDir`，Pod 重建即丢数据，**仅用于打通链路**；
- `02-secret.yaml` 中的凭据是本地占位值，生产必须改用 External Secrets/Sealed Secrets；
- 生产应使用托管数据库或带 PVC 的 Operator，而非单副本 Deployment；
- 本机没有线上环境，因此**不做真实部署验收与压测**，只验证清单可被 API Server 接受、
  Pod 能起来、健康检查与 HPA 目标可解析。
