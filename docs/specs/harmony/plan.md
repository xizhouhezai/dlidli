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

### 2.1 投稿（M4 追加，承 §2.6 需求）

| 决策点 | 决策 | 理由 | 备选方案 |
| --- | --- | --- | --- |
| 上传通道 | **A 路线：`@ohos.request` 的 `uploadFile`（`UploadTask`）为主**，`HttpClient` 的二进制 PUT 作为降级路径。分片用 `PUT`（裸 body），**封面复用同一 API 的 `POST` 分支（multipart）** | 2026-10-07 初测 + **2026-10-08 复测定论**（方法与证据见 §6.1 首条）：`uploadFile` 支持 `method:'POST'\|'PUT'`，且**帧格式由 method 决定**——`PUT` 在「显式 `begins`/`ends` + 单文件」下把**该字节区间作为 HTTP body 裸发**（后端落盘长度与区间严格相等、合并成品 SHA-256 与端侧一致）；`POST` 则发 **multipart 信封**（后端 `c.FormFile("file")` 可解析）。故分片走 PUT、封面走 POST，**后端零改动**成立。另带系统级后台传输与 `on('progress')`。**注意**：不传 `begins`/`ends` 时 PUT 发的是**整个源文件**（不是单片），超大文件会撑爆连接并卡住客户端，**分片必须显式给区间**。B 路线同样实测跑通（`extraData` 传 `ArrayBuffer`），但需自管切片/重试与保活 | 仅 B 路线（否：失去系统级后台传输，锁屏保活要自建，见 §6.1 第二条）；`@ohos.net.rcp`（否：无专用上传任务与进度语义）；改后端新增 multipart 变体（否：实测无必要，保住零后端改动） |
| 分片协议 | **复用后端既有分片接口，零后端改动** | 后端 `/upload/init`（秒传+断点恢复）、`PUT /upload/{id}/parts/{index}`（**裸二进制 body**，见 `packages/api-client` 的 `putRaw`）、`POST /upload/{id}/complete` 已完备（chunk 5MB / 上限 8GB / 白名单 mp4·mov·mkv·flv·avi）。**端侧发分片必须用 `method:'PUT'` + 显式 `begins`/`ends`**：实测 PUT 才发裸 body，POST 会发 multipart 信封而被 `io.Copy` 原样落盘（实测落盘 `ChunkSize+1=5242881` 字节即触发后端「分片大小不合法」） | 整文件直传（否：8GB 级文件必超时且不可续传）；新增端侧专用上传接口（否：契约分叉，无必要） |
| 文件选取 | **`@ohos.file.picker` 的 `PhotoViewPicker` / `DocumentViewPicker`**（相册优先，不可用/取消时回退文档选择器） | 系统选择器直接返回 `uri`，**无需申请读媒体库权限**（选择器是授权入口）。**2026-10-08 HMY-51 补**：picker URI **不能直接投递给 `request.uploadFile`**（实测报 401 `user file can only for request.agent.`），必须先 `fileIo.copy`/`copyFile` 到应用 cache 再以 `internal://cache/…` 投递——详见 §6.1 首条待验项的定论 | 申请 `READ_MEDIA` 直读相册（否：权限面大、需用户额外授权，系统选择器已够用）；只走相册（否：相册为空/无视频时用户完全无法选片，实测模拟器相册即为空） |
| 哈希与秒传 | **端侧算 SHA-256**（`@ohos.file.hash` 的 `HashStream` **分块**读取，非整文件一次调用），命中秒传则跳过上传 | 后端 `InitReq.file_hash` 为必填且长度须为 64 位 hex；秒传可让重复投稿零上传。**2026-10-08 HMY-51 补两条实测口径**：① `hash.hash()` 返回的是**大写** hex，而后端 `Init` 里 `strings.ToLower` 后比对，故端侧**必须转小写**，否则秒传永不命中且 `complete` 会以「文件校验失败」(30004) 收场；② 分块（4MB/次）而非整文件一次，以避开 8GB 上限文件的单次调用约束 | 不算哈希传空（否：后端 binding `required,len=64` 直接拒）；仅按文件名/大小推断（否：不可靠且后端不支持） |
| 上传态与草稿 | **`upload_file` 会话由后端持有，端侧只存「稿件草稿」**（标题/简介/分区/标签/file_id/本地封面路径）于 `preferences` | 后端已用 Redis 存上传会话（`up:sess:*`/`up:parts:*`，TTL 24h）并支持 `GET /upload/{id}` 查已传分片，**端侧无需自建分片账本**；草稿只解决"填了一半退出"的场景 | 端侧自建分片状态表（否：与后端 Redis 会话重复，且不一致时更难排查）；不做草稿（否：HMY-52 明确要求） |
| 封面处理 | **`@ohos.multimedia.image` 解码 + `PixelMap` 裁切 16:9**，再以 `multipart/form-data` 传 `POST /videos/cover`（字段名 `file`）；**multipart 由 `request.uploadFile` 的 `POST` 分支直接产出**，无需扩展 `HttpClient` | 后端封面接口是 `c.FormFile("file")` multipart（**与分片的裸 body 不同**），限 5MB / jpg·png·webp；端侧先裁切可避免上传后被裁掉主体。**2026-10-08 实测修正**：`uploadFile` 配 `method:'POST'` + `files:[{name:'file'}]` 即可产出 multipart 并被 `FormFile` 解析（封面落盘字节数与源文件相等，5 次皆 200），故 `UploadApi` 只需一个「同一 API、两种 method」的分支，不必为封面另造 multipart 编码器 | 直接传原图（否：相册图常见 4:3/竖图，后端按 16:9 消费会裁掉内容）；**用 PUT 传封面（否：实测 PUT 发裸 body → 后端拿不到 `file` 字段，且 `PUT /videos/cover` 路由不存在返回 404）** |
| 期望类型 | `@ohos.request` / `@ohos.file.picker` / `@ohos.multimedia.image` 按 OpenAPI **手写端侧模型**（沿用既有口径） | 与全网一致：swag 产物无响应模型，手写 + 真实响应核对；本次新增 `model/Upload.ets`（`InitResp`/`CompleteResp`/`Draft`）与 `SubmitReq` | 生成器（否：不可行，见 §6） |
| 提交与状态回看 | 复用 `POST /videos`（`SubmitReq`）与 `GET /videos/mine` | 两者均已存在，且 `MineResult` 已在 M4-HMY-09 落地；`video.status` 枚举（0 草稿/1 上传中/2 转码中/3 待审核/4 已发布/5 已驳回）可直接驱动 HMY-53 的状态展示 | 新增状态查询接口（否：`/videos/mine` 已含 status 与 reject_reason） |

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

