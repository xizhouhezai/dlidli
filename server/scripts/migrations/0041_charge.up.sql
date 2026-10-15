-- 0041_charge: 充电打赏 + 分成结算 + 提现（M4-PAY-02，monetization spec CHG-01/02/03、plan §2H）
--
-- 三条硬约束（延续 PAY-01 §6.1 与 LIV-04 的口径）：
--  1. 金额一律整数「分」：BIGINT / Go int64，全链路禁用浮点。
--  2. **充电是现金，不是虚拟币**：观众支付的现金只进 pay_order（复用 PAY-01 支付链路）；
--     创作者收益是**平台对创作者的现金负债**（creator_income / creator_balance），
--     本文件不含任何 coin_log / melon_seed_* 写入路径，也不反向读取其余额。
--  3. 分成口径与 M4-LIV-04 礼物一致：创作者 700‰ / 平台 300‰（spec CHG-02，与
--     `live.giftShareDefault = 700` 同源）。比例在**充电时快照**落库（share_rate），
--     保证将来调整比例不影响历史订单的结算金额。

-- charge_tier：充电档位配置（spec CHG-01「档位 5/10/50/自定义（元）」）。
--   金额存**分**；sort=0 的 custom 档表示允许自定义金额（服务端按 min/max 校验）。
--   档位可运营改价与上下架，故独立成表而非硬编码常量。
CREATE TABLE `charge_tier` (
  `id`         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `code`       VARCHAR(32)  NOT NULL COMMENT '档位标识（custom 为自定义档）',
  `name`       VARCHAR(64)  NOT NULL COMMENT '展示名（如「5 元」）',
  `amount`     BIGINT       NOT NULL COMMENT '档位金额（分，整数）',
  `min_amount` BIGINT       NOT NULL DEFAULT 0 COMMENT '自定义档下限（分；非自定义档为 0=不适用）',
  `max_amount` BIGINT       NOT NULL DEFAULT 0 COMMENT '自定义档上限（分；非自定义档为 0=不适用）',
  `status`     TINYINT      NOT NULL DEFAULT 0 COMMENT '0 上架 1 下架',
  `sort`       INT          NOT NULL DEFAULT 0 COMMENT '展示排序（升序）',
  `created_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY `uk_code` (`code`),
  KEY `idx_status_sort` (`status`, `sort`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='充电档位（M4-PAY-02，金额分）';

-- charge_log：充电记录（一笔支付 = 一条充电）。
--   生命周期：**下单时**随订单同事务落一行（status=0 待支付，快照档位与分成比例），
--   支付成功时在同事务内置 status=1 并记分成收益。这样「付款成功但充电记录缺失」
--   在结构上不可能发生；未支付的行不会被鸣谢名单与结算看见（一律过滤 status=1）。
--   anonymous/message 这类非资金字段只存在于本表，故不必塞进 pay_order.sku（VARCHAR(64)）。
--   uk_order_no 是「一单一充」的硬保证：同一支付订单不可能产生两条充电记录。
--   share_rate 为**下单时快照**的分成比例（‰），是按笔追溯的来源凭据。
--   video_id 允许为 0（空间页对 UP 主充电，不指定稿件），故鸣谢名单按 anchor_id 聚合。
CREATE TABLE `charge_log` (
  `id`          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `charge_no`   VARCHAR(40)  NOT NULL COMMENT '充电业务单号（对外唯一）',
  `order_no`    VARCHAR(40)  NOT NULL COMMENT '关联 pay_order.order_no（来源订单，可追溯）',
  `user_id`     BIGINT UNSIGNED NOT NULL COMMENT '充电者 user.id',
  `anchor_id`   BIGINT UNSIGNED NOT NULL COMMENT '被充电的 UP 主 user.id',
  `video_id`    BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '来源稿件（0=空间页充电）',
  `amount`      BIGINT       NOT NULL COMMENT '充电金额（分，整数）',
  `share_rate`  INT          NOT NULL DEFAULT 700 COMMENT '分成比例快照（‰，创作者所得）',
  `anonymous`   TINYINT      NOT NULL DEFAULT 0 COMMENT '1=匿名（鸣谢名单不展示身份）',
  `message`     VARCHAR(200) NOT NULL DEFAULT '' COMMENT '充电留言',
  `status`      TINYINT      NOT NULL DEFAULT 0 COMMENT '0 待支付 1 已支付（只有 1 计入鸣谢与结算）',
  `paid_at`     DATETIME     NULL COMMENT '支付成功时间（跨月归属依据，见 plan §2H.5）',
  `created_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY `uk_order_no` (`order_no`),
  UNIQUE KEY `uk_charge_no` (`charge_no`),
  KEY `idx_anchor_paid` (`anchor_id`, `status`, `paid_at`),
  KEY `idx_user` (`user_id`, `id`),
  KEY `idx_video` (`video_id`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='充电记录（M4-PAY-02）';

-- creator_income：创作者**分成收益台账**（平台对创作者的现金负债）。
--   uk_source 承载幂等：source_type=1（充电）时 source_id = charge_log.id，
--   同一笔充电重复入账第二次插入撞唯一键、事务回滚、收益不变。
--   settle_status：0 待结算 / 1 已入结算单（月度批处理按 charge_log.paid_at 归属自然月）。
--   资金不进 cash_account（那是**消费者现金钱包**的口径），本表即「待结算余额」的真源。
CREATE TABLE `creator_income` (
  `id`            BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `user_id`       BIGINT UNSIGNED NOT NULL COMMENT '创作者 user.id（收款方）',
  `source_type`   TINYINT      NOT NULL COMMENT '1 充电分成（预留：2 直播礼物分成）',
  `source_id`     BIGINT UNSIGNED NOT NULL COMMENT '来源记录 id（充电为 charge_log.id）',
  `order_no`      VARCHAR(40)  NOT NULL DEFAULT '' COMMENT '来源订单号（追溯）',
  `video_id`      BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '来源稿件（按稿聚合用）',
  `amount`        BIGINT       NOT NULL COMMENT '分成金额（分，创作者所得）',
  `platform_amount` BIGINT     NOT NULL DEFAULT 0 COMMENT '平台所得（分）',
  `share_rate`    INT          NOT NULL DEFAULT 700 COMMENT '分成比例快照（‰）',
  `settle_status` TINYINT      NOT NULL DEFAULT 0 COMMENT '0 待结算 1 已入结算单',
  `settle_month`  CHAR(7)      NOT NULL DEFAULT '' COMMENT '结算月（YYYY-MM，入单后回填）',
  `settlement_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '关联 creator_charge_settlement.id',
  `paid_at`       DATETIME     NULL COMMENT '来源订单支付时间（跨月归属按此）',
  `created_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY `uk_source` (`source_type`, `source_id`),
  KEY `idx_user_settle` (`user_id`, `settle_status`, `id`),
  KEY `idx_settle_month` (`settle_month`, `settle_status`),
  KEY `idx_order` (`order_no`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='创作者分成收益台账（M4-PAY-02，现金负债）';

-- creator_balance：创作者**账户余额**（权威余额 = 现金负债）。
--   balance    可用余额（分）——可提现部分
--   frozen     冻结金额（分）——已发起提现、尚未打款的部分
--   强一致不变量：balance >= 0、frozen >= 0；提现一律「事务 + SELECT ... FOR UPDATE 行锁」
--   （与 LIV-04 SendGiftTx 同写法），故并发提现不可能超提。
--   对账口径：balance + frozen = Σ creator_income.amount − Σ 已打款 withdraw.amount
CREATE TABLE `creator_balance` (
  `id`         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `user_id`    BIGINT UNSIGNED NOT NULL COMMENT '创作者 user.id',
  `balance`    BIGINT       NOT NULL DEFAULT 0 COMMENT '可用余额（分，≥0）',
  `frozen`     BIGINT       NOT NULL DEFAULT 0 COMMENT '冻结金额（分，≥0；提现审核中）',
  `created_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY `uk_user` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='创作者账户余额（M4-PAY-02，现金负债）';

-- creator_balance_log：创作者余额流水（审计 + 余额守恒取证）。
CREATE TABLE `creator_balance_log` (
  `id`            BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `user_id`       BIGINT UNSIGNED NOT NULL,
  `delta`         BIGINT       NOT NULL COMMENT '可用余额变动（分）',
  `frozen_delta`  BIGINT       NOT NULL DEFAULT 0 COMMENT '冻结金额变动（分）',
  `balance_after` BIGINT       NOT NULL COMMENT '变动后可用余额（分）',
  `frozen_after`  BIGINT       NOT NULL DEFAULT 0 COMMENT '变动后冻结金额（分）',
  `biz_type`      TINYINT      NOT NULL COMMENT '1 收益入账 2 提现冻结 3 提现驳回解冻 4 打款扣减 5 打款失败解冻 6 月结入账',
  `biz_no`        VARCHAR(80)  NOT NULL COMMENT '业务单号（幂等键）',
  `remark`        VARCHAR(128) NOT NULL DEFAULT '',
  `created_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY `uk_biz_no` (`biz_no`),
  KEY `idx_user` (`user_id`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='创作者余额流水（M4-PAY-02）';

-- withdraw_order：提现申请单（状态机 0 申请 → 1 审核中 → 2 已通过 → 3 已拒绝 / 4 已打款 / 5 打款失败）。
--   uk_biz_no 承载「打款幂等」：同一提现单的打款 biz_no 固定为 withdraw:{withdraw_no}，
--   重复打款第二次插入撞唯一键 → 事务回滚 → 不会二次出款（验收：重复打款被拦）。
--   通道字段为**换入点**：真实出款需支付资质，当前 mock（见 plan §2H.7）。
CREATE TABLE `withdraw_order` (
  `id`           BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `withdraw_no`  VARCHAR(40)  NOT NULL COMMENT '提现单号（对外唯一）',
  `user_id`      BIGINT UNSIGNED NOT NULL COMMENT '创作者 user.id',
  `amount`       BIGINT       NOT NULL COMMENT '提现金额（分）',
  `fee`          BIGINT       NOT NULL DEFAULT 0 COMMENT '手续费（分；本卡为 0，预留）',
  `channel`      VARCHAR(16)  NOT NULL DEFAULT 'mock' COMMENT '出款通道（mock；真实通道为换入点）',
  `account`      VARCHAR(128) NOT NULL DEFAULT '' COMMENT '收款账户（脱敏后展示）',
  `account_name` VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '收款人姓名',
  `status`       TINYINT      NOT NULL DEFAULT 0 COMMENT '0 申请 1 审核中 2 已通过 3 已拒绝 4 已打款 5 打款失败',
  `audit_by`     BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '审核人 admin_user.id',
  `audit_at`     DATETIME     NULL COMMENT '审核时间',
  `audit_note`   VARCHAR(200) NOT NULL DEFAULT '' COMMENT '审核备注/驳回原因',
  `paid_at`      DATETIME     NULL COMMENT '打款时间',
  `channel_trade_no` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '通道流水号（mock 生成）',
  `fail_reason`  VARCHAR(200) NOT NULL DEFAULT '' COMMENT '打款失败原因',
  `created_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY `uk_withdraw_no` (`withdraw_no`),
  KEY `idx_user` (`user_id`, `id`),
  KEY `idx_status` (`status`, `id`),
  KEY `idx_created` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='提现申请单（M4-PAY-02）';

-- 提现打款幂等闸 ②：同一条提现单只允许一条打款流水。
--   （闸 ① 是 withdraw_order 的条件 UPDATE + RowsAffected；与 PAY-01 回调动幂等的双闸同构）
CREATE TABLE `withdraw_payout_log` (
  `id`               BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `withdraw_no`      VARCHAR(40)  NOT NULL,
  `user_id`          BIGINT UNSIGNED NOT NULL,
  `amount`           BIGINT       NOT NULL COMMENT '打款金额（分）',
  `channel`          VARCHAR(16)  NOT NULL DEFAULT 'mock',
  `channel_trade_no` VARCHAR(64)  NOT NULL DEFAULT '',
  `result`           TINYINT      NOT NULL DEFAULT 0 COMMENT '0 成功 1 失败',
  `remark`           VARCHAR(200) NOT NULL DEFAULT '',
  `created_at`       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY `uk_withdraw_no` (`withdraw_no`),
  KEY `idx_user` (`user_id`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='提现打款流水（M4-PAY-02，打款幂等）';

-- creator_settlement 扩展（**复用** plan §23 约定的结算表，而非另起一套）：
--   新增 source 区分「创作激励」（既有，source=0→默认 incentive）与「充电分成」（本卡，source=charge），
--   充电分成按（月 × 创作者 × 稿件）聚合写入 settle_date = 该自然月 1 号。
--
-- **唯一键必须补上 user_id（本卡实测发现的缺陷）**：原 uk_date_video(settle_date, video_id)
--   在「空间页充电」这类 video_id=0 的行上会让**不同创作者互相覆盖**——第二个创作者的
--   ON DUPLICATE KEY UPDATE 命中第一个创作者的行，金额被改写而 user_id 仍是前者的，
--   于是结算数据静默错账（实测：92710003 的行被 92710004 的 21000 分覆盖）。
--   改为 (settle_date, user_id, video_id)：对创作激励语义完全不变（一个稿件只属于一个
--   创作者，(date,video) 已隐含 user_id），对充电分成则正确区分创作者。
ALTER TABLE `creator_settlement`
  DROP KEY `uk_date_video`,
  ADD COLUMN `source` VARCHAR(16) NOT NULL DEFAULT 'incentive' COMMENT '来源：incentive 创作激励 / charge 充电分成' AFTER `video_id`,
  ADD COLUMN `charge_count` INT NOT NULL DEFAULT 0 COMMENT '充电笔数（仅 source=charge）' AFTER `amount`,
  ADD COLUMN `settlement_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '关联 creator_charge_settlement.id（仅 source=charge）' AFTER `charge_count`,
  ADD UNIQUE KEY `uk_source_date_user_video` (`settle_date`, `user_id`, `video_id`, `source`),
  ADD KEY `idx_source_user_date` (`source`, `user_id`, `settle_date`);

-- creator_charge_settlement：**充电分成结算单**（月结产物，spec CHG-02「月度结算入创作者收益」）。
--   uk_user_month 保证一个有创作者一个自然月一张结算单（批处理可重放）。
--   金额口径：amount = Σ 该月该创作者各笔分成，且为**生成时刻的快照**（settlement 行可对账回溯）。
CREATE TABLE `creator_charge_settlement` (
  `id`            BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `settle_month`  CHAR(7)      NOT NULL COMMENT '结算月（YYYY-MM，按支付时间归属）',
  `user_id`       BIGINT UNSIGNED NOT NULL COMMENT '创作者',
  `charge_count`  INT          NOT NULL DEFAULT 0 COMMENT '纳入结算的充电笔数',
  `amount`        BIGINT       NOT NULL DEFAULT 0 COMMENT '创作者分成合计（分）',
  `platform_amount` BIGINT     NOT NULL DEFAULT 0 COMMENT '平台分成合计（分）',
  `status`        TINYINT      NOT NULL DEFAULT 0 COMMENT '0 已生成 1 已打款（提现维度，预留）',
  `created_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY `uk_user_month` (`user_id`, `settle_month`),
  KEY `idx_month` (`settle_month`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='充电分成月度结算单（M4-PAY-02）';

-- 充电档位出厂初值（spec CHG-01：5/10/50/自定义元）。
INSERT INTO `charge_tier` (`code`, `name`, `amount`, `min_amount`, `max_amount`, `status`, `sort`) VALUES
  ('tier_5',   '5 元',  500,   0, 0, 0, 1),
  ('tier_10',  '10 元', 1000,  0, 0, 0, 2),
  ('tier_30',  '30 元', 3000,  0, 0, 0, 3),
  ('tier_50',  '50 元', 5000,  0, 0, 0, 4),
  ('custom',   '自定义', 0,  100, 50000, 0, 5);
