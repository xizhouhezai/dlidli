-- 0027_user_block: 用户黑名单（MSG-12 私信拉黑拦截）
CREATE TABLE user_block (
  id          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  uid         BIGINT UNSIGNED NOT NULL COMMENT '发起拉黑的用户',
  blocked_uid BIGINT UNSIGNED NOT NULL COMMENT '被拉黑的用户',
  created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uk_uid_blocked (uid, blocked_uid),
  KEY idx_blocked (blocked_uid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户黑名单';
