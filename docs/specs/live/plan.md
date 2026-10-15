# plan：直播

> 对应规格：[spec](/specs/live/spec) ｜ 技术基线：[总体架构](/architecture/overview) · [视频处理流水线](/architecture/video-pipeline)
> 实施状态：V3.4（2026-10-15 LIV-05 细化）；M4-LIV-01/02/03/04/06 已交付、M4-LIV-05 已实现。
>
> 变更记录：
> - 2026-08-21 随 spec 迁移为 SDD，技术预研。
> - 2026-10-13 补 M4-LIV-02 技术决策：房间表结构（§2.1）、WS 通道与 danmaku hub 关系（§2.2）、在线人数实现（§2.4）、开播密钥（§2.5）、API 布局（§3）。
> - 2026-10-13 补 M4-LIV-03 技术决策：live hub 形态与连接治理（§2B.1）、弹幕持久化口径（§2B.2）、频控参数（§2B.3）、WS 端点布局（§3B）。
> - 2026-10-13 补 M4-LIV-04 技术决策：资产强一致实现（§2C.1）、充值幂等与 PAY-01 替换点（§2C.2）、礼物/分成表结构（§2C.3）、WS 特效帧（§2C.4）。
> - 2026-10-15 补 M4-LIV-05 技术决策（§2D）：截帧选型与 Worker 调度（§2D.1）、机审复用 contentmod 与 fail-open 口径（§2D.2）、快速切断的确定性与降级（§2D.3）、表结构与巡查接口（§2D.4）、阈值实测标定（§2D.5）。

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

## 2C. M4-LIV-04 技术决策（2026-10-13）

### 2C.1 资产强一致实现

**决策：MySQL 事务 + `SELECT ... FOR UPDATE` 行锁（`lockAccount`），单事务完成「锁余额 → 校验扣减 → 观众流水 → 礼物流水 → 分成流水」。**

- 行锁是唯一并发闸：同账户并发扣款在锁上串行化，余额校验在锁内读，负余额不可能出现；账户行懒创建（无账户先插 0 余额行，唯一键冲突重读加锁）。
- 广播在事务**提交成功后**执行（Service 层）：扣款失败（91021）不产生任何流水、不广播特效帧。
- 备选（否决）：A) Redis 预扣 + 异步落库——异步崩窗会丢单，与强一致要求冲突；B) 乐观锁 version 字段——20 并发重试风暴，行锁在 MySQL 上更直接。

### 2C.2 充值幂等与 PAY-01 替换点

- 入账统一走 `RechargeTx(userID, delta, bizNo, remark)`：`melon_seed_log.uk_biz_no` 唯一键承载幂等，同 biz_no 重复入账返回当前余额且 `applied=false`（集成测试 TestRechargeIdempotent 覆盖）。
- **PAY-01 替换点（写明）**：① 真实支付回调以支付单号为 bizNo 调 `RechargeTx`，天然防重放；② 开发直充 `POST /api/v1/admin/dev/recharge`（AdminAuth + RequirePerm("config:edit")），bizNo=`dev-{雪花}-{uid}`，PAY-01 启动后下线；③ 金瓜子与真实货币的兑换比价随 PAY-01 定，本卡瓜子直接按个计。

### 2C.3 礼物/分成表结构（迁移 0036）

五表：`melon_seed_account`（余额）/`melon_seed_log`（流水，uk_biz_no 幂等）/`gift`（配置，含价格数量级为开发口径的种子数据）/`gift_log`（赠送流水，单价/分成比例下单时快照，雪花 ID 主键）/`gift_share_log`（分成流水，uk_gift_log 防重复分成）。分成比例 700‰（plan 预研值，对齐 monetization CHG-02 的 70/30），月度结算归 creator 体系（PAY-02 消费 gift_share_log）。

### 2C.4 WS 特效帧与前端渲染

- live hub 新增消息类型 `gift`（`GiftFrame`：log_id/sender_id/nickname/gift_name/count/effect/level），`BroadcastGift` 走既有房间广播（复用 LIV-03 hub，单连接顶替/慢消费者丢帧语义不变）。
- 前端（LIV-06 页面挂靠）：`GiftPanel.vue`（礼物列表/余额/连击/赠送，91021 引导提示）+ `GiftEffects.vue`（普通礼物弹幕区连击浮层、高级礼物全屏 CSS 动画——粒子上浮 + 礼物名浮现，4s 退场，不引动画库）。`useLiveDanmaku` 增加 `gift` 帧分发（`giftEvents`）。

## 2D. M4-LIV-05 技术决策（2026-10-15）

对应 spec §5E。四项决策逐条给出选型、理由与否决备选。

### 2D.1 截帧选型与 Worker 调度

**决策：HTTP-FLV + 后端进程内 `time.Ticker` Worker，每周期对该房间截 1 帧。**

