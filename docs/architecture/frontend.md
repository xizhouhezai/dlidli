# 前端架构（Web / H5 / 小程序）

> 状态：草案 ｜ 更新日期：2026-07-29

## 1. Monorepo 前端布局

```
apps/
├── web/            # PC Web：Vue 3 + Vite + TS + Pinia + Vue Router
├── h5/             # H5：uni-app(Vue 3) 编译 H5 目标
├── miniprogram/    # 微信小程序：与 h5 共用 uni-app 工程（多编译目标）或独立工程
└── admin/          # 管理后台前端（独立应用，与 C 端 web 分离部署；dev :5175）
packages/
├── api-client/     # OpenAPI 生成的 TS 类型 + 请求封装（多端适配 fetch/uni.request）
├── shared/         # 工具函数、常量、业务模型（纯 TS，无环境依赖）
├── ui/             # PC 组件库封装（基于 Element Plus 二次封装）
└── player/         # 播放器内核封装（hls.js + 弹幕渲染引擎，Web/H5 共用）
```

> 决策：H5 与小程序合并为一个 uni-app 工程（`apps/h5` 与 `apps/miniprogram` 起步阶段可指向同一套源码的不同编译目标），最大化复用；PC Web 独立工程，交互形态差异大不强行复用视图层。

## 2. 各端技术栈

| 端 | 框架 | UI | 状态 | 说明 |
| --- | --- | --- | --- | --- |
| Web | Vue 3 + Vite + TS | Element Plus + UnoCSS/SCSS | Pinia | SPA；SEO 关键页（视频详情）V2 评估 SSR/预渲染 |
| H5 | uni-app (Vue 3) | uni-ui + UnoCSS/SCSS | Pinia | 编译目标 H5 |
| 小程序 | uni-app 同工程 | 同上 | Pinia | 编译目标 mp-weixin |
| App（远期） | uni-app 打包 App 或独立 Flutter | - | - | V3 决策 |
| 管理后台 | Vue 3 + Vite + TS | Element Plus | 无（页面级状态） | 独立应用 `apps/admin`；管理员令牌与 C 端会话完全隔离，登录/审核工作台已上线，用户管理/分区管理/敏感词库逐步补齐 |

## 3. 关键模块设计

### 3.1 播放器（packages/player）

- 内核：`hls.js`（Web/H5）；小程序用原生 `<video>` 传 HLS 地址。
- 分层：`core`（加载/清晰度/进度）+ `danmaku`（Canvas 弹幕渲染，碰撞检测轨道分配）+ `ui`（控制栏皮肤，各端注入）。
- 埋点：起播耗时、卡顿、清晰度切换事件统一上报。

### 3.2 API 层（packages/api-client）

- 后端 OpenAPI → `openapi-typescript` 生成类型，杜绝手写接口类型。
- 请求适配器模式：Web/H5 用 `fetch`，小程序用 `uni.request`。
- 统一拦截：token 注入、401 自动刷新重试、错误码 toast 映射、trace_id 透传。

### 3.3 上传模块（Web）

- 分片上传（5MB）+ 并发 3 + 断点续传（localStorage 记录已传分片）+ 秒传（先算 hash 询问）。
- Web Worker 计算文件 hash，避免阻塞 UI。

### 3.4 样式方案（UnoCSS + SCSS）

- **UnoCSS**：原子化 CSS，web 用 `presetUno + presetAttributify + presetIcons`，h5 用 `presetUno`（小程序属性化受限）；布局/间距/字号等高频样式直接写在模板类名。
- **SCSS**：交互态（hover/active）、`:deep()`、复杂选择器仍用局部 `<style scoped lang="scss">`；h5 保留 rpx 适配。
- **品牌 token 单一真源**：`apps/web/src/styles/_variables.scss`（SCSS 变量+mixin）、`uno.config.ts` theme、`main.scss` 导出的 `--dli-*` CSS 变量三处同源；旧页面 `var(--dli-*)` 写法零改动兼容，新页面用 Uno/SCSS，渐进式迁移。h5 同源变量在 `apps/h5/src/styles/variables.scss`。
- **坑点**：uni-app 旧版 vite 配置加载器用 require 读取，`unocss/vite`（ESM-only）需用异步 `defineConfig(async () => { const UnoCSS = (await import('unocss/vite')).default })` 惰性加载；h5 SCSS 中 `@use` 用相对路径（uni sass 解析器不认 `@` 别名）。

## 4. 工程规范

### 4.0 公用抽取规范（DRY，强制）

> 核心原则：**同一段逻辑/结构出现第 2 次就抽公用**，禁止复制粘贴。新增/修改页面时，优先复用已有公用资产；发现重复先抽再写。

**抽取到哪里（分层归属）**：

