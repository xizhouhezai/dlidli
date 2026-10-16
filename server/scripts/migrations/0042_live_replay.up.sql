-- 0042_live_replay: 直播回放（M4-LIV-14，spec LIV-14 / plan liv14）
--
-- 背景：spec §4 的「直播内容存档 ≥60 天」与 LIV-14「下播自动生成回放稿件」共用一个前置——
--   **推流全量录制**。本迁移只落「回放素材 → 转码 → 草稿稿件」这条链的登记表，
--   录制本身由 SRS DVR 落盘（server/deploy/srs/srs.conf，plan liv14 §3.1）。
--
-- 职责边界（刻意收窄）：
--   - 本表是**回放素材的登记与追溯**（哪场直播、哪个录制文件、生成了哪个稿件、成败原因）。
--   - 稿件本身的可见性/状态**权威在 video 表**（草稿=0，发布=4）：本表不复制该状态，
--     避免两处真相。列表接口按 video_id 关联读出真实状态。
--   - 录制文件的合规保留期由本表 created_at + live.replay.retentionDays 判定（plan liv14 §3.5）。
CREATE TABLE `live_replay` (
  `id`                 BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `room_id`            BIGINT UNSIGNED NOT NULL COMMENT '直播间 live_room.id（关联追溯，spec LIV-14 关联条款）',
  `anchor_id`          BIGINT UNSIGNED NOT NULL COMMENT '主播 user.id（稿件归属，plan liv14 D5）',
  `session_started_at` DATETIME     NOT NULL COMMENT '本场开播时间（快照，房间随后会进入下一场）',
  `session_ended_at`   DATETIME     NOT NULL COMMENT '本场下播时间（幂等键的一部分：同场只登记一次）',
  `dvr_key`            VARCHAR(255) NOT NULL DEFAULT '' COMMENT '录制文件相对 dvrDir 的路径（缺失/未找到时为空）',
  `source_key`         VARCHAR(255) NOT NULL DEFAULT '' COMMENT '转码源（存储 key，videos/replay/.../*.mp4）',
  `video_id`           BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '生成的稿件 video.id（0=未生成）',
  `duration`           INT          NOT NULL DEFAULT 0 COMMENT '录制时长（秒，ffprobe 实测）',
  `file_size`          BIGINT       NOT NULL DEFAULT 0 COMMENT '录制文件字节数',
  `status`             TINYINT      NOT NULL DEFAULT 0 COMMENT '0 待处理 1 已生成草稿 2 生成失败 3 超时长上限拒绝 4 素材已清理',
  `fail_reason`        VARCHAR(255) NOT NULL DEFAULT '' COMMENT '失败/拒绝原因（可追溯，spec LIV-14 失败处理）',
  `created_at`         DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`         DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  -- 幂等键：同一房间的同一场次（按开播+下播时刻）只登记一行 —— on_unpublish 可能重复投递
  -- （SRS 重启/回调重试），重复投递必须退化为无操作而不是生成第二个回放稿件。
  UNIQUE KEY `uk_room_session` (`room_id`, `session_started_at`, `session_ended_at`),
  KEY `idx_anchor` (`anchor_id`, `id`),
  KEY `idx_video` (`video_id`),
  -- 清理任务扫描用：按「未清理 + 超保留期」过滤
  KEY `idx_status_created` (`status`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='直播回放素材登记（M4-LIV-14）';
