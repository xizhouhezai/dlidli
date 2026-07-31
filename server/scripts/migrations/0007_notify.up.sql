-- 0007_notify: 站内通知（MSG-01）
CREATE TABLE `notify` (
  `id`         BIGINT UNSIGNED PRIMARY KEY COMMENT '雪花ID，兼作游标',
  `user_id`    BIGINT UNSIGNED NOT NULL COMMENT '收件人',
  `sender_id`  BIGINT UNSIGNED NOT NULL COMMENT '触发人',
  `type`       TINYINT         NOT NULL COMMENT '1点赞 2评论/回复 3关注 4系统',
  `content`    VARCHAR(200)    NOT NULL DEFAULT '',
  `link`       VARCHAR(200)    NOT NULL DEFAULT '' COMMENT '前端跳转路由',
  `is_read`    TINYINT         NOT NULL DEFAULT 0,
  `created_at` DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY `idx_user` (`user_id`, `id` DESC),
  KEY `idx_user_read` (`user_id`, `is_read`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='站内通知';
