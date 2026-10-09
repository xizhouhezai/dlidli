# tasks：原生鸿蒙端（HarmonyOS NEXT）

> 对应规格：[spec](/specs/harmony/spec) ｜ 方案：[plan](/specs/harmony/plan)
> 任务编号：`{阶段}-{模块}-{序号}`（如 M4-HMY-03）；完成即勾选并追加完成日期；括号补注实现要点与验证结论；每条任务必须标注"覆盖"的需求 ID。
> 口径：本期仅模拟器/真机本地验证，**不提交平台审核**（对齐 [M3-MP-03](/specs/engineering/tasks) 小程序的个人主体口径）。

## M4（W49+）观看端

- [x] M4-HMY-01 预研：原生技术链路可行性（AVPlayer 播后端签名 m3u8；`@ohos.net.webSocket` 连弹幕 WS；`@ohos.net.http` 跑通登录与视频列表；ArkUI 状态管理 V2 写法；沉浸光感在 API 26 模拟器上的实际表现）（2026-10-08 完成）
  - 覆盖：—（工程；为 HMY-01/03/10/20/22 定实现方案）
  - 前置：DevEco 26.0.0.821 + SDK API 26 + 模拟器镜像 API 26（`D:/Program Files/Huawei/sdk/system-image/HarmonyOS-7.0.0/phone_all_x86`）已就位，实例名 **`Pura X View`**（apiVersion 26.0.0 / guest 7.0.0.106）。模拟器视频硬解受限时播放结论记「待真机确认」，**不得据此判定鸿蒙不支持 HLS**
  - 连设备口径（2026-09-21 实测）：模拟器**不注册为 USB 目标**，只跑 `hdc list targets` 恒为 `[Empty]`；须先 `hdc tconn 127.0.0.1:5555`（模拟器监听 `127.0.0.1:5555`），连上后 `list targets -v` 由 `TCP Offline` 转 Online；未就绪时 shell/install 报 `[E001005] Device not found or connected`。另行拉起模拟器的命令行可从 DevEco `idea.log` 的 `LocalDeviceConnection - start hvd log` 抄取：`Emulator.exe -hvd "<实例名>" -path "D:/Program Files/Huawei/Emulator/deployed" -t <pipe> -imageRoot "D:/Program Files/Huawei/sdk"`；`hdc file recv` 的本地路径须写 Windows 反斜杠形式并在 Git Bash 下关掉路径转换，否则被当相对路径
  - **启动窗口期注意事项（2026-09-21 实测）**：模拟器冷启后需等 **SystemUI（`com.ohos.sceneboard` 进 FOREGROUND / 窗口数 > 0）** 才算就绪；就绪前 `hidumper -s WindowManagerService` 窗口数为 0、`snapshot_display` 恒返回同一张 47KB 全黑帧、`aa start` 报 `10106102 … device screen is locked … developer mode`——这些都是**未就绪的假象，不是宿主渲染故障**。当日早前 3 次尝试即在启动窗口期（2~4 分钟内）操作并遇 VM 退出，一度误判为宿主 OpenGL 不可用；当晚重试则**完全可用**（安装/启动/渲染/切页签均正常）。操作口径：先 `uptime` + `aa dump -a` 确认 sceneboard 就绪，再安装与启动；避免在启动窗口期反复 `snapshot_display`
  - 可用结论（同期实测）：模拟器运行时 **API 26 / guest `7.0.0.106(SP1DEVC00E999R4P11)` / abi `x86_64`**；**未签名 HAP 可直接 `hdc install` 成功**，本地验证无需配置 `signingConfigs`
  - 模拟器操作速查：安装 `hdc -t 127.0.0.1:5555 install -r <hap>`（`<hap>` 务必用**相对路径**：先 `cd` 到产物目录；Git Bash 下传 Windows 绝对路径会被 hdc 拼成 `<当前目录>/D:/...` 而报 `Error opening file`，安装失败后紧接着的 `aa start` 就报 `10104001`，极易误判成包名问题）；启动 `hdc -t … shell "aa start -a EntryAbility -b com.dlidli.app -m entry"`；点击用 `hdc -t … shell "uitest uiInput click <x> <y>"`（`uinput -T -m x y x y` 等长 trace 不触发点击）；取控件实际矩形用 `uitest dumpLayout -p /data/local/tmp/layout.json` 再 `hdc file recv`（本次即以它纠正了 1719→2064 的坐标误判）；截图 `snapshot_display -f <路径>` + `file recv`
  - 视觉预研要点（降级为实测确认，设计侧结论已定稿见 [plan §2/§6](/specs/harmony/plan)）：① `uiMaterial.isImmersiveMaterialSupported()` 在 x86 模拟器返回什么；② `getMaterialInfo()`/`getGlobalMaterialLevel()` 的实际取值（x86 模拟器算力档位可能非高/中档，`style`/`colorInvert` 等可能不生效）；③ 在 `Navigation` 标题栏与 `Tabs` 底部标签栏上挂 `ImmersiveMaterial` 的实际观感与帧率开销；④ 结论若为模拟器不支持，**不得据此判定真机不支持**，记「待真机确认」
  - **预研进度（2026-09-21 起，2026-09-22 补记）**：预研项中的**网络层已随 M4-HMY-03 实测覆盖**——`@ohos.net.http` 走通真实后端（首页分区列表 12 项）、超时/失败重试与 401 续期链路均已验，且模拟器可达宿主机（`10.0.2.2:8000`），结论见 M4-HMY-03；**AVPlayer HLS 播放已随 M4-HMY-06 实测覆盖**（后端签名 m3u8 真实解码出画 + 清晰度切换 + 签名临期换源续播 + 横屏全屏，结论见 M4-HMY-06）；**沉浸光感已随 M4-HMY-11 实测覆盖**（`supported=true`/`state=ENABLE`/`level=EXQUISITE`，结论见 M4-HMY-11 与 [plan §6](/specs/harmony/plan)）；**弹幕 WS 通道已随 M4-HMY-07 实测覆盖**（`@ohos.net.webSocket` 与 Go Hub 互通无碍，唯一阻塞是服务端 Origin 白名单未含客户端来源，端侧无从绕过；降级轮询链路与真实帧上屏均已验，结论见 M4-HMY-07）
  - **材质叠加的帧率/功耗实测（2026-10-08 完成，末项收口）**：见下条「材质帧率实测」；**至此预研五项全部收口**
  - **材质帧率实测（2026-10-08，API 26 模拟器 `Pura X View`，app 零改动）**
    - **可复现测量方法（纯外部采集，不依赖应用埋点）**：应用进程每收到一次 UI vsync 会写一条 trace 标记，故用 `hitrace` 抓帧数即可反推真实出帧率——
      ```sh
      PID=$(pidof com.dlidli.app)
      hitrace -t 10 -b 65536 -o /data/local/tmp/perf.ftrace graphic ace animation
      grep "B|$PID|H:OnVsyncEvent" /data/local/tmp/perf.ftrace | sed -n 's/.*now:\([0-9]*\).*/\1/p' > frames.txt
      # 帧率 = (行数-1) / (末条 now - 首条 now) 秒；条目自带纳秒时间戳，宿主机侧算分位数
      ```
      同一份 trace 里另可取 RenderService（pid 800）侧计数作交叉印证：`RSUniRenderThread::Render`（出图次数）、`RSSurfaceRenderNodeDrawable::OnDraw`（节点绘制）、**`wouldDrawLargeAreaBlur`（大面积模糊绘制，即沉浸光感的模糊路径）**、`RSFilterCacheManager::ClearFilterCache`。
      **两条必须遵守的口径**：① **不能用「固定窗口内数帧数」**——实测**静止时该标记为 0**（系统按需出帧，无新帧就不画），必须用「帧数 ÷ 首末时间戳跨度」；② **`hidumper -s RenderService -a fpsCount` 在模拟器上恒返回 `Refresh Rate:60, Count:1`（不随滚动变化），`SP_daemon -f` 与 `SP_daemon -ohtestfps` 恒返回 `fps=0`**——本机这两个口径都取不到值，**只有 hitrace 这条路径有效**（二者为何为空值本轮未深究，未取到即不用）。
    - **载荷与取值（首页信息流，材质面最全：卡片 `REGULAR` + 搜索胶囊 `THIN` + 标题栏 `ULTRA_THIN` + HDS 悬浮胶囊底栏）**：`uitest uiInput drag 660 1700 660 400 220` 用**一次持续 drag** 覆盖整段采集窗口（不能用 `fling` 循环——手势之间的空档会把均值拉低）：
      | 场景 | 帧率（逐轮） | 均值 | p50 帧间隔 | p90 | >33ms |
      | --- | --- | --- | --- | --- | --- |
      | 滚动 · 材质 `enable` | 61.7 / 60.2 / 60.0 | **60.6** | 16.00ms | 16.00ms | 0 / 1 / 3 |
      | 滚动 · 材质 `disable`（同机同载荷对照） | 61.4 / 61.5 / 60.4 | **61.1** | 16.00ms | 16.00ms | 1 / 1 / 1 |
      | 静止（无输入） | 0 帧 | — | — | — | — |
      → **滚动场景两侧都顶在 60Hz vsync 上限，材质开关测不出帧率差**（均值差 0.5fps，小于轮间抖动 ±0.85fps）。
    - **开销在哪：模糊路径绘制次数翻倍（但被模拟器余量吸收）**。同一滚动载荷、同一采集窗口下比对 RenderService 侧计数：
      | | `RS_RENDER` | `RS_NODE_DRAW` | **`wouldDrawLargeAreaBlur`** | `RS_FILTER_CLEAR` |
      | --- | --- | --- | --- | --- |
      | 材质 `enable` | 618 / 276 / 228 | 927 / 414 / 342 | **1664 / 720 / 608** | — |
      | 材质 `disable` | 390 / 258 / 324 | 585 / 387 / 486 | **528 / 336 / 432** | — |
      → 同窗口同载荷下，模糊路径绘制次数 **材质开 = 1664 / 720 / 608**，**材质关 = 528 / 336 / 432**（按轮次对应），逐轮比值 **3.15× / 2.14× / 1.41×**。**即材质开销是真实存在的（模糊绘制次数显著增加，最低一轮也有 1.4×），只是模拟器在 60Hz 上限下还有余量把它吸收了**——这正是不该把「模拟器不掉帧」外推为「真机不掉帧」的原因。
    - **进程 CPU 不可作判据**：`hidumper --cpuusage <pid>` 单次采样在两侧分别是 3.35% / 4.24%，轮间抖动大于差异，**本轮未取得可信的 CPU 功耗对比**（`SP_daemon -p` 在模拟器直接报 `RK does not support power acquisition`，电池是虚拟的 100%/充电中，**功耗在模拟器上根本不可测**）。
    - **弹幕层逐帧重绘 + 材质叠加（播放页）**：`DanmakuLayer` 以 `setInterval(…, 16)` 逐帧驱动 Canvas 重绘，与视频画面同屏。实测播放页**恒停在片源帧率上，与弹幕无关**：
      | 场景 | 帧率 | p50 帧间隔 |
      | --- | --- | --- |
      | 弹幕开（片源自带 45 条） | 31.1 / 31.3 / 31.4 | **32.00ms** |
      | 弹幕关（同片源、同机对照） | 30.8 / 31.3 / 33.1 | **32.00ms** |
      | 弹幕开（**灌入 1000 条满屏**，Redis 段缓存已清） | 29.6 / 30.3 / 32.3（另有一轮 56.6 为部分窗口，不计） | **32.00ms** |
      → **p50 恰好 32.00ms = 1/30s，即 30fps 片源的帧间隔**；`ffprobe` 证实该 HLS 片源 `r_frame_rate=30/1`。**同屏弹幕 45 条与 1000 条、开与关，播放页帧率都锁在 30fps**，说明该窗口内**弹幕 Canvas 逐帧重绘不是瓶颈**（每帧先 `clearRect` 再按活动条数描边+填充，活动数受 `MAX_ACTIVE=240` 与轨道数封顶）。
    - **测量方法上的两个坑（备查，避免重走）**：① **不能用 5 秒的短测试片做弹幕 A/B**——片源 5s 播完后页面不再出帧（`帧数=0`，会被误读成「卡死」），必须挑 `duration` 足够长的片源，或每轮重新进页面并在播放窗口内采集；② 抓到的 `frames.txt` **必须在每轮结束时立刻改名留存**，否则后一轮会覆盖前一轮证据（本轮早期即因此得到过两组完全相同的假数据，已作废不计）。
    - **结论（模拟器可得 vs 须真机确认）**：
      - **模拟器可得（已收口）**：① 帧率采集方法跑通且可复现（hitrace + `OnVsyncEvent`，附 RenderService 侧模糊路径计数）；② **材质叠加在 API 26 模拟器上不导致掉帧**——首页滚动顶满 60Hz、播放页锁在片源 30fps，材质开/关与弹幕 45/1000 条均无帧率差；③ 材质开销的**相对量**可测：同一滚动载荷下模糊路径绘制次数为关闭时的 **1.41×~3.15×**（逐轮 3.15 / 2.14 / 1.41）。
      - **须真机确认（不得由模拟器外推）**：① 真机上材质模糊的**绝对开销与掉帧风险**——模拟器是 **x86_64 软件渲染**（`SP_daemon -deviceinfo` = `hmos.emulator` / `abilist x86_64`，走宿主 Intel Arc 转译，`GL_RENDERER` 报 `Mali-G77` 只是字符串），GPU 负载与真机完全不同；② **功耗/发热**——模拟器无功耗采集能力（`SP_daemon -p` 报不支持），电池为虚拟值，**功耗结论在模拟器上不可得，必测真机**；③ **spec §4 的「同屏弹幕满载 ≥ 55fps」**：本轮因片源 30fps 上限无法验证该阈值（要验须用 ≥60fps 片源 + 真机）；④ 长时播放（>5min）下的稳定性与内存增长，本轮仅在秒级窗口内采样。
