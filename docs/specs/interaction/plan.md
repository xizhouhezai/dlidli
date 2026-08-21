# plan：互动体系

> 对应规格：[spec](/specs/interaction/spec) ｜ 技术基线：[后端架构](/architecture/backend) §2.2 计数系统 · [数据模型](/architecture/data-model)
> 实现位置：`server/internal/module/interaction`（含 triple.go 三连聚合）

## 1. 方案概览

点赞/投币/收藏统一收敛到 `user_action` 行为明细模型（唯一键天然幂等），评论为两级结构（root_id/parent_id），计数与主表分离（video_stat）。MVP 阶段直写 MySQL 保证正确性，性能优化路径为 Redis 计数 + 异步落库；硬币相关操作走 MySQL 强一致事务。三连由聚合接口一次完成"赞+币+藏"。

## 2. 技术决策

| 决策点 | 决策 | 理由 | 备选方案 |
| --- | --- | --- | --- |
| 行为模型 | `user_action` 统一表（oid + obj_type + action + extra），唯一键 `(user_id, oid, obj_type, action)` | 点赞/投币/收藏共用幂等与查询路径；按 user_id 分表预留 | 三张独立表（模型重复） |
| 计数 | MVP 直写 DB（video_stat）；规划 Redis INCR + Kafka 聚合批量刷库（最终一致） | 先保证正确性；读多写少场景缓存收益后置 | 首版即异步（调试难） |
| 硬币 | MySQL 本地事务（扣减+流水+计数）+ 唯一键幂等 + 失败退款补偿 | 资产强一致不可最终一致 | Redis 预扣（一致性风险） |
| 投币规则 | 自制 ≤ 2 / 转载 ≤ 1，禁投自己，服务端校验 | 规则集中在 service | 前端约束（可绕过） |
| 三连 | 聚合接口：已完成项跳过、投币失败不阻塞、delta 回传 | 弱依赖组合，单点失败不拖垮整体 | 串行强一致事务（体验差） |
| 收藏夹 | 默认夹懒创建不可删；删除夹清理夹内关联 | 避免空默认夹；数据一致 | 预创建（冗余） |
| 评论排序 | 热度 = 赞数 + 回复数 + 时间衰减，SQL 计算可切换最新 | 无需引入排序服务 | 离线热度分（过度设计） |
| 评论治理 | status 字段影子屏蔽（机审命中）+ 作者/UP 主/管理员三级删除权限 | 先发后审平衡 | 全量先审（体验差） |
| 转发 | 生成 type=3 引用动态 + share_cnt 回写 + 敏感词校验 | 复用动态体系承载转发 | 独立转发表（重复） |

## 3. 数据模型

全局见 [数据模型](/architecture/data-model)（`user_action` / `comment` / `video_stat`）。模块扩展：

```sql
favorite_folder         { id, user_id, name, is_default, visibility }   -- 默认夹懒创建
favorite_folder_item    { folder_id, video_id, created_at }
```

Redis：`like:v:{video_id}`（点赞去重 set，规划）、互动限流键。

## 4. 接口设计

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST/DELETE | /api/v1/videos/{bvid}/like | 点赞/取消 |
| POST | /api/v1/videos/{bvid}/coin | 投币（1-2 枚，幂等） |
| GET | /api/v1/interaction?bvid= | 互动状态聚合（赞/币/藏回显） |
| POST | /api/v1/interaction/triple | 一键三连（delta 回传） |
| GET/POST/PUT/DELETE | /api/v1/folders... | 收藏夹 CRUD + 收藏到指定夹 |
| GET/POST/DELETE | /api/v1/videos/{bvid}/comments... | 评论列表（热度/最新）/发布/删除；评论点赞 |

> 完整契约以 swaggo 生成的 OpenAPI 为准（`server/docs/`）。

## 5. 关键流程

三连聚合（对应 ITR-30）：长按 0.8s 触发（前端交互阈值）→ 聚合接口 → 逐项执行（已赞则跳过；投币默认 2 枚余额不足投 1 枚、再不足跳过且不阻塞；收藏入默认夹）→ 返回各项 delta 前端刷新。

## 6. 风险与待定项

- [ ] Redis 计数 + 异步落库性能优化（MVP 直写 DB 待压测评估）
- [ ] 弹幕点赞（复用 user_action obj_type=3，延后）
- [ ] 点踩（ITR-03，P2 负反馈信号）
- [ ] 稍后再看（ITR-23）、批量收藏管理（ITR-22）