**M4 追加：投稿新增消费的接口**（同样**不新增后端接口**，全部为既有能力）：

| 方法 | 路径 | 说明 | 端侧注意 |
| --- | --- | --- | --- |
| POST | `/api/v1/upload/init` | 初始化上传（秒传/断点恢复） | `file_hash` 必填且为 64 位 hex；响应 `fast=true` 时直接拿 `file_id` |
| PUT | `/api/v1/upload/{id}/parts/{index}` | 上传分片 | **裸二进制 body**（非 multipart）；分片大小以后端 `chunk_size` 为准（5MB） |
| GET | `/api/v1/upload/{id}` | 查询已传分片 | 用于断点续传与草稿校验 |
| POST | `/api/v1/upload/{id}/complete` | 合并分片 | 返回 `file_id` 供投稿使用 |
| POST | `/api/v1/videos/cover` | 上传封面 | **multipart**（字段名 `file`）；≤5MB；jpg/png/webp |
| POST | `/api/v1/videos` | 提交稿件 | `SubmitReq`：`file_id`/`title`≤80/`description`≤2000/`category_id`/`tags`(1~10)/`copyright`(1自制·2转载)/`cover`/`parts`(多P) |
| GET | `/api/v1/videos/mine` | 我的投稿 | 返回 `{list,total}`，含 `status` 与 `reject_reason`，驱动 HMY-53 |
| GET | `/api/v1/categories` | 分区列表 | **注意不在 `/videos` 下**；与首页分区共用 |

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
>
> **§7.2 影音娱乐场景「无黑边时同屏弹幕不宜过多」的落地与实测**：同屏密度由 `DanmakuSettings.densityRatio()`（0.7/1/1.3）与轨道数（`trackCount`，含 `MAX_ACTIVE=240` 封顶）共同控制；**2026-10-08 实测**：把该片源灌到 1000 条同屏候选、与仅 45 条对比，播放页帧率**都锁在片源 30fps（p50 = 32.00ms）**，即该档位在模拟器上不是瓶颈（真机口径见 §6「材质叠加帧率实测」）。

**登录与会话**（承 ACC-01/03/06）：

```
登录页 → 验证码或密码 → 存 token 至 preferences
  → 请求拦截器注入 Authorization → 401 → 用 refresh 静默续期并重放原请求
  → 续期失败 → 清凭证 → 跳登录页
```

**三连**（承 ITR-30）：长按点赞 1.5s → `POST /videos/{bvid}/triple`（服务端一次做完点赞 + 投币 2 枚不足则 1 枚 + 收藏默认夹，返回三项状态与 delta）→ 端侧按 delta 修正计数 → 原生动画反馈。

> 落地细节见 [tasks M4-HMY-08](/specs/harmony/tasks)：长按由 `LongPressGesture({duration:1500})` 直接驱动（不在控制器内重复计时，否则手感变 3s）；长按松手时 Tap 与 LongPress 在 Parallel 组下都会命中，故加 300ms 单击抑制窗口（三连已含点赞，否则会把刚点上的赞取消掉）；动画用 `uiContext.animateTo` 做脉冲 + 三段递进点亮；幂等靠服务端开关/唯一键语义 + 端侧 `acting` 串行化与失败回滚。

