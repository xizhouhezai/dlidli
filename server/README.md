# dlidli-server（Go 后端）

Go 后端服务，模块化单体架构。详细设计见文档站：[后端架构](../docs/architecture/backend.md)、[数据模型](../docs/architecture/data-model.md)、[视频流水线](../docs/architecture/video-pipeline.md)。

## 目录结构

```
server/
├── cmd/
│   ├── api/                # 核心业务 API（已就绪：/health、/api/v1/ping）
│   └── migrate/            # 数据库迁移工具
│   # admin / comet / worker 按里程碑逐步添加
├── configs/
│   └── dev.yaml            # dev 配置（环境变量 DLIDLI_* 可覆盖）
├── internal/
│   ├── module/             # 业务模块（M1 起：account / video / danmaku ...）
│   ├── pkg/                # config / logger / errcode / response / jwtx
│   ├── infra/              # MySQL / Redis 客户端（后续扩展 Kafka / OSS / ES）
│   ├── middleware/         # TraceID / AccessLog / Recovery / CORS / Auth
│   └── router/             # 路由组装
├── scripts/migrations/     # golang-migrate 迁移文件
└── deploy/docker-compose.yaml  # 本地依赖：MySQL / Redis / Kafka / MinIO
```

## 快速开始

```bash
# 1. 启动依赖（需要 Docker）
docker compose -f deploy/docker-compose.yaml up -d

# 2. 初始化数据库
go run ./cmd/migrate

# 3. 启动 API（默认端口 8000）
go run ./cmd/api

# 验证
curl http://localhost:8000/health
curl http://localhost:8000/api/v1/ping
```

> M0 阶段 MySQL/Redis 连接失败时服务会降级启动（仅告警），便于无 Docker 环境开发框架代码；M1 业务模块接入后将改为强依赖。

## 常用命令

| 命令 | 说明 |
| --- | --- |
| `go build ./...` | 全量构建 |
| `go vet ./...` | 静态检查 |
| `go test ./... -race` | 单元测试 |
| `go run ./cmd/migrate` | 应用迁移 |
| `go run ./cmd/migrate -down` | 回滚一步 |
| `swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal` | 生成接口文档（改接口后重跑） |

## 接口文档（Swagger / OpenAPI）

- 基于 swaggo：handler 上写 `// @Summary/@Tags/@Router/@Param/@Success` 注解，`swag init` 生成 `docs/`。
- 非生产环境启动后访问 **http://localhost:8000/swagger/index.html** 查看可交互文档（可直接调试）。
- 需授权的接口在 UI 右上角 **Authorize** 填入 `Bearer {token}`。
- 新增或修改接口后必须重跑 `swag init` 刷新（`docs/` 为生成产物但需提交，因 router 直接导入）。

## 约定

- 统一响应：`{ code, message, data, trace_id }`；错误码分段见 `internal/pkg/errcode`。
- 模块间只允许通过 service 接口调用，禁止跨模块访问 repo。
- 配置密钥不入库：生产环境通过 `DLIDLI_JWT_SECRET`、`DLIDLI_MYSQL_DSN` 等环境变量注入（前缀 `DLIDLI_`，yaml 键名点号转下划线，如 `DLIDLI_TRANSCODE_FFMPEGPATH`）。
- 环境配置：`configs/dev|staging|prod.yaml`；prod 为模板，`env: prod` 时 `Load()` 强制校验 JWT secret 与 MySQL DSN，缺失或 dev 占位值直接拒绝启动。
- ffmpeg/ffprobe 不写机器路径：缺省走 PATH，特殊环境用 `DLIDLI_TRANSCODE_FFMPEGPATH` / `DLIDLI_TRANSCODE_FFPROBEPATH` 覆盖。
