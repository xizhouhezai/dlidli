# spec：Flutter 移动端（Android + iOS）

> 模块标识：`flutter`（端侧基础 FLT / 播放 FLT / 弹幕 FLT / 互动 FLT / 发现 FLT / 创作 FLT）｜ 优先级：P1 ｜ 覆盖版本：M5
> 上游输入：[产品概述](/product/overview) · [用户画像](/product/personas) · [版本规划](/product/versions) · [非功能需求](/product/nfr)（全局约束，默认继承）
> 实施状态：见 [tasks](/specs/flutter/tasks)（spec 不记录进度）
> 技术方案：[plan](/specs/flutter/plan)
>
> 变更记录：
> - 2026-11-05 V1.0 初稿：确立 Android/iOS 双端用 **Flutter 单工程双产物**；定位为「与鸿蒙端并行、非替代」；首期范围**观看端 MVP**（登录/发现/播放/弹幕/互动/个人中心），创作端二期；iOS 本机不可验收的口径事先写死

## 1. 背景与用户故事

Web/H5/小程序/鸿蒙四端已交付，但**Android 与 iOS 用户在当前没有任何原生入口**——H5 在移动端体验受限（无系统级播放器、无后台播放、无法上架应用商店），而鸿蒙端是独立技术栈（ArkTS/ArkUI），其代码无法复用到 Android/iOS。Flutter 用一套 Dart 代码产出 Android 与 iOS 两个原生包，是覆盖这两个平台的最短路径。

**与鸿蒙端的关系：并行，不替代。** 鸿蒙端 16/16 已收口、真机验收手册已产出，推翻重写沉没成本过高；且鸿蒙不在 Flutter 官方 target 内（需第三方引擎），Flutter 无法覆盖鸿蒙。故两套端侧长期并存，**共享的是后端契约与产品口径，不共享端侧代码**。

- 作为 **Android/iOS 用户**，我可以用手机号登录并观看视频（含弹幕），以便在原生 App 内获得流畅体验。
- 作为 **观众**，我可以点赞/投币/收藏/三连并发表评论，以便表达对 UP 主的支持。
- 作为 **跨端用户**，我可以在手机端继续上次的观看进度与历史，以便无缝续看。
- 作为 **创作者**（二期），我可以直接在手机上选片投稿，以便不必先传到电脑再用 Web 投稿。

## 2. 需求规格

> 验收标准使用 EARS 语法（见 [SDD 总纲](/specs/)），关键字粗体。
> **书写约定**：标「继承」的条目表示业务验收标准以被继承模块 spec 为唯一事实来源，本 spec 不复述其语义；标「端侧差异」的条目为 Flutter 端特有或对实现有额外约束，须独立验收。

### 2.1 端侧基础（M5）

| ID | 需求 | 验收标准（EARS） | 优先级 |
| --- | --- | --- | --- |
| FLT-01 | 登录与会话 | **WHEN** 用户在 App 内完成登录 **THE SYSTEM SHALL** 颁发与 Web/H5/小程序/鸿蒙同源的多端会话（继承 [account spec](/specs/account/spec) ACC-01 手机号验证码、ACC-03 密码登录、ACC-06 多端会话）；**WHEN** 访问令牌过期 **THE SYSTEM SHALL** 静默续期且用户无感，**IF** 续期失败 **THEN THE SYSTEM SHALL** 清理本地凭证并引导重新登录 | P0 |
| FLT-02 | 凭证本地存储 | **THE SYSTEM SHALL** 将令牌与刷新凭证存于**平台安全存储**（Android Keystore / iOS Keychain），**THE SYSTEM SHALL NOT** 明文写入日志或普通缓存文件；**WHEN** 用户主动退出 **THE SYSTEM SHALL** 清除全部本地凭证 | P0 |
| FLT-03 | 网络层与错误处理 | **THE SYSTEM SHALL** 统一解析后端 `{code,message,data,trace_id}` 信封并映射为可读文案（继承 [nfr](/product/nfr) 统一响应约定）；**IF** 网络不可用或超时 **THEN THE SYSTEM SHALL** 展示可重试的失败态而不白屏；**端侧差异**：Android 模拟器访问宿主后端**SHALL** 使用 `10.0.2.2` 映射而非 `localhost` | P0 |
| FLT-04 | 端侧存储 | **THE SYSTEM SHALL** 本地持久化弹幕展示设置、观看进度缓存与搜索历史；**WHEN** 应用重启 **THE SYSTEM SHALL** 恢复上述设置 | P1 |
| FLT-05 | 平台适配与降级 | **THE SYSTEM SHALL** 同时产出 Android 与 iOS 两个原生包；**端侧差异**：鸿蒙的「沉浸光感」系统材质在 Android/iOS **无等价能力**，**THE SYSTEM SHALL** 降级为普通背景 + 卡片层级，**SHALL NOT** 因缺少材质导致内容不可读或控件不可点 | P0 |
| FLT-06 | 最小系统要求 | **THE SYSTEM SHALL** 支持 Android 8.0（API 26）及以上、iOS 14.0 及以上；**IF** 设备低于该版本 **THEN THE SYSTEM SHALL** 在应用商店侧限制安装而不是装后崩溃 | P1 |

