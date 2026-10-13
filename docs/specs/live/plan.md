# plan：直播

> 对应规格：[spec](/specs/live/spec) ｜ 技术基线：[总体架构](/architecture/overview) · [视频处理流水线](/architecture/video-pipeline)
> 实施状态：V3.2（2026-10-13 LIV-03 细化）；M4-LIV-02 已交付、M4-LIV-03 已实现。
>
> 变更记录：
> - 2026-08-21 随 spec 迁移为 SDD，技术预研。
> - 2026-10-13 补 M4-LIV-02 技术决策：房间表结构（§2.1）、WS 通道与 danmaku hub 关系（§2.2）、在线人数实现（§2.4）、开播密钥（§2.5）、API 布局（§3）。
> - 2026-10-13 补 M4-LIV-03 技术决策：live hub 形态与连接治理（§2B.1）、弹幕持久化口径（§2B.2）、频控参数（§2B.3）、WS 端点布局（§3B）。

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
- [x] live hub 弹幕通道设计（→ §2B，2026-10-13 随 LIV-03 定案）
- [ ] 直播资质与合规评估（启动前完成，见 spec §4）
- [ ] CDN 选型与成本测算

## 2B. M4-LIV-03 技术决策（2026-10-13）

### 2B.1 live hub 形态与连接治理

**决策：`live` 模块内新建独立 `hub.go`（`Hub/room/client` 三层复制先例，不 import danmaku/im），非扩展 danmaku hub。** 按 §2A.2 既定决策执行；实测差异点：

- 消息类型多类（`enter`/`danmaku`/`pong`），danmaku hub 单一 `type` 语义不兼容；
- 客户端**可上行**（心跳 + 弹幕），danmaku hub 客户端只读；上行帧 ≤512B，弹幕 ≤50 字；
- 心跳：客户端应用层 `{"type":"heartbeat"}` ≤15s 一次（复用 spec §5B 口径），服务端 pong 单播回执；WS 层 30s ping + 40s 读超时（pongWait）踢出——实测 40s 无 pong 连接被断（closeCode=1005）；
- 单房间连接上限沿用 5000（danmaku 同值）：无压测依据前不调，10 万连接分片目标留给演进卡；
- **单连接语义**：同 uid 旧连接被新连接顶替（防多开重复广播/重复计数）；
- 断开即离房：连接 remove 时回调 `Online.LeaveForce`，以进房时 `EnterForce` 返回的 counted 标志**配对扣减**（hb 键 TTL 30s 可能先于长连接过期，扣减不能依赖键存在性——实测修掉的漏扣缺陷）。该口径下 §2A.3 登记的「超时重进重复计入」边界随 WS 生命周期管理自然消失。

### 2B.2 弹幕持久化口径

**决策：落库最近 N=50 条（迁移 0035 `live_room_msg`），不落全量、不进视频域弹幕表。**

依据：spec §4 的「直播内容存档 ≥60 天」合规义务以**截帧机审（每 10s，LIV-05）+ 下播回放转码（LIV-14）**承载——存档对象是直播画面与回放，弹幕不在存档义务内；弹幕的观众侧价值只有「进房时最近几句」，窗口裁剪（每房间仅保留最近 50 条，追加时按雪花 id 裁掉窗口外旧行）把表体量钉死为 `房间数 × 50` 行。备选：A) 全量落库（60 天 × 高频房间 = 千万行级，无查询方，否决）；B) 完全不落库（进房无历史可拉，体验缺失，否决）。下播 `ClearRoomMsg` 清空（场次隔离），开播 `Reset` 同步清计数。

### 2B.3 频控参数（与视频弹幕差异）

| 参数 | 视频弹幕 | 直播弹幕 | 依据 |
| --- | --- | --- | --- |
| 发送冷却 | 5s（dm:cd） | **2s**（live:cd:{room}:{uid}） | 直播节奏快，B 站直播间口径 1-2s |
| 重复去重 | 30s（dm:dup） | **10s**（live:dup） | 同房间高频场景，30s 过苛 |
| 内容长度 | — | ≤50 字（上行帧 ≤512B） | WS 帧体量约束 |
| Redis 故障 | 报错拒绝 | 频控放行（日志告警） | 直播弹幕非资金数据，可用性优先 |

### 2B.4 WS 端点与消息协议

- `GET /api/v1/live/rooms/{id}/ws`（auth，token 走 query——WS 无法自定义 header，与 im 先例一致）。建连即进房：`EnterForce` 计数 + `enter` 帧广播 → 升级连接。上行：`heartbeat` / `danmaku`；下行：`enter` / `danmaku` / `pong`。
- `GET /api/v1/live/rooms/{id}/danmaku/history`：最近 50 条正序 + 当前在线人数。
- 弹幕校验链（服务端判定，不靠前端）：`EnsureCanPublish`（账号封禁/禁言）→ 房间直播中 → `InRoom`（未进房 91017）→ 房间禁言执行（91015，LIV-04 落地）→ 机审影子屏蔽（命中仅落库不广播）→ 频控（91016）→ 去重（91018）→ 落库 + 广播。拒绝经服务端日志留痕，WS 上行不回错误帧（客户端无法携带请求 id，错误提示走 LIV-06 前端本地频控预判）。