> **与 Web 端的差异**（承 spec 端侧差异条款）：长按 800ms → 1.5s；分享由「转发到动态」弹层改为**系统分享面板**（`systemShare.ShareController`，鸿蒙端无小程序卡片载体）。

## 6. 风险与待定项

- [x] **AVPlayer 播后端 HLS 兼容性（2026-09-22 已验，M4-HMY-06）**：模拟器（API 26 `Pura X View`）上**真实解码出画**，`initialized → prepared → playing` 链路完整，720P/360P 两档 HLS 与签名 URL 组合均可用——**系统 AVPlayer 直接吃 ffmpeg 产出的 m3u8/ts，无需自带解封装**。**注意口径**：模拟器视频硬解受限，播放类结论一律不作「鸿蒙不支持」判定，**起播（约 12s）与换源（约 3.5s）耗时、长期播卡顿率仍待真机复验**（spec §4 的 P90 < 1.5s 亦须真机测）。
- [x] **签名 URL 的请求头约束（2026-09-22 已验，M4-HMY-06）**：AVPlayer 以 `?e=&s=` 签名 URL 直接拉取 `.m3u8` 与 `.ts` 分片**无需附加任何自定义请求头**（Referer/UA 均不必），故 `PlaySignGuard` 现有边界（`.m3u8` 需签名、`.ts` 放行）对端侧已够用，本期零后端改动成立。续签走「重取详情换新签名地址」而非复用旧地址，签名解析收敛在 `media/PlaySign.ets`。
- [x] **模拟器可用性（2026-09-21：已澄清，非宿主故障）**：早前 3 次尝试均在**冷启窗口期内**操作——`hidumper` 窗口数恒 0、`snapshot_display` 恒返回同一张 47KB 全黑帧、`aa start` 报 `10106102 … device screen is locked … developer mode`，并遇 2 次 VM 退出，一度误判为宿主 OpenGL/WGL 故障（`qemu.log` 的 `gl error 502` / `wglMakeCurrent` 失败属启动期现象）。**当晚重试完全可用**：未签名 HAP 安装、启动、渲染、切页签均正常。
  - **操作口径**：先确认 SystemUI 就绪（`com.ohos.sceneboard` 进 FOREGROUND / `hidumper -s WindowManagerService` 窗口数 > 0）再安装与启动；不在启动窗口期反复 `snapshot_display`。命令速查见 [tasks M4-HMY-01](/specs/harmony/tasks)。
  - **附带结论**：模拟器为 **API 26 / guest `7.0.0.106(SP1DEVC00E999R4P11)` / abi `x86_64`**；**未签名 HAP 可直接 `hdc install` 成功**——本机本地验证**无需配置 `signingConfigs`**，可砍掉签名前置。
  - **沉浸光感是否生效（2026-09-22 已验，见下条「沉浸光感落地」）**：当时判「浅色纯色底无可比对参照」只说对了一半——**材质确实生效了**（`supported=true`、应用级开关 `state=ENABLE`、全局档 `level=EXQUISITE`），但**纯色底上玻璃本来就看不出**，真因在背景而非能力。
- [ ] **弹幕 Canvas 性能上限：帧率已测，阈值与内存待真机（2026-10-08 部分收口）**：同屏弹幕满载下的帧率已测——**播放页恒锁在片源帧率上**（片源 30fps → p50 帧间隔恒为 32.00ms），**同屏 45 条与灌满 1000 条、弹幕开与关，帧率都无差异**，即该窗口内 Canvas 逐帧重绘不是瓶颈（活动条数受 `MAX_ACTIVE=240` 与轨道数封顶）。**仍未收口的两项**：① **spec §4 的「≥55fps」本轮无法验证**——示例片源本身只有 30fps，要验该阈值须用 ≥60fps 片源；② **内存与长时稳定性未测**（采样窗口为秒级）。两项均归真机。方法见下条「材质叠加帧率实测」。
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
  - 页面内容区（播放器控制层、评论面板、弹幕面板）改用材质**同样可挂**；其观感已随 M4-HMY-06/07 落地，**帧率开销已于 2026-10-08 实测收口（见下条）**。
  - **材质叠加帧率实测（2026-10-08，API 26 模拟器 `Pura X View`，应用零改动；方法与逐项数值见 [tasks M4-HMY-01](/specs/harmony/tasks)）**：
    - **方法**：`hitrace -t N -b 65536 -o <f> graphic ace animation` 抓帧，数应用进程的 `B|<pid>|H:OnVsyncEvent` 条数，**帧率 = (条数-1) ÷ 首末 `now` 纳秒差**。两条铁律：① 静止时该标记为 **0**（系统按需出帧），**不能用「固定窗口数帧数」**；② 模拟器上 `hidumper … fpsCount` 恒为 `Refresh Rate:60, Count:1`、`SP_daemon -f/-ohtestfps` 恒为 `fps=0`，**三者只有 hitrace 可用**。交叉印证取 RenderService 侧 `RSUniRenderThread::Render` 与 **`wouldDrawLargeAreaBlur`（材质模糊路径）**。
    - **结论（模拟器可得）**：**帧率测不出材质差**——首页滚动材质开/关均顶满 **60.6 / 61.1 fps**；播放页锁在片源 30fps（p50 = 32.00ms），弹幕开/关、45 条 vs 1000 条**都无差异**。**但开销真实存在**：同载荷下材质模糊路径绘制次数 **1664/720/608 对 528/336/432**（逐轮 3.15× / 2.14× / 1.41×），只是被 60Hz 上限下的余量吸收。
    - **须真机确认（不得外推）**：① 模拟器为 **x86_64 软件渲染**（`hmos.emulator` / `abilist x86_64`，宿主 Intel Arc 转译），材质模糊的**绝对开销与掉帧风险**必测真机；② **功耗/发热在模拟器上不可得**——`SP_daemon -p` 报 `RK does not support power acquisition`、电池为虚拟值，**功耗结论必须真机测**；③ spec §4「同屏弹幕满载 ≥55fps」须用 ≥60fps 片源在真机验证；④ 长时（>5min）播放的稳定性与内存增长未测。
