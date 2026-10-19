# plan：视频投稿与播放

> 对应规格：[spec](/specs/video/spec) ｜ 技术基线：[视频处理流水线](/architecture/video-pipeline) · [数据模型](/architecture/data-model) · [后端架构](/architecture/backend)
> 实现位置：`server/internal/module/video`（handler/service/repo/transcoder）、`packages/player`、Web 投稿/详情页

## 1. 方案概览

上传走"初始化 → 分片直传 → 合并/秒传"三段式；转码以任务表队列解耦（Worker 内嵌进程起步，可平滑替换 Kafka）；播放走签名 URL + HLS；前端播放器封装为独立框架无关包 `@dlidli/player`。稿件状态机由 `video.status` + 乐观锁 `version` 驱动，全链路见 [视频处理流水线](/architecture/video-pipeline)。

## 2. 技术决策

| 决策点 | 决策 | 理由 | 备选方案 |
| --- | --- | --- | --- |
| 上传通道 | 分片（5MB）直传业务服务器 + 本地磁盘，SHA-256 校验 | 本地开发无 Docker/MinIO；Storage 抽象接口已隔离，可平滑切对象存储 | MinIO 预签名直传（部署环境接入后启用） |
| 转码队列 | DB 任务表 + `FOR UPDATE SKIP LOCKED` 认领 + 失败重试 2 次 | 免 Kafka 依赖即可用；接口层预留切换 | Kafka 任务队列（规模化后） |
| 转码执行 | dev 内嵌进程 Worker：FFmpeg 输出 HLS（360P/720P）+ 抽帧封面 + ffprobe 时长 | 单机起步，部署环境可拆独立 Worker | 独立转码集群 |
| 播放防盗链 | playsign 包 HMAC-SHA256 签名路径 + 过期参数（TTL 6h），PlaySignGuard 中间件校验 `/static/videos` 入口（.m3u8/.mp4），.ts 分片/封面/头像放行 | 签名校验集中在播放入口；E2E：签名 200 / 无签 403 / 篡改 403 | 全路径签名（分片流量大不划算） |
| 清晰度档位与权益（M4-VIP-02） | 档位编号沿用 `quality`（0/360/720/1080/2160），参数与开关落 `video_quality_tier` 配置表；下发流列表时按身份过滤（未登录按免费）；转码档位产物的签名覆盖**档位 + 用户**，静态入口再加会员复核；新增 `video_stream.height` 记录实际画面高度（原画行=源高度，防 4K 源走原画绕过上限） | 权益不可前端绕过，且泄漏的 URL 也被兜住（改档位参数/转发他人签名均 403）；既有 0/360/720 数据与签名语义零影响 | 只在下发时过滤（泄漏 URL 无法兜底）／只签路径（改档位即越权）／编号重排（需数据迁移 + 全端改造） |
| 观看进度 | Redis 存储 90d + 前端 10s 节流上报 / 离开落盘 / 续播定位 | 高频写不进 MySQL | 直写 MySQL |
| 播放计数 | > 5s 上报 + UID/IP 8h Redis 去重 | 防刷且容忍近似 | 实时精确计数（成本高） |
| 播放器 | 独立包 `@dlidli/player`：PlayerCore 框架无关封装 hls.js（生命周期/清晰度切换保留进度/mp4 兜底）+ bindKeyboard + qualityLabel 工具 | 多端复用（Web/H5） | 各端各写播放器 |
| 稿件标识 | 雪花 ID 主键 + bvid 短编码（DV 前缀）对外 | 防遍历；对外展示友好 | 自增 ID（可遍历） |
| 稿件编辑 | 状态机 + `version` 乐观锁；dev 提供 autoApprove 自动过审（审核后台上线后关闭） | 防并发状态错乱 | 分布式锁 |
| 多 P | `video_part` 表 + `video_stream`/`transcode_job` 加 part_index，唯一键含分 P | 转码按 P 隔离互不影响 | 稿件拆多条（体验差） |
| 合集 | `video_collection` / `video_collection_item`（与收藏夹 favorite_folder 表名区分） | 合集=稿件组织形态，与收藏行为解耦 | 复用收藏夹模型（语义冲突） |
| 上传文件归属（VID-24） | 投稿前 `uploadSvc.GetUserFile(ctx, uid, fileID)` 强制校验 `file.user_id == uid`，不匹配返回禁权；跨用户秒传不再返回他人 `file_id`（uk_hash 唯一约束下改为走本人上传会话） | 杜绝越权复用他人上传原文件（P0 安全）；不改表结构、最小侵入 | 去掉 uk_hash 唯一约束 + 跨用户秒传建立本人记录（需迁移、成本高） |
| **认领机制**（转码积压治理） | **DB 条件认领 + 行锁 + 租约**：事务内 `SELECT … FOR UPDATE SKIP LOCKED` 选行 → 条件 `UPDATE … WHERE id=? AND status=?` 校验 `RowsAffected=1` → 写 `lease_until/worker_id/started_at`；WHERE 额外放开「`status=1` 且 `lease_until` 已过期」使崩溃实例的任务可回收。认领后 Worker 每 `lease/3` 续租，续租失败即放弃本任务并**不回写状态** | ① 队列本来就在 MySQL，零新增依赖；② 三层保证（行锁跳过、条件 UPDATE 幂等、租约回收）使任意进程数下**同一任务只有一个赢家**；③ 租约解决"进程崩溃留下永久 processing"这一水平扩容的真实风险（扩容会放大它）。实测 3 进程×2 协程消费 24 任务零重复 | **Redis 队列**（LIST/LMOVE + BRPOPLPUSH）：更快但要新增一致性边界（DB 写成功而缓存没入队＝任务丢失）；**纯轮询无原子认领**：两实例必抢同一任务（现状的核心缺陷）；**MySQL 无 SKIP LOCKED 的 `FOR UPDATE`**：会串行阻塞，扩容反而更慢 |
| **指标载体**（转码积压治理） | **复用既有 prometheus 基建**（`internal/pkg/metrics` 已有 client_golang + `/metrics` 路由）：新增 `dlidli_transcode_queue_depth{status}`、`oldest_pending_seconds`、`job_duration_seconds`（histogram，均值取 `_sum/_count`）、`jobs_processed_total{result}`、`reclaimed_total`、`lease_lost_total`；另有只读快照接口 `GET /admin/videos/transcode/queue`（`video:view`）。指标由周期 refresher（15s）+ 任务边界共同刷新 | 项目已有 prometheus client 与 `/metrics`，**不引新库**；快照接口补 Gauge 表达不了的"当前瞬时值"（多实例各自上报会被最后一个覆盖，单次查库更准） | 自建 Redis 计数（多一套状态要维护，且与 DB 真相易漂移）；只做接口不做指标（无趋势、无法告警） |
| **重试语义**（转码积压治理） | `transcode.maxRetries`（默认 0，即一次失败即终态）+ 指数退避 `retryBackoff*2^(n-1)`（上限 1h）写 `next_retry_at`；未超上限→回 `pending` 且退避期内不可认领，超上限→终态 `failed` 并写 `finished_at` 清租约。失败分类：`context.Canceled`（租约丢失/关停）**不回写状态**，`DeadlineExceeded`（`jobTimeout` 兜底）按正常失败计 | 退避的作用不是"慢一点"，而是防止坏任务被回收的瞬间被其他 Worker 抢走，把多实例变成对同一坏文件的热循环；终态保证坏文件不会无限重试也不会永久占 pending | 无退避的重排（多实例下形成热循环）；无限重试（队列被一个坏文件堵死）；无限 pending（同样堵死） |
| **水平扩容形态**（转码积压治理） | **进程级扩容**：新增独立入口 `cmd/worker`，读同一份配置与 `transcode.workers`（每进程协程数），与 API 内嵌模式共用 `RunTranscodeWorkers` 同一套认领/重试语义；生产设 `transcode.embedInAPI=false` | 扩容单位是进程而非协程：协程数受 CPU 核数限制，且与 API 共享 GC/连接池；独立进程可单独扩缩容、重启不影响 API | 只把 API 内 `workers` 调大（受核数限制、与 API 抢资源、API 重启打断转码）；K8s/HPA 真实多机部署（本卡不做，见部署文档） |
| **投稿时长上限**（转码积压治理） | `transcode.maxDurationSeconds`（默认 1800）：**投稿阶段**用 ffprobe 读源文件真实时长拦截，超限返回 `30007`；多 P 逐 P 校验；探测失败放行（时长限制是体验护栏而非安全边界） | 在投稿阶段拒绝＝同步可读原因、**不产生转码任务**，从源头削减积压；若放到转码阶段失败，用户已上传完并排了队，体验最差还要白烧一次 Worker 时间 | 转码阶段失败（体验最差且浪费 CPU）；只信前端上报时长（可伪造）；上传完成时落 duration 列（需改上传链路，本卡收益不足） |

