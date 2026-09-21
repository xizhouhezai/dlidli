# ADR：鸿蒙端技术选型（M4-APP-01）

- **状态**：已决策（ArkTS/ArkUI 原生），实现方案见 [harmony plan](/specs/harmony/plan)
- **日期**：2026-09-21
- **范围**：C 端鸿蒙应用（HarmonyOS NEXT）的技术路线与工程形态
- **关联**：[harmony spec](/specs/harmony/spec) ｜ [前端架构](/architecture/frontend) ｜ [路线图](/project/roadmap)

## 背景

M4 里程碑需接入 App 端。原任务项 [M4-APP-01](/specs/engineering/tasks) 仅写了「uni-app 打包 vs Flutter」，未覆盖鸿蒙。本次先把鸿蒙端作为首个落地终端决策。

现状资产盘点（决定选型的关键事实）：

| 现有资产 | 能否被鸿蒙端复用 | 原因 |
| --- | --- | --- |
| `packages/player`（hls.js 内核） | ❌ | 依赖 MSE/WebView 播放环境，鸿蒙不提供；须换系统 AVPlayer |
| `packages/api-client`（fetch / uni.request 适配器） | ❌ | TS 包，ArkTS 不能直接消费，且需换成 `@ohos.net.http` |
| `packages/shared`（纯 TS 工具） | ⚠️ 需移植 | 逻辑可移植为 ArkTS，但 ArkTS 禁用 `any`/动态属性，不能直接拷贝 |
| `packages/ui`（Element Plus 封装） | ❌ | Web 组件库，鸿蒙无对应物 |
| 全部 `.vue` 页面（web/h5） | ❌ | ArkUI 是另一套声明式 UI 与组件模型 |
| `docs/specs/*` 的 EARS 需求与业务规则 | ✅ | 端无关，鸿蒙端直接继承 |
| 后端 REST/WS 契约（`server/docs` OpenAPI） | ✅ | 端无关，本期后端零改动 |
| 品牌色板与圆角（`_variables.scss`） | ✅ | 导出为 ArkTS 常量即可保持全端一致 |
| 图标 | ❌ 换体系 | 沿用 Web 的 MingCute 会与系统视觉割裂，端侧改用 **HarmonyOS Symbol**（`SymbolGlyph` 消费） |
| Web 端弹幕轨道分配等实现思路（`DanmakuLayer.vue`） | ⚠️ 思路复用 | Canvas 绘制逻辑同构，需按 ArkTS 重写 |

同时评估过 uni-app `app-harmony` 路线（`@dcloudio/uni-app-harmony@3.0.0-5010520260709002` 存在且与 `apps/h5` 现有 uni-app 版本号一致，本机 DevEco Studio 6.1.1.300 / HarmonyOS SDK API 24 满足官方要求，官方限定 Vue3 项目——条件均成立），因此这是一个**真实的二选一**，而非"只能原生"。

## 决策

采用 **ArkTS + ArkUI 声明式（Stage 模型）原生开发**，工程置于本仓 `apps/harmony`，并**从 pnpm workspace 排除**（hvigor/ohpm 与 pnpm 是两套构建体系）。

**决策理由**：

1. **原生体验与性能优先**：滑动、动画、长列表与播放交互在原生 ArkUI 上明显优于跨端壳，是本端选择的首要动机；
2. **跨端复用率实际不划算**：uni-app 路线的复用主要集中在页面结构，而技术难点（播放器内核、弹幕渲染、UI）恰好都在不可复用的部分——`packages/player` 的 hls.js 在鸿蒙无效，弹幕层与全部视图都需重写，实际节省的工作量有限；
3. **系统能力接入更完整**：媒体（AVPlayer 硬解）、系统分享、深色模式、安全区适配等原生能力接入路径最短。

## 备选方案与否定理由

| 方案 | 结论 | 否定理由 |
| --- | --- | --- |
| uni-app `app-harmony` 复用 `apps/h5` | 否 | 复用率集中在页面结构，播放器/弹幕/UI 三处核心仍需重写；跨端壳在体验与系统能力上受限，与"原生体验优先"目标冲突 |
| Flutter 打包鸿蒙 | 否 | 鸿蒙侧为社区/非官方支持路径，长期维护与生态一致性风险高；且同样无法复用现有 TS 资产 |
| 方舟开发框架类 Web 范式（JS/ArkUI Web 范式） | 否 | 非鸿蒙官方长期推荐路线，能力受限 |
| ArkTS/ArkUI 原生 | ✅ 采用 | — |

## 影响

- 前端代码在鸿蒙端**零复用**，但需求（EARS）与接口契约**零重写**：后端本期不做任何改动；
- 需新增 `docs/specs/harmony/` 三件套（已建立）作为端侧需求与任务基线；
- 需在 `pnpm-workspace.yaml` 排除 `apps/harmony`，并确认不影响现有 web/admin/h5/docs 的安装与 CI；
- 维护成本：全端视觉与业务规则需要额外同步（品牌 token、图标映射需在 ArkTS 侧复制一份常量并保持同步）。

## 未决事项

- 目标 API 版本（`compatibleSdkVersion`/`targetSdkVersion`）取值，需结合设备覆盖与可用系统能力确定。
- 是否将鸿蒙构建纳入 CI（依赖 DevEco 环境）。
- 后续若接入 iOS/Android，是否复用本次选型结论（Flutter 等）仍在 [M4-APP-01](/specs/engineering/tasks) 范围内待定；本 ADR 只约束鸿蒙端。
- 上架资质与开发者主体：本期口径为仅本地/真机验证、不提审；提审前置条件（视频类目资质、软著、隐私双清单）待上架阶段评估。
