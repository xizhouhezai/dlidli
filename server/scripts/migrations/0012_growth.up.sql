-- 0012_growth: 成长体系（M2-GRW-01 经验流水）
CREATE TABLE exp_log (
  id         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  user_id    BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
  delta      INT            NOT NULL COMMENT '经验增减',
  reason     VARCHAR(32)    NOT NULL COMMENT '来源 daily_login/daily_watch/video_upload/danmaku_send/comment_send',
  created_at DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_user (user_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='经验流水';
