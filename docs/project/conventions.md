# 协作规范

> 状态：`V1.1` ｜ 更新日期：2026-07-31

## 1. 分支与提交

### 分支模型（Git Flow）

```
main        ── 生产发布分支，受保护；只接受 release/hotfix 合入，每次发布打 tag
develop     ── 集成分支，日常开发成果的汇聚处（默认开发基线）
feature/*   ── 功能分支：每个任务一个，如 feature/m1-adm-03-user-ban
optimize/*  ── 轻量优化分支：文案修改/样式微调/小重构等“无新功能”需求
fix/*       ── 缺陷修复分支：开发/测试阶段的 bug 修复，从 develop 切出，如 fix/category-status
release/*   ── 发布分支：release/v0.2.0，从 develop 切出做发布准备
hotfix/*    ── 线上紧急修复，从 main 切出
```

### 版本号规则（SemVer，任务完成必发 release）

> 铁律：**无论大任务还是小任务，完成后都要发布一个 release 并打 tag**，版本号按改动大小调整（`vMAJOR.MINOR.PATCH`）：

| 改动类型 | 版本位 | 示例 | 典型场景 |
| --- | --- | --- | --- |
| 破坏性变更（不兼容） | MAJOR | v1.0.0 → v2.0.0 | API v2、大重构、不兼容迁移 |
| 新功能（feature） | MINOR | v0.2.0 → v0.3.0 | 一个新业务模块/页面/接口 |
| 优化/修复（optimize/fix） | PATCH | v0.3.0 → v0.3.1 | 样式微调、小重构、bug 修复 |

- feature 任务 → MINOR（含破坏性升 MAJOR）；optimize/fix 任务 → PATCH。
- 当前处于 0.x 阶段（未正式发布）：新功能走 MINOR，优化/修复走 PATCH；首个正式版再定 v1.0.0。

### 铁律：任何改动禁止直接在 main/develop 上进行

