# tasks：原生鸿蒙端（HarmonyOS NEXT）

> 对应规格：[spec](/specs/harmony/spec) ｜ 方案：[plan](/specs/harmony/plan)
> 任务编号：`{阶段}-{模块}-{序号}`（如 M4-HMY-03）；完成即勾选并追加完成日期；括号补注实现要点与验证结论；每条任务必须标注"覆盖"的需求 ID。
> 口径：本期仅模拟器/真机本地验证，**不提交平台审核**（对齐 [M3-MP-03](/specs/engineering/tasks) 小程序的个人主体口径）。

## M4（W49+）观看端

- [ ] M4-HMY-01 预研：原生技术链路可行性（AVPlayer 播后端签名 m3u8；`@ohos.net.webSocket` 连弹幕 WS；`@ohos.net.http` 跑通登录与视频列表；ArkUI 状态管理 V2 写法；沉浸光感在 API 26 模拟器上的实际表现）
  - 覆盖：—（工程；为 HMY-01/03/10/20/22 定实现方案）
  - 前置：DevEco 26.0.0.821 + SDK API 26 + 模拟器镜像 API 26（`D:/Program Files/Huawei/sdk/system-image/HarmonyOS-7.0.0/phone_all_x86`）已就位，实例名 **`Pura X View`**（apiVersion 26.0.0 / guest 7.0.0.106）。模拟器视频硬解受限时播放结论记「待真机确认」，**不得据此判定鸿蒙不支持 HLS**
  - 连设备口径（2026-09-21 实测）：模拟器**不注册为 USB 目标**，只跑 `hdc list targets` 恒为 `[Empty]`；须先 `hdc tconn 127.0.0.1:5555`（模拟器监听 `127.0.0.1:5555`），连上后 `list targets -v` 由 `TCP Offline` 转 Online；未就绪时 shell/install 报 `[E001005] Device not found or connected`。另行拉起模拟器的命令行可从 DevEco `idea.log` 的 `LocalDeviceConnection - start hvd log` 抄取：`Emulator.exe -hvd "<实例名>" -path "D:/Program Files/Huawei/Emulator/deployed" -t <pipe> -imageRoot "D:/Program Files/Huawei/sdk"`；`hdc file recv` 的本地路径须写 Windows 反斜杠形式并在 Git Bash 下关掉路径转换，否则被当相对路径
  - **启动窗口期注意事项（2026-09-21 实测）**：模拟器冷启后需等 **SystemUI（`com.ohos.sceneboard` 进 FOREGROUND / 窗口数 > 0）** 才算就绪；就绪前 `hidumper -s WindowManagerService` 窗口数为 0、`snapshot_display` 恒返回同一张 47KB 全黑帧、`aa start` 报 `10106102 … device screen is locked … developer mode`——这些都是**未就绪的假象，不是宿主渲染故障**。当日早前 3 次尝试即在启动窗口期（2~4 分钟内）操作并遇 VM 退出，一度误判为宿主 OpenGL 不可用；当晚重试则**完全可用**（安装/启动/渲染/切页签均正常）。操作口径：先 `uptime` + `aa dump -a` 确认 sceneboard 就绪，再安装与启动；避免在启动窗口期反复 `snapshot_display`
  - 可用结论（同期实测）：模拟器运行时 **API 26 / guest `7.0.0.106(SP1DEVC00E999R4P11)` / abi `x86_64`**；**未签名 HAP 可直接 `hdc install` 成功**，本地验证无需配置 `signingConfigs`
  - 模拟器操作速查：安装 `hdc -t 127.0.0.1:5555 install -r <hap>`（`<hap>` 务必用**相对路径**：先 `cd` 到产物目录；Git Bash 下传 Windows 绝对路径会被 hdc 拼成 `<当前目录>/D:/...` 而报 `Error opening file`，安装失败后紧接着的 `aa start` 就报 `10104001`，极易误判成包名问题）；启动 `hdc -t … shell "aa start -a EntryAbility -b com.dlidli.app -m entry"`；点击用 `hdc -t … shell "uitest uiInput click <x> <y>"`（`uinput -T -m x y x y` 等长 trace 不触发点击）；取控件实际矩形用 `uitest dumpLayout -p /data/local/tmp/layout.json` 再 `hdc file recv`（本次即以它纠正了 1719→2064 的坐标误判）；截图 `snapshot_display -f <路径>` + `file recv`
  - 视觉预研要点（降级为实测确认，设计侧结论已定稿见 [plan §2/§6](/specs/harmony/plan)）：① `uiMaterial.isImmersiveMaterialSupported()` 在 x86 模拟器返回什么；② `getMaterialInfo()`/`getGlobalMaterialLevel()` 的实际取值（x86 模拟器算力档位可能非高/中档，`style`/`colorInvert` 等可能不生效）；③ 在 `Navigation` 标题栏与 `Tabs` 底部标签栏上挂 `ImmersiveMaterial` 的实际观感与帧率开销；④ 结论若为模拟器不支持，**不得据此判定真机不支持**，记「待真机确认」
  - **预研进度（2026-09-21）**：预研项中的**网络层已随 M4-HMY-03 实测覆盖**——`@ohos.net.http` 走通真实后端（首页分区列表 12 项）、超时/失败重试与 401 续期链路均已验，且模拟器可达宿主机（`10.0.2.2:8000`），结论见 M4-HMY-03；**余 AVPlayer HLS 播放、弹幕 WS 通道、沉浸光感三项待验**（分归 M4-HMY-06、M4-HMY-07 与本节视觉预研）
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
  - **应用图标与 favicon（2026-09-21）**：弃用 DevEco 模板默认图标，自研品牌图标——品牌粉渐变底（`#FC8BAB`→`#F2557F`，起点即既有 `brand_primary_hover` token）+ 白色几何字母 **D**，内孔挖成圆角播放三角（字母与播放语义合一，48/32px 可辨）。分层图标按 HarmonyOS 规范产出：`AppScope/resources/base/media/` 与 `entry/src/main/resources/base/media/` 各一份 `background.png`（满幅渐变，系统负责遮罩）+ `foreground.png`（白色图形、透明底），均 1024×1024，前景图形占画布 **67%**（对齐 DevEco 模板前景安全区实测值）；`startIcon.png` 152×152，圆角已烘焙（对齐模板做法）。Web 端以 `apps/web/public/favicon.svg` 作为该图形的**自包含矢量源**，衍生 `favicon.ico`（16/32/48，PNG 内嵌）与 `apple-touch-icon.png`（180），并接入 `apps/web/index.html` 的 `rel=icon` 与 `theme-color`
  - 验证结论（图标）：`hvigorw assembleHap` BUILD SUCCESSFUL；**模拟器桌面实测图标正确渲染**——系统 squircle 遮罩下品牌粉圆角方 + 白色 D，与系统应用并排无异常；Web 端起 `vite` 实测三个 favicon 资源均 200 且 MIME 正确（`image/svg+xml` / `image/x-icon` / `image/png`），浏览器实渲 SVG 无畸变
  - 未覆盖：**沉浸光感是否真正生效仍未验**——壳层配色为浅色纯色底，材质模糊/蒙层无可比对参照，且 `isImmersiveMaterialSupported()`/`getGlobalMaterialLevel()` 取值未取；三项能力探测与观感/帧率开销归 M4-HMY-01。`viewmodel/`/`service/`/`model/`/`media/`/`danmaku/`/`store/` 目录待各自任务落地时创建，不做空目录占位
  - OpenAPI 生成 ArkTS 类型：**实测判定不可行**（82 个响应 schema 全为无类型的统一包裹 `response.Body`，`data` 为空 schema），改为手写 + 契约核对，依据见 [plan §6](/specs/harmony/plan)
