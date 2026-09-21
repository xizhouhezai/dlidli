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
- [x] M4-HMY-02 工程骨架：DevEco 工程入库 `apps/harmony` + `pnpm-workspace.yaml` 排除该目录 + 分层目录 + 品牌色/圆角 ArkTS 常量 + HarmonyOS Symbol 图标接入 + 接口类型来源定案 + `compatibleSdkVersion` 取 26 + **应用壳层搭在 `Navigation` + `Tabs(BottomTabBarStyle)` 上**（沉浸光感的唯一合法作用面，见 [plan §2](/specs/harmony/plan)）+ `module.json5` 配 `ohos.arkui.UIMaterial.state`（2026-09-21 完成）
  - 覆盖：—（工程）
  - 实现要点：`bundleName` `com.dlidli.app` / `vendor` `DliDli`；`targetSdkVersion` 与 `compatibleSdkVersion` 均 `26.0.0`（产物 `targetAPIVersion` = `minAPIVersion` = `260000026`）。壳层 `pages/Index.ets` = `Navigation`（标题栏材质 `ImmersiveStyle.ULTRA_THIN` + `interactive`）+ `Tabs(barPosition: BarPosition.End)`（`.barFloatingStyle()` 挂 `THIN` 材质 + `maskColor`/`maskHeight` 蒙层），首页标题栏内嵌搜索入口（对齐官方「一镜到底」搜索）；三页签 首页/搜索/我的（HMY-40/41/42）。材质统一经 `common/constants/MaterialTokens.ets` 工厂产出，内部以 `uiMaterial.isImmersiveMaterialSupported()` 判支持性，不支持时返回 `undefined` **自然降级**，`DliMaterial.off()` 暴露 `Material.empty` 语义（与传 `undefined` 的「恢复默认」区分）。品牌 token 落两处：`common/constants/Theme.ets`（字面量，供 Canvas 绘制消费）+ `resources/{base,dark}/element/color.json`（含深色变体，供声明式 UI 消费）；新增字符串资源与 `common/utils/Logger.ets`（hilog 封装）
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
- [ ] M4-HMY-05 发现：首页信息流（LazyForEach 分页 + 下拉刷新 + 触底加载）、分区导航与最新/最热、搜索（含排序筛选与历史同步）
  - 覆盖：HMY-40、HMY-41
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

## 进度

| 里程碑 | 任务数 | 已完成 |
| --- | :-: | :-: |
| M4 | 10 | 3 |
| **合计** | **10** | **3** |

> 勾选任务后同步更新上表与 [开发进度管理](/project/progress) 的模块矩阵。
