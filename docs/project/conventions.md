# 协作规范

> 状态：`V1.0` ｜ 更新日期：2026-07-28

## 1. 分支与提交

### 分支模型（Trunk-Based 简化版）

```
main        ── 随时可发布，受保护，仅 PR 合入
feature/*   ── 功能分支：feature/m1-acc-01-phone-login
fix/*       ── 缺陷修复
release/*   ── 发布分支（公测后启用）
```

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