- [x] **OpenAPI → ArkTS 类型生成（已解决，2026-09-21：判定为不可行，改为手写）**：实测 `server/docs/swagger.json`（swagger 2.0，64 个 path / 82 个响应）：**82 个响应 schema 无一例外全是 `$ref: #/definitions/github_com_dlidli_server_internal_pkg_response.Body`**，而该 `Body` 的 `data` 字段是空 schema `{}`（等价 `any`）；13 个 `definitions` 里只有**请求体**（且多为 admin 域）有结构。
  - **结论**：生成器无料可生——产出只有统一包裹与少量请求体，端侧真正需要的响应模型（稿件详情、弹幕分段、搜索结果…）一个都拿不到。**放弃生成路线，改为手写端侧模型 + 契约核对**。
  - **根因与不可解性**：根因在后端 handler 未标注具体响应类型，修它必须改后端，与本期「零后端改动」冲突。后续若要重开生成，需后端先为响应引入泛型 DTO 标注（如 `response.BodyOf[T]`），届时再评估。
  - **执行口径**：`model/` 目录为手写产出，M4-HMY-03 起按 OpenAPI 逐条核对字段名、类型与可空性，并在端侧联调时以后端真实响应为准。
- [ ] **微信登录**：鸿蒙端无小程序载体，微信开放平台鸿蒙版 SDK 可用性待查证（本期不做）。
- [ ] **CI 影响**：hvigor 构建需 DevEco 环境，CI 是否纳入鸿蒙构建待定（本期可先本地构建）。
- [ ] **应用签名与上架**：本期口径为仅模拟器/真机本地验证，不提审；签名证书与资质材料待上架阶段再办。

### 6.1 投稿专项风险（M4 追加，2026-09-30 预研）