- [x] M4-HMY-02 工程骨架：DevEco 工程入库 `apps/harmony` + `pnpm-workspace.yaml` 排除该目录 + 分层目录 + 品牌色/圆角 ArkTS 常量 + HarmonyOS Symbol 图标接入 + 接口类型来源定案 + `compatibleSdkVersion` 取 26 + **应用壳层搭在 `Navigation` + `Tabs(BottomTabBarStyle)` 上**（当时认为底栏是沉浸光感的唯一合法作用面）+ `module.json5` 配 `ohos.arkui.UIMaterial.state`（2026-09-21 完成；**该壳层已于 2026-09-22 由 M4-HMY-11 改造为 `HdsTabs` 悬浮胶囊底栏，见 [plan §2/§6](/specs/harmony/plan)**）
  - 覆盖：—（工程）
  - 实现要点：`bundleName` `com.dlidli.app` / `vendor` `DliDli`；`targetSdkVersion` 与 `compatibleSdkVersion` 均 `26.0.0`（产物 `targetAPIVersion` = `minAPIVersion` = `260000026`）。壳层 `pages/Index.ets` = `Navigation`（标题栏材质 `ImmersiveStyle.ULTRA_THIN` + `interactive`）+ `Tabs(barPosition: BarPosition.End)`（`.barFloatingStyle()` 挂 `THIN` 材质 + `maskColor`/`maskHeight` 蒙层；**2026-09-22 起改 `HdsTabs` 悬浮胶囊**），首页标题栏内嵌搜索入口（对齐官方「一镜到底」搜索）；三页签 首页/搜索/我的（HMY-40/41/42）。材质统一经 `common/constants/MaterialTokens.ets` 工厂产出，内部以 `uiMaterial.isImmersiveMaterialSupported()` 判支持性，不支持时返回 `undefined` **自然降级**，`DliMaterial.off()` 暴露 `Material.empty` 语义（与传 `undefined` 的「恢复默认」区分）。品牌 token 落两处：`common/constants/Theme.ets`（字面量，供 Canvas 绘制消费）+ `resources/{base,dark}/element/color.json`（含深色变体，供声明式 UI 消费）；新增字符串资源与 `common/utils/Logger.ets`（hilog 封装）
  - 验证结论：`hvigorw --no-daemon assembleHap` **BUILD SUCCESSFUL**；产物 HAP 内 `module.json` 已含 `ohos.arkui.UIMaterial.state = enable`；`sys.symbol.{house,magnifyingglass,person}` 通过资源编译；**ArkUI 状态管理 V2（`@Entry @ComponentV2` / `@Local` / `@Param`）编译通过，可定版**（消解 [plan §6](/specs/harmony/plan) 的 V2 待定项）
  - **模拟器实测（2026-09-21，API 26 实例 `Pura X View`）**：未签名 HAP 装入后启动成功，**点检通过**——标题栏（首页为搜索入口 / 其余页签为纯标题）、底部三页签 首页·搜索·我的 的 Symbol 图标与品牌粉选中态、页面占位内容均正确渲染；`uitest uiInput click` 切页签生效（标题栏与内容同步切换）
  - **实测修掉一处**：根内容标题栏多出返回键 → 补 `.hideBackButton(true)`（返回键应由后续压栈的 `NavDestination` 自带）
  - **改 `bundleName` 后的 `10104001` 陷阱（2026-09-21 二次复现并定位到真因）**：现象是 DevEco 点 Run 报 `10104001 The specified ability does not exist / is not installed`，而同一台设备上 `hdc install` + `aa start -a EntryAbility -b com.dlidli.app -m entry` 完全正常——**根因不在代码，在本机两份 DevEco 缓存仍是旧 `com.example.dlidli`**：
    - ① `.idea/.deveco/project.cache.json` 的 `BUNDLE_NAME`
    - ② **`.hvigor/outputs/sync/output.json` 的 `ohos-project.BUNDLE_NAME`**——这才是 DevEco 部署与启动取包名的**真实来源**，且**只在工程首次打开时由 `hvigorw --sync` 生成一次**，此后 DevEco 直接复用（日志可证：17:50 之后每次打开工程都只跑 `assembleHap`，再无 `--sync`），故改 `AppScope/app.json5` 不会自动刷新。DevEco 于是去 `bm dump` / `aa start` 一个从未安装过的包名——`idea.log` 里 `bm dump -n ***.dlidli`、`HapMetadataParse - Result of get bm dump doesn't contain moduleNames`、`WARN OpenHarmonyLaunchTaskExecutor - Some launch tasks failed` 就是这个包名（该日志掩码把前两段替换为 `***`，故 `com.example.dlidli`→`***.dlidli`、`com.dlidli.app`→`***.app`，据此可反查 DevEco 实际用的包名）
    - 修复：① 改回 `project.cache.json`；② 重跑 `hvigorw --sync -p product=default` 让 `output.json` 重生为 `com.dlidli.app`。修后按 DevEco 的部署时序复跑 `aa force-stop` → `bm dump -n` → `aa start` 全通
    - **判别口径**：CLI 装/启正常而 DevEco Run 报 `10104001` → 先查上面两处缓存，不要去改包名或 ability 配置。两处均在本机 `.gitignore` 内（`/.idea`、`/.hvigor`）不入库，**换机或重新克隆后再改包名，必须重跑一次 `--sync`**
  - **应用图标与 favicon（2026-09-28 改版定版）**：弃用 DevEco 模板默认图标，自研品牌图标。分层图标按 HarmonyOS 规范产出：`AppScope/resources/base/media/` 与 `entry/src/main/resources/base/media/` 各一份 `background.png`（满幅渐变、不带圆角，系统负责遮罩）+ `foreground.png`（白色图形、透明底），均 1024×1024；`startIcon.png` 152×152，圆角已烘焙（对齐模板做法）。Web 端 `apps/web/public/favicon.svg` 为自包含矢量产物，衍生 `favicon.ico`（16/32/48，PNG 内嵌）与 `apple-touch-icon.png`（180），接入 `apps/web/index.html` 的 `rel=icon` 与 `theme-color`
    - **图形与配色（2026-09-28）**：图形换成**脉冲播放标记**（播放三角 + 两道向右鼓出的同心弧），取代首版「字母 D 挖内孔」；渐变底改为**三色全部取自仓库既有 token**——`#FFD9E4`（`primary-light`）→ `#FB7299`（`primary`）→ `#E45C84`（文档站暗色 -3）。首版 `#FC8BAB`→`#F2557F` 中 `#F2557F` 仓库内无 token 对应，随改版废弃
    - **前景占比口径修正（2026-09-28）**：改为**较长边占画布 67%**。首版「占画布 67%」实按**高度**取值，得 52.7% 宽 × 64.5% 高——这恰是旧图形为竖直长条时高度=较长边的巧合；新图形包围盒 1.29:1 更宽，沿用高度口径会到 86% 宽、贴到安全区边缘。同时补 `translate` 补偿「弧只向右鼓出」的包围盒偏心，使缩放后包围盒中心恰为 (512,512)（首版实测偏心至 x=532）
    - **简化小尺寸变体（2026-09-28）**：`logo-mark-small.svg` 去外弧、内弧加粗到 78、三角放大，仅用于 16/32 两帧；占比规则与完整标记一致（较长边 686px），故 32→48 跨帧切换不出现图形尺寸跳变
    - **图形再改版：简化鲸（2026-09-28，当日即被下条 H3 取代）**：因「脉冲播放标记」太像通用播放器图标、缺辨识度，改为**白色简化鲸**——钝头低长身 + 沿下颌的嘴缝 + 单条闭合路径的浅 V 尾鳍 + 「水柱接水滴」的喷水；**播放三角改为 evenodd 负空间挖在肚皮上**（位置即鲸的胸鳍），保留播放语义。完整标记包围盒 742×455（1.63:1），仍按较长边 = 67% 取 `scale(0.9245)`，`translate(-540,-412.5)` 把包围盒中心移到 (512,512)（实测归位后 686×421，中心 (512,511.5)）
      - **被否决的几版与原因（备查，省得重走）**：喷水用两笔/三笔直线扇形、或根部相连的实心双股水柱，都会被读成**触须/触角/兔耳**；纯水滴读成**气泡/彩屑**；只有「水柱接住水滴」这一版读成喷泉——**水滴是「水」而不是「触须」的关键线索**。尾鳍用两片椭圆分开叠会读成**螺旋桨**（缺口太深、两叶互不搭接），改成单条闭合路径 + 浅 V 缺口才对。嘴试图做成轮廓上的内切缺口，读成**鸟喙**，改回沿下颌的负空间细缝
      - **小尺寸变体同步**：`logo-mark-small.svg` 另去掉喷水 / 嘴缝 / 眼并把播放三角放大——16px 下喷水只会糊成噪点，能成像的只有「钝头轮廓 + 尾鳍缺口 + 一个大三角」；包围盒 742×267，中心 (540,506.5)
      - **封面同步换标记**：`cover-default.svg` 内联的旧标记换成同形鲸（`scale(0.8356)`，中心落在 (800,450)）；原先与旧标记「声波」呼应的两道粗弧（`stroke-width` 34/30）在鲸上没有对应物，改为 4 道等宽 2px 同心涟漪圈；标记压深椭圆随之从 (800,466)/430×278 收到 (800,452)/440×250
      - **验证结论（2026-09-28 鲸版，脚本产物逐项核对）**：完整标记与简化标记归位后均 686px 宽 = 67.0%、水平中心 512.0；`foreground.png` 透明底白形、`AppScope` 与 `entry` 两份 md5 逐字节相同；`favicon.ico` 三帧仍 PNG 内嵌；Web 首页顶栏（28px）、Web 登录页（64px）、Admin 登录页（44px）与首页/搜索等默认封面均已实渲确认换成鲸。**未覆盖**：Admin 侧栏 24px 与鸿蒙模拟器桌面图标未重跑实渲（与上一版同一套资源链路，风险低）
    - **图形再定版：贴参考的具象鲸 H3（2026-09-28，当日即被下条「生图具象鲸」取代）**：「简化鲸」被判定**太抽象**——用户给了一张写实向卡通鲸参考图（大圆头 / 张开的嘴 / 眼 / 胸鳍 / 大尾鳍 / 小背鳍），据此重画：**大圆头在左、下颌张开成楔形缺口**、圆眼（evenodd 负空间）、**多边形 + 粗圆角描边**构造的**两叶尾鳍**（`stroke-width` 90）与**胸鳍**（66）、顶部**背鳍**；播放三角仍为 evenodd 负空间挖在身体中段。候选 H1/H2/H3 比对后定 **H3 = 胸鳍 + 背鳍**（最贴参考）。原始包围盒 895×627，按较长边 = 67% 取 `scale(0.7665)` → 686px，`translate(-587.5,-455.5)` 归位（实测 67.0%、中心 (512,512)）
      - **尾鳍/胸鳍为什么不用贝塞尔而是「多边形 + 粗圆角描边」**：手写贝塞尔控制不住「叶瓣宽度 vs 缺口深度」，出来总是两根细手指；改成多边形顶点 + `stroke-linejoin:round` 后叶瓣宽度直接由 `stroke-width` 给定。**代价：描边会把凹角顶点往外吃掉**，缺口在几何上必须留足深度——做浅了（G5 版）缺口会被整条吞掉、尾鳍变一块实心三角。此教训已写进 `logo-mark.svg` 注释
      - **嘴缝必须闭合、不能自交**：定版第一稿上下唇边在舌尖处**交叉**（上唇起点 y=458 低于下唇终点 y=456），nonzero 填充在舌尖留下一个**孤立白点**（512 图标与封面实渲可见）；改为唇缘分别收在 (318,468)/(318,444)，用 `A12,12 0 0 1 318,444` 半圆收口，舌尖成圆角、不再自交
      - **包围盒要带留白量**：尾鳍描边会溢出 1024 画布被裁掉（量得 884 而非实际 895），量尺脚本必须渲染在带 `PAD` 的画布上，且 sharp 的 `trimOffset*` 是**负向**的（内容左上角 = `-trimOffsetLeft/Top`）——两处都踩过
      - **小尺寸变体同步**：`logo-mark-small.svg` 去掉眼 / 胸鳍 / 背鳍并把播放三角放大——16/32px 下这几处细节只会糊成一团；保留「钝头轮廓 + 嘴缺口 + 尾鳍缺口 + 一个大三角」。包围盒 895×456，`scale(0.7665)` + `translate(-587.5,-393)`
      - **封面同步换标记**：`cover-default.svg` 内联标记换成同形 H3，渲染宽度定 620px（与旧封面等宽）→ `scale(0.6927)`，中心 (800,450)；涟漪圈与压深椭圆沿用「简化鲸」版（本就与图形无关）
      - **验证结论（2026-09-28 H3 版，逐项实渲）**：完整 / 简化标记归位后均 686px 宽 = 67.0%、中心 512.0；`foreground.png` 透明底白形、`AppScope` 与 `entry` 两份 md5 相同；`favicon.ico` 三帧仍 PNG 内嵌；实渲确认 Web 首页顶栏 28px（含卡片默认封面）、Web 登录 64px、Admin 登录 44px、H5 首页均已换成 H3；舌尖白点已消除。**未覆盖**：Admin 侧栏 24px 与鸿蒙模拟器桌面图标未重跑实渲
    - **图形定版：生图具象鲸（2026-09-28，取代手写矢量 H3；标记层自此为位图。形态于同日二次定版修订为「闭嘴唇 + 头顶喷泉」，见下条；位图源 + 脚本流水线这套口径不变）**：手写的 H3 仍被判定**太抽象**，改为**直接调生图模型出鲸再二次改进**。产出与清理链路：①`ImageGen` 出「圆形胖身 + 张嘴 + 眼 + 短胸鳍 + 厚尾鳍 + 负空间播放三角」的纯白剪影（迭代四次才拿到圆润 + 简体画的版本，前几版偏修长/偏鱼尾）；②清理：生图带右下角「Qoder AI 生成」水印（峰值亮度 149）与轻微灰阶，而鲸体亮度 ~253，故用 **170→232 的亮度对比拉伸做 alpha**——低于 170 全透明（水印/背景/杂色一并去掉）、高于 232 全白，且靠原始抗锯齿边缘自然得到柔和轮廓；③归一化：取 alpha 包围盒后缩到较长边 686px、居中到 (512,512)（实测 686×512、67.0%、中心 (512,512)）。
      - **标记层从矢量源改成位图源**：`assets/brand/logo-mark.svg` / `logo-mark-small.svg` 删除，改为 `logo-mark.png` / `logo-mark-small.png`（1024×1024，纯白 + 透明底，均 19KB）。流水线改为把标记以 `<image href="data:image/png;base64,…">` 嵌进合成 SVG——复用原有「底板 + 圆角裁切 + 倒角」全部代码，也让 `favicon.svg` 仍是自包含单文件（30KB）。本机无 potrace/inkscape 等描摹器，故不走矢量化。
      - **小尺寸变体改用「填小洞」而不是重画**：简化变体不再手工重写路径，而是把源图里**面积 < 5000px 的封闭洞填实**——眼（~1.1k px）被去掉、播放三角（~45k px）保留，等价于此前手写的「去眼、放大三角」简化，且对杂点自动免疫
      - **封面改为合成阶段贴标记**：`cover-default.svg` 只留底板（玻璃渐变/折射/涟漪/压深），鲸标记由脚本按「宽度 620px、中心 (800,450)」贴合到 1280×720 产物上；直接打开该 SVG 看不到鲸（文件头注释已写明）
      - **验证结论（2026-09-28 生图版，逐项实渲）**：`logo-mark.png` / `logo-mark-small.png` 均 686×512、较长边 66.99%、中心 (512,512)；三端 `default-cover.png` 三份 **md5 逐字节相同**；`foreground.png` 非不透明（alpha 均值 44.8，白形 + 透明底）、`AppScope` 与 `entry` 两份 md5 相同；`favicon.ico` 三帧 16/32/48 仍 PNG 内嵌、IHDR 尺寸与目录一致、偏移与总长自洽；实渲确认 Web 首页顶栏 28px（含卡片默认封面）、Web 登录 64px、Admin 登录 44px 均已换成生图鲸。**未覆盖**：Admin 侧栏 24px、鸿蒙模拟器桌面图标；H5 首页数据来自真实接口、无兜底封面可看，故默认封面在该端未实渲（H5 登录面板标记已于同日实渲确认）
      - **H5 登录面板补上标记（2026-09-28）**：H5 此前只有纯文字「DliDli」，`apps/h5/src/static/logo.png` 产出后一直无人引用；现于 `profile.vue` 未登录面板的文字上方加 96rpx 标记，与 Web 64px / Admin 44px 登录页的品牌呈现对齐。**必须走模块导入（`import logoUrl from '@/static/logo.png'`），不能在模板里写 `/static/logo.png`**：dev 下 `/static` 被 vite 代理到后端（后端 `/static/**` 是带签名的媒体命名空间），而模板里的绝对 `/static` 路径会被 uni-app 编译成模块导入 → 404 → `profile.vue` 的动态导入整体失败、页面直接进错误态（实测控制台报 `Failed to fetch dynamically imported module`）。走模块导入则 dev 由 `/src/static/…` 提供、构建时按 asset 处理，两端都正常
      - **H5 端 `default-cover` / `default-avatar` 仍未接通（既有问题，本次未修）**：`apps/h5/src/pages/**` 里 10 处 JS 常量写的仍是字符串 `/static/default-cover.png`、`/static/default-avatar.png`，与上条同源——dev 下同样命中 `/static` 代理而 404。本次只修了会让整页崩掉的**模板级**引用（`profile.vue`），JS 字符串引用只导致图片缺省、不影响页面加载，故未一并动。**后果**：H5 端的默认封面与默认头像实测仍是坏图（列表页兜底封面、评论头像都受影响）。彻底修需要给 H5 自有静态资源换一个不与后端媒体命名空间冲突的路径（或让后端让出 `/static`），属独立改动，待定
      - **产物归档目录（2026-09-28）**：脚本末尾把本轮全部产物按平台命名镜像到 `assets/brand/dist/`（标记两版 / favicon 三件套 / 三端 logo / 鸿蒙三件 / 封面 / ico 三帧，共 15 个），同目录 `index.html` 是自包含看图页。该目录不参与各端引用（各端吃的仍是工程内那一份），且被 `.gitignore` 里既有的 `dist/` 规则忽略、不进仓库，随时可重跑重建
    - **图形二次定版：闭合的嘴 + 头顶简化小喷泉（2026-09-28，当前版）**：用户判定「开合的嘴」不好、要求嘴闭合且头顶加一个简化小喷泉。三轮共 9 个生图候选后定版，形态：**饱满正圆的大头**（下颌也是连续圆弧，无平底/棱角）+ **嘴完全闭合**（头左端即那条平滑圆弧，不再有楔形开口）+ **头顶一簇简化小喷泉**（三颗纯白水滴成短弧排列）+ 眼在**头前部偏上** + 播放三角**较大且居身体中段** + 贴着身体下缘的**细弧胸鳍** + 厚实两叶尾鳍
      - **同轮修掉的两处内部布局回归**：第一轮候选把**眼放到了身体中段、紧挨播放三角**（鲸的眼该在头前部），且**播放三角明显变小**（32px 帧糊掉、丢了播放语义）；定版轮在提示词里明确「眼在头前部偏上、与三角明显分开」「三角高度约为身体高度的三分之一」，两项均回到上一版观感
      - **被否决的喷泉形态**：薄柱顶一颗大圆球（Y2）会读成**气球插棍/温度计**；纯水滴不连线才读成「喷水」。故定版取「三颗水滴成短弧」
      - **小尺寸变体必须独立重新归一化**：水滴是**独立于鲸身的连通块**，按既有小尺寸口径去掉后鲸身包围盒随之变小；若沿用「含喷泉包围盒」的中心，鲸身会整体**下移约 59px（画布 5.8%）**——实测 `logo-mark-small.png` 中心落在 (511.5,**573.0**)，32px 下肉眼可见偏下。修法：去掉水滴与填小洞之后，按鲸身自己的包围盒**再做一次「较长边 67% + 居中」**，修后中心 (511.5,511.0)。故小尺寸变体处理链为「只留最大连通块 → 填 < 5000px 的洞 → 重新归一化」
      - **验证结论（2026-09-28 二次定版，逐项核对）**：完整标记 `686×610`、较长边 66.99%、中心 (511.5,511.5)；小尺寸标记 `686×487`、较长边 66.99%、中心 (511.5,511.0)；`foreground.png` 与完整标记同包围盒（686×610 / 66.99% / 中心 (511.5,511.5)），`AppScope` 与 `entry` 两份 md5 相同；`background.png` 与 `default-avatar.png` **逐字节未变**（底板纯渐变、不含标记，本轮不含其改动）；三端 `default-cover.png` 三份 md5 逐字节相同、1280×720（16:9）；`favicon.ico` 三帧 16/32/48 仍 PNG 内嵌、尺寸与偏移自洽（6 + 16×3 + Σ帧长 = 文件长 7883）；实渲确认 Web 首页顶栏 28px（含卡片默认封面）、Web 登录页、Admin 登录 44px、H5 登录面板 96rpx 均已换新，28px 与 48px 下仍能读出「鲸 + 喷泉 + 播放三角」。**未覆盖**：**Admin 侧栏 24px**（需登录态）与鸿蒙模拟器桌面图标；H5 端默认封面仍不可见（`default-cover` 未接通的既有问题，非本轮引入）
    - **源文件与流水线（2026-09-28）**：设计源为 `assets/brand/{logo-bg,logo-rim,cover-default}.svg` + `{logo-mark,logo-mark-small}.png`（标记层是生图产出的位图，见上条），全部产物由 `node scripts/svg2png.mjs` 派生；`apps/web/public/favicon.svg` 不再是手写源文件，改为脚本合成产物（带 generated 头注，内嵌标记位图）。改品牌视觉只改 `assets/brand/` 后重跑脚本
    - **液态玻璃与 PNG 交付（2026-09-28）**：底板改液态玻璃——三色渐变叠 `dliDepth`/`dliVignette`/`dliSheen`/`dliSpecular` 四层（全部是黑白透明度叠加，不引入新色相），封面另加 `feGaussianBlur` 折射光带与波纹圈。倒角（`logo-rim.svg`：上白 0.90 → 下黑 0.24 的镜面环，内缩 10px、rx 230 与 rx 240 圆角同心）只在**成品图标合成阶段**叠加，源文件保持满幅无边（鸿蒙遮罩形状由系统决定）。**除 web 端 `favicon.svg` 外一律交付 PNG**：H5 小程序与鸿蒙端本就只吃位图，统一格式避免同一张图两种形态；封面 1280×720 且 `effort: 10`（331KB → 162KB）
    - Admin 登录页原先内联的手绘电视机 SVG（含非 token 蓝 `#23ade5`）一并替换为脚本产出的 `apps/admin/public/logo.png`；同页 `#23ade5` 装饰性光晕/标题渐变未动（属既有装饰，不在本次品牌资产范围）
  - 验证结论（图标）：`hvigorw assembleHap` BUILD SUCCESSFUL；**模拟器桌面实测图标正确渲染**——系统 squircle 遮罩下品牌粉圆角方 + 白色图形，与系统应用并排无异常（以上为首版 2026-09-21 结论）。**2026-09-28 改版验证（脚本产物逐项核对，未重跑模拟器桌面实测）**：`foreground.png` 透明底、白色图形、包围盒中心 (512,512)、较长边 67.0%（完整标记 67.2% / 简化标记 67.0%）；`background.png` 满幅不透明，**与上一版逐字节一致（md5 未变）**——此处原写「左上角像素 `#FFD9E4` 与渐变首色一致」，**该断言不成立**：实测左上角是 `#EED2DA`，底板渐变首色确实是 `#FFD9E4`，但成品边缘被 `dliVignette`（边缘 0.15 黑）与 `dliDepth` 压深，故**不能用角像素反推渐变首色**（口径已于 2026-09-28 二次定版时修正）；`AppScope` 与 `entry` 两份 **md5 逐字节相同**；`favicon.ico` 三帧 16/32/48 均 PNG 内嵌、IHDR 尺寸与目录声明一致、偏移与总长自洽（6 + 16×3 + Σ帧长 = 文件长）；Web 端起 `vite` 实测三个 favicon 资源均 200 且 MIME 正确（`image/svg+xml` / `image/x-icon` / `image/png`）；默认封面在 320px 宽、`aspect-ratio:16/9` 容器中以 `object-fit:cover` 实测 **cropPercent = 0**（首版封面 1600×1000 = 1.6:1，在同一容器内被上下各裁约 5.5%）；新封面 1600×900 恰为 16:9，与消费方容器一致
  - 验证结论（界面材质，2026-09-28）：起 `vite` 实测——Web 首页顶栏与登录页、Admin 登录页与侧栏均正确显示新标记；Admin 顶栏玻璃实测让列表内容滚到吸顶栏下方穿过（截图存档 `tmp_brand_preview/admin-users-scrolled.png`，临时目录不入库）；`vue-tsc` 两个应用均 0 error，`vite build` 均成功，产物 CSS 中断言 `backdrop-filter:blur(18px) saturate(1.6)` 与 `background:#ffffffb8` 已落地。**未覆盖**：需要真实后端数据的浮层（Web 视频页投币/收藏弹层、首页卡片下拉菜单）只有编译产物断言、无实渲截图，后端 `dlidli-api:8000` 未启动时无法复现
  - 未覆盖：**沉浸光感是否真正生效仍未验**——壳层配色为浅色纯色底，材质模糊/蒙层无可比对参照，且 `isImmersiveMaterialSupported()`/`getGlobalMaterialLevel()` 取值未取；三项能力探测与观感/帧率开销归 M4-HMY-01。`viewmodel/`/`service/`/`model/`/`media/`/`danmaku/`/`store/` 目录待各自任务落地时创建，不做空目录占位（**该「未覆盖」已于 2026-09-22 由 M4-HMY-11 补齐能力探测与观感、2026-10-08 由 M4-HMY-01 补齐帧率开销，见本文件 M4-HMY-01/M4-HMY-11**）
  - OpenAPI 生成 ArkTS 类型：**实测判定不可行**（82 个响应 schema 全为无类型的统一包裹 `response.Body`，`data` 为空 schema），改为手写 + 契约核对，依据见 [plan §6](/specs/harmony/plan)
