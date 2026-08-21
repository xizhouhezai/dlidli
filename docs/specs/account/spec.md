# spec：账号体系

> 模块标识：account ｜ 优先级：P0 ｜ 覆盖版本：MVP 起（成长体系 V1）
> 上游输入：[产品概述](/product/overview) · [用户画像](/product/personas) · [版本规划](/product/versions) · [非功能需求](/product/nfr)
> 实施状态：见 [tasks](/specs/account/tasks)（spec 不记录进度）
>
> 变更记录：
> - 2026-08-21 自 PRD `account.md`（V1.0 草案）迁移为 SDD；补录 ACC-30/40/41/42（原 §2.4 安全风控未编号条目）

## 1. 背景与用户故事

账号是所有业务的基础，需支持多端统一账号（Web/H5/小程序/App 同一 UID），并承载社区成长体系（等级、硬币）。

- 作为**新用户**，我可以用手机号验证码一步完成注册与登录，以便零门槛开始使用平台。
- 作为**用户**，我可以管理资料、隐私与登录设备，以便掌控个人身份与安全。
- 作为**活跃用户**，我可以积累经验等级与硬币，以便解锁更多互动权益（如彩色弹幕、投币）。

## 2. 需求规格

### 2.1 注册与登录（MVP）

| ID | 需求 | 验收标准（EARS） | 优先级 |
| --- | --- | --- | --- |
| ACC-01 | 手机号注册/登录 | **WHEN** 用户提交有效手机号与正确短信验证码 **THE SYSTEM SHALL** 为已注册手机号直接登录、为未注册手机号自动注册（默认昵称 dli_xxxx）并颁发登录凭证（验证码 60s 冷却、10 分钟有效） | P0 |
| ACC-02 | 邮箱注册/登录 | **WHEN** 用户以邮箱 + 密码注册 **THE SYSTEM SHALL** 创建待激活账号并发送激活邮件，**WHERE** 账号未激活 **THE SYSTEM SHALL** 拒绝登录 | P0 |
| ACC-03 | 密码登录 | **WHEN** 用户以手机号/邮箱 + 正确密码登录 **THE SYSTEM SHALL** 颁发登录凭证；**IF** 密码连续错误 5 次 **THEN THE SYSTEM SHALL** 锁定 15 分钟并强制图形验证码 | P0 |
| ACC-04 | 微信登录 | **WHERE** 微信端（小程序授权 / Web 扫码）**THE SYSTEM SHALL** 支持微信身份登录并绑定既有账号 | P1（V2 小程序同步） |
| ACC-05 | 找回密码 | **WHEN** 用户通过短信/邮件完成身份验证并重置密码 **THE SYSTEM SHALL** 生效新密码 | P0 |
| ACC-06 | 多端会话 | **THE SYSTEM SHALL** 支持多端同账号登录：登录态短期有效（2h）且 30d 内可免登录刷新；在线设备可查看/踢出（踢出 P1） | P0（踢出 P1） |
| ACC-07 | 注销账号 | **WHEN** 用户申请注销 **THE SYSTEM SHALL** 进入 7 天冷静期，期满后对账号数据匿名化 | P1 |

### 2.2 用户资料

| ID | 需求 | 验收标准（EARS） | 优先级 |
| --- | --- | --- | --- |
| ACC-10 | 基础资料 | **WHEN** 用户编辑昵称（唯一，2-24 字符）、头像、签名、性别、生日 **THE SYSTEM SHALL** 保存并在个人空间即时生效 | P0 |
| ACC-11 | 头像上传 | **WHEN** 用户上传头像 **THE SYSTEM SHALL** 提供裁剪能力，且仅在机审通过后对外生效 | P0 |
| ACC-12 | 个人空间页 | **THE SYSTEM SHALL** 在个人空间展示投稿列表、动态、收藏（可设隐私）、关注/粉丝数 | P0 基础 / P1 完整 |
| ACC-13 | 隐私设置 | **WHERE** 用户开启隐私开关 **THE SYSTEM SHALL** 对其他用户隐藏对应收藏夹/关注列表 | P1 |

### 2.3 成长体系（V1）

| ID | 需求 | 验收标准（EARS） | 优先级 |
| --- | --- | --- | --- |
| ACC-20 | 经验等级 | **THE SYSTEM SHALL** 按行为累积经验（登录 +5/日、观看 +5/日、投稿 +10、弹幕评论互动等）并映射 Lv0-Lv6 等级；升级解锁权益（Lv1 可发弹幕、Lv3 可发彩色弹幕） | P1 |
| ACC-21 | 硬币系统 | **THE SYSTEM SHALL** 通过登录、投稿被赞等途径发放硬币（每日获取上限），硬币仅用于投币 | P1 |
| ACC-22 | 每日任务 | **THE SYSTEM SHALL** 提供每日任务列表与奖励领取 | P2 |
| ACC-23 | 实名认证 | **WHERE** 用户发布内容 **THE SYSTEM SHALL** 要求手机号实名（合规前置）；UP 主开通收益需身份证实名 | P1（收益前置） |
| ACC-30 | 青少年模式（迁移补录） | **WHEN** 用户开启青少年模式 **THE SYSTEM SHALL** 限制每日使用时长并收敛可用内容池 | P1（合规 V1 必须） |

### 2.4 账号安全与风控

| ID | 需求 | 验收标准（EARS） | 优先级 |
| --- | --- | --- | --- |
| ACC-40 | 敏感操作二次验证（迁移补录） | **WHEN** 用户执行改密码/换绑手机等敏感操作 **THE SYSTEM SHALL** 要求二次验证 | P1 |
| ACC-41 | 登录与注册风控（迁移补录） | **IF** 系统检测到异地登录或批量注册特征 **THEN THE SYSTEM SHALL** 触发登录提醒或注册拦截（设备指纹 + IP 风控） | P1 |
| ACC-42 | 资料内容机审（迁移补录） | **WHEN** 用户提交昵称/签名/头像 **THE SYSTEM SHALL** 先经内容安全机审（策略见 [admin spec](/specs/admin/spec) 审核对象与策略） | P0 |

## 3. 边界与异常

- **IF** 同一手机号 1 分钟内已请求过短信 **THEN THE SYSTEM SHALL** 拒绝本次发送（上限：1 分钟 1 条、1 天 10 条）。
- **WHEN** 注册/改名的昵称冲突 **THE SYSTEM SHALL** 自动追加随机后缀并提示用户修改。
- **WHEN** 处于封禁期的用户尝试登录 **THE SYSTEM SHALL** 提示封禁原因与期限，并提供申诉入口（V1）。

## 4. 成功指标

| 指标 | 目标 |
| --- | --- |
| 注册转化率（进入注册页 → 完成注册） | ≥ 60% |
| 短信到达率 | ≥ 98% |
| 登录接口 | P99 < 300ms |

## 5. 依赖与关联

- 依赖：内容安全机审（[admin plan](/specs/admin/plan) contentmod）、对象存储（头像）
- 被依赖：
  - 所有 C 端模块消费登录态与 UID；
  - 等级权益被 [danmaku spec](/specs/danmaku/spec) 消费（DM-01 发送门槛 Lv1、DM-02 彩色 Lv3）；
  - 硬币被 [interaction spec](/specs/interaction/spec) 消费（ITR-10 投币）；
  - 实名认证被 [creator spec](/specs/creator/spec)（CRT-30 开通收益）与 [live spec](/specs/live/spec)（LIV-01 开播资格）消费；
  - 青少年模式（ACC-30）为 [nfr 合规](/product/nfr)的落地点。
