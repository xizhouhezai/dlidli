// Package admin 后台管理域：管理员登录、稿件审核工作台、操作审计。
// MVP 内嵌于 api 服务（/api/v1/admin）；规模化后拆独立 cmd/admin 服务。
package admin

import "time"

// AdminUser 对应 admin_user 表。
type AdminUser struct {
	ID          int64 `gorm:"primaryKey"`
	Username    string
	Password    string // bcrypt
	Nickname    string
	Role        string // 主角色（展示用；实际鉴权走 admin_user_role → RBAC）
	Status      int8
	LastLoginAt *time.Time
	CreatedAt   time.Time
}

func (AdminUser) TableName() string { return "admin_user" }

// AuditLog 对应 audit_log 表。
type AuditLog struct {
	ID        int64 `gorm:"primaryKey;autoIncrement"`
	AdminID   int64
	Action    string
	ObjType   string
	Oid       int64
	Detail    string
	CreatedAt time.Time
}

func (AuditLog) TableName() string { return "audit_log" }

// SensitiveWord 对应 sensitive_word 表。
type SensitiveWord struct {
	ID        int64     `gorm:"primaryKey" json:"id,string"` // 字符串化防 JS 精度丢失
	Word      string    `json:"word"`
	CreatedAt time.Time `json:"created_at"`
}

func (SensitiveWord) TableName() string { return "sensitive_word" }

// AddWordReq 新增敏感词请求。
type AddWordReq struct {
	Word string `json:"word" binding:"required,max=64"`
}

// PunishReq 用户处罚请求。
type PunishReq struct {
	Action string `json:"action" binding:"required,oneof=mute unmute ban unban"` // 处罚动作
	Days   int    `json:"days" binding:"min=0,max=3650"`                          // 处罚天数（ban 传 0=永久）
	Reason string `json:"reason" binding:"max=200"`                               // 处罚原因（审计留痕）
}

// LoginReq 后台登录请求。
type LoginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResp 后台登录响应。
type LoginResp struct {
	Token    string `json:"token"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

// ReviewReq 审核操作请求。
type ReviewReq struct {
	Approve bool   `json:"approve"`
	Reason  string `json:"reason"`
}