- **不允许在 main 或 develop 上直接改代码/提交**；任何改动（含一行修复）都必须先从 develop 切出对应类型分支（feature/optimize/fix），在分支上完成后再合入。
- **bug 修复也必须在 fix/* 分支上进行**，完成后合 develop+main 并发一个 PATCH release（与 optimize 同流程）。
- 若发现已误在 main/develop 上产生未提交改动：先 stash → 切到正确的分支 → pop → 再提交，不得将非 release/hotfix 提交直推 main。

### 缺陷修复流程（fix/*）

> 适用场景：开发/测试阶段发现的 bug（非线上紧急）。流程与 optimize 一致，仅分支名为 fix/<短描述>，版本位 PATCH。

```bash
git checkout develop && git pull
git checkout -b fix/<短描述>              # 如 fix/category-status
# 开发提交（fix: 类型）
git checkout develop && git merge --no-ff fix/<...> && git push
git checkout main    && git merge --no-ff fix/<...> && git push
git branch -d fix/<...>
# 然后按“发布步骤”切 release 打 PATCH tag
```

- 线上已发布版本的紧急修复走 hotfix/*（从 main 切出，修完回合 main+develop）；开发期 bug 走 fix/*。

### 轻量优化流程（optimize/*，适用于纯优化/小改动）

> 适用场景：纯 UI/样式微调、文案/文档修改、小重构、依赖升级等**不引入新功能**且风险低的改动。分支命名比 feature 轻（无需任务编号），但**完成后同样合 develop+main 并发一个 PATCH release**。

```bash
git checkout develop && git pull
git checkout -b optimize/<短描述>          # 如 optimize/admin-menu-icon
# 开发提交（多用 fix/refactor/style/chore/docs 类型）
git checkout develop && git merge --no-ff optimize/<...> && git push
git checkout main    && git merge --no-ff optimize/<...> && git push
git branch -d optimize/<...>
# 然后按下方“发布步骤”切 release 打 PATCH tag
```

- 与 feature 的区别：仅分支命名（feature 带任务编号，optimize 带短描述）与版本位（feature=MINOR，optimize=PATCH）；合入与发布流程一致。
- 区分不确定时的经验法则：改动引入/改变用户可感知的新行为 → feature；仅打磨现有行为/观感 → optimize。

### 任务开发完整流程（每个任务遵循）

1. **开分支**：从 `develop` 切出 feature 分支
   ```bash
   git checkout develop && git pull
   git checkout -b feature/<任务编号-短描述>   # 如 feature/m1-vid-05-play-sign
   ```
2. **开发**：在 feature 分支提交（Conventional Commits，关联任务编号）
3. **任务完成 → 合入 develop 与 main**（先验证 lint/build/测试全绿）
   ```bash
   git checkout develop && git merge --no-ff feature/<...> && git push
   git checkout main    && git merge --no-ff feature/<...> && git push
   git branch -d feature/<...>                              # 删除已合分支
   ```
4. **发布 → 切 release 分支并打 tag**
   ```bash
   git checkout -b release/v<X.Y.Z> develop
   # 只做版本号/CHANGELOG/阶段性修复，不加新功能
   git checkout main && git merge --no-ff release/v<X.Y.Z>
   git tag -a v<X.Y.Z> -m "release v<X.Y.Z>" && git push --follow-tags
   git push origin release/v<X.Y.Z>
   ```

> 约定：feature 始终从 `develop` 切出；任务完成同时合回 `develop` 和 `main`；**无论大小任务，完成后都切 `release/vX.Y.Z` 合 `main` 并打 tag**（版本位按上方 SemVer 规则：feature=MINOR、optimize/fix=PATCH、破坏性=MAJOR）。单人开发可直推，团队协作走 PR（CI 全绿 + ≥ 1 人 Review）。

### 提交规范（Conventional Commits）

```
<type>(<scope>): <subject>

type:  feat | fix | docs | refactor | test | chore | perf
scope: acc | vid | dm | itr | flw | msg | srh | rec | crt | adm | eng | web | h5 | mp
例：   feat(acc): 手机号验证码登录接口
       docs(prd): 更新弹幕屏蔽规则
```

- PR 必须关联开发清单任务编号（如 `M1-ACC-01`），CI 全绿 + 至少 1 人 Review 后合入。

## 2. 代码规范

| 范围 | 规范 |
| --- | --- |
| Go | `gofmt` + `golangci-lint`；错误必须处理或显式忽略；公共函数必须有注释 |
| TS/Vue | ESLint + Prettier；组件 PascalCase；组合式 API（`<script setup>`） |
| SQL | 禁止 `SELECT *`；DDL 变更走 migration 文件评审 |
| API | REST 命名复数资源；破坏性变更须升 `/api/v2` |

## 3. 文档维护约定

| 文档 | 更新时机 | 责任人 |
| --- | --- | --- |
| PRD（product/prd/*） | 需求变更时同步修改，标注版本 | 产品 |
| 架构文档 | 技术方案变更（先文档后编码） | 后端/前端负责人 |
| [开发清单](/project/checklist) | 任务完成即勾选 | 任务执行者 |
| [进度管理](/project/progress) | 每周五 | 项目负责人 |
| API 文档（OpenAPI） | 接口变更随 PR 提交 | 后端 |

## 4. 环境与发布

| 环境 | 用途 | 部署方式 |
| --- | --- | --- |
| dev | 本地开发（docker-compose） | 手动 |
| test | 联调测试 | push 自动部署 |
| staging | 预发（生产同构） | 手动触发 |
| prod | 生产 | 审批 + 灰度发布 |

- 发布窗口：周二/周四；周五不发布。
- 数据库变更：先兼容后清理（两阶段），禁止直接破坏性 DDL。

## 5. 缺陷管理

- 等级：P0（线上核心不可用，2h 响应）/ P1（主要功能异常，24h）/ P2（一般，排期）/ P3（体验优化）。
- P0/P1 修复后必须补充回归用例与故障复盘（5 Why）。
