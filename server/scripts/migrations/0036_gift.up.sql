-- 0036_gift: 礼物系统（M4-LIV-04，spec live LIV-13）
-- 金瓜子资产与硬币（user.coin/coin_log）严格分离（monetization spec 硬约束）；
-- 强一致口径：扣款/入账走 MySQL 事务 + SELECT ... FOR UPDATE 行锁（余额无负数）；
-- 充值幂等：melon_seed_log.uk_biz_no 唯一键承载，PAY-01 真实支付回调按 biz_no 防重放。

-- 金瓜子账户（余额；不存在则懒创建）
CREATE TABLE `melon_seed_account` (
  `id`         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `user_id`    BIGINT UNSIGNED NOT NULL COMMENT '用户 user.id',
  `balance`    BIGINT       NOT NULL DEFAULT 0 COMMENT '金瓜子余额（≥0，事务行锁保证）',
  `created_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY `uk_user` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='金瓜子账户（直播礼物资产，与硬币严格分离）';

-- 金瓜子流水（模式对齐 coin_log + 幂等单号；余额强一致由 account 行锁保证，流水为审计口径）
CREATE TABLE `melon_seed_log` (
  `id`            BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `user_id`       BIGINT UNSIGNED NOT NULL COMMENT '归属用户',
  `delta`         BIGINT       NOT NULL COMMENT '变动量（正=入账 负=扣减）',
  `balance_after` BIGINT       NOT NULL COMMENT '变动后余额快照',
  `biz_type`      TINYINT      NOT NULL COMMENT '1 充值(dev/支付) 2 送礼扣减 3 退款(预留 PAY-01)',
  `biz_no`        VARCHAR(64)  NOT NULL COMMENT '业务单号：充值=支付单号（回调幂等键）；送礼=gift-{雪花ID}',
  `remark`        VARCHAR(128) NOT NULL DEFAULT '',
  `created_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY `uk_biz_no` (`biz_no`),
  KEY `idx_user` (`user_id`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='金瓜子流水（充值/扣减审计 + 充值幂等）';

-- 礼物配置（名称/图标/单价/特效类型/分级）
CREATE TABLE `gift` (
  `id`         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `name`       VARCHAR(32)  NOT NULL COMMENT '礼物名',
  `icon`       VARCHAR(512) NOT NULL DEFAULT '' COMMENT '图标 URL（可空，前端 emoji 兜底）',
  `price`      BIGINT UNSIGNED NOT NULL COMMENT '单价（金瓜子）',
  `effect`     TINYINT      NOT NULL DEFAULT 1 COMMENT '特效类型 1 弹幕区连击 2 全屏特效',
  `level`      TINYINT      NOT NULL DEFAULT 1 COMMENT '分级 1 普通 2 高级',
  `status`     TINYINT      NOT NULL DEFAULT 0 COMMENT '0 上架 1 下架',
  `created_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  KEY `idx_status` (`status`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='礼物配置（M4-LIV-04）';

-- 初始礼物配置（价格数量级为开发口径，PAY-01 后可经后台调整）
INSERT INTO `gift` (`id`, `name`, `icon`, `price`, `effect`, `level`, `status`) VALUES
  (1, '辣条',     '', 100, 1, 1, 0),
  (2, '小心心',   '', 500, 1, 1, 0),
  (3, '元气满满', '', 2000, 2, 2, 0);

-- 礼物赠送流水（单价/分成比例按下单时快照，防配置变更污染历史）
CREATE TABLE `gift_log` (
  `id`          BIGINT UNSIGNED PRIMARY KEY COMMENT '雪花 ID',
  `room_id`     BIGINT UNSIGNED NOT NULL COMMENT '直播间',
  `sender_id`   BIGINT UNSIGNED NOT NULL COMMENT '送礼观众',
  `anchor_id`   BIGINT UNSIGNED NOT NULL COMMENT '主播（收益归属）',
  `gift_id`     BIGINT UNSIGNED NOT NULL COMMENT '礼物 gift.id',
  `count`       INT UNSIGNED    NOT NULL DEFAULT 1 COMMENT '连击数量',
  `unit_price`  BIGINT UNSIGNED NOT NULL COMMENT '下单时单价快照',
  `total_price` BIGINT UNSIGNED NOT NULL COMMENT 'unit_price × count',
  `share_rate`  INT UNSIGNED    NOT NULL COMMENT '主播分成比例‰（下单时快照）',
  `created_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY `idx_room` (`room_id`, `id`),
  KEY `idx_sender` (`sender_id`, `id`),
  KEY `idx_anchor` (`anchor_id`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='礼物赠送流水（M4-LIV-04）';

-- 主播分成流水（月度结算入 creator 体系的输入，PAY-02 消费；每笔赠送唯一）
CREATE TABLE `gift_share_log` (
  `id`              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `gift_log_id`     BIGINT UNSIGNED NOT NULL COMMENT '对应 gift_log.id（唯一，防重复分成）',
  `room_id`         BIGINT UNSIGNED NOT NULL,
  `anchor_id`       BIGINT UNSIGNED NOT NULL COMMENT '主播（收益归属）',
  `amount`          BIGINT UNSIGNED NOT NULL COMMENT '主播收益（金瓜子）= total × share_rate / 1000',
  `platform_amount` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '平台留存',
  `created_at`      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY `uk_gift_log` (`gift_log_id`),
  KEY `idx_anchor` (`anchor_id`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='礼物分成流水（月度结算输入，PAY-02）';
