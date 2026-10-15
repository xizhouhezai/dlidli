-- 0037_live_audit: 直播截帧机审与快速切断（M4-LIV-05，spec §5E / plan §2D）
-- 合规口径（spec §4）：截帧图 + 判定记录留存 ≥60 天。回放视频存档属 LIV-14，不在本表范围。

-- live_frame_audit: 每个截帧周期每房间一行 —— 「判定发生过」的审计凭据
--   pass     1=通过 0=命中违规
--   degraded 1=机审链路不可用（fail-open：不切断，仅记录；plan §2D.2 待替换点）
--   image_key 存储键；图片落盘失败时留空但判定仍须落库（spec §5E.5）
CREATE TABLE `live_frame_audit` (
  `id`         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `room_id`    BIGINT UNSIGNED NOT NULL COMMENT '直播间 live_room.id',
  `anchor_id`  BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '主播 user.id（冗余，便于按主播追溯）',
  `image_key`  VARCHAR(255) NOT NULL DEFAULT '' COMMENT '截帧图存储键（落盘失败留空）',
  `width`      INT NOT NULL DEFAULT 0 COMMENT '截帧宽度',
  `height`     INT NOT NULL DEFAULT 0 COMMENT '截帧高度',
  `pass`       TINYINT NOT NULL DEFAULT 1 COMMENT '1=通过 0=命中违规',
  `hit_words`  VARCHAR(255) NOT NULL DEFAULT '' COMMENT '命中原因（逗号分隔）',
  `degraded`   TINYINT NOT NULL DEFAULT 0 COMMENT '1=机审不可用（fail-open，不切断）',
  `cost_ms`    INT NOT NULL DEFAULT 0 COMMENT '截帧+机审耗时毫秒',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY `idx_room_time` (`room_id`, `created_at`),
  KEY `idx_created` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='直播截帧机审记录（合规存档，spec §5E.5）';

-- live_violation_log: 违规流水（机审命中 / 人工巡查切断）
--   source   1=机审自动 2=人工巡查
--   cut_mode 1=SRS API 踢流成功 2=SRS API 不可达（降级：仅封禁+登记，不静默）
--   cut_by   0=系统，>0=admin_user.id
CREATE TABLE `live_violation_log` (
  `id`             BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `room_id`        BIGINT UNSIGNED NOT NULL COMMENT '直播间 live_room.id',
  `anchor_id`      BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '主播 user.id',
  `source`         TINYINT NOT NULL DEFAULT 1 COMMENT '1=机审 2=人工巡查',
  `reason`         VARCHAR(255) NOT NULL DEFAULT '' COMMENT '违规原因',
  `cut_mode`       TINYINT NOT NULL DEFAULT 0 COMMENT '1=srs_api 2=degraded(仅封禁)',
  `frame_audit_id` BIGINT UNSIGNED NULL COMMENT '来源截帧记录（人工巡查无来源帧则为 NULL）',
  `cut_by`         BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '操作人 admin_user.id，0=系统',
  `created_at`     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY `idx_room_time` (`room_id`, `created_at`),
  KEY `idx_source` (`source`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='直播违规流水（M4-LIV-05）';

-- live_room 增列：封禁与状态机正交（plan §2D.3）——ban_status 是合规处置，
-- status 是场次生命周期（0 空闲/1 直播中/2 已下播），不复用以免污染 idx_status 列表查询。
ALTER TABLE `live_room`
  ADD COLUMN `ban_status` TINYINT      NOT NULL DEFAULT 0   COMMENT '0 正常 1 封禁' AFTER `live_cnt`,
  ADD COLUMN `ban_reason` VARCHAR(255) NOT NULL DEFAULT ''  COMMENT '封禁原因'        AFTER `ban_status`,
  ADD COLUMN `ban_by`     BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '封禁操作人 admin_user.id，0=系统自动' AFTER `ban_reason`,
  ADD COLUMN `ban_at`     DATETIME     NULL                 COMMENT '封禁时间'        AFTER `ban_by`;
