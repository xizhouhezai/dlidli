# DliDli Flutter 移动端（Android + iOS）

> 规格：[spec](../../docs/specs/flutter/spec.md) · [plan](../../docs/specs/flutter/plan.md) · [tasks](../../docs/specs/flutter/tasks.md)
> 当前阶段：**M5 一期 T-01 工程骨架 + T-02 网络层**

## 快速开始

```powershell
# 1) 后端（MySQL :3307 + Redis :6379 + API :8000）
cd ../../server && go run ./cmd/migrate && go run ./cmd/api

# 2) 连 MuMu 模拟器（端口动态，用脚本而非写死）
powershell -ExecutionPolicy Bypass -File ../../scripts/mumu-adb-connect.ps1

# 3) 跑起来
flutter pub get
flutter run -d 127.0.0.1:16416        # 端口以脚本输出为准
```

**后端地址**：Android 模拟器（MuMu/AVD）用 `10.0.2.2:8000`（宿主映射），iOS 模拟器用 `localhost:8000`；
真机用注入：`flutter run --dart-define=DLIDLI_API_BASE=http://<局域网IP>:8000`。

## 目录结构

```
lib/
├── main.dart                    # 装配入口（ProviderScope + MaterialApp）
├── core/                        # 端侧基础设施（与业务无关）
│   ├── env.dart                 # 环境与后端基址（平台差异在这里收口）
│   ├── api_client.dart          # 统一信封解析 + 按 code 判成败 + 401 单飞续期
│   ├── api_error.dart           # ApiError 分类与错误码→文案映射
│   ├── token_store.dart         # 令牌存储抽象（测试可注入内存实现）
│   ├── secure_token_store.dart  # Keystore/Keychain 实现（FLT-02）
│   └── url_rewriter.dart        # 后端下发 localhost 地址的端侧重写
├── data/
│   ├── api/video_api.dart       # 视频接口（复用既有后端契约）
│   └── models/video.dart        # 模型（含 streams[] 真实契约）
├── app/providers.dart           # Riverpod 装配 + 首页分页状态
└── features/feed/feed_page.dart # 首页信息流 + 网络自检页
```

## 三条容易踩的坑（都已处理，勿删）

1. **Android 必须声明 `INTERNET` 权限** —— Flutter 模板默认不含，缺失时所有请求静默失败。
2. **Android 9+ 默认禁止明文 HTTP** —— dev 后端是 `http://10.0.2.2:8000`，
   已在 `android/app/src/main/res/xml/network_security_config.xml` 里**只对本地地址**放开，
   生产域名仍强制 HTTPS。
3. **后端下发的资源地址是服务端视角的 `localhost`**（封面/播放地址），在端机不可路由 →
   必须走 `UrlRewriter.rewrite` 重写主机；**且不能重建 Uri**（会重排 query 导致播放签名失效）。

## 校验

```powershell
dart format lib test     # 格式统一（期望输出 0 changed）
flutter analyze          # 必须 0 issue（闸门）
flutter test             # 59 个用例：信封/错误分类/401 单飞/URL 重写/鉴权/token 对齐/播放/手势
flutter build apk --debug
```

## 未覆盖（如实登记）

- **iOS 本机不可构建**（Windows 无 Xcode/CocoaPods）：本仓 iOS 部分只保证代码同源与 `analyze` 通过。
- **MuMu 为 x86_64**：Flutter 3.27 后官方宣布移除 x86 支持，发布验收须 arm64 真机。
- 本期未做：登录页 UI、播放页、弹幕、互动（T-03 起逐步补齐）。