- 截帧命令：`ffmpeg -y -loglevel error -i <flv url> -frames:v 1 -f image2 <out>.png`（单帧、单进程、无中间文件）。`-frames:v 1` 在收到首个可解码视频帧后即退出，不需要 `-t`。
- **源选 HTTP-FLV 而非 HLS**：FLV 是 SRS 实时转封装（`http_remux`），首帧到达即可截；HLS 依赖切片落盘（`hls_fragment` 秒级），截帧至少多等一个切片周期，且新开播房间在首个切片产出前没有 m3u8。
- **源不选 SRS 截图 API**：SRS 5 的 HTTP API 只有 streams/clients/versions 这类运维端点，**没有截图端点**（已实测 `GET /api/v1/versions` 与 `/api/v1/clients` 的形态）。截帧必须后端调 ffmpeg。
- **调度**：与 `internal/module/video/transcoder.go` 同风格——`StartLiveAuditWorker(ctx)` 内 `for { select { case <-ctx.Done(): return; case <-ticker.C: ... } }`，在 `router.go` 的 DB/Redis 就绪分支启动。**不引入 Redis 队列 / MQ / 独立 worker 进程**（YAGNI：单机单实例、房间数少，进程内定时器足够；横向扩展时改为「按房间分片 + 分布式锁」是后续项，登记在 §4 风险表）。
- **每周期遍历 `status=1` 的房间**，逐房间**串行**截帧（避免一瞬间起 N 个 ffmpeg 进程打满 CPU）；单房间超时用 `context.WithTimeout` 兜底（默认 15s），进程被 `exec.CommandContext` 连带杀掉。
- 开关 `live.audit.enabled` **dev 默认 false**；周期 `live.audit.frameIntervalSeconds` 默认 10（spec §3 硬要求）。
- 备选（否决）：A) 独立 cmd/worker 进程——本卡只解决「有没有」，拆分是部署演进项；B) SRS 侧 `exec` 旁路脚本截帧——把业务判定逻辑放进媒体服务器，违反分层且不可测。

### 2D.2 截帧机审：复用 contentmod + 本地画面规则 + fail-open

**决策：机审走 `pkg/contentmod`；本卡为其新增图片入口 `CheckImage`，先只实现无外部依赖的本地画面规则。**

- **复用既有接口**（需求原文：复用 `pkg/contentmod` 的既有接口）：`CheckText` 已是视频机审的通用入口；图片检测按包内既有 TODO 的口径「增加 external 实现并保持接口不变」扩展为 `CheckImage(scene, imagePath) Result`。
- **本地画面规则的实现方式（实测修正）**：**不用 ffprobe**，改用 Go 标准库 `image.Decode` + BT.601 加权自行统计 `YAVG`/`YMIN`/`YMAX`（按步长抽样到约 65536 点，复杂度与分辨率解耦）。
  - 原因（2026-10-15 实测）：计划初稿写的 `ffprobe -f lavfi -i "movie=<路径>,signalstats"` 在 Windows 上**不可用**——`movie=C:/…`、`movie='C:/…'`、`movie=C\:/…` 三种写法全部报 `Failed to avformat_open_input 'C'`（lavfi 滤镜图把 `:` 当选项分隔符，盘符必被吃掉），只有「工作目录 + 相对文件名」能成功；而 `-i x.png -vf signalstats` 也不行（ffprobe 无 `-vf`）。照抄会导致**恒返回 degraded，即「黑帧永不命中」的静默假通过**。
  - 纯 Go 实现顺带去掉一个外部进程依赖，并让 `CheckImage` 可在测试里用内存构造的图直接覆盖。
- **阈值走配置**（`live.audit.blackYAvgMax` / `live.audit.flatRangeMax`，默认值见 §2D.5 实测标定），避免把魔法数写进代码。
- **外部图审 API 是替换点（写明）**：`contentmod.CheckImage` 当前只做本地规则；接入外部内容安全服务时在同一函数内增加 external 分支（包内既有 TODO 的既定做法），调用方（`live` 审计服务）不改。
- **fail-open 决策（与项目「Redis 故障放行」口径一致）**：机审链路异常（ffmpeg 不可用 / 图片读不到 / 尺寸异常 / 规则执行 panic）时 **不切断**，只记 `degraded=true` 的截帧记录。理由：直播可用性是 P2 功能的前提，自动化误切（把正常直播切掉）的代价远高于漏判（漏判由**人工巡查**这条腿兜住，spec §3 边界原文即是「截帧机审 + 人工巡查」两条腿）。**这是待替换点**：接入外部图审后，「无法判定」不应再等同于「通过」——应改为按比例告警/转人工队列；本卡在 plan 与 tasks 中登记该替换点。
- 备选（否决）：A) fail-closed（机审不可用即切断）——机审不可用时会把全部直播切断，线上事故；B) 机审异常静默丢弃——不可审计。本卡选「记录降级事实 + 不切断」。

