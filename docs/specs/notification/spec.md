# spec：消息通知与私信

> 模块标识：notification（通知 MSG / 私信 MSG） ｜ 优先级：P1 ｜ 覆盖版本：V1（通知）/ V2（私信）/ V3（推送）
> 上游输入：[产品概述](/product/overview) · [版本规划](/product/versions)
> 实施状态：见 [tasks](/specs/notification/tasks)（spec 不记录进度）
>
> 变更记录：
> - 2026-08-21 自 PRD `notification.md`（V1.0 草案）迁移为 SDD；补录 MSG-20（离线推送，原 §2.3 未编号条目）

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