### 2.2 播放（M5）

| ID | 需求 | 验收标准（EARS） | 优先级 |
| --- | --- | --- | --- |
| FLT-10 | HLS 播放 | **THE SYSTEM SHALL** 播放后端下发的签名 HLS 地址（继承 [video spec](/specs/video/spec) PLY-01）；**端侧差异**：起播时间 P90 < 1.5s、播放卡顿率 < 1%（继承 [nfr](/product/nfr) 阈值，验收以真机为准） | P0 |
| FLT-11 | 清晰度与倍速 | **THE SYSTEM SHALL** 支持清晰度切换与 0.5x~2x 倍速（继承 PLY-02、PLY-03）；**端侧差异**：切换清晰度 **SHALL** 保留当前进度与播放状态 | P0 |
| FLT-12 | 进度记忆与续播 | **WHEN** 用户中断观看 **THE SYSTEM SHALL** 记录进度并支持跨端续播（继承 PLY-04） | P0 |
| FLT-13 | 有效播放上报 | **WHEN** 观看时长 > 5s **THE SYSTEM SHALL** 计一次有效播放并上报（继承 PLY-05） | P0 |
| FLT-14 | 播放地址签名与续签 | **WHEN** 获取播放地址 **THE SYSTEM SHALL** 使用后端签名 URL（继承 PLY-08，形如 `?e=&q=&u=&s=`）；**IF** 播放中地址过期 **THEN THE SYSTEM SHALL** 静默换取新地址续播，用户无感 | P0 |
| FLT-15 | 播放器交互 | **端侧差异**：**THE SYSTEM SHALL** 提供单击播放/暂停、横向拖动进度、竖向左半屏亮度/右半屏音量、横屏全屏（继承 PLY-06 的键鼠快捷键语义，转换为触屏手势）；**WHEN** 应用切后台 **THE SYSTEM SHALL** 暂停或按系统能力后台播放并保留进度 | P1 |

### 2.3 弹幕（M5）

| ID | 需求 | 验收标准（EARS） | 优先级 |
| --- | --- | --- | --- |
| FLT-20 | 弹幕渲染 | **THE SYSTEM SHALL** 按分段拉取并随播放进度渲染弹幕（继承 [danmaku spec](/specs/danmaku/spec) DM-10、DM-11）；**端侧差异**：同屏弹幕满载时渲染帧率 ≥ 55fps，且 **THE SYSTEM SHALL NOT** 因弹幕渲染阻塞播放起播 | P0 |
| FLT-21 | 弹幕发送 | **THE SYSTEM SHALL** 支持发送弹幕（继承 DM-01、DM-04 频控；DM-02 字号/颜色、DM-03 顶/底模式随 M5 一并交付）；**IF** 用户等级不足 **THEN THE SYSTEM SHALL** 按 DM-02/DM-03 的等级门槛提示并阻断 | P0 |
| FLT-22 | 实时弹幕通道 | **WHEN** 同视频在线观众发送弹幕成功 **THE SYSTEM SHALL** 经 WebSocket 即时上屏（继承 DM-15）；**IF** 长连接断开 **THEN THE SYSTEM SHALL** 自动重连，**IF** 重连失败 **THEN THE SYSTEM SHALL** 回退分段拉取（承 DM-15 回退语义） | P0 |
| FLT-23 | 弹幕 WS 握手来源 | **端侧差异**：Flutter 的 WebSocket **SHALL** 在握手时携带有效 access token（`?token=` 或 `Authorization`），以命中服务端 `app.allowNativeClients` 的 native 例外分支（见 [deployment §5.6](/project/deployment)）；**IF** 服务端开关关闭 **THEN THE SYSTEM SHALL** 降级为分段轮询且不报错、不白屏 | P0 |
| FLT-24 | 弹幕屏蔽与展示设置 | **THE SYSTEM SHALL** 支持关键词/发送者屏蔽与服务端账号级生效（继承 DM-20、DM-21），并提供不透明度/字号/显示区域/速度设置（继承 DM-12） | P1 |
| FLT-25 | 弹幕列表面板 | **THE SYSTEM SHALL** 提供按时间排序的弹幕列表并支持点击跳转进度（继承 DM-16） | P2 |

### 2.4 互动与评论（M5）

