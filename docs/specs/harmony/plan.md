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
| 应用壳层结构 | **`Navigation` 标题栏 + `HdsTabs` 悬浮胶囊底栏**（HDS，`@kit.UIDesignKit`） | 底栏用 HDS 的 `barFloatingStyle`：窄 `barWidth`（328vp）把标签栏收成悬浮胶囊、`barOverlap(true)` 让内容从胶囊下穿过、材质由 `systemMaterialEffect` 统一出——官方样张 `multi-news-read` 的观感，比自绘 `Tabs` + 手配 `maskColor`/`systemMaterial` 更整、更少自管细节 | 自绘底栏（否：材质与蒙层全要自管）；整宽 `Tabs` 底栏（否：铺满整宽的永远是"一条色条"，不是胶囊） |
| 沉浸光感挂载范围 | **不限标题栏/TabBar，按需挂到任意组件**（卡片、胶囊均挂；底栏改由 HDS 出） | `systemMaterial()` 实为 `CommonMethod` 上的通用属性（`@since 26.0.0`），任意组件可挂——**修正本表早期「仅标题栏/TabBar 生效」的口径**（该口径来自设计规范的场景建议，非能力边界）。挂载层级按官方规范取值：顶部悬浮 `ULTRA_THIN`、卡片/面板 `REGULAR`、底栏由 HDS 的 `MaterialType.ADAPTIVE` 自适应 | 仅挂标题栏/TabBar（否：内容区保持纯色，观感仍是扁平） |
| 玻璃的可读性前提 | **壳层铺品牌氛围底，页面背景一律透明**；材质赋色取中性色 | 材质的观感来自**折射背景**：在均匀纯色底上，模糊结果等于底色，玻璃不可见（模拟器实测：`materialColor` 从 13% 加到 35% 均无可见变化）。故壳层铺**极淡**品牌粉顶部洗色 + 三枚柔和光斑（`components/AmbientBackdrop.ets`，仅挂一次），内容页背景透明化。**品牌识别改由「选中态/主按钮实色品牌」承担，材质一律中性**——首版把品牌粉染进材质并铺到 30%，界面到处是半透明的粉，玻璃与品牌色互相稀释，反而读不出主次 | 页面各自铺底（否：背景不连续，材质折射到的是拼接色块） |
| 小控件的边界 | 小尺寸胶囊/行内控件**用中性次级底色实填、不挂材质**（选中态才用品牌实色） | 小控件在均匀浅底上挂材质出不了轮廓——按钮会失去可点区域的视觉暗示（实测：退出登录按钮、搜索历史胶囊都出现过）。改用固定的中性次级底色（与材质同色）保形，观感与挂材质一致但一定可见 | 一律挂材质（否：控件视觉上消失）；半透明**品牌**实底（否：小控件满屏粉，是"割裂感"的直接来源） |
| 沉浸光感实现路线 | **原生 ArkUI 系统材质**：`uiMaterial.ImmersiveMaterial` + `.systemMaterial()` 通用属性，`module.json5` 配 `ohos.arkui.UIMaterial.state`；**底栏另取 HDS `HdsTabs`** | API 26 已就位：前者是内容区组件的一等能力（含 `interactive`/`lightEffect`），后者是本机已内置的 `@kit.UIDesignKit`（`HdsTabs` 自 6.0.0(20) 起），负责悬浮胶囊的几何与材质。二者分域不重叠 | 全部自绘光效（否：非系统级，功耗与一致性差）；内容区也走 HDS（否：HDS 面向的是导航/列表等成品组件，无通用"给任意组件上材质"的能力） |

## 3. 数据模型

端侧无业务数据库（不建表），仅本地偏好存储：

| Key | 类型 | 说明 |
| --- | --- | --- |
| `auth.access_token` | string | 访问令牌（2h，承 ACC-06） |
| `auth.refresh_token` | string | 刷新凭证（30d，可吊销） |
| `danmaku.settings` | JSON | 不透明度/字号/显示区域/速度/开关（承 DM-11、DM-12） |
| `playback.local_progress` | JSON | 本地进度缓存，服务端进度为准（承 PLY-04） |
| `search.local_history` | JSON | 本地搜索历史副本（最多 10 条，重复置顶）；**本期仅端侧**，承 SRH-04 的云端同步待后端接口就位 |

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
进入播放页 → GET 稿件详情（含签名 m3u8 URL，TTL 6h；服务端按 quality 降序下发，取首档即最高画质）
  → 详情就绪 且 XComponent surface 就绪 → AVPlayer 起播（两者先到先等，surface 只能赋在 initialized 态）
  → 500ms 心跳：按 positionSec 真实增量累计观看时长（单次增量 ≥ 2s 判跳转不计入）
    → 累计 > 5s 上报有效播放（服务端按 uid/IP 去重）
    → 每 10s 节流落盘进度，另在返回/切后台/自然播完/组件销毁四处主动 flush
  → 签名距到期 < 5min → 静默重取地址 → 按 quality 值对齐 → 保留进度与播放态换源续播
  → 手势：单击切控件 / 双击播放暂停 / 横拖改进度 / 竖拖左半屏亮度右半屏音量；全屏 = 横屏 + 铺满窗口
  → 切后台 → 暂停并落盘（回前台不自动续播）；离开页面释放 AVPlayer 并还原窗口态与亮度
