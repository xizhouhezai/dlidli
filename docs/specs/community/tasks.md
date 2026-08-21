# tasks：社区动态与关注

> 对应规格：[spec](/specs/community/spec) ｜ 方案：[plan](/specs/community/plan)
> 任务编号：`{阶段}-{模块}-{序号}`；完成即勾选并追加完成日期；括号补注实现要点与验证结论。

## M2（W13-W24）

- [x] M2-FLW-01 后端：关注/取关/粉丝列表/关注列表（/space/:id 域；禁关自己、幂等、状态聚合；黑名单待补） `2026-07-29`
  - 覆盖：FLW-01、FLW-02
- [x] M2-DYN-01 后端：动态发布（投稿发布钩子自动生成 + 图文动态）+ 敏感词预检（完整机审 API 待 M2-AUD） `2026-07-29`
  - 覆盖：DYN-01、DYN-02
- [x] M2-DYN-02 后端：Feed 流（MVP 拉模式：关注列表 IN 查询 + 雪花 ID 游标分页；推拉结合收件箱待规模化） `2026-07-29`
  - 覆盖：DYN-10
- [x] M2-DYN-03 Web：动态页（关注流 + 发布器，投稿动态带视频卡片，头部"动态"导航入口） `2026-07-29`
  - 覆盖：DYN-10、DYN-02（发布入口）
- [x] M2-FLW-02 Web：个人空间页 /space/:uid（资料头部+关注按钮+关注/粉丝数；投稿/关注/粉丝/收藏(仅本人) Tab；播放页 UP 卡片与头像下拉入口） `2026-07-29`
  - 覆盖：FLW-02、ACC-12（完整版，跨模块见 [account spec](/specs/account/spec)）

> 转发到动态（M2-ITR-06，覆盖 DYN-03）见 [interaction tasks](/specs/interaction/tasks)。

## M3（W25-W48）

- [x] M3-FLW-01 私信拉黑/举报（原 progress 周志记录、开发清单漏登，迁移补录）：0027 迁移 user_block；relation Block/BlockStatus（POST /space/:id/block + block-status）；im Send 拉黑拦截（对方拉黑我→拒绝）；会话头部拉黑/举报按钮 + 被拉黑方输入禁用警告条 + 举报会话（target_type=5）；实测拉黑拦截/取消恢复/举报提交全通 `2026-08-10`
  - 覆盖：FLW-05、MSG-12（跨模块，见 [notification spec](/specs/notification/spec)）

## 进度

| 里程碑 | 任务数 | 已完成 |
| --- | :-: | :-: |
| M2 | 5 | 5 |
| M3 | 1 | 1 |
| **合计** | **6** | **6** |

> 勾选任务后同步更新上表与 [开发进度管理](/project/progress) 的模块矩阵。