- [x] **上传通道与后端分片协议对接（2026-10-07 初测、2026-10-08 复测定论，M4-HMY-50）——结论：A 路线（`request.uploadFile`）可行且为主路线，后端零改动成立；B 路线亦可行，仅作降级**。
  - **实测方法（可复现）**：`apps/harmony` 加一次性探针页（跑完即删，树中不留痕），对本地真实后端（`http://10.0.2.2:8000`，MySQL/Redis 均 up）用**端侧自生成的确定性填充文件（1 KB / 5 MB / 9 MB / 11 MB / 20 MB / 25 MB / 30 MB）**跑完整分片链路（init → 逐片 PUT → complete）。判定**不读端侧自述**：① 读后端落盘 `server/uploads/chunks/<uploadId>/<index>.part` 的字节数；② 读 `complete` 结果；③ **由宿主机对合并成品 `server/uploads/videos/source/<sha>.mp4` 重算 SHA-256，与文件名（即端侧上报的 `file_hash`）逐字节比对**——11 MB / 9 MB / 25 MB 三份成品 size 与 sha256 **全部 MATCH**（服务端在 `complete` 时按合并结果校验 hash，校验不过不会按该 hash 落盘命名）。
  - **A 路线全链路跑通**：`method:'PUT'` 被接受；11 MB 三片（5/5/1 MB）全部 `responseCode=0`，`POST /upload/{id}/complete` 返回 `file_id`——**服务端把合并结果算出 SHA-256 与 `file_hash` 比对通过**，这是端到端正确性的硬判据。
  - **① 能否发 PUT —— 能**：`UploadConfig.method` 的 SDK 声明即写明「value can be **POST** or **PUT**」（`@ohos.request.d.ts`，since 6），实测 `PUT` 直接生效。
  - **② 帧格式由 `method` 决定 —— 2026-10-08 复测修正了原「uploadFile 一律 multipart」的口径**：
    - `method:'PUT'` → **裸 body**。显式 `begins`/`ends` 时请求体**恰为该闭区间**：打 `[1000,1999]` 落盘 `000000.part` 恰 **1000** 字节；整片 5 MB 落盘恰 **5242880** 字节（**无任何边界开销**）。
    - `method:'POST'` → **multipart/form-data 信封**。打 `POST /videos/cover`（后端 `c.FormFile("file")`）**5 次全部 200 且封面成功落盘 50000 字节**（= 源文件长度）；对照组 `PUT /videos/cover` 返回 **404**（该路由不存在）。
    - 反向对照：以 `POST` 打分片接口 → 服务端返回 **404**（`POST /upload/:id/parts/:index` 路由不存在），且客户端 `uploadFile` 因拿不到响应而**挂起到超时**。
    - **结论：A 路线既不需要后端 multipart 变体，也不需要自造 multipart 编码器**——分片走 `PUT`（裸 body）、封面走 `POST`（multipart），**两条路都复用同一个 `request.uploadFile`**，零后端改动成立。
  - **③ `begins`/`ends` 是真字节闭区间，且分片必须显式给**：声明为 "File start point / end point to read …, in bytes … closed interval"；实测落盘恰为 `ends - begins + 1`。**反例（务必记住）**：**不传** `begins`/`ends` 时 PUT 发的是**整个源文件**而非一片——11 MB 文件无区间打单片时，后端 `io.Copy(io.LimitReader(body, ChunkSize+1))` 读到 **5242881** 字节并报「分片大小不合法」，客户端同时因连接被提前掐断而挂到超时。实现须固定 `begins = i*chunkSize`、`ends = begins+len-1`。
  - **④ 切后台/锁屏是否持续 —— 行为已实测（模拟器可得），保活边界仍须真机确认**：
    - 切后台（`moveAbilityToBackground()`）后分片继续推进；**并在「上传途中」由宿主经 hdc `power-shell suspend` 锁屏**，锁屏后 part 2–5 仍逐片完成、`complete` 成功、成品 sha256 校验 MATCH（30 MB / 6 片：锁屏发生在 t≈3.0 s，收口于 t≈16.4 s）。系统上传任务确实把网络请求与页面生命周期解耦。
    - **坑 1：切后台后进程仍可能被回收**。同尺寸对照中曾出现切后台瞬间进程被 WMS 销毁、后台阶段零进展的情形——「切后台没暂停」不等于安全，长任务必须配 `backgroundTaskManager.startBackgroundRunning` 申请长时任务并声明 `backgroundModes`。
    - **坑 2：模拟器的 `moveAbilityToBackground()` 可能直接返回 `16000065`**（该 API 仅能在前台调用），属模拟器/时序限制，不代表真机行为；判定「后台是否持续」不能只看该调用是否成功。
    - **边界（须真机确认）**：真机锁屏 + 切后台 5 分钟的长时任务保活、省电策略与厂商后台管控，**模拟器结论不作数**，见 §6.1 第二条。
  - **B 路线亦跑通（降级路径）**：扩展 `HttpClient` 传二进制——`HttpRequestOptions.extraData` 传 `ArrayBuffer`（**裸 body 由 `extraData` 决定**；`expectDataType` 控制的是**响应**类型，本次取 `HttpDataType.STRING` 以解析统一包裹），`Content-Type: application/octet-stream`，自切 5 MB 片 PUT；9 MB 两片均 200、`complete` 返回 `file_id`，成品 size 9438418 与 sha256 由宿主机复核 **MATCH**。**代价**：失去系统级后台传输（锁屏保活要自建），且需自管切片/重试/进度。
  - **落地约束（供 HMY-51）**：本轮已验证可用的 `files[].uri` 形态是 `internal://cache/<相对 cacheDir 路径>`，**`fileIo` 给出的绝对 `file://…` 形态会报 `401 … GetInternalPath failed`**（其余形态是否可用见下条待验项）；`config.index` 是 `files` 数组下标（单文件时必须为 `0`），**不是分片序号**。**（修正）**`data` 是否非空不决定帧格式：**`PUT` 即使 `data` 非空仍发裸 body**，发 multipart 的是 **`POST`**（见上条②）。
  - **并发教训（承 2026-10-07 初测记录，本轮未重复该项压测）**：切勿在循环里密集 `await request.uploadFile()`——初测 100 次齐发触发 `appfreeze THREAD_BLOCK_6S`（主线程阻塞 6s 被杀）。分片任务应**限并发（建议 ≤2）或严格串行**；本轮所有链路均为串行、全程无冻结。
  - **结论口径**：以上均为 **API 26 x86 模拟器（`Pura X View`）所得**；「协议适配 / PUT / 字节区间 / 裸 body」属协议层结论，可直接用于实现；**后台传输的保活边界属设备行为，须真机确认**。
