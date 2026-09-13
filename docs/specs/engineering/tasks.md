# tasks：工程与端侧（横切）

> 本目录收编**跨模块工程任务**（基建/发布准备/多端适配/架构演进），不设独立 spec——技术基线以 [architecture/](/architecture/overview) 与 [非功能需求](/product/nfr) 为准。
> 任务编号：`{阶段}-{模块}-{序号}`；完成即勾选并追加完成日期。

## M0（W1-W4）基建

- [x] M0-ENG-01 Monorepo 初始化（pnpm workspace + 目录规划） `2026-07-28`
  - 覆盖：—（工程）
- [x] M0-ENG-02 VitePress 文档系统搭建 `2026-07-28`
  - 覆盖：—（工程）
- [x] M0-ENG-03 产品/架构/项目管理文档初稿 `2026-07-28`
  - 覆盖：—（工程）
- [x] M0-ENG-04 Go 后端脚手架（cmd/api、internal 分层、Gin 接入） `2026-07-28`
  - 覆盖：—（工程）
- [x] M0-ENG-05 配置管理（viper 多环境）+ 结构化日志（zap）+ 全局错误处理 `2026-07-28`
  - 覆盖：—（工程）
- [x] M0-ENG-06 统一响应/错误码包（pkg/response、pkg/errcode） `2026-07-28`
  - 覆盖：—（工程）
- [x] M0-ENG-07 数据库迁移工具（golang-migrate）+ 初始 schema `2026-07-28`
  - 覆盖：—（工程）
- [x] M0-ENG-08 docker-compose 开发环境（MySQL/Redis/Kafka/MinIO，ES 延至 M2） `2026-07-28`
  - 覆盖：—（工程）
- [x] M0-ENG-09 CI 流水线：Go vet+test+build、Web lint+typecheck+build、文档构建 `2026-07-28`
  - 覆盖：—（工程）
- [x] M0-ENG-10 Web 前端脚手架（Vue3+Vite+TS+Pinia+Router+Element Plus，含 ESLint、登录页框架、后端联通验证） `2026-07-28`
  - 覆盖：—（工程）
- [x] M0-ENG-11 packages/api-client 骨架（请求封装、token 拦截、401 回调、适配器模式、系统接口） `2026-07-28`
  - 覆盖：—（工程）
- [x] M0-ENG-12 packages/shared 骨架（统一响应类型、错误码/状态常量、格式化工具） `2026-07-28`
  - 覆盖：—（工程）
- [x] M0-ENG-13 staging 环境部署脚本 + HelloWorld API 上线验证（configs/staging.yaml（独立端口 8100/库 dlidli_staging/Redis db1/uploads_staging/自动过审）+ deploy/staging.ps1（构建→迁移→启动→/health 与 /api/v1/ping 验证→汇总）；修复迁移文件历史冲突：0001 与 0005 重复建 relation、0022 与收藏夹 collection 冲突（0022 改为 video_collection/0025 no-op）；实测 staging 全流程通过且与 dev 完全隔离） `2026-08-06`
  - 覆盖：—（工程）

## M1（W5-W12）发布准备

- [x] M1-REL-01 核心链路压测（scripts/k6/core-load.js：首页推荐→视频详情→弹幕列表→搜索，匿名 5→20 VU 60s + 登录链路；阈值 failed<1%/p95<500ms；实测 2964 请求 0 失败、p95=384ms、checks 99.96%；k6 v0.57.0 二进制被 *.exe 忽略不入库） `2026-08-07`
  - 覆盖：—（工程）
- [x] M1-REL-02 监控告警（Prometheus + Grafana 基础面板）（后端 /metrics 已暴露：dlidli_http 请求量/耗时直方图/in-flight + Go 运行时指标；deploy/monitoring.ps1 便携版启动 Prometheus(9090)+Grafana(3000)（无 Docker），provisioning 自动加载 Prometheus 数据源 + DliDli 基础监控面板；实测 target up/指标可查询/面板加载全通；docker-compose 双配置（本地/docker 抓取目标）） `2026-08-10`
  - 覆盖：—（工程）
- [x] M1-REL-03 内测环境部署 + 种子内容准备（≥50 个视频）（种子脚本：FFmpeg 生成 10 源×5 UP 投 50 稿→转码→批量过审，发布总数 56；全流程走查：首页/播放/赞币评弹/搜索全通；部署待 staging 环境） `2026-07-30`
  - 覆盖：—（工程）
- [x] M1-REL-04 内测邀请机制（邀请码注册开关）（invite_code 表（0030 迁移）+ app.inviteCodeRequired 开关（默认关）；注册（短信自动注册/邮箱注册）开启时必填一次性邀请码，条件 UPDATE 原子占用；admin POST /admin/invite-codes 批量生成（权限 config:edit）；随机码去易混淆字符 + 过期/已用校验单测；go build/vet/test 全绿） `2026-08-28`
  - 覆盖：ACC-44

