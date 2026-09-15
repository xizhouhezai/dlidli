# ADR：互动/计数服务拆分（gRPC）（M3-ENG-01）

- **状态**：服务边界与契约已冻结，proto 与可编译骨架已落地；真实拆分待联调环境
- **日期**：2026-09-15
- **范围**：`interaction`（评论/点赞/投币/收藏）与计数（`video_stat`）读写链路
- **关联**：[M3-ENG-03 分表 ADR](./adr-m3-eng-03-sharding) ｜ [后端架构](./backend) ｜ [路线图](/project/roadmap)

## 背景

当前后端是**模块化单体**：`internal/module/*` 以包边界隔离，但所有模块共享同一个 `*gorm.DB` 与 `*redis.Client`，跨模块调用直接走 Go 函数调用（如 `interaction.Service` 直接持有 `video.Service`、`account.Service`、`notify.Service`、`growth.Service`）。

M3 阶段互动量增长后，`interaction` 与计数写入成为热点：

- 互动写路径（点赞/投币/收藏）需要**原子去重**（`user_action` 唯一键）并回写计数；
- 计数回写（`video.AddStat`）与互动写在同一事务语义之外，失败仅告警，存在计数漂移；
- 评论/弹幕/`user_action` 面临分表（M3-ENG-03），分片路由需要一个明确的**服务归属**，否则会散落在多个模块。

本机此前无 Docker/K8s 与 gRPC 联调环境，因此不能直接做物理拆分。本次先冻结**服务边界与契约**，并落地可编译、可单测的 proto 与骨架，避免"只拆目录"的假拆分。

## 决策

### 1. 拆分出「互动服务（interaction-svc）」与「计数服务（counter-svc）」

第一阶段只拆出这两个服务，**不**做全面微服务化（账号、视频、推荐等仍留在单体）：

| 服务 | 职责 | 拥有数据 |
| --- | --- | --- |
| `interaction-svc` | 评论/回复、点赞/投币/收藏明细、三连状态聚合 | `comment`（含分表）、`user_action`（含分表）、`collection` |
| `counter-svc` | 视频统计计数（播放/点赞/投币/收藏/评论/弹幕/分享） | `video_stat` |
| 单体 `api`（保持） | 对外 HTTP/BFF、账号、视频、弹幕、推荐、IM 等 | 其余表 |

选择「互动 + 计数」作为首个切入点的理由：边界清晰（明细 vs 聚合）、写路径独立、跨模块依赖少、且正好是分表影响面。

### 2. 契约优先：proto 先冻结，实现后替换

- `proto/interaction/v1/interaction.proto`、`proto/counter/v1/counter.proto` 为**唯一契约来源**；
- 单体侧通过 `internal/client` 的 gRPC 客户端调用；函数签名与现有 Go 方法保持一一对应，便于灰度期双跑；
- 生成代码入库（`server/internal/gen/...`），保证无 protoc 环境也能 `go build`。

### 3. 计数改为事件驱动 + 幂等累加（消除漂移）

现有 `AddStat` 是同步直调、失败仅告警。拆分后：

- 互动/播放等写路径发出**计数事件**（`counter.v1.ApplyDelta`），携带 `event_id`（幂等键）；
- `counter-svc` 按 `event_id` 去重后累加，保证**至少一次投递**下计数不重复；
- `counter-svc` 不反向依赖 `interaction-svc`，避免循环依赖。

### 4. 灰度策略：本地进程内 gRPC，后续再切网络

- 第一阶段（本次）：单体进程内仍用本地实现，`internal/client` 提供**同进程直连**（in-process gRPC 或接口直调）作为默认，网络 gRPC 为可选开关；
- 第二阶段（有联调环境后）：把 `interaction-svc`/`counter-svc` 作为独立二进制（`cmd/interaction-svc`、`cmd/counter-svc`）部署，客户端切到网络地址；
- 通过配置开关（`grpc.interactionAddr`/`grpc.counterAddr` 为空则走本地）实现可回滚。

### 5. 不做的部分（明确边界）

- **不做**跨服务分布式事务：互动写与计数更新改为最终一致 + 幂等补偿；
- **不做**数据库按服务拆分（shared database → 逻辑服务边界）：物理库拆分待分表落地稳定后再评估；
- **不引入**服务网格/注册中心：第一阶段用静态地址 + 环境变量。

## 服务契约（要点）

### interaction.v1

```text
AddComment(uid, bvid, content, root_id, parent_id) -> CommentItem
ListComments(viewer_uid, bvid, sort, page, size) -> (items, total)
DeleteComment(uid, comment_id) -> ()
ToggleLike(uid, bvid) -> (liked, like_delta)
ToggleCommentLike(uid, comment_id) -> (liked, like_delta)
Coin(uid, bvid, count) -> (coin_delta, coin_count)
ToggleFav(uid, bvid, collection_id) -> (faved, fav_delta)
GetTripleState(uid, bvid) -> TripleState
```

### counter.v1

```text
ApplyDelta(event_id, video_id, column, delta) -> (applied, current)
GetStat(video_id) -> StatSnapshot
BatchGetStats(video_ids) -> map<video_id, StatSnapshot>
```

约定：

- `column` 使用**白名单枚举**（`COUNTER_COLUMN_PLAY/LIKE/COIN/FAV/COMMENT/DANMAKU/SHARE`），禁止字符串拼接，与 M3-ENG-03 的表名白名单同一防御思路；
- 所有 `delta` 为 `int64`，服务端对下限做 `GREATEST(...,0)` 保护；
- `uid<=0` 表示游客（仅读路径有效）。

## 与分表（M3-ENG-03）的关系

- `comment`/`user_action` 的分片路由**收敛在 `interaction-svc` 内部**，通过已落地的 `internal/pkg/shard`；
- 计数服务只认 `video_id`，不感知分片，避免分片逻辑跨服务泄漏；
- 跨分片聚合（如某用户全部收藏）由 `interaction-svc` 并行查分片后聚合，不经 gRPC 暴露分片细节。

## 验收标准（EARS）

- **WHEN** 客户端以相同 `event_id` 重复调用 `ApplyDelta`，**THE SYSTEM SHALL** 只累加一次并返回首次结果。
- **WHEN** `grpc.interactionAddr` 为空，**THE SYSTEM SHALL** 走本地实现，行为与拆分前完全一致。
- **WHEN** `counter-svc` 不可用，**THE SYSTEM SHALL** 在互动写路径记录待补偿事件，不阻断用户主流程。
- **WHILE** 灰度期，**THE SYSTEM SHALL** 保证 HTTP 响应结构与字段与拆分前一致（前端零改动）。
- **WHERE** `column` 不在白名单，**THE SYSTEM SHALL** 拒绝并返回参数错误，不执行任何 DDL/DML。

## 未决事项

- `interaction-svc` 与 `counter-svc` 的独立数据库拆分时机（依赖分表迁移完成度）。
- 待补偿事件的持久化介质（本地表 / Kafka，M0 已有 Kafka 组件）。
- 是否需要把「评论通知」也移出单体（涉及 `notify` 与 `im` 边界）。
- 生产环境服务发现与 mTLS 方案。