- [x] M4-HMY-03 网络层：HTTP 封装（统一响应包裹/错误码文案/超时与重试/401 静默续期重放）（2026-09-21 完成）
  - 覆盖：HMY-03
  - 实现要点：`service/HttpClient.ets` 为全端唯一 HTTP 出口，职责收敛为四件事——包裹解析、错误码文案、超时与退避重试、401 静默续期重放。配置集中在 `common/constants/ApiConfig.ets`（baseUrl/连接与读取超时/重试次数与退避基数）；文案表 `common/constants/ErrorMessages.ets` 按服务端 `errcode` 的分段规则（1xxxx 通用 / 2xxxx 账号 / …）落表，**服务端 message 存在时优先采用**（同码文案可能带 `WithMsg` 上下文），缺失才回落端侧表与按域兜底；`model/Api.ets` 提供 `ApiBody<T>` 包裹与 `ApiError`（带 `code`/`traceId`/`retryable`），负数段为端侧合成码（离线/超时/响应不合契约）。重试口径：**仅网络类失败与 HTTP 5xx 可重试**（退避 300ms×2ⁿ，共 3 次尝试），业务错误不重试；401 走**单一飞行**续期（并发 401 只发一次 `/auth/refresh`，其余请求复用同一 Promise），续期成功重放一次原请求，续期不可用则清凭证并回「登录已过期，请重新登录」且不给重试入口。`store/TokenStore.ets` 基于 `@ohos.data.preferences` 并以内存镜像供请求同步取用，**存储异常一律降级为「仅内存生效」并记日志**——存储不可用不应让请求链路抛错。配套 `components/ErrorStateView.ets`（可重试失败态）与 `pages/home/HomePage.ets` 三态接通，`module.json5` 补 `INTERNET`/`GET_NETWORK_INFO` 权限，`EntryAbility.onCreate` 注入 Context 并回填凭证
  - **实测抓到并修掉一处**：`send()` 的 catch 原先把**所有**异常都重写为「网络不可用」，把 `parse()` 抛出的业务错误一并吞掉——后果是 401 被降级成网络失败、触发 3 次无意义重试，且**续期分支永远不可达**。改为 `err instanceof ApiError` 时原样上抛（只把 `req.request` 的传输层异常定级为网络类），修后日志链路正确：`status=401 code=10003`（不重试）→ 续期不可用 → 清凭证 → `retryable=false`
  - 验证结论：`hvigorw assembleHap` **BUILD SUCCESSFUL**（ArkTS 零告警）；**模拟器实测三态**——① 成功：首页分区栏拉到后端真实 12 项（动画/游戏/科技数码…）；② 失败可重试：停掉后端 → 连接超时判为网络类（`errCode=2300028 → -2`）→ 退避重试 3 次 → 呈现「加载失败 / 网络超时，请重试」+ 重试按钮，**未白屏**；③ 恢复：后端起回后点「重试」（按 `dumpLayout` 实测矩形 `[549,1158][772,1278]` 取中心点击）→ 重新加载 12 项。**401 分支**用临时把接口指向需鉴权的 `/api/v1/users/me` 验证（验完已还原）：`code=10003` 不重试 → 无 refresh_token → 清凭证 → 文案「登录已过期，请重新登录」且**不出现重试按钮**（业务错误不可重试）
  - **模拟器访问宿主机的口径（本次实测）**：`http://10.0.2.2:8000` 直连宿主 loopback 可用（QEMU 用户态网络），明文 HTTP 未被系统拦截，无需 `hdc fport` 转发；该地址由 `ApiConfig.BASE_URL` 单点维护，真机联调改宿主局域网 IP 即可
  - 未覆盖：401**续期成功后的重放**分支（需真实登录态才能造出有效 refresh_token，归 M4-HMY-04，**已于 2026-09-21 由 M4-HMY-04 实测补齐**）；WS 弹幕通道（M4-HMY-07，**已于 2026-09-22 实测覆盖**，结论见 M4-HMY-07）
- [x] M4-HMY-04 登录与会话：手机号验证码与密码登录、令牌偏好存储、静默续期、退出清理（2026-09-21 完成）
  - 覆盖：HMY-01、HMY-02
  - 实现要点：契约以**服务端实现为准**核对——`GET /auth/captcha` 实返 `{id, svg}`（**内联 SVG 文本**，swagger 注释里的 `{captcha_id, image_base64}` 与实现不符，已在 `model/Auth.ets` 注明）；`/auth/sms-code` 在 dev 环境回显 `debug_code`。`model/Auth.ets` 落 `Profile`/`TokenPair`/`SmsCodeResult`/`CaptchaResult`；`service/AuthApi.ets` 覆盖验证码/短信/密码登录/刷新/登出/`users/me`，**鉴权前的自证类请求（验证码、登录）一律走 `postPublic`/`getPublic`**，不带 Authorization、不参与 401 续期，避免「未登录却先续期」；`store/Session.ets` 以 `@ObservedV2` + `@Trace` 单例承载登录态（`loggedIn`/`nickname`/`avatar`/`level`），`hydrate()` 先用 `TokenStore.ready()` 对齐异步回填再判定，并以 `AuthApi.me()` 校验令牌（401 时清凭证回落未登录）；`logout()` 先尽力调用服务端 `/auth/logout`（失败只记日志，不阻断本地清理）再清凭证。`common/utils/AppRouter.ets` 收口 `NavPathStack` 的路由入口（`RouteName.LOGIN` + `bind/push/pop`），`pages/Index.ets` 由 `Navigation(this.pathStack).navDestination(...)` 承接压栈页；`pages/auth/LoginPage.ets` 为 `NavDestination`（自带返回键），双模式（验证码/密码）、60s 倒计时、dev 回显验证码自动填充、密码模式失败后清空验证码并换图；`pages/profile/ProfilePage.ets` 拆未登录/已登录两态，退出走 `this.getUIContext().showAlertDialog`（`AlertDialog.show` 在 API 26 已废弃，改后 ArkTS 零告警）。`TokenStore` 新增 `ready()` 并让 `restore()` 幂等，解决「onCreate 异步回填与读登录态竞态」。新增端侧语义色 `state_danger`（`#F56C6C`，对齐 Web 端 ElMessage error 色；Web 侧无对应 SCSS 变量，为端侧新增 token）
  - **图形验证码渲染：实测两条 ArkUI 路线均不可用，定案为端侧解析 SVG + Canvas 重绘**（`components/CaptchaImage.ets`）：
    - ① ArkUI `Image` 对 SVG 的支持**不含 `<text>`**——同一份服务端 SVG 里 `<rect>`/`<line>` 正常渲染，`<text>` 恒不渲染（去掉 `font-family`、换字体仍不渲染）；
    - ② 改 ArkWeb：`loadData` 会把入参直接拼进 `data:` URI，SVG 里 `fill="#f5f5f5"` 的 `#` 被当作 fragment 起始符截断 → 整页空白；换 `loadUrl` + base64 data URI 后 `onControllerAttached`/`onPageEnd` 均正常触发（svg 长度 872、页面加载结束），**但模拟器上 Web 不上屏**——同一页面里插纯色 `div` 也不渲染（chromium 侧 `BlankScreenDetector result_count 0`）；
    - 定案：服务端 SVG 由本仓库自己生成、元素形态固定（`rect` + 4 `line` + 4 `text`），端侧正则解析后用 `Canvas` 重绘（`line` → `moveTo/lineTo/stroke`，`text` → 平移+旋转+`fillText`，基线语义与 SVG 的 `text x/y` 一致）。**本期零后端改动，接口形态不变**
  - **实测抓到并修掉两处**：
    - ① **`TextInput` 不回写状态**：输入框原经 `@Builder` 按值传参构造（`TextInput({ text: value })`），dev 回显验证码后提示文案已变、**输入框仍为空**；密码模式登录失败后 `captchaCode=''` 也**清不掉已输入的验证码**（换图后旧码残留）。根因是 builder 形参非状态引用、可编辑组件的文本不随外部状态回写。改为**内联 `TextInput({ text: $$this.xxx })` 双向绑定**后，自动填充与失败清空均正确
    - ② 模拟器首次 `uitest uiInput inputText` 会弹「小艺输入法」隐私同意弹窗（系统级 IME 弹窗，非 App 问题），未点「同意」前输入不生效，会误判为输入框故障
  - 验证结论：`hvigorw assembleHap` **BUILD SUCCESSFUL（ArkTS 零告警）**；模拟器实测（API 26 实例 `Pura X View`，后端本地 `http://10.0.2.2:8000`）逐项通过：
    - **短信登录**：手机号 `13800138000` → 发送验证码（倒计时 60s 起、dev 回显码自动填入 `944963`、提示「验证码已发送，本地调试环境已自动填充」）→ 登录成功，自动注册落库（`user` 表 1 行、`dli_22465501`/Lv1），返回「我的」呈现已登录卡片（昵称 + Lv1）
    - **登录态持久化**：`aa force-stop` 后重启 → 「我的」仍为已登录卡片（凭证经偏好存储回填 + `me()` 校验通过）
    - **退出登录**：`showAlertDialog` 确认框（取消 / 退出登录）→ 确认后回未登录态；重启后仍为未登录（清理已落盘）
    - **图形验证码**：端侧 Canvas 重绘结果与 Redis 答案**逐字符一致**（屏上 `W42J` ↔ Redis `w42j`；`6VYH` ↔ `6vyh`）；按实测反馈放大到 **150×50**（与后端 SVG 同为 3:1 等比，字号 22→27.5），点击可换图
    - **密码登录**：验证码被后端接受——`/auth/login/password` 返回**「账号或密码错误」而非「验证码错误或已过期」**（服务端 `LoginByPassword` 首步即 `captcha.Verify`，该校验通过才可能落到密码比对），随后验证码字段自动清空并换新图
    - **401 静默续期 + 重放（补齐 M4-HMY-03 的未覆盖分支）**：停机后篡改偏好存储里的 `auth.access_token` 签名→重启，日志链路完整：`凭证已恢复，登录态=true` → `业务失败：/api/v1/users/me status=401 code=10003`（不重试）→ `凭证已写入偏好存储` → `访问令牌已静默续期`；页面仍呈现已登录（重放成功）。**轮换已闭环**：磁盘 access_token 由被篡改值换为新 JWT，refresh_token 由 `bc7c…` 轮换为 `9615…`，Redis 旧 `sess:bc7c…` 已删除、仅存 `sess:9615…`
  - 未覆盖：资料/我的投稿/观看历史/收藏（归 M4-HMY-09）；全部结论均在 API 26 模拟器取得，**真机待验**
