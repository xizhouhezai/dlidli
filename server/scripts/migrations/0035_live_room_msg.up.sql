-- 0035_live_room_msg: 直播弹幕历史窗口（M4-LIV-03，LIV-12）
-- 直播弹幕不落视频域弹幕表：不与稿件生命周期绑定，仅保留每房间最近 N 条
-- 供进房拉历史；下播时按场次时间清理（存档合规走截帧+回放，见 plan live §2B.2）。
CREATE TABLE `live_room_msg` (
  `id`         BIGINT UNSIGNED PRIMARY KEY COMMENT '雪花 ID（应用侧生成）',
  `room_id`    BIGINT UNSIGNED NOT NULL COMMENT '直播间 live_room.id',
  `user_id`    BIGINT UNSIGNED NOT NULL COMMENT '发送者 user.id',
  `nickname`   VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '发送时昵称快照',
  `avatar`     VARCHAR(512) NOT NULL DEFAULT '' COMMENT '发送时头像 URL 快照',
  `content`    VARCHAR(100) NOT NULL COMMENT '弹幕内容（≤50 字）',
  `created_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY `idx_room_time` (`room_id`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='直播弹幕历史窗口（M4-LIV-03）';
