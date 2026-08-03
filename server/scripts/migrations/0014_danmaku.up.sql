-- 0014_danmaku: 弹幕进阶（M2-DM-02 屏蔽设置）
CREATE TABLE danmaku_block (
  id         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  user_id    BIGINT UNSIGNED NOT NULL COMMENT '设置屏蔽的用户',
  block_type TINYINT       NOT NULL COMMENT '1关键词 2用户',
  keyword    VARCHAR(64)   NOT NULL DEFAULT '' COMMENT '关键词（type=1）',
  block_hash VARCHAR(64)   NOT NULL DEFAULT '' COMMENT '被屏蔽用户哈希（type=2，不暴露UID）',
  created_at DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uk_user_type_val (user_id, block_type, keyword, block_hash),
  KEY idx_user (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='弹幕屏蔽设置';
