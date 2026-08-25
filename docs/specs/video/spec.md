# spec：视频投稿与播放

> 模块标识：video（播放 PLY） ｜ 优先级：P0 ｜ 覆盖版本：MVP 起
> 上游输入：[产品概述](/product/overview) · [版本规划](/product/versions) · [非功能需求](/product/nfr)
> 实施状态：见 [tasks](/specs/video/tasks)（spec 不记录进度）
>
> 变更记录：
> - 2026-08-21 自 PRD `video.md`（V1.0 草案）迁移为 SDD；补录 VID-20（分区体系）、VID-23（合集，已实现未立规格）、PLY-30/31（详情页）；PLY-08 签名有效期按实现修正为 6h（原 PRD 2h）
> - 2026-08-25 新增 VID-24（上传文件归属校验，安全加固）：稿件引用的上传文件必须属于当前用户，杜绝越权复用他人已上传原文件

## 1. 背景与用户故事

视频是平台核心资产。本模块覆盖：上传 → 转码 → 审核 → 发布 → 播放的完整生命周期。

- 作为 **UP 主**，我可以在 Web 端上传视频、填写稿件信息并提交审核，以便发布作品。
- 作为 **观众**，我可以流畅播放视频（清晰度/倍速/进度记忆），以便获得良好观看体验。
- 作为 **平台**，我需要对稿件全生命周期状态管理，以便审核治理与数据统计。

## 2. 需求规格

### 2.1 投稿（Web 端，MVP）

| ID | 需求 | 验收标准（EARS） | 优先级 |
| --- | --- | --- | --- |
| VID-01 | 视频上传 | **WHEN** 用户上传视频文件（≤ 8GB；mp4/mov/mkv/flv/avi）**THE SYSTEM SHALL** 按 5MB 分片接收，支持断点续传与基于文件 hash 的秒传 | P0 |
| VID-02 | 稿件信息 | **WHEN** 用户提交稿件 **THE SYSTEM SHALL** 校验：标题 ≤ 80 字、简介 ≤ 2000 字、分区必选、标签 1-10 个、类型自制/转载（转载必填出处） | P0 |
| VID-03 | 封面 | **WHEN** 用户设置封面 **THE SYSTEM SHALL** 支持自定义上传（16:10）或从视频抽帧候选中选择 | P0 |
| VID-04 | 定时发布 | **WHEN** 稿件过审且到达预约时间 **THE SYSTEM SHALL** 自动发布 | P1 |
| VID-05 | 多P投稿 | **WHEN** UP 主提交多 P 稿件（≤ 10 P）**THE SYSTEM SHALL** 逐 P 转码并在播放页提供分 P 列表切换 | P2（V2） |
| VID-06 | 稿件编辑 | **WHEN** 用户编辑已发布稿件的标题/简介/封面/标签 **THE SYSTEM SHALL** 保存并重新送审；**THE SYSTEM SHALL NOT** 允许替换视频文件 | P1 |
| VID-07 | 稿件删除 | **WHEN** 用户删除稿件 **THE SYSTEM SHALL** 软删除（前台不可见）并保留播放数据用于统计 | P0 |
| VID-20 | 分区体系（迁移补录） | **THE SYSTEM SHALL** 以 12 个一级分区组织内容（动画/游戏/科技数码/知识/生活/美食/音乐/舞蹈/影视/体育/时尚/娱乐）；V1 起二级分区由运营后台配置（见 [admin spec](/specs/admin/spec) OPS-01） | P0 |
| VID-23 | 合集（迁移补录） | **WHEN** UP 主创建合集并归集稿件 **THE SYSTEM SHALL** 在空间页提供合集 Tab 与合集详情页；合集仅本人可管理 | P1（V2） |
| VID-24 | 上传文件归属校验（安全） | **IF** 稿件引用的上传文件归属于其他用户 **THEN THE SYSTEM SHALL** 拒绝提交并返回无权限错误；**WHEN** 秒传命中他人文件 **THE SYSTEM SHALL** 不暴露该文件的存储标识供投稿复用 | P0 |

### 2.2 转码与存储（MVP）

| ID | 需求 | 验收标准（EARS） | 优先级 |
| --- | --- | --- | --- |
| VID-10 | 自动转码 | **WHEN** 上传完成 **THE SYSTEM SHALL** 自动入队转码并输出 HLS（H.264 + AAC） | P0 |
| VID-11 | 清晰度档位 | **THE SYSTEM SHALL** 提供 MVP 360P/720P；V1 +1080P；V3 +4K（大会员，见 [monetization spec](/specs/monetization/spec)） | P0 |
| VID-12 | 转码进度通知 | **WHEN** 转码状态变化 **THE SYSTEM SHALL** 发送站内通知并在创作者中心展示状态（见 [notification spec](/specs/notification/spec)） | P1 |
| VID-13 | 视频指纹 | **WHEN** 稿件提交 **THE SYSTEM SHALL** 计算抽帧 hash 用于重复/搬运检测 | P2 |

