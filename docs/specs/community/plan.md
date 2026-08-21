# plan：社区动态与关注

> 对应规格：[spec](/specs/community/spec) ｜ 技术基线：[后端架构](/architecture/backend) §2.4 Feed 流 · [数据模型](/architecture/data-model)
> 实现位置：`server/internal/module/relation`（关注/黑名单）、`server/internal/module/dynamic`（动态/Feed）

## 1. 方案概览

关注关系读多写少（MySQL `relation` 表 + 计数缓存）；动态以 `dynamic` 表承载多种类型（投稿/图文/转发），Feed 流目标形态为推拉结合，MVP 落地为拉模式（关注列表 IN 查询 + 雪花 ID 游标分页），规模化后演进收件箱。

## 2. 技术决策

| 决策点 | 决策 | 理由 | 备选方案 |
| --- | --- | --- | --- |
| Feed 流 | MVP 拉模式：关注列表 IN 查询 + 雪花 ID 游标分页 | 实现简单；数据量小阶段够用 | 推拉结合收件箱（Redis zset 截断 1000 条，粉丝 > 1 万大 V 拉模式，待规模化启用） |
| 动态类型 | 单表多类型（type：1 投稿 / 2 图文 / 3 转发引用） | 转发复用动态载体，链式展示取源动态 | 分表（查询 union 复杂） |
| 投稿动态 | 视频过审发布钩子自动生成 | 免用户手动同步 | 定时扫描（延迟） |
| 关注约束 | 禁关自己、幂等（唯一键）、状态聚合接口 | 基础正确性 | — |
| 黑名单 | relation type=3 + 0027 迁移 user_block；发言/弹幕/私信链路过滤 | 复用关系模型；治理一处生效 | 独立黑名单表 |

## 3. 数据模型

全局见 [数据模型](/architecture/data-model)（`relation`：type 1关注/2特别关注/3拉黑）。模块扩展：

```sql
dynamic        { id, user_id, type(1投稿 2图文 3转发), content, imgs, ref_id, created_at }
user_block     { user_id, blocked_uid }                          -- 0027 迁移
```

Redis：`feed:inbox:{uid}`（zset 1000 截断，规模化启用）、关注/粉丝集合缓存。

## 4. 接口设计

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST/DELETE | /api/v1/space/{uid}/follow ｜ /follows ｜ /fans | 关注/取关（幂等）、关注/粉丝列表 |
| GET/POST/DELETE | /api/v1/dynamics... | 关注流（游标分页）/ 发布（机审预检）/ 删除 |
| GET | /api/v1/space/{uid} | 空间域聚合（资料+稿件+关注状态） |

> 完整契约以 swaggo 生成的 OpenAPI 为准（`server/docs/`）。

## 5. 关键流程

发布动态：图文（敏感词预检）或投稿钩子 → `dynamic` 落库 →（规模化后：Kafka → 扇出 Worker 推粉丝收件箱）→ 关注流读取（MVP 直接 IN 查询）。

## 6. 风险与待定项

- [ ] 推拉结合收件箱（规模化后，Redis zset + 大 V 拉模式合并）
- [ ] 特别关注提醒（FLW-03）、话题（DYN-04）、投票（DYN-06）
- [ ] 关注分组（FLW-04，P3）