### 2D.3 快速切断的确定性（SRS API + 降级登记）

**决策：优先走 SRS API 踢流（确定性主通道），不可达时降级为「仅改房间封禁状态 + 流水标记 degraded」。**

- 踢流链路（**字段名以 2026-10-15 真实推流实测为准，勿照抄旧描述**）：`GET /api/v1/streams/` 找到 `name` 匹配 `room_<id>` 且 `publish.active=true` 的流 → 取 `publish.cid` → `DELETE /api/v1/clients/<cid>`。
  - 实测纠正：`publish.active` / `publish.cid` **只出现在 `/api/v1/streams/`**；`/api/v1/clients/` 里 `publish` 是**布尔** `true`，且其 `stream` 字段是**内部 sid（`vid-…`）而非流名**，流名也在 `name`。按 `/clients/` 解析会取不到 cid。
  - 两个端点都**必须带尾斜杠**（无斜杠 302 重定向）；`DELETE /api/v1/clients/<id>` 成功返回 `{"code":0}`，**目标不存在返回 `{"code":2049}`（视为成功，幂等）**。
- **踢流的确切性**：SRS 断开推流后会触发 `on_unpublish` → 后端既有 `StreamStop` 把房间置 status=2（已下播），这一条已经在 LIV-01 验证过（`{"stopped":true}`）。所以「踢流 → 房间状态」是有回调保证的因果链，不需要轮询确认。
- **降级（SRS API 不可达）**：直接改房间封禁状态 + 落 `cut_mode=degraded` 的违规流水。**不静默**：接口响应与流水都带降级标记，管理端可见。
- **封禁语义**：`live_room.ban_status`（0 正常 / 1 封禁）+ `ban_reason` / `ban_by` / `ban_at`。**刻意不复用 `status` 枚举**：`status` 是场地生命周期（0 空闲/1 直播中/2 已下播），封禁是合规处置，两者正交；复用会让「封禁中的房间」污染既有 `idx_status` 列表查询与开播校验的语义。
- **封禁与开播的关系**：`AuthorizePublish`（on_publish 鉴权）增加封禁判定——被封禁房间即使密钥正确也拒绝推流（否则主播一踢就再推，切断形同虚设）。这是本卡对 LIV-01 契约的一处扩展。

### 2D.4 表结构与巡查接口（迁移 0037）

三张表（全 InnoDB / utf8mb4，字段命名对齐既有 live 表）：

- `live_frame_audit`：`room_id`、`anchor_id`、`image_key`（存储键，可空）、`width`/`height`、`pass`、`hit_words`（命中原因）、`degraded`、`cost_ms`、`created_at`。索引 `idx_room_time`（房间 + 时间，巡查按房间看最近判定）。**每周期每房间一行**——这是「判定发生过」的审计凭据（spec §5E.5 要求，保留期 ≥60 天由配置声明）。
- `live_violation_log`：`room_id`、`anchor_id`、`source`（1 机审 / 2 人工巡查）、`reason`、`cut_mode`（1 srs_api / 2 degraded）、`frame_audit_id`（可空，人工巡查无来源帧）、`cut_by`（0=系统，>0=admin_user.id）、`created_at`。索引 `idx_room_time`、`idx_source`。
- `live_room` 增列：`ban_status`、`ban_reason`、`ban_by`、`ban_at`（`ALTER TABLE`，与 0034 同表）。

**巡查接口**（挂 `v1.Group("/admin", AdminAuth, RequirePerm("live:audit"))`，写法对齐既有 `RegisterDevRoutes`）：

| 方法/路径 | 说明 |
| --- | --- |
| GET `/admin/live/rooms` | 直播中房间列表（含在线人数 + 最近一次截帧判定摘要） |
| POST `/admin/live/rooms/:id/cut` | 手动切断（body: `reason`），走 §2D.3 同一处置链，source=2 |
| POST `/admin/live/rooms/:id/restore` | 解封（`ban_status=0`） |

**权限点 `live:audit` 落库**：`admin_permission` 表由 `rbac_model.go` 的 `builtinPerms` 在启动时 seed（`seedRBAC` 用 `clause.OnConflict DoUpdates` 幂等），新增权限点条目 + 内置角色（`review_lead`/`moderator`）归属即可生效；**仅加路由不加权限点会让所有非 super 管理员 403**，故两者必须同改。

### 2D.5 实测标定（2026-10-15，见 `.dev-logs/liv05/`）

- 截帧源与命令形态、单帧耗时、产出尺寸：见 `.dev-logs/liv05/recon/`。
- 黑帧/纯色阈值标定（`YAVG` / `YMIN`-`YMAX` 差值）：见 `.dev-logs/liv05/recon/thresholds.md`；配置默认值取自该实测量化区间。