- [x] M4-HMY-03 网络层：HTTP 封装（统一响应包裹/错误码文案/超时与重试/401 静默续期重放）（2026-09-21 完成）
  - 覆盖：HMY-03
  - 实现要点：`service/HttpClient.ets` 为全端唯一 HTTP 出口，职责收敛为四件事——包裹解析、错误码文案、超时与退避重试、401 静默续期重放。配置集中在 `common/constants/ApiConfig.ets`（baseUrl/连接与读取超时/重试次数与退避基数）；文案表 `common/constants/ErrorMessages.ets` 按服务端 `errcode` 的分段规则（1xxxx 通用 / 2xxxx 账号 / …）落表，**服务端 message 存在时优先采用**（同码文案可能带 `WithMsg` 上下文），缺失才回落端侧表与按域兜底；`model/Api.ets` 提供 `ApiBody<T>` 包裹与 `ApiError`（带 `code`/`traceId`/`retryable`），负数段为端侧合成码（离线/超时/响应不合契约）。重试口径：**仅网络类失败与 HTTP 5xx 可重试**（退避 300ms×2ⁿ，共 3 次尝试），业务错误不重试；401 走**单一飞行**续期（并发 401 只发一次 `/auth/refresh`，其余请求复用同一 Promise），续期成功重放一次原请求，续期不可用则清凭证并回「登录已过期，请重新登录」且不给重试入口。`store/TokenStore.ets` 基于 `@ohos.data.preferences` 并以内存镜像供请求同步取用，**存储异常一律降级为「仅内存生效」并记日志**——存储不可用不应让请求链路抛错。配套 `components/ErrorStateView.ets`（可重试失败态）与 `pages/home/HomePage.ets` 三态接通，`module.json5` 补 `INTERNET`/`GET_NETWORK_INFO` 权限，`EntryAbility.onCreate` 注入 Context 并回填凭证
  - **实测抓到并修掉一处**：`send()` 的 catch 原先把**所有**异常都重写为「网络不可用」，把 `parse()` 抛出的业务错误一并吞掉——后果是 401 被降级成网络失败、触发 3 次无意义重试，且**续期分支永远不可达**。改为 `err instanceof ApiError` 时原样上抛（只把 `req.request` 的传输层异常定级为网络类），修后日志链路正确：`status=401 code=10003`（不重试）→ 续期不可用 → 清凭证 → `retryable=false`
  - 验证结论：`hvigorw assembleHap` **BUILD SUCCESSFUL**（ArkTS 零告警）；**模拟器实测三态**——① 成功：首页分区栏拉到后端真实 12 项（动画/游戏/科技数码…）；② 失败可重试：停掉后端 → 连接超时判为网络类（`errCode=2300028 → -2`）→ 退避重试 3 次 → 呈现「加载失败 / 网络超时，请重试」+ 重试按钮，**未白屏**；③ 恢复：后端起回后点「重试」（按 `dumpLayout` 实测矩形 `[549,1158][772,1278]` 取中心点击）→ 重新加载 12 项。**401 分支**用临时把接口指向需鉴权的 `/api/v1/users/me` 验证（验完已还原）：`code=10003` 不重试 → 无 refresh_token → 清凭证 → 文案「登录已过期，请重新登录」且**不出现重试按钮**（业务错误不可重试）
  - **模拟器访问宿主机的口径（本次实测）**：`http://10.0.2.2:8000` 直连宿主 loopback 可用（QEMU 用户态网络），明文 HTTP 未被系统拦截，无需 `hdc fport` 转发；该地址由 `ApiConfig.BASE_URL` 单点维护，真机联调改宿主局域网 IP 即可
  - 未覆盖：401**续期成功后的重放**分支（需真实登录态才能造出有效 refresh_token，归 M4-HMY-04，**已于 2026-09-21 由 M4-HMY-04 实测补齐**）；WS 弹幕通道（M4-HMY-07）
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
    - **首页信息流**：推荐首屏 20 条 + 触底续 6 条后呈现「没有更多了」；分区切换（动画 → 3 条，推荐页签隐去并自动落最新）；最新/最热切换（最热按播放量降序，26.6万 > 24.3万 > 22.8万 逐屏验证）；下拉刷新（日志 7 次 page-1 重载且列表回顶）；封面走 `10.0.2.2` 正常出图、空封面为底色块占位、时长角标 `04:11`/`1:02:05` 两种格式、meta 行 `1.2亿 · 生活UP主05 · 5小时前`（万/亿与相对时间均正确）
    - **搜索**：关键词 `测试` → 视频 tab 20 条 + 「共 26 条」→ 触底续 6 条 + 「没有更多了」；UP 主 tab 同样 20 + 6 且双 Tab 各 26 条；0 结果两种文案（视频/UP 主）与 tab 切换均正常；历史新增（`测试` → `UP主` 后 `UP主` 置顶）、单条删除、清空、**重启后历史仍在**（`aa force-stop` 后重进）；点卡片弹「播放页建设中，敬请期待」（播放页归 M4-HMY-06）
  - 未覆盖：触底加载失败与刷新失败的**重试分支**未做停机实测（沿用 M4-HMY-03 已验证的 `ErrorStateView` 口径）；全部结论均在 API 26 模拟器取得，**真机待验**
