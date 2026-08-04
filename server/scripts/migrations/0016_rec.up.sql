-- 0016_rec: 推荐系统（M3-REC 行为采集/负反馈/开关）
-- 行为日志（本地 MySQL 落库；规模化后迁移 ClickHouse，字段保持兼容）
CREATE TABLE user_behavior (
  id         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  user_id    BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '游客为0',
  video_id   BIGINT UNSIGNED NOT NULL,
  action     TINYINT NOT NULL COMMENT '1曝光 2点击 3播放(>5s) 4互动(赞/币/藏/评)',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_user (user_id, created_at),
  KEY idx_video (video_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户行为日志';

-- 负反馈（不感兴趣）
CREATE TABLE user_dislike (
  id          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  user_id     BIGINT UNSIGNED NOT NULL,
  target_type TINYINT NOT NULL COMMENT '1内容 2UP主 3分区',
  target_id   BIGINT UNSIGNED NOT NULL,
  created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uk_user_target (user_id, target_type, target_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='负反馈（7天内不推荐同类）';

-- 个性化推荐开关（合规）
ALTER TABLE user ADD COLUMN recommend_on TINYINT NOT NULL DEFAULT 1 COMMENT '0关闭 1开启（关闭后退化为热度榜）';