## 3. 数据模型

全局见 [数据模型](/architecture/data-model)（`video` / `video_stream` / `video_stat`）。模块扩展：

```sql
video_part            { video_id, part_index, title, duration }     -- 0021~0024 迁移
video_collection      { id, user_id, title, ... }                   -- 0025 迁移
video_collection_item { collection_id, video_id, sort }
transcode_job         { video_id, part_index, status, retry_cnt }   -- 任务队列
-- 0043 迁移（转码积压治理）：租约与退避
transcode_job         += { lease_until, next_retry_at, worker_id, started_at, finished_at }
-- 索引：idx_claim(status, next_retry_at, id) 供认领扫描；idx_lease(status, lease_until) 供回收扫描
```

> 租约字段语义：`status=1` 且 `lease_until` 已过期的行被视为"僵尸任务"，
> 下次认领会直接把它当可认领（同一事务里改持有者），因此**不需要**额外的清扫进程；
> `ReclaimExpired()` 只在统计口径上显式回收并计数，供 `reclaimed_total` 观察。

## 4. 接口设计

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | /api/v1/upload/init ｜ /upload/{id}/parts/{index} ｜ /upload/{id}/complete | 分片上传三段式（init 返回任务与已传分片；upload 独立模块路由，见 upload handler） |
| GET | /api/v1/upload/{id} | 上传进度查询 |
| POST/PUT/DELETE | /api/v1/videos... | 稿件 CRUD（投稿/详情/我的稿件/公开列表/软删除） |
| GET | /api/v1/videos/{bvid}/parts | 分 P 列表（各 P 签名流，**按会员状态过滤档位**；挂 optionalAuth，未登录按免费） |
| GET | /api/v1/videos/{bvid} | 详情（含签名播放地址下发，**同上过滤**；挂 optionalAuth） |
| GET/PUT | /api/v1/admin/videos/quality-tiers[/:quality] | 清晰度档位配置（挂 AdminAuth + `video:view` / `video:tier`） |
| GET | /api/v1/admin/videos/transcode/queue | **转码积压快照**（挂 AdminAuth + `video:view`）：pending/running/success/failed、oldest_pending_seconds、oldest_pending_job_id、running_overdue、avg_duration_seconds。只出接口，管理端页面由另一张卡负责 |
| GET | /metrics | Prometheus 抓取端点（既有）：本卡新增 `dlidli_transcode_*` 系列；独立 Worker 可通过 `transcode.metricsAddr` 自曝同口径指标 |