```

> 落地细节与实测结论见 [tasks M4-HMY-06](/specs/harmony/tasks)（含起播时序、换源路径、五处实测修复）。

**弹幕链路**（承 DM-10/11/15/20/21）：

```
进入播放页 → 按 6min 分段拉取当前段 + 预取下一段 → 入渲染队列（按 time_ms 排序）
  → Canvas 逐帧绘制（轨道分配 → 碰撞检测 → 移出回收）
  → 订阅 WS 房间 → 实时弹幕插入队列 → 命中屏蔽词/UID → 不入队
  → WS 断开 → 指数退避重连 → 重连失败回退 HTTP 分段轮询
  → 发送：校验等级门槛（本地提示 + 服务端校验）→ 频控 → 上屏
```

> 落地细节与实测结论见 [tasks M4-HMY-07](/specs/harmony/tasks)（含 WS 的 Origin 阻塞与降级轮询、§7.2 黑边分支的实测几何与一处越界缺陷修复）。

**登录与会话**（承 ACC-01/03/06）：

```
登录页 → 验证码或密码 → 存 token 至 preferences
  → 请求拦截器注入 Authorization → 401 → 用 refresh 静默续期并重放原请求
  → 续期失败 → 清凭证 → 跳登录页
```

**三连**（承 ITR-30）：长按点赞 1.5s → 并行发起点赞/投币（默认 2 枚，不足则 1 枚）/收藏 → 幂等键防重 → 原生动画反馈。

## 6. 风险与待定项

- [x] **AVPlayer 播后端 HLS 兼容性（2026-09-22 已验，M4-HMY-06）**：模拟器（API 26 `Pura X View`）上**真实解码出画**，`initialized → prepared → playing` 链路完整，720P/360P 两档 HLS 与签名 URL 组合均可用——**系统 AVPlayer 直接吃 ffmpeg 产出的 m3u8/ts，无需自带解封装**。**注意口径**：模拟器视频硬解受限，播放类结论一律不作「鸿蒙不支持」判定，**起播（约 12s）与换源（约 3.5s）耗时、长期播卡顿率仍待真机复验**（spec §4 的 P90 < 1.5s 亦须真机测）。
- [x] **签名 URL 的请求头约束（2026-09-22 已验，M4-HMY-06）**：AVPlayer 以 `?e=&s=` 签名 URL 直接拉取 `.m3u8` 与 `.ts` 分片**无需附加任何自定义请求头**（Referer/UA 均不必），故 `PlaySignGuard` 现有边界（`.m3u8` 需签名、`.ts` 放行）对端侧已够用，本期零后端改动成立。续签走「重取详情换新签名地址」而非复用旧地址，签名解析收敛在 `media/PlaySign.ets`。
- [x] **模拟器可用性（2026-09-21：已澄清，非宿主故障）**：早前 3 次尝试均在**冷启窗口期内**操作——`hidumper` 窗口数恒 0、`snapshot_display` 恒返回同一张 47KB 全黑帧、`aa start` 报 `10106102 … device screen is locked … developer mode`，并遇 2 次 VM 退出，一度误判为宿主 OpenGL/WGL 故障（`qemu.log` 的 `gl error 502` / `wglMakeCurrent` 失败属启动期现象）。**当晚重试完全可用**：未签名 HAP 安装、启动、渲染、切页签均正常。
  - **操作口径**：先确认 SystemUI 就绪（`com.ohos.sceneboard` 进 FOREGROUND / `hidumper -s WindowManagerService` 窗口数 > 0）再安装与启动；不在启动窗口期反复 `snapshot_display`。命令速查见 [tasks M4-HMY-01](/specs/harmony/tasks)。
  - **附带结论**：模拟器为 **API 26 / guest `7.0.0.106(SP1DEVC00E999R4P11)` / abi `x86_64`**；**未签名 HAP 可直接 `hdc install` 成功**——本机本地验证**无需配置 `signingConfigs`**，可砍掉签名前置。
  - **沉浸光感是否生效（2026-09-22 已验，见下条「沉浸光感落地」）**：当时判「浅色纯色底无可比对参照」只说对了一半——**材质确实生效了**（`supported=true`、应用级开关 `state=ENABLE`、全局档 `level=EXQUISITE`），但**纯色底上玻璃本来就看不出**，真因在背景而非能力。
- [ ] **弹幕 Canvas 性能上限**：同屏弹幕满载下的帧率与内存需实测（spec §4 要求 ≥ 55fps）。2026-09-22 补：轨道渲染已由 M4-HMY-07 落地，但**帧率/内存未测**，归 M4-HMY-10 端侧验收（同一处一并测控制层材质叠加的开销，见 §7.2 末条）。
- [ ] **弹幕 WS 的 Origin 白名单（2026-09-22 实测，归线上配置）**：ArkTS 的 WebSocket **自生成 Origin**（默认 `http://<host>`，不带端口；API 26 起 `supportOriginPort` 可带上），而服务端 `CheckOrigin` 对 `allow_origins` 做**精确匹配**——dev 白名单（`http://localhost:5173~5175`）不含端侧来源，故直连 `ws://10.0.2.2:8000` 握手 403，端侧只能降级为 15s 分段轮询。**dev 联调可临时同源绕过（做法见 [tasks M4-HMY-07](/specs/harmony/tasks)），生产必须让服务端白名单纳入 App 的实际 Origin，或同源部署 `wss://<正式域名>`**；本期零后端改动，未动白名单。
- [x] **状态管理版本（已解决，2026-09-21）**：壳层 `pages/Index.ets` 以 **ArkUI 状态管理 V2**（`@Entry @ComponentV2` / `@Local` / `@Param`）实写并通过 API 26 编译（`assembleHap` BUILD SUCCESSFUL），**定版 V2**，不再保留 V1 备选。
- [x] **目标 API 版本选择（已解决，2026-09-21）**：DevEco 升级至 **26.0.0.821** 后内置 SDK 为 **HarmonyOS 26.0.0 / API 26**（version 26.0.0.105），模拟器镜像 **API 26 / 7.0.0.106**（`D:/Program Files/Huawei/sdk/system-image/HarmonyOS-7.0.0/phone_all_x86`）已就位，实例含 Mate 70 Pro / Mate 80 Pro Max / Pura 90 / Pura X View。**`compileSdkVersion` 与 `compatibleSdkVersion` 均取 26**，原"编译 24 / 运行时 23"的双口径已失效，历史结论仅备查。
- [x] **沉浸光感落地（2026-09-22 已落地并在模拟器实测，M4-HMY-11）**：主路线为**原生系统材质**——`uiMaterial.ImmersiveMaterial` + `.systemMaterial()`，应用级开关走 `module.json5` 的 `metadata` → `ohos.arkui.UIMaterial.state`（`default`/`enable`/`disable`，仅 entry 模块生效，本工程已配 `enable`）。
  - **挂载点（修正早期口径）**：`.systemMaterial()` 是 **`CommonMethod` 上的通用属性**（`@since 26.0.0`），**任意组件可用**；`Navigation` 标题栏（`titleOptions.systemMaterial`）与 `Tabs` 的 `BottomTabBarStyle`（`systemMaterial`）只是系统**默认适配面**，不是能力边界。另有 `MaterialState.ENABLE` 自动生效的组件：Dialog / Toast / AlphabetIndexer / Chip / ChipGroup / Select / Menu / Toggle / SegmentButton / Slider / bindSheet / SelectionMenu（弹窗实测确实自带玻璃观感）。
  - **能力探测（API 26 x86 模拟器实测取值）**：`isImmersiveMaterialSupported()` = `true`；`getMaterialInfo().state` = `MaterialState.ENABLE`；`getGlobalMaterialLevel()` = `MaterialLevel.EXQUISITE`。即模拟器算力档位为最高档，`style`/`materialColor`/`colorInvert` 全部生效——早期「x86 模拟器可能非高/中档」的担心不成立。启动时经 `Logger.info` 打一条 `沉浸光感：supported=… state=… level=…` 便于从日志核对。
  - **配置项**：`ImmersiveOptions { style, materialColor, colorInvert, applyShadow, interactive, lightEffect }`。`style` 取 `ImmersiveStyle.{ULTRA_THIN|THIN|REGULAR|THICK|ULTRA_THICK}`，按官方场景规范：顶部悬浮 `ULTRA_THIN`、底部悬浮 `THIN`、卡片/面板 `REGULAR`、任意弹出 `THICK`、半模态/弹窗 `ULTRA_THICK`（本工程实际只用前三档：标题栏 `ULTRA_THIN`、卡片 `REGULAR`、胶囊 `THIN`；**底栏材质已交 HDS 的 `systemMaterialEffect` 出**）。`lightEffect` 为感光交互反馈（可配色），`interactive` 为交互形变。
  - **必须尊重系统分层**：`style`/`materialColor`/`colorInvert` 仅在高/中算力设备生效（低算力设备退化为影响背景色/边框/阴影），**不得假设效果恒定**；用户三档强度（强/均衡/弱）由系统自动映射，开发只定义一次。
  - **`systemMaterial` 与 `backgroundColor` 可以并存（2026-09-22 复测修正）**：早期结论「材质会压过 `backgroundColor`」**不成立**——把退出登录按钮的 `backgroundColor` 换成明显色（`border_default`）后底色照旧透出，材质只在上面叠一层光感。故 `DliMaterial.surface()`（材质地基色）**始终返回中性次级底色**（`surface_secondary`），不返回 `Color.Transparent`：能力可用时是"中性玻璃 + 光感"，不可用时退化为平铺次级底色，两种情况下组件都有轮廓。关闭某组件光感仍用 `uiMaterial.Material.empty`（与传 `undefined` 语义不同——后者是恢复默认）。**`borderWidth`/`borderColor` 仍不建议同挂**（官方口径如此，本轮未逐项复测）。
  - **关键经验：玻璃靠「折射背景」显形，均匀纯色底上材质等于不可见**。实测把 `materialColor` 不透明度从 13% 逐级加到 35%，在近白底上始终看不出边界——**材质本身是"透"而不是"填"**。因此：① 氛围底是材质可读的前提（见 §2「玻璃的可读性前提」），但**氛围底要克制**：品牌粉铺到 30% 时界面到处是半透明的粉，反而割裂，最终收敛为顶部 ≈3% 的洗色 + 光斑 ≤12%；② 材质赋色必须取**中性**（页面白 → 材质用次级灰），染品牌色既稀释品牌又让玻璃失去中性底；③ 需要保形的小控件直接用同色 `backgroundColor` 实填、不挂材质（见 §2「小控件的边界」）；④ 判定材质是否生效**不能靠纯色底上的观感**，要用能力探测取值 + 滚动内容从胶囊下穿过时的折射。
  - **底栏最终形态：HDS 悬浮胶囊，不是自绘 Tabs**。首版用 `Tabs(barPosition: End)` + `barFloatingStyle({ maskColor, systemMaterial })`，出来仍是"整宽底栏 + 蒙层"。改为 `HdsTabs`（`@kit.UIDesignKit`）后：`barMode(BarMode.Fixed)` + `scrollable(false)` + `divider({mode: DividerMode.NONE})` + `barFloatingStyle({ barBottomMargin: 16, adaptToHandedness: true, barWidth: { smallWidth: 294, mediumWidth: 328, largeWidth: 328 }, systemMaterialEffect: { materialType: MaterialType.ADAPTIVE, materialLevel: MaterialLevel.ADAPTIVE } })`；**窄 `barWidth` 才是"胶囊"的来源**。`barOverlap(true)` 要求页面滚动内容留出底部空白（`DliSize.TAB_RESERVED = 96`），否则最后一行被胶囊压住。能力不足时降级为 `MaterialType.NONE` + `MaterialLevel.SMOOTH`（经 `hdsMaterial.getSystemMaterialTypes()` 判定）。**不引入 `HdsTabsController`**：`index` 绑定已够用（标题栏搜索胶囊点按切页签实测生效），少一个成员变量。
  - **HDS（`@kit.UIDesignKit`）分域使用**：`HdsTabs` 已升为主线用于**底栏**（见上条，本机 `@since 6.0.0(20)` 起内置）；其余 HDS 组件（`HdsVisualComponent` 等，`@since 6.1.0(23)`）本期未使用——内容区的材质仍走 `uiMaterial` 通用属性，二者分域不重叠。若后续需兼容低版本设备，HDS 系可作降级路径。
  - 页面内容区（播放器控制层、评论面板、弹幕面板）改用材质**同样可挂**，其观感与功耗开销待 M4-HMY-06/07 落地时实测（弹幕层逐帧重绘，是否与材质叠加掉帧需单独验）。
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