- [x] M4-HMY-05 发现：首页信息流（LazyForEach 分页 + 下拉刷新 + 触底加载）、分区导航与最新/最热、搜索（含排序筛选与历史同步）`2026-09-21`
  - 覆盖：HMY-40、HMY-41
  - 实现要点：
    - **端侧基础件**（`common/utils/`）：`Format.ets`（`formatCount` 万/亿、`formatDuration` mm:ss 与 h:mm:ss、`formatPubdate` 相对时间，对齐 `packages/shared/src/format.ts`，端侧不消费 TS 包故手写移植）、`MediaUrl.ets`（回环地址 → `ApiConfig.BASE_URL`，封面/头像同源改写）、`LazyDataSource.ets`（泛型 `IDataSource`，仅 `reset`/`append`/`count`，用逐条 `onDataAdd` 规避 `DataOperationType` 联合类型字面量）、`Toast.ets`（`showToast` 声明 `@throws`，包一层免调用点 try-catch）。`PreferenceStore.ets` 收口 `dlidli_prefs` 单一入口（`TokenStore` 改为复用，对外 API 不变）
    - **模型与 API**：`model/Video.ets`（`OwnerBrief`/`StatBrief`/`VideoCard`/`VideoListResult`/`PagedResult<T>`，并注明 `/videos` **不返回 total**，`hasMore` 只能以「本页是否满页」判定）；`service/VideoApi.ets`（`listCategories` / `listVideos`（`page_size`）/ `listRecommended`（参数名为 `size`））；`service/SearchApi.ets`（`searchVideos`/`searchUsers`）
    - **首页**：分区 chips（「首页」+ 一级分区，按 `parent_id === 0` 过滤）+ 排序页签 推荐/最新/最热（推荐仅全站可见；切到分区时自动落到最新）；`Grid` 双列 + `LazyForEach`，`Refresh({refreshing: $$this.refreshing})` 下拉刷新（刷新失败且已有内容时只弹 toast，不砸列表）、`onReachEnd` 触底加载下一页（失败保留 `page` 不变，重试即重取同页）；页脚 `LoadMoreHint` 作为跨两列的 `GridItem`（`columnStart(0)/columnEnd(1)`），呈现 加载中/加载失败点击重试/没有更多了；空态与失败重试走既有 `ErrorStateView`
    - **搜索**：关键词搜索视频 / UP 主 + Tab 切换 + 分页；**搜索历史落端侧** `search.local_history`（最多 10 条、重复即置顶、逐条删除、一键清空、重启后仍在）；卡片与 UP 主行复用 `VideoCardItem` 版式，UP 主行展示头像/昵称/签名/等级
    - **壳层**：首页标题栏搜索框点击切到搜索页签（入口非输入框）；删除已无引用的 `PlaceholderView.ets` 与 `page_pending_home`/`page_pending_search` 文案
  - **待后端（本期零后端改动，两端无接口可用，均已实测确认）**：
    - **SRH-02 排序/筛选**：`GET /api/v1/search` 只有 `keyword`/`type`/`page`/`page_size`，无 `sort`/筛选参数（M2-SRH-03 起即登记待补）→ 端侧不呈现排序筛选 UI，待后端扩参后接
    - **SRH-04 历史云端同步**：全仓无搜索历史接口（Web 端同样无），故本期历史**仅端侧**，plan §3 的「与云端同步」待后端接口就位后再接
  - **实测抓到并修掉两处**：
    - ① **搜索历史面板不可达**：原按 `submitted` 是否为空决定展示历史面板，一旦搜过一次就再也回不到历史（清空关键词后提交会被「请输入关键词」拦下）。改为「已提交且有输入 → 结果，否则 → 历史」，并在输入框内加清空按钮（`sys.symbol.xmark_circle`）
    - ② **空结果时 tab 行消失**：`resultHeader`（含 视频/UP 主 tab 与「共 N 条」）原只在与结果列表同一分支渲染，0 结果时没有 tab 可切（视频无果就无法去 UP 主 tab）。改为 tab 行独立于结果态渲染，「共 N 条」仅在 `total > 0` 时展示
  - 验证结论：`hvigorw assembleHap` **BUILD SUCCESSFUL（ArkTS 零告警）**；`go test ./...` 与 `go vet ./...` 全绿（本期无后端改动，回归确认）；模拟器实测（API 26 实例，后端本地 `http://10.0.2.2:8000`）逐项通过：
    - **数据口径**：dev 库原无稿件，种子数据 26 稿件 / 26 UP 主（覆盖 12 个一级分区，含空封面、超 1 小时时长、亿级播放等边界）——推荐链路的同 UP 打散要求一稿一主，故 UP 主数与稿件数 1:1
    - **首页信息流**：推荐首屏 20 条 + 触底续 6 条后呈现「没有更多了」；分区切换（动画 → 3 条，推荐页签隐去并自动落最新）；最新/最热切换（最热按播放量降序，26.6万 > 24.3万 > 22.8万 逐屏验证）；下拉刷新（日志 7 次 page-1 重载且列表回顶）；封面走 `10.0.2.2` 正常出图、时长角标 `04:11`/`1:02:05` 两种格式、meta 行 `1.2亿 · 生活UP主05 · 5小时前`（万/亿与相对时间均正确）
    - **默认封面接入（2026-09-30）**：空封面原先只画一块 `border_default` 底色占位，与 Web/H5/Admin 的品牌默认封面不一致。现改为 `Image($r('app.media.default_cover'))`——产物由 `scripts/svg2png.mjs` 与三端一并派生（同一次合成循环、同一份底板与标记贴法），落在 `entry/src/main/resources/base/media/default_cover.png`（鸿蒙资源名只允许小写字母/数字/下划线，故用下划线而非连字符），并同步进 `assets/brand/dist/` 归档清单。**验证结论**：四端 `default-cover.png` **md5 逐字节相同**（`B93C45D8…`，163475 字节）；模拟器实测首页「测试 05/25/20」三张空封面卡片已渲染品牌鲸+喷泉封面（原为灰块）；`hvigorw assembleHap` BUILD SUCCESSFUL
    - **搜索**：关键词 `测试` → 视频 tab 20 条 + 「共 26 条」→ 触底续 6 条 + 「没有更多了」；UP 主 tab 同样 20 + 6 且双 Tab 各 26 条；0 结果两种文案（视频/UP 主）与 tab 切换均正常；历史新增（`测试` → `UP主` 后 `UP主` 置顶）、单条删除、清空、**重启后历史仍在**（`aa force-stop` 后重进）；点卡片弹「播放页建设中，敬请期待」（播放页归 M4-HMY-06）
  - 未覆盖：触底加载失败与刷新失败的**重试分支**未做停机实测（沿用 M4-HMY-03 已验证的 `ErrorStateView` 口径）；全部结论均在 API 26 模拟器取得，**真机待验**
- [x] M4-HMY-06 播放页：AVPlayer HLS 播放、清晰度切换与倍速、进度记忆与跨端续播、有效播放上报、签名过期静默换签、触屏手势与横屏全屏、切后台处理（2026-09-22 完成）
  - 覆盖：HMY-10、HMY-11、HMY-12、HMY-13、HMY-14、HMY-15
  - 实现要点：
    - **分工**：`media/VideoPlayer.ets` 只管播放器状态机与事件（`open`/`play`/`pause`/`toggle`/`seekTo`/`setRate`/`setVolume`/`release`，对外只暴露 `@ObservedV2 @Trace` 的 `phase`/`positionMs`/`durationMs`/`buffering`/`videoWidth`/`videoHeight`），`pages/play/PlayPage.ets` 管业务（详情装载、跨端续播、进度与有效播放上报、签名续签、手势、横屏全屏、切后台）。HLS 调用收敛在一处，页面里没有 AVPlayer
    - **起播时序（surface 先于起播）**：AVPlayer 的 `surfaceId` 首次只能赋在 `initialized` 态，故详情（`detailReady`）与 XComponent `onLoad`（`surfaceReady`）**两者都就绪才 open**，先到先等（`tryStart`）。`VideoPlayer.open` 内部统一用「**先注册 `waitForState` 再改状态**」的顺序防漏事件：等 `initialized` 才赋 `url`、等 `prepared` 才 seek（`prepare()` 的 Promise 会立即 resolve，不能只 await 它），整链超时 15s 判失败
    - **`media/PlaySign.ets`**：从签名 URL 的 `e=` 参数解析过期时刻（`signExpiryMs`），续签判定只依赖这一处
    - **跨端续播（HMY-12）**：`resolveResume` 已登录优先取 `GET /videos/{bvid}/progress`，未登录或读失败回落端侧 `store/PlaybackProgressStore.ets`（键 `playback.local_progress`，只留最近 50 条、按 `updatedAt` 淘汰）；**片头 3s 内、片尾 3s 内不续播**——那两处续播对用户是打扰
    - **进度与有效播放上报（HMY-13）**：单条 500ms 心跳收口全部周期动作。观看时长按 `positionSec` 的**真实增量**累计（单次增量 ≥ 2s 判为跳转、不计入；采样取 500ms 而非 1s，是因为 **2x 倍速下 1s 增量恰为 2.0 会被误判成跳转**）；累计满 5s 上报一次 `reportView`（失败允许下个心跳重试，服务端按 uid/IP 去重）；进度落盘节流 10s，且**返回、切后台、自然播完、组件销毁四处都主动 flush**（不能只靠节流定时器）
    - **签名静默续签（HMY-14）**：心跳里判签名距到期不足 5min 就重取详情，按**画质值**（非下标——防服务端档位增减后切到别的清晰度）对齐新地址，再以「保留进度 + 保留播放态」换源续播；失败只记日志、下个心跳重试。与切清晰度**共用同一条 `reopen` 路径**（reset → url → prepare → seek → play）
    - **切清晰度与倍速（HMY-11）**：`settingsPanel` 里清晰度 chips（服务端按 `quality` 降序下发，取首档即最高画质，对齐 Web 端 `pickDefaultSource`）+ 倍速 chips `[0.5, 0.75, 1, 1.25, 1.5, 2]`（对齐 Web 端），倍速走 `setPlaybackRate` 连续取值，非档位跳变
    - **手势与全屏（HMY-15）**：手势层是一层铺满舞台的透明 `Column`（必须显式 `hitTestBehavior(Block)`，否则触摸被下层 XComponent 的原生 surface 吃掉）；`GestureGroup(Parallel, Tap×2, Tap×1, Pan)`——**Parallel 下双击会让 count:1 与 count:2 各命中一次，净效果正好是「控件切两次回到原状 + 暂停」**，于是无需延迟单击、单击也不必为双击窗口让路（手写计时版单击要等 300ms 才响应）。拖动主轴**由起手方向定死、途中不再切换**：横向 = 进度（`offsetX / 屏宽 × 总时长`），竖向按左右半屏分亮度 / 音量（手指划过整屏高度对应 0→100%）。全屏 = `setWindowLayoutFullScreen(true)` + `setPreferredOrientation(LANDSCAPE)`，退出反向（先回 PORTRAIT 再关 fullscreen）；返回键在全屏态先退全屏（`onBackPressed` 返 true），第二次才离开播放页
    - **亮度**：走 `setWindowBrightness`（只影响本应用窗口，不动系统亮度）；端侧无读接口故从满亮起算，下限 5%（允许压暗但不给全黑，否则用户找不到恢复的控件）；离开页面时若用户真调过则还原为跟随系统（`-1`）
    - **切后台**：压栈的 `NavDestination` 拿不到 `@Entry` 页的生命周期，改由 `EntryAbility` 经 `emitter` 广播（`common/constants/AppEvents.ets` 的 `ID_BACKGROUND=1002`/`ID_FOREGROUND=1001`）→ 收到即暂停并落盘；**回前台不自动续播**（用户可能只是切走看了眼别的）
    - **页面装配**：`stage`（XComponent + 手势层 + 控制层 + 中央态）+ `infoBlock`（标题/统计meta/UP 主/标签/简介）；中央态三态互斥（缓冲·起播中转圈 / `ended` 重播按钮 / `error` 文案 + 重试）；无流稿件走独立文案「该稿件暂无可播放的清晰度」
  - **实测抓到并修掉五处**：
    - ① **控件一显示手势就全失效**：控制层是铺满舞台的 `Column`，默认 `HitTestMode.Default` 会把空白处的触摸连同下层手势层一并吞掉（实测：控件可见时单击/双击/拖动**都不触发**，控件隐藏时才正常）→ 控制层根容器改 `hitTestBehavior(HitTestMode.Transparent)`（自身照常响应、不阻塞兄弟节点），手势层另用 `Block` 挡住 XComponent 的原生 surface
    - ② **拖动会顺带把控件栏闪掉**：`GestureMode.Parallel` 里 `TapGesture` **不会因位移自行取消**，横向拖进度 / 竖向拖亮度时这根手指会额外命中一次单击，正在看的进度反馈反而被隐藏 → 加 `dragging` 标记与 `DRAG_TAP_GUARD_MS = 200` 尾闸，拖动期间与刚结束 200ms 内丢弃单击
    - ③ **续签插进起播途中导致起播失败**（`5400102`）：心跳**先于首次 open** 就起来了（`tryStart` 里 `startTicker` 在 `reopen` 之前），若签名已临期（用户隔了几小时回来点开）会在起播途中再插一次 `reopen`，两次 open 交错、**先落地的那个把状态打回 `idle`**，后一个的 seek 撞 `5400102 Operate Not Permit`，表现为起播失败 → `maybeRenew()` 加 `phase ∈ {playing, paused}` 闸（只在播放器稳住时换签）；复测起播链路零 `5400102`
    - ④ **时间戳在换源途中被打回 0**：换源会 reset 到 `preparing`，`positionMs`/`durationMs` 双双归零，进度条被写成 `00:00 / 00:01`；更麻烦的是 `durationSec()` 的下限 1 会把非零当前进度夹到 1、触发一次**程序化** `onChange`，反过来 `seekTo(1000)` 覆盖掉正要恢复的进度 → 加 `lastDurationSec` 缓存上一次时长 + 换源期间冻结 `sliderSec` + `switching` 期间屏蔽 `onChange` 的 seek
    - ⑤ **档位高亮不跟随状态**：`@Builder` 多参数走**按值传递**，参数变化不引起内部 UI 刷新（实测：切到 360P 后仍高亮 720P）→ 改单对象参数（`ChipOption`）走按引用传递，高亮才跟随
  - 验证结论：`hvigorw --no-daemon assembleHap` **BUILD SUCCESSFUL**（ArkTS 编译通过；`media` 值引用会带 4 条 syscap 提示，见下方「口径修正」）；`go test ./...` 与 `go vet ./...` 全绿（本期零后端改动，回归确认）；模拟器实测（API 26 实例 `Pura X View`，后端本地 `http://10.0.2.2:8000`，种子稿 `BVSEED0001`（流 90001，720P+360P HLS，时长 12:21））逐项通过：
    - **HLS 起播（HMY-10）**：真实解码出画面（非黑屏占位），`initialized → prepared → playing` 完整
    - **跨端续播（HMY-12）**：以 Redis（`wp:u:{uid}` 为进度真值）预置进度后进页即从该位置起播（实测 300000ms → 屏上 `05:09`）并弹「已为你续播」
    - **进度与有效播放上报（HMY-13）**：以 Redis 为 oracle 逐段核对——观看中 `wp:u:{uid}` 由 90 → 115 → 129 递增，`his:u:{uid}` zset 同步写入
    - **清晰度切换（HMY-11）**：720P → 360P 日志 `切换清晰度：quality=360 pos=90.092s`，屏上位置保持 `01:30`、播放态保持，未回零
    - **倍速**：切 1.5x 后约 12s 墙钟推进 18.76s（≈1.56×，含起播损耗）
    - **手势（HMY-15）**：单击切控件（计数 `2 → 0 → 2`）；双击暂停（画面冻结 10s 且覆盖层不变）；横向拖动 −400px 使进度由 311s 退到 90s（与 `offsetX / 屏宽 × 总时长` 换算**严格吻合**），拖动过程中提示条（`mm:ss / mm:ss`）与进度条常驻；竖向拖动左半屏 `亮度 77%`、压到底 `亮度 5%`，右半屏 `音量 0%` / `27%` / `100%`
    - **横屏全屏（HMY-15）**：全屏后窗口 2232×1320、视频铺满、隐藏信息区；退出全屏后进度保持且底栏子控件仍可点；返回键在全屏态先退全屏
    - **切后台（HMY-15 后半）**：切后台后 Redis 进度冻结（两次采样均 129/241 不变），重启后停在原位**且不自动续播**
    - **签名静默续签（HMY-14）**：临时把续签阈值放大到 24h 强制触发，得到完整干净的一轮 `initialized → prepared → 换源后跳转到 → 跳转完成 → playing → 静默换源（playing=true）`，每次换源位置都保住、零 `5400102`（验完阈值已还原 5min）
    - **实测方法补充**：播放页**截图会取到旧帧**（两次间隔 2s 的快照完全一致），本任务改以 `uitest dumpLayout` 导出的控件树 + Redis 作为可判定 oracle；`uitest uiInput swipe` 的第 5 个参数是**速率**（200~40000）而非时长，要做「慢拖」须传 200
  - 未覆盖：**换源瞬间画面会短暂空白**（同实例 reset，模拟器约 3.5s），双播放器无缝切换列为后续优化；弹幕与互动评论未接（**弹幕已于 2026-09-22 由 M4-HMY-07 补齐**，见该条；互动评论归 M4-HMY-08，页内以一行提示标注）；**起播（约 12s）与换源（约 3.5s）耗时、以及 HLS 硬解表现均为模拟器口径，真机待验**（模拟器视频硬解受限，播放类结论一律不据此判定「鸿蒙不支持」）
  - **交付后修复（2026-09-22，PATCH v0.48.1）**：验收反馈「白底画面下进度条看不清」，同时复现出两处起播缺陷。
    - ① **控件层无蒙层**：控制层是白字 + 半透明白轨，压在亮画面上整体消失（白底素材下进度条与时间戳几乎不可见）→ 顶栏/底栏各加一层渐变蒙层（`SCRIM_TOP` `#B3000000` 顶实→透、`SCRIM_BOTTOM` `#CC000000` 透→底实）。蒙层挂在控件容器自身，底部控件行落在渐变更实的一侧；实测白底帧下时间戳、已播（品牌粉）与未播（半透明白）轨道均可辨，蒙层不影响命中测试（倍速面板仍正常展开）
    - ② **等待 `prepared` 超时阈值偏紧**：`STATE_TIMEOUT_MS` 原为 15s，而模拟器冷启下 HLS 拉清单到 `prepared` 实测 **15.0~15.5s**（连续两次失败，`prepared` 恰在超时后 0.4~0.5s 到达），表现为起播直接判失败、状态机停在 `error` → 提到 **30s**（真机起播是秒级，该上限只在对端无响应时生效）。修后同一路径起播正常（`prepared` 14.9s → 续播 seek → `playing`）
    - ③ **超时路径把 `undefined` 摆到用户面前**：等待失败抛的是自造 `Error`，而 catch 一律 `as BusinessError` 取 `code`（类型断言不改运行时形态）→ 屏上显示「播放失败（undefined）」。改抛 `StateWaitError`（带 `friendly` 字段），日志留技术细节、UI 给「起播超时，请重试」/「播放出错，请重试」；**用临时把阈值压到 1s 强制触发实测**（日志与屏上文案均已核对，验完已还原 30s）
    - 口径修正：本节原记「ArkTS 零告警」**不准确**——`media` 命名空间的值引用（`SeekMode`/`BufferingInfoType` 常量）会触发 SDK syscap 提示共 4 条，`assembleHap` 输出为 `BUILD SUCCESSFUL` 但带 `ArkTS:WARN`，说明见 `media/VideoPlayer.ets` 文件头