- [x] **选片 URI 到 `request.uploadFile` 的交接（2026-10-08 于 HMY-51 实测定论）——结论：picker URI 直接投递被拒，拷进 cache 是必做步骤，不是可选优化**。
  - **实测方法与结论**：模拟器（API 26，`Pura X View`）里由宿主投放测试视频后走**真实系统选择器**选片，取到文档 URI `file://docs/storage/Users/currentUser/brink_20_512.mp4`。两组对照：
    - **① 直接投递该 picker URI → 被拒**：`request.uploadFile` 抛 `code=401 The parameters check fails, Parameter verification failed, user file can only for request.agent.`——即该 API **只认应用沙箱路径**，picker 给的媒体库/文档 URI 不在其可解析范围内（与 SDK 对 `File.uri` "Only `internal://cache/` is supported" 的声明一致）。
    - **② 拷进应用 cache 后再投递 → 接受**：同一文件先落到 `cacheDir/upload_staging/…`，再以 `internal://cache/upload_staging/…` 投递，任务正常发出并返回 `responseCode=0`。
    - **附带坑**：`fileIo.copy(pickerUri, destPath)` 的目标**必须是沙箱真实路径**——传 `internal://cache/…` 形态（`request.uploadFile` 认、但文件系统 API 不认）会报 `401 The input parameter is invalid`；且 `fileIo.copy` 本身在文档 URI 上也会失败，需退回 **`fileIo.copyFile(fd, dest)`**（先 `openSync(uri, READ_ONLY)` 拿 fd）。实测 372MB 文件走该退路拷贝成功。
  - **代价与结论**：每次投稿多一次整文件拷贝（分片源就是这份副本，故 cache 剩余空间必须先校验），换来的是唯一被实测验证可用的投递形态。若日后要走 picker URI 直投，应改用 `request.agent`（错误信息指向它）而非 `request.uploadFile`。
  - **口径**：以上为模拟器所得；属**协议/API 行为**结论（与设备型号无关），可直接用于实现。
- [ ] **后台传输的保活边界**：即便走 `request.uploadFile`，系统对后台任务仍有约束（长时任务需声明 `backgroundModes`，且可能受省电策略影响）。HMY-50 的"锁屏继续上传"需在真机上以"锁屏 + 切后台 5 分钟"实测确认，**模拟器结论不作数**。2026-10-08 补：模拟器已实测「**上传途中**锁屏后分片继续推进并完成、成品 sha256 校验 MATCH」（30 MB / 6 片），但**同尺寸对照中仍出现过切后台即被回收进程**的情形，且模拟器 `moveAbilityToBackground()` 可能直接返回 `16000065`；故保活不能只依赖"系统任务解耦"的观察，**HMY-51 必须显式申请长时任务**。
- [ ] **端侧算大文件 SHA-256 的耗时**：8GB 文件全量哈希在端侧可能达数十秒。需实测并决定策略（如对 > 500MB 的文件改用"抽样哈希 + 尺寸"做弱秒传，或直接跳过秒传改由后端在合并时校验）。**后端 `file_hash` 是必填 `len=64`**，跳过秒传仍须算完哈希，故此耗时无法回避，只能优化或后端放宽。
- [ ] **`uploads/` 与数据库的一致性运维**（承本仓 [部署文档 §4.8](/project/deployment)）：投稿功能上线后，端侧产生的媒体文件同样落在 `server/uploads/`（**不在 Docker 卷内、被 gitignore**），**该目录与数据库必须同周期备份**，否则重演 2026-09-30 的"文件在、元数据丢"事故。此项为运维约束，非端侧代码问题，但投稿上线前应在 [deployment](/project/deployment) 的检查清单中确认。
- [ ] **草稿的清理策略**：本地草稿（`preferences`）与服务端上传会话（Redis TTL 24h）生命周期不一致——草稿可能引用已被清理的上传会话。需定义：进入草稿时**先 `GET /upload/{id}` 校验会话是否仍有效**，失效则提示"上传已过期，需重新选择文件"。
- [ ] **端侧投稿的定位复核**：spec §1 原定"投稿引导至 Web"，V1.3 放开为端侧承接。**Web 端 `UploadView` 已有的能力边界需逐项对齐**（多P 上限 10、标签 1~10、标题 ≤80、简介 ≤2000），端侧不得超出或遗漏（HMY-52/54）。


### 6.2 上传链路实现（M4-HMY-51，2026-10-08）

按 §2.1 的 A 路线落地，**后端零改动**。端侧新增四个单元：

