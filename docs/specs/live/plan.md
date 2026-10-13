# plan：直播

> 对应规格：[spec](/specs/live/spec) ｜ 技术基线：[总体架构](/architecture/overview) · [视频处理流水线](/architecture/video-pipeline)
> 实施状态：V3.1（2026-10-13 LIV-02 细化）；M4-LIV-02 已启动。
>
> 变更记录：
> - 2026-08-21 随 spec 迁移为 SDD，技术预研。
> - 2026-10-13 补 M4-LIV-02 技术决策：房间表结构（§2.1）、WS 通道与 danmaku hub 关系（§2.2）、在线人数实现（§2.4）、开播密钥（§2.5）、API 布局（§3）。

## 1. 方案概览

推流（RTMP 接入）→ 转码集群（多码率）→ CDN 分发（HLS 起步，HTTP-FLV/WebRTC 降延迟演进）；弹幕网关复用/comet 房间广播模式分层扩展；礼物链路接入支付系统强一致事务。

房间域（M4-LIV-02）：`live` 模块（`server/internal/module/live`），gin 常规分层 handler/service/repo，MySQL（GORM）存房间/白名单/房管/禁言，Redis 存在线计数与心跳。RTMP 发流不在本卡（LIV-01 接 SRS）。

## 2. 技术预研要点

| 决策点 | 方向 | 说明 |
| --- | --- | --- |
| 推流与分发 | RTMP → 转码集群（多码率）→ CDN（HLS 延迟 6-15s；后续 HTTP-FLV/WebRTC 降至 3s 内） | 延迟与成本平衡的演进路径 |
| 弹幕网关 | 单房间 10 万连接目标：网关分片 + 房间维度聚合分层广播 | 复用 [danmaku plan](/specs/danmaku/plan) hub 模型横向扩展 |
| 礼物链路 | 充值/扣款/分成走支付系统事务 + 对账（见 [monetization plan](/specs/monetization/plan)） | 资产强一致 |
| 内容安全 | 直播截帧机审（每 10s）+ 人工巡查 + 快速切断机制 | 合规硬要求 |
| 回放 | 下播后转码生成稿件（复用视频转码流水线） | 内容资产复用 |

## 2A. M4-LIV-02 技术决策（2026-10-13）

### 2A.1 表结构（迁移 0034）

四张表，全 InnoDB / utf8mb4：

- `live_room`：`anchor_id`（唯一，一主播一房）、`title`、`cover`、`category_id`（复用视频分区表）、`status`（0 空闲 / 1 直播中 / 2 已下播）、`stream_key_hash`（bcrypt）、`started_at` / `ended_at`（当前/上一场时间）、`live_cnt`（DB 侧兜底计数，非实时权威）、时间戳。索引：`uk_anchor`、`idx_status`（列表按 status=1 扫）。
- `live_whitelist`：`user_id` 唯一、`note`、`created_by`、时间戳。资格第一道闸。
- `live_room_admin`：`room_id` + `user_id` 唯一、`created_by`（主播）、时间戳。
- `live_room_mute`：`room_id` + `user_id` 唯一、`muted_until`、`reason`、`created_by`、时间戳；未到期重复禁言 UPSERT 覆盖 `muted_until`。

### 2A.2 WS 通道与 danmaku hub 的关系

**决策：本卡不建 WS 通道；LIV-03 新建独立 `live` hub，不复用、不扩展 danmaku hub。**

理由：
1. danmaku hub 的房间键是 `videoID`（int64 视频域）、消息类型固定 `danmaku`、客户端只读（发送走 HTTP）。直播房间键是 `live_room.id`，消息类型有弹幕/进场/礼物/系统多类，客户端需要上行心跳——语义不同，硬扩展会把视频域枚举与直播域状态机耦进同一个 hub。
2. 项目已有先例：`notify`（按用户房间）与 `im`（按会话房间）都是各自独立 hub，`Hub/room/client` 结构复制而互不引用。第三个域照此办理，一致性最好。
3. 直播 hub 未来要承载 10 万连接目标（分片），独立演进不受视频弹幕发布节奏约束。

备选（否决）：A) 扩展 danmaku hub 支持多消息类型 + 双房间命名空间——省一份样板代码，但把两域生命周期焊死，10 万连接分片改造时互相拖累；B) 抽公共 comet 库三域共用——本次不做（YAGNI，等第四个域出现再抽）。本卡仅提供房间元数据与 `IsMuted` 判定，供 LIV-03 的 live hub 校验调用。

