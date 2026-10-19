-- 0043_transcode_scale: 转码积压治理——任务租约 + 失败退避 + 积压可观测（M4-ENG transcode-scale）
--
-- 背景：转码任务是 DB 队列，认领走 FOR UPDATE SKIP LOCKED（repo.ClaimJob），
-- 多 Worker 并发本就安全。缺的是「认领后进程崩溃」的回收：任务停在 status=1 后再无人碰，
-- 永久 pending 等价物，积压只增不减。本次补三件事：
--   1. **任务租约**：认领需续租（lease_until），过期即可被其他 Worker 重新认领。
--      不做租约的话，水平扩容把 Worker 进程数拉起来只会**放大**僵尸任务：进程越多，
--      中途被重启/崩溃带走的「假处理中」任务绝对数越多。
--   2. **失败退避**：next_retry_at + retry_cnt。原实现失败即重新入队，
--      坏文件会被两个 Worker 在毫秒级反复抢，形成热循环，把队列堵死。
--   3. **积压可观测**：started_at / finished_at 与 idx_status 组合出
--      「排队数、正在转、最老等待、平均耗时」四个口径（见 video/queue_stats.go）。
--
-- 兼容性：全部为新列且带默认值，旧行读出来即"无租约/无退避/无耗时"；
-- 旧 Worker 进程（不写这些列）仍能工作，可与新 Worker 混跑滚动升级。
ALTER TABLE `transcode_job`
  ADD COLUMN `lease_until`  DATETIME NULL DEFAULT NULL COMMENT '租约到期时间；NULL 表示未被持有。过期即可被其他 Worker 重新认领' AFTER `status`,
  ADD COLUMN `next_retry_at` DATETIME NULL DEFAULT NULL COMMENT '下次可认领时间（失败退避）；NULL 表示立即可认领' AFTER `lease_until`,
  ADD COLUMN `worker_id`    VARCHAR(96) NOT NULL DEFAULT '' COMMENT '持有者标识（hostname/pid/协程号），排障用' AFTER `next_retry_at`,
  ADD COLUMN `started_at`   DATETIME NULL DEFAULT NULL COMMENT '本次认领的开始时间（每次重试重置）' AFTER `worker_id`,
  ADD COLUMN `finished_at`  DATETIME NULL DEFAULT NULL COMMENT '终态时间（成功或进死信）；非终态为 NULL' AFTER `started_at`;

-- 认领热路径的索引：WHERE status=0 AND (next_retry_at IS NULL OR next_retry_at<=now) ORDER BY id
-- 原 idx_status(status, updated_at) 对 ORDER BY id 无帮助（id 是主键，但过滤条件先走 status）。
-- 换成 (status, next_retry_at, id) 后，认领扫描与回收扫描（status=1 AND lease_until<now）都能命中。
ALTER TABLE `transcode_job` DROP INDEX `idx_status`;
CREATE INDEX `idx_claim` ON `transcode_job` (`status`, `next_retry_at`, `id`);
CREATE INDEX `idx_lease` ON `transcode_job` (`status`, `lease_until`);
