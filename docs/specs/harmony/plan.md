# plan：原生鸿蒙端（HarmonyOS NEXT）

> 对应规格：[spec](/specs/harmony/spec) ｜ 技术基线：[后端架构](/architecture/backend) · [数据模型](/architecture/data-model) · [前端架构](/architecture/frontend)
> plan 只写模块级方案与差异；全局分层/规范/部署以架构文档为准。
> 选型 ADR：[ADR-M4-APP-01 鸿蒙端技术选型](/architecture/adr-m4-app-01-harmony)

## 1. 方案概览

鸿蒙端为 **ArkTS + ArkUI 声明式**（Stage 模型）原生应用，DevEco 工程置于本仓 `apps/harmony`，构建链为 hvigor + ohpm（与 pnpm 互不干扰）。

**本期全部复用后端既有能力，后端零改动**：登录、视频详情与签名播放地址、弹幕分段与 WS、互动计数、搜索、空间/历史均走现有 `/api/v1/*` 与 WS 通道。

前端侧**不复用任何 TS 包**（`packages/player` 依赖 hls.js 在鸿蒙无 MSE 环境不可用；`packages/api-client`/`shared`/`ui` 为 TS 与 Element Plus，ArkTS 无法直接消费）。可复用的资产是：各业务 spec 的端无关需求、后端 OpenAPI 契约、品牌 token 与色板/圆角（导出为 ArkTS 常量）、以及 Web 端弹幕轨道分配等**逻辑思路**（需按 ArkTS 重写）。图标不沿用 Web 端方案，改用系统 **HarmonyOS Symbol**（见 §7.1）。

```
apps/harmony/
├── AppScope/                # 应用级配置（app.json5、应用级分层图标资源）
├── entry/                   # 主 HAP 模块
│   └── src/main/
│       ├── ets/
│       │   ├── common/      # 跨层共用
│       │   │   ├── constants/  # 品牌 token（Theme）、沉浸光感材质工厂（MaterialTokens）
│       │   │   └── utils/      # 日志（Logger）等
│       │   ├── pages/       # 页面；壳层 Index.ets 居此，业务页按域分子目录（home/search/profile/…）
│       │   ├── components/  # 公用组件（视频卡片/弹幕层/互动栏/评论）
│       │   ├── viewmodel/   # 页面状态与业务编排
│       │   ├── service/     # 网络层（HTTP/WS）+ 各域 API
│       │   ├── model/       # 端侧接口模型（手写并与后端 DTO 对齐，见 §6）
│       │   ├── media/       # AVPlayer 封装（对应 Web 的 packages/player 角色）
│       │   ├── danmaku/     # 弹幕引擎（轨道分配/渲染/队列）
│       │   └── store/       # 会话与偏好（preferences 封装）
│       └── resources/       # 字符串/颜色资源（base + dark 双份色板，品牌 token 落地点）+ media 分层图标与启动图标
├── oh-package.json5         # ohpm 依赖
├── build-profile.json5      # 构建与签名配置
└── hvigorfile.ts            # 构建脚本
```

## 2. 技术决策

