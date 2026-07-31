-- 0009_sensitive_word: 敏感词库（ADM-04，支持后台管理 + 热加载）
CREATE TABLE sensitive_word (
  id         BIGINT UNSIGNED PRIMARY KEY COMMENT '雪花ID',
  word       VARCHAR(64)     NOT NULL COMMENT '敏感词',
  created_at DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uk_word (word)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='敏感词库';
