-- 回滚 0043：去掉租约/退避/耗时的列与索引，恢复原 idx_status。
-- 注意：回滚会丢失租约信息，正在处理中的任务（status=1）将无法被回收，
-- 回滚前应先确保没有 processing 任务（或手工把它们置回 status=0）。
DROP INDEX `idx_lease` ON `transcode_job`;
DROP INDEX `idx_claim` ON `transcode_job`;
ALTER TABLE `transcode_job` ADD INDEX `idx_status` (`status`, `updated_at`);
ALTER TABLE `transcode_job`
  DROP COLUMN `finished_at`,
  DROP COLUMN `started_at`,
  DROP COLUMN `worker_id`,
  DROP COLUMN `next_retry_at`,
  DROP COLUMN `lease_until`;