## M2（W13-W24）H5 端

- [x] M2-H5-01 uni-app 工程搭建 + api-client 适配 uni.request（apps/h5，vite-ts 模板，复用 workspace 共享包，dev :5176） `2026-07-30`
  - 覆盖：—（工程）
- [x] M2-H5-02 首页/分区/搜索页（首页视频流+分区+最新/最热+下拉刷新+触底加载已完成；搜索页已完成：搜索框+视频/用户双Tab+结果列表+首页搜索入口） `2026-07-30`
  - 覆盖：—（工程）
- [x] M2-H5-03 播放页（含弹幕展示/发送、互动栏）（原生 video 播 HLS+有效播放上报+UP主/三连数据/简介已完成；互动栏已完成：点赞/投币/收藏/分享四按钮+弹幕发送栏） `2026-07-30`
  - 覆盖：—（工程）
- [x] M2-H5-04 个人中心/空间/历史/收藏（**完整交付 2026-08-28**：个人中心页+短信登录入口+首页入口（profile.vue：未登录短信登录 dev 自动 debug_code、已登录用户卡片/我的投稿/收藏夹/退出，api 补 saveLogin/clearLogin/hasLogin）；收藏页（collection.vue：收藏视频/收藏夹 Tab，interaction.favorites+listCollections）；空间页（space.vue：relation.profile/stat + video.list({uid})，UP 卡片+投稿网格+下拉刷新/触底分页，视频页 UP 区可跳空间）；历史页（history.vue：新增后端 GET /videos/history（SaveProgress 同步写 Redis zset 记录观看时间戳）+ api-client video.history，最近观看倒序分页）；typecheck + uni build 全通） `2026-08-28`
  - 覆盖：—（工程）
- [x] M2-H5-05 消息中心 + 动态页（**完整交付 2026-09-03**：消息中心+私信对话（messages.vue 通知/私信 Tab + im.vue 自定义导航/气泡/发送）+ 动态页（feed.vue：发动态输入+关注动态流（api.dynamic.feed cursor 分页）+转发视频卡片点击进播放+列表顶部下拉刷新+触底加载，个人中心入口；复用完全固定头部+内部 scroll-view 布局）；typecheck + uni build 全通） `2026-09-03`
  - 覆盖：—（工程）
- [x] M2-H5-06 微信内浏览器适配 + 分享 JSSDK（`apps/h5/src/utils/wechat.ts`：UA 检测 isWeChat/版本 + jweixin-1.6.0 动态加载 + wx.config 分享卡片（后端签名未启用时静默降级，卡片标题取 document.title 兜底）；视频页：`playsinline`/`webkit-playsinline`/`x5-video-player-type="h5-page"` 防 iOS/微信 X5 全屏劫持，微信内禁 autoplay 引导手动起播，加载后配置分享卡片（title/desc/link/imgUrl）；后端新增 `wechat` 模块：jsapi_ticket/access_token Redis 缓存（6600s 提前过期）+ 官方 SHA1 签名算法 + `GET /wechat/jssdk-sign?url=`（未配置 appId 返回"微信分享未启用"），config 加 `wechat.appId/appSecret`（环境变量可覆盖）；签名算法/nonce/未启用 3 个单测，20 个测试包全绿，签名接口实测返回未启用降级） `2026-09-08`
  - 覆盖：—（工程）

## M3（W25-W48）小程序与架构演进

### 小程序（MP）

- [x] M3-MP-01 编译目标 mp-weixin 适配（登录改微信授权）：H5 uni-app 工程新增 mp-weixin 平台依赖与 `dev:mp`/`build:mp` 脚本；`apps/miniprogram` README 明确共享工程与构建产物导入方式；profile 页在 MP-WEIXIN 条件编译下提供微信一键登录；后端新增 `POST /auth/login/wechat`，通过小程序 `code2session` 换 openid 后查找/自动注册账号并签发 JWT；配置支持 `DLIDLI_WECHAT_MPAPPID`/`DLIDLI_WECHAT_MPAPPSECRET`，凭据缺失优雅降级。H5 与 mp-weixin 构建、Go vet/test 全绿 `2026-09-11`
  - 覆盖：—（工程）
- [x] M3-MP-02 核心页面：首页/搜索/播放/个人中心：复用 H5 uni-app 页面作为共享源码，在 `mp-weixin` 目标下生成 `pages/index`、`pages/search`、`pages/video`、`pages/profile` 四个页面产物；主包构建产物约 203KB，页面编译无错误 `2026-09-11`
  - 覆盖：—（工程）
