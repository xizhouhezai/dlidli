// Package live 直播域（M4-LIV-02）：房间生命周期、开播资格、在线人数、房管与禁言。
// 弹幕广播与弹幕通道拦截归 M4-LIV-03（独立 live hub，不复用 danmaku hub，见 plan live §2A.2）。
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
	CreatedAt     time.Time  `json:"-"`
	UpdatedAt     time.Time  `json:"-"`
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

func (LiveRoomMute) TableName() string { return "live_room_mute" }
