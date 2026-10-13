package live

import "time"

// 金瓜子流水业务类型
const (
	MelonBizRecharge  = 1 // 充值（dev 直充 / PAY-01 支付回调）
	MelonBizGiftSpend = 2 // 送礼扣减
	MelonBizRefund    = 3 // 退款（PAY-01 预留）
)

// 礼物特效类型 / 分级
const (
	GiftEffectDanmaku = 1 // 弹幕区连击样式
	GiftEffectScreen  = 2 // 全屏特效
)

const (
	GiftLevelNormal   = 1 // 普通礼物
	GiftLevelAdvanced = 2 // 高级礼物
)

// MelonSeedAccount 对应 melon_seed_account 表（金瓜子余额；与硬币严格分离）。
type MelonSeedAccount struct {
	ID        int64     `gorm:"primaryKey;autoIncrement"`
	UserID    int64     `gorm:"uniqueIndex:uk_user"`
	Balance   int64     `json:"balance"`
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
}

func (MelonSeedAccount) TableName() string { return "melon_seed_account" }

// MelonSeedLog 对应 melon_seed_log 表（流水审计 + 充值幂等键）。
type MelonSeedLog struct {
	ID           int64     `gorm:"primaryKey;autoIncrement"`
	UserID       int64     `json:"user_id"`
	Delta        int64     `json:"delta"`
	BalanceAfter int64     `json:"balance_after"`
	BizType      int8      `json:"biz_type"`
	BizNo        string    `gorm:"uniqueIndex:uk_biz_no" json:"biz_no"`
	Remark       string    `json:"remark"`
	CreatedAt    time.Time `json:"-"`
}

func (MelonSeedLog) TableName() string { return "melon_seed_log" }

// Gift 对应 gift 表（礼物配置）。
type Gift struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Name      string    `json:"name"`
	Icon      string    `json:"icon"`
	Price     int64     `json:"price"`
	Effect    int8      `json:"effect"` // 1 弹幕区连击 2 全屏特效
	Level     int8      `json:"level"`  // 1 普通 2 高级
	Status    int8      `json:"status"` // 0 上架 1 下架
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
}

func (Gift) TableName() string { return "gift" }

// GiftLog 对应 gift_log 表（赠送流水；单价/分成比例下单时快照）。
type GiftLog struct {
	ID         int64     `gorm:"primaryKey" json:"id"` // 雪花 ID
	RoomID     int64     `json:"room_id"`
	SenderID   int64     `json:"sender_id"`
	AnchorID   int64     `json:"anchor_id"`
	GiftID     int64     `json:"gift_id"`
	Count      int64     `json:"count"`
	UnitPrice  int64     `json:"unit_price"`
	TotalPrice int64     `json:"total_price"`
	ShareRate  int64     `json:"share_rate"` // ‰
	CreatedAt  time.Time `json:"-"`
}

func (GiftLog) TableName() string { return "gift_log" }

// GiftShareLog 对应 gift_share_log 表（主播分成流水，月度结算输入 PAY-02）。
type GiftShareLog struct {
	ID             int64     `gorm:"primaryKey;autoIncrement"`
	GiftLogID      int64     `gorm:"uniqueIndex:uk_gift_log"`
	RoomID         int64     `json:"room_id"`
	AnchorID       int64     `json:"anchor_id"`
	Amount         int64     `json:"amount"`
	PlatformAmount int64     `json:"platform_amount"`
	CreatedAt      time.Time `json:"-"`
}

func (GiftShareLog) TableName() string { return "gift_share_log" }

// ---- 对外 DTO ----

// GiftInfo 礼物列表项（下架不输出）。
type GiftInfo struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Icon   string `json:"icon"`
	Price  int64  `json:"price"`
	Effect int8   `json:"effect"`
	Level  int8   `json:"level"`
}

// MelonBalance 金瓜子余额。
type MelonBalance struct {
	Balance int64 `json:"balance"`
}

// SendGiftResult 赠送结果（余额 + 本次礼物帧信息回显）。
type SendGiftResult struct {
	Balance int64      `json:"balance"`
	GiftLog *GiftFrame `json:"gift_log"`
}

// GiftFrame WS 礼物特效帧负载（type=gift 下行帧的 data）。
type GiftFrame struct {
	LogID    int64  `json:"log_id,string"`
	SenderID string `json:"sender_id"` // 字符串避免 JS 精度丢失
	Nickname string `json:"nickname"`
	GiftID   int64  `json:"gift_id"`
	GiftName string `json:"gift_name"`
	Icon     string `json:"icon,omitempty"`
	Count    int64  `json:"count"`
	Effect   int8   `json:"effect"` // 1 弹幕区连击 2 全屏特效
	Level    int8   `json:"level"`  // 1 普通 2 高级
}

// RechargeResult dev 直充结果。
type RechargeResult struct {
	UserID  int64 `json:"user_id"`
	Balance int64 `json:"balance"`
	Delta   int64 `json:"delta"`
}