- [ ] M3-MP-03 微信卡片分享 + 类目资质提审：播放页已接入 `onShareAppMessage`（好友卡片）与 `onShareTimeline`（朋友圈），分享路径携带 `bvid`、封面使用视频封面并带默认封面兜底；真实分享菜单/卡片验证与视频类目资质提审仍需真实小程序 AppID、开发者账号及资质，待外部条件具备后验收 `2026-09-11`
  - 覆盖：—（工程）

### 基建演进（ENG）

- [ ] M3-ENG-01 微服务拆分：互动/计数服务独立（gRPC）
  - 覆盖：—（工程）
- [ ] M3-ENG-02 Kubernetes 迁移 + HPA
  - 覆盖：—（工程）
- [ ] M3-ENG-03 分表实施（comment/danmaku/user_action）
  - 覆盖：—（工程）
- [x] M3-ENG-04 链路追踪全覆盖（OpenTelemetry）：新增可选 OTLP/HTTP TracerProvider 与 Gin server-span 中间件；支持 `traceparent`/`baggage` 入站上下文传播、HTTP 路由/状态/耗时/request-id 属性、5xx span error、批量导出与优雅 shutdown；`TRACING_ENDPOINT` 为空时 noop，不改变本地默认行为；补 tracing 单测，go test/vet/build 全绿 `2026-09-13`
  - 覆盖：—（工程）
- [x] M3-ENG-05 后端核心层单测补全 + 中间件组合顺序缺陷修复（middleware：TraceID/Auth/OptionalAuth/AdminAuth/CORS/Recovery/PlaySignGuard/限流 fail-open/组合中间件；pkg：storage 本地驱动含跨平台路径穿越防护、config 默认值、contentmod 规则机审、moderate 词库热加载；**修复** v0.23.1 引入的 Chain 组合顺序缺陷——Auth 尾部 c.Next() 直通业务导致限流器后置执行，改为单一 AuthedRateLimited 中间件保证限流先于业务；go build/vet/test 全绿） `2026-08-25`
  - 覆盖：—（工程）
- [x] M3-ENG-06 Web 视频详情页巨型组件拆分（VideoView 1743 行 → 1375 行）：script 逻辑抽为 4 个组合式（useVideoPlayer/useDanmakuController/useVideoActions/usePlaybackReport），模板与样式零改动、行为不变；vue-tsc + vite build + eslint 全绿） `2026-08-25`
  - 覆盖：—（工程）
- [x] M3-ENG-07 Web 投稿页巨型组件拆分（UploadView 728 行 → 594 行）：script 逻辑抽为 3 个组合式（useUploadParts 分P 管理/useUploadCover 封面三级兜底/useUploadForm 表单与投稿），模板与样式零改动、行为不变；vue-tsc + vite build + eslint 全绿） `2026-08-25`
  - 覆盖：—（工程）
- [x] M3-ENG-08 Web 首页巨型组件拆分（HomeView 720 行 → 595 行）：script 逻辑抽为 2 个组合式（useHomeFeed 视频流/无限滚动/曝光上报/useHomeBanners 推荐区轮播），模板与样式零改动、行为不变；vue-tsc + vite build + eslint 全绿） `2026-08-25`
  - 覆盖：—（工程）
- [x] M3-ENG-09 Web 个人空间巨型组件拆分（SpaceView 642 行 → 535 行）：script 逻辑抽为 3 个组合式（useSpaceProfile 头部资料与关注/useSpaceCollections 合集管理/useSpaceTabs 五 Tab 内容加载），模板与样式零改动、行为不变；vue-tsc + vite build + eslint 全绿） `2026-08-25`
  - 覆盖：—（工程）
- [x] M3-ENG-10 Web 创作者中心巨型组件拆分（CreatorView 613 行 → 473 行）：script 逻辑抽为 4 个组合式（useCreatorOverview 概览/useCreatorTrend echarts 趋势图/useCreatorVideos 稿件分页/useCreatorSettles 收益明细），模板与样式零改动、行为不变；vue-tsc + vite build + eslint 全绿） `2026-08-25`
  - 覆盖：—（工程）
- [x] M3-ENG-11 Web 私信页巨型组件拆分（MessagesView 588 行 → 452 行）：script 逻辑抽为 3 个组合式（useConversations 会话与消息/useBlockActions 拉黑/useMessagesWs WebSocket 实时接收与重连），模板与样式零改动、行为不变；vue-tsc + vite build + eslint 全绿） `2026-08-25`
  - 覆盖：—（工程）