| 决策点 | 决策 | 理由 | 备选方案 |
| --- | --- | --- | --- |
| 端侧技术栈 | ArkTS + ArkUI 声明式（Stage 模型） | 原生体验与性能优先；鸿蒙官方长期路线，系统能力（媒体/分享/深色模式）接入最完整 | uni-app `app-harmony` 复用（否：`packages/player` 的 hls.js 不可用、弹幕渲染与 UI 需重写，实际复用率低）；方舟开发框架类 Web 范式（否：受限且非推荐路线） |
| 工程形态 | DevEco 工程放本仓 `apps/harmony`，**从 pnpm workspace 排除** | 与 specs/后端契约同仓便于对齐；hvigor/ohpm 与 pnpm 是两套构建体系，不能被 pnpm 当 workspace 包 | 独立仓库（否：跨仓对齐成本高） |
| 播放器 | 系统 `AVPlayer`（`@ohos.multimedia.media`） | 系统原生支持 HLS/m3u8，无需自带解封装；硬解与功耗最优 | hls.js（否：依赖 MSE，鸿蒙不可用）；第三方 ohpm 播放器（待评估，成熟度不明） |
| 弹幕渲染 | ArkUI Canvas（CanvasRenderingContext2D）绘制，必要时降级为 XComponent + native drawing | 与 Web 端 Canvas 轨道分配逻辑同构，便于移植；单节点绘制避免大量 Text 组件 | 逐条 Text/Stack 组件（否：节点数随弹幕量线性增长，性能不可接受） |
| 网络层 | `@ohos.net.http`（或 RCP）封装 + 统一响应/错误码处理 | 官方能力，支持超时/证书/拦截；与 Web 的 api-client 职责对齐但独立实现 | 移植 `packages/api-client`（否：ArkTS 不能直接消费 TS 包，且依赖 fetch 语义） |
| 接口类型 | **按 `server/docs` 的 OpenAPI 手写端侧模型**，字段名与可空性逐条核对 | 生成路线实测不可行（响应 schema 全为无类型的统一包裹，见 §6）；手写 + 契约核对是拿到响应模型的唯一路径 | OpenAPI 生成 ArkTS 类型（否：swag 产出无响应模型，见 §6）；引第三方 ArkTS 生成器（否：无合适产物） |
| 状态管理 | ArkUI 状态管理 V2（`@ComponentV2`/`@Local`/`@Param`/`@ObservedV2`），预研确认后定版 | 声明式数据流与 Web 端 Pinia 心智接近；V1 装饰器在复杂页面下更易产生不必要刷新 | V1（`@State`/`@Observed`）：作为备选 |
| 长连接 | `@ohos.net.webSocket` | 承接弹幕实时下发（DM-15）；与 Web 端 WS 协议一致（token 鉴权 + 房间订阅） | 轮询（否：延迟高，仅作 WS 失败时的回退，符合 DM-15 语义） |
| 本地存储 | `@ohos.data.preferences` | 令牌、弹幕设置、进度缓存均为轻量 KV | 关系型 Store（否：本期无本地表需求） |
| 列表性能 | `LazyForEach` + 分页游标 | 首页/搜索/历史均为长列表，避免一次性构建 | `ForEach`（否：大列表卡顿） |
| 与后端关系 | **本期零后端改动**，baseUrl 走配置 | 现有 REST/WS 契约已满足观看端全部需求 | 为端侧新增聚合接口（否：无必要，避免契约分叉） |
| 目标 API 版本 | **`compileSdkVersion`/`compatibleSdkVersion` 均取 26**（`7.0.0(26)`） | 本机 DevEco 26.0.0.821 内置 SDK 为 HarmonyOS 26.0.0(API 26)，模拟器镜像 `HarmonyOS-7.0.0/phone_all_x86`（apiVersion 26 / 7.0.0.106）齐备；本期口径为仅本地验证，全量取用系统材质最省事 | 兼容旧设备（否：本期不提审、无覆盖诉求；**若后续要上架，需下调 `compatibleSdkVersion` 并用 `uiMaterial.isImmersiveMaterialSupported()` 做能力分级**） |
| 应用壳层结构 | **必须建立在 `Navigation` + `Tabs(BottomTabBarStyle)` 之上** | 沉浸光感的官方作用面硬约束：除 `Toggle`/`Select`/弹窗类外，仅 `Navigation`/`NavDestination` 标题栏与 `barPosition: BarPosition.End` 的底部 TabBar 生效。壳层若自绘，沉浸光感将无处挂载，且后期改造代价高 | 自定义顶栏/底栏（否：直接丧失系统材质能力） |
| 沉浸光感实现路线 | **原生 ArkUI 系统材质**：`uiMaterial.ImmersiveMaterial` + `.systemMaterial()` 通用属性，`module.json5` 配 `ohos.arkui.UIMaterial.state` | API 26 已就位，官方一等能力，含 `colorInvert`/`interactive`（交互形变）/`lightEffect`（感光反馈）等完整配置 | HDS `hdsMaterial` 迂回（否：仅作旧设备/低 API 的降级路径保留）；自绘光效（否：非系统级，功耗与一致性差） |

