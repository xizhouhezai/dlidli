-- 0039_vip: 大会员套餐/权益/连续包月（M4-VIP-01，monetization spec §2.1 / §2.2 / §2.7 / plan §2F）
-- 硬约束：
--   1. 会员判定一律以 `membership.expire_at > NOW()` 为准（服务端判定，不缓存，理由见 plan §2F.2）。
--   2. 金额一律整数「分」（BIGINT），禁用浮点（沿用 0038 口径）。
--   3. 赠的是**硬币**（user.coin / coin_log），不是金瓜子（melon_seed_*）——spec 中两者不同；
--      本文件新增的表**不得**触碰 melon_seed_* 体系。
--   4. 同一订单只能生效一次：`vip_order.uk_order_no` 是「订单 → 会员权益」幂等的第二道闸
--      （第一道是 pay_order 条件 UPDATE 的 RowsAffected，见 0038 注释）。

-- pay_order 补列：sku 指向所购商品（本卡为套餐 code）。PAY-01 的订单表不含商品标识，
-- 而「支付成功 → 该给用户开哪档会员」必须能从订单本身判定（回调侧只拿得到订单号与金额，
-- 金额可被改价影响，不可作为套餐判据）。
ALTER TABLE `pay_order`
  ADD COLUMN `sku` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '商品标识（会员卡为 vip_plan.code）' AFTER `subject`;

