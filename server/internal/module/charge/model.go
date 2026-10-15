// Package charge 充电打赏、分成结算与提现（M4-PAY-02）。
//
// 覆盖规格：monetization spec CHG-01/CHG-02/CHG-03（§2.3）+ creator spec CRT-31/32；
// 技术决策见 monetization plan §2H。
//
// 三条硬约束（延续 PAY-01 §6.1 与 LIV-04 口径，本包逐条落地）：
//  1. **金额一律整数「分」**：DB BIGINT / Go int64，全链路禁用浮点（含聚合与比较）。
//  2. **现金 ≠ 虚拟币**：充电的观众侧资金复用 PAY-01 的 `pay_order`（现金订单），
//     创作者侧收益是**平台对创作者的现金负债**（`creator_income` / `creator_balance`），
//     与硬币（`coin_log`）/金瓜子（`melon_seed_*`）无任何写入或读取关系。
//  3. **分成口径与 M4-LIV-04 一致**：创作者 700‰ / 平台 300‰（spec CHG-02），
//     且比例在充电时**快照**落库（`share_rate`），保证改比例不影响历史订单。
//
// 资金强一致：一切余额变更走「事务 + SELECT ... FOR UPDATE 行锁」，
// 与 LIV-04 `SendGiftTx` 同写法（见 repo.go）。
package charge

import "time"

// ---- 分成比例（spec CHG-02，与 live.giftShareDefault = 700 同源）----

// ShareRateCreator 创作者分成比例（‰）。**必须与 M4-LIV-04 礼物分成口径一致**。
const ShareRateCreator int64 = 700

// ShareRatePlatform 平台分成比例（‰）= 1000 − 创作者。
const ShareRatePlatform int64 = 1000 - ShareRateCreator

// ---- 档位 ----

// 档位状态。
const (
	TierOn  int8 = 0 // 上架
	TierOff int8 = 1 // 下架
)

// TierCustomCode 自定义档位的 code（该档位的 amount 字段无意义，用 min/max 约束）。
const TierCustomCode = "custom"

// ChargeTier 对应 charge_tier 表（充电档位，金额单位：分）。
type ChargeTier struct {
	ID        int64 `gorm:"primaryKey;autoIncrement"`
	Code      string
	Name      string
	Amount    int64 // 分（整数）
	MinAmount int64 // 自定义档下限（分）
	MaxAmount int64 // 自定义档上限（分）
	Status    int8
	Sort      int
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (ChargeTier) TableName() string { return "charge_tier" }

// TierView 档位对外视图。
type TierView struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	Amount    int64  `json:"amount"` // 分
	MinAmount int64  `json:"min_amount"`
	MaxAmount int64  `json:"max_amount"`
	Custom    bool   `json:"custom"`
}

// ---- 充电记录 ----

// 充电记录状态（下单即落行，支付成功才计入鸣谢与结算）。
const (
	ChargeUnpaid int8 = 0 // 待支付
	ChargePaid   int8 = 1 // 已支付（唯一计入鸣谢与结算的状态）
)

// ChargeLog 对应 charge_log 表（一笔支付 = 一条充电）。
//
// `OrderNo` 上的唯一键是「一单一充」的硬保证：同一支付订单不可能产生两条充电记录，
// 这是回调重放/并发下的资金安全底座（与 PAY-01 的 `cash_log.uk_biz_no` 同思路）。
type ChargeLog struct {
	ID        int64  `gorm:"primaryKey;autoIncrement"`
	ChargeNo  string `gorm:"uniqueIndex:uk_charge_no"`
	OrderNo   string `gorm:"uniqueIndex:uk_order_no"`
	UserID    int64  // 充电者
	AnchorID  int64  // 被充电的 UP 主
	VideoID   int64  // 来源稿件（0=空间页充电）
	Amount    int64  // 分
	ShareRate int    // ‰ 快照
	Anonymous int8   // 1=匿名
	Message   string
	Status    int8       // 0 待支付 1 已支付
	PaidAt    *time.Time // 支付成功时间（跨月归属依据）
	CreatedAt time.Time
}

func (ChargeLog) TableName() string { return "charge_log" }

