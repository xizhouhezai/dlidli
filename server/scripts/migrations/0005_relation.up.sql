-- 0005_relation: 关注关系（FLW-01）
-- MVP 直接 COUNT 查询关注/粉丝数；量级上来后引入 user_stat 冗余计数表
CREATE TABLE `relation` (
  `id`         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `user_id`    BIGINT UNSIGNED NOT NULL COMMENT '发起关注的人',
  `target_id`  BIGINT UNSIGNED NOT NULL COMMENT '被关注的人',
  `created_at` DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY `uk_user_target` (`user_id`, `target_id`),
  KEY `idx_user` (`user_id`, `created_at`),
  KEY `idx_target` (`target_id`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='关注关系';