## 3. 数据模型

端侧无业务数据库（不建表），仅本地偏好存储：

| Key | 类型 | 说明 |
| --- | --- | --- |
| `auth.access_token` | string | 访问令牌（2h，承 ACC-06） |
| `auth.refresh_token` | string | 刷新凭证（30d，可吊销） |
| `danmaku.settings` | JSON | 不透明度/字号/显示区域/速度/开关（承 DM-11、DM-12） |
| `playback.local_progress` | JSON | 本地进度缓存，服务端进度为准（承 PLY-04） |
| `search.local_history` | JSON | 本地搜索历史副本，与云端同步（承 SRH-04） |

> 服务端数据模型无变更，见 [数据模型设计](/architecture/data-model)。

## 4. 接口设计

**不新增接口**，本期消费以下既有后端能力（事实来源为 `server/docs` 的 OpenAPI，本地 `http://localhost:8000/swagger/index.html`）：

| 域 | 用途 | 对应既有接口组 |
| --- | --- | --- |
| auth | 验证码/密码登录、刷新、退出 | `/api/v1/auth/*` |
| video | 稿件详情、分区列表、观看历史 | `/api/v1/videos/*` |
| danmaku | 分段拉取、发送 | `/api/v1/danmaku/*` |
| interaction | 点赞/投币/收藏/三连、评论 | `/api/v1/interactions/*`、`/api/v1/comments/*` |
| relation | UP 主资料与关注态 | `/api/v1/relation/*` |
| search | 综合搜索 | `/api/v1/search/*` |
| WS | 弹幕实时下发 | 既有 comet WS 通道 |

> 模块级接口清单待预研阶段以 OpenAPI 逐条核对后补全（见 [tasks](/specs/harmony/tasks) M4-HMY-02）。

## 5. 关键流程

**播放链路**（承 PLY-01/04/05/08）：

```
进入详情页 → GET 稿件详情（含签名 m3u8 URL，TTL 6h）
  → AVPlayer.prepare/play → 播放中每 10s 上报进度（节流）
  → 时长 > 5s 上报有效播放（服务端去重）
  → 签名临期/过期 → 静默重取地址 → 保留进度换源续播
  → 切后台 → 暂停并保进度；超时释放 AVPlayer 资源
```

**弹幕链路**（承 DM-10/11/15/20/21）：

```
进入播放页 → 按 6min 分段拉取当前段 + 预取下一段 → 入渲染队列（按 time_ms 排序）
  → Canvas 逐帧绘制（轨道分配 → 碰撞检测 → 移出回收）
  → 订阅 WS 房间 → 实时弹幕插入队列 → 命中屏蔽词/UID → 不入队
  → WS 断开 → 指数退避重连 → 重连失败回退 HTTP 分段轮询
  → 发送：校验等级门槛（本地提示 + 服务端校验）→ 频控 → 上屏
```

**登录与会话**（承 ACC-01/03/06）：

```
登录页 → 验证码或密码 → 存 token 至 preferences
  → 请求拦截器注入 Authorization → 401 → 用 refresh 静默续期并重放原请求
  → 续期失败 → 清凭证 → 跳登录页
```

**三连**（承 ITR-30）：长按点赞 1.5s → 并行发起点赞/投币（默认 2 枚，不足则 1 枚）/收藏 → 幂等键防重 → 原生动画反馈。

## 6. 风险与待定项

