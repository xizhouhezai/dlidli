# spec：原生鸿蒙端（HarmonyOS NEXT）

> 模块标识：harmony（端侧基础 HMY / 播放 HMY / 弹幕 HMY / 互动 HMY / 发现 HMY） ｜ 优先级：P1 ｜ 覆盖版本：M4
> 上游输入：[产品概述](/product/overview) · [用户画像](/product/personas) · [版本规划](/product/versions) · [非功能需求](/product/nfr)（全局约束，默认继承）
> 实施状态：见 [tasks](/specs/harmony/tasks)（spec 不记录进度）
> 技术方案：[plan](/specs/harmony/plan)
>
> 变更记录：
> - 2026-09-21 V1.0 初稿：确认采用 **ArkTS/ArkUI 原生**路线（非 uni-app 复用），首期范围为**观看端优先**；端侧需求以「继承各业务模块已有需求 + 声明端侧差异」方式书写，不重复定义业务验收标准

## 1. 背景与用户故事

鸿蒙（HarmonyOS NEXT）为存量 C 端之外的独立终端，需原生应用承接。选型结论为 **ArkTS/ArkUI 原生开发**（理由见 [plan §2 技术决策](/specs/harmony/plan)与 [ADR](/architecture/adr-m4-app-01-harmony)）：原生方案在滑动、动画、系统能力（媒体/分享/深色模式）上优于跨端壳，且鸿蒙生态是长期方向。

首期定位与小程序一致：**观看端优先**，跑通「登录 → 找内容 → 观看（含弹幕）→ 互动」闭环；投稿与创作者中心**不在本期范围**（引导至 Web）。

- 作为**鸿蒙设备用户**，我可以用手机号登录并观看视频（含弹幕），以便在原生应用内获得流畅体验。
- 作为**观众**，我可以点赞/投币/收藏/三连并发表评论，以便表达对 UP 主的支持。
- 作为**跨端用户**，我可以在鸿蒙端继续上次的观看进度与历史，以便无缝续看。

> **书写约定**：本节之后的「继承」表示业务验收标准以被继承模块的 spec 为单一事实来源，本 spec 不复制其语句；标「端侧差异」的条目为鸿蒙端特有或对实现有额外约束的要求，须独立验收。

## 2. 需求规格

### 2.1 端侧基础（M4）

| ID | 需求 | 验收标准（EARS） | 优先级 |
| --- | --- | --- | --- |
| HMY-01 | 登录与会话 | **WHEN** 用户在鸿蒙端完成登录 **THE SYSTEM SHALL** 颁发与 Web/H5/小程序同源的多端会话（继承 [account spec](/specs/account/spec) ACC-01 手机号验证码登录、ACC-03 密码登录、ACC-06 多端会话）；**WHEN** 访问令牌过期 **THE SYSTEM SHALL** 静默续期且用户无感，**IF** 续期失败 **THEN THE SYSTEM SHALL** 清理本地凭证并引导重新登录 | P0 |
| HMY-02 | 凭证本地存储 | **THE SYSTEM SHALL** 将令牌与刷新凭证存于系统偏好存储，**THE SYSTEM SHALL NOT** 明文写入日志或缓存文件；**WHEN** 用户主动退出登录 **THE SYSTEM SHALL** 清除全部本地凭证 | P0 |
| HMY-03 | 网络请求与错误处理 | **THE SYSTEM SHALL** 统一解析后端响应包裹与错误码并映射为可读文案（继承 [nfr](/product/nfr) 统一响应约定）；**IF** 网络不可用或请求超时 **THEN THE SYSTEM SHALL** 展示可重试的失败态而不白屏 | P0 |
| HMY-04 | 端侧存储 | **THE SYSTEM SHALL** 本地持久化弹幕展示设置与观看进度缓存，**WHEN** 应用重启 **THE SYSTEM SHALL** 恢复上述设置 | P1 |

### 2.2 播放（M4）