| 单元 | 职责 | 关键约束（均来自实测，改动前必读） |
| --- | --- | --- |
| `service/UploadApi.ets` | `init` / `progress` / `complete` 三个 JSON 接口 | 分片接口**不在此处**——它的 body 是裸二进制，而 `HttpClient` 固定发 JSON。`file_hash` 在此统一 `.toLowerCase()` |
| `service/UploadController.ets` | 链路编排：暂存 → 哈希 → init（秒传短路）→ **只补缺失分片** → complete；进度发布；前置校验 | ① 必须 `method:'PUT'`；② 必须显式 `begins`/`ends` 闭区间；③ **限并发 ≤2**；④ `TaskState.responseCode` 成功时为 **0**（不是 200）；⑤ 进度必须**每次新建对象**再回调 |
| `service/BackgroundTransfer.ets` | `DATA_TRANSFER` 长时任务申请/释放 | `backgroundModes: ["dataTransfer"]` + `KEEP_BACKGROUND_RUNNING` 已声明；申请失败不阻断上传 |
| `common/utils/MediaLib.ets` | 选片、cache 暂存、分块 SHA-256 | picker URI 必须先拷进 cache；`fileIo.copy` 的目标须为沙箱真实路径且文档 URI 上会失败，需退回 `copyFile(fd)` |

**五个实测坑（易复发的实现细节）**：

1. **`responseCode` 成功为 `0`**——SDK 明确「0 means that the task is successful」。按 HTTP 语义判 `!== 200` 会把**每一个成功分片都判成失败**（首轮实测即如此：三轮重试后报「还有 2 个分片未上传成功」而其实都传上去了）。判定写作 `0 || 200` 兼容。
2. **进度必须回调新对象**——`@Local` 按引用比对，反复回传同一个 `this.view` 会使 UI 只渲染第一次的值（实测现象：进度条停在 10%、阶段一直显示「正在校验文件」，而日志里其实已上传完成）。`UploadController.publish()` 每次构造新对象。
3. **字节进度不能按 `片数 × chunk_size` 估算**——末尾片通常不满，7MB 文件会算出「已传 10MB / 总 7MB」的 **142%**。`completedBytes()` 逐片按真实区间长度累加。
4. **`fileIo.copy` 在 picker URI 上会失败**，且目标不能是 `internal://cache/…`；`copyFile(fd, dest)` 是可用退路（372MB 实测通过）。
5. **不用 `Progress` 组件自带的百分比文案**——本 SDK 的 `Progress` 无 `showDefaultPercentage`，未设 `style` 时会在轨道上叠出内置的 `10.000000`。进度条改为自绘（外层轨道 + 内层定宽填充）。

**端到端验证（模拟器 API 26，`Pura X View`；判定不读端侧自述）**：

| 场景 | 判据 | 结果 |
| --- | --- | --- |
| ① 完整上传 7MB / ② 续传 11MB / ④ 进度斜坡 30MB | `complete` 返回 `file_id`；**宿主对合并成品重算 SHA-256 与文件名逐字节比对** | **MATCH**（7340032 / 11534336 / 31457280 三份） |
| ② 断点续传 | 手工先传 part0 → 重新 init 得 `uploaded=[0]`、`resumed=true`，控制器**只传 2 片**（1→3） | PASS，**未重传已传分片** |
| ③ 秒传 | 同用户重复上传同内容 → `fast=true`、`file_id` 与首次相同、**上传分片数 0** | PASS |
| ④ 进度可见（HMY-50 ③） | 上传阶段出现过的百分比档位 | `16,33,50,66,83,100`（6 片 6 档）——连续推进，非 0/100 两段跳 |
| ⑤ 选片交接 | picker URI 直投 vs 拷 cache 后投递 | 直投 **REJECTED(401)**／拷 cache 后 **ACCEPTED(rc=0)** |
| ⑥ 投稿接口接受度 | 用上传得到的 `file_id` 调 `POST /api/v1/videos` | 通过，返回 `bvid DV2VivO0Xq7CC`（`status=2` 转码中，已下发签名流） |
| ⑦ 真 UI 走查 | 「我的」→「投稿」→ 选片 → 开始上传 → 进度 → `file_id` | 阶段依次出现「正在校验文件 → 正在上传 → 正在合并文件 → 上传完成」 |

**须真机确认（模拟器结论不作数）**：长时任务的实际保活边界（锁屏 + 切后台 5 分钟）、真机吞吐与弱网重试、大文件（GB 级）端侧哈希耗时与 cache 占用。模拟器为 x86_64 软件渲染，且 `moveAbilityToBackground()` 可能直接返回 `16000065`。

**管理后台/RBAC**：本卡仅端侧改动，不涉及。

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

