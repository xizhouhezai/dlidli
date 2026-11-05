# plan：Flutter 移动端（Android + iOS）

> 模块标识：`flutter` ｜ 对应需求：[spec](/specs/flutter/spec) ｜ 任务：[tasks](/specs/flutter/tasks)
>
> 变更记录：
> - 2026-11-05 V1.0 初稿：工程形态 / 状态管理 / 播放器 / 弹幕 / 网络 / 存储 / 构建与验收环境七项技术决策，附本机环境实测结论

## 1. 环境实测（本机，2026-11-05）

| 项 | 实测值 | 结论 |
| --- | --- | --- |
| Flutter | 3.29.0 stable / Dart 3.7.0（`D:\flutter`） | ✅ |
| Android SDK | API 35 / build-tools 35.0.1 / NDK 26.3 | ✅ |
| JDK | PATH 为 JDK17；Android Studio 自带 JDK21 | ✅ |
| 真构建 | `flutter build apk --debug` → **成功，121.9s，83.6MB** | ✅ 全链路可编译 |
| 产物 ABI | `arm64-v8a / armeabi-v7a / x86 / x86_64` | ⚠ 含 x86，但见下方风险 |
| 依赖镜像 | `PUB_HOSTED_URL`/`FLUTTER_STORAGE_BASE_URL` 已指国内镜像 | ✅ |
| 模拟器 | MuMu Player 12，实例 index=**1**，`adb_port=16416`（**动态**），Android 15 (API 35)，**x86_64**，root=true | ✅ 已实测连上 |
| 端到端 | probe APK 安装并启动 → 截图确认 Flutter 界面渲染（`Flutter Demo Home Page`） | ✅ **Flutter 能在 MuMu 跑** |
| 后端连通 | 用 Dart 直连 `:8000`：`/health` ready、`admin/login` 拿 token、带 Bearer 调 `admin/me/permissions`（19 菜单）、取 `streams[].url` 签名地址并 HEAD **200** | ✅ **API 可直接复用，无需新接口** |
| iOS | **无 Xcode / 无 CocoaPods / 本机 Windows** | ❌ 不可本地构建 |
| Desktop | `flutter doctor`：Visual Studio 缺失 | 不影响本需求（不做 Windows 桌面端） |

## 2. 技术决策

### 2.1 工程形态：单工程双平台

`apps/flutter_app`（单 Flutter 工程，`flutter create --platforms=android,ios`）。

理由：Flutter 的定位就是一套代码两端产物；拆成两个工程只会让 UI/逻辑双写，与选型初衷相悖。实测 Windows 上 `flutter create --platforms=android,ios` 能正常生成 iOS 工程目录（只是不能编译）。

**不与鸿蒙共享代码**：鸿蒙是 ArkTS/ArkUI，Flutter 无法复用；共享的是后端契约与产品口径（见 spec §1）。

### 2.2 状态管理：Riverpod

选 Riverpod（而非 Provider/Bloc）：编译期安全的依赖注入 + 天然的异步状态（`AsyncValue`）表达，正好覆盖本项目大量「加载中/错误/空态」的三态渲染；测试可脱离 Widget 树。

### 2.3 网络层：dio + 手写 envelope 解析

- `dio`：拦截器做 token 注入、401 单飞续期重放（对齐 Web/H5/Harmony 的既有语义）。
- 统一 envelope `{code,message,data,trace_id}`：**按 `code` 判成败，不按 HTTP 状态**（承 spec §3）。业务失败可能是 HTTP 200 + `code≠0`。
- 端基址差异：Android 模拟器用 `10.0.2.2`，iOS 模拟器用 `localhost`，真机走配置注入（`--dart-define`）。

### 2.4 播放器：`media_kit`（备选 `video_player`）

- 首选 `media_kit`：底层 libmpv，HLS 支持好、可控制缓冲与倍速、跨 Android/iOS 行为一致。
- 兜底 `video_player`（官方插件，ExoPlayer/AVPlayer）：若 `media_kit` 在 iOS 侧包体或稳定性不达标则回退。
- **决策延后到 T-10 完成**（双端各跑一次同源片源后再定），避免拍脑袋。

### 2.5 弹幕渲染：`CustomPainter` 自绘轨道

参照鸿蒙端 Canvas 轨道渲染口径（同一套轨道分配/防重叠算法），用 `CustomPainter` + `Ticker` 驱动；目标同屏满载 ≥ 55fps（spec §4）。不做 DOM/Widget 级弹幕（性能不可控）。

### 2.6 本地存储

- 凭证：`flutter_secure_storage`（Android Keystore / iOS Keychain），满足 FLT-02。
- 设置/进度缓存：`shared_preferences` + `sqflite`（进度与历史量级需要结构化存储）。

### 2.7 构建与验收

| 场景 | 做法 |
| --- | --- |
| Android 开发/验收 | MuMu（**x86_64**，端口动态，用 `scripts/mumu-adb-connect.ps1` 连接） |
| Android 真机 | `flutter build apk --release --target-platform android-arm64` |
| iOS | **本机不可编译**：需 macOS（Xcode + CocoaPods）或 CI（macOS runner）。本机仅能 `flutter analyze` 与 Dart 层单测 |
| 合规 | iOS 需 Apple 开发者账号与设备签名；Android 上架需签名 AAB |

## 3. 风险与对策

| 风险 | 等级 | 说明与对策 |
| --- | --- | --- |
| **MuMu 为 x86_64，Flutter 3.27 后宣布移除 x86** | **高** | 实测 3.29 仍带 x86 且能跑，但构建时官方已警告。**对策**：① 开发期可用 MuMu；② 验收与发布必须用 **arm64 真机或 arm64 AVD**；③ 锁 Flutter 版本，升级前先验 arm64 路径；④ 不把「MuMu 跑不起来」当功能缺陷 |
| **iOS 在 Windows 上无法构建** | **高** | 事实约束，不掩饰。对策：iOS 验收登记为「需 macOS/CI」，本机只保证代码同源与 `analyze` 通过；上架另立任务 |
| adb 连不上 MuMu | 中 | 端口动态 + 两套 adb（SDK 35.0.2 / MuMu 36.0.0）互踢。已固化 `scripts/mumu-adb-connect.ps1`（RPC 取端口、单 adb、`-Restart` 自救），并实测连上 |
| 弹幕 WS 握手 Origin | 中 | 服务端已有 `app.allowNativeClients`（native 例外判据为**有效 token**，默认 fail-closed）。Flutter 端须在握手带 token（FLT-23）；开关未开时降级轮询 |
| 播放器双端行为不一致 | 中 | `media_kit` 兜底 `video_player`；T-10 双端同源片源对照后再定 |
| 包体超 30MB | 中 | arm64 单架构 + `--split-per-abi` + 资源裁剪；`media_kit` 若引入 libmpv 需评估体积 |

## 4. 里程碑与任务拆解原则

- 一期（观看端 MVP）：T-01 骨架与基础设施 → T-10 播放 → T-20 弹幕 → T-30 互动 → T-40 发现/个人中心 → T-90 端侧验收。
- 二期（创作端）：待一期收口后评估投稿链路在双端的差异面，不在一期承诺。
- 每个任务须带「覆盖需求 ID」与可复跑证据（沿用本项目既有纪律：不读自述，用真实接口/截图/日志取证）。
