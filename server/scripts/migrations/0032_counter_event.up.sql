-- 0032_counter_event: 计数服务幂等事件登记（M3-ENG-01）
-- ApplyDelta 以 event_id 为幂等键：同一事件重复投递只累加一次。
-- 该表由计数服务独占，不参与分表（M3-ENG-03）。
-- 注意：column 是 MySQL 保留字，建表与查询都必须反引号包裹。
CREATE TABLE `counter_event` (
  `event_id`   VARCHAR(128) NOT NULL COMMENT '幂等键（调用方生成）',
  `video_id`   BIGINT UNSIGNED NOT NULL COMMENT '视频内部 ID',
  `column`     VARCHAR(32)  NOT NULL COMMENT '计数列（白名单）',
  `delta`      BIGINT       NOT NULL DEFAULT 0 COMMENT '本次增量',
  `created_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`event_id`),
  KEY `idx_video` (`video_id`),
  KEY `idx_created` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='计数幂等事件登记（M3-ENG-01）';
