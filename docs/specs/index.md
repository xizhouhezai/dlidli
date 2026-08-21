# SDD 规格库（Spec-Driven Development）

> 状态：生效 ｜ 建立日期：2026-08-21 ｜ 前身：`product/prd/`（11 篇 PRD）与 `project/checklist.md`（任务台账），已完成全量迁移

本目录是 DliDli **功能规格的单一事实来源**：每个功能模块一个目录，内含三件套 **spec（需求）→ plan（方案）→ tasks（任务）**。需求变更先改 spec 并评审，通过后才能进入开发；代码、提交与进度均以 spec 为基准追溯。

## 1. 目录结构

```
specs/
├── index.md            # 本页：SDD 总纲
├── templates/          # 三件套模板（新模块从这里复制）
│   ├── spec.md / plan.md / tasks.md
├── account/            # 账号体系（注册登录 / 资料 / 成长）
├── video/              # 视频投稿与播放
├── danmaku/            # 弹幕系统
├── interaction/        # 互动体系（赞 / 币 / 藏 / 评 / 三连）
├── community/          # 社区动态与关注
├── notification/       # 消息通知与私信
├── search-recommend/   # 搜索与推荐
├── creator/            # 创作者中心
├── admin/              # 内容审核与管理后台（含 RBAC）
├── monetization/       # 会员与商业化
├── live/               # 直播（V3 预研）
└── engineering/        # 工程与端侧横切任务（不设独立 spec，以架构文档为基线）
```

## 2. 三件套职责

| 文件 | 回答 | 内容 | 禁止出现 |
| --- | --- | --- | --- |
| `spec.md` | **WHAT** | 用户故事、EARS 验收标准、边界与异常、成功指标、模块依赖 | 技术选型、表结构、接口路径等实现细节；实现进度与状态 |
| `plan.md` | **HOW** | 技术决策（决策/理由/备选）、数据模型、接口设计、关键流程、风险 | 需求定义与优先级 |
| `tasks.md` | **进度** | 按里程碑组织的任务清单：勾选状态 + 完成日期 + 验证结论 + 需求覆盖 | 新增需求（新需求必须先进 spec） |

**WHAT / HOW 判定准则**：描述"系统对外的可观察行为"（用户可感知、可验证）→ spec；描述"实现该行为的手段"（更换手段而行为不变）→ plan。例如"登录态 2 小时内有效"是 spec，"用 JWT 实现登录态"是 plan。

## 3. EARS 验收标准语法

spec 中的每条需求必须给出 EARS（Easy Approach to Requirements Syntax）验收标准，关键字用**粗体英文**，条件与响应用中文：

| 句式 | 模板 | 适用 |
| --- | --- | --- |
| 无条件 | **THE SYSTEM SHALL** 响应 | 恒成立的系统义务 |
| 事件驱动 | **WHEN** 触发 **THE SYSTEM SHALL** 响应 | 某事件发生时 |
| 状态驱动 | **WHILE** 状态 **THE SYSTEM SHALL** 响应 | 某状态持续期间 |
| 不需要的行为 | **IF** 条件 **THEN THE SYSTEM SHALL** 响应 | 异常/边界防护 |
| 可选特性 | **WHERE** 特性包含 **THE SYSTEM SHALL** 响应 | 可选功能挂载点 |

示例（取自 [interaction spec](/specs/interaction/spec)）：

> **WHEN** 用户长按点赞按钮 1.5s **THE SYSTEM SHALL** 触发三连：点赞 + 投币（默认 2 枚，不足则 1 枚）+ 收藏（默认夹），并播放动画反馈
>
> **IF** 同一用户在同一视频 5s 内重复发送弹幕 **THEN THE SYSTEM SHALL** 拒绝第二条并返回频控错误

## 4. 编号体系与追溯链路

**需求 ID**：沿用原 PRD 的分域前缀（`ACC-` `VID-` `PLY-` `DM-` `ITR-` `CMT-` `SHR-` `FLW-` `DYN-` `MSG-` `SRH-` `REC-` `CRT-` `ADT-` `VMG-` `UGV-` `OPS-` `ADMU-` `ROLE-` `PERM-` `SYS-` `VIP-` `CHG-` `PAY-` `ADV-` `LIV-`），同一子域内按 10 号留空插入。迁移时为原 PRD 未编号但已存在（部分已实现）的需求补录了新 ID，补录条目在 spec 中标注"迁移补录"。

**任务 ID**：沿用原开发清单规则 `{阶段}-{模块}-{序号}`（如 `M1-VID-03`），与 git 分支命名（`feature/m1-vid-05-xxx`）及 PR 关联要求保持一致，见 [协作规范](/project/conventions)。

**追溯链路**：

