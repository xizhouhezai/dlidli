// Package vip 大会员（M4-VIP-01）：套餐、开通/顺延、权益判定、连续包月与月度赠币。
//
// 四条硬约束（monetization spec §2.1/§2.2/§2.7 / plan §2F）：
//  1. 会员判定一律 `expire_at > NOW()`，**服务端判定**，前端不可绕过；不缓存（plan §2F.2）。
//  2. 续期是**顺延**而非覆盖：`expire_at = max(now(), expire_at) + 时长`；过期后从 now() 起算。
//  3. 同一订单只生效一次：`vip_order.uk_order_no` 是第二道闸，权益在 pay 的入账事务内发放。
//  4. 赠的是**硬币**（user.coin / coin_log），不是金瓜子（melon_seed_*）；
//     幂等键为 (user_id, 周期键 YYYY-MM) 唯一索引。
package vip

import "time"

// 会员档位（V3 仅一档；字段保留以便将来多档而不改表）。
const LevelVip = 1

// 套餐/签约状态。
const (
	PlanOff int8 = 0 // 下架
	PlanOn  int8 = 1 // 上架

	SubCancelled int8 = 0 // 已解约
	SubActive    int8 = 1 // 签约中
)

// 订单 → 权益生效状态。
const (
	ApplyPending int8 = 0 // 未生效
	ApplyDone    int8 = 1 // 已生效（权益已发放）
)

// 赠币来源。
const (
	GrantOnApply  int8 = 1 // 开通/续期即时发放
	GrantByWorker int8 = 2 // 定时补发（月度扫描）
	GrantByAdmin  int8 = 3 // 管理端补发
)

// CoinReasonVIP VIPGrantReason 硬币流水的 reason 文案（coin_log.reason）。
// 对齐 account.coinReasonNames 的登记口径（未登记会原样透出，故此处用可读中文）。
const CoinReasonVIP = "vip_monthly"