- [x] M4-HMY-07 弹幕：分段拉取与预取、Canvas 轨道渲染、WS 实时下发与断线重连及 HTTP 回退、关键词/发送者屏蔽、展示设置、发送与频控、列表面板（2026-09-22 完成）
  - 覆盖：HMY-20、HMY-21、HMY-22、HMY-23、HMY-24
  - 全屏弹幕须对齐[官方影音娱乐规范](/specs/harmony/plan)（§7.2）：上下有黑边时弹幕仅在上方黑边区域内显示；无黑边时限制同屏弹幕密度
  - 实现要点：
    - **分工**：`danmaku/DanmakuEngine.ets`（Canvas 轨道渲染，纯渲染无 IO）、`danmaku/DanmakuController.ets`（分段池/屏蔽/发送/列表/降级轮询）、`danmaku/DanmakuSocket.ets`（WS 通道）、`danmaku/DanmakuSettings.ets`（`@ObservedV2` 单例设置）、`components/DanmakuLayer.ets`（逐帧驱动：取位置 → 补分段上屏 → 绘制）、`model/Danmaku.ets` + `service/DanmakuApi.ets`。播放页只接线与画 UI，与 Web 端 `useDanmakuController + DanmakuLayer` 同分工
    - **引擎以视频时间为相位基准**：一条弹幕的位置是 `(videoMs - startMs) / durationMs` 的**纯函数**，由此免费得到三个正确行为——暂停时弹幕冻住（Web 端 DOM 动画做不到，暂停后弹幕仍在飘）、seek 后落到新位置的正确轨迹而非从右侧重飘一遍、分段/实时弹幕**迟到**时直接出现在它该在的位置。代价是没有「动画完成回调」，回收改为每帧按 `videoMs` 判定。`videoMs()` 以播放器采样值为锚点 + 墙钟按倍速外推（`timeUpdate` 约 100ms 一次，直接拿它画会「一格一格跳」）
    - 常量与口径：`LINE_HEIGHT=32` / `TRACK_PAD=4` / `MIN_TRACKS=3` / `MAX_ACTIVE=240` / `FIXED_DURATION_MS=4000` / `LATE_WINDOW_MS=12000`（迟到窗口——不加它，一次前进式 seek 会把整段几百条早已过期的弹幕塞进引擎，白跑一轮绘制与轨道分配）；轨道分配让同速弹幕在**前车走完 1/3** 时入场（hold = 滚动总时长/3），全满时随机叠放；`fillText` 的 y 是**基线**而非行框上缘，须从行框底部上提一个降部（`DESCENT_RATIO=0.25`），否则首行会被裁到区域外（实测）
    - **分段拉取与预取（HMY-20）**：段号变化时拉当前段并**预取下一段**；失败回滚 `loadedSegs` 允许下个心跳重试；段长 `segment_ms` 由首次响应带回（端侧不硬编码 6min）。池按段存放 + 每段一个扫描游标，正常播放整体是 O(新上屏条数)（池可上万条）
    - **§7.2 区域与密度**：`dmLetterboxBar(stageW, stageH, videoW, videoH)` 只在视频比舞台更「宽」时算出上下黑边，且**窄于 `LINE_HEIGHT + TRACK_PAD*2`（40vp）的黑边按「无有效黑边」处理**（放不下整行，不如不截）；有黑边 → 区域只取上方黑边区（`densityScale=1`）；无黑边 → 铺满舞台，全屏态再乘 `FULLSCREEN_DENSITY_SCALE=0.6` 压同屏密度。舞台尺寸取 `onAreaChange`（vp，与 Canvas 绘制单位一致），画幅/全屏变化清屏重排（旧轨道号在新区间可能越界）
    - **屏蔽（HMY-23）**：关键词（`content.includes`）+ 发送者哈希，服务端账号级下发；**WS 帧不经服务端过滤，本地过滤是唯一防线**；`is_self` 的弹幕始终可见（否则发完自己先看不见，观感像发失败了）。DM-20/21 均未要求「解除屏蔽」UI，故本期不做
    - **发送（HMY-21）**：等级门槛/频控/去重**全部由服务端裁决**，端侧只翻译错误码（40001 太频繁 / 40002 需 Lv1 / 40003 需 Lv3 / 40004 内容重复）——**不做本地倒计时**，端侧与服务端时间窗不同步，本地拦反而误伤。发送成功走乐观上屏（服务端广播排除发送者本人，故不会重复）
    - **WS 与回退（HMY-22）**：指数退避重连（2/4/8/15/15s，`MAX_RETRY=5`），次数用尽置 `degraded` 并起 **15s 分段轮询**兜底（轮询重取当前段——发送会让服务端段缓存失效，故能拿到新弹幕），状态回到 `open` 即停轮询
    - **列表面板（HMY-24）**：`/list` 每页 50（服务端上限 200）、按 id 倒序（最新在前）、触底加载更多、点条目跳到该时间点
    - **展示设置**：开关**落盘记忆**（对齐 DM-11「并记忆状态」；Web 端只持久化展示项、不记忆开关）；不透明度/字号/区域/速度/密度。影响轨道分配的档位（区域/密度）改动要**清屏重排**，只影响画笔的档位（开关/不透明度/字号/速度）重画即可——**暂停时没有新弹幕入队，光靠引擎的 dirty 标记不会重绘**，故 `DanmakuLayer` 每帧比较两个签名并分别触发
    - 契约以服务端实现为准核对：`/danmaku` 返回 `{segment, segment_ms, list[]}`；`/list` 的分页参数是 **`size`**（不是 `page_size`）；雪花 id 在 JSON 里是**字符串**（端侧定 `id: string`）；`/ws?token=` 只出不进（上行帧被服务端丢弃，发送一律走 HTTP）；游客可用 `/danmaku` 与 `/list`，`/blocks` 需登录（未登录只记日志，不阻塞弹幕展示）
  - **官方 §7.2 黑边分支：实测几何 + 一处越界缺陷（本次抓到已修）**：
    - **本机内容下黑边分支在稳态不可达**：竖屏舞台高度由视频画幅等比算出（`screenWidthVp × videoH/videoW`），舞台画幅恒等于视频画幅 → `bar=0`；横屏全屏实测舞台 `744×440vp` vs 视频 `1280×720` → `bar=0`（走「无黑边 + 密度 0.6」分支）。实测日志：竖屏 `弹幕区域：舞台 440x247.5vp 视频 1280x720 全屏=false 黑边=0vp`、横屏 `舞台 744x440vp … 全屏=true 黑边=0vp`（本条日志为本次新增，真机排查黑边问题可直接看它）
    - **但旋屏过渡会短暂经过「竖屏 + 全屏」中间态**：`舞台 440x677vp → 黑边 214.75vp`、`舞台 440x744vp → 黑边 248.25vp`（随后才转成横屏），说明黑边分支不是死代码
    - 为核验该分支渲染，按 2.39:1 宽银幕片横屏全屏下的黑边（≈64vp）**临时抬一个黑边下限做探针**，并在 dev 库**临时插 3 条聚集弹幕**（6.0/6.2/6.4s）占满轨道（两者验完均已还原/删除）：
      - 修前：3 条轨道全开，第 3 行文字落在 68~100vp，**越出 64vp 黑边、画到画面里**
      - 修后：只剩 1 行且全在 64vp 之内（截图核对）
    - 根因：`trackCount` 的 `Math.max(MIN_TRACKS, count)` 只按「区域高 × 区域比例 × 密度档 × 全屏系数」算，**没有区域高度的物理上限**，`MIN_TRACKS=3` 在黑边 40~104vp 时会把弹幕排到区域外（3 行需要 104vp）。规范要求有黑边时弹幕**只在上方黑边区域内**显示，故加 `capacity = floor((frame.height - TRACK_PAD*2) / LINE_HEIGHT)` 并取 `min(max(MIN_TRACKS, count), capacity)`，`enter()` 在 `tracks === 0` 时直接不上屏（宁可不显示，也不画到画面里）。常规画幅下 `capacity` 不收紧（竖屏 247.5vp → 容量 7 > 档位 6；横屏 440vp → 容量 13 > 档位 7），只影响「区域放不下 3 行」的黑边与极扁画幅
  - **结项 M4-HMY-01 余项「弹幕 WS 通道待验」（2026-09-22 实测）**：ArkTS `@ohos.net.webSocket` 与 Go Hub 互通本身无碍，**唯一阻塞是 Origin 白名单**——① ArkTS 的 WebSocket **自生成 Origin**，默认形态 `http://<host>`（**不带端口**，`connect()` 的同名 header 覆盖不掉），API 26 起可用 `supportOriginPort: true` 让它带上端口；② 服务端 `CheckOrigin` 对 `allow_origins` 做**精确匹配**（dev 为 `http://localhost:5173~5175`），直连 `ws://10.0.2.2:8000` 时 `Origin=http://10.0.2.2` 不在白名单 → 握手 403 → 端侧按失败重连、`MAX_RETRY` 用尽后置 `degraded`（这条降级链路已实测：`弹幕通道异常：code=200` ×5 → `实时弹幕不可用，改用分段轮询兜底` → 15s 后 `弹幕分段 0 就绪：4 条`）；③ 端侧临时把 WS 地址指向设备侧 `localhost:5173`（`hdc rport tcp:5173 tcp:8000` 反向转发到本机 8000）并置 `supportOriginPort: true` 后：握手成功（`弹幕通道已连接`）、`?token=` 鉴权生效、收到真实广播帧并上屏（`收到实时弹幕帧：id=2102328490358476800 t=3000`），且命中屏蔽的帧被**本地过滤**；④ 结论：dev/局域网环境的唯一阻塞是**服务端白名单未含客户端来源，端侧无从绕过**；生产若同源（App 连 `wss://<正式域名>`）则 Origin 与白名单条目一致。**上线前需服务端确认此项**（本期零后端改动，故未动白名单；`supportOriginPort` 也未随包发布——默认带端口在正式域名上反而可能破坏 Origin 匹配）
  - 验证结论：`hvigorw --no-daemon assembleHap` **BUILD SUCCESSFUL**；`go test ./... -count=1` 全绿（本期零后端改动，回归确认；`internal/module/danmaku` 0.552s）；模拟器实测（API 26 实例 `Pura X View`，后端本地 `http://10.0.2.2:8000`，种子稿 `BVSEED0001` = 4 条 @2/5/8/11s）逐项通过：
    - **分段与预取（HMY-20）**：进页**同一瞬间**打出 `弹幕分段 0 就绪：4 条（段长 360000ms）` 与 `弹幕分段 1 就绪：0 条（段长 360000ms）`——段 1 起点在 6:00，是预取而非按需（为取这条证据本次新增了段就绪日志）
    - **Canvas 渲染**：竖屏 00:03 时「弹幕自检 A」自右缘进入并左移；横屏全屏同一条按密度 0.6 分支正常上屏（截图核对，修 `capacity` 后两条分支均无回归）
    - **发送（HMY-21）**：Lv1 白字滚动 → toast「弹幕已发送」+ **乐观上屏且带 `is_self` 白框** + 服务端落库；Lv3 门槛（顶部/彩色）→ toast「Lv3 解锁彩色弹幕与顶部/底部弹幕」且选中态不变
    - **屏蔽（HMY-23）**：加关键词后服务端落 `danmaku_block` 行 + 面板出 chip + 重 seek 后该条（00:08）不再渲染；屏蔽发送者后落行 + 「已屏蔽 1 位用户」+ 该发送者 4 条全部不渲染
    - **列表面板（HMY-24）**：共 4 条、最新在前、点「复制」出「已复制」toast
    - **WS（HMY-22）**：见上条 M4-HMY-01 结项（真实握手 + 帧上屏 + 本地过滤均已实测）；`degraded → 分段轮询` 兜底另在本次复跑中再次实测
    - 验证后状态复原：临时弹幕与屏蔽行已删（`BVSEED0001` 4 行、`video_stat.danmaku_cnt=4`、`danmaku_block` 0 行）、反向端口转发已撤、段缓存键 `dm:v:{vid}:0` 已清
  - 未覆盖：**真机待验**（全部结论均在 API 26 模拟器取得）；**黑边分支的稳态真机场景未覆盖**——需画幅比宽于窗口的宽银幕片（本机内容全 16:9，故只以探针核验，见上）；列表「加载更多」未在 >50 条的稿上实测（种子仅 4 条）；发送频控/去重（40001/40004）的**服务端裁决文案**未在 UI 复现（端侧不做本地拦截，错误码翻译由 `ErrorMessages` 表覆盖）；暗色模式下的弹幕观感未核对
