-- 0008_collection: 多收藏夹（ITR-03 完整版）
CREATE TABLE collection (
  id         BIGINT UNSIGNED PRIMARY KEY COMMENT '雪花ID',
  user_id    BIGINT UNSIGNED NOT NULL,
  name       VARCHAR(50)     NOT NULL,
  is_default TINYINT         NOT NULL DEFAULT 0 COMMENT '1=默认收藏夹（不可删）',
  created_at DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_user (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='收藏夹';

-- 收藏关系改为带 collection_id
ALTER TABLE user_action
  ADD COLUMN collection_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '收藏夹ID（action=3 时有效）' AFTER extra;
