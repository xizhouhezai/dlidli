# 部署与运行指南

> 状态：`V1.0` ｜ 更新日期：2026-07-31 ｜ 覆盖：前端（web/admin/h5）+ 后端（Go API）｜ 本地开发 + 生产上线

本文档提供 DliDli 从零到运行的完整步骤，分**本地开发环境**与**生产上线部署**两大部分。命令默认在仓库根目录 `dlidli/` 执行，特殊目录会显式标注。

## 1. 技术栈与端口速查

| 组件 | 技术 | 本地端口 | 说明 |
| --- | --- | :-: | --- |
| 后端 API | Go 1.25 + Gin | 8000 | 核心业务接口 + Swagger |
| Web C 端 | Vue3 + Vite | 5174 | 用户站 |
| 管理后台 | Vue3 + Vite | 5175 | apps/admin |
| H5 | uni-app | 5176+ | 移动端 |
| 文档站 | VitePress | 5173 | 本文档 |
| MySQL | 8.0 | 3306/3307 | 主数据库 |
| Redis | 7 | 6379 | 缓存/会话/计数 |

> Vite dev 端口若被占用会自动 +1，以启动日志实际输出为准。

## 2. 前置依赖

| 依赖 | 版本 | 用途 | 校验命令 |
| --- | --- | --- | --- |
| Node.js | ≥ 20 | 前端构建 | `node -v` |
| pnpm | ≥ 9（推荐 10.18.3） | 包管理（monorepo） | `pnpm -v` |
| Go | ≥ 1.25 | 后端 | `go version` |
| MySQL | 8.0 | 数据库 | — |
| Redis | 7 | 缓存 | — |
| FFmpeg / ffprobe | 任意近版 | 视频转码/抽帧 | `ffmpeg -version` |
| Docker（可选） | — | 一键起中间件 | `docker -v` |

安装 pnpm：`npm i -g pnpm`。FFmpeg（Windows）：`winget install Gyan.FFmpeg`，安装后确认 `ffmpeg`/`ffprobe` 在 PATH，否则在配置里写绝对路径（见 §4.3）。

---

## 3. 本地开发 - 快速开始（TL;DR）

```bash
# 0. 克隆 + 安装前端依赖
git clone git@github.com:xizhouhezai/dlidli.git && cd dlidli
pnpm install

# 1. 起中间件（二选一）
#    A) 有 Docker：一键起 MySQL/Redis/Kafka/MinIO
docker compose -f server/deploy/docker-compose.yaml up -d
#    B) 无 Docker：本地自备 MySQL(3307) + Redis(6379)，见 §4.1

# 2. 初始化数据库（建表）
cd server && go run ./cmd/migrate && cd ..

# 3. 启动后端 API（新终端，端口 8000）
cd server && go run ./cmd/api

# 4. 启动前端（各开一个终端）
pnpm web:dev      # 用户站   :5174
pnpm admin:dev    # 管理后台 :5175
pnpm docs:dev     # 文档站   :5173

# 5. 验证
#    http://localhost:8000/health          后端健康检查
#    http://localhost:8000/swagger/index.html  接口文档
#    http://localhost:5174   用户站   /   http://localhost:5175  管理后台
```

> 管理后台默认账号：`admin / admin123`（首次启动后端自动创建，请尽快改密）。

---

## 4. 本地开发 - 详细步骤

### 4.1 准备中间件（MySQL + Redis）

**方式 A：Docker（推荐，一键起全套）**

```bash
docker compose -f server/deploy/docker-compose.yaml up -d
docker compose -f server/deploy/docker-compose.yaml ps   # 查看状态
```

该 compose 起 MySQL(3306, root/dlidli123, 库 dlidli)、Redis(6379)、Kafka(9092)、MinIO(9000/9001)。若用它，需把后端配置 DSN 改为 `root:dlidli123@tcp(127.0.0.1:3306)/dlidli`（见 §4.3）。

**方式 B：本地自备（无 Docker）**

自行安装并启动 MySQL 与 Redis，然后建库：

