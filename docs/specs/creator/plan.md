# plan：创作者中心

> 对应规格：[spec](/specs/creator/spec) ｜ 技术基线：[后端架构](/architecture/backend) · [数据模型](/architecture/data-model)
> 实现位置：`server/internal/module/creator`、Web `/creator` 页面

## 1. 方案概览

M3 落地为"本地务实版"：以 `video_stat` + `user_behavior` 行为日志实时聚合替代 T+1 数仓；激励按有效播放结算（1 分/播放）请求时全量幂等结算到日结算表；多 P 与合集扩展视频域表结构（见 [video plan](/specs/video/plan)）。

## 2. 技术决策

| 决策点 | 决策 | 理由 | 备选方案 |
| --- | --- | --- | --- |
| 数据看板 | 实时聚合（video_stat + user_behavior 按需查询，趋势补零对齐），不做 T+1 数仓 | 数据量小阶段免运维负担；ClickHouse 预留 | T+1 数仓（延迟高、运维重） |
| 收益结算 | 请求时全量结算：INSERT SELECT 按日期×稿件聚合有效播放（action=3）→ 收益 = 播放 × 1 分 upsert 到 creator_settlement（幂等） | 免定时任务；幂等可重放 | 定时批处理（需调度） |
| 看板可视化 | echarts 柱状图（近 7/30 日切换 + 9 项指标切换，tooltip/渐变/交互） | 交互成熟 | 手写 SVG（成本高） |
| 多 P/合集 | 0021~0024 迁移（video_part + 流/任务加 part_index）；0025 合集（video_collection 与收藏夹表名区分） | 结构见 [video plan](/specs/video/plan) | 稿件拆分（体验差） |

## 3. 数据模型

```sql
creator_settlement { id, user_id, video_id, date, valid_play, income, ... }  -- 0017 迁移，日结算幂等 upsert
video_part / video_collection(_item)                                        -- 见 [video plan](/specs/video/plan)
```

## 4. 接口设计

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | /api/v1/creator/overview | 总览：稿件数/总播放/赞/币/藏/粉丝/近 7 日播放/累计收益 |
| GET | /api/v1/creator/videos | 单稿数据：统计 + 有效播放 + 收益 |
| GET | /api/v1/creator/trend | 近 N 天播放趋势（补零对齐） |
| GET | /api/v1/creator/settlements | 收益明细分页 |

> 完整契约以 swaggo 生成的 OpenAPI 为准（`server/docs/`）。

## 5. 关键流程

结算：进入创作者中心 → 全量结算（INSERT SELECT 聚合当日有效播放 → upsert creator_settlement）→ 看板/明细读取。E2E：4 天行为数据结算 17 播放 = 17 分。

多 P 投稿：Submit 扩展 parts → 逐 P 建 video_part + 原画流 + 转码任务 → Worker 按分 P 隔离转码（源流/输出目录/时长回写）→ 播放页分 P 列表切换。

## 6. 风险与待定项

- [ ] 播放来源/留存曲线/互动漏斗（CRT-11 细化，需埋点扩展）
- [ ] 粉丝画像（CRT-12）、数据对比（CRT-13）
- [ ] 提现与月度结算（CRT-32，依赖支付基础设施）
- [ ] 充电分成（CRT-31，依赖 [monetization](/specs/monetization/spec)）
- [ ] 观看时长指标（待行为数据扩展）
