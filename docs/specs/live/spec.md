# spec：直播

> 模块标识：live ｜ 优先级：P2 ｜ 覆盖版本：V3（预研）
> 上游输入：[产品概述](/product/overview) · [版本规划](/product/versions) · [非功能需求](/product/nfr)（合规）
> 实施状态：见 [tasks](/specs/live/tasks)（spec 不记录进度）
>
> 变更记录：
> - 2026-08-21 自 PRD `live.md`（预研草案）迁移为 SDD。本文档为范围预研，开发前需细化。

## 1. 背景与用户故事

直播提升用户时长与商业化空间（礼物打赏），并为 UP 主提供第二收入曲线。

- 作为 **主播**，我可以通过 OBS 推流开播并管理房间，以便与粉丝实时互动。
- 作为 **观众**，我可以观看直播、发弹幕、送礼物，以便参与实时互动并支持主播。

## 2. 需求规格

### 2.1 主播侧（第一期）

| ID | 需求 | 验收标准（EARS） | 优先级 |
| --- | --- | --- | --- |
| LIV-01 | 开播资格 | **WHERE** 用户申请开播 **THE SYSTEM SHALL** 要求实名认证 + 粉丝 ≥ 100（初期白名单制） | P2 |
| LIV-02 | 开播管理 | **THE SYSTEM SHALL** 提供房间标题/封面/分区设置、RTMP 推流地址+密钥（配合 OBS）、开播/下播 | P2 |
| LIV-03 | 直播预告 | **WHEN** 主播预约开播 **THE SYSTEM SHALL** 关联动态发布预告并在开播时提醒粉丝 | P2 |
| LIV-04 | 房管 | **THE SYSTEM SHALL** 支持任命房管、禁言用户、弹幕关键词屏蔽 | P2 |

### 2.2 观众侧（第一期）

| ID | 需求 | 验收标准（EARS） | 优先级 |
| --- | --- | --- | --- |
| LIV-10 | 直播间列表 | **THE SYSTEM SHALL** 提供直播首页（推荐 + 分区）与关注的主播开播提醒 | P2 |
| LIV-11 | 观看 | **THE SYSTEM SHALL** 支持 HLS/HTTP-FLV 拉流、清晰度切换、人气值展示 | P2 |
| LIV-12 | 直播弹幕 | **THE SYSTEM SHALL** 提供实时聊天弹幕（WebSocket）、进场消息与弹幕频控 | P2 |
| LIV-13 | 礼物打赏 | **THE SYSTEM SHALL** 支持充值金瓜子 → 赠送礼物 → 礼物特效 → 主播分成（强一致，见 [monetization spec](/specs/monetization/spec) PAY-01） | P2 |
| LIV-14 | 直播回放 | **WHEN** 主播下播 **THE SYSTEM SHALL** 自动生成回放稿件（主播可选发布） | P2 |

## 3. 边界与异常

- **IF** 直播内容违规 **THEN THE SYSTEM SHALL** 经截帧机审（每 10s）+ 人工巡查快速切断。

## 4. 合规前置（硬约束）

- 网络文化经营许可证 / 直播相关资质。
- 主播实名 + 未成年人打赏限制与退款通道。
- 直播内容存档 ≥ 60 天（监管要求）。

## 5. 成功指标

| 指标 | 目标 |
| --- | --- |
| 直播渗透率（V3 商业化北极星辅助指标） | 见 [产品概述](/product/overview) |
| 推流 → 观看延迟 | HLS 6-15s；HTTP-FLV/WebRTC ≤ 3s（演进目标） |

## 6. 依赖与关联

- 依赖：实名认证（[account spec](/specs/account/spec) ACC-23）、粉丝关系（[community spec](/specs/community/spec) FLW）、支付与分成（[monetization spec](/specs/monetization/spec)）、动态预告（DYN）
- 被依赖：直播回放生成稿件（[video spec](/specs/video/spec)）；直播弹幕复用 comet 模式（[danmaku plan](/specs/danmaku/plan) hub 演进）
