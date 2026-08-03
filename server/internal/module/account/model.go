// Package account 账号域：注册登录、会话、用户资料。
package account

import "time"

// 认证方式
const (
	IdentityPhone  = 1
	IdentityEmail  = 2
	IdentityWeChat = 3
)

// 用户状态
const (
	UserStatusNormal = 0
	UserStatusMuted  = 1
	UserStatusBanned = 2
	UserStatusClosed = 3
)

// User 对应 user 表。
type User struct {
	ID          int64 `gorm:"primaryKey"`
	Nickname    string
	Avatar      string
	Signature   string
	Gender      int8
	Level       int8
	Exp         int
	Coin        int
	Status      int8
	MutedUntil  *time.Time // 禁言到期（status=1 时有效）
	BannedUntil *time.Time // 封禁到期（status=2 时有效，nil=永久）
	YouthMode   int8       // 青少年模式 0关 1开（M2-AUD-04）
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (User) TableName() string { return "user" }

// UserAuth 对应 user_auth 表。
type UserAuth struct {
	ID           int64 `gorm:"primaryKey"`
	UserID       int64
	IdentityType int8
	Identifier   string
	Credential   string
	CreatedAt    time.Time
}

func (UserAuth) TableName() string { return "user_auth" }

// CoinLog 对应 coin_log 表（硬币流水）。
type CoinLog struct {
	ID        int64 `gorm:"primaryKey;autoIncrement"`
	UserID    int64
	Delta     int
	Reason    string
	CreatedAt time.Time
}

func (CoinLog) TableName() string { return "coin_log" }

// Profile 对外用户信息（ID 用字符串避免 JS 精度丢失）。
type Profile struct {
	ID          string     `json:"id"`
	Nickname    string     `json:"nickname"`
	Avatar      string     `json:"avatar"`
	Signature   string     `json:"signature"`
	Gender      int8       `json:"gender"`
	Level       int8       `json:"level"`
	Coin        int        `json:"coin"`
	Status      int8       `json:"status"`                 // 0 正常 1 禁言 2 封禁 3 注销
	MutedUntil  *time.Time `json:"muted_until,omitempty"`  // 禁言到期
	BannedUntil *time.Time `json:"banned_until,omitempty"` // 封禁到期（nil=永久）
}

// TokenPair 登录/刷新返回的令牌对。
type TokenPair struct {
	AccessToken  string  `json:"access_token"`
	RefreshToken string  `json:"refresh_token"`
	ExpiresIn    int     `json:"expires_in"` // access token 有效期（秒）
	User         Profile `json:"user"`
}