-- vip_plan：会员套餐（月/季/年 + 连续包月首月优惠）。管理端可改价与上下架。
--   months          时长月数（1/3/12）
--   price           现价（分）
--   original_price  划线价（分，0 表示不展示）
--   first_month_price 连续包月首月优惠价（分，0 表示无优惠）
--   auto_renew      1 支持连续包月（可与一次性购买并存）
CREATE TABLE `vip_plan` (
  `id`                BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `code`              VARCHAR(32)  NOT NULL COMMENT '套餐标识（下单 sku）',
  `name`              VARCHAR(64)  NOT NULL COMMENT '展示名，如「大会员 月度」',
  `months`            INT UNSIGNED NOT NULL COMMENT '时长（月）',
  `price`             BIGINT       NOT NULL COMMENT '现价（分，整数；禁用浮点）',
  `original_price`    BIGINT       NOT NULL DEFAULT 0 COMMENT '划线价（分，0=不展示）',
  `first_month_price` BIGINT       NOT NULL DEFAULT 0 COMMENT '连续包月首月优惠价（分，0=无优惠）',
  `auto_renew`        TINYINT      NOT NULL DEFAULT 0 COMMENT '1 支持连续包月 0 仅一次性',
  `sort`              INT          NOT NULL DEFAULT 0 COMMENT '展示排序（小在前）',
  `status`            TINYINT      NOT NULL DEFAULT 0 COMMENT '0 下架 1 上架',
  `created_at`        DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`        DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY `uk_code` (`code`),
  KEY `idx_status_sort` (`status`, `sort`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='会员套餐（M4-VIP-01）';

-- membership：会员状态（user_id 唯一，一人一行）。
--   **判定只看 expire_at > NOW()**：不引入 level 门限的额外判定，避免出现
--   「level=1 但已过期」仍被判为会员这类自相矛盾状态（level 仅作未来多档预留）。
--   started_at 记录首次开通时间（首月优惠资格判据之一）。
CREATE TABLE `membership` (
  `id`              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `user_id`         BIGINT UNSIGNED NOT NULL COMMENT '用户 user.id',
  `level`           TINYINT      NOT NULL DEFAULT 1 COMMENT '会员档位（V3 仅 1=大会员）',
  `expire_at`       DATETIME     NOT NULL COMMENT '到期时间（判定依据：expire_at > NOW() 即会员）',
  `auto_renew`      TINYINT      NOT NULL DEFAULT 0 COMMENT '1 连续包月生效中 0 未签约',
  `started_at`      DATETIME     NOT NULL COMMENT '首次开通时间（首月优惠资格）',
  `total_months`    INT          NOT NULL DEFAULT 0 COMMENT '累计购买月数（续期累加，审计口径）',
  `last_order_no`   VARCHAR(40)  NOT NULL DEFAULT '' COMMENT '最近一次生效订单号',
  `created_at`      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY `uk_user` (`user_id`),
  KEY `idx_expire` (`expire_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='会员状态（M4-VIP-01，判定依据 expire_at > NOW()）';

-- vip_order：订单 → 会员权益的关联与生效台账（幂等第二道闸）。
--   uk_order_no 承载「同一订单只生效一次」：重放回调时若权益已生效，第二道闸先拦下；
--   expire_before/expire_after 记录顺延前后的精确值，供验收与客诉核对（顺延语义的审计凭据）。
CREATE TABLE `vip_order` (
  `id`            BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `order_no`      VARCHAR(40)  NOT NULL COMMENT '关联 pay_order.order_no',
  `user_id`       BIGINT UNSIGNED NOT NULL,
  `plan_code`     VARCHAR(32)  NOT NULL COMMENT '套餐 code（下单时的 sku）',
  `months`        INT          NOT NULL COMMENT '本次购买时长（月）',
  `apply_status`  TINYINT      NOT NULL DEFAULT 0 COMMENT '0 未生效 1 已生效（权益已发放）',
  `expire_before` DATETIME     NULL COMMENT '生效前到期时间（NULL=首次开通）',
  `expire_after`  DATETIME     NULL COMMENT '生效后到期时间（顺延结果）',
  `applied_at`    DATETIME     NULL COMMENT '生效时间',
  `created_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY `uk_order_no` (`order_no`),
  KEY `idx_user` (`user_id`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='会员订单生效台账（M4-VIP-01，订单幂等第二道闸）';

-- vip_subscription：连续包月签约（user_id 唯一，一人一份签约）。
--   next_charge_at 是续期扫描的驱动字段；**续期用 CAS 推进它**
--   （`WHERE user_id=? AND status=1 AND next_charge_at=<读取值>`），
--   故多实例并发扫描只会有一方抢到本轮续期，无需分布式锁（plan §2F.5）。
CREATE TABLE `vip_subscription` (
  `id`             BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `user_id`        BIGINT UNSIGNED NOT NULL,
  `plan_code`      VARCHAR(32)  NOT NULL COMMENT '签约套餐 code',
  `channel`        VARCHAR(16)  NOT NULL DEFAULT 'mock' COMMENT '扣款渠道（mock 为换入点占位）',
  `status`         TINYINT      NOT NULL DEFAULT 1 COMMENT '1 签约中 0 已解约',
  `sign_at`        DATETIME     NOT NULL COMMENT '签约时间',
  `cancel_at`      DATETIME     NULL COMMENT '解约时间',
  `next_charge_at` DATETIME     NOT NULL COMMENT '下次扣款时间（续期扫描驱动）',
  `last_charge_at` DATETIME     NULL COMMENT '最近一次扣款（成功）时间',
  `fail_count`     INT          NOT NULL DEFAULT 0 COMMENT '累计扣款失败次数',
  `created_at`     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY `uk_user` (`user_id`),
  KEY `idx_status_next` (`status`, `next_charge_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='连续包月签约（M4-VIP-01，自动续期走 mock 渠道）';

-- vip_coin_grant：月度赠币发放台账（**幂等核心**）。
--   uk_user_period(user_id, period_key) 是幂等唯一闸：period_key = 'YYYY-MM'，
--   同月第二次发放撞唯一键 → 事务回滚 → 硬币与流水都不会变化。
--   coin_log_id 指向本次产生的 coin_log 行，便于「币从哪来」的溯源与验收逐条核对。
CREATE TABLE `vip_coin_grant` (
  `id`          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `user_id`     BIGINT UNSIGNED NOT NULL COMMENT '用户 user.id',
  `period_key`  VARCHAR(16)  NOT NULL COMMENT '发放周期键（YYYY-MM，月度幂等）',
  `amount`      INT          NOT NULL COMMENT '发放硬币枚数（spec §2.1：每月 5 枚）',
  `coin_log_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '关联 coin_log.id（溯源）',
  `source`      TINYINT      NOT NULL DEFAULT 1 COMMENT '1 开通/续期即时发放 2 定时补发 3 管理端补发',
  `created_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY `uk_user_period` (`user_id`, `period_key`),
  KEY `idx_period` (`period_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='会员月度赠币台账（M4-VIP-01，uk_user_period 幂等）';