> 完整契约以 swaggo 生成的 OpenAPI 为准（`server/docs/`）。

## 5. 关键流程

- 稿件状态机与状态值：`0草稿 1上传中 2转码中 3待审 4已发布 5驳回 6锁定 7删除`（业务规则见 [spec §2.3](/specs/video/spec)）。
- 投稿页前端：hash-wasm 增量校验 + 并发分片 + 断点续传 + 秒传；封面三级兜底（首帧截取 → 自定义上传 → 默认封面）。
- dev 环境 autoApprove：投稿即自动过审发布（审核后台上线后关闭）。

### 5.1 转码任务生命周期（含租约与重试）

```
投稿 ──(时长 > maxDurationSeconds → 30007 拒绝，不落库不排任务)
  └─(通过) → transcode_job(status=0, next_retry_at=NULL)
        │
        │  Worker 认领：SELECT … FOR UPDATE SKIP LOCKED（顺手接纳 status=1 且租约过期）
        │              + UPDATE … WHERE id=? AND status=?（RowsAffected 必须=1）
        ▼
     status=1 + lease_until=now+lease + worker_id + started_at
        │
        ├─ 每 lease/3 续租（续租失败=已被回收 → cancel 任务，不回写状态）
        │
        ├─ 成功 → status=2 + finished_at + 清租约 → 该稿件全部档位完成时推进稿件状态机
        │
        └─ 失败/超时 → 分类：
             ├─ context.Canceled（租约丢失/关停）→ 不回写（任务已归别人）
             └─ 其他（含 DeadlineExceeded）
                   ├─ retry_cnt+1 <= maxRetries → status=0 + next_retry_at=now+backoff
                   └─ 超过 maxRetries           → status=3 + finished_at（死信终态）
```

