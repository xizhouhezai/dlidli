# plan：搜索与推荐

> 对应规格：[spec](/specs/search-recommend/spec) ｜ 技术基线：[后端架构](/architecture/backend) · [总体架构](/architecture/overview)
> 实现位置：`server/internal/module/search`、`server/internal/module/recommend`、`cmd/itemsim`（离线计算）

## 1. 方案概览

搜索目标形态为 Elasticsearch（IK 分词 + 消息队列同步索引），MVP 以 MySQL LIKE 过渡（接口层可平滑切换）。推荐为"多路召回 + 规则粗排 + 打散过滤"的本地务实版：行为日志 MySQL 落库（ClickHouse 预留）、ItemCF 离线相似度计算（cmd/itemsim）、热度榜加权公式，策略演进路径 V2 热度+ItemCF → V2.5 双塔+GBDT/LR → V3 深度多目标。

## 2. 技术决策

| 决策点 | 决策 | 理由 | 备选方案 |
| --- | --- | --- | --- |
| 搜索引擎 | MVP MySQL LIKE（视频标题/UP 主昵称），ES 部署后接口层切换 | 本地无 Docker；检索接口已隔离 | 首版即 ES（部署依赖重） |
| 索引同步 | 规划：稿件发布/编辑/删除经 Kafka 同步 ES 索引（延迟 < 10s） | 与写路径解耦 | 双写（一致性难保） |
| 综合排序 | 规划：文本相关性 × 质量分（播放/互动）× 时效衰减 | 多因子平衡 | 单因子（体验差） |
| 行为日志 | `user_behavior` MySQL 落库（1曝光 2点击 3播放 4互动），POST /behaviors 批量上报 | ClickHouse 预留；结算/已看过滤/推荐共用 | 直接上 ClickHouse（运维成本） |
| 热度榜 | 加权分：播放×1+赞×3+币×5+藏×4+评×4+弹幕×2+转发×3；全站/分区榜，Redis 5min 缓存 | 可解释可调参 | 神经网络排序（无数据积累） |
| 相似度 | ItemCF 离线计算：行为 action=3 构造用户-物品矩阵 → 共现 → 余弦相似度 → 每稿 top 10（阈值 0.1）upsert 幂等 | 无外部依赖的经典协同过滤 | 双塔向量召回（V2.5 演进） |
| 召回合并 | 混合召回（兴趣分区热度 + 全站热度 + 新稿池 + 相似视频）+ 已看/负反馈过滤 + 打散（同 UP≤1、同分区连续≤3） | 规则透明可调试 | 端到端模型（黑盒难排查） |
| A/B 分流 | 按 uid 哈希稳定分流（FNV-1a % 100 < ratio → B 组），experiment 表配置 | 无需用户表字段；实验配置化 | 用户打标（迁移成本） |

## 3. 数据模型

```sql
user_behavior   { id, user_id, action(1曝光 2点击 3播放 4互动), oid, created_at }  -- 0016 迁移
video_similar   { video_id, similar_video_id, score }   -- 0019 迁移，唯一键 + score
experiment      { id, target, variant_b, ratio, status } -- 0020 迁移，A/B 实验
```

Redis：热度榜缓存（5min）、推荐结果短缓存。

## 4. 接口设计

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | /api/v1/search?kw=&tab= | 综合搜索（视频/用户 Tab） |
| GET | /api/v1/videos/hot | 全站/分区热度榜 |
| GET | /api/v1/recommend/videos | 个性化推荐（关闭个性化时退化为热度） |
| POST | /api/v1/behaviors | 行为批量上报 |
| POST | /api/v1/dislikes | 负反馈（不感兴趣） |
| GET/PUT | /api/v1/users/me/recommend-settings | 个性化推荐开关 |

> 完整契约以 swaggo 生成的 OpenAPI 为准（`server/docs/`）。

## 5. 关键流程

推荐服务：混合召回（兴趣分区热度 + 全站热度 + 新稿池（48h 内播放 <100 保底）+ 最近观看 2 视频的相似视频（DISTINCT 限 12））→ 已看过滤（行为日志播放记录）→ 负反馈过滤（内容/UP 主/分区）→ 多样性打散。A/B 变体：target=recommend，variant_b=hot_only 时退化为纯热度榜。

## 6. 风险与待定项

- [ ] Elasticsearch 部署 + IK 分词 + 索引同步（M2-SRH-01/02，当前 LIKE 替代）
- [ ] 搜索联想/历史/热搜（SRH-03~05 随 ES）
- [ ] 新用户兴趣分区引导（REC-04 冷启动选分区）
- [ ] 向量召回与精排模型（V2.5+ 演进）
- [ ] 每周必看/周榜（REC-03 细化）
