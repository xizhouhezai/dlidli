-- 0002_upload: 上传文件登记表（秒传依据 + 稿件关联源文件）
CREATE TABLE `upload_file` (
  `id`         BIGINT UNSIGNED PRIMARY KEY COMMENT '雪花ID',
  `user_id`    BIGINT UNSIGNED NOT NULL COMMENT '首个上传者',
  `file_name`  VARCHAR(255) NOT NULL DEFAULT '' COMMENT '原始文件名',
  `file_hash`  CHAR(64)     NOT NULL COMMENT 'SHA-256',
  `file_size`  BIGINT       NOT NULL DEFAULT 0,
  `store_key`  VARCHAR(255) NOT NULL COMMENT '对象存储 key',
  `created_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY `uk_hash` (`file_hash`),
  KEY `idx_user` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='上传文件';
