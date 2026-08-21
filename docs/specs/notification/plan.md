# plan：消息通知与私信

> 对应规格：[spec](/specs/notification/spec) ｜ 技术基线：[后端架构](/architecture/backend) · [数据模型](/architecture/data-model)
> 实现位置：`server/internal/module/notify`（通知）、`server/internal/module/im`（私信，含 hub）

## 1. 方案概览

通知为写扩散点对点模型（MVP 同库直写，规模化演进事件总线 + 聚合策略）；私信独立 `im` 模块：会话规范化存储 + 实时通道复用弹幕 hub 的房间模式（按 uid 分房间）。前端当前以 60s 轮询拉取未读，comet 网关上线后升级长连接。

## 2. 技术决策

| 决策点 | 决策 | 理由 | 备选方案 |
| --- | --- | --- | --- |
| 通知生成 | MVP 同库直写（四类触发点：赞/评论/回复/关注，自我触发过滤）；规模化演进 Kafka 事件总线 | 内联直写简单可靠；触发点已在各模块埋好 | 首版即 MQ（依赖重） |
| 通知存储 | MySQL 通知表（按接收者分表预留）+ Redis 未读计数 hash | 未读高频读写走缓存 | 纯 MySQL |
| 私信会话 | `conversation` 规范化（a<b 排序 upsert）+ `private_message` 明细，读取即已读 | 会话列表查询友好 | 仅消息表（会话需聚合） |
| 私信实时 | im hub 按 uid 分房间（参考弹幕 hub：Origin 白名单/慢消费者丢消息/ping 保活），WS query token 鉴权 | 复用已验证的 hub 模式 | 轮询（延迟高） |
| 发送限制 | 未互关每日 1 条（服务端校验），禁言拦截，机审 SceneComment | 防骚扰三道闸 | 仅前端提示（可绕过） |
| 前端触达 | 当前 60s 轮询未读 + 红点；comet 网关（M2-MSG-02 规划）承接 WS 下发 | 渐进演进 | 首版长连接（连接管理成本） |

## 3. 数据模型

```sql
notification      { id, receiver_id, type, content, is_read, created_at }   -- 按接收者分表预留
conversation      { id, user_a, user_b, last_msg_id, updated_at }           -- a<b 规范化，0026 迁移
private_message   { id, conversation_id, sender_id, content_type, content, is_read }
```

Redis：`unread:{uid}`（hash 常驻）、总未读计数。

## 4. 接口设计

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | /api/v1/notifications | 游标分页列表 |
| POST | /api/v1/notifications/read-all ｜ /read | 全部/单条已读 |
| GET | /api/v1/notifications/unread | 未读计数（轮询） |
| GET/POST | /api/v1/conversations ｜ /conversations/{id}/messages | 会话列表（含未读）/ 发送与分页 |
| WS | /api/v1/ws/im?token= | 私信实时下发（token 鉴权） |

> 完整契约以 swaggo 生成的 OpenAPI 为准（`server/docs/`）。

## 5. 关键流程

私信发送：输入（500 字限制）→ 禁言/拉黑拦截 → 机审敏感词 → 未互关每日 1 条校验 → 落库 + 会话 upsert → 接收方在线经 im hub 实时 Push，离线进未读。

## 6. 风险与待定项

- [ ] comet WebSocket 网关统一在线推送（通知 + 私信，当前轮询）
- [ ] 赞类通知聚合策略（规模化）
- [ ] 通知分类 Tab（MSG 分类细化）
- [ ] 离线推送（MSG-20，随 V3 App）
