-- 0025_collection_fix: UP主合集表改名（避免与收藏夹 collection 表冲突）
DROP TABLE IF EXISTS collection_item; -- 0022 中断遗留的孤儿表（无原表对应）

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