```
spec.md 需求 ID ──覆盖──> tasks.md 任务 ID ──关联──> git 分支 / PR / 提交
      │                        │
      └──── plan.md 技术决策（HOW 承接 WHAT）────┘
```

- 每个 tasks 任务必须标注"覆盖"的需求 ID（跨模块覆盖合法，如 `M2-ITR-01 硬币体系` 覆盖 `ACC-21`）；
- 无覆盖对象的任务（纯工程/体验类）标注"覆盖：—（工程）"；
- 需求的完成状态由 tasks 反查，spec 永不记录进度。

## 5. SDD 工作流

新功能或需求变更必须按以下顺序进行（与 [协作规范](/project/conventions) 的"先文档后编码"一致）：

```
clarify ──> specify ──> plan ──> tasks ──> implement ──> checklist
（澄清）    （改 spec）  （定方案） （拆任务）  （开发勾选）  （交付核对）
```

| 阶段 | 动作 | 产出 |
| --- | --- | --- |
| clarify | 在模块 spec 下澄清需求边界（可选） | 讨论结论 |
| specify | 修改 `spec.md`：新增/修改需求行（EARS）+ 头部 changelog | 需求评审通过 |
| plan | 修改 `plan.md`：技术决策、数据模型、接口 | 方案确定 |
| tasks | 在 `tasks.md` 拆解任务并建立"覆盖"关系 | 任务清单 |
| implement | 开发实现，完成任务即勾选 + 日期 + 验证结论 | 代码合并 |
| checklist | 交付前逐条核对覆盖需求的 EARS 验收标准 | 全部满足方可关闭 |

**变更纪律**：需求变更必须先改 spec（含 changelog 与日期）再动代码，禁止"代码先行、文档后补"；spec 与实现不一致时以先修正 spec 为第一动作。

## 6. 与其他文档目录的关系

| 目录 | 角色 | 与 specs 的关系 |
| --- | --- | --- |
| `product/` | 上游输入 | 产品定位、用户画像、版本规划（MVP→V3 范围切分）、非功能需求（全局约束，所有 spec 默认继承） |
| `architecture/` | 全局横切架构 | 后端/前端/数据模型/视频流水线，是所有 plan 的技术基线；plan 只写模块级差异 |
| `project/` | 进度与协作 | roadmap（里程碑与 DoD）、progress（汇总视图，数据来自各 tasks）、conventions（含 SDD 流程规范） |

## 7. 模块索引

| 模块 | spec 需求 | plan 方案 | tasks 任务 | 版本 |
| --- | --- | --- | --- | --- |
| [账号体系](/specs/account/spec) | [spec](/specs/account/spec) | [plan](/specs/account/plan) | [tasks](/specs/account/tasks) | MVP 起 |
| [视频投稿与播放](/specs/video/spec) | [spec](/specs/video/spec) | [plan](/specs/video/plan) | [tasks](/specs/video/tasks) | MVP 起 |
| [弹幕系统](/specs/danmaku/spec) | [spec](/specs/danmaku/spec) | [plan](/specs/danmaku/plan) | [tasks](/specs/danmaku/tasks) | MVP / V1 |
| [互动体系](/specs/interaction/spec) | [spec](/specs/interaction/spec) | [plan](/specs/interaction/plan) | [tasks](/specs/interaction/tasks) | MVP / V1 |
| [社区动态与关注](/specs/community/spec) | [spec](/specs/community/spec) | [plan](/specs/community/plan) | [tasks](/specs/community/tasks) | V1 |
| [消息通知与私信](/specs/notification/spec) | [spec](/specs/notification/spec) | [plan](/specs/notification/plan) | [tasks](/specs/notification/tasks) | V1 / V2 |
| [搜索与推荐](/specs/search-recommend/spec) | [spec](/specs/search-recommend/spec) | [plan](/specs/search-recommend/plan) | [tasks](/specs/search-recommend/tasks) | V1 / V2 |
| [创作者中心](/specs/creator/spec) | [spec](/specs/creator/spec) | [plan](/specs/creator/plan) | [tasks](/specs/creator/tasks) | MVP / V2 |
| [内容审核与管理后台](/specs/admin/spec) | [spec](/specs/admin/spec) | [plan](/specs/admin/plan) | [tasks](/specs/admin/tasks) | MVP 起 |
| [会员与商业化](/specs/monetization/spec) | [spec](/specs/monetization/spec) | [plan](/specs/monetization/plan) | [tasks](/specs/monetization/tasks) | V3 |
| [直播](/specs/live/spec) | [spec](/specs/live/spec) | [plan](/specs/live/plan) | [tasks](/specs/live/tasks) | V3 预研 |
| [工程与端侧（横切）](/specs/engineering/tasks) | — | — | [tasks](/specs/engineering/tasks) | M0-M4 |
