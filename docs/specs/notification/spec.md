# spec：消息通知与私信

> 模块标识：notification（通知 MSG / 私信 MSG） ｜ 优先级：P1 ｜ 覆盖版本：V1（通知）/ V2（私信）/ V3（推送）
> 上游输入：[产品概述](/product/overview) · [版本规划](/product/versions)
> 实施状态：见 [tasks](/specs/notification/tasks)（spec 不记录进度）
>
> 变更记录：
> - 2026-08-21 自 PRD `notification.md`（V1.0 草案）迁移为 SDD；补录 MSG-20（离线推送，原 §2.3 未编号条目）
> - 2026-10-16 新增 §2.4 与 MSG-21~24：WS 握手 Origin 口径与 native 客户端例外
>   （`app.allowNativeClients`，默认 fail closed）。原因：鸿蒙 ArkTS WebSocket 自生成
>   `http://<host>` 来源被纯白名单拒绝，导致实时通道全程降级轮询

## 1. 背景与用户故事

互动闭环需要"被回应感"：评论被回复、视频被点赞都应及时触达；私信满足用户间、商务合作沟通需求。

- 作为 **内容作者**，我的评论被回复、内容被点赞/@ 时能收到通知，以便及时回应互动。
- 作为 **用户**，我可以与他人一对一私信，以便进行深度交流。
- 作为 **系统**，审核结果与违规处罚等系统事件应触达用户。

## 2. 需求规格

### 2.1 通知中心（V1）

| ID | 需求 | 验收标准（EARS） | 优先级 |
| --- | --- | --- | --- |
| MSG-01 | 回复我的 | **WHEN** 用户的评论/弹幕被回复 **THE SYSTEM SHALL** 聚合通知并可点击跳转原文定位 | P1 |
| MSG-02 | @ 我的 | **WHEN** 用户在评论/动态中被 @ **THE SYSTEM SHALL** 发送通知 | P1 |
| MSG-03 | 收到的赞 | **WHEN** 用户内容被点赞 **THE SYSTEM SHALL** 按内容聚合通知（"xxx 等 N 人赞了你的评论"）防刷屏 | P1 |
| MSG-04 | 系统通知 | **WHEN** 审核结果、违规处罚、活动公告、等级提升事件发生 **THE SYSTEM SHALL** 发送系统通知 | P1 |
| MSG-05 | 未读红点 | **THE SYSTEM SHALL** 维护分类未读数（上限 99+），已读即消，支持全部标记已读 | P1 |
| MSG-06 | 通知设置 | **WHERE** 用户关闭某类通知 **THE SYSTEM SHALL** 不再推送该类提醒 | P2 |

### 2.2 私信（V2）

| ID | 需求 | 验收标准（EARS） | 优先级 |
| --- | --- | --- | --- |
| MSG-10 | 一对一私信 | **WHEN** 用户发送私信 **THE SYSTEM SHALL** 支持文字（≤ 500 字）与图片，会话列表按最新消息排序 | P1（V2） |
| MSG-11 | 发送限制 | **IF** 双方未互关 **THEN THE SYSTEM SHALL** 限制每天最多向同一人发送 1 条；互关后不限 | P1（V2） |
| MSG-12 | 消息治理 | **IF** 接收方已拉黑发送方 **THEN THE SYSTEM SHALL** 拒绝发送；**THE SYSTEM SHALL** 支持举报会话与机审过滤 | P1（V2） |
| MSG-13 | 在线推送 | **WHERE** 接收方在线 **THE SYSTEM SHALL** 经 WebSocket 实时下发；离线进入未读 | P1（V2） |

### 2.3 推送（V3，随 App）

| ID | 需求 | 验收标准（EARS） | 优先级 |
| --- | --- | --- | --- |
| MSG-20 | 离线推送（迁移补录） | **WHERE** 用户离线且绑定 App **THE SYSTEM SHALL** 经厂商通道推送，运营推送每日 ≤ 3 条；特别关注实时推送、普通关注每日聚合 | P2（V3） |

### 2.4 WebSocket 握手 Origin 口径（2026-10-16 补录）

