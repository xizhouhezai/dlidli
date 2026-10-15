-- 0041_charge 回滚：删除充电/分成/提现新增表，并撤销 creator_settlement 的扩展列。
-- 顺序与 up 相反；creator_settlement 的既有行（创作激励）不受影响——
-- 只回滚列与索引，不删除任何已结算的激励数据（与 0040 down 同口径）。
DROP TABLE IF EXISTS `creator_charge_settlement`;
DROP TABLE IF EXISTS `withdraw_payout_log`;
DROP TABLE IF EXISTS `withdraw_order`;
DROP TABLE IF EXISTS `creator_balance_log`;
DROP TABLE IF EXISTS `creator_balance`;
DROP TABLE IF EXISTS `creator_income`;
DROP TABLE IF EXISTS `charge_log`;
DROP TABLE IF EXISTS `charge_tier`;

-- 必须**先删掉充电分成行**再恢复 uk_date_video：这些行的 video_id 可能为 0，
-- 多个创作者在同一月各有一行 (settle_date, video_id=0)，直接加唯一键会失败；
-- 且这些行本就是本卡引入的充电产物，回滚时不应保留（创作激励行不受影响）。
DELETE FROM `creator_settlement` WHERE `source` = 'charge';

ALTER TABLE `creator_settlement`
  DROP KEY `idx_source_user_date`,
  DROP KEY `uk_source_date_user_video`,
  DROP COLUMN `settlement_id`,
  DROP COLUMN `charge_count`,
  DROP COLUMN `source`,
  ADD UNIQUE KEY `uk_date_video` (`settle_date`, `video_id`);