| 类型 | 位置 | 说明 |
| --- | --- | --- |
| 纯函数（无框架依赖） | `packages/shared/src` | 格式化、时间/数字处理、业务常量、错误归一化；跨端复用 |
| 跨端组合式逻辑（纯 Vue reactive） | `packages/shared` 或各端 `composables` | 不依赖 Element Plus/uni 的可进 shared |
| 带 UI 库依赖的组合式逻辑 | `apps/<app>/src/composables/useXxx.ts` | 依赖 ElMessage/el-* 的单端 hook |
| 公用组件 | `apps/<app>/src/components` | 单端 UI；web 与 h5 模板不兼容（uni 标签）各自建 |
| 全局样式类 | 各 app `styles/main.scss` | 多页重复的 el-* 覆盖/按钮主题不写 scoped，提到全局 |

**命名与组织**：
- Composable 一律 `useXxx` 命名，一文件一 hook，放 `src/composables/`。
- 公用组件大驼峰命名，放 `src/components/`（跨页复用）或页面目录下 `components/`（仅本页子组件）。
- 纯函数进 `packages/shared` 后从 `@dlidli/shared` 导入，不得在组件内重写。

**已沉淀的公用资产**（新页面优先复用）：
- `@dlidli/shared`：`formatCount/formatDuration/formatPubdate`、`apiErrorMessage(err, fallback)` 错误文案归一化。
- **admin**：`composables/useApiAction`（异步操作 loading + 错误 toast 兜底 + cancel 忽略）、`composables/usePagedList`（分页列表 page/total/loading/reload/onPageChange）、`components/PageHead.vue`（页面标题栏 + actions 插槽）、全局 `.pink-btn`（品牌粉按钮，不再逐页写 --el-button-* 覆盖）。
- **web**：`composables/useCountdown`（验证码倒计时，自动清理定时器）、`components/VideoCard.vue`（视频卡片，layout=grid/side，封装封面+时长角标+标题+播放/弹幕 meta）。

### 4.1 Web 路由与布局分层

- `App.vue` 仅作应用入口：全局初始化（登录态资料拉取）+ `<RouterView/>`，不含任何 UI。
- `layouts/MainLayout.vue`：全局头部 `AppHeader` + 内容区，业务页面均为其**子路由**。
- `components/layout/AppHeader.vue`：公用头部组件（导航/搜索/未读铃铛 60s 轮询/用户菜单），从 App.vue 抽出，随布局复用。
- 顶级独立路由（全屏、无头部）：`/login`、`/reset-password`。
- 登录守卫：`requiresAuth` 页未登录 → `/login?redirect=<原路径>`，登录成功后回跳（LoginView 消费 `route.query.redirect`）。
- 这样头部/拦截逻辑集中在布局层，新增受保护页只需挂 MainLayout 子路由并标 `meta.requiresAuth`。

### 4.2 图标方案（Iconify 主推）

#### 现状问题

当前全端图标使用 **emoji 字符**（🔍👍🪙⭐↪️🔔▶），存在：
- 跨平台/浏览器渲染不一致（Windows/macOS/Android 样式差异大）
- 无法控制颜色、大小、描边
- 不支持 hover/active 状态变色
- 不专业，影响品牌一致性

#### 图标集选型（✅ MingCute）

选定 **MingCute**（`@iconify-json/mingcute`，3324 个图标）作为主图标集，理由：
- 国人设计，圆角友好的现代 App 风格，高度贴合 B 站/中文应用设计语言
- **line / fill 双变体**：默认线性（`-line`），激活态切填充（`-fill`）——点赞/收藏变实心，与 B 站一致
- 自带 `danmaku`（弹幕）、`coin`、`fire` 等视频社区专用图标

**常用图标映射**（同一功能全端统一）：

| 用途 | line（默认） | fill（激活态） |
| --- | --- | --- |
| 搜索 | `i-mingcute-search-2-line` | — |
| 铃铛/通知 | `i-mingcute-notification-line` | — |
| 播放量 | `i-mingcute-play-circle-line` | — |
| 弹幕 | `i-mingcute-danmaku-line` | — |
| 点赞 | `i-mingcute-thumb-up-2-line` | `i-mingcute-thumb-up-2-fill` |
| 投币 | `i-mingcute-coin-2-line` | `i-mingcute-coin-2-fill` |
| 收藏 | `i-mingcute-star-2-line` | `i-mingcute-star-2-fill` |
| 分享 | `i-mingcute-share-forward-line` | — |

激活态切换写法：`:class="liked ? 'i-mingcute-thumb-up-2-fill' : 'i-mingcute-thumb-up-2-line'"`。

> ⚠️ **关键配置**：presetIcons 必须加 `extraProperties: { display: 'inline-block', 'vertical-align': 'middle' }`，
> 否则 `<span>` 图标为 inline 时忽略 width/height、宽高塌陷为 0 而不显示。

#### 方案对比