> 本节的 WS 通道指 `notification` 域两个实时端点：`/notifications/ws`（MSG-13 通知）与
> `/messages/ws`（MSG-13 私信）。同口径亦适用于 `danmaku` 与 `live` 域的 WS 端点。

**背景**：浏览器同源策略不约束 WebSocket，故服务端必须校验 `Origin` 防跨站 WS 劫持（CSWSH）。
但移动端/native 客户端（鸿蒙 ArkTS）会**自生成** `Origin`（形如 `http://<host>`、不带端口，
`connect()` 的同名 header 覆盖不掉），按纯白名单判定必然被拒 → 端侧降级为轮询。

| ID | 需求 | 验收标准（EARS） | 优先级 |
| --- | --- | --- | --- |
| MSG-21 | WS 握手来源校验 | **WHEN** WS 握手携带 `Origin` **THE SYSTEM SHALL** 仅放行白名单（`app.allowOrigins`）内的来源；**WHEN** 请求完全不带 `Origin` 头（非浏览器客户端）**THE SYSTEM SHALL** 放行 | P1 |
| MSG-22 | native 客户端来源例外 | **WHERE** `app.allowNativeClients` 开启 **AND** `Origin` 不在白名单 **AND** 请求携带经 JWT 校验有效的 access token（`?token=` 或 `Authorization: Bearer`）**THEN THE SYSTEM SHALL** 放行该次握手；**IF** token 缺失或无效 **THEN THE SYSTEM SHALL** 拒绝（403） | P1 |
| MSG-23 | 例外默认关闭（fail closed） | **WHERE** `app.allowNativeClients` 为 `false`（默认）**THE SYSTEM SHALL** 对非白名单 `Origin` 一律拒绝——**即使**请求携带有效 token | P1 |
| MSG-24 | 游客 native 连接的边界 | **WHERE** 请求未携带有效 token（游客）**AND** `Origin` 不在白名单 **THEN THE SYSTEM SHALL** 拒绝该次握手；即允许游客只读的 WS 端点，其 native 客户端仍须让来源命中白名单 | P1 |

**安全性约束（实现不得违反）**

- 例外判据必须是**显式凭据**，**不得**按来源形态（`file://`、无端口 `http://<host>` 等）裸放行。
- 本仓库凭据**只走 Bearer / query token，不使用 Cookie 会话**（`Set-Cookie`/`document.cookie` 零使用）。
  这是本例外不扩大浏览器攻击面的前提：浏览器不会自动携带该凭据，恶意页面拿不到 token；
  若将来引入 Cookie 鉴权，**必须重新评估本节**（届时 CSWSH 前提成立，例外需另加约束）。
- 四个 WS hub（danmaku/notify/im/live）**必须共用同一实现**，防止策略语义漂移。
- **未登录（游客）连接不受例外覆盖**：例外判据是有效 token，游客没有 token，
  故其 `Origin` 仍须命中白名单。对允许游客只读的弹幕 WS，这一点必须在部署时知悉
  （见 MSG-24 与 [deployment §5.6](/project/deployment)）。
- 鸿蒙 ArkTS 自生成 `Origin` 的默认形态是 `http://<host>`（**http 前缀、不带端口**），
  与 `https://<域名>` 白名单条目**不相等**；运维不得假定「App 同源即命中」，
  须以真实握手日志（`WS Origin`）为准。

## 3. 边界与异常

- **IF** 通知/私信内容命中机审敏感词 **THEN THE SYSTEM SHALL** 拦截或影子屏蔽。
- **IF** 用户被禁言 **THEN THE SYSTEM SHALL** 拦截其私信发送。

## 4. 成功指标

| 指标 | 目标 |
| --- | --- |
| 通知生成延迟 | < 5s |
| 未读计数准确率 | 100% |
| 通知点击率 | ≥ 20%（回复类 ≥ 50%） |

## 5. 依赖与关联

- 依赖：互动/审核/系统事件源（各模块触发）；WebSocket 通道（[danmaku plan](/specs/danmaku/plan) hub 模式复用）
- 被依赖：举报反馈（[admin spec](/specs/admin/spec) ADT-10 处理结果通知举报人）；审核结果触达（[video spec](/specs/video/spec) VID-12）
