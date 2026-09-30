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
#    B) 无 Docker：本地自备 MySQL + Redis（默认端口见 §4.1，注意本机 3306/3307 是两个独立实例）

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

`server/configs/dev.yaml` 默认指向 **MySQL `127.0.0.1:3306`、`root/dlidli123`**（对齐方式 A 的 compose 映射）与 **Redis `127.0.0.1:6379`**。用方式 B 时按你的实际端口/账密改 DSN。

> ⚠️ **两个本地 MySQL 实例极易混淆（曾导致数据"丢失"事故）**：本机 phpstudy 自带实例在 **3307**（`root/root`，`staging.yaml` 也用它连 `dlidli_staging`），Docker 实例在 **3306**（`root/dlidli123`，库名 `dlidli`）。二者是**完全独立的两套数据**，互不联通。
> 若你的稿件"在但播不了"、或首页只剩测试数据，先确认后端实际连的是哪一个：`curl http://127.0.0.1:8000/health` 看组件连通性，再用 `SELECT COUNT(*) FROM video` 比对两边行数。详见 §4.8。

### 4.2 安装前端依赖

```bash
pnpm install     # 根目录执行，一次性装齐 web/admin/h5/docs/packages 全部工作区
```

### 4.3 后端配置

配置文件：`server/configs/dev.yaml`。关键项：

```yaml
mysql:
  dsn: "root:dlidli123@tcp(127.0.0.1:3306)/dlidli?charset=utf8mb4&parseTime=True&loc=Local"
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
| 稿件在、但首页看不到 / 全部"暂无可播放的清晰度" | **数据库指错了**：应用连的库不是入库时那个实例，或库被重建过。按 §4.8 核对与恢复 |
| 上传/转码产物还在，但库里查不到对应稿件 | 同上。`uploads/` 不随数据库备份，重建库后元数据会与文件脱节 |

---

### 4.8 数据备份与恢复

> ⚠️ **本地数据分两处存放，缺一不可**。二者生命周期独立，只备份其中一个会导致"文件在、记录没了"或"记录在、文件 404"。

| 数据 | 位置 | 是否进 Git | 说明 |
| --- | --- | --- | --- |
| 业务数据 | MySQL 数据卷 `dlidli-dev_mysql-data`（容器内 `/var/lib/mysql`） | 否 | 稿件/用户/评论/弹幕/互动等全部元数据 |
| **媒体文件** | `server/uploads/`（宿主机目录，**不在 Docker 卷内**） | 否（被 `.gitignore` 忽略，可 `git check-ignore -v server/uploads` 复核） | 原片 `videos/source/`、HLS 切片 `videos/hls/<video_id>/`、封面 `covers/`、头像 `avatars/` |

**为什么必须一起备份**：`uploads/` 既不在版本库、也不在 MySQL 数据卷里，是**完全无备份的孤岛**。而 `video_stream.play_path` 存的是**相对 `uploads/` 的路径**（如 `videos/hls/<id>/<part>/<quality>/index.m3u8`），`video.cover` 同理。因此重建数据库卷后，磁盘上的转码产物虽仍在，却**没有任何记录指向它们**——表现为"稿件全部消失"或"暂无可播放的清晰度"。

**备份命令**（PowerShell，输出到 `.dev-logs/`，该目录已 gitignore）：

```powershell
$bk = ".dev-logs/db-backup-$(Get-Date -Format yyyyMMdd-HHmmss)"
New-Item -ItemType Directory -Force -Path $bk | Out-Null

# ① 数据库（Docker 实例 3306）
docker exec dlidli-mysql sh -c `
  "mysqldump -uroot -pdlidli123 --single-transaction --routines --triggers --events --default-character-set=utf8mb4 --databases dlidli > /tmp/db.sql"
docker cp dlidli-mysql:/tmp/db.sql "$bk/db.sql"

# ② 媒体文件（必须单独备份）
Copy-Item -Recurse -Force server/uploads "$bk/uploads"
```

**恢复顺序**（先库后文件，顺序反了不影响结果，但都必须在同一批次）：

```powershell
# ① 恢复数据库
docker cp "$bk/db.sql" dlidli-mysql:/tmp/db.sql
docker exec dlidli-mysql sh -c "mysql -uroot -pdlidli123 < /tmp/db.sql"
# 若只想补结构、不动数据，用增量迁移：cd server && go run ./cmd/migrate

# ② 恢复媒体文件
Copy-Item -Recurse -Force "$bk/uploads/*" server/uploads/

# ③ 核对两边是否一致（关键一步）
```

**一致性核对**（迁移/恢复后**必须**执行，`path` 与磁盘一一对应才算成功）：

```powershell
# 所有 play_path 指向的文件是否都在磁盘上（缺失应为 0）
docker exec dlidli-mysql mysql -uroot -pdlidli123 dlidli -N -e "SELECT play_path FROM video_stream;" |
  ForEach-Object { if (-not (Test-Path "server/uploads/$_")) { "缺失: $_" } }

# 库内 video 数 vs 磁盘 HLS 目录数（差距大即为脱节）
docker exec dlidli-mysql mysql -uroot -pdlidli123 dlidli -N -e "SELECT COUNT(*) FROM video;"
(Get-ChildItem server/uploads/videos/hls -Directory).Count
```

