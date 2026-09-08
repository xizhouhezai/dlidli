# tasks：消息通知与私信

> 对应规格：[spec](/specs/notification/spec) ｜ 方案：[plan](/specs/notification/plan)
> 任务编号：`{阶段}-{模块}-{序号}`；完成即勾选并追加完成日期；括号补注实现要点与验证结论。

## M2（W13-W24）

- [x] M2-MSG-01 后端：通知投递（赞/评论/回复/关注四类触发点、自我触发过滤）+ 未读计数 + 全部已读 + 游标列表（MVP 同库直写；事件总线/聚合策略待规模化） `2026-07-29`
  - 覆盖：MSG-01、MSG-02、MSG-03、MSG-04、MSG-05
- [x] M2-MSG-02 后端：WebSocket 网关 comet（通知在线推送）（notify 模块新增 Hub：按 uid 房间推送 + Origin 白名单 + 30s ping 保活 + 慢消费者丢弃，与 im.Hub 同构；`Push` 写库后异步组装完整条目（含 sender 资料）推送给在线接收者，不阻塞点赞/评论/关注高频路径；新增 `GET /notifications/ws`（query token）；Hub nil 时安全降级为仅写库。Web 端 AppHeader 接入 WS 实时未读（指数退避重连，60s 轮询保留为兜底）+ NotifyView 监听 `notify-received` 实时插入列表头。**E2E 实测**：B 连 WS → A 关注 B → B 实时收到 `{type:"notify",data:{...关注了你...}}` 帧；新增 Hub 并发/离线/nil 单测，go build/vet/test 全绿） `2026-09-08`
  - 覆盖：MSG-13（通知侧在线推送）
- [x] M2-MSG-03 Web：消息中心（通知列表/未读红点/点击跳转/进页已读）+ 头部小铃铛未读角标；分类 Tab 待补 `2026-07-29`
  - 覆盖：MSG-01、MSG-05

## M3（W25-W48）

- [x] M3-IM-01 后端：会话/消息存储 + 发送限制 + 机审（0026 迁移 conversation/private_message；im 模块：发送（禁言拦截+机审敏感词 SceneComment+未互关每日 1 条/互关不限+会话 upsert 规范化 a<b）+ 会话列表（JOIN user 含未读）+ 消息分页（读取即已读）+ 总未读；E2E 实测：未互关第 2 条拦截/互关后放行/未读 1→0/敏感词拦截全通） `2026-08-05`
  - 覆盖：MSG-10、MSG-11、MSG-12
- [x] M3-IM-02 comet：私信实时下发（im hub 按 uid 分房间（参考弹幕 hub：Origin 白名单/慢消费者丢消息/ping 保活），发送成功即 Push 接收方；WS query token 鉴权（middleware 已支持）；浏览器实测：B 在线时 A 发消息 B 实时收到 frame） `2026-08-05`
  - 覆盖：MSG-13（私信侧实时）
- [x] M3-IM-03 Web/H5：私信会话界面（/messages 页：会话列表（头像/昵称/预览/未读/时间）+ 聊天窗（气泡 mine 右粉 peer 左灰/图片消息/500 字输入） + WS 实时接收；头部私信图标+未读红点（与通知同轮询）；空间页「发私信」按钮直达会话；修复 peer_id/sender_id 双重引号（string+,string）与 content_type 缺失） `2026-08-05`
  - 覆盖：MSG-10（前端）

## 进度

| 里程碑 | 任务数 | 已完成 |
| --- | :-: | :-: |
| M2 | 3 | 3 |
| M3 | 3 | 3 |
| **合计** | **6** | **6** |

> 勾选任务后同步更新上表与 [开发进度管理](/project/progress) 的模块矩阵。
