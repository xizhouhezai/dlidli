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
- [x] 提现与月度结算（CRT-32，依赖支付基础设施）—— **2026-10-15 由 M4-PAY-02 交付**
- [x] 充电分成（CRT-31，依赖 [monetization](/specs/monetization/spec)）—— **2026-10-15 由 M4-PAY-02 交付**
- [ ] 观看时长指标（待行为数据扩展）

## 7. 充电收入与提现（CRT-31/32，由 M4-PAY-02 交付）

> 实现位置：`server/internal/module/charge`（**不在本模块**）。验收依据：
> [monetization spec §9](/specs/monetization/spec) + [monetization plan §2H](/specs/monetization/plan)。
> 本模块只消费其读模型，不重复实现结算与提现。

**为什么放在 monetization 域而不是 creator 域**：充电的入账发生在**支付回调事务内**
（与支付域强耦合），提现涉及资金状态机与出款通道，二者与支付基础设施同属一层；
creator 域保持「只读聚合 + 激励结算」的定位，避免支付语义渗入内容侧模块。

与本模块的接口关系：

| 事项 | 实现 | 与本模块的关系 |
| --- | --- | --- |
| 充电分成入账 | `charge_log` → `creator_income`（70‰）→ `creator_balance` | 创作者收益的**新来源**，与创作激励并存 |
| 月度结算 | `creator_charge_settlement` + 复用 `creator_settlement`（`source='charge'`） | **复用本模块既有的结算表**（plan §3 的既定决策），按（月 × 创作者 × 稿件）落行 |
| 收益查询 | `GET /charge/balance`、`GET /charge/settlements` | 本模块 `GET /creator/overview` 的 `earnings` 已把 `creator_settlement` 全额计入，故**充电分成会自动出现在既有总览里** |
| 提现 | `withdraw_order` 状态机 + mock 出款 | 本模块不涉及；真实出款与个税代扣为换入点 |

**口径一致性（硬要求）**：充电分成 700‰/300‰ 与 [live plan](/specs/live/plan) LIV-04 礼物分成**必须相同**。
两处均以落库快照（`charge_log.share_rate` / `gift_log.share_rate`）为准，已实测一致（各 700‰）。

**未覆盖（如实声明）**：**直播礼物分成尚未接入月结**——`creator_income.source_type=2` 已预留该维度，
但 LIV-04 的礼物分成目前只落 `gift_share_log`，本卡未改动其归属，避免跨卡在口径未定论时先行变更。
**个税代扣与税务合规**同理未实现（无税务资质与规则输入，不得宣称达标）。