- [x] M4-HMY-08 互动与评论：点赞/投币/收藏/长按三连（幂等 + 原生动画）、评论一级与二级、分享走系统面板（2026-09-30 完成）
  - 覆盖：HMY-30、HMY-31、HMY-32
  - 状态：**代码已完成、ArkTS 编译零告警、接口层与模拟器端到端交互均已实测（2026-09-28 首轮 + 2026-09-30 复验），已勾选**
  - 实现要点：
    - **分工**：`service/InteractionController.ets`（互动状态与动作：赞/币/藏/三连 + 收藏夹弹层）、`components/ActionBar.ets`（播放页互动栏：四个入口 + 投币/收藏弹层 + 三连原生动画）、`components/CommentSection.ets`（评论区：一级 + 二级、排序、分页、发布/回复、点赞、删除）、`service/InteractionApi.ets`（接口层）、`service/ShareUtil.ets`（系统分享面板）、`model/Interaction.ets`（端侧模型）。播放页只接线：`ActionBar` 与 `CommentSection` 挂在「简介」页签下，替掉原先的 `play_pending` 占位
    - **长按三连（ITR-30）**：由 `LongPressGesture({duration: 1500})` 直接驱动 `InteractionController.doTriple()`——**不在控制器里另起定时器**（手势组件自带时长判定，重复计时会把手感拉成 3s；开发中一度如此，已修）。松手时 Tap 与 LongPress 在 Parallel 组下**都会命中**（与播放页双击/单击并存同一现象），而三连本身已含点赞，故用 `TAP_AFTER_LONG_PRESS_GUARD_MS = 300` 抑制紧随的单击——否则刚点上的赞会被自己取消
    - **原生动画反馈（HMY-30「原生动画」）**：`uiContext.animateTo({duration:180, curve:EaseOut, iterations:2, playMode:Alternate})` 让点赞图标做一次 1→1.4→1 脉冲；另以 0/240/480ms 三段递进把 赞→币→藏 依次点亮（1200ms 后复位）。用 `getUIContext().animateTo` 而非全局 `animateTo`（后者已废弃，会产生 ArkTS 告警）
    - **幂等（承 ITR §3）**：服务端互动接口本身幂等（点赞/收藏为开关语义、投币有唯一键、三连先扣币失败退款），端侧再以 `acting` 串行化 + **乐观更新与失败回滚**兜底——双击或网络重试在服务端只生效一次
    - **投币上限按版权判定**：`detail.copyright === 1 ? 2 : 1`（自制 2、转载 1），与 Web 端一致；已投过时端侧提前提示「已经投过币啦」，服务端 50001 仍会兜底
    - **收藏走默认夹**：`toggleFavorite(bvid, '')`，服务端懒创建「默认收藏夹」。弹层列出收藏夹供选择（默认夹排在首位、带「（默认）」后缀）。多收藏夹管理（ITR-21 的增删改）属 P1，本期不做，接口层已注明
    - **分享（HMY-32）**：`systemShare.SharedData({utd: general.hyperlink, content, title})` → `new ShareController(data).show(context, options)` 调起系统分享面板。**端侧差异**：不依赖微信小程序卡片（鸿蒙端无小程序载体）。面板调起失败兜底为复制链接到剪贴板（`@throws` 已在 `copyLink` 声明，对齐 `showToast` 口径）。分享**不统计计数**——SHR-02 只要求能调起面板，且 `show()` 的 Promise 仅表示面板关闭，不代表用户完成分享
    - **分享链接可配置**：新增 `ApiConfig.WEB_BASE_URL`（与 `BASE_URL` 分开，因本地联调时后端在 `:8000`、Web dev server 在 `:5173`，生产各自换正式域名），`ShareUtil.videoShareUrl` 由它拼接 `/video/{bvid}`
    - **评论（CMT-01~04）**：一级评论分页（`page_size` 参数）+ 热度/最新排序 + 楼中楼「展开更多回复」。发布走乐观插入列表头；回复走乐观挂载到该一级评论的 `replies` + `reply_cnt++`；点赞按会话级 `likedSet` 乐观更新计数，失败回滚；删除仅 `is_self` 显示入口，确认框走 `getUIContext().showAlertDialog`（`AlertDialog.show` 在 API 26 已废弃）
    - **ArkTS 约束下的一处写法**：`CommentItem` 是 interface，`{...c, like_cnt: n}` 会触发 `arkts-no-spread`（interface 不可展开）→ 收敛为 `clone()` + `replaceRoot()`/`replaceReply()` 三个具名方法做不可变替换，既过编译又集中了「哪一层要刷新」的逻辑
    - **契约以服务端实现为准核对**：评论雪花 id 在 JSON 里是**字符串**（`json:"id,string"`）；`/users/me/collections` 的 data **直接是数组**（非 `{list}` 包裹）；评论列表返回 `{list, total}`；`/comments/{id}/replies` 分页参数同为 `page_size`；`toggleFavorite` 请求体的 `collection_id` 用字符串（服务端按 int64 解析，空串=默认夹）
  - 验证结论（接口层，真实后端）：`go test ./...` 与 `go vet ./...` 全绿（本期零后端改动，回归确认）；`hvigorw assembleHap` **BUILD SUCCESSFUL（ArkTS 零告警）**；以本地后端（`http://10.0.2.2:8000` 对应宿主 `:8000`）逐项打通：**互动**——点赞 true→false（开关幂等）、投币 2 枚（自制上限）后再投返回「已经投过币啦」、收藏默认夹 `faved=true` 且收藏夹列表出现「默认收藏夹 default=1」、三连返回 `{liked:true, coin_count:2, faved:true}` 且硬币余额 6→4、互动状态聚合正确；**评论**——发一级评论 → 回复楼中楼（`reply_cnt=1`）→ 列表 hot/new 两种排序正确返回 → 评论点赞 `liked=true` → 楼中楼分页 total=1 → 删除自己的回复成功
  - 验证结论（模拟器端到端交互，API 26 `Pura X View`）：在已登录态（`13800000002`）下走 `BVSEED0001`（唯一带转码流可播放的种子稿）实测——**点赞**开关（16↔15，实心/空心随状态切换）、**投币** 2 枚（自制上限）后再投被拦（"已经投过币啦"）、**收藏**进默认夹并弹出收藏夹选择、**长按三连**出"三连成功，感谢支持！"且赞未被随后的单击取消。首轮（2026-09-28）另实测：**发布评论**（计数 1→2）、**评论点赞**、**分享**调起系统面板。
  - **本轮抓到并修掉两个真实缺陷（2026-09-28 发现、2026-09-30 复验）**：
    - **收藏夹弹层渲染 `默认收藏夹[object Object]`**：`col.name + (col.is_default === 1 ? $r('...') : '')` 把 `$r()` 返回的 **Resource 对象**拼进字符串。改为名称与「（默认）」两个相邻 `Text`（不用字符串拼接）。复验：弹层显示 `默认收藏夹` + `（默认）`，全屏布局转储中**不含 `object Object`**
    - **评论点赞计数不刷新**：`ForEach` 的 key 生成器只用 `c.id`，而 `CommentItem` 是**普通 interface（非 `@Observed`）**——key 不变时 ArkUI 不重建子项，`like_cnt` 更新后视图保持旧值。key 改为 `${index}#${c.id}#${c.like_cnt}#${c.reply_cnt}`（楼中楼同法加 `like_cnt`）。复验：点赞后计数由 1→2 且图标转实心，与服务端 `comment.like_cnt` 一致
    - **一处误判已更正**：「2 条评论只渲染出 1 条」**不是丢项**——第二条根评论原本落在 y≈2240、而屏幕高 2232，属**视口截断**；下滑后两条根评论并列正常渲染。首轮的"丢项"结论作废（key 修复的真正收益是上一条的计数刷新）
  - 未覆盖：**真机待验**（全部结论均在 API 26 模拟器取得）；评论的 @用户与表情（CMT-01 的 P1 部分）与举报（CMT-06）本期不做；暗色模式下互动栏/评论区的观感未核对
- [x] M4-HMY-09 个人中心：资料与我的投稿、观看历史、收藏、未登录态入口、端侧偏好与进度缓存（2026-09-28 完成）
  - 覆盖：HMY-42、HMY-04
  - 实现要点
    - **装配**：`pages/profile/ProfilePage.ets` 一文件三结构——`ProfilePage`（壳层第三个页签，只按 `session.loggedIn` 分流）、`ProfileGuest`（未登录：图标 + 提示 + 「登录 / 注册」按钮推 `RouteName.LOGIN`）、`ProfileContent`（已登录主体：资料卡 + 投稿·历史·收藏三档 + 双列分页列表 + 退出登录）
    - **列表状态刻意下沉到 `ProfileContent`**（而非外层）：登录成功使外层重新渲染到已登录分支 → 子组件全新创建、`aboutToAppear` 自然触发首屏拉取；退出登录随分支销毁一并清空。如此无需在登录/登出时手工重载或重置，登出后再登录也不会看到上一账号的残留列表
    - **资料卡**：头像（无则 HarmonyOS Symbol 占位）+ 昵称 + `Lv{n}` 徽标（品牌主色字 + 品牌浅底）+ 个性签名（空则回落 `profile_signature_empty`「这个人很神秘，什么都没有写」，`maxLines(1)` 溢出省略）+ 右侧「退出登录」小胶囊。卡片走 `DliMaterial.surface()` + `.systemMaterial(DliMaterial.card())`；退出登录小胶囊按 M4-HMY-11 定下的「小控件用中性次级底色实填、不挂材质」口径处理
    - **`Session` 补 `signature`**（`@Trace`）——`applyProfile` 写入、`reset` 清空，资料卡据此渲染
    - **三档列表共用一套分页骨架**（沿用首页 `Refresh` + `Grid` + `LazyForEach` + `LoadMoreHint` + `ErrorStateView` 口径）：`LazyDataSource<VideoCard>` + `VideoCardItem`（开 `showDate`，用投稿/观看时间替掉播放量行）。**接口差异收敛为一个内部结构** `ProfilePageResult { list, total }`，`total = -1` 表示「该接口不返回 total」
    - **hasMore 双口径**（按各接口实际契约取，不搞「一律满页推断」）：`/videos/history` 只返回 `{list}`，只能按「满页即还有」判定（`list.length === PAGE_SIZE`）；`/videos/mine`、`/users/me/favorites` 返回 `{list,total}`，按「已加载 < total」精确判定。三档由 `ProfileContent.fetchPage` 按 `tab` 分派
    - **错误与刷新的分工**：首屏失败置 `ErrorStateView` + 重试；下拉刷新失败若已有数据则只弹 `feed_refresh_failed` toast（**不清列表**，与首页一致）；触底加载失败只把 `LoadMoreHint` 置 `failed` 态提供行内重试。`ErrCode.UNAUTHORIZED` 一律回落为 `session.hydrate()`——若令牌确已失效，分支自然切回未登录态
    - **新增接口层**：`VideoApi.mine(page, size)` + 端侧模型 `MineResult`（`model/Video.ets`，注释里显式标注「返回 total，与 `/videos`、`/recommend/videos`、`/videos/history` 的满页口径不同」）；历史/收藏复用已有的 `VideoApi.history` 与 `InteractionApi.favorites`
    - **字符串资源**：新增 7 条（`profile_signature_empty`、`profile_tab_mine/history/favorites`、`profile_empty_mine/history/favorites`）；「我的投稿」空态文案点明「投稿请到 Web 端」（鸿蒙端无投稿入口，本期不做）
    - **退出登录**：`getUIContext().showAlertDialog`（`AlertDialog.show` 在 API 26 已废弃）确认后 `session.logout()`
  - 验证结论：`hvigorw --no-daemon assembleHap` **BUILD SUCCESSFUL**（新增文件 ArkTS 零告警；仅 `media/VideoPlayer.ets` 4 条既有 `media` 能力告警）；`go test ./...` 全绿（本期零后端改动，回归确认）；接口契约以真实令牌对本地后端逐项核对（`/users/me` 含 `signature`/`level`；`/videos/mine` 与 `/users/me/favorites` 返回 `{list,total}`；`/videos/history` 只回 `{list}`）
  - **模拟器端到端走查（API 26 `Pura X View`，2026-09-28）**：未登录态（图标 + 文案 + 登录入口）→ 验证码登录（`13800000002`）→ 资料卡昵称/`Lv`/签名回落文案正确 → **投稿**空态（日志 `tab=mine 共 0 项 total=0`）→ **历史** 2 条真实记录且顺序与接口一致（日志 `tab=history 共 2 项 total=-1`，触底置「没有更多了」）→ **收藏** 3 条真实记录（日志 `tab=favorites 共 3 项 total=3`）→ 点卡片正确进入播放页（标题与 bvid 对上）→ 返回 → 退出登录确认框（系统玻璃观感）→ 确认后回落未登录态
  - **分页实测**：造 26 条观看历史（`BVSEED0001..0026`）后，接口 page1=20 / page2=6 正确；端侧日志 `tab=history 共 20 项 total=-1` 证实「不返回 total」一侧的满页 `hasMore` 判定生效
  - 未覆盖：**真机待验**（全部结论均在 API 26 模拟器取得）；HMY-04 的三类端侧偏好键（`danmaku.settings`/`playback.local_progress`/`search.local_history`）已分别随 M4-HMY-07/06/05 落地实测，**本任务未新增偏好键**；个人信息编辑（改昵称/头像/签名）本期不做（Web 端已有）；暗色模式下的资料卡与列表观感未核对
