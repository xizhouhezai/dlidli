# 视频处理流水线

> 状态：草案 ｜ 更新日期：2026-07-28

## 1. 全链路流程

```
① 客户端                ② dlidli-api              ③ Worker 集群
┌──────────┐  init    ┌──────────────┐  任务消息  ┌─────────────────┐
│ 计算hash  ├─────────►│ 秒传判断      │──Kafka───►│ 转码 Worker      │
│ 分片上传  ├─────────►│ 分片直传OSS   │           │ 1. 拉源片        │
│ (5MB×N)  │  merge   │ (预签名URL)   │           │ 2. FFmpeg 多档   │
│          ├─────────►│ 合并校验      │           │ 3. HLS 切片上传  │
└──────────┘          └──────┬───────┘           │ 4. 抽帧/封面/时长 │
                             │                    └───────┬─────────┘
                             ▼                            ▼
                      稿件状态: 转码中 ──────────────► 状态: 待审核
                                                          │
                      ┌───────────────────────────────────┤
                      ▼ 机审(V1)                           ▼ 人审
               内容安全 API(抽帧图/ASR文本)          管理后台审核工作台
                      └──────────────┬────────────────────┘
                                     ▼ 通过
                          状态: 已发布 → 事件广播(Kafka)
                          ├─ 生成投稿动态（Feed）
                          ├─ 同步搜索索引（ES）
                          ├─ 通知粉丝（V1）
                          └─ CDN 预热（热门 UP 主）
```

## 2. 上传设计

| 步骤 | 接口 | 说明 |
| --- | --- | --- |
| 初始化 | `POST /upload/init` | 提交文件名/大小/hash；命中已有文件即秒传返回 |
| 分片地址 | `POST /upload/{id}/parts` | 批量获取 OSS 预签名 URL（客户端直传，不过业务服务器） |
| 完成 | `POST /upload/{id}/complete` | 服务端校验分片 ETag、触发合并、投递转码任务 |
| 进度查询 | `GET /upload/{id}` | 已上传分片列表（断点续传依据） |

- 分片 5MB，前端并发 3；24h 未完成的上传任务由定时任务清理。

## 3. 转码规格

| 档位 | 分辨率 | 视频码率 | 编码 | 版本 |
| --- | --- | --- | --- | --- |
| 360P | 640×360 | 500 kbps | H.264 baseline | MVP |
| 720P | 1280×720 | 2000 kbps | H.264 main | MVP |
| 1080P | 1920×1080 | 4500 kbps | H.264 high | V1 |
| 4K | 3840×2160 | 12000 kbps | H.265 | V3（大会员） |

FFmpeg 关键参数（720P 示例）：

```bash
ffmpeg -i source.mp4 \
  -c:v libx264 -profile:v main -b:v 2000k -maxrate 2400k -bufsize 4000k \
  -vf "scale=-2:720" -r 30 -g 60 \
  -c:a aac -b:a 128k -ac 2 \
  -hls_time 6 -hls_playlist_type vod \
  -hls_segment_filename "seg_%05d.ts" index.m3u8
```

附加产物：封面候选（3/10/30% 进度抽帧）、雪碧图（进度条预览，V1）、时长与元信息（ffprobe）。

## 4. Worker 调度

- Kafka topic `video.transcode` 按 video_id 分区保序；Worker 消费并发 = CPU 核数相关。
- 任务状态机存 MySQL（`transcode_job`）：pending → running → success/failed（重试 2 次）。
- 心跳超时（10min 无进展）任务重新入队；档位并行转码，全部完成才置"待审核"。
- 扩容：Worker 无状态，直接加实例；GPU 转码（V2 评估）。

## 5. 分发与播放

- 播放地址下发：`GET /videos/{bvid}/play?quality=720` → 返回带签名的 CDN m3u8 URL（2h 有效）。
- 签名校验在 CDN 边缘（鉴权参数：过期时间 + uid + 路径 hash）。
- CDN 回源 MinIO/OSS；热门内容预热，冷内容按需回源。
- 客户端上报 QoS（起播、卡顿、错误码）→ ClickHouse（V2）监控播放质量。