// ChargeView 充电记录对外视图（spec CHG-03 鸣谢名单的数据来源）。
type ChargeView struct {
	ChargeNo  string `json:"charge_no"`
	OrderNo   string `json:"order_no"`
	UserID    string `json:"user_id"`
	AnchorID  string `json:"anchor_id"`
	VideoID   string `json:"video_id,omitempty"`
	Amount    int64  `json:"amount"` // 分
	Anonymous bool   `json:"anonymous"`
	Message   string `json:"message"`
	// Nickname 匿名时为空串（不泄露身份，spec CHG-03「可匿名」）。
	Nickname  string `json:"nickname"`
	PaidAt    string `json:"paid_at,omitempty"`
	CreatedAt string `json:"created_at"`
}

// ThanksItem 鸣谢名单条目：匿名者只暴露金额与时间，不暴露身份。
type ThanksItem struct {
	Nickname  string `json:"nickname"`
	Avatar    string `json:"avatar,omitempty"`
	Amount    int64  `json:"amount"`
	Message   string `json:"message,omitempty"`
	Anonymous bool   `json:"anonymous"`
	CreatedAt string `json:"created_at"`
}

// ---- 创作者收益 ----

// 收益来源类型（预留：2 直播礼物分成）。
const (
	IncomeSourceCharge int8 = 1
)

// 结算状态。
const (
	IncomeUnsettled int8 = 0 // 待结算
	IncomeSettled   int8 = 1 // 已入结算单
)

// CreatorIncome 对应 creator_income 表（平台对创作者的现金负债明细）。
type CreatorIncome struct {
	ID             int64 `gorm:"primaryKey;autoIncrement"`
	UserID         int64
	SourceType     int8
	SourceID       int64 // 充电时为 charge_log.ID（uk_source 幂等键）
	OrderNo        string
	VideoID        int64
	Amount         int64 // 分（创作者所得）
	PlatformAmount int64 // 分（平台所得）
	ShareRate      int
	SettleStatus   int8
	SettleMonth    string
	SettlementID   int64
	PaidAt         *time.Time
	CreatedAt      time.Time
}

func (CreatorIncome) TableName() string { return "creator_income" }

