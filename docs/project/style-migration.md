# 样式迁移计划（UnoCSS + SCSS）

> 状态：✅ 全部完成（已归档，仅作历史参考） ｜ 更新日期：2026-07-30 ｜ 归档日期：2026-08-28

## 目标

三端（web / h5 / admin）统一样式方案：**UnoCSS 原子类**（布局/间距/字号/颜色等高频样式）+ **SCSS**（交互态、`:deep()`、动画、复杂选择器）+ **品牌 token 单一真源**。旧页面 `var(--dli-*)` 写法零改动兼容，按本计划逐页增量迁移。

## 迁移原则

1. **模板类名**：高频布局/盒模型/间距/字号/颜色改为 Uno 原子类（`flex items-center gap-3`、`max-w-160`、`text-3.5`、`text-text-2`、`bg-primary` 等）。
2. **保留 SCSS 的场景**：`hover/active/is-xxx` 状态、`:deep()` 定制 Element Plus、`@keyframes` 动画、`mask`/渐变等复杂视觉、多行省略（用 `@include v.ellipsis(n)`）。
3. **颜色**：一律走 token——Uno 用 `text-primary/bg-bg/text-text-2`，SCSS 用 `@use '@/styles/variables' as v; v.$primary`。禁止新增硬编码色值。
4. **验证**：每页改完 `pnpm <app>:lint` + `build`；关键页浏览器实测视觉不回归。

## 页面清单与优先级

### admin（2 页，已完成）

| 页面 | 状态 | 说明 |
| --- | --- | --- |
| ReviewView | ✅ | 模板全量 Uno 化，仅多行省略留 SCSS |
| LoginView | ✅ | 重动画/玻璃拟态保留 SCSS，主色引用化 |

### web（11 页）

| 优先级 | 页面 | 状态 | 备注 |
| --- | --- | --- | --- |
| P0 | NotifyView | ✅ | 基建示范页 |
| P1 | SearchView | ✅ | 容器/Tab/网格 Uno 化，hover 联动与 auto-fill grid 留 SCSS |
| P1 | HomeView | ✅ | 分区/排序/卡片网格 Uno 化，chip/tab/hover 联动留 SCSS |
| P1 | FeedView | ✅ | 发布器/动态卡片 Uno 化，按钮变量覆盖留 SCSS |
| P2 | SpaceView | ✅ | 头部 banner/关注按钮多态留 SCSS，tab/网格/卡片 Uno 化 |
| P2 | MineVideosView | ✅ | 列表/状态标签 Uno 化，投稿按钮变量留 SCSS |
| P2 | SettingsView | ✅ | 头像行/表单容器 Uno 化，保存按钮变量留 SCSS |
| P2 | ResetPasswordView | ✅ | 卡片/表单容器 Uno 化，主按钮变量留 SCSS（位于 auth/ 目录） |
| P3 | LoginView | ✅ | 重动画背景保留 SCSS，硬编码色/按钮变量化 |
| P3 | VideoView | ✅ | 保守策略：style 转 SCSS + 变量化（模板不动，体量最大页面控风险） |
| P3 | App.vue（头部） | ✅ | 布局容器 Uno 化，铃铛 badge/:deep 搜索框/导航激活态留 SCSS |

### h5（uni-app）

| 页面 | 状态 | 备注 |
| --- | --- | --- |
| index（首页） | ✅ | 变量同源（保留 rpx 适配，不强推原子类） |
| video（播放页） | ✅ | 同上策略，色值全部变量化 |

## 执行批次

- **批次 1**：admin 2 页 + web SearchView（P1 示范）——已完成
- **批次 2**：web P1 剩余（Home/Feed）+ P2（Space/Mine/Settings/Reset）——已完成
- **批次 3**：web P3（Login/Video/App 头部）+ h5 video——已完成

## 收官结论

三端 15 个页面/组件全部迁移完成（admin 2 + web 11 + h5 2）。全仓不再有硬编码品牌色（#fb7299/#fc8bab 仅存在于三处 token 真源与 SVG 素材）；后续新页面直接按「迁移原则」编写，无需再迁移。

## 注意事项（踩坑记录）

- uni-app（h5）：`unocss/vite` ESM-only，vite.config 用异步 `defineConfig(async () => await import(...))`；SCSS `@use` 用相对路径。
- web/admin：`@use '@/styles/variables' as v` 可用 `@` 别名（vite resolve.alias 已配）。
- Element Plus 组件内部样式改不到时用 `:deep()`，保留在 SCSS。
