-- 0017_creator: 创作者中心（M3-CRT-04 创作激励日结算）
CREATE TABLE creator_settlement (
  id          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  settle_date DATE NOT NULL COMMENT '结算日期',
  user_id     BIGINT UNSIGNED NOT NULL COMMENT '创作者',
  video_id    BIGINT UNSIGNED NOT NULL COMMENT '稿件',
  valid_views INT  NOT NULL DEFAULT 0 COMMENT '有效播放数（>5s）',
  amount      INT  NOT NULL DEFAULT 0 COMMENT '收益（分）',
  created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uk_date_video (settle_date, video_id),
  KEY idx_user_date (user_id, settle_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='创作激励日结算';
