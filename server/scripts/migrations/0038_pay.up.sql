-- 0038_pay: 支付基础设施（M4-PAY-01，monetization spec §6 / plan §2E）
-- 硬约束：金额一律整数「分」（BIGINT），禁用浮点；现金账户与虚拟币（硬币 coin_log / 金瓜子 melon_seed_*）严格分离，
--         本文件新增的四张资金/订单表**不得**被写入 coin 或 melon_seed 体系，也不得反向读取其余额（spec §6.1）。

-- pay_order：支付订单（状态机 0 待支付 / 1 已支付 / 2 已关闭 / 3 已退款）
--   uk_channel_trade_no 是回调幂等的第二道闸（第一道是条件 UPDATE 的 RowsAffected）：
--   MySQL 唯一索引允许多行 NULL，故未支付订单该列为 NULL 不互相冲突，支付成功后写入即锁定。
CREATE TABLE `pay_order` (
  `id`               BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `order_no`         VARCHAR(40)  NOT NULL COMMENT '对外业务单号（全局唯一）',
  `user_id`          BIGINT UNSIGNED NOT NULL COMMENT '下单用户 user.id',
  `amount`           BIGINT       NOT NULL COMMENT '金额（分，整数；禁用浮点）',
  `channel`          VARCHAR(16)  NOT NULL COMMENT '渠道标识：mock / alipay / wechat',
  `subject`          VARCHAR(128) NOT NULL DEFAULT '' COMMENT '商品描述',
  `status`           TINYINT      NOT NULL DEFAULT 0 COMMENT '0 待支付 1 已支付 2 已关闭 3 已退款',
  `channel_trade_no` VARCHAR(64)  NULL COMMENT '渠道交易号（未支付为 NULL）',
  `refund_amount`    BIGINT       NOT NULL DEFAULT 0 COMMENT '已退款金额（分）',
  `client_ip`        VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '下单来源 IP（风控留痕）',
  `expire_at`        DATETIME     NOT NULL COMMENT '过期时间（下单时确定，惰性关单依据）',
  `paid_at`          DATETIME     NULL COMMENT '支付成功时间',
  `closed_at`        DATETIME     NULL COMMENT '关单时间',
  `refunded_at`      DATETIME     NULL COMMENT '退款完成时间',
  `created_at`       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY `uk_order_no` (`order_no`),
  UNIQUE KEY `uk_channel_trade_no` (`channel_trade_no`),
  KEY `idx_user` (`user_id`, `id`),
  KEY `idx_status_expire` (`status`, `expire_at`),
  KEY `idx_created` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='支付订单（M4-PAY-01，状态机 + 金额整数分）';

-- cash_account：现金账户余额（与 melon_seed_account 同形但语义完全独立：现金 ≠ 金瓜子 ≠ 硬币）
CREATE TABLE `cash_account` (
  `id`         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `user_id`    BIGINT UNSIGNED NOT NULL COMMENT '用户 user.id',
  `balance`    BIGINT       NOT NULL DEFAULT 0 COMMENT '现金余额（分，≥0；事务行锁保证）',
  `created_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY `uk_user` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='现金账户（M4-PAY-01，与硬币/金瓜子严格分离）';

-- cash_log：现金流水（审计口径 + 资金幂等）
--   uk_biz_no 承载幂等：充值 biz_no = 订单号 pay:{order_no}，退款 = refund:{order_no}，
--   同一业务单号重复入账第二次插入撞唯一键、事务回滚、余额不变。
CREATE TABLE `cash_log` (
  `id`            BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `user_id`       BIGINT UNSIGNED NOT NULL COMMENT '归属用户',
  `delta`         BIGINT       NOT NULL COMMENT '变动量（分，正=入账 负=退款）',
  `balance_after` BIGINT       NOT NULL COMMENT '变动后余额快照（分）',
  `biz_type`      TINYINT      NOT NULL COMMENT '1 支付入账 2 退款',
  `biz_no`        VARCHAR(80)  NOT NULL COMMENT '业务单号（幂等键）',
  `order_no`      VARCHAR(40)  NOT NULL DEFAULT '' COMMENT '关联 pay_order.order_no',
  `remark`        VARCHAR(128) NOT NULL DEFAULT '',
  `created_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY `uk_biz_no` (`biz_no`),
  KEY `idx_user` (`user_id`, `id`),
  KEY `idx_order` (`order_no`),
  KEY `idx_created` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='现金流水（M4-PAY-01，幂等 + 对账口径）';

-- pay_callback_log：每一次渠道回调的原始报文与验签结果（审计与重放取证）
--   验签失败**也要落库**（spec §6.4）：否则「有人持续伪造回调」这一事实在系统内不可观测。
CREATE TABLE `pay_callback_log` (
  `id`               BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `channel`          VARCHAR(16)  NOT NULL COMMENT '渠道标识',
  `raw_body`         TEXT         NOT NULL COMMENT '原始报文（取证）',
  `signature`        VARCHAR(256) NOT NULL DEFAULT '' COMMENT '收到的签名',
  `sign_ok`          TINYINT      NOT NULL DEFAULT 0 COMMENT '1 验签通过 0 失败',
  `order_no`         VARCHAR(40)  NOT NULL DEFAULT '' COMMENT '报文解析出的订单号（解析失败留空）',
  `channel_trade_no` VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '报文解析出的渠道交易号',
  `amount`           BIGINT       NOT NULL DEFAULT 0 COMMENT '报文金额（分；金额不符异常判据）',
  `result`           VARCHAR(128) NOT NULL DEFAULT '' COMMENT '业务处理结果（文本）',
  `created_at`       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY `idx_order` (`order_no`),
  KEY `idx_created` (`created_at`),
  KEY `idx_sign` (`sign_ok`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='支付回调日志（M4-PAY-01，验签轨迹）';

-- pay_recon_daily：日对账单（渠道 × 日期聚合；重跑为 UPSERT 覆盖，保证人工反复核对不污染）
CREATE TABLE `pay_recon_daily` (
  `id`             BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `recon_date`     DATE         NOT NULL COMMENT '对账日期',
  `channel`        VARCHAR(16)  NOT NULL COMMENT '渠道标识',
  `order_count`    INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '当日下单笔数（created_at 口径）',
  `paid_count`     INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '当日支付成功笔数',
  `paid_amount`    BIGINT       NOT NULL DEFAULT 0 COMMENT '当日支付成功金额（分）',
  `refund_count`   INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '当日退款笔数',
  `refund_amount`  BIGINT       NOT NULL DEFAULT 0 COMMENT '当日退款金额（分）',
  `cash_log_amount` BIGINT      NOT NULL DEFAULT 0 COMMENT '当日现金流水净额（分，支付-退款）',
  `diff_count`     INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '异常单数',
  `status`         TINYINT      NOT NULL DEFAULT 0 COMMENT '0 正常 1 有异常',
  `created_at`     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY `uk_date_channel` (`recon_date`, `channel`),
  KEY `idx_date` (`recon_date`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='支付日对账单（M4-PAY-01）';

-- pay_recon_issue：异常订单清单（人工处理台数据源）
--   issue_type 1 金额不符 / 2 长期挂起 / 3 流水缺失 / 4 重复回调
--   status     0 待处理 / 1 已处理
CREATE TABLE `pay_recon_issue` (
  `id`          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `recon_date`  DATE         NOT NULL COMMENT '所属对账日期',
  `channel`     VARCHAR(16)  NOT NULL DEFAULT '' COMMENT '渠道标识',
  `order_no`    VARCHAR(40)  NOT NULL COMMENT '异常订单号',
  `issue_type`  TINYINT      NOT NULL COMMENT '1 金额不符 2 长期挂起 3 流水缺失 4 重复回调',
  `detail`      VARCHAR(255) NOT NULL DEFAULT '' COMMENT '异常详情',
  `status`      TINYINT      NOT NULL DEFAULT 0 COMMENT '0 待处理 1 已处理',
  `handled_by`  BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '处理人 admin_user.id',
  `handled_at`  DATETIME     NULL COMMENT '处理时间',
  `handle_note` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '处理备注',
  `created_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY `uk_date_order_type` (`recon_date`, `order_no`, `issue_type`),
  KEY `idx_status` (`status`, `id`),
  KEY `idx_date` (`recon_date`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='支付异常订单清单（M4-PAY-01 人工处理台）';