- [ ] **AVPlayer 播后端 HLS 兼容性**（最高风险）：需实测 ffmpeg 产出的 m3u8/ts 切片与签名 URL 组合；本机仅有鸿蒙模拟器，模拟器视频硬解能力受限，**播不了不得判定为不支持**，须真机复验。
- [x] **模拟器可用性（2026-09-21：已澄清，非宿主故障）**：早前 3 次尝试均在**冷启窗口期内**操作——`hidumper` 窗口数恒 0、`snapshot_display` 恒返回同一张 47KB 全黑帧、`aa start` 报 `10106102 … device screen is locked … developer mode`，并遇 2 次 VM 退出，一度误判为宿主 OpenGL/WGL 故障（`qemu.log` 的 `gl error 502` / `wglMakeCurrent` 失败属启动期现象）。**当晚重试完全可用**：未签名 HAP 安装、启动、渲染、切页签均正常。
  - **操作口径**：先确认 SystemUI 就绪（`com.ohos.sceneboard` 进 FOREGROUND / `hidumper -s WindowManagerService` 窗口数 > 0）再安装与启动；不在启动窗口期反复 `snapshot_display`。命令速查见 [tasks M4-HMY-01](/specs/harmony/tasks)。
  - **附带结论**：模拟器为 **API 26 / guest `7.0.0.106(SP1DEVC00E999R4P11)` / abi `x86_64`**；**未签名 HAP 可直接 `hdc install` 成功**——本机本地验证**无需配置 `signingConfigs`**，可砍掉签名前置。
  - **仍未验**：沉浸光感是否真正生效（浅色纯色底无可比对参照，且 `isImmersiveMaterialSupported()` / `getGlobalMaterialLevel()` 取值未取），归 M4-HMY-01 能力探测。
- [ ] **签名 URL 的请求头约束**：若 AVPlayer 无法附加 Referer 等校验头，需确认后端 `PlaySignGuard` 对端侧放行的边界（现有实现放行 `.ts` 分片，`.m3u8` 需签名）。
- [ ] **弹幕 Canvas 性能上限**：同屏弹幕满载下的帧率与内存需实测（spec §4 要求 ≥ 55fps）。
- [x] **状态管理版本（已解决，2026-09-21）**：壳层 `pages/Index.ets` 以 **ArkUI 状态管理 V2**（`@Entry @ComponentV2` / `@Local` / `@Param`）实写并通过 API 26 编译（`assembleHap` BUILD SUCCESSFUL），**定版 V2**，不再保留 V1 备选。
- [x] **目标 API 版本选择（已解决，2026-09-21）**：DevEco 升级至 **26.0.0.821** 后内置 SDK 为 **HarmonyOS 26.0.0 / API 26**（version 26.0.0.105），模拟器镜像 **API 26 / 7.0.0.106**（`D:/Program Files/Huawei/sdk/system-image/HarmonyOS-7.0.0/phone_all_x86`）已就位，实例含 Mate 70 Pro / Mate 80 Pro Max / Pura 90 / Pura X View。**`compileSdkVersion` 与 `compatibleSdkVersion` 均取 26**，原"编译 24 / 运行时 23"的双口径已失效，历史结论仅备查。
- [ ] **沉浸光感落地（API 26 已就位）**：主路线改为**原生系统材质**——`uiMaterial.ImmersiveMaterial` + `.systemMaterial()` 通用属性（`CommonMethod`，`@since 26.0.0`），挂载点限于 `Navigation` 标题栏（`titleOptions.systemMaterial`）与 `Tabs` 的 `BottomTabBarStyle`（`systemMaterial`），另有 sheet/popup/alert/actionSheet 等弹窗类；应用级开关走 `module.json5` 的 `metadata` → `ohos.arkui.UIMaterial.state`（`default`/`enable`/`disable`，仅 entry 模块生效）。
  - **能力探测**：`uiMaterial.isImmersiveMaterialSupported()` 判支持性；`uiMaterial.getMaterialInfo()` 取应用级 `MaterialState`；`uiMaterial.getGlobalMaterialLevel()` 取全局材质档。
  - **配置项**：`ImmersiveOptions { style, materialColor, colorInvert, applyShadow, interactive, lightEffect }`。`style` 取 `ImmersiveStyle.{ULTRA_THIN|THIN|REGULAR|THICK|ULTRA_THICK}`，按官方场景规范：顶部悬浮 `ULTRA_THIN`、底部悬浮 `THIN`、任意弹出 `THICK`、半模态/弹窗 `ULTRA_THICK`。`lightEffect` 为感光交互反馈（可配色），`interactive` 为交互形变。
  - **必须尊重系统分层**：`style`/`materialColor`/`colorInvert` 仅在高/中算力设备生效（低算力设备退化为影响背景色/边框/阴影），**不得假设效果恒定**；用户三档强度（强/均衡/弱）由系统自动映射，开发只定义一次。
  - **注意**：`systemMaterial` 与 `backgroundColor`/`borderColor`/`borderWidth`/`shadow` 官方不建议并用；关闭某组件光感用 `uiMaterial.Material.empty`（与传 `undefined` 语义不同——后者是恢复默认）。
  - HDS `hdsMaterial`/`HdsVisualComponent`（`@since 6.0.0(20)`/`6.1.0(23)`）退为**降级路径**：仅在需兼容低版本设备时启用。
  - 页面内容区（播放器控制层、评论面板、弹幕面板）系统材质**够不到**，仍用 `backgroundBlurStyle` + HDS 光效 + 自绘 `Particle`。