### 2A.3 在线人数实现

**决策：Redis 计数 + 心跳过期键，双键方案**：

- `live:room:{id}:online`：INCR/DECR 计数，房间详情读它；DB `live_cnt` 仅重启兜底，不做权威。
- `live:room:{id}:hb:{uid}`：TTL 30s 过期键，心跳刷新；进房判重（键已存在则不 INCR）、懒剔除依据。

口径（对齐 spec §5B）：心跳 ≤15s 一次，30s 无心跳视为离线。**剔除方式选「心跳续期 + 懒剔除」，否决定期全量对账集合方案**：心跳集合（如 `live:room:{id}:set` 存 uid 集合 + ZSET 时间戳）可精确剔除但集合成员与计数要双写保持一致，单机内存部署下复杂度不划算；TTL 键方案键自过期，无需对账 Worker，读侧 `CountExpired` 校准即可。Redis 不可用时计数降级为读 DB `live_cnt`（误差容忍，在线人数非资金数据）。

**已知边界（2026-10-13 实测登记）**：观众「心跳超时后重新进房」（hb 键已过期且未走过心跳校准）会把陈旧条目重复计入一次（超时后走心跳则 DECR+INCR 校准、计数不变，已实测）。HTTP 心跳是过渡口径：M4-LIV-03 的 live hub 以 WS 连接生命周期管理在线状态（断连即离房），该边界届时自然消失；过渡期内的轻微高估可接受（在线人数非资金数据）。

### 2A.4 开播密钥生成与校验

- 生成：`crypto/rand` 32 字节 → hex 32 位（参考 account 验证码风格）；仅申请开播接口明文返回一次，库中 bcrypt 哈希。
- 校验：LIV-01 接入 SRS 后由推流回调 `/api/v1/live/stream/auth`（后续卡）用 `bcrypt.CompareHashAndPassword` 校验 `name=stream_key`；本卡只落「生成 + 重置（重置即换哈希，旧密钥失效）」。
- 推流地址：配置 `Live.PushDomain` 占位拼 `rtmp://{domain}/{room_key}/{stream_key}`，本卡不校验连通性。

### 2A.5 资格校验

`Service.CanStartLive(ctx, uid)`：白名单存在 → 实名标记 → `relation` 粉丝计数 ≥ 100，逐项短路返回可区分错误码（9xxxx 商业化段新增 LIV 码）。校验通过才创建房间。

## 3. API 布局（M4-LIV-02）

挂 `/api/v1/live/*`（auth = 登录 + 限流）：

| 方法/路径 | 鉴权 | 说明 |
| --- | --- | --- |
| POST `/live/apply` | auth | 申请开播（资格校验 → 建房 → 生成密钥，明文仅此一次返回） |
| POST `/live/room/start` | auth | 开播（status 0/2→1） |
| POST `/live/room/stop` | auth | 下播（1→2） |
| PUT `/live/room` | auth | 改标题/封面/分区 |
| POST `/live/room/stream-key/reset` | auth | 重置推流密钥（明文一次性返回） |
| GET `/live/room/mine` | auth | 我的房间（含推流地址，仅主播可见密钥不回传） |
| GET `/live/rooms` | 公开 | 直播中房间列表（status=1，分页） |
| GET `/live/rooms/:id` | optionalAuth | 房间详情（含在线人数） |
| POST `/live/rooms/:id/enter` | auth | 进房（计数 +1） |
| POST `/live/rooms/:id/heartbeat` | auth | 心跳（刷新 TTL） |
| POST `/live/rooms/:id/leave` | auth | 离房（计数 −1） |
| POST `/live/rooms/:id/admins/:uid` | auth | 任命房管 |
| DELETE `/live/rooms/:id/admins/:uid` | auth | 撤销房管 |
| POST `/live/rooms/:id/mutes/:uid` | auth | 禁言 |
| DELETE `/live/rooms/:id/mutes/:uid` | auth | 解禁 |

`swag init` 注释完整；router.go 在 DB/Redis 就绪分支注册。

## 4. 风险与待定项

- [x] 房间在线人数分层广播架构设计（→ §2A.3，广播层归 LIV-03）
- [ ] 直播资质与合规评估（启动前完成，见 spec §4）
- [ ] CDN 选型与成本测算
- [ ] live hub 弹幕通道设计（LIV-03 细化时定）
