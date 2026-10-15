// Package live 直播域（M4-LIV-02/03）：房间生命周期、开播资格、在线人数、房管与禁言、
// 弹幕通道（WS hub + 频控 + 禁言执行 + 历史窗口）。
package live

import "time"

// 房间状态（spec live §5A.1）
const (
	RoomStatusIdle  = 0 // 空闲：有资格未开播
	RoomStatusLive  = 1 // 直播中
	RoomStatusEnded = 2 // 已下播（本场结束；下一场开播回到 1）
)

// LiveRoom 对应 live_room 表。
type LiveRoom struct {
	ID            int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	AnchorID      int64      `gorm:"uniqueIndex:uk_anchor" json:"anchor_id"`
	Title         string     `json:"title"`
	Cover         string     `json:"cover"`
	CategoryID    int64      `json:"category_id"`
	Status        int8       `json:"status"` // 0 空闲 1 直播中 2 已下播
	StreamKeyHash string     `json:"-"`      // bcrypt 哈希，永不出参
	StartedAt     *time.Time `json:"started_at,omitempty"`
	EndedAt       *time.Time `json:"ended_at,omitempty"`
	LiveCnt       int64      `json:"-"` // DB 兜底计数，对外读 Redis
	// 封禁（M4-LIV-05，迁移 0037）：与 status 正交——status 是场次生命周期，
	// ban_status 是合规处置（plan §2D.3）。二者不复用，避免污染列表查询与开播校验。
	BanStatus int8       `json:"ban_status"`
	BanReason string     `json:"ban_reason"`
	BanBy     int64      `json:"ban_by"`
	BanAt     *time.Time `json:"ban_at,omitempty"`
	CreatedAt time.Time  `json:"-"`
	UpdatedAt time.Time  `json:"-"`
}

func (LiveRoom) TableName() string { return "live_room" }

// LiveWhitelist 对应 live_whitelist 表（LIV-01 开播资格第一道闸）。
type LiveWhitelist struct {
	ID        int64      `gorm:"primaryKey;autoIncrement"`
	UserID    int64      `gorm:"uniqueIndex:uk_user"`
	Note      string     `json:"note"`
	CreatedBy int64      `json:"created_by"`
	CreatedAt time.Time  `json:"-"`
	RemovedAt *time.Time `json:"removed_at,omitempty"` // NULL=在名单内
}

func (LiveWhitelist) TableName() string { return "live_whitelist" }

// LiveRoomAdmin 对应 live_room_admin 表（房管；主播不落本表，由 live_room.anchor_id 判定）。
type LiveRoomAdmin struct {
	ID        int64     `gorm:"primaryKey;autoIncrement"`
	RoomID    int64     `gorm:"uniqueIndex:uk_room_user,priority:1"`
	UserID    int64     `gorm:"uniqueIndex:uk_room_user,priority:2"`
	CreatedBy int64     `json:"created_by"`
	CreatedAt time.Time `json:"-"`
}

func (LiveRoomAdmin) TableName() string { return "live_room_admin" }

// LiveRoomMute 对应 live_room_mute 表（禁言记录；弹幕通道执行在 M4-LIV-03）。
type LiveRoomMute struct {
	ID         int64     `gorm:"primaryKey;autoIncrement"`
	RoomID     int64     `gorm:"uniqueIndex:uk_room_user,priority:1"`
	UserID     int64     `gorm:"uniqueIndex:uk_room_user,priority:2"`
	MutedUntil time.Time `json:"muted_until"`
	Reason     string    `json:"reason"`
	CreatedBy  int64     `json:"created_by"`
	CreatedAt  time.Time `json:"-"`
	UpdatedAt  time.Time `json:"-"`
}

// LiveRoomMsg 对应 live_room_msg 表（M4-LIV-03 直播弹幕历史窗口；
// 仅保留最近 N 条供进房拉历史，下播清理，不与稿件生命周期绑定）。
type LiveRoomMsg struct {
	ID        int64     `gorm:"primaryKey" json:"id"` // 雪花 ID
	RoomID    int64     `json:"room_id"`
	UserID    int64     `json:"user_id"`
	Nickname  string    `json:"nickname"`
	Avatar    string    `json:"avatar"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

func (LiveRoomMsg) TableName() string { return "live_room_msg" }

func (LiveRoomMute) TableName() string { return "live_room_mute" }

// 房间封禁状态（M4-LIV-05，迁移 0037）
const (
	RoomBanNone    = 0 // 正常
	RoomBanApplied = 1 // 已封禁
)

// 违规来源与切断方式（live_violation_log，spec §5E.3）
const (
	ViolationSourceAuto   = 1 // 机审自动命中
	ViolationSourceManual = 2 // 人工巡查手动切断

	CutModeSrsAPI   = 1 // SRS API 踢流成功（确定性主通道）
	CutModeDegraded = 2 // SRS API 不可达：仅封禁 + 登记（不静默，plan §2D.3）
)

// LiveFrameAudit 对应 live_frame_audit 表（M4-LIV-05）：每截帧周期每房间一行。
// 这是 spec §4「内容存档 ≥60 天」与「判定发生过」的审计凭据（spec §5E.5）。
type LiveFrameAudit struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	RoomID    int64     `json:"room_id"`
	AnchorID  int64     `json:"anchor_id"`
	ImageKey  string    `json:"image_key"` // 落盘失败留空，但判定仍须落库
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	Pass      bool      `json:"pass"`
	HitWords  string    `json:"hit_words"` // 逗号分隔
	Degraded  bool      `json:"degraded"`  // 机审不可用（fail-open，不切断）
	CostMs    int       `json:"cost_ms"`
	CreatedAt time.Time `json:"created_at"`
}

func (LiveFrameAudit) TableName() string { return "live_frame_audit" }

// LiveViolationLog 对应 live_violation_log 表（M4-LIV-05）：违规流水。
type LiveViolationLog struct {
	ID           int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	RoomID       int64     `json:"room_id"`
	AnchorID     int64     `json:"anchor_id"`
	Source       int8      `json:"source"`         // 1 机审 2 人工巡查
	Reason       string    `json:"reason"`         //
	CutMode      int8      `json:"cut_mode"`       // 1 srs_api 2 degraded
	FrameAuditID *int64    `json:"frame_audit_id"` // 人工巡查无来源帧则为 NULL
	CutBy        int64     `json:"cut_by"`         // 0=系统
	CreatedAt    time.Time `json:"created_at"`
}

func (LiveViolationLog) TableName() string { return "live_violation_log" }