```sql
CREATE DATABASE IF NOT EXISTS dlidli DEFAULT CHARSET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

默认配置 `server/configs/dev.yaml` 指向 MySQL `127.0.0.1:3307 root/root`、Redis `127.0.0.1:6379`。按你的实际端口/账密调整 DSN。

### 4.2 安装前端依赖

```bash
pnpm install     # 根目录执行，一次性装齐 web/admin/h5/docs/packages 全部工作区
```

### 4.3 后端配置

配置文件：`server/configs/dev.yaml`。关键项：

```yaml
mysql:
  dsn: "root:root@tcp(127.0.0.1:3307)/dlidli?charset=utf8mb4&parseTime=True&loc=Local"
redis:
  addr: 127.0.0.1:6379
jwt:
  secret: "dev-secret-do-not-use-in-prod"   # 生产必须用环境变量覆盖
storage:
  driver: local
  localDir: ./uploads
  baseUrl: http://localhost:8000/static
transcode:
  enabled: true
  ffmpegPath: ffmpeg     # 不在 PATH 时填绝对路径，如 C:\...\ffmpeg.exe
  ffprobePath: ffprobe
```

任意配置项都可用环境变量覆盖，前缀 `DLIDLI_`、`.` 换 `_`。例：

```bash
# 覆盖数据库 DSN 与 JWT 密钥（无需改文件）
DLIDLI_MYSQL_DSN="root:pwd@tcp(127.0.0.1:3306)/dlidli?..." DLIDLI_JWT_SECRET="xxx" go run ./cmd/api
```

### 4.4 数据库迁移

```bash
cd server
go run ./cmd/migrate           # 应用全部未执行迁移（建表）
go run ./cmd/migrate -down     # 回滚一步
go run ./cmd/migrate -dsn "..." # 指定库
```

> 迁移不随 API 启动自动执行，改表后需手动跑。迁移文件在 `server/scripts/migrations/`。

### 4.5 启动后端

```bash
cd server
go run ./cmd/api                       # 默认读 configs/dev.yaml
go run ./cmd/api -config configs/dev.yaml
```

启动后：健康检查 `curl http://localhost:8000/health`；接口文档 `http://localhost:8000/swagger/index.html`。

> 后端内嵌转码 Worker（dev），投稿后自动转码；生产由独立 worker 承担。

### 4.6 启动前端

```bash
pnpm web:dev      # 用户站   http://localhost:5174
pnpm admin:dev    # 管理后台 http://localhost:5175
pnpm h5:dev       # H5       http://localhost:5176
pnpm docs:dev     # 文档站   http://localhost:5173
```

前端 dev 通过 Vite 代理把 `/api`、`/static` 转发到后端 `:8000`（见各 `apps/*/vite.config.ts` 的 `server.proxy`），无需额外配置跨域。

### 4.7 常见问题

| 现象 | 原因/排查 |
| --- | --- |
| 后端启动报 MySQL/Redis 连接失败 | 中间件未起或 DSN/端口不符；核对 §4.1/4.3 |
| 投稿后视频不转码 | FFmpeg 不在 PATH；在 dev.yaml 填 ffmpegPath/ffprobePath 绝对路径 |
| 前端 `/api` 404 | 后端未启动，或 vite proxy 目标端口与后端不一致 |
| 管理后台无法登录 | 后端首启才创建 admin/admin123；确认 DB 已迁移 |
| 端口被占用 | Vite 自动换端口，以日志为准；后端改 `app.port` 或杀占用进程 |

---

## 5. 生产上线部署

生产采用**前后端分离**：前端构建为静态资源交给 Nginx/CDN；后端编译为单二进制常驻运行；中间件用托管实例。

### 5.1 后端上线

**① 交叉编译二进制**（在 server/ 下）

```bash
cd server
go build -o dlidli-api ./cmd/api
go build -o dlidli-migrate ./cmd/migrate
# 交叉编译到 Linux（在 Windows/macOS 打包）：
#   $env:GOOS="linux"; $env:GOARCH="amd64"; go build -o dlidli-api ./cmd/api
```

**② 生产配置**：复制一份 `configs/prod.yaml`（env 改 `prod`），敏感项**一律用环境变量注入，不写入文件、不进仓库**：