**从 `uploads/` 反推恢复线索**（元数据已丢但文件还在时）：

- HLS 目录名**就是 `video_id`**：`uploads/videos/hls/<video_id>/..`
- 封面名**含 `video_id`**：`uploads/covers/auto_<video_id>.jpg`
- 但**标题/UP主/简介/标签等人工信息无法从磁盘还原**——这些只在数据库里。所以备份数据库不是可选项。
- 若手头有旧的本地 MySQL 实例（如 phpstudy 的 3307），**优先从它整库迁回**，比从文件反推完整得多（下条）。

**从另一个 MySQL 实例整库迁回**（本机 3306/3307 双实例场景，2026-09-30 实战）：

```powershell
$old = "D:\phpstudy_pro\Extensions\MySQL8.0.12\bin\mysqldump.exe"   # 以实际安装路径为准
$bk  = ".dev-logs/db-migrate-$(Get-Date -Format yyyyMMdd-HHmmss)"
New-Item -ItemType Directory -Force -Path $bk | Out-Null

# ① 双份备份：旧库与当前库都要备（当前库用于回滚）
& $old --host=127.0.0.1 --port=3307 --user=root --password=root `
  --single-transaction --routines --triggers --events --default-character-set=utf8mb4 `
  --databases dlidli --result-file="$bk/old-3307.sql"
docker exec dlidli-mysql sh -c `
  "mysqldump -uroot -pdlidli123 --single-transaction --routines --triggers --events --default-character-set=utf8mb4 --databases dlidli > /tmp/new-3306.sql"
docker cp dlidli-mysql:/tmp/new-3306.sql "$bk/new-3306.sql"

# ② 覆盖导入（先确认旧库确实是想要的那份：SELECT COUNT(*) FROM video;）
docker cp "$bk/old-3307.sql" dlidli-mysql:/tmp/old.sql
docker exec dlidli-mysql mysql -uroot -pdlidli123 -e "DROP DATABASE IF EXISTS dlidli;"
docker exec dlidli-mysql sh -c "mysql -uroot -pdlidli123 < /tmp/old.sql"

# ③ 补齐旧库可能缺的迁移（旧库 schema 版本可能落后于当前代码）
cd server; $env:DLIDLI_MIGRATE_DSN="mysql://root:dlidli123@tcp(127.0.0.1:3306)/dlidli?multiStatements=true"; go run ./cmd/migrate

# ④ 逐表比对行数（不一致应为 0），再跑 §4.8 的路径核对
```

> 注意：迁移前务必确认**两个库的表结构兼容**（`SELECT version FROM schema_migrations`）。若旧库版本较新则无需 ③；若旧库缺少新表，③ 会补上。

**上线检查**：生产（§5.4）必须把 `uploads/` 对应的对象存储桶与数据库**同周期备份**，二者恢复点目标（RPO）应一致。

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

### 3.1 staging 环境（M0-ENG-13，2026-08-06）

staging 与 dev **完全隔离**（独立端口/库/Redis/上传目录），用于联调演示：

| 维度 | dev | staging |
| --- | --- | --- |
| API 端口 | 8000 | 8100 |
| 数据库 | dlidli | dlidli_staging（脚本自动创建） |
| Redis db | 0 | 1 |
| 上传目录 | ./uploads | ./uploads_staging |
| 审核 | 人工（autoApprove=false） | 自动（autoApprove=true） |

**部署命令**（Windows）：

```powershell
cd server
powershell -ExecutionPolicy Bypass -File deploy/staging.ps1
# 可选：-SkipMigrate 跳过迁移（库已就绪）；-SkipStart 仅构建+验证已有实例
```

脚本流程：构建二进制 → 迁移独立库 → 启动服务（8100）→ HelloWorld 验证（`/health` 组件健康 + `/api/v1/ping`）→ 环境汇总。

**验证命令**：

```powershell
Invoke-RestMethod http://127.0.0.1:8100/health      # {code:0, env:staging, mysql/redis:up}
Invoke-RestMethod http://127.0.0.1:8100/api/v1/ping   # {code:0, pong:...}
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
- [ ] 对象存储桶与数据库**同周期备份**且 RPO 一致（见 §4.8）
- [ ] Nginx SPA 回退、`/api` 与 `/static` 反代正确
- [ ] 管理后台独立域名 + 访问控制
- [ ] 后端进程守护（systemd）+ 日志采集 + `/metrics` 接入监控

---

## 6. 相关文档

- [后端架构](/architecture/backend)：模块划分、API 规范、接口文档（Swagger）
- [前端架构](/architecture/frontend)：monorepo 布局、构建、图标/样式方案
- [协作规范](/project/conventions)：Git Flow 分支与发布流程
- 后端 README：`server/README.md`（命令速查）
