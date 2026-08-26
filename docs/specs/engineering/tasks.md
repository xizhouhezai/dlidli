# tasks：工程与端侧（横切）

> 本目录收编**跨模块工程任务**（基建/发布准备/多端适配/架构演进），不设独立 spec——技术基线以 [architecture/](/architecture/overview) 与 [非功能需求](/product/nfr) 为准。
> 任务编号：`{阶段}-{模块}-{序号}`；完成即勾选并追加完成日期。

## M0（W1-W4）基建

- [x] M0-ENG-01 Monorepo 初始化（pnpm workspace + 目录规划） `2026-07-28`
- [x] M0-ENG-02 VitePress 文档系统搭建 `2026-07-28`
- [x] M0-ENG-03 产品/架构/项目管理文档初稿 `2026-07-28`
- [x] M0-ENG-04 Go 后端脚手架（cmd/api、internal 分层、Gin 接入） `2026-07-28`
- [x] M0-ENG-05 配置管理（viper 多环境）+ 结构化日志（zap）+ 全局错误处理 `2026-07-28`
- [x] M0-ENG-06 统一响应/错误码包（pkg/response、pkg/errcode） `2026-07-28`
- [x] M0-ENG-07 数据库迁移工具（golang-migrate）+ 初始 schema `2026-07-28`
- [x] M0-ENG-08 docker-compose 开发环境（MySQL/Redis/Kafka/MinIO，ES 延至 M2） `2026-07-28`
- [x] M0-ENG-09 CI 流水线：Go vet+test+build、Web lint+typecheck+build、文档构建 `2026-07-28`
- [x] M0-ENG-10 Web 前端脚手架（Vue3+Vite+TS+Pinia+Router+Element Plus，含 ESLint、登录页框架、后端联通验证） `2026-07-28`
- [x] M0-ENG-11 packages/api-client 骨架（请求封装、token 拦截、401 回调、适配器模式、系统接口） `2026-07-28`
- [x] M0-ENG-12 packages/shared 骨架（统一响应类型、错误码/状态常量、格式化工具） `2026-07-28`
- [x] M0-ENG-13 staging 环境部署脚本 + HelloWorld API 上线验证（configs/staging.yaml（独立端口 8100/库 dlidli_staging/Redis db1/uploads_staging/自动过审）+ deploy/staging.ps1（构建→迁移→启动→/health 与 /api/v1/ping 验证→汇总）；修复迁移文件历史冲突：0001 与 0005 重复建 relation、0022 与收藏夹 collection 冲突（0022 改为 video_collection/0025 no-op）；实测 staging 全流程通过且与 dev 完全隔离） `2026-08-06`

## M1（W5-W12）发布准备

- [x] M1-REL-01 核心链路压测（scripts/k6/core-load.js：首页推荐→视频详情→弹幕列表→搜索，匿名 5→20 VU 60s + 登录链路；阈值 failed<1%/p95<500ms；实测 2964 请求 0 失败、p95=384ms、checks 99.96%；k6 v0.57.0 二进制被 *.exe 忽略不入库） `2026-08-07`
- [x] M1-REL-02 监控告警（Prometheus + Grafana 基础面板）（后端 /metrics 已暴露：dlidli_http 请求量/耗时直方图/in-flight + Go 运行时指标；deploy/monitoring.ps1 便携版启动 Prometheus(9090)+Grafana(3000)（无 Docker），provisioning 自动加载 Prometheus 数据源 + DliDli 基础监控面板；实测 target up/指标可查询/面板加载全通；docker-compose 双配置（本地/docker 抓取目标）） `2026-08-10`
- [x] M1-REL-03 内测环境部署 + 种子内容准备（≥50 个视频）（种子脚本：FFmpeg 生成 10 源×5 UP 投 50 稿→转码→批量过审，发布总数 56；全流程走查：首页/播放/赞币评弹/搜索全通；部署待 staging 环境） `2026-07-30`
- [ ] M1-REL-04 内测邀请机制（邀请码注册开关）

## M2（W13-W24）H5 端

