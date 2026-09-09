-- 0031_search_outbox: 稿件搜索索引同步 Outbox（M2-SRH-02）
-- 稿件发布/下架/删除在事务内写本表，后台 Worker 异步同步 Elasticsearch；
-- 同稿件多条待处理记录按"最新覆盖"折叠（Enqueue 时先清旧待处理再插入）。
CREATE TABLE `search_index_outbox` (
  `id`         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `video_id`   BIGINT UNSIGNED NOT NULL COMMENT '稿件内部 ID',
  `action`     VARCHAR(16)    NOT NULL COMMENT 'upsert|delete',
  `attempts`   INT            NOT NULL DEFAULT 0 COMMENT '已尝试次数',
  `status`     TINYINT        NOT NULL DEFAULT 0 COMMENT '0待处理 1完成 2失败(超限)',
  `created_at` DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  KEY `idx_status_id` (`status`, `id`),
  KEY `idx_video` (`video_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='稿件搜索索引同步 Outbox';
