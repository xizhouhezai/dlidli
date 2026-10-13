-- 0034_live_room: 直播间服务域（M4-LIV-02，LIV-01/LIV-02/LIV-04 数据模型）
-- 房间生命周期：0 空闲（有资格未开播）/ 1 直播中 / 2 已下播（spec live §5A.1）
-- 推流密钥仅存 bcrypt 哈希，明文只在申请/重置接口返回一次（plan live §2A.4）
CREATE TABLE `live_room` (
  `id`              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `anchor_id`       BIGINT UNSIGNED NOT NULL COMMENT '主播 user.id，一主播一房',
  `title`           VARCHAR(80)  NOT NULL DEFAULT '' COMMENT '房间标题（1-80 字符，spec §5A.2）',
  `cover`           VARCHAR(512) NOT NULL DEFAULT '' COMMENT '封面 URL（可空，复用 upload 模块落 URL）',
  `category_id`     BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '分区（复用视频分区 category）',
  `status`          TINYINT      NOT NULL DEFAULT 0 COMMENT '0 空闲 1 直播中 2 已下播',
  `stream_key_hash` VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '推流密钥 bcrypt 哈希',
  `started_at`      DATETIME     NULL COMMENT '当前（或最近一场）开播时间',
  `ended_at`        DATETIME     NULL COMMENT '最近一场下播时间',
  `live_cnt`        BIGINT       NOT NULL DEFAULT 0 COMMENT 'DB 兜底在线计数（权威在 Redis，见 plan §2A.3）',
  `created_at`      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY `uk_anchor` (`anchor_id`),
  KEY `idx_status` (`status`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='直播房间';

-- 0034_live_whitelist: 开播白名单（LIV-01 资格第一道闸，初期白名单制）
CREATE TABLE `live_whitelist` (
  `id`         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `user_id`    BIGINT UNSIGNED NOT NULL COMMENT '白名单用户 user.id',
  `note`       VARCHAR(128) NOT NULL DEFAULT '' COMMENT '备注',
  `created_by` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '操作人（admin_user.id 或 0=运营脚本）',
  `created_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `removed_at` DATETIME     NULL COMMENT '移出时间，NULL=在名单内',
  UNIQUE KEY `uk_user` (`user_id`),
  KEY `idx_removed` (`removed_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='开播白名单（LIV-01）';

-- 0034_live_room_admin: 房间房管（LIV-04 数据模型；主播不落本表，由 live_room.anchor_id 判定）
CREATE TABLE `live_room_admin` (
  `id`         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `room_id`    BIGINT UNSIGNED NOT NULL COMMENT '直播间 live_room.id',
  `user_id`    BIGINT UNSIGNED NOT NULL COMMENT '房管 user.id',
  `created_by` BIGINT UNSIGNED NOT NULL COMMENT '任命人（主播）',
  `created_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY `uk_room_user` (`room_id`, `user_id`),
  KEY `idx_user` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='直播间房管（LIV-04）';

-- 0034_live_room_mute: 房间禁言（LIV-04 数据模型；弹幕通道执行在 M4-LIV-03）
-- 未到期重复禁言 UPSERT 覆盖 muted_until（spec §5C）
CREATE TABLE `live_room_mute` (
  `id`          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `room_id`     BIGINT UNSIGNED NOT NULL COMMENT '直播间 live_room.id',
  `user_id`     BIGINT UNSIGNED NOT NULL COMMENT '被禁言 user.id',
  `muted_until` DATETIME     NOT NULL COMMENT '禁言到期（1min-720h）',
  `reason`      VARCHAR(128) NOT NULL DEFAULT '' COMMENT '禁言原因',
  `created_by`  BIGINT UNSIGNED NOT NULL COMMENT '操作人（主播/房管）',
  `created_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY `uk_room_user` (`room_id`, `user_id`),
  KEY `idx_user` (`user_id`),
  KEY `idx_until` (`muted_until`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='直播间禁言（LIV-04）';
