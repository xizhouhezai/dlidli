# tasks：视频投稿与播放

> 对应规格：[spec](/specs/video/spec) ｜ 方案：[plan](/specs/video/plan)
> 任务编号：`{阶段}-{模块}-{序号}`；完成即勾选并追加完成日期；括号补注实现要点与验证结论。
> 多 P 投稿与合集（M3-CRT-05，覆盖 VID-05、VID-23）随创作者中心任务交付，见 [creator tasks](/specs/creator/tasks)。

## M1（W5-W12）

- [x] M1-VID-01 后端：上传初始化/分片上传/完成合并/秒传接口（本地无 Docker：分片直传业务服务器+本地磁盘，SHA-256 校验；MinIO 预签名直传待部署环境接入） `2026-07-28`
  - 覆盖：VID-01
- [x] M1-VID-02 后端：稿件 CRUD + 状态机 + 分区管理（投稿/详情/我的稿件/公开列表/软删除；dev autoApprove 自动过审，待审核后台上线后关闭） `2026-07-28`
  - 覆盖：VID-02、VID-07、VID-20、状态机（spec §2.3）
- [x] M1-VID-03 后端：转码任务队列（本地无 Kafka：DB 任务表 + FOR UPDATE SKIP LOCKED 认领 + 失败重试 2 次；部署环境可平滑换 Kafka） `2026-07-29`
  - 覆盖：VID-10
- [x] M1-VID-04 Worker：FFmpeg HLS 转码（360P/720P）+ 抽帧封面 + ffprobe 时长（dev 内嵌进程 Worker，状态机：转码中→待审/发布） `2026-07-29`
  - 覆盖：VID-10、VID-11、VID-03
- [x] M1-VID-05 后端：播放地址签名下发接口（playsign 包 HMAC-SHA256 签名路径+过期；video 详情/审核预览下发 `?e=&s=` 签名 URL（TTL 6h）；PlaySignGuard 中间件校验 /static 下 videos 播放入口（.m3u8/.mp4），.ts分片/封面/头像放行；E2E：签名访问200/无签403/篡改403/播放readyState4） `2026-07-31`
  - 覆盖：PLY-08
- [x] M1-VID-06 后端：观看进度记录 + 有效播放计数（Redis 去重）（进度：Redis 存储 90d + 前端 10s 节流上报/离开落盘/续播定位；计数：>5s 上报 + UID/IP 8h 去重） `2026-07-29`
  - 覆盖：PLY-04、PLY-05
- [x] M1-VID-07 Web：投稿页（拖拽上传、hash-wasm 增量校验、并发分片+断点续传+秒传、表单/标签/分区、封面：首帧截取/自定义上传/DliDli 默认封面三级兜底） `2026-07-28`
  - 覆盖：VID-01、VID-02、VID-03
- [x] M1-VID-08 Web：稿件管理页（列表/状态标签/数据概览/删除/分页；编辑入口随 VID-06 稿件编辑接口补） `2026-07-29`
  - 覆盖：—（创作者中心稿件管理的前置载体，需求见 [creator spec](/specs/creator/spec) CRT-01~03）
- [x] M1-VID-09 packages/player：hls.js 内核封装（独立包 @dlidli/player：PlayerCore 框架无关内核封装 hls.js 生命周期/清晰度切换保留进度/mp4兜底；bindKeyboard 快捷键（空格/k播停、←→快进退、↑↓音量、m静音、f全屏）；qualityLabel/pickDefaultSource 工具；VideoView 接入 + 新增倍速 UI（0.5~2x）；浏览器实测 readyState4/快捷键/倍速/切清晰度保进度全绿） `2026-07-31`
  - 覆盖：PLY-01、PLY-03、PLY-06
- [x] M1-VID-10 Web：视频详情页（原生播放器播原画 + 信息区/UP主卡片/简介标签 + 同分区热度相关推荐；多清晰度/弹幕待转码与 DM 模块） `2026-07-29`
  - 覆盖：PLY-30、PLY-31
- [x] M1-VID-11 Web：首页（分区导航 + 最新/最热 Tab + 加载更多分页，全部接真实数据） `2026-07-29`
  - 覆盖：VID-20（分区导航与列表）

## 进度

| 里程碑 | 任务数 | 已完成 |
| --- | :-: | :-: |
| M1 | 11 | 11 |
| **合计** | **11** | **11** |

> 勾选任务后同步更新上表与 [开发进度管理](/project/progress) 的模块矩阵。