| ID | 需求 | 验收标准（EARS） | 优先级 |
| --- | --- | --- | --- |
| FLT-30 | 点赞/投币/收藏/三连 | **THE SYSTEM SHALL** 支持点赞、投币、收藏与长按三连（继承 [interaction spec](/specs/interaction/spec) ITR-01、ITR-10、ITR-20、ITR-30）；**端侧差异**：三连 **SHALL** 以原生动画反馈，且 **IF** 请求重复提交 **THEN THE SYSTEM SHALL** 依赖幂等键保证只生效一次（承 ITR §3） | P0 |
| FLT-31 | 评论 | **THE SYSTEM SHALL** 支持一级评论与二级回复、排序切换、分页与展开（继承 CMT-01、CMT-02、CMT-03），作者与 UP 主可删除（继承 CMT-04） | P1 |
| FLT-32 | 分享 | **THE SYSTEM SHALL** 通过系统分享面板分享视频链接（继承 SHR-02 站外分享）；**端侧差异**：**THE SYSTEM SHALL NOT** 依赖微信小程序卡片（Android/iOS 走系统分享目标） | P2 |

### 2.5 发现与个人中心（M5）

| ID | 需求 | 验收标准（EARS） | 优先级 |
| --- | --- | --- | --- |
| FLT-40 | 首页与分区 | **THE SYSTEM SHALL** 提供首页信息流与分区导航、最新/最热切换（继承 [search-recommend spec](/specs/search-recommend/spec) REC-01 与 [video spec](/specs/video/spec) VID-20）；**端侧差异**：列表 **SHALL** 使用惰性加载，滚动不掉帧 | P0 |
| FLT-41 | 搜索 | **THE SYSTEM SHALL** 支持关键词搜索视频与 UP 主（继承 SRH-01、SRH-02 排序筛选）；**端侧差异**：搜索历史 **SHALL** 与云端同步（承 SRH-04） | P1 |
| FLT-42 | 个人中心/历史/收藏 | **THE SYSTEM SHALL** 展示个人资料、我的投稿、观看历史与收藏（继承 ACC-12、ITR-21），**WHEN** 未登录 **THE SYSTEM SHALL** 展示登录入口 | P1 |

### 2.6 创作（M5 二期，本期不交付）

> 端侧定位自 V1.3 起包含创作，但 Flutter 端**首期不承接创作**：投稿链路（分片上传、后台保活、封面裁切）在 Android/iOS 上的差异面比观看端大，安排在观看端 MVP 收口后二期评估。本节仅登记范围边界，**不产生本期验收项**。

## 3. 边界与异常

- **IF** 后端返回业务错误码（HTTP 200 + `code≠0`）**THEN THE SYSTEM SHALL** 按业务码判定成败，**SHALL NOT** 仅以 HTTP 状态码判定（承本项目既有口径，误用状态码会把「被正确拦下」误判为缺陷）
- **IF** 播放地址签名过期（`e=` 已过）**THEN THE SYSTEM SHALL** 静默换签重试，**SHALL NOT** 向用户抛出 403 原文
- **IF** 设备为 x86_64 模拟器且所用 Flutter 版本已移除 x86 支持 **THEN THE SYSTEM SHALL** 在开发文档中登记「改用 ARM 真机或 arm64 AVD 验收」，**SHALL NOT** 把模拟器不可用当作功能缺陷
- **IF** 运行环境为 Windows **THEN THE SYSTEM SHALL** 明确 iOS **不可**本地构建/签名，**SHALL NOT** 声称 iOS 已验证
- **WHEN** 用户未登录访问需登录的页面 **THE SYSTEM SHALL** 引导登录并在登录后回跳原页面，**SHALL NOT** 静默丢弃用户操作意图

## 4. 成功指标

| 指标 | 目标 |
| --- | --- |
| 冷启动耗时（Android，真机中位数） | < 2s |
| 起播时间 P90（真机） | < 1.5s |
| 弹幕满载渲染帧率 | ≥ 55fps |
| 播放卡顿率 | < 1% |
| 安装包体积（arm64 单架构 release） | < 30MB |
| 崩溃率（验收窗口内） | < 0.5% |
| 双端代码共享率 | ≥ 90%（仅平台通道与少量 UI 适配允许分叉） |

> 与 [harmony spec §4](/specs/harmony/spec) 保持一致的产品阈值；**模拟器结论不得外推真机**（承鸿蒙端既有纪律）。

## 5. 依赖与关联

- 依赖：[account spec](/specs/account/spec)（ACC-01/03/06/12）、[video spec](/specs/video/spec)（PLY-01~08、VID-20）、[danmaku spec](/specs/danmaku/spec)（DM-01~21）、[interaction spec](/specs/interaction/spec)（ITR-01/10/20/21/30）、[search-recommend spec](/specs/search-recommend/spec)（REC-01、SRH-01/02/04）
- 被依赖：无（端侧为消费方）
- 服务端配合项（**均为既有能力，无需新接口**）：
  - `app.allowNativeClients`（Flutter WS 握手放行，见 [deployment §5.6](/project/deployment)）
  - `playsign` 签名播放地址（FLT-14）
  - 统一 envelope 与业务错误码
- 全局约束：[非功能需求](/product/nfr) 中性能、安全、兼容性条目
