# MuMu 模拟器 adb 连接故障手册

> 适用：本机 MuMu Player 12（实例 `MuMuPlayer-15.0-1`）+ Flutter/Android 开发
> 配套脚本：`scripts/mumu-adb-connect.ps1`
> 实测日期：2026-11-05

## 0. 一句话结论

**「有时连得上、有时连不上」几乎不是 USB 调试开关的问题，而是这三件事之一：**
① 端口每次启动会变（不能写死）；② 本机两套 adb 版本不同会互相 kill server；③ 实例没真正启动完成。
脚本已把这三点全部处理掉。

## 1. 正确姿势（推荐）

```powershell
# 仓库根，连接并等待 device ready（自动从 MuMu RPC 读端口）
powershell -ExecutionPolicy Bypass -File scripts/mumu-adb-connect.ps1

# 只看当前状态
powershell -ExecutionPolicy Bypass -File scripts/mumu-adb-connect.ps1 -Status

# 卡住时的第一招（重启 adb server 再连）
powershell -ExecutionPolicy Bypass -File scripts/mumu-adb-connect.ps1 -Restart

# 多开时指定实例（实例下标从 1 开始，不是 0）
powershell -ExecutionPolicy Bypass -File scripts/mumu-adb-connect.ps1 -Index 2
```

成功后 Flutter 直接可见：

```
flutter devices
flutter run -d 127.0.0.1:16416
```

## 2. 三个坑的机理与判据

### 坑 1：adb 端口是动态的（头号原因）

MuMu 用 NAT 端口映射。**权威来源不是配置文件，而是 MuMuManager 的 RPC**：

```powershell
& "D:\Program Files\Netease\MuMu Player 12\nx_main\MuMuManager.exe" info -v all
```

关键字段：`adb_port`（本次 16416）、`adb_host_ip`、`player_state`（须为 `start_finished`）、`is_android_started`。

配置文件 `vms/<实例>/configs/vm_config.json` 里的 `vm.nat.port_forward.adb.host_port` **会滞后**，读它会得到过期端口。

> 另注意：**实例下标从 1 开始**。`info -v 0` 返回 `player index not found`，容易误判成「实例不存在」。

### 坑 2：两套 adb 互踢

本机同时存在：

| adb | 版本 | 路径 |
| --- | --- | --- |
| Android SDK | 35.0.2 | `%LOCALAPPDATA%\Android\Sdk\platform-tools\adb.exe` |
| MuMu 自带 | 36.0.0 | `D:\Program Files\Netease\MuMu Player 12\nx_main\adb.exe` |

两者都抢 5037 端口的 adb server，**版本不一致时会互相 kill 对方 server** —— 典型表现：
「刚才 `adb devices` 还有设备，跑了个别的东西后突然空了」。

**对策**：全程只用一套（脚本默认用 SDK 的）；已混用时先 `-Restart`。
必要时关掉 MuMu 那套：`"...\nx_main\adb.exe" kill-server`。

### 坑 3：端口是 Established 不是 Listen

`Get-NetTCPConnection -LocalPort 16416 -State Listen` 查不到端口 **不代表实例没起来**：
adbd 通道建立后，监听套接字被已接受的连接占用，状态会显示 `Established`。

**别用端口 Listen 判断实例是否存活**，用 `player_state` 或直接 `adb connect`。

## 3. 分场景处置表

| 现象 | 判据 | 处置 |
| --- | --- | --- |
| `adb devices` 空 | `MuMuManager info -v all` 的 `player_state` 不是 `start_finished` | 先启动实例，等桌面出现再连 |
| 刚还能连，突然空 | 近期是否调用过另一套 adb / 其他工具（如某些 IDE） | 只保留一套 adb；`-Restart` |
| `connect` 成功但 `offline` | 实例正在启动或 adbd 卡住 | 等 10~30s；仍不行重启实例 |
| 端口变了 | RPC 里 `adb_port` 与手头写的不同 | 用脚本（每次现读），不要写死 |
| `unauthorized` | 设备未授权本机 | 重启 adbd：`adb kill-server` 后重连；模拟器一般直连 `device` |
| MuMu 官方通道报 `vm not running` | 同上 | 启动实例；`MuMuManager launch -v 1` |

## 4. 已验证的事实（本机实测）

```
MuMuManager info -v all
  index=1  name=MuMu安卓设备-1  adb_port=16416  player_state=start_finished
  android_version=15.0  vt_enabled=true  hyperv_enabled=true

脚本执行结果
  实例     : index=1  MuMu安卓设备-1
  adb 目标 : 127.0.0.1:16416
  connect  : already connected
  [OK] 已连接
       机型 PGAM10 / Android 15 (API 35) / ABI x86_64 / 1080x1920

Flutter 侧
  flutter devices → PGAM10 (mobile) • 127.0.0.1:16416 • android-x64 • Android 15 (API 35)

端到端
  flutter build apk --debug → 成功（121.9s / 83.6MB）
  adb install -r app-debug.apk → Success
  启动后截图确认 Flutter 界面渲染（Flutter Demo Home Page）
```

## 5. 重要风险提示（Flutter 侧）

MuMu 是 **x86_64** 模拟器。当前 Flutter 3.29 的构建产物仍含 `x86_64` 且实测可跑，
但 `flutter build apk` 已明确警告：

> Support for Android x86 targets will be removed in the next stable release after 3.27.

**含义**：升级 Flutter stable 后，MuMu 可能直接装不上/跑不起来。
**因此**：开发期可用 MuMu，但**发布验收必须用 arm64 真机或 arm64 AVD**；升级 Flutter 前先验证 arm64 路径。
