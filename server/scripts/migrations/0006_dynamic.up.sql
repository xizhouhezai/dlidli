-- 0006_dynamic: 动态（DYN-01/02）
-- MVP 拉模式 Feed：按关注列表查动态，游标分页；规模化后推拉结合（收件箱表）
CREATE TABLE `dynamic` (
  `id`         BIGINT UNSIGNED PRIMARY KEY COMMENT '雪花ID，兼作游标',
  `user_id`    BIGINT UNSIGNED NOT NULL,
  `type`       TINYINT         NOT NULL COMMENT '1投稿动态 2图文动态',
  `content`    VARCHAR(1000)   NOT NULL DEFAULT '',
  `video_id`   BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'type=1 时关联稿件',
  `status`     TINYINT         NOT NULL DEFAULT 0 COMMENT '0正常 1删除',
  `created_at` DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY `idx_user` (`user_id`, `id` DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='动态';