- [x] M2-H5-01 uni-app 工程搭建 + api-client 适配 uni.request（apps/h5，vite-ts 模板，复用 workspace 共享包，dev :5176） `2026-07-30`
- [x] M2-H5-02 首页/分区/搜索页（首页视频流+分区+最新/最热+下拉刷新+触底加载已完成；搜索页已完成：搜索框+视频/用户双Tab+结果列表+首页搜索入口） `2026-07-30`
- [x] M2-H5-03 播放页（含弹幕展示/发送、互动栏）（原生 video 播 HLS+有效播放上报+UP主/三连数据/简介已完成；互动栏已完成：点赞/投币/收藏/分享四按钮+弹幕发送栏） `2026-07-30`
- [ ] M2-H5-04 个人中心/空间/历史/收藏
- [ ] M2-H5-05 消息中心 + 动态页
- [ ] M2-H5-06 微信内浏览器适配 + 分享 JSSDK

## M3（W25-W48）小程序与架构演进

### 小程序（MP）

- [ ] M3-MP-01 编译目标 mp-weixin 适配（登录改微信授权）
- [ ] M3-MP-02 核心页面：首页/搜索/播放/个人中心
- [ ] M3-MP-03 微信卡片分享 + 类目资质提审

### 基建演进（ENG）

- [ ] M3-ENG-01 微服务拆分：互动/计数服务独立（gRPC）
- [ ] M3-ENG-02 Kubernetes 迁移 + HPA
- [ ] M3-ENG-03 分表实施（comment/danmaku/user_action）
- [ ] M3-ENG-04 链路追踪全覆盖（OpenTelemetry）
- [x] M3-ENG-05 后端核心层单测补全 + 中间件组合顺序缺陷修复（middleware：TraceID/Auth/OptionalAuth/AdminAuth/CORS/Recovery/PlaySignGuard/限流 fail-open/组合中间件；pkg：storage 本地驱动含跨平台路径穿越防护、config 默认值、contentmod 规则机审、moderate 词库热加载；**修复** v0.23.1 引入的 Chain 组合顺序缺陷——Auth 尾部 c.Next() 直通业务导致限流器后置执行，改为单一 AuthedRateLimited 中间件保证限流先于业务；go build/vet/test 全绿） `2026-08-25`
- [x] M3-ENG-06 Web 视频详情页巨型组件拆分（VideoView 1743 行 → 1375 行）：script 逻辑抽为 4 个组合式（useVideoPlayer/useDanmakuController/useVideoActions/usePlaybackReport），模板与样式零改动、行为不变；vue-tsc + vite build + eslint 全绿） `2026-08-25`
- [x] M3-ENG-07 Web 投稿页巨型组件拆分（UploadView 728 行 → 594 行）：script 逻辑抽为 3 个组合式（useUploadParts 分P 管理/useUploadCover 封面三级兜底/useUploadForm 表单与投稿），模板与样式零改动、行为不变；vue-tsc + vite build + eslint 全绿） `2026-08-25`
- [x] M3-ENG-08 Web 首页巨型组件拆分（HomeView 720 行 → 595 行）：script 逻辑抽为 2 个组合式（useHomeFeed 视频流/无限滚动/曝光上报/useHomeBanners 推荐区轮播），模板与样式零改动、行为不变；vue-tsc + vite build + eslint 全绿） `2026-08-25`
- [x] M3-ENG-09 Web 个人空间巨型组件拆分（SpaceView 642 行 → 535 行）：script 逻辑抽为 3 个组合式（useSpaceProfile 头部资料与关注/useSpaceCollections 合集管理/useSpaceTabs 五 Tab 内容加载），模板与样式零改动、行为不变；vue-tsc + vite build + eslint 全绿） `2026-08-25`

## M4（W49+）App

- [ ] M4-APP-01 技术选型决策（uni-app 打包 vs Flutter）
- [ ] M4-APP-02 核心功能移植 + 离线缓存 + 推送
- [ ] M4-APP-03 应用商店上架

## 进度

| 里程碑 | 任务数 | 已完成 |
| --- | :-: | :-: |
| M0 | 13 | 13 |
| M1 | 4 | 3 |
| M2 | 6 | 3 |
| M3 | 12 | 5 |
| M4 | 3 | 0 |
| **合计** | **38** | **24** |

> 勾选任务后同步更新上表与 [开发进度管理](/project/progress) 的模块矩阵。