// CreatorBalance 对应 creator_balance 表（**权威余额** = 平台对创作者的现金负债）。
//
// 不变量：Balance ≥ 0 且 Frozen ≥ 0；二者之和 = Σ收益 − Σ已打款。
type CreatorBalance struct {
	ID        int64 `gorm:"primaryKey;autoIncrement"`
	UserID    int64 `gorm:"uniqueIndex:uk_user"`
	Balance   int64 // 可用（分）
	Frozen    int64 // 冻结（分，提现审核中）
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (CreatorBalance) TableName() string { return "creator_balance" }

// 余额流水业务类型。
const (
	BalBizIncome         int8 = 1 // 收益入账
	BalBizWithdrawFreeze int8 = 2 // 提现冻结
	BalBizWithdrawReject int8 = 3 // 提现驳回解冻
	BalBizPayout         int8 = 4 // 打款扣减
	BalBizPayoutFail     int8 = 5 // 打款失败解冻
	BalBizSettle         int8 = 6 // 月结入账（预留）
)

// CreatorBalanceLog 对应 creator_balance_log 表（审计 + 余额守恒取证）。
type CreatorBalanceLog struct {
	ID           int64 `gorm:"primaryKey;autoIncrement"`
	UserID       int64
	Delta        int64
	FrozenDelta  int64
	BalanceAfter int64
	FrozenAfter  int64
	BizType      int8
	BizNo        string `gorm:"uniqueIndex:uk_biz_no"`
	Remark       string
	CreatedAt    time.Time
}

func (CreatorBalanceLog) TableName() string { return "creator_balance_log" }

// ---- 提现 ----

// 提现状态机（条件 UPDATE + RowsAffected 推进，plan §2H.7）：
//
//	0 审核中 ──→ 2 已通过 ──→ 4 已打款
//	   │            │
//	   ↓            ↓
//	3 已拒绝      5 打款失败（解冻返还，可重新申请）
//
// 说明：状态 1 未使用。「申请」与「审核中」在业务上不可区分（提交即待审），
// 单独造一个「认领」态会引入一个无产品需求、且无人能离开的状态，故合并为 0。
const (
	WithdrawAuditing int8 = 0 // 审核中（已提交，冻结已生效）
	WithdrawApproved int8 = 2 // 已通过（可打款）
	WithdrawRejected int8 = 3 // 已拒绝（已解冻返还）
	WithdrawPaid     int8 = 4 // 已打款（终态）
	WithdrawPayFail  int8 = 5 // 打款失败（已解冻返还）
)

// WithdrawOrder 对应 withdraw_order 表。
type WithdrawOrder struct {
	ID             int64  `gorm:"primaryKey;autoIncrement"`
	WithdrawNo     string `gorm:"uniqueIndex:uk_withdraw_no"`
	UserID         int64
	Amount         int64 // 分
	Fee            int64
	Channel        string
	Account        string
	AccountName    string
	Status         int8
	AuditBy        int64
	AuditAt        *time.Time
	AuditNote      string
	PaidAt         *time.Time
	ChannelTradeNo string
	FailReason     string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (WithdrawOrder) TableName() string { return "withdraw_order" }

// WithdrawPayoutLog 对应 withdraw_payout_log 表（打款幂等闸 ②：一人一单一条）。
type WithdrawPayoutLog struct {
	ID             int64  `gorm:"primaryKey;autoIncrement"`
	WithdrawNo     string `gorm:"uniqueIndex:uk_withdraw_no"`
	UserID         int64
	Amount         int64
	Channel        string
	ChannelTradeNo string
	Result         int8
	Remark         string
	CreatedAt      time.Time
}

func (WithdrawPayoutLog) TableName() string { return "withdraw_payout_log" }

// WithdrawView 提现单对外视图。
type WithdrawView struct {
	WithdrawNo     string `json:"withdraw_no"`
	UserID         string `json:"user_id"`
	Amount         int64  `json:"amount"`
	Fee            int64  `json:"fee"`
	Channel        string `json:"channel"`
	Account        string `json:"account"`
	AccountName    string `json:"account_name"`
	Status         int8   `json:"status"`
	StatusText     string `json:"status_text"`
	AuditNote      string `json:"audit_note,omitempty"`
	ChannelTradeNo string `json:"channel_trade_no,omitempty"`
	FailReason     string `json:"fail_reason,omitempty"`
	CreatedAt      string `json:"created_at"`
	AuditAt        string `json:"audit_at,omitempty"`
	PaidAt         string `json:"paid_at,omitempty"`
}

// CreatorBalanceView 创作者账户视图（**现金负债**口径，非虚拟币）。
type CreatorBalanceView struct {
	UserID  string `json:"user_id"`
	Balance int64  `json:"balance"` // 可用（分）
	Frozen  int64  `json:"frozen"`  // 冻结（分）
	// Unsettled 待结算收益（分）：已入账但尚未纳入月度结算单的部分，仅作报表口径。
	Unsettled int64 `json:"unsettled"`
	// Withdrawn 累计已打款（分）。
	Withdrawn int64 `json:"withdrawn"`
	// TotalIncome 累计分成收益（分）。
	TotalIncome int64 `json:"total_income"`
}

// ---- 结算 ----

// CreatorChargeSettlement 对应 creator_charge_settlement 表（月结产物）。
type CreatorChargeSettlement struct {
	ID             int64  `gorm:"primaryKey;autoIncrement"`
	SettleMonth    string // YYYY-MM
	UserID         int64
	ChargeCount    int
	Amount         int64 // 分（创作者合计）
	PlatformAmount int64 // 分（平台合计）
	Status         int8
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (CreatorChargeSettlement) TableName() string { return "creator_charge_settlement" }

// SettlementView 结算单对外视图。
type SettlementView struct {
	SettleMonth    string `json:"settle_month"`
	UserID         string `json:"user_id"`
	ChargeCount    int64  `json:"charge_count"`
	Amount         int64  `json:"amount"`
	PlatformAmount int64  `json:"platform_amount"`
	CreatedAt      string `json:"created_at"`
}

// SettlementByVideo 结算单内按稿聚合明细（写入 creator_settlement.source='charge'）。
type SettlementByVideo struct {
	VideoID        int64
	ChargeCount    int64
	Amount         int64
	PlatformAmount int64
}

// statusText / tierStatusText 状态文案（对外输出统一口径）。
func withdrawStatusText(s int8) string {
	switch s {
	case WithdrawAuditing:
		return "审核中"
	case WithdrawApproved:
		return "已通过"
	case WithdrawRejected:
		return "已拒绝"
	case WithdrawPaid:
		return "已打款"
	case WithdrawPayFail:
		return "打款失败"
	default:
		return "未知"
	}
}

func fmtTime(t time.Time) string { return t.Format("2006-01-02 15:04:05") }

func fmtTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return fmtTime(*t)
}
