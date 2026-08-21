# spec：创作者中心

> 模块标识：creator ｜ 优先级：P1 ｜ 覆盖版本：MVP（基础投稿管理）→ V2（完整）
> 上游输入：[产品概述](/product/overview) · [版本规划](/product/versions)
> 实施状态：见 [tasks](/specs/creator/tasks)（spec 不记录进度）
>
> 变更记录：
> - 2026-08-04 CRT-10 数据趋势扩展：统计卡 5 项指标（总播放/点赞/投币/粉丝/收益）可查趋势，指标粒度细化，趋势图升级 echarts
> - 2026-08-21 自 PRD `creator.md` 迁移为 SDD；补录 CRT-40（创作辅助，原 §2.5 未编号条目）

## 1. 背景与用户故事

创作者是内容供给侧的根本。创作者中心提供"投稿管理 → 数据洞察 → 粉丝运营 → 收益变现"一站式工作台（Web 为主，H5 提供数据概览）。

- 作为 **UP 主**，我可以在创作者中心查看播放/互动/粉丝数据趋势，以便优化创作方向。
- 作为 **UP 主**，我可以集中管理稿件、评论与弹幕，以便高效运营内容。
- 作为 **UP 主**，我可以查看与提取创作收益，以便内容变现。

## 2. 需求规格

### 2.1 稿件管理（MVP 基础）

| ID | 需求 | 验收标准（EARS） | 优先级 |
| --- | --- | --- | --- |
| CRT-01 | 稿件列表 | **THE SYSTEM SHALL** 按状态筛选稿件（进行中/已发布/已驳回/已锁定）并展示核心数据（播放/点赞/评论） | P0 |
| CRT-02 | 状态跟踪 | **THE SYSTEM SHALL** 可视化上传→转码→审核进度并展示驳回原因 | P0 |
| CRT-03 | 稿件编辑/删除 | **THE SYSTEM SHALL** 提供编辑/删除入口（复用投稿流程，见 [video spec](/specs/video/spec) VID-06/07） | P0 |
| CRT-04 | 评论管理 | **THE SYSTEM SHALL** 支持集中查看/回复/删除/置顶自己稿件的评论 | P1 |
| CRT-05 | 弹幕管理 | **THE SYSTEM SHALL** 支持查看/删除自己稿件的弹幕与关键词黑名单 | P2 |

### 2.2 数据中心（V2）

| ID | 需求 | 验收标准（EARS） | 优先级 |
| --- | --- | --- | --- |
| CRT-10 | 概览看板 | **THE SYSTEM SHALL** 提供昨日/近 7 日/近 30 日的播放、观看时长、互动、涨粉等指标（粒度：有效播放/点赞/投币/收藏/涨粉/收益/互动/点击/曝光）及环比趋势图 | P1 |
| CRT-11 | 单稿分析 | **THE SYSTEM SHALL** 提供播放来源（推荐/搜索/关注/其他）、观众留存曲线、互动漏斗 | P1 |
| CRT-12 | 粉丝画像 | **THE SYSTEM SHALL** 提供性别/年龄/地域/活跃时段分布（脱敏聚合） | P2 |
| CRT-13 | 数据对比 | **THE SYSTEM SHALL** 支持多稿件横向对比 | P2 |

### 2.3 粉丝运营（V2）

| ID | 需求 | 验收标准（EARS） | 优先级 |
| --- | --- | --- | --- |
| CRT-20 | 粉丝列表 | **THE SYSTEM SHALL** 提供新增粉丝列表与铁粉标识（高互动粉丝） | P2 |
| CRT-21 | 动态发布入口 | **THE SYSTEM SHALL** 提供快捷发布图文动态/预告入口（见 [community spec](/specs/community/spec) DYN-02） | P1 |

### 2.4 收益中心（V2 末 / V3）

| ID | 需求 | 验收标准（EARS） | 优先级 |
| --- | --- | --- | --- |
| CRT-30 | 创作激励 | **WHEN** UP 主满足开通条件（实名 + 粉丝 ≥ 1000 或播放 ≥ 10 万）**THE SYSTEM SHALL** 按有效播放计算并发放激励金 | P1（V2） |
| CRT-31 | 充电收入 | **WHEN** 观众充电打赏 **THE SYSTEM SHALL** 按 UP 主 70% / 平台 30% 分成入创作者收益（见 [monetization spec](/specs/monetization/spec) CHG-02） | P1（V3） |
| CRT-32 | 提现 | **THE SYSTEM SHALL** 支持绑定支付宝/银行卡、月度结算与个税代扣提示 | P1（V3） |
| CRT-33 | 收益明细 | **THE SYSTEM SHALL** 提供按日/按稿件的收益流水 | P1（V3） |

### 2.5 创作辅助（远期）

| ID | 需求 | 验收标准（EARS） | 优先级 |
| --- | --- | --- | --- |
| CRT-40 | 创作辅助（迁移补录） | **THE SYSTEM SHALL** 提供热门选题灵感（分区热点）、封面模板、AI 字幕、违规预检提示 | P3（远期） |

## 3. 边界与异常

- **IF** UP 主未实名 **THEN THE SYSTEM SHALL** 阻止开通收益类功能（见 [account spec](/specs/account/spec) ACC-23）。

## 4. 成功指标

| 指标 | 目标 |
| --- | --- |
| 投稿次日留存（发布第 2 稿比例） | ≥ 30% |
| 数据看板 T+1 更新准时率 | 100%（核心指标 V2.5 起准实时） |

## 5. 依赖与关联

- 依赖：稿件数据（[video spec](/specs/video/spec) video_stat）、行为日志（[search-recommend plan](/specs/search-recommend/plan)）、互动数据（[interaction spec](/specs/interaction/spec)）、充电分成（[monetization spec](/specs/monetization/spec)）
- 被依赖：多 P 投稿与合集任务在此交付（覆盖 VID-05/VID-23，见 [video spec](/specs/video/spec)）
