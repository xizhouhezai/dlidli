# 后端架构（Go）

> 状态：草案 ｜ 更新日期：2026-07-28

## 1. 工程结构（模块化单体）

`server/` 采用 Go 单仓多可执行单元结构：

```
server/
├── go.mod                     # module github.com/dlidli/server
├── cmd/
│   ├── api/main.go            # 核心业务 API 服务
│   ├── admin/main.go          # 管理后台 API 服务（当前阶段：admin 模块内嵌 api 服务 /api/v1/admin，规模化后拆出）
│   ├── comet/main.go          # WebSocket 网关（V1）
│   └── worker/main.go         # 异步 Worker（转码调度/通知/计数/索引）
├── configs/                   # 配置文件（yaml，多环境）
├── internal/
│   ├── module/                # 业务模块（领域边界 = 未来微服务边界）
│   │   ├── account/           #   账号：handler / service / repo / model
│   │   ├── video/             #   稿件与播放
│   │   ├── danmaku/           #   弹幕
│   │   ├── interaction/       #   点赞/投币/收藏/评论
│   │   ├── relation/          #   关注/黑名单
│   │   ├── feed/              #   动态流
│   │   ├── message/           #   通知/私信
│   │   ├── search/            #   搜索（ES 封装）
│   │   ├── audit/             #   审核域
│   │   └── admin/             #   后台管理域
│   ├── pkg/                   # 内部共享：jwt、response、errcode、pagination
│   ├── infra/                 # 基础设施：mysql、redis、kafka、oss、es 客户端
│   └── middleware/            # 认证、限流、日志、恢复、CORS
├── api/                       # OpenAPI 定义（swagger）
├── scripts/                   # 数据库迁移（migrate）、构建脚本
└── deploy/                    # Dockerfile、docker-compose、k8s 清单
```

### 模块内分层

```
handler(HTTP 层，参数校验/DTO) → service(业务编排/事务) → repo(数据访问) → model(实体)
```

**模块间只允许通过 service 接口调用，禁止跨模块访问 repo** —— 保证 V2 拆微服务时切割成本最低。

## 2. 关键机制设计

### 2.1 认证与会话

- 登录颁发 `access_token`（JWT, 2h）+ `refresh_token`（随机串, 30d, Redis 存储可吊销）。
- 中间件解析 JWT 注入 `user_id` 到 context；`refresh` 接口轮换双 token。
- 会话表记录设备信息，支持"踢出设备"（refresh token 吊销）。

### 2.2 计数系统（高频写）

```
用户点赞 → Redis(INCR + 去重 set) → 立即返回
         → 投递 Kafka 事件 → 计数 Worker 批量聚合 → 定时刷 MySQL（最终一致）
```

- 读计数：Redis 优先，miss 回源 MySQL 并回填。
- 防重：`like:uid:{oid}` set 判断；硬币扣减走 MySQL 事务（强一致）。

### 2.3 弹幕链路

- 写：HTTP 发送 → 机审过滤 → MySQL 落库 + Redis 段缓存追加 → Kafka → comet 广播房间。
- 读：按 `video_id + 段号(6min)` 读 Redis 缓存（miss 回源构建）。
- comet：goroutine per connection，房间订阅模型，单实例目标 5 万连接。

### 2.4 Feed 流（推拉结合）

- 发布动态 → 写自己发件箱（MySQL）→ Kafka → 扇出 Worker 推送到粉丝收件箱（Redis zset，粉丝 >1 万的大 V 不扇出）。
- 读取：收件箱 + 关注大 V 发件箱实时合并，游标分页。

### 2.5 事务与一致性策略

| 场景 | 策略 |
| --- | --- |
| 投币（硬币扣减+计数） | MySQL 本地事务 + 幂等键 |
| 播放计数 | Redis HyperLogLog/set 去重 + 异步聚合 |
| 稿件状态流转 | 状态机 + 乐观锁（version 字段） |
| 跨模块事件（审核通过→发动态→通知） | Kafka 事件驱动，消费者幂等 |

## 3. API 规范

- REST 风格：`GET /api/v1/videos/{bvid}`、`POST /api/v1/videos/{bvid}/like`
- 统一响应包裹 + 分页游标（`cursor` + `page_size`，避免深分页）。
- OpenAPI 3 文档自动生成，前端据此生成 TS 类型（`packages/api-client`）。

## 4. 质量保障

- 单测：service 层覆盖率 ≥ 60%（MVP）→ 75%（V1+）；repo 层用 sqlmock/testcontainers。
- 集成测试：docker-compose 拉起依赖，跑核心链路用例。
- 静态检查：golangci-lint（CI 强制）；`go vet` + `staticcheck`。
- 压测：上线前对登录/播放/弹幕接口做基准压测（k6）。

## 5. 配置与部署

- 配置：viper 读取 `configs/{env}.yaml` + 环境变量覆盖（密钥只走环境变量/Secret）。
- MVP 部署：docker-compose（api/admin/worker/mysql/redis/kafka/minio）单机起步。
- CI：push → lint + test → 构建镜像 → 推送 registry → staging 自动部署。