积压四个口径的取数（`QueueSnapshot`）：
排队数＝`COUNT(status=0)`、正在转＝`COUNT(status=1)`、最老等待＝`now-MIN(created_at WHERE status=0)`、
平均耗时＝`AVG(finished_at-started_at WHERE status=2)`（failed 行耗时无产品意义，不计入）。

## 6. 风险与待定项

- [x] **转码积压治理：水平扩容 + 积压可观测 + 失败重试上限 + 投稿时长上限**（2026-10-19，`feat/transcode-horizontal-scale`）：
      决策见 §2 的「认领机制 / 指标载体 / 重试语义 / 水平扩容形态 / 投稿时长上限」五行，生命周期见 §5.1。
      实测：3 进程×2 协程消费 24 任务**每个任务恰好处理一次**（DB 逐条核对无重复产物、无僵尸 running）；
      积压指标 `pending 10 → 0`、管理端快照四个口径齐全；坏任务按重试上限进终态且不影响其他任务；
      挂死任务被 `jobTimeout` 兜底；1900s 素材在投稿阶段被 30007 拒绝且不产生转码任务。
      独立入口 `cmd/worker` 可单独启动消费。证据见 `.dev-logs/transcode-scale/`。
- [ ] MinIO 预签名直传接入（部署环境）
- [ ] Kafka 替换 DB 任务队列（规模化）——**本卡未选**：MySQL 行锁认领已满足"原子认领 + 租约 + 可回收"，
      且换队列要新增一致性边界（DB 写成功而消息没入队＝任务丢失）。当前瓶颈是 CPU 而非队列吞吐，
      结论：等单队列吞吐真的顶不住（当前 `SKIP LOCKED` 认领是毫秒级）再评估，不是现在的瓶颈。
- [ ] 真实多机 / K8s 部署形态（本卡只做到"可水平扩容且竞态安全"，启动命令与并行注意事项见 [deployment §5.7](/project/deployment)）
- [x] 1080P / 4K 档位与会员清晰度鉴权（V1 / V3）已实现（2026-10-15，M4-VIP-02：档位配置表 + 下发时过滤 + 签名绑定档位与用户 + 静态入口会员复核；规格见 [monetization spec §8](/specs/monetization/spec)、决策见 [monetization plan §2G](/specs/monetization/plan)）
- [ ] 整片 4K 转码的量级验证与运维开关（本卡 4K 默认不排程，以 3 秒样片实测链路）
- [ ] 定时发布（VID-04）、视频指纹（VID-13）未实现
- [x] 上传文件归属校验（VID-24）已实现（2026-08-25，投稿前 uid 校验 + 跨用户秒传禁复用）
- [ ] 稿件编辑重新送审完整闭环（VID-06，编辑入口已随稿件管理页补）
