-- 0040_video_quality 回滚：删档位配置表 + 移除 video_stream.height 列。
-- 注意：回滚**不会**删除已转出的 1080/2160 转码产物行（video_stream 行与磁盘 HLS 目录保留）——
-- 它们是合法产物，只是回滚后不再有档位配置可供鉴权；重新升级即恢复。
DROP TABLE IF EXISTS `video_quality_tier`;

ALTER TABLE `video_stream` DROP COLUMN `height`;
