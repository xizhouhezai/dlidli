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
| 观看进度 | Redis 存储 90d + 前端 10s 节流上报 / 离开落盘 / 续播定位 | 高频写不进 MySQL | 直写 MySQL |
| 播放计数 | > 5s 上报 + UID/IP 8h Redis 去重 | 防刷且容忍近似 | 实时精确计数（成本高） |
| 播放器 | 独立包 `@dlidli/player`：PlayerCore 框架无关封装 hls.js（生命周期/清晰度切换保留进度/mp4 兜底）+ bindKeyboard + qualityLabel 工具 | 多端复用（Web/H5） | 各端各写播放器 |
| 稿件标识 | 雪花 ID 主键 + bvid 短编码（DV 前缀）对外 | 防遍历；对外展示友好 | 自增 ID（可遍历） |
| 稿件编辑 | 状态机 + `version` 乐观锁；dev 提供 autoApprove 自动过审（审核后台上线后关闭） | 防并发状态错乱 | 分布式锁 |
| 多 P | `video_part` 表 + `video_stream`/`transcode_job` 加 part_index，唯一键含分 P | 转码按 P 隔离互不影响 | 稿件拆多条（体验差） |
| 合集 | `video_collection` / `video_collection_item`（与收藏夹 favorite_folder 表名区分） | 合集=稿件组织形态，与收藏行为解耦 | 复用收藏夹模型（语义冲突） |

## 3. 数据模型

全局见 [数据模型](/architecture/data-model)（`video` / `video_stream` / `video_stat`）。模块扩展：

```sql
video_part            { video_id, part_index, title, duration }     -- 0021~0024 迁移
video_collection      { id, user_id, title, ... }                   -- 0025 迁移
video_collection_item { collection_id, video_id, sort }
transcode_job         { video_id, part_index, status, retry_cnt }   -- 任务队列
```

## 4. 接口设计

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | /api/v1/videos/upload/init ｜ /parts ｜ /complete | 分片上传三段式（init 返回任务与已传分片） |
| GET | /api/v1/videos/upload/progress ｜ /hash/{hash} | 进度查询 ｜ 秒传探测 |
| POST/PUT/DELETE | /api/v1/videos... | 稿件 CRUD（投稿/详情/我的稿件/公开列表/软删除） |
| GET | /api/v1/videos/{bvid}/parts | 分 P 列表（各 P 签名流） |
| GET | /api/v1/videos/{bvid} | 详情（含签名播放地址下发） |

> 完整契约以 swaggo 生成的 OpenAPI 为准（`server/docs/`）。

## 5. 关键流程

- 稿件状态机与状态值：`0草稿 1上传中 2转码中 3待审 4已发布 5驳回 6锁定 7删除`（业务规则见 [spec §2.3](/specs/video/spec)）。
- 投稿页前端：hash-wasm 增量校验 + 并发分片 + 断点续传 + 秒传；封面三级兜底（首帧截取 → 自定义上传 → 默认封面）。
- dev 环境 autoApprove：投稿即自动过审发布（审核后台上线后关闭）。

## 6. 风险与待定项

- [ ] MinIO 预签名直传接入（部署环境）
- [ ] Kafka 替换 DB 任务队列（规模化）
- [ ] 1080P / 4K 档位与会员清晰度鉴权（V1 / V3）
- [ ] 定时发布（VID-04）、视频指纹（VID-13）未实现
- [ ] 稿件编辑重新送审完整闭环（VID-06，编辑入口已随稿件管理页补）
