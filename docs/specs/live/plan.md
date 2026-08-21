# plan：直播

> 对应规格：[spec](/specs/live/spec) ｜ 技术基线：[总体架构](/architecture/overview) · [视频处理流水线](/architecture/video-pipeline)
> 实施状态：**V3 未启动**，本 plan 为技术预研，启动前需细化评审。

## 1. 方案概览

推流（RTMP 接入）→ 转码集群（多码率）→ CDN 分发（HLS 起步，HTTP-FLV/WebRTC 降延迟演进）；弹幕网关复用/comet 房间广播模式分层扩展；礼物链路接入支付系统强一致事务。

## 2. 技术预研要点

| 决策点 | 方向 | 说明 |
| --- | --- | --- |
| 推流与分发 | RTMP → 转码集群（多码率）→ CDN（HLS 延迟 6-15s；后续 HTTP-FLV/WebRTC 降至 3s 内） | 延迟与成本平衡的演进路径 |
| 弹幕网关 | 单房间 10 万连接目标：网关分片 + 房间维度聚合分层广播 | 复用 [danmaku plan](/specs/danmaku/plan) hub 模型横向扩展 |
| 礼物链路 | 充值/扣款/分成走支付系统事务 + 对账（见 [monetization plan](/specs/monetization/plan)） | 资产强一致 |
| 内容安全 | 直播截帧机审（每 10s）+ 人工巡查 + 快速切断机制 | 合规硬要求 |
| 回放 | 下播后转码生成稿件（复用视频转码流水线） | 内容资产复用 |

## 3. 风险与待定项

- [ ] 直播资质与合规评估（启动前完成，见 spec §4）
- [ ] CDN 选型与成本测算
- [ ] 房间在线人数分层广播架构设计