```bash
export DLIDLI_APP_ENV=prod
export DLIDLI_MYSQL_DSN="user:pwd@tcp(db-host:3306)/dlidli?charset=utf8mb4&parseTime=True&loc=Local"
export DLIDLI_REDIS_ADDR="redis-host:6379"
export DLIDLI_JWT_SECRET="<强随机密钥>"
export DLIDLI_STORAGE_DRIVER=minio        # 生产用对象存储
export DLIDLI_STORAGE_BASEURL="https://cdn.example.com"
```

> `app.env=prod` 时 Gin 进入 release 模式，且 Swagger UI 路由**自动关闭**。

**③ 迁移 + 启动**（建议用 systemd 守护）

```bash
./dlidli-migrate -dsn "$DLIDLI_MYSQL_DSN"    # 上线前先迁移
./dlidli-api -config configs/prod.yaml       # 常驻
```

systemd 示例 `/etc/systemd/system/dlidli-api.service`：

```ini
[Unit]
Description=DliDli API
After=network.target
[Service]
WorkingDirectory=/opt/dlidli/server
EnvironmentFile=/opt/dlidli/server/.env.prod
ExecStart=/opt/dlidli/server/dlidli-api -config configs/prod.yaml
Restart=always
[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload && sudo systemctl enable --now dlidli-api
sudo journalctl -u dlidli-api -f     # 看日志
```

### 5.2 前端上线（web / admin）

**① 构建静态资源**（根目录）

```bash
pnpm install --frozen-lockfile
pnpm web:build      # 产出 apps/web/dist
pnpm admin:build    # 产出 apps/admin/dist
pnpm docs:build     # 产出 docs/.vitepress/dist（文档站，可选）
```

> 生产 API 地址：前端通过 Nginx 反代 `/api` 到后端，无需在构建时写死后端地址（沿用相对路径 `/api`）。若前端与后端不同域，需在构建环境配 `VITE_API_BASE` 之类变量或由 Nginx 统一同域反代。

**② 部署到 Nginx**（web 与 admin 分别用不同域名/子路径）

```nginx
# 用户站 www.example.com
server {
  listen 80;
  server_name www.example.com;
  root /opt/dlidli/web;            # apps/web/dist 上传至此
  location / { try_files $uri $uri/ /index.html; }   # SPA history 路由回退
  location /api/    { proxy_pass http://127.0.0.1:8000; proxy_set_header Host $host; proxy_set_header X-Real-IP $remote_addr; }
  location /static/ { proxy_pass http://127.0.0.1:8000; }
}

# 管理后台 admin.example.com（独立域名，与 C 端隔离）
server {
  listen 80;
  server_name admin.example.com;
  root /opt/dlidli/admin;          # apps/admin/dist
  location / { try_files $uri $uri/ /index.html; }
  location /api/ { proxy_pass http://127.0.0.1:8000; }
}
```

要点：
- **SPA 路由回退**：`try_files ... /index.html` 必配，否则刷新子路由 404。
- **管理后台建议独立域名 + IP 白名单**，与用户站物理隔离。
- 静态资源开启 gzip/brotli + 长缓存；`index.html` 不缓存。

### 5.3 H5 / 小程序上线

```bash
pnpm h5:build                       # H5：产出 apps/h5/dist/build/h5，按普通静态站部署
# 小程序：uni-app 编译 mp-weixin 目标后用微信开发者工具上传（后置，按需）
```

### 5.4 上线检查清单

- [ ] `app.env=prod`、Swagger 已关闭、Gin release 模式
- [ ] JWT 密钥、DB/Redis 密码均由环境变量注入，未进仓库
- [ ] 上线前已执行数据库迁移
- [ ] 存储切换为对象存储（MinIO/OSS），`baseUrl` 指向 CDN
- [ ] Nginx SPA 回退、`/api` 与 `/static` 反代正确
- [ ] 管理后台独立域名 + 访问控制
- [ ] 后端进程守护（systemd）+ 日志采集 + `/metrics` 接入监控

---

## 6. 相关文档

- [后端架构](/architecture/backend)：模块划分、API 规范、接口文档（Swagger）
- [前端架构](/architecture/frontend)：monorepo 布局、构建、图标/样式方案
- [协作规范](/project/conventions)：Git Flow 分支与发布流程
- 后端 README：`server/README.md`（命令速查）