- [x] M4-HMY-10 端侧验收：核心链路走查（登录 → 找内容 → 播放 → 弹幕 → 互动 → 个人中心）+ 性能指标测量（冷启动、起播、弹幕帧率、崩溃率、包体积）（2026-10-08 完成）
  - 覆盖：[spec §4 成功指标](/specs/harmony/spec)
  - **环境**：API 26 模拟器 `Pura X View`（`hdc -t 127.0.0.1:5555`，x86_64 软件渲染），宿主本地后端 `:8000`（模拟器内 `http://10.0.2.2:8000`，MySQL/Redis 均 up）；被测包 `entry-default-unsigned.hap`（1,545,391 B，sha256 `43EC2E4D…EB56`，2026-10-08 15:31 构建，`hdc install -r` 装入）。**所有数值一律标注环境**：模拟器可得 / 真机待验。
  - **走查方法（可复现，UI 自动化注入 + 三重取证）**：
    - 操作注入：`hdc -t 127.0.0.1:5555 shell "uitest uiInput <click|longClick|drag|keyEvent|inputText> …"`（长按三连用 `longClick`，进度用 `drag`，中文输入用 `inputText`）
    - 控件定位：`uitest dumpLayout -p /data/local/tmp/layout.json` + `hdc file recv`，**递归遍历 `{attributes,children}` 树**取 `type/text/description/bounds`（顶层即 `attributes`+`children`，不能按数组解析；TextInput 的 `text` 常为空、须看 `hint`）；辅助脚本 `.dev-logs/m4hmy10/ui.ps1`（dump/click/swipe/drag/snap）与 `ui-click-text.ps1`（按文本命中后点其 bounds 中心）
    - 判定依据：应用内读 `hilog -x -D 0xD11D`（domain `0xD11D`，DEBUG 级，含 `PlayPage/VideoPlayer/Danmaku/Interaction/ProfilePage` 等 TAG）与 `hilog -x -D 0x0`（`EntryAbility`）；**服务端数据一律用接口实测值判定，不读端侧自述**；关键界面用 `snapshot_display` 截图存证
    - 起播/冷启动脚本：`.dev-logs/m4hmy10/measure-coldstart.ps1`、`measure-playstart.ps1`、`seek2.ps1`
  - **走查结果（逐项 PASS）**：
    - **登录**：游客态可浏览首页/搜索/播放；退出登录日志 `凭证已清理` + `已退出登录` 并回落未登录 UI；验证码登录 `13800000002` → `登录成功，uid=2108099597086756864 level=1`，昵称 `dli_05071407`
    - **找内容**：首页信息流 `信息流加载完成：sort=<sort> category=<id> 共 N 项`；分区+排序 `sort=new category=1 共 20 项` / `sort=hot category=1 共 20 项`；搜索 `搜索完成：kw=课程 type=0 total=2`
    - **播放**（`DV2TpILB9KIOO`，741s，720P/360P/原画三档）：起播 `状态：idle→initialized→prepared→playing`；**清晰度切换** `切换清晰度：quality=360 pos=372.875s` → `换源后跳转到 372875ms 已下发` → `跳转完成：372875ms`，且 `弹幕区域：… 视频 640x360`（由 1280x720 变 640x360，证明确实换流）**进度不丢**；**倍速** `倍速已生效：1.5`，12s 墙钟推进 20s 内容（≈1.5×）；**横屏全屏** `全屏：true`，舞台由 440x247.5vp 转 744x440vp（截图存证）
    - **弹幕**：**渲染**——截图三帧分别捕获 `vVV VV`（646.2s）、`发广告刚刚`（651.8s）、`嘎嘎嘎嘎…个`（676.5s）在视频上层滚动且位置逐帧左移；**发送** `M4HY10danmaku` → `/danmaku/list` total 23→24（id `2108102926701432832`，`mode=1`，`time_ms=719600`）；**屏蔽**——关键词 `哈哈`/`haha` 落 `danmaku_block`（id 5/6），**同一 token 下 `/danmaku?segment=1` 由 14 条降为 11 条**（服务端过滤），面板出 chip；**列表面板**——`共 23 条`、条目含 `时间/内容/复制/屏蔽`、点条目 `跳转完成：676515ms`、点「复制」出 `已复制` toast；**开关** `弹幕开关：开`
    - **互动**（含长按）：**长按 1.5s 三连** → `三连完成：like=true coin=1 fav=true`，服务端 stat `like 1→2 / coin 0→1 / fav 2→3`，赞/币/藏三枚图标转实心（截图）；**点赞**再点回退（`like 2→1`，开关幂等）；**投币**再投被拦 `已经投过币啦`；**收藏**点击取消（`fav 3→2`）→ 再点弹层选「默认收藏夹（默认）」→ `fav 2→3`；**评论**发布 `M4HMY10cmt` → UI `4 条评论`→`5 条评论` 且列表首条即该评论，接口 `/comments` total 4→5
    - **个人中心**：资料卡（昵称 / `Lv1` / 签名回落「这个人很神秘，什么都没有写」）；**投稿**空态「还没有投稿，投稿请到 Web 端」；**历史** `列表加载完成：tab=history 共 1 项 total=-1`（卡片带观看进度 `12:22`）；**收藏** `tab=favorites 共 1 项 total=1`（即刚收藏稿件）；**退出登录** 确认框 → 回落游客态
  - **性能实测（方法与数值）**：
    - **冷启动**（口径：`hilog` domain 0 的 `Ability onCreate` → domain `0xD11D` 的首页 `信息流加载完成`，脚本按 hilog 毫秒时间戳做差）`measure-coldstart.ps1 -Runs 6 -WaitSec 10`：`onCreate→contentLoaded` 286/343/339/296/359/383ms；**`onCreate→首页信息流就绪` 1231/1305/1330/1236/1286/1486ms（均值 ≈1312ms）**。模拟器可得，**真机待验**（spec 目标 <2s 为真机口径）
    - **HLS 起播**（口径：`VideoPlayer` 的 `状态：initialized` → `状态：playing`，拆分 `init→prepared` 与 `prepared→playing`）首开（播放器/编解码冷启）`15.16s + 0.33s ≈ 15.5s`；同一稿件**连续 10 轮**（`measure-playstart.ps1 -Runs 10 -WaitSec 25`）：`init→prepared` 2407~2773ms、`prepared→playing` 706~1263ms、**合计 3113~4036ms，均值 3608ms，P90 = 3837ms**。**模拟器可得、真机待验**：spec 目标 P90 < 1.5s 在该环境下无法达成，且**不得据此判定「鸿蒙不支持」**（x86_64 无硬解直通，M4-HMY-06 已记 15.0~15.5s）
    - **弹幕渲染帧率**（方法同 M4-HMY-01：`hitrace -t 10 -b 65536 -o <f> graphic ace animation` 抓帧 → 数应用进程的 `B|<pid>|H:OnVsyncEvent`，取标记内 `now:` 纳秒，**帧率 = (条数-1) ÷ 首末差**；静止时该标记恒为 0，故不能用固定窗口数帧）：播放在 634→650s（弹幕密集段，**同屏 1~3 条**）→ **551 个标记 / 9.968s → 均值 55.18fps**，帧间隔 p50 16.00ms（≈62.5fps）、p90 32.00ms、**50 帧 >32ms（≈9% 掉帧）**。**不构成 spec §4「≥55fps」达标证据**：非「满载」（同屏 1~3 条；M4-HMY-01 已测 45 条与 1000 条无差异）、模拟器 x86_64 软件渲染、片源 30fps——该阈值须**真机 + ≥60fps 片源 + 满载弹幕**验证
    - **崩溃率**（判定口径：验收窗口内 `/data/log/faultlog/faultlogger` 新增条目）**0 条**：该目录内 `com.dlidli.app` 共 4 条 appfreeze，时间戳 10-07 12:03/12:06/12:14 与 10-08 10:01，**均早于本轮**；无 cppcrash；同期 `filemanager` 的 cppcrash 与本应用无关。样本 = 6 次冷启动 + 6 次进播放页，**0 崩溃 / 0 卡死**。属**窗口内样本**，非线上统计口径
    - **包体积**：`entry-default-unsigned.hap` **1,545,391 B = 1.47 MiB**（13 条目，**无原生 `.so`** 故无 ABI 分裂；`ets/modules.abc` 819KB + `ets/sourceMaps.map` 364KB（release 可去）+ 资源 ≈344KB）。**达标**：占 spec 目标 <30MB 的 **4.9%**
  - **缺陷与遗留（如实登记，含未修项）**：
    - ① **弹幕 WS 实时通道仍不可用**（承 M4-HMY-07 结项项，**未修**）：本轮日志 `弹幕通道 15000ms 后第 5 次重连`，面板常显「实时弹幕已断开，正在定时刷新」，即**全程走分段轮询降级**；根因是服务端 Origin 白名单（ArkTS WebSocket 自生成 `http://<host>`、不带端口），**上线前须服务端确认**。端侧按降级设计闭环，不影响本期验收结论
    - ② **弹幕开关被上轮实验留成「关」→ 首跑会误判为渲染故障**：首进播放页无任何弹幕渲染，设置面板 chip 显示「关」。该值持久化在 `danmaku.settings`，**代码默认是 `enabled = true`**（`DanmakuSettings`），故本次是**上轮实验的残留偏好、非产品默认**；**验收/真机首跑必须先确认该开关**，本轮据此修正后才取到渲染证据
    - ③ **进度条点击跳转存在无效点击**（**未修，低优先级待复现**）：同一自动化坐标区间内 `click 868 1289` 不改变进度（进度停在 `676515ms`），而 `click 500 1289` 正常跳 `141000ms`；**拖动稳定生效**（本轮取数改用 `drag`，见 `seek2.ps1`）。暂判为「自动化注入与真实手指差异」未排除，**未修**
    - ④ **分段接口与列表接口的屏蔽过滤口径不一致**（**待议**）：同一账号 `/danmaku?segment=1` 返回 11 条（过滤 3 条含「哈哈」），而 `/danmaku/list` total 仍 24。列表是管理面板（每条带「屏蔽」入口），按 HMY-24 口径不算缺陷，但两处口径不一致已登记
    - ⑤ **残留测试数据**：服务端无「删弹幕」接口，本轮遗留**弹幕 1 条**（`M4HY10danmaku`，id `2108102926701432832`）与**投币 1 枚**（无取消接口）；屏蔽词 2 条、评论 1 条、收藏、点赞均已按接口复原，验收后 `video_stat = like 1 / coin 1 / fav 2 / danmaku 24 / comment 5`（`view` 由 9→10 为回放预期）。另观察：评论删除后 `video_stat.comment` 未回落（接口 total 4、stat 仍 5），属既有计数口径
  - 未覆盖：**真机待验**（spec §4 五项的绝对数值、暗色模式、弱网重试、长时间播放的内存/稳定性）；本轮**未覆盖弹幕满载（同屏数十条）**的帧率与内存（须真机 + 高帧率片源）；后台/锁屏长时保活沿用 M4-HMY-50 结论（真机项）
- [x] M4-HMY-11 视觉：沉浸光感落地——底栏改 HDS 悬浮玻璃胶囊、材质赋色改中性系统色、氛围背景组件与壳层透明化、卡片/胶囊接入系统材质（2026-09-22 完成）
  - 覆盖：HMY-05
  - 说明：视觉独立成条，**先于端侧验收交付**——观感要在验收走查前定版，否则验收完再返工
  - 实现要点
    - **底栏改造为 HDS 悬浮胶囊**：壳层由自绘 `Tabs` + `barFloatingStyle` 换为 `@kit.UIDesignKit` 的 `HdsTabs`（对齐官方样张 `multi-news-read`）——`.barPosition(End)` + `.barOverlap(true)` + `.barMode(Fixed)` + `.divider({ mode: DividerMode.NONE })` + `barFloatingStyle({ barBottomMargin: 16, adaptToHandedness: true, barWidth: { smallWidth: 294, mediumWidth: 328, largeWidth: 328 }, systemMaterialEffect: { materialType, materialLevel } })`。窄 `barWidth` 正是「收成悬浮胶囊」的机关（铺满整宽的永远是一条色条）；材质由 HDS 出，`MaterialTokens` 不再管底栏。`HdsTabs({ index })` 绑定 `@Local currentIndex` 已足够，**不引入 `HdsTabsController`**。因 `barOverlap(true)` 让内容从胶囊下穿过，各长列表底部留白改用 `DliSize.TAB_RESERVED = 96`（否则最后一行被胶囊压住）
    - **材质赋色改中性、品牌粉只留选中态**：色板删掉 `glass_tint`/`chip_bg`/`tabbar_mask`，改为 `surface_secondary`（浅 `#F1F3F5` / 深 `#262626`）与 `material_edge`（白描边）；`bg_page` 提为**纯白**（对齐 HDS「页面白、材质灰」的层次约定）。`MaterialTokens` 由 5 档工厂收敛为 **3 档**（`titleBar` ULTRA_THIN+interactive / `card` REGULAR+shadow / `chip` THIN+interactive+shadow+lightEffect），三者的 `materialColor` 一律取 `surface_secondary`；`panel()` 与 `PANEL` 因无引用删除；材质对象仍在模块加载时构造一次复用
    - **`surface()` 永远返回实色中性底，不返回透明**：实测 `systemMaterial` **不会吃掉 `backgroundColor`**（二者并存，底色照旧透出），而材质在均匀浅底上几乎不可见——若地基色透明，卡片会整个「消失」。故兜底色与材质赋色取同一档中性色：能力可用时是「中性玻璃 + 光感」，不可用时退化为平铺次级底色，两种情况都不丢轮廓
    - **氛围底收敛为很淡的品牌洗色**：`AmbientBackdrop` 保留结构（180° 渐变 + 三枚径向光斑，`Column` + `radialGradient`），但把品牌粉压到近乎不可察觉——顶部 `ambient_top` `#FFF7FA` 与白页仅约 3% 亮度差，光斑不透明度降到 `#1F`/`#14`/`#0F`（≤12%）。**克制是这一层的全部要点**：早前把品牌粉铺到 30%、还染进材质（`glass_tint`），界面到处是半透明的粉，玻璃与品牌色互相稀释，反倒读不出主次
    - **小控件改为中性次级底色实填、不挂材质**（撤掉原 `chip_bg` 半透明品牌实底）：小尺寸胶囊/行内控件在均匀浅底上挂材质出不了轮廓，按钮会失去可点区域的视觉暗示（退出登录按钮、搜索历史胶囊都出现过）；改用与材质同色的中性实底保形，观感与挂材质一致但一定可见。选中态/主按钮才用品牌实色
  - **实测修正一处早期结论**（模拟器 API 26 `Pura X View`）：早前记为「`systemMaterial` 压过 `backgroundColor`」——**2026-09-22 复测推翻**：以退出登录按钮做探针，保留 `systemMaterial(card())` 同时把 `backgroundColor` 置 `border_default`(`#E3E5E7`)，灰色实底正常渲染出来。真因是当时地基色透明 + 材质赋色为品牌粉、在均匀浅底上出不了轮廓，被误读成「压过」。据此才定下「`surface()` 永远给实色中性底」的口径
  - **能力探测取值（2026-09-22 复跑仍成立）**：启动日志 `沉浸光感：supported=true state=1 level=0`（`ENABLE`/`EXQUISITE`），继续消解 [plan §6](/specs/harmony/plan) 的「沉浸光感是否真正生效」；系统弹窗（退出登录确认框）自带明显玻璃观感，是最直观的旁证
  - **实测抓到并修掉两处**：
    - ① `AmbientBackdrop` 初版用 `Circle` 画光斑，形状组件默认填充盖住下层渐变 → 改 `Column` + `radialGradient`
    - ② `position({ y: -DliSize.TITLE_BAR_HEIGHT - 60 })` 触发 ArkTS `arkts-no-polymorphic-unops`（一元负号仅限字面量）→ 提为模块级 `const PRIMARY_BLOB_Y: number = 0 - DliSize.TITLE_BAR_HEIGHT - 60`
  - 验证结论：`hvigorw --no-daemon assembleHap` **BUILD SUCCESSFUL**（ArkTS 零告警）；`go test ./...` 全绿（本期零后端改动，回归确认）；模拟器逐页实测截图通过——首页（极淡品牌顶 + 悬浮玻璃胶囊选中态 + 中性卡片）、搜索历史、搜索结果（**玻璃胶囊身后红色海报清晰可辨**，是沉浸光感的关键证据）、我的（已登录/未登录）、登录页；壳层标题栏搜索胶囊点击切页签经 `uitest` 实测生效（无需 `HdsTabsController`），登录链路顺带复跑通过
  - 未覆盖：暗色模式下的氛围底与材质观感未截图核对（模拟器 `param set persist.sys.color.mode` 报 errNum 1001、`settings` 二进制缺失，切不过去；两套 token 均已就位）；全部结论均在 API 26 模拟器取得，**真机待验**

## M4 追加（W49+）创作端

> **口径变更（2026-09-30）**：[spec §1](/specs/harmony/spec) 原定「投稿不在本期范围、引导至 Web」，V1.3 起端侧扩为**观看 + 创作**。M4 的结论并未作废——它是**阶段性范围决策**，而技术预研（见 [plan §2.1](/specs/harmony/plan)）已确认端侧具备完整投稿能力。**创作者中心（数据看板/收益/合集管理）仍只在 Web**，端侧不承接。

- [x] M4-HMY-50 投稿预研：`@ohos.request` 与后端分片协议的对接方式定论（**阻塞 HMY-51，必须先做**）
  - 覆盖：—（工程；为 HMY-50~54 定实现方案）
  - 背景（立项时的**假设**，已被下方实测修正）：后端分片是 **`PUT` + 裸二进制 body**（`packages/api-client` 的 `putRaw`），而 `request.uploadFile` 的 `UploadConfig` 的 `files` 文档写 "multipart/form-data"，SDK 未见 `PUT` 示例——当时的推断是「uploadFile 只能发 multipart、二者不兼容」。两条路线必须实测择一，详见 [plan §6.1](/specs/harmony/plan)。**实测结论：该推断不成立**，帧格式由 `method` 决定（`PUT`→裸 body、`POST`→multipart），见下方②
  - 判定项（用真实 ≥5MB 文件对本地后端跑通即可定论）：① `request.uploadFile` 能否发 `PUT`；② 后端是否接受 multipart（若不接受，B 路线或"加后端 multipart 变体"需二选一）；③ `begins`/`ends` 是否为**字节区间**语义（决定能否只传一段）；④ 切后台/锁屏时上传是否持续（`backgroundModes` 声明是否必需）
  - **实测方法（2026-10-07 初测 / 2026-10-08 复测，均为 `apps/harmony` 一次性探针页、跑完即删）**：对本地真实后端（模拟器内 `http://10.0.2.2:8000`，MySQL/Redis 均 up）跑完整链路（init → 逐片 PUT → complete）。判定**不读端侧自述**：① 读后端落盘 `server/uploads/chunks/<uploadId>/<index>.part` 的字节数；② 读 `complete` 结果；③ **宿主机对合并成品 `server/uploads/videos/source/<sha>.mp4` 重算 SHA-256 并与文件名（=端侧上报 `file_hash`）比对**。2026-10-08 复测用**端侧自生成的确定性填充文件**（1 KB / 5 MB / 9 MB / 11 MB / 20 MB / 25 MB / 30 MB；生成方式受限于 hdc 无法写入应用沙箱，故由端侧在 cacheDir 自建并做首/中/尾抽点校验），**11 MB / 9 MB / 25 MB 三份成品 size 与 sha256 全部 MATCH**。
  - **验证结论**：**A 路线可行且为主路线；B 路线亦可行，作降级。后端零改动成立。**
    - **① PUT：能**。SDK 声明即写「value can be **POST** or **PUT**」（`@ohos.request.d.ts`，since 6），实测多轮全部 `responseCode=0`。
    - **② 帧格式由 `method` 决定（2026-10-08 复测修正）**：`method:'PUT'` → **裸 body**（显式区间时请求体恰为该闭区间，落盘无边界开销）；`method:'POST'` → **multipart/form-data 信封**。证据：`POST /videos/cover`（后端 `c.FormFile("file")`）**5 次全 200 且封面落盘 50000 字节**（=源文件长度），而 `PUT /videos/cover` 返回 **404**；反向以 `POST` 打分片接口 → 服务端 **404**、客户端挂起到超时。**故分片走 PUT、封面走 POST，同一 `request.uploadFile` 覆盖两条路径，无需后端 multipart 变体、也无需自造 multipart 编码器**。
    - **③ `begins`/`ends`：真字节闭区间，且分片必须显式给**。落盘恰为 `ends - begins + 1`（`[1000,1999]`→1000 字节；整片 5 MB→5242880 字节）。**反例**：不传区间时 PUT 发的是**整个源文件**（11 MB 打单片 → 后端 `LimitReader` 读到 **5242881** 字节报「分片大小不合法」，客户端挂到超时）。
    - **④ 后台：行为已实测（模拟器可得），保活边界未测**。切后台后分片持续推进；并做了**「上传途中」锁屏**（宿主 hdc `power-shell suspend`）实验：锁屏后 part 2–5 仍逐片完成、`complete` 成功、成品 sha256 MATCH（30 MB/6 片，锁屏于 t≈3.0 s，收口 t≈16.4 s）。**两个坑**：切后台后**进程可能被回收**（对照中出现切后台即被 WMS 销毁、后台零进展）；模拟器 `moveAbilityToBackground()` 可能直接返回 `16000065`（仅前台可调）。**锁屏 + 切后台 5 分钟的长时任务保活属真机项，模拟器不作数**（见 plan §6.1 第二条）。
    - **B 路线**：`extraData` 传 `ArrayBuffer` + `Content-Type: application/octet-stream` 自切 5 MB 片 PUT（`expectDataType` 控制的是响应类型，取 `STRING` 以解析统一包裹）；9 MB 两片均 200、`complete` 返回 `file_id`、成品 sha256 宿主机复核 MATCH。代价是失去系统级后台传输、需自管切片/重试/进度。
    - **端到端硬判据**：A/B 两条路线的 `POST /upload/{id}/complete` 均返回 `file_id`，即**服务端把合并结果算出的 SHA-256 与端侧 `file_hash` 比对通过**。
  - 落地约束（已写入 plan §6.1，供 HMY-51 直接照做）：已验证可用的 `files[].uri` 是 `internal://cache/<相对 cacheDir 路径>`，**绝对 `file://` 形态报 `401 GetInternalPath failed`**；picker 返回的媒体库 URI 是否可直接投递**未测**（见 plan §6.1 待验项）；`config.index` 是 `files` 数组下标（单文件必须为 `0`），**不是分片序号**；分片必须**显式给 `begins`/`ends` 且用 `PUT`**；**切勿在循环里密集发起上传任务**（2026-10-07 实测 100 次齐发触发 `appfreeze THREAD_BLOCK_6S`），必须限并发（建议 ≤2）或严格串行——本轮全部链路串行、全程无冻结。
  - 未覆盖：真机锁屏/长时任务保活边界、真机吞吐与弱网重试——均待真机；模拟器仅为 API 26 x86 环境。