### 2.3 稿件状态机（业务规则）

```
草稿 → 上传中 → 转码中 → 待审核 → ┬→ 已发布（开放浏览）
                                  ├→ 已驳回（可修改后重新提交）
                                  └→ 已锁定（违规下架，可申诉）
已发布 → 已删除（UP 主删除 / 管理员删除）
```

### 2.4 播放（MVP）

| ID | 需求 | 验收标准（EARS） | 优先级 |
| --- | --- | --- | --- |
| PLY-01 | HLS 播放器 | **THE SYSTEM SHALL** 提供 HLS 播放能力（Web 播放器内核；H5/小程序原生 video + HLS 地址） | P0 |
| PLY-02 | 清晰度切换 | **THE SYSTEM SHALL** 支持手动切换与按带宽自动切换；未登录限 480P 以下，大会员解锁 4K | P0 |
| PLY-03 | 倍速播放 | **THE SYSTEM SHALL** 支持 0.5x / 0.75x / 1x / 1.25x / 1.5x / 2x / 3x 倍速 | P0 |
| PLY-04 | 进度记忆 | **WHEN** 用户中断观看 **THE SYSTEM SHALL** 服务端记录进度并支持跨端续播 | P0 |
| PLY-05 | 播放统计 | **WHEN** 观看时长 > 5s **THE SYSTEM SHALL** 计一次有效播放（去重计数），并埋点完播率 | P0 |
| PLY-06 | 快捷键 | **WHERE** Web 播放器获得焦点 **THE SYSTEM SHALL** 响应快捷键（空格暂停、←→ 快进退、↑↓ 音量、F 全屏、D 弹幕开关） | P1（Web） |
| PLY-07 | 画中画 / 网页全屏 | **THE SYSTEM SHALL** 提供画中画与网页全屏模式（Web） | P1 |
| PLY-08 | 防盗链 | **WHEN** 下发播放地址 **THE SYSTEM SHALL** 附签名 URL（有效期 6h，按 M1-VID-05 实现修正；原 PRD 2h）并做 Referer 校验；**IF** 签名缺失/篡改/过期 **THEN THE SYSTEM SHALL** 拒绝访问 | P0 |
| PLY-09 | 试看与限制 | **IF** 稿件处于锁定状态 **THEN THE SYSTEM SHALL** 禁止播放；地区限制（预留） | P2 |
| PLY-30 | 详情信息区（迁移补录） | **THE SYSTEM SHALL** 在视频详情页展示：标题、UP 主卡片（头像/昵称/粉丝数/关注按钮）、播放数/弹幕数/发布时间、简介折叠展开、标签跳转搜索 | P0 |
| PLY-31 | 相关推荐（迁移补录） | **THE SYSTEM SHALL** 在详情页提供相关推荐列表（MVP 同分区最热，V2 换算法推荐，见 [search-recommend spec](/specs/search-recommend/spec)） | P1 |

> 详情页的互动栏与评论区为 [interaction spec](/specs/interaction/spec) 的聚合展示，不在此重复定义。

## 3. 边界与异常

- **IF** 上传中断后 24h 内重新发起 **THEN THE SYSTEM SHALL** 允许断点续传；超时分片清理。
- **IF** 转码失败 **THEN THE SYSTEM SHALL** 自动重试 2 次，仍失败则通知 UP 主并允许重传。
- **WHEN** 播放地址过期 **THE SYSTEM SHALL** 由播放器自动静默换取新地址续播（用户无感）。

## 4. 成功指标

| 指标 | 目标 |
| --- | --- |
| 上传成功率 | ≥ 99% |
| 1GB 文件转码完成 | ≤ 15 分钟 |
| 播放起播时间 | P90 < 1.5s |
| 卡顿率 | < 1% |

## 5. 依赖与关联

- 依赖：审核流（[admin spec](/specs/admin/spec) ADT，状态机"待审核→已发布/驳回/锁定"）；对象存储与 CDN（[视频处理流水线](/architecture/video-pipeline)）
- 被依赖：稿件是互动（ITR/CMT/DM）、推荐（REC）、搜索（SRH）、创作者中心（CRT）的对象载体；清晰度权益被 monetization 消费（VIP 4K）
- 转码规格与 Worker 调度实现见 [video plan](/specs/video/plan) 与 [视频处理流水线](/architecture/video-pipeline)
