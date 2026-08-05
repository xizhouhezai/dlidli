-- 0021_multipart: 多P投稿（M3-CRT-05 / PRD VID-05）
CREATE TABLE video_part (
  id         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  video_id   BIGINT UNSIGNED NOT NULL COMMENT '稿件',
  part_index TINYINT NOT NULL DEFAULT 1 COMMENT '第几P（1起）',
  title      VARCHAR(80) NOT NULL DEFAULT '' COMMENT '分P标题',
  duration   INT      NOT NULL DEFAULT 0 COMMENT '时长（秒）',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uk_video_part (video_id, part_index)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='多P投稿分P';

-- 播放流增加分P归属（0=单P默认，>0 对应 video_part.part_index）
ALTER TABLE video_stream ADD COLUMN part_index TINYINT NOT NULL DEFAULT 0 COMMENT '分P（0单P）' AFTER video_id;