- [x] **OpenAPI → ArkTS 类型生成（已解决，2026-09-21：判定为不可行，改为手写）**：实测 `server/docs/swagger.json`（swagger 2.0，64 个 path / 82 个响应）：**82 个响应 schema 无一例外全是 `$ref: #/definitions/github_com_dlidli_server_internal_pkg_response.Body`**，而该 `Body` 的 `data` 字段是空 schema `{}`（等价 `any`）；13 个 `definitions` 里只有**请求体**（且多为 admin 域）有结构。
  - **结论**：生成器无料可生——产出只有统一包裹与少量请求体，端侧真正需要的响应模型（稿件详情、弹幕分段、搜索结果…）一个都拿不到。**放弃生成路线，改为手写端侧模型 + 契约核对**。
  - **根因与不可解性**：根因在后端 handler 未标注具体响应类型，修它必须改后端，与本期「零后端改动」冲突。后续若要重开生成，需后端先为响应引入泛型 DTO 标注（如 `response.BodyOf[T]`），届时再评估。
  - **执行口径**：`model/` 目录为手写产出，M4-HMY-03 起按 OpenAPI 逐条核对字段名、类型与可空性，并在端侧联调时以后端真实响应为准。
- [ ] **微信登录**：鸿蒙端无小程序载体，微信开放平台鸿蒙版 SDK 可用性待查证（本期不做）。
- [ ] **CI 影响**：hvigor 构建需 DevEco 环境，CI 是否纳入鸿蒙构建待定（本期可先本地构建）。
- [ ] **应用签名与上架**：本期口径为仅模拟器/真机本地验证，不提审；签名证书与资质材料待上架阶段再办。

## 7. 视觉设计资源与规范依据