// VipPlan 会员套餐（管理端可改价与上下架）。
type VipPlan struct {
	ID              int64 `gorm:"primaryKey;autoIncrement"`
	Code            string
	Name            string
	Months          int
	Price           int64 // 分（整数，禁用浮点）
	OriginalPrice   int64 // 分
	FirstMonthPrice int64 // 分（连续包月首月优惠；0=无优惠）
	AutoRenew       int8
	Sort            int
	Status          int8
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (VipPlan) TableName() string { return "vip_plan" }

// Membership 会员状态（user_id 唯一）。
//
// **判定只看 ExpireAt**：Level 不参与判定，避免出现「level=1 但已过期」被判为会员。
type Membership struct {
	ID          int64 `gorm:"primaryKey;autoIncrement"`
	UserID      int64
	Level       int8
	ExpireAt    time.Time
	AutoRenew   int8
	StartedAt   time.Time
	TotalMonths int
	LastOrderNo string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (Membership) TableName() string { return "membership" }

// VipOrder 订单 → 权益生效台账（uk_order_no 承载「同一订单只生效一次」）。
type VipOrder struct {
	ID           int64 `gorm:"primaryKey;autoIncrement"`
	OrderNo      string
	UserID       int64
	PlanCode     string
	Months       int
	ApplyStatus  int8
	ExpireBefore *time.Time
	ExpireAfter  *time.Time
	AppliedAt    *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (VipOrder) TableName() string { return "vip_order" }

// VipSubscription 连续包月签约（user_id 唯一；NextChargeAt 是续期扫描驱动字段）。
type VipSubscription struct {
	ID           int64 `gorm:"primaryKey;autoIncrement"`
	UserID       int64
	PlanCode     string
	Channel      string
	Status       int8
	SignAt       time.Time
	CancelAt     *time.Time
	NextChargeAt time.Time
	LastChargeAt *time.Time
	FailCount    int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (VipSubscription) TableName() string { return "vip_subscription" }

// VipCoinGrant 月度赠币台账（uk_user_period 是幂等唯一闸）。
type VipCoinGrant struct {
	ID        int64 `gorm:"primaryKey;autoIncrement"`
	UserID    int64
	PeriodKey string // 'YYYY-MM'
	Amount    int
	CoinLogID int64
	Source    int8
	CreatedAt time.Time
}

func (VipCoinGrant) TableName() string { return "vip_coin_grant" }

// ---- 对外视图 ----

// PlanView 套餐对外视图（金额整数分）。
type PlanView struct {
	Code            string `json:"code"`
	Name            string `json:"name"`
	Months          int    `json:"months"`
	Price           int64  `json:"price"`             // 分
	OriginalPrice   int64  `json:"original_price"`    // 分（0=不展示）
	FirstMonthPrice int64  `json:"first_month_price"` // 分（0=无优惠）
	AutoRenew       bool   `json:"auto_renew"`
	Sort            int    `json:"sort"`
}

func toPlanView(p *VipPlan) *PlanView {
	return &PlanView{
		Code: p.Code, Name: p.Name, Months: p.Months, Price: p.Price,
		OriginalPrice: p.OriginalPrice, FirstMonthPrice: p.FirstMonthPrice,
		AutoRenew: p.AutoRenew == 1, Sort: p.Sort,
	}
}

// Rights 权益位（spec §2.1 权益矩阵的机读形态）。
//
// **清晰度位归 VIP-02 消费**（本卡不实现鉴权，只下发权益位，见 plan §2F.7）：
// max_resolution 给出该用户被允许的最高清晰度档位，VIP-02 在下发播放地址时读它。
type Rights struct {
	MaxResolution string `json:"max_resolution"` // 1080p / 4k（含 HDR）
	HDR           bool   `json:"hdr"`
	Exclusive     bool   `json:"exclusive"`      // 会员抢先看/专属稿件
	Badge         bool   `json:"badge"`          // 身份标识：会员徽章
	NickColor     string `json:"nickname_color"` // 身份标识：昵称颜色（粉色）
	CoinPerMonth  int    `json:"coin_per_month"` // 每月赠币枚数
	ExpBoost      bool   `json:"exp_boost"`      // 经验加成
	DressUp       bool   `json:"dress_up"`       // 装扮（未覆盖，恒 false，见 tasks.md 未覆盖项）
}

// freeRights 免费用户权益位（与会员位的差异即验收判据）。
func freeRights() *Rights {
	return &Rights{
		MaxResolution: "1080p",
		HDR:           false,
		Exclusive:     false,
		Badge:         false,
		NickColor:     "",
		CoinPerMonth:  0,
		ExpBoost:      false,
		DressUp:       false,
	}
}

// vipRights 会员权益位（spec §2.1「大会员」列）。
func vipRights(coinPerMonth int) *Rights {
	return &Rights{
		MaxResolution: "4k",
		HDR:           true,
		Exclusive:     true,
		Badge:         true,
		NickColor:     "pink",
		CoinPerMonth:  coinPerMonth,
		ExpBoost:      true,
		// 装扮/皮肤无前端设计资源，未覆盖（恒 false，不谎报已实现）。
		DressUp: false,
	}
}

// StatusView `GET /vip/status` 响应：是否会员、到期时间、权益位。
type StatusView struct {
	IsVip        bool    `json:"is_vip"`
	Level        int8    `json:"level"`
	ExpireAt     string  `json:"expire_at,omitempty"`
	AutoRenew    bool    `json:"auto_renew"`
	ExpireDays   int     `json:"expire_days"`   // 剩余天数（非会员为 0）
	ExpiringSoon bool    `json:"expiring_soon"` // 到期前 N 天（VIP-04 提醒位）
	Rights       *Rights `json:"rights"`
}

// SubscribeView 签约视图。
type SubscribeView struct {
	PlanCode     string `json:"plan_code"`
	PlanName     string `json:"plan_name"`
	Status       int8   `json:"status"`
	SignAt       string `json:"sign_at"`
	NextChargeAt string `json:"next_charge_at,omitempty"`
	LastChargeAt string `json:"last_charge_at,omitempty"`
	FailCount    int    `json:"fail_count"`
	Price        int64  `json:"price"` // 下次扣款金额（分）
}

// BadgeView 会员标识（供评论/弹幕等读模型拼装的最小信息）。
type BadgeView struct {
	IsVip     bool   `json:"is_vip"`
	NickColor string `json:"nickname_color,omitempty"`
	Badge     string `json:"badge,omitempty"` // 徽章标识（前端据此渲染）
}

// Badge 是跨模块只读的会员标识查询契约（interaction/live 以接口注入，不反向依赖本包）。
type Badge interface {
	// VipFlags 批量取用户会员标识（一次查询，避免读模型 N+1）。
	VipFlags(userIDs []int64) (map[int64]BadgeView, error)
}

// 时间格式统一（与 pay 模块口径一致）。
const timeLayout = "2006-01-02 15:04:05"

func fmtTime(t time.Time) string { return t.Format(timeLayout) }

// periodKey 赠币周期键（YYYY-MM，本地时区口径：月度发放按运营日历的自然月）。
func periodKey(t time.Time) string { return t.Format("2006-01") }
