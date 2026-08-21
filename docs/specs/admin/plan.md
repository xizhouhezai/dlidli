# plan：内容审核与管理后台

> 对应规格：[spec](/specs/admin/spec) ｜ 技术基线：[后端架构](/architecture/backend) · [前端架构](/architecture/frontend)（apps/admin 独立应用）
> 实现位置：`server/internal/module/admin`（RBAC/sys/dashboard）、`report`、`banner`、`abtest` 模块、`apps/admin`

## 1. 方案概览

后台为独立前端应用（apps/admin，dev :5175，与 C 端完全分离部署），后端 admin 域当前内嵌 api 服务 `/api/v1/admin`（规模化后拆出独立服务）。RBAC 五表模型 + 权限点 code 鉴权（中间件 RequirePerm + 前端 v-perm 指令）；全量写操作经 audit_log 留痕；审核链路 = 机审预筛（contentmod）+ 人审工作台 + 举报处理。

## 2. 技术决策

| 决策点 | 决策 | 理由 | 备选方案 |
| --- | --- | --- | --- |
| 部署形态 | 后台前端独立应用；后端模块内嵌 `/api/v1/admin` | 独立部署安全隔离；拆服务留到规模化 | 同域路径区分（隔离弱） |
| 管理员认证 | 独立令牌与 C 端完全隔离（jwtx Admin claim）；默认账号 admin/admin123 首启自建 | 防跨端令牌混用 | 复用 C 端会话（风险高） |
| RBAC 模型 | 五表：admin_user/admin_role/admin_permission/admin_user_role/admin_role_permission；权限 code `模块:操作` | 标准模型，菜单树 + 按钮两级 | ACL（管理成本高） |
| 鉴权执行 | 后端 middleware.RequirePerm(code) + super 短路放行；前端 permissionStore 加载 /me/permissions + v-perm 指令 + 动态菜单过滤 | 前后端双重校验；菜单按钮统一数据源 | 仅后端（菜单体验差） |
| 内置数据 | seedRBAC 幂等 upsert 11+ 权限点 + 6 内置角色，默认 admin 绑 super | 环境重建可幂等 | 手工 SQL（易漂移） |
| 机审 | contentmod 包：敏感词热加载词库（moderate 线程安全动态词库，增删即时热刷）+ 广告/联系方式正则，统一接入评论/弹幕/动态/转发/投稿五处调用点；预留外部内容安全 API | 本地零依赖；命中影子屏蔽或拒发 | 直连云内容安全（内测期成本） |
| 风险分级 | video.risk_level：标题/简介机审命中→高风险、新账号（注册<7 天）→中风险、默认低；审核队列按风险降序 | 人审资源投向高风险（ADT-06 策略落地） | 全量同级（浪费） |
| 处罚执行 | account govern 层统一处罚 + 到期懒解除（muted_until/banned_until）；发言链路（评论/弹幕/动态/转发/投稿）禁言拦截 + 登录封号拦截 | 处罚判定集中一处 | 各模块各自判断（易漏） |
| 审计 | audit_log 表全量写操作留痕 + SYS-01 查询/CSV 导出（UTF-8 BOM，上限 1 万条） | 合规硬要求 | 日志文件（难查询） |
| A/B 分流 | FNV-1a(target:uid) % 100 < ratio → B 组，experiment 表配置 | 无需用户表字段，稳定可复现 | 随机分流（不稳定） |

## 3. 数据模型

```sql
admin_user / admin_role / admin_permission / admin_user_role / admin_role_permission  -- 0011 迁移
sensitive_word { id, word, level }      -- 0009 迁移，热加载词库
report         { id, reporter, obj_type, oid, reason, status, handler }  -- 0013 迁移
audit_log      { id, operator, action, obj_type, obj_id, before, after, ip }  -- 0013 迁移
system_config  { key, value }           -- 0015 迁移，热更新
data_dict      { type, value, label }   -- 0015 迁移
banner         { id, image, bvid, sort, status }  -- 0018 迁移
experiment     { target, variant_b, ratio, status }  -- 0020 迁移
```

> RBAC 字段语义见 [spec §5](/specs/admin/spec)，建表 DDL 以 `server/scripts/migrations/` 为准。

## 4. 接口设计

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | /api/v1/admin/login | 管理员登录（独立令牌） |
| GET | /api/v1/admin/me/permissions | 当前用户权限集合（菜单过滤） |
| GET/PUT/DELETE | /api/v1/admin/videos... | 审核队列/通过/驳回/稿件管理（VMG，状态流转 + 审计） |
| GET | /api/v1/admin/reports ｜ PUT /reports/{id} | 举报队列与处理 |
| GET/PUT | /api/v1/admin/users... | 用户查询/处罚 |
| CRUD | /api/v1/admin/{roles,permissions,admins,categories,sensitive-words,banners,experiments,configs,dicts} | 各管理域 |
| GET | /api/v1/admin/dashboard/stats | 数据大盘（4 实时指标 + 7 日趋势 + 审核时效 + 待审数） |
| GET | /api/v1/admin/audit-logs?export=csv | 审计日志查询/导出 |

> 完整契约以 swaggo 生成的 OpenAPI 为准（`server/docs/`）。

## 5. 关键流程

- 审核工作台：队列（风险降序 + 认领）→ 预览（签名地址播放/抽帧）→ 通过/驳回（必选原因模板）/锁定 → 状态机流转 + 审计 + 通知 UP 主。
- 举报处理：C 端全场景入口（播放页/评论区一级+楼中楼/空间页/动态页）→ 队列（状态筛选）→ 忽略/删除/删除并处罚（禁言或封禁）→ 站内通知反馈举报人。
- 稿件管理：全状态列表筛选 → 下架（已发布→锁定）/恢复（锁定→已发布）/软删除，操作均审计留痕（video_offline/video_restore/video_delete）。

## 6. 风险与待定项

- [ ] admin 服务独立部署（规模化拆出）
- [ ] 复审流程（ADT-04，双人复审 + 申诉队列）
- [ ] 2FA 与 IP 白名单（SYS-04，V1）
- [ ] 热搜管理（OPS-03）、公告/活动（OPS-04）
- [ ] 信用分（UGV-04）