- [x] M3-ENG-12 Web 账号设置页巨型组件拆分（SettingsView 494 行 → 306 行）：script 逻辑抽为 5 个组合式（useProfileSettings 资料与头像/usePasswordChange 改密/useYouthMode 青少年模式计时/useDmBlocks 弹幕屏蔽/useRecommendSetting 推荐合规开关），模板与样式零改动、行为不变；vue-tsc + vite build + eslint 全绿） `2026-08-25`
  - 覆盖：—（工程）
- [x] M3-ENG-13 跨端令牌逻辑收敛进 api-client（新增 token.ts：TokenStorage 抽象 + createLocalStorageTokens + refreshTokens 共享静默续期；web 改用共享实现删除自建刷新，h5 补齐此前缺失的 401 静默续期能力；admin 保持管理员会话独立无刷新设计；api-client 保持端无关不引入 uni 全局） `2026-08-25`
  - 覆盖：—（工程）
- [x] M3-ENG-14 一键启动所有服务（scripts/dev-all.mjs + 根 `dev:all` 脚本）：依赖探测（MySQL 3307/Redis 6379，可用 DLIDLI_* 覆盖）→ 数据库迁移（go run ./cmd/migrate）→ go build 后端二进制再启动 API（避免 go run 子进程树难清理）→ 并发生起 web/admin/h5（可选 --docs 起文档站）→ 轮询 /health 就绪确认 → Ctrl+C 统一 taskkill /T 清理全部子进程；带彩色标签日志；--check-only / --skip-migrate / --no-h5 / --strict 开关；实测 MySQL+Redis 就绪、迁移完成、API /health 200、web(5173)/admin(5175)/h5(5176) vite ready 全通） `2026-09-03`
  - 覆盖：—（工程）
- [x] M3-ENG-15 Web 提示词实验室页（`/pelican`，提示词产物演示）：新增独立页面 views/lab/PelicanView.vue（提示词原文可一键复制 + 内联手绘 SVG），SVG 全手绘无外部依赖——白色鹈鹕（长橙喙/下垂喉囊/前伸翅膀/橙蹼足）+ 蔚蓝色自行车 + 周围风景（太阳/云朵/远山/树木/带虚线公路）；骑行动画由 CSS 关键帧驱动（车轮旋转 0.8s / 云朵漂移 22s / 公路虚线流动 0.9s / 速度线脉动 1s / 车身起伏 1.7s），`prefers-reduced-motion` 下自动静止；路由免登录；经真实浏览器校验动画生效、vue-tsc + eslint(0 error) + vite build 全绿 `2026-09-11`
  - 覆盖：—（工程）
- [x] M3-ENG-16 Web 提示词实验室·夜骑版（`/pelican-night`，同题异风格对照）：新增独立页面 views/lab/PelicanNightView.vue（提示词原文可一键复制 + 内联手绘 SVG），与 `/pelican` 日间水彩版构成同一提示词下的两条视觉路线对照；SVG 全手绘无外部依赖——夜色霓虹方向：深蓝紫渐变夜空 + 明月与三组错峰闪烁繁星 + 亮窗城市天际线（pattern 窗火/楼顶信号灯）+ 带流动虚线公路；鹈鹕主体沿用可信解剖（长橙喙/下垂喉囊含小鱼尾/前伸翅膀/橙色蹼足），新增月光冷边（青蓝描边）与腹部暖反光以贴合夜景；自行车为霓虹配色，车头灯暖黄光束（呼吸）+ 车尾红色脉动尾灯 + 车后青蓝/品红速度光轨；动效由 CSS 关键帧驱动（车轮旋转 0.75s / 公路虚线 0.85s / 星光三档闪烁 2.6~3.8s / 车灯呼吸 3.4s / 尾灯脉动 1.6s / 车身起伏 1.9s），`prefers-reduced-motion` 下全部静止；路由免登录、随主布局；经组件内 SVG 提取栅格化肉眼校验构图与解剖、vue-tsc + eslint(0 error) + vite build 全绿 `2026-09-11`
  - 覆盖：—（工程）

## M4（W49+）App

- [ ] M4-APP-01 技术选型决策（uni-app 打包 vs Flutter）
  - 覆盖：—（工程）
- [ ] M4-APP-02 核心功能移植 + 离线缓存 + 推送
  - 覆盖：—（工程）
- [ ] M4-APP-03 应用商店上架
  - 覆盖：—（工程）

## 进度

| 里程碑 | 任务数 | 已完成 |
| --- | :-: | :-: |
| M0 | 13 | 13 |
| M1 | 4 | 4 |
| M2 | 6 | 6 |
| M3 | 20 | 15 |
| M4 | 3 | 0 |
| **合计** | **46** | **38** |

> 勾选任务后同步更新上表与 [开发进度管理](/project/progress) 的模块矩阵。
