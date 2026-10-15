// Package pay 支付基础设施（M4-PAY-01）：统一支付网关（适配器模式）、订单状态机、
// 回调验签与幂等、现金账户、日对账与异常订单处理台。
//
// 三条硬约束（monetization spec §6 / plan §2E）：
//  1. 金额一律整数「分」：DB BIGINT、Go int64，全链路禁用浮点。
//  2. 现金账户与虚拟币账户严格分离：本包只写 cash_account / cash_log，
//     绝不触碰 coin_log / melon_seed_account / melon_seed_log。
//  3. 状态迁移一律条件 UPDATE + RowsAffected 判定；入账与迁移同事务。
package pay

import "time"

// 订单状态机（只允许 0→1、0→2、1→3；2/3 为终态）。
const (
	StatusPending  int8 = 0 // 待支付
	StatusPaid     int8 = 1 // 已支付
	StatusClosed   int8 = 2 // 已关闭
	StatusRefunded int8 = 3 // 已退款
)

// 现金流水业务类型。
const (
	CashBizPay    int8 = 1 // 支付入账
	CashBizRefund int8 = 2 // 退款
)

// 对账异常类型。
const (
	IssueAmountMismatch int8 = 1 // 金额不符
	IssueStuck          int8 = 2 // 长期挂起
	IssueMissingLog     int8 = 3 // 流水缺失
	IssueDuplicateCb    int8 = 4 // 重复回调
)

// 对账异常处理状态。
const (
	IssuePending  int8 = 0
	IssueHandled  int8 = 1
	ReconOK       int8 = 0
	ReconHasIssue int8 = 1
)

// PayOrder 支付订单。
type PayOrder struct {
	ID             int64      `gorm:"primaryKey;autoIncrement"`
	OrderNo        string     // 对外业务单号
	UserID         int64      // 下单用户
	Amount         int64      // 金额（分）
	Channel        string     // mock / alipay / wechat
	Subject        string     // 商品描述
	Sku            string     // 商品标识（M4-VIP-01 补列；会员卡为 vip_plan.code）
	Status         int8       // 0/1/2/3
	ChannelTradeNo *string    // 渠道交易号（未支付为 NULL）
	RefundAmount   int64      // 已退款金额（分）
	ClientIP       string     // 下单来源 IP
	ExpireAt       time.Time  // 过期时间
	PaidAt         *time.Time // 支付成功时间
	ClosedAt       *time.Time
	RefundedAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (PayOrder) TableName() string { return "pay_order" }

// CashAccount 现金账户（与金瓜子 melon_seed_account 严格分离）。
type CashAccount struct {
	ID        int64 `gorm:"primaryKey;autoIncrement"`
	UserID    int64
	Balance   int64 // 分
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (CashAccount) TableName() string { return "cash_account" }

// CashLog 现金流水（uk_biz_no 承载资金幂等）。
type CashLog struct {
	ID           int64 `gorm:"primaryKey;autoIncrement"`
	UserID       int64
	Delta        int64 // 分
	BalanceAfter int64 // 分
	BizType      int8
	BizNo        string
	OrderNo      string
	Remark       string
	CreatedAt    time.Time
}

func (CashLog) TableName() string { return "cash_log" }

// PayCallbackLog 回调日志（验签失败也落库，spec §6.4）。
type PayCallbackLog struct {
	ID             int64 `gorm:"primaryKey;autoIncrement"`
	Channel        string
	RawBody        string
	Signature      string
	SignOK         int8
	OrderNo        string
	ChannelTradeNo string
	Amount         int64
	Result         string
	CreatedAt      time.Time
}

func (PayCallbackLog) TableName() string { return "pay_callback_log" }

// PayReconDaily 日对账单（uk_date_channel 唯一，重跑 UPSERT）。
//
// ReconDate 用 time.Time 而非 string：DATE 列读回 string 字段时 GORM 会给出
// `2026-10-15T00:00:00+08:00` 这种带时区的完整时间戳（实测），与「对账日期」语义不符、
// 也让前端无法直接做日期比较。用 time.Time 承接、在对外视图里统一格式化为 YYYY-MM-DD。
type PayReconDaily struct {
	ID            int64 `gorm:"primaryKey;autoIncrement"`
	ReconDate     time.Time `gorm:"column:recon_date"`
	Channel       string
	OrderCount    int64
	PaidCount     int64
	PaidAmount    int64
	RefundCount   int64
	RefundAmount  int64
	CashLogAmount int64
	DiffCount     int64
	Status        int8
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (PayReconDaily) TableName() string { return "pay_recon_daily" }

// PayReconIssue 异常订单（人工处理台数据源）。
type PayReconIssue struct {
	ID         int64 `gorm:"primaryKey;autoIncrement"`
	ReconDate  time.Time `gorm:"column:recon_date"`
	Channel    string
	OrderNo    string
	IssueType  int8
	Detail     string
	Status     int8
	HandledBy  int64
	HandledAt  *time.Time
	HandleNote string
	CreatedAt  time.Time
}

func (PayReconIssue) TableName() string { return "pay_recon_issue" }

// dateOnly 统一日期输出格式（对账日期一律 YYYY-MM-DD，与入参口径一致）。
func dateOnly(t time.Time) string { return t.Format("2006-01-02") }

// issueTypeText 异常类型的中文说明（对账接口输出用）。
func issueTypeText(t int8) string {
	switch t {
	case IssueAmountMismatch:
		return "金额不符"
	case IssueStuck:
		return "长期挂起"
	case IssueMissingLog:
		return "流水缺失"
	case IssueDuplicateCb:
		return "重复回调"
	default:
		return "未知"
	}
}

// statusText 订单状态的中文说明。
func statusText(s int8) string {
	switch s {
	case StatusPending:
		return "待支付"
	case StatusPaid:
		return "已支付"
	case StatusClosed:
		return "已关闭"
	case StatusRefunded:
		return "已退款"
	default:
		return "未知"
	}
}
