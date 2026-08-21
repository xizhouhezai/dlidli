# spec：互动体系

> 模块标识：interaction（点赞 ITR / 投币 ITR / 收藏 ITR / 评论 CMT / 分享 SHR） ｜ 优先级：P0（点赞/评论）/ P1（投币/收藏/三连） ｜ 覆盖版本：MVP / V1
> 上游输入：[产品概述](/product/overview) · [版本规划](/product/versions)
> 实施状态：见 [tasks](/specs/interaction/tasks)（spec 不记录进度）
>
> 变更记录：
> - 2026-08-21 自 PRD `interaction.md`（V1.0 草案）迁移为 SDD；补录 ITR-40（防刷风控，原 §3 未编号条目）

## 1. 背景与用户故事

互动是社区活跃度与内容分发权重的核心信号。"三连"是平台文化符号，投币消耗硬币建立"付出感"，比单纯点赞更能代表内容质量。

- 作为 **观众**，我可以点赞、投币、收藏与评论，以便表达对内容的支持。
- 作为 **观众**，我可以长按点赞完成三连，以便用平台文化符号快速支持 UP 主。
- 作为 **UP 主**，我可以管理（删除/置顶）稿件评论，以便运营社区氛围。

## 2. 需求规格

### 2.1 点赞（MVP）

| ID | 需求 | 验收标准（EARS） | 优先级 |
| --- | --- | --- | --- |
| ITR-01 | 视频点赞/取消 | **WHEN** 用户对视频点赞/取消 **THE SYSTEM SHALL** 保证单账号单视频至多一次，且计数实时展示 | P0 |
| ITR-02 | 评论/弹幕点赞 | **WHEN** 用户点赞评论/弹幕 **THE SYSTEM SHALL** 记录并计入热度排序（评论 P0 / 弹幕 P1） | P0 评论 / P1 弹幕 |
| ITR-03 | 点踩 | **WHERE** 用户点踩 **THE SYSTEM SHALL** 仅记录为负反馈信号，不展示计数 | P2 |

### 2.2 投币（V1）

| ID | 需求 | 验收标准（EARS） | 优先级 |
| --- | --- | --- | --- |
| ITR-10 | 投币 | **WHEN** 用户对稿件投币 **THE SYSTEM SHALL** 允许 1-2 枚（自制 ≤ 2 枚、转载 ≤ 1 枚），并扣减硬币（见 [account spec](/specs/account/spec) ACC-21） | P1 |
| ITR-11 | 投币回馈 | **WHEN** 用户投币 **THE SYSTEM SHALL** 支持勾选"同时点赞"；UP 主获得经验/激励权重 | P1 |
| ITR-12 | 硬币不足提示 | **IF** 硬币余额不足 **THEN THE SYSTEM SHALL** 提示并引导查看硬币获取任务 | P1 |

### 2.3 收藏（V1）

| ID | 需求 | 验收标准（EARS） | 优先级 |
| --- | --- | --- | --- |
| ITR-20 | 默认收藏夹 | **WHEN** 用户一键收藏 **THE SYSTEM SHALL** 收藏至默认收藏夹（懒创建，不可删除） | P1 |
| ITR-21 | 多收藏夹 | **THE SYSTEM SHALL** 支持创建/重命名/删除收藏夹（≤ 50 个），含公开/私密属性 | P1 |
| ITR-22 | 收藏管理 | **THE SYSTEM SHALL** 支持批量移动/移除、失效稿件标记 | P2 |
| ITR-23 | 稍后再看 | **THE SYSTEM SHALL** 提供独立队列（≤ 100 个）与播放页快捷入口 | P2 |

### 2.4 一键三连（V1）

| ID | 需求 | 验收标准（EARS） | 优先级 |
| --- | --- | --- | --- |
| ITR-30 | 长按三连 | **WHEN** 用户长按点赞按钮 1.5s **THE SYSTEM SHALL** 触发：点赞 + 投币（默认 2 枚，不足则 1 枚）+ 收藏（默认夹），并播放动画反馈 | P1 |

### 2.5 评论（MVP）

| ID | 需求 | 验收标准（EARS） | 优先级 |
| --- | --- | --- | --- |
| CMT-01 | 发布评论 | **WHEN** 用户发布评论 **THE SYSTEM SHALL** 支持一级评论 + 二级回复（楼中楼），≤ 1000 字，支持 @用户 与表情（@ 与表情 P1） | P0 |
| CMT-02 | 评论排序 | **THE SYSTEM SHALL** 默认按"热度"（赞数+回复数+时间衰减）排序，可切换"最新" | P0 |
| CMT-03 | 评论分页 | **THE SYSTEM SHALL** 一级评论分页加载；楼中楼默认露出 2 条，点击展开 | P0 |
| CMT-04 | 删除 | **WHERE** 操作者为评论作者、视频 UP 主或管理员 **THE SYSTEM SHALL** 允许删除该评论 | P0 |
| CMT-05 | UP 主运营 | **THE SYSTEM SHALL** 支持置顶 1 条评论、UP 主标识、"UP 觉得很赞"标识 | P1 |
| CMT-06 | 评论举报 | **WHEN** 用户举报评论 **THE SYSTEM SHALL** 受理并入举报处理体系（见 [admin spec](/specs/admin/spec) ADT-10） | P1 |
| CMT-07 | 评论区开关 | **WHERE** UP 主关闭评论区或开启精选模式 **THE SYSTEM SHALL** 生效（先审后放） | P2 |
| CMT-08 | 机审 | **WHEN** 评论提交 **THE SYSTEM SHALL** 先经敏感词/内容安全过滤，命中影子屏蔽或拒绝 | P0 |

### 2.6 转发分享（V1）

| ID | 需求 | 验收标准（EARS） | 优先级 |
| --- | --- | --- | --- |
| SHR-01 | 站内转发 | **WHEN** 用户转发视频 **THE SYSTEM SHALL** 生成个人动态（可附评论，见 [community spec](/specs/community/spec) DYN-03） | P1 |
| SHR-02 | 站外分享 | **THE SYSTEM SHALL** 支持复制链接、二维码海报；小程序微信卡片分享 | P1 |

### 2.7 防刷与风控

| ID | 需求 | 验收标准（EARS） | 优先级 |
| --- | --- | --- | --- |
| ITR-40 | 互动风控（迁移补录） | **IF** 互动接口按 UID+IP 触发限频或短时计数激增 **THEN THE SYSTEM SHALL** 限流/触发风控复核并剔除作弊量 | P0 |

## 3. 边界与异常

- **IF** 投币/点赞请求重复提交（并发或重放）**THEN THE SYSTEM SHALL** 以幂等键保证只生效一次；投币失败（如余额扣减异常）**THEN THE SYSTEM SHALL** 退款补偿。

## 4. 成功指标

| 指标 | 目标 |
| --- | --- |
| 互动接口 | P99 < 200ms |
| 计数最终一致延迟 | < 5s |
| 三连率（三连数/播放量） | ≥ 1.5% |
| 评论率 | ≥ 2% |

## 5. 依赖与关联

- 依赖：硬币账户（[account spec](/specs/account/spec) ACC-21，投币扣减）；通知触发（[notification spec](/specs/notification/spec)）；内容机审（CMT-08）
- 被依赖：互动计数是推荐权重与热度榜因子（[search-recommend spec](/specs/search-recommend/spec) REC-03）；评论/点赞数据回流创作者中心（[creator spec](/specs/creator/spec) CRT-10）