| ID | 需求 | 验收标准（EARS） | 优先级 |
| --- | --- | --- | --- |
| HMY-10 | HLS 播放 | **THE SYSTEM SHALL** 使用系统 AVPlayer 播放后端下发的 HLS（继承 [video spec](/specs/video/spec) PLY-01）；**端侧差异**：起播时间 P90 < 1.5s，播放卡顿率 < 1%（承 [nfr](/product/nfr)） | P0 |
| HMY-11 | 清晰度与倍速 | **THE SYSTEM SHALL** 支持清晰度切换与 0.5x~2x 倍速（继承 PLY-02、PLY-03）；**端侧差异**：切换清晰度**THE SYSTEM SHALL** 保留当前进度与播放态 | P0 |
| HMY-12 | 进度记忆与续播 | **WHEN** 用户中断观看 **THE SYSTEM SHALL** 记录进度并支持跨端续播（继承 PLY-04） | P0 |
| HMY-13 | 有效播放上报 | **WHEN** 观看时长 > 5s **THE SYSTEM SHALL** 计一次有效播放并上报（继承 PLY-05） | P0 |
| HMY-14 | 播放地址签名与续签 | **WHEN** 获取播放地址 **THE SYSTEM SHALL** 使用后端签名 URL（继承 PLY-08）；**IF** 播放中地址过期 **THEN THE SYSTEM SHALL** 静默换取新地址续播，用户无感 | P0 |
| HMY-15 | 播放器交互 | **端侧差异**：**THE SYSTEM SHALL** 提供双击播放/暂停、横向拖动进度、竖向拖动调亮度/音量、横屏全屏（继承 PLY-06 的键鼠快捷键语义需转换为触屏手势，Web 快捷键不适用）；**WHEN** 应用切后台 **THE SYSTEM SHALL** 暂停播放并保留进度 | P1 |

### 2.3 弹幕（M4）

| ID | 需求 | 验收标准（EARS） | 优先级 |
| --- | --- | --- | --- |
| HMY-20 | 弹幕渲染 | **THE SYSTEM SHALL** 按分段拉取并随播放进度渲染弹幕（继承 [danmaku spec](/specs/danmaku/spec) DM-10、DM-11）；**端侧差异**：同屏弹幕满载时渲染帧率 ≥ 55fps，且**THE SYSTEM SHALL NOT** 因弹幕渲染阻塞播放起播 | P0 |
| HMY-21 | 弹幕发送 | **THE SYSTEM SHALL** 支持发送弹幕（继承 DM-01、DM-04 频控；DM-02 字号/颜色、DM-03 顶/底模式随 M4 一并交付）；**IF** 用户等级不足 **THEN THE SYSTEM SHALL** 按 DM-02/DM-03 的等级门槛提示并阻断 | P0 |
| HMY-22 | 实时弹幕通道 | **WHEN** 同视频在线观众发送弹幕成功 **THE SYSTEM SHALL** 经 WebSocket 即时上屏（继承 DM-15）；**IF** 长连接断开 **THEN THE SYSTEM SHALL** 自动重连，**IF** 重连失败 **THEN THE SYSTEM SHALL** 回退分段拉取（承 DM-15 回退语义） | P0 |
| HMY-23 | 弹幕屏蔽与展示设置 | **THE SYSTEM SHALL** 支持关键词/发送者屏蔽与服务端账号级生效（继承 DM-20、DM-21），并提供不透明度/字号/显示区域/速度设置（继承 DM-12） | P1 |
| HMY-24 | 弹幕列表面板 | **THE SYSTEM SHALL** 提供按时间排序的弹幕列表并支持点击跳转进度（继承 DM-16） | P2 |

### 2.4 互动与评论（M4）

