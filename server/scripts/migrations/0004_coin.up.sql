-- 0004_coin: 硬币流水（ACC-21 硬币体系）
CREATE TABLE `coin_log` (
  `id`         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `user_id`    BIGINT UNSIGNED NOT NULL,
  `delta`      INT          NOT NULL COMMENT '正为获取，负为消耗',
  `reason`     VARCHAR(32)  NOT NULL COMMENT 'register/daily_login/coin_video...',
  `created_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY `idx_user` (`user_id`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='硬币流水';