- [x] M4-HMY-51 投稿上传链路：选片、分片上传、断点续传、秒传、进度可见（2026-10-08）
  - 覆盖：HMY-50、HMY-54
  - 实现要点（HMY-50 已定论：走 A 路线 `request.uploadFile`，见上条）：
    - `@ohos.file.picker` 选片取 `uri`；分片用 `@ohos.request` 的 `request.uploadFile`（`method:'PUT'` + `begins`/`ends` 字节区间 + `files[].uri` 取 `internal://cache/…` 形态），**串行或限并发 ≤2**。**分片务必显式给区间**：不给区间时 PUT 发的是整个源文件而非单片，大文件会撑爆连接并卡住客户端（HMY-50 实测）
    - 端侧算 SHA-256 → `POST /upload/init`；`fast=true` 直接拿 `file_id`；否则按 `chunk_size` 切片依 `uploaded` 数组**只传缺失分片**。注意后端 `Complete` 先校验**已传分片数 == `chunk_count`**，不满足直接返回 `ErrUploadIncomplete`（不会先合并再判），故客户端必须在补齐全部缺失分片后再收口
    - 进度用 `on('progress')` 或按分片计数上报；`POST /upload/{id}/complete` 收口
    - 新增 `service/UploadApi.ets` 与 `model/Upload.ets`（手写模型，字段以 OpenAPI 为准）
    - 上传前校验：扩展名白名单（mp4/mov/mkv/flv/avi）、单文件 ≤8GB、剩余空间与网络类型提示（HMY-54）
  - 落地物：`model/Upload.ets`、`service/UploadApi.ets`、`service/UploadController.ets`、`service/BackgroundTransfer.ets`、`common/utils/MediaLib.ets`、`store/AppContext.ets`、`pages/upload/UploadPage.ets`；模块侧加 `KEEP_BACKGROUND_RUNNING` 权限与 `backgroundModes:["dataTransfer"]`；「我的」页加投稿入口
  - 实现中新增的三个实测坑（已写入 [plan §6.2](/specs/harmony/plan)）：
    - **`TaskState.responseCode` 成功时为 `0` 而非 200**——首轮按 `!== 200` 判定，导致每个**成功**分片都被判失败、三轮重试后误报「还有 2 个分片未上传成功」
    - **进度必须每次回调新建对象**——`@Local` 按引用比对，回传同一个 `view` 会使 UI 停在首个值（实测现象：进度条卡在 10%、阶段一直「正在校验文件」，而日志里上传早已完成）
    - **字节进度不能按 `片数 × chunk_size` 估算**——末尾片不满时 7MB 文件会算出 **142%**；改逐片按真实区间累加
  - 验证结论（模拟器 API 26 `Pura X View`，**判定不读端侧自述**）：
    - **端到端硬判据 MATCH**：7MB / 11MB / 30MB 三份由**宿主机对合并成品重算 SHA-256**，与文件名（即端侧上报的 `file_hash`）逐字节比对，**全部 MATCH**；`complete` 均返回 `file_id`，DB `upload_file` 各登记一条
    - **断点续传 PASS**：手工先传 part0（`responseCode=0`）后重新 init 得 `uploaded=[0]`/`resumed=true`，控制器**只传 2 片**（uploadedParts 1→3），**未重传已传分片**，收口通过
    - **秒传 PASS**：同用户重复上传同内容 → `fast=true`、`file_id` 与首次相同、**上传分片数 0**
    - **进度可见 PASS（HMY-50 ③）**：30MB/6 片的百分比档位为 `16,33,50,66,83,100`——连续推进而非 0/100 两段跳
    - **选片交接定论**：picker URI（`file://docs/…`）**直投被拒**（`401 … user file can only for request.agent.`），**拷进 cache 后投递被接受**（`rc=0`）——故拷贝是必做步骤（plan §6.1 首条待验项就此收口）
    - **投稿接口接受 `file_id`**：`POST /api/v1/videos` 通过，返回 `bvid DV2VivO0Xq7CC`（`status=2` 转码中，已下发签名流）
    - **真 UI 走查**：「我的」→「投稿」→ 选片 → 开始上传，阶段依次出现「正在校验文件 → 正在上传 → 正在合并文件 → 上传完成」并显示 `file_id`
  - 未覆盖：**真机待验**——长时任务的实际保活边界（锁屏 + 切后台 5 分钟）、真机吞吐与弱网重试、GB 级文件端侧哈希耗时与 cache 占用。模拟器为 API 26 x86 环境，保活结论不作数

- [x] M4-HMY-52 稿件信息与封面：标题/简介/分区/标签、封面选图或截帧 + 16:9 裁切
  - 覆盖：HMY-51
  - 实现要点：分区取 `GET /api/v1/categories`；封面走 `@ohos.multimedia.image` 解码 + `PixelMap` 裁切 16:9，`multipart/form-data` 传 `POST /videos/cover`（字段 `file`，≤5MB，jpg/png/webp）；**multipart 由 `HttpClient.postRaw` 自拼信封发送**（HMY-50 曾判「复用 `request.uploadFile` 的 `POST` 分支、无需扩展 `HttpClient`」——发得出去成立，但该 API **不回响应体**（`TaskState.message` 实测空串），拿不到 `data.cover`，故封面改走裸 body 通道，详见 [plan §6.3](/specs/harmony/plan) 坑②）；表单校验对齐 Web `UploadView`（标题 ≤80、简介 ≤2000、标签 1~10）
  - 验证结论（2026-10-08 模拟器 API 26 实测，`feature/m4-hmy-52-cover-form`）：
    - **端到端 PASS**：选片（`hmy52-4x3.mp4` 4:3 与 `hmy52-video.mp4` 16:9 各走一遍；秒传与真传各一次）→ 开始上传 → 表单（标题 `9/80`、分区 `动画`、标签 `1/10`、类型 自制）→ 封面自动截帧 → 立即投稿 → **`投稿成功 稿件号 DV2Vj4s7v69vE`**；后端 `video` 行 `status=3`（审核中）、`category_id=1`、`cover=http://localhost:8000/static/covers/2108055781461987328_1791443006894.jpg`，`curl` 该地址 **200 / image/jpeg / 60569B**
    - **16:9 裁切 PASS（宿主解析落盘 JPEG 实际像素）**：4:3 源（352×288）→ 端侧显示 `已按 16:9 裁切 · 352×198`，服务端封面实测 **352×198**；16:9 源 → `640×360`，服务端封面实测 **640×360**——两组比值均 **1.7778**，即端侧裁的就是服务端收下的那张，没被二次裁掉主体
    - **投稿可见 PASS**：重进「我的」→ 投稿档首位 `hmy52-video | 0 · dli_57983775 · 1分钟前`，**缩略图正是端侧截帧的那一帧**（无封面的旧稿件显示默认占位图）——截帧 → multipart 上传 → 提交 → 列表展示全链路闭合
    - **校验分支 PASS（4/4，均端侧拦下）**：分区未选→`请选择分区`；标签为空→`请至少添加 1 个标签`；标签超 10→`最多 10 个标签（当前 10 个）`；标题超长→`标题不能超过 80 个字（当前 90 个）`。提示紧贴提交按钮上方（放页面顶部时用户在页尾看不见）；**4 次失败提交期间 `video` 行数保持 1，成功提交后才 1→2**（旁证「不发请求」，因无法读取运行中后端 stdout）
    - **取消不脏状态 PASS**：相册选择器按 BACK → 封面保持「未设置封面」，无异常
    - **表单字段落库 PASS（第二条投稿 `DV2Vj5fiQ9xIW`）**：DB 行 `title=hmy52-video`、`description=HMY52 desc 16:9 cover`、`copyright=2`（转载）、`tags=["E2E2"]`；首条投稿的中文标签存的是**真 UTF-8**（`["E2E封面"]`，`hex=E5B081E99DA2`——`mysql` 默认字符集下显示 `??` 只是 CLI 显示问题）——标题/简介/分区/类型/标签五项与封面 URL 全部原样落库
    - **实测修正（ArkUI 刷新）**：`@Builder` 的**值类型参数不参与刷新**——计数当参数传入带参 Builder 时，加满 10 个标签计数仍显示 `0/10`；动态文案改到无参 `@Builder` 内直读 `@Local` 后实时正确（详见 plan §6.3 坑①）
  - 未覆盖：真机；**相册选图的「选中→裁切」段**——模拟器图库为空（`所有图片` 无内容、`拍照` 无相机应用），只验到「选择器可打开、取消不脏状态」；该路径与已验的截帧路径共用 `ImageCropper.fromUri`（同为 fd 解码），残余风险在 picker uri 的临时授权 `fileIO.openSync` 一段

- [ ] M4-HMY-53 多P 与草稿：多分P 管理、投稿中退出可恢复
  - 覆盖：HMY-52
  - 实现要点：多P 上限 10（对齐 Web 与 `SubmitReq.parts`）；草稿存 `preferences`（标题/简介/分区/标签/各P 的 file_id/封面路径）；**重进时先 `GET /upload/{id}` 校验会话有效性**（Redis TTL 24h），失效则提示重选文件
  - 验证结论：待补

- [x] M4-HMY-54 提交与状态回看：提交投稿、我的投稿状态与驳回原因（2026-10-09 完成）
  - 覆盖：HMY-53
  - 实现要点：`POST /videos` 提交；「我的投稿」（M4-HMY-09 的 `ProfileContent` 投稿档）展示 `status`（0 草稿/1 上传中/2 转码中/3 待审核/4 已发布/5 已驳回）并对已驳回展示 `reject_reason`；下拉刷新取最新状态
  - 实现提示：`GET /videos/mine` 已返回 `{list,total}`，`MineResult` 已在 M4-HMY-09 落地，本任务多数为展示层改造
  - **前置纠正（本任务开工时发现，属文档与实现不一致）**：原 `plan.md` 接口表声称 `/videos/mine` 返回「含 `status` 与 `reject_reason`」，**实测为假**——`Card` DTO（`model.go`）无该字段，唯一带 `reject_reason` 的 `Detail` 只对已发布稿件开放（`PublicDetail` 要求 `StatusPublished`，已驳回稿件返回 `404 10005`），DB 里 `video.reject_reason` **有值但任何端侧可达接口都取不到**。故本任务**含一处后端改动**（原计划的「纯展示层改造」不成立）。
  - **后端改动（`server/internal/module/video`，随本任务一并交付）**：
    - `Card` 增 `RejectReason string json:"reject_reason,omitempty"`；
    - **只在 `Service.Mine` 内按下标回填**，不放进共用的 `card()`——`cards()` 有 8 处调用点（含 `PublicList`（首页/分区/个人空间）、`Search`），放进去会让**公开列表一并带出驳回原因**；
    - 回填与 `detail()` 均加 **`status == StatusRejected`** 判定。这是必需的：审核通过只写 `status`/`published_at`，**不清空 `video.reject_reason` 列**，故重新通过后该列仍留历史值。
  - **顺带修复一处既有缺陷（非本任务引入）**：`detail()` 原先只看 `RejectReason != nil`，导致**公开详情页 `GET /videos/{bvid}` 对已重新通过的稿件向所有观众泄露陈旧驳回原因**。已同源修正。修复前后实测：修复前 `status=4` 时详情页仍返回该字段，修复后不返回。
  - **端侧实现（`apps/harmony`）**：
    - 新增 `components/MineVideoCardItem.ets`：状态胶囊（6 态文案 + 语义配色；2 转码中另给「完成后自动进入审核」说明）+ 驳回原因区块（浅红底 + 「驳回原因」标签，非裸文本）；**按「字段可能不存在」防御性渲染**（区块以 `status === 5` 为门槛，字段缺失时回落通用文案，绝不渲染空白块）；
    - `ProfilePage.ets`：投稿档改用该卡片；非已发布态（`status !== 4`）**入口拦截不可点进播放页**（公开详情对非发布态返回 404，与 Web 端 `MineVideosView` 同口径）。
  - **修复 `LazyForEach` 键导致的刷新不更新（本任务实测发现）**：`feed()` 的 key 原为 `item.bvid`，稿件状态变化时 key 不变 → ArkUI 复用缓存组件、界面停在旧状态（下拉刷新已重新拉取成功但 UI 不动）。改为 `` `${item.bvid}:${item.status}` ``。**同类问题在本项目已有先例**（互动栏点赞数：key 需含 `like_cnt`）。
  - **验证结论（2026-10-09 实测，API 26 模拟器 `Pura X View` + 宿主后端 `:8000`）**：
    - **三态同屏取证**（`.dev-logs/hmy54/final-three-states.jpeg`）：转码中(2) / 已驳回(5) / 已发布(4) 三种胶囊与说明各按其语义渲染；驳回态正文为真实中文「封面含违规内容，请更换后重新投稿」。
    - **状态守卫有判别用例**：该截图中第三张稿件 DB 列为 `status=4` **但 `reject_reason` 有 16 字符历史值**，界面**未**渲染原因区块 → 证明守卫是「状态驱动」而非「列非空」。
    - **接口层证据**：`/videos/mine` 三个测试稿中**仅 `status=5` 的那条**返回 `reject_reason`；原始响应 1411 字节内 `reject_reason` 字面出现 **0 次**（全部非驳回态下键被 `omitempty` 整体省略，非 `null` 非 `""`）。
    - **无泄露回归**：首页公开列表与搜索接口 0 条带该字段；已发布态公开详情页不返回该字段。
    - **下拉刷新**：改库后下拉，卡片从「已驳回 + 原因区块」正确变为「已发布」且原因区块消失（`.dev-logs/hmy54/v9-refresh.jpeg`）。**注意**：`uitest uiInput drag` 需作用于 `Refresh` 区域（约 y=655–2148）内；起点落在区域外时手势不触发刷新（子会话曾因此误判为「刷新失效」）。
  - **方法学备注（踩坑）**：用 `docker exec mysql -e "update…中文…"` 注入测试数据会把中文**双重编码成 mojibake**（HEX 呈 `C3A5…` 而非 `E5B0…`），界面如实显示乱码——**这是注入侧编码问题、不是端侧缺陷**。正确做法：经 stdin 传 SQL 并带 `--default-character-set=utf8mb4`。测试库中的 mojibake 值已用正确 UTF-8 覆写。
  - 未覆盖：**真机待验**（API 26 x86 模拟器结论）；`status=0 草稿 / 1 上传中 / 6 已锁定` 三态未逐一截图（0/1 在本期链路中不可自然产生，6 不在本任务范围，走通用兜底文案）；驳回原因的**多行长文本截断**（`maxLines(4)`）观感未逐长度核对。
  - 残留测试数据：测试稿 3 条（`DV2VkUdFnDaKG` / `DV2VkUdF8cwj2` / `DV2VkS2hKfT1s`，均为 `dli_05071407` 名下）；模拟器内的视频样本与 `hdc file send` 残留。

## 进度

| 里程碑 | 任务数 | 已完成 |
| --- | :-: | :-: |
| M4 观看端 | 11 | 11 |
| M4 追加·创作端 | 5 | 4 |
| **合计** | **16** | **15** |

> 勾选任务后同步更新上表与 [开发进度管理](/project/progress) 的模块矩阵。