- [ ] M4-HMY-06 播放页：AVPlayer HLS 播放、清晰度切换与倍速、进度记忆与跨端续播、有效播放上报、签名过期静默换签、触屏手势与横屏全屏、切后台处理
  - 覆盖：HMY-10、HMY-11、HMY-12、HMY-13、HMY-14、HMY-15
- [ ] M4-HMY-07 弹幕：分段拉取与预取、Canvas 轨道渲染、WS 实时下发与断线重连及 HTTP 回退、关键词/发送者屏蔽、展示设置、发送与频控、列表面板
  - 覆盖：HMY-20、HMY-21、HMY-22、HMY-23、HMY-24
  - 全屏弹幕须对齐[官方影音娱乐规范](/specs/harmony/plan)（§7.2）：上下有黑边时弹幕仅在上方黑边区域内显示；无黑边时限制同屏弹幕密度
- [ ] M4-HMY-08 互动与评论：点赞/投币/收藏/长按三连（幂等 + 原生动画）、评论一级与二级、分享走系统面板
  - 覆盖：HMY-30、HMY-31、HMY-32
- [ ] M4-HMY-09 个人中心：资料与我的投稿、观看历史、收藏、未登录态入口、端侧偏好与进度缓存
  - 覆盖：HMY-42、HMY-04
- [ ] M4-HMY-10 端侧验收：核心链路走查（登录 → 找内容 → 播放 → 弹幕 → 互动 → 个人中心）+ 性能指标测量（冷启动、起播、弹幕帧率、崩溃率、包体积）
  - 覆盖：[spec §4 成功指标](/specs/harmony/spec)
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

## 进度

| 里程碑 | 任务数 | 已完成 |
| --- | :-: | :-: |
| M4 | 11 | 5 |
| **合计** | **11** | **5** |

> 勾选任务后同步更新上表与 [开发进度管理](/project/progress) 的模块矩阵。
