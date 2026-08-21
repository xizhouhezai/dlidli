# spec：社区动态与关注

> 模块标识：community（关注 FLW / 动态 DYN） ｜ 优先级：P1 ｜ 覆盖版本：V1
> 上游输入：[产品概述](/product/overview) · [版本规划](/product/versions)
> 实施状态：见 [tasks](/specs/community/tasks)（spec 不记录进度）
>
> 变更记录：
> - 2026-08-21 自 PRD `community.md`（V1.0 草案）迁移为 SDD

## 1. 背景与用户故事

关注关系是留存的核心资产；动态流让"追更"有固定阵地，把一次性观看转化为长期订阅关系。

- 作为 **观众**，我可以关注喜欢的 UP 主并在动态页看到更新，以便不错过追更内容。
- 作为 **用户**，我可以发布图文动态与转发，以便在视频之外表达与互动。

## 2. 需求规格

### 2.1 关注体系

| ID | 需求 | 验收标准（EARS） | 优先级 |
| --- | --- | --- | --- |
| FLW-01 | 关注/取关 | **WHEN** 用户关注/取关他人 **THE SYSTEM SHALL** 生效（上限 2000 个关注）并更新按钮态（已关注/互相关注） | P1 |
| FLW-02 | 粉丝/关注列表 | **THE SYSTEM SHALL** 分页展示粉丝/关注列表，支持列表内直接关注操作 | P1 |
| FLW-03 | 特别关注 | **WHERE** 用户设置特别关注 **THE SYSTEM SHALL** 对其更新提供优先提醒（红点+推送） | P2 |
| FLW-04 | 关注分组 | **THE SYSTEM SHALL** 支持自定义分组（≤ 20 组） | P3 |
| FLW-05 | 黑名单 | **WHERE** 用户拉黑他人 **THE SYSTEM SHALL** 使双方互相不可见对方评论/弹幕/私信 | P1 |

### 2.2 动态发布

| ID | 需求 | 验收标准（EARS） | 优先级 |
| --- | --- | --- | --- |
| DYN-01 | 投稿动态 | **WHEN** 稿件过审发布 **THE SYSTEM SHALL** 自动生成投稿动态 | P1 |
| DYN-02 | 文字/图文动态 | **WHEN** 用户发布动态 **THE SYSTEM SHALL** 允许文字 ≤ 2000 字 + 最多 9 图，并经机审后发布 | P1 |
| DYN-03 | 转发动态 | **WHEN** 用户转发视频/动态 **THE SYSTEM SHALL** 生成带附言的转发动态并链式展示原始内容（见 [interaction spec](/specs/interaction/spec) SHR-01） | P1 |
| DYN-04 | 话题 | **THE SYSTEM SHALL** 支持 #话题# 关联聚合页与运营活动话题 | P2 |
| DYN-05 | 动态删除 | **WHERE** 作者或管理员删除动态 **THE SYSTEM SHALL** 生效；**IF** 转发链上原动态被删 **THEN THE SYSTEM SHALL** 显示"源内容已删除" | P1 |
| DYN-06 | 投票动态 | **THE SYSTEM SHALL** 支持发起投票（2-10 选项，可设截止期） | P3 |

### 2.3 动态流

| ID | 需求 | 验收标准（EARS） | 优先级 |
| --- | --- | --- | --- |
| DYN-10 | 关注流 | **THE SYSTEM SHALL** 将已关注用户的动态按时间倒序聚合，顶部提供 UP 主头像快捷筛选 | P1 |
| DYN-11 | 动态互动 | **THE SYSTEM SHALL** 支持对动态点赞、评论（复用评论体系）、转发 | P1 |
| DYN-12 | 类型筛选 | **THE SYSTEM SHALL** 支持按类型筛选（全部/仅视频投稿/追番远期） | P2 |
| DYN-13 | 综合动态页 | **WHERE** 未登录或无关注 **THE SYSTEM SHALL** 展示热门动态引导 | P2 |

## 3. 边界与异常

- **IF** 关注数达到 2000 上限 **THEN THE SYSTEM SHALL** 拒绝新关注并提示。
- **IF** 动态含敏感内容 **THEN THE SYSTEM SHALL** 预检拦截或影子屏蔽（机审）。

## 4. 成功指标

| 指标 | 目标 |
| --- | --- |
| 关注流加载 | P99 < 400ms |
| 发布动态到粉丝可见延迟 | < 10s |
| 关注渗透率（有关注行为用户占比） | ≥ 50% |
| 关注流人均日访问 | ≥ 2 次 |

## 5. 依赖与关联

- 依赖：稿件状态（[video spec](/specs/video/spec)：过审钩子触发 DYN-01）；评论体系（[interaction spec](/specs/interaction/spec) CMT）；机审（DYN-02）
- 被依赖：通知（[notification spec](/specs/notification/spec)：关注/动态事件触发）；黑名单（FLW-05）被评论/弹幕/私信消费
