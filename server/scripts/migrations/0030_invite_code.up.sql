-- 0030_invite_code: 内测邀请码（ACC-44，M1-REL-04）
CREATE TABLE `invite_code` (
  `id`          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `code`        VARCHAR(16)  NOT NULL COMMENT '邀请码（大写字母数字，去掉易混淆字符）',
  `created_by`  BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '生成人（admin_user.id）',
  `used_by`     BIGINT UNSIGNED NULL COMMENT '使用者（user.id），NULL=未使用',
  `used_at`     DATETIME     NULL,
  `expires_at`  DATETIME     NULL COMMENT '过期时间，NULL=永久有效',
  `created_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY `uk_code` (`code`),
  KEY `idx_used` (`used_by`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='内测邀请码';
