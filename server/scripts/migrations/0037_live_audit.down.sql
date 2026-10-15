-- 0037_live_audit down: 回滚直播截帧机审与快速切断（M4-LIV-05）
ALTER TABLE `live_room`
  DROP COLUMN `ban_at`,
  DROP COLUMN `ban_by`,
  DROP COLUMN `ban_reason`,
  DROP COLUMN `ban_status`;

DROP TABLE IF EXISTS `live_violation_log`;
DROP TABLE IF EXISTS `live_frame_audit`;