| ID | 需求 | 验收标准（EARS） | 优先级 |
| --- | --- | --- | --- |
| HMY-30 | 点赞/投币/收藏/三连 | **THE SYSTEM SHALL** 支持点赞、投币、收藏与长按三连（继承 [interaction spec](/specs/interaction/spec) ITR-01、ITR-10、ITR-20、ITR-30）；**端侧差异**：三连**THE SYSTEM SHALL** 以原生动画反馈，且**IF** 请求重复提交 **THEN THE SYSTEM SHALL** 依赖幂等键保证只生效一次（承 ITR §3） | P0 |
| HMY-31 | 评论 | **THE SYSTEM SHALL** 支持一级评论与二级回复、排序切换、分页与展开（继承 CMT-01、CMT-02、CMT-03），作者/UP 主可删除（继承 CMT-04） | P1 |
| HMY-32 | 分享 | **THE SYSTEM SHALL** 通过系统分享面板分享视频链接（继承 SHR-02 站外分享）；**端侧差异**：分享**THE SYSTEM SHALL NOT** 依赖微信小程序卡片（鸿蒙端无小程序载体） | P2 |

### 2.5 发现与个人中心（M4）

| ID | 需求 | 验收标准（EARS） | 优先级 |
| --- | --- | --- | --- |
| HMY-40 | 首页与分区 | **THE SYSTEM SHALL** 提供首页信息流与分区导航、最新/最热切换（继承 [search-recommend spec](/specs/search-recommend/spec) REC-01 与 [video spec](/specs/video/spec) VID-20）；**端侧差异**：列表**THE SYSTEM SHALL** 使用惰性加载，滚动不丢帧 | P0 |
| HMY-41 | 搜索 | **THE SYSTEM SHALL** 支持关键词搜索视频与 UP 主（继承 SRH-01、SRH-02 排序筛选）；**端侧差异**：搜索历史**THE SYSTEM SHALL** 与云端同步（承 SRH-04） | P1 |
| HMY-42 | 个人中心/历史/收藏 | **THE SYSTEM SHALL** 展示个人资料、我的投稿、观看历史与收藏（继承 ACC-12、ITR-21），**WHEN** 未登录 **THE SYSTEM SHALL** 展示登录入口 | P1 |

> **本期明确不含**（后续版本再评估，见 [versions](/product/versions)）：视频投稿与上传、创作者中心、私信 IM、通知中心与离线推送（[notification spec](/specs/notification/spec) MSG-20）、直播、会员支付、微信登录（鸿蒙端需微信开放平台鸿蒙版 SDK，可用性待查证）。

## 3. 边界与异常

- **IF** 稿件处于锁定状态 **THEN THE SYSTEM SHALL** 禁止播放（承 [video spec](/specs/video/spec) PLY-09）。
- **IF** 未登录用户观看 **THEN THE SYSTEM SHALL** 限制清晰度并引导登录（承 PLY-02），**THE SYSTEM SHALL NOT** 阻塞观看主链路。
- **IF** 网络在播放中中断 **THEN THE SYSTEM SHALL** 保留进度并在恢复后可从断点续播。
- **WHEN** 应用进入后台 **THE SYSTEM SHALL** 暂停播放与弹幕渲染，**IF** 后台驻留超时 **THEN THE SYSTEM SHALL** 释放播放器资源。
- **IF** 用户被禁言 **THEN THE SYSTEM SHALL** 拦截弹幕与评论发送并提示原因（承 [account spec](/specs/account/spec) §3）。
- **THE SYSTEM SHALL NOT** 在端侧绕过服务端鉴权：清晰度权益、弹幕等级门槛、频控一律以服务端校验为准。

## 4. 成功指标

| 指标 | 目标 |
| --- | --- |
| 冷启动耗时（至首页可交互，真机） | < 2s |
| 播放起播时间 | P90 < 1.5s |
| 弹幕满载渲染帧率 | ≥ 55fps |
| 崩溃率 | < 0.5% |
| 安装包体积 | 待实测（目标 < 30MB） |

## 5. 依赖与关联

- 依赖：
  - 后端 REST / WebSocket 契约（现有实现，**本期不改后端**）；
  - 各业务模块的端无关需求（account / video / danmaku / interaction / search-recommend），验收标准同源；
  - 内容与账号的合规前置（[nfr](/product/nfr) 实名制、青少年模式）——本期仅本地验证，不涉及上架合规。
- 被依赖：无（本期为终端消费者）。
- 全局约束：[非功能需求](/product/nfr) 中的性能 SLO、安全（传输加密、令牌管理）、兼容性。
