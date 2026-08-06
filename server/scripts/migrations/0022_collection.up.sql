-- 0022_collection: UP 主合集（M3-CRT-05 合集部分，PRD 扩展）
-- 注：表名 video_collection（避免与收藏夹 collection 表冲突；0025 为历史修复记录）
CREATE TABLE video_collection (
  id          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  user_id     BIGINT UNSIGNED NOT NULL COMMENT 'UP主',
  title       VARCHAR(64)  NOT NULL COMMENT '合集标题',
  description VARCHAR(200) NOT NULL DEFAULT '' COMMENT '简介',
  cover       VARCHAR(255) NOT NULL DEFAULT '' COMMENT '封面（空则取首个视频封面）',
  created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_user (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='UP主视频合集';

CREATE TABLE video_collection_item (
  id            BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  collection_id BIGINT UNSIGNED NOT NULL,
  video_id      BIGINT UNSIGNED NOT NULL,
  sort          INT NOT NULL DEFAULT 0 COMMENT '排序（小在前）',
  created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uk_collection_video (collection_id, video_id),
  KEY idx_collection (collection_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='合集内稿件';