**应用图标（2026-09-28 定版，自研）**：不采用 DevEco 模板默认图标，也不套用上表可下载的图标资源包，自研品牌图标——品牌粉渐变底（`#FFD9E4` → `#FB7299` → `#E45C84`，三色全部取自仓库既有 token：`primary-light` / `primary` / 文档站暗色 -3）+ 白色**鲸标记**（饱满正圆的大头 + **闭合的嘴**（头左端即一条平滑圆弧，无楔形开口）+ 眼在头前部 + 贴着下缘的细弧胸鳍 + 厚实两叶尾鳍 + **头顶一簇简化小喷泉**（三颗纯白水滴）；播放三角以负空间挖在身体中段，保住播放语义）。分层图标资源落 `AppScope/resources/base/media/` 与 `entry/src/main/resources/base/media/`（`background` 满幅渐变不带圆角 / `foreground` 白色图形 + 透明底，均 1024×1024，**较长边占画布 67%**），启动图标 `startIcon.png` 152×152 圆角烘焙。图形共五版，前四版均已取代：首版（2026-09-21）「字母 D 内孔挖播放三角」+ 渐变 `#FC8BAB`→`#F2557F`；二版（同日）「脉冲播放标记」（播放三角 + 两道向右鼓出的同心弧）；三版（同日）「简化鲸」（钝头低长身 + 嘴缝 + 浅 V 尾鳍 + 喷水）；四版（同日）手写矢量的具象鲸 H3 ——前四版皆因**太抽象**或形态不满意被否决，五版改为**调生图模型出鲸再二次改进**（清理成纯白 + 透明底的 1024 位图），并于同日**二次定版**为「闭合的嘴 + 头顶简化小喷泉」（用户否决了生图初版开合的嘴，并要求头上加喷泉）。注意 `#F2557F` 在仓库内无 token 对应，首版文档「起点即 `brand_primary_hover` token」的口径只对了一个色标。**全端品牌图形单一来源**为 `assets/brand/`（`logo-bg` / `logo-rim` / `cover-default` 三个 SVG 底板 + `logo-mark` / `logo-mark-small` 两个 PNG 标记），全部产物（Web favicon 三件套、各端顶栏与登录页 `logo.png`、鸿蒙分层图标与启动图标、三端默认封面）由 `node scripts/svg2png.mjs` 派生——`apps/web/public/favicon.svg` 亦是该脚本的合成产物、不再是手写源文件，改版只改 `assets/brand/` 后重跑脚本。视觉取向为液态玻璃：渐变底板叠黑/白透明度的景深、暗角、扫光与高光，标记与倒角（`logo-rim` 的上亮下暗镜面环）在合成阶段叠加，圆角与倒角**不进源文件**（鸿蒙遮罩形状由系统决定，源文件必须满幅无边）。落地与实测记录见 [tasks M4-HMY-02](/specs/harmony/tasks)。

### 7.2 对口设计规范（约束端侧实现）

| 规范 | 关键结论 |
| --- | --- |
| [沉浸光感](https://developer.huawei.com/consumer/cn/doc/design-guides/immersivelight-0000002612101053) | 材质层级枚举从 `ULTRA_THIN` 到 `ULTRA_THICK` 共 5 档；顶部悬浮用 `ULTRA_THIN`（+渐变模糊）、底部悬浮用 `THIN`（+渐变蒙层）、任意位置弹出用 `THICK`、半模态/弹窗用 `ULTRA_THICK`。用户侧三档强度由系统映射，开发只定义一次 |
| [影音娱乐场景最佳实践](https://developer.huawei.com/consumer/cn/doc/design-guides/responsive-design-examples1-0000001957369849) | ① **沉浸全屏播放**：上下有黑边时弹幕**仅在上方黑边区域内**显示；无黑边时同屏弹幕不宜过多（直接约束 HMY-20/HMY-24）② 全屏下选集/倍速等临时操作走**侧边或底部面板**（手机横屏/折叠态/平板从右侧，折叠屏展开态从底部）③ 方形屏（折叠屏展开态）点全屏**不旋转** ④ 折叠屏悬停态自动切沉浸播放 ⑤ 宫格：折叠屏/平板竖屏 3 列、平板横屏 5 列，支持双指缩放调列数 ⑥ 建议支持**画中画**，首次使用调起授权弹窗 ⑦ 播放页全局滑动：右滑出上级瀑布流、左滑出右侧 UP 主详情 ⑧ 搜索框"一镜到底"不切层级 |
| [圆角参数](https://developer.huawei.com/consumer/cn/doc/design-guides/corner-radius-parameter-0000002556468705) | 端侧圆角取值参照系统规范，与 Web 端 `radius-sm/md/lg`（6/8/12）映射关系需在 M4-HMY-02 对齐 |

### 7.3 沉浸光感开发侧文档（API 26 已就位，本期主路线）

- [沉浸光感简介](https://developer.huawei.com/consumer/cn/doc/harmonyos-guides/arkts-immersive-light-sense-overview) / [开启沉浸光感](https://developer.huawei.com/consumer/cn/doc/harmonyos-guides/arkts-immersive-light-sense-enable) / [功耗优化](https://developer.huawei.com/consumer/cn/doc/harmonyos-guides/arkts-immersive-light-sense-constraints) / [组件适配](https://developer.huawei.com/consumer/cn/doc/harmonyos-guides/arkts-immersive-light-sense-component-adaptation) / [沉浸式系统材质视效](https://developer.huawei.com/consumer/cn/doc/harmonyos-guides/arkts-immersive-light-sense-common-capability)（反色、材质赋色、交互形变、点光源、阴影开关）
- [一多开发实例（长视频）](https://developer.huawei.com/consumer/cn/doc/best-practices/multi-video-app)：折叠屏/平板播放页一多布局官方参考实现