| 方案 | 原理 | 优点 | 缺点 | 适用场景 |
| --- | --- | --- | --- | --- |
| **A. Iconify + UnoCSS presetIcons**（✅ 主推） | 构建时将 SVG 内联为 CSS mask/background，模板写 `class="i-mdi-thumb-up text-primary"` | 零运行时、tree-shake、可继承 currentColor、与 UnoCSS 原子类无缝配合、10 万+ 图标 | 需安装 `@iconify-json/xxx` 图标集包（devDep） | Web / Admin |
| B. Iconify Vue 组件（运行时） | `@iconify/vue` 组件按需从 Iconify API/CDN 拉 SVG | 无需预装图标集、动态切换方便 | 依赖网络/CDN、首屏多一次请求 | 需要动态图标的场景（如后台图标选择器） |
| C. Element Plus Icons | `@element-plus/icons-vue`，组件式 `<el-icon><Search /></el-icon>` | 已内置、与 EP 组件风格统一 | 图标集有限（~280 个）、无法用 UnoCSS 原子类控制 | 仅 EP 组件内嵌图标 |
| D. SVG Sprite / 自维护 | 手动管理 SVG 文件 + `<use xlink:href="#icon-xxx">` | 完全可控、无外部依赖 | 维护成本高、需手动导出/优化 SVG | 品牌 Logo 等少量固定图标 |
| E. 字体图标（iconfont） | 阿里 iconfont / Font Awesome 字体文件 | 兼容性好、使用简单 | 单色限制、字体加载阻塞、无法 tree-shake | 不推荐新项目使用 |

#### 主推方案 A 实施细节

**Web / Admin 端**（已具备条件）：

1. `uno.config.ts` 已配 `presetIcons({ scale: 1.2, warn: true })`——无需改动。
2. 安装常用图标集（devDependency）：
   ```bash
   pnpm add -D @iconify-json/mdi @iconify-json/carbon @iconify-json/tabler
   ```
   - `mdi`（Material Design Icons）：通用 UI 图标最全
   - `carbon`（IBM Carbon）：线条风格，适合后台
   - `tabler`：B 站风格圆角线条图标
3. 模板使用：
   ```html
   <!-- 搜索图标，继承文字颜色 -->
   <span class="i-mdi-magnify text-4" />
   <!-- 点赞，品牌粉 -->
   <span class="i-mdi-thumb-up text-primary text-5" />
   <!-- 铃铛 -->
   <span class="i-mdi-bell-outline text-5" />
   ```
4. 颜色控制：直接用 UnoCSS 的 `text-xxx` 类（图标通过 CSS mask 继承 `currentColor`）。
5. 大小控制：`text-4`（16px）、`text-5`（20px）、`text-6`（24px）或任意值 `text-[18px]`。

**H5 端**（uni-app 限制）：

- UnoCSS presetIcons 在 H5 目标下可用（构建时内联 SVG 为 CSS）。
- 但 uni-app 模板不支持 `<span>` 自闭合，需用 `<view class="i-mdi-xxx" />`。
- 备选：使用 uni-icons 组件（`@dcloudio/uni-ui` 内置）或自封装 SVG 组件。

#### 迁移计划

| 阶段 | 范围 | 说明 |
| --- | --- | --- |
| 批次 1 ✅ | AppHeader（搜索/铃铛）+ VideoView 互动栏 | 已迁移为 `i-mingcute-*` |
| 批次 2 ✅ | HomeView / SearchView / FeedView / SpaceView / MineVideosView 列表图标 | ▶👍💬 全部图标化 |
| 批次 3 ✅ | NotifyView 通知类型图标 + CommentSection 点赞（line/fill） | 完成 |
| 批次 4 ✅ | H5 端（index/search/video） | uni 模板用 `<text class="i-mingcute-*" />` |

> 全端 emoji 已清零，统一为 MingCute。VideoView 布局同步对标 B 站：互动栏（赞/币/藏/分享）下移到播放器+弹幕栏下方独立一行。

#### 图标命名规范

- 格式：`i-{集合}-{图标名}`，如 `i-mdi-thumb-up`、`i-carbon-search`、`i-tabler-bell`。
- 同一功能全端统一图标名（避免 web 用 mdi、h5 用 carbon 导致视觉不一致）。
- 品牌 Logo / 吉祥物保持 SVG 文件方案（方案 D），不走 Iconify。

- TypeScript 严格模式；ESLint + Prettier + stylelint，CI 强制。
- 提交规范：Conventional Commits + commitlint + husky。
- 组件开发：packages/ui 配 Storybook（V1 起）。
- 环境变量：`.env.development / .env.staging / .env.production`。

## 5. 性能策略

- 路由懒加载 + 组件级代码分割；首页卡片图片懒加载 + CDN 裁剪参数。
- 视频封面 WebP/AVIF 自适应；骨架屏统一组件。
- H5 首屏目标 LCP < 2.5s：关键请求预连接、SSR 评估（V2）。
- 小程序分包加载，主包 < 1.5MB。