**事实来源**：华为开发者联盟[设计中心](https://developer.huawei.com/consumer/cn/design/)与[设计资源库](https://developer.huawei.com/consumer/cn/design/resource/)。下表为**面向本模块可直接取用**的资源与规范，非全量搬运。

### 7.1 可下载资源（设计交付用）

| 资源 | 用途 | 备注 |
| --- | --- | --- |
| 手机/折叠屏/平板/电脑 设计组件库 | 端侧页面与组件稿 | Sketch 33.0MB / PIX 13.1MB，2026-06-12 更新 |
| HarmonyOS Sans 字体 | 端侧字体族 | 与端侧 `fontFamily` 取值对齐 |
| HarmonyOS 应用图标 | 应用图标与启动图标资源 | 配合 `AppScope` 图标配置 |
| 服务卡片 / 实况窗 / 播控中心 / 分享 组件库 | 系统能力对接稿 | 本期仅"分享"相关 |
| 启动页 设计模板 | 启动页视觉 | 对应 HMY-01 冷启动体验 |
| 品牌与产品 / 产品机框 | 机型适配参照 | 折叠屏/平板另见 §7.2 |

> 图标体系**改用 HarmonyOS Symbol**（[harmonyos-symbol](https://developer.huawei.com/consumer/cn/design/harmonyos-symbol)）而非 Web 端的 MingCute：Symbol 具备字体属性、多层颜色与分层动效，端侧以 `SymbolGlyph` 消费，与系统视觉一致（已同步修正 §1 与 §2 的图标口径）。

**应用图标（2026-09-21 定版，自研）**：不采用 DevEco 模板默认图标，也不套用上表可下载的图标资源包，自研品牌图标——品牌粉渐变底（`#FC8BAB` → `#F2557F`，起点即 Web 端既有 `brand_primary_hover` token）+ 白色几何字母 **D**，其内孔挖成圆角播放三角，字母与播放语义合一。分层图标资源落 `AppScope/resources/base/media/` 与 `entry/src/main/resources/base/media/`（`background` 满幅 / `foreground` 白色图形 + 透明底，均 1024×1024，前景占画布 67%），启动图标 `startIcon.png` 152×152 圆角烘焙。同一图形以 `apps/web/public/favicon.svg` 为**自包含矢量源**，Web 端 favicon 由此派生——**全端品牌图形单一来源**，改版时自此 SVG 重新导出各端位图即可。落地与实测记录见 [tasks M4-HMY-02](/specs/harmony/tasks)。

### 7.2 对口设计规范（约束端侧实现）

| 规范 | 关键结论 |
| --- | --- |
| [沉浸光感](https://developer.huawei.com/consumer/cn/doc/design-guides/immersivelight-0000002612101053) | 材质层级枚举从 `ULTRA_THIN` 到 `ULTRA_THICK` 共 5 档；顶部悬浮用 `ULTRA_THIN`（+渐变模糊）、底部悬浮用 `THIN`（+渐变蒙层）、任意位置弹出用 `THICK`、半模态/弹窗用 `ULTRA_THICK`。用户侧三档强度由系统映射，开发只定义一次 |
| [影音娱乐场景最佳实践](https://developer.huawei.com/consumer/cn/doc/design-guides/responsive-design-examples1-0000001957369849) | ① **沉浸全屏播放**：上下有黑边时弹幕**仅在上方黑边区域内**显示；无黑边时同屏弹幕不宜过多（直接约束 HMY-20/HMY-24）② 全屏下选集/倍速等临时操作走**侧边或底部面板**（手机横屏/折叠态/平板从右侧，折叠屏展开态从底部）③ 方形屏（折叠屏展开态）点全屏**不旋转** ④ 折叠屏悬停态自动切沉浸播放 ⑤ 宫格：折叠屏/平板竖屏 3 列、平板横屏 5 列，支持双指缩放调列数 ⑥ 建议支持**画中画**，首次使用调起授权弹窗 ⑦ 播放页全局滑动：右滑出上级瀑布流、左滑出右侧 UP 主详情 ⑧ 搜索框"一镜到底"不切层级 |
| [圆角参数](https://developer.huawei.com/consumer/cn/doc/design-guides/corner-radius-parameter-0000002556468705) | 端侧圆角取值参照系统规范，与 Web 端 `radius-sm/md/lg`（6/8/12）映射关系需在 M4-HMY-02 对齐 |

### 7.3 沉浸光感开发侧文档（API 26 已就位，本期主路线）

- [沉浸光感简介](https://developer.huawei.com/consumer/cn/doc/harmonyos-guides/arkts-immersive-light-sense-overview) / [开启沉浸光感](https://developer.huawei.com/consumer/cn/doc/harmonyos-guides/arkts-immersive-light-sense-enable) / [功耗优化](https://developer.huawei.com/consumer/cn/doc/harmonyos-guides/arkts-immersive-light-sense-constraints) / [组件适配](https://developer.huawei.com/consumer/cn/doc/harmonyos-guides/arkts-immersive-light-sense-component-adaptation) / [沉浸式系统材质视效](https://developer.huawei.com/consumer/cn/doc/harmonyos-guides/arkts-immersive-light-sense-common-capability)（反色、材质赋色、交互形变、点光源、阴影开关）
- [一多开发实例（长视频）](https://developer.huawei.com/consumer/cn/doc/best-practices/multi-video-app)：折叠屏/平板播放页一多布局官方参考实现
