# plan：弹幕系统

> 对应规格：[spec](/specs/danmaku/spec) ｜ 技术基线：[后端架构](/architecture/backend) §2.3 弹幕链路 · [数据模型](/architecture/data-model)
> 实现位置：`server/internal/module/danmaku`（handler/service/repo/hub）

## 1. 方案概览

写路径：HTTP 发送 → 机审（moderate/contentmod）→ 频控与去重（Redis）→ MySQL 落库 + Redis 分段缓存 → WS 房间广播。读路径：按 `video_id + 段号(6min)` 读 Redis（miss 回源构建）。屏蔽采用双层过滤：服务端拉取接口登录态过滤 + 实时广播客户端本地过滤。

## 2. 技术决策

| 决策点 | 决策 | 理由 | 备选方案 |
| --- | --- | --- | --- |
| 分段缓存 | Redis zset `dm:{video_id}:{seg}`，6min/段，写后失效，7d 滑动 | 时间轴按段拉取，缓存粒度与读取粒度一致 | 全量缓存（大视频内存浪费） |
| 实时通道 | gorilla/websocket 按视频房间广播（`/api/v1/videos/:bvid/danmaku/ws`），Origin 白名单防劫持，单房间 5000 连接上限 | 同视频强 locality，房间模型简单 | 全局 comet 网关（V1 预留，弹幕先行独立实现） |
| 前端降级 | WS 断线重连（5 次指数退避），不可用回退 HTTP 分段拉取 | 实时性是增强而非强依赖 | 仅轮询（延迟高） |
| 频控 | 5s 频控 + 30s 重复内容去重（Redis，错误码 40004） | 防刷屏的最小充分手段 | 消息队列削峰（过度设计） |
| 等级门槛 | Lv1 发送、Lv3 彩色/顶底，服务端强校验（40003） | 权益必须服务端判定，前端仅置灰提示 | 仅前端拦截（可绕过） |
| 渲染引擎 | M1：DOM 轨道分配渲染（滚动/顶部/底部三模式）；Canvas 高性能版随 packages/player 重构 | 先跑通再优化 | 首版即 Canvas（调试成本高） |
| 屏蔽存储 | `danmaku_block` 表账号级：关键词 ≤ 200 + 用户按 UID 哈希/直传哈希 | 跨设备生效（服务端存储）优于 localStorage | 纯本地存储（不跨端） |
| 机审 | 敏感词热加载词库（moderate 包）+ 预留外部内容安全 API，命中影子屏蔽 | 先发后审的轻量平衡 | 全量人审（成本高） |

## 3. 数据模型

全局见 [数据模型](/architecture/data-model)（`danmaku` 表：mode 1滚动/2顶部/3底部、color RGB、time_ms 视频内毫秒）。模块私有（0014 迁移）：

```sql
danmaku_block { id, user_id, block_type(1关键词 2用户), keyword/block_hash, created_at }
```

Redis：`dm:{video_id}:{seg}`（zset / 7d 滑动）、频控与去重键。

## 4. 接口设计

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | /api/v1/videos/{bvid}/danmaku?segment= | 分段拉取（登录态过滤屏蔽项） |
| POST | /api/v1/videos/{bvid}/danmaku | 发送（Lv 门槛 + 频控 + 机审） |
| WS | /api/v1/videos/{bvid}/danmaku/ws | 实时房间广播（Vite 代理 ws:true） |
| GET/POST/DELETE | /api/v1/users/me/danmaku-blocks | 屏蔽词/屏蔽用户 CRUD |

> 完整契约以 swaggo 生成的 OpenAPI 为准（`server/docs/`）。

## 5. 关键流程

```
发送：输入框（模式/颜色工具条，Lv3 置灰提示）→ POST → 机审影子屏蔽 → 5s/30s 频控去重
     → MySQL 落库 + Redis 段缓存 → WS 推送同房间连接 → 各端乐观上屏 + 本地屏蔽过滤
读取：播放进度 → 按段拉取（预取下一段）→ 渲染（不透明度/字号/区域/速度/密度设置）
```

## 6. 风险与待定项

- [ ] Canvas 高性能渲染引擎（随 packages/player 重构）
- [ ] 高级弹幕（DM-05，消耗硬币的位置/动画）
- [ ] 防挡字幕（DM-14，需画面区域检测）
- [ ] 弹幕点赞（复用 user_action obj_type=3，延后）
- [ ] UP 主弹幕管理（DM-23，延后）
