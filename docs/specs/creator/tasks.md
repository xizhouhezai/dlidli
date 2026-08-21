# tasks：创作者中心

> 对应规格：[spec](/specs/creator/spec) ｜ 方案：[plan](/specs/creator/plan)
> 任务编号：`{阶段}-{模块}-{序号}`；完成即勾选并追加完成日期；括号补注实现要点与验证结论。
> 稿件管理基础页（M1-VID-08）随视频模块交付，见 [video tasks](/specs/video/tasks)。

## M3（W25-W48）

> 2026-08-04 落地（本地务实版）：实时聚合替代 T+1 数仓（video_stat + 行为日志）；激励按有效播放结算（1 分/播放）。

- [x] M3-CRT-01 数据仓库聚合任务（播放来源/留存曲线/粉丝画像 T+1）（降级：实时聚合——video_stat + user_behavior 行为日志按需聚合，不做 T+1 数仓；播放趋势/稿件统计/粉丝数实时查询） `2026-08-04`
  - 覆盖：CRT-10、CRT-11（降级形态）、CRT-12（未做，画像待数据规模）
- [x] M3-CRT-02 后端：数据看板/单稿分析接口（0017 迁移 creator_settlement 日结算表；GET /creator/overview（总览：稿件数/总播放/赞/币/藏/粉丝/近7日播放/累计收益）、/creator/videos（单稿数据：统计+有效播放+收益）、/creator/trend（近 N 天播放趋势补零对齐）、/creator/settlements（收益明细分页）） `2026-08-04`
  - 覆盖：CRT-10、CRT-11（单稿统计）、CRT-33
- [x] M3-CRT-03 Web：创作者中心（概览/稿件数据/评论弹幕管理）（/creator 页面：5 统计卡（总播放/总赞/总投币/粉丝/累计收益）+ 数据趋势 echarts 柱状图（指标切换：有效播放/互动/点击/曝光 + 近 7/30 日切换，tooltip/渐变/交互）+ 稿件数据 Tab（封面/状态/播放/赞币藏/有效播放/收益）+ 收益明细 Tab；头部下拉「创作者中心」入口；修复收益明细空数据 loading 卡住（空列表 null 兜底 + loaded 标记防重复加载）） `2026-08-04`
  - 覆盖：CRT-10、CRT-01（数据 Tab）、CRT-33
- [x] M3-CRT-04 创作激励：有效播放结算 + 收益明细（请求时全量结算：INSERT SELECT 按日期×稿件聚合有效播放（行为 action=3）→ 收益=播放×1 分/次 upsert 到 creator_settlement（幂等）；E2E 实测 4 天行为数据结算 17 播放=17 分、趋势/明细全通） `2026-08-04`
  - 覆盖：CRT-30、CRT-33
- [x] M3-CRT-05 多P投稿 + 合集功能（多P：0021~0024 迁移（video_part 表 + video_stream/transcode_job 加 part_index + 唯一键含分P）；Submit 扩展 parts（逐 P 建 video_part+原画流+转码任务）；转码 worker 按分P隔离（源流/输出目录/时长回写）；GET /videos/:bvid/parts（各 P 签名流）；投稿页分P管理（最多 10 P）+ 播放页分P列表切换；合集：0025 迁移 video_collection/video_collection_item（改名避收藏夹冲突）；collection 模块（创建/列表/详情/归集稿件，仅本人管理）；空间页合集 Tab（新建弹窗/卡片）+ 合集详情页；E2E 实测：双P投稿→转码完成各 3 档流、播放切P、合集 CRUD、空列表/ID 序列化修复全通） `2026-08-05`
  - 覆盖：VID-05、VID-23（跨模块，见 [video spec](/specs/video/spec)）

## 进度

| 里程碑 | 任务数 | 已完成 |
| --- | :-: | :-: |
| M3 | 5 | 5 |
| **合计** | **5** | **5** |

> 勾选任务后同步更新上表与 [开发进度管理](/project/progress) 的模块矩阵。
