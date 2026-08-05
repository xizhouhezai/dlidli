// Package im 私信（M3-IM，PRD MSG-10~13）：一对一会话、消息存储、发送限制、机审、WS 实时推送。
package im

import "time"

// Conversation 对应 conversation 表（user_a < user_b 规范化）。
type Conversation struct {
	ID          int64 `gorm:"primaryKey;autoIncrement"`
	UserA       int64
	UserB       int64
	LastContent string
	LastAt      time.Time
	UnreadA     int
	UnreadB     int
}

func (Conversation) TableName() string { return "conversation" }

// Message 对应 private_message 表。
type Message struct {
	ID             int64 `gorm:"primaryKey;autoIncrement"`
	ConversationID int64
	SenderID       int64
	Content        string
	ContentType    int8
	CreatedAt      time.Time
	ReadAt         *time.Time
}

func (Message) TableName() string { return "private_message" }

// ConversationItem 会话列表项（含对方用户信息与未读数）。
type ConversationItem struct {
	PeerID      int64     `json:"peer_id,string"`
	Nickname    string    `json:"nickname"`
	Avatar      string    `json:"avatar"`
	LastContent string    `json:"last_content"`
	LastAt      time.Time `json:"last_at"`
	Unread      int       `json:"unread"`
}

// MessageItem 消息项。
type MessageItem struct {
	ID          int64     `json:"id,string"`
	SenderID    int64     `json:"sender_id,string"`
	Content     string    `json:"content"`
	ContentType int8      `json:"content_type"`
	CreatedAt   time.Time `json:"created_at"`
	Mine        bool      `json:"mine"` // 是否我发送（前端气泡方向）
}

// WSMsg 服务端实时下发的消息帧。
type WSMsg struct {
	Type string      `json:"type"` // message
	Data MessageItem `json:"data"`
}
