// Package danmaku 弹幕域：发送、按时间轴分段拉取。
//
// 分段规则：每段 6 分钟（SegmentMS），前端按播放进度拉取当前段与下一段；
// 读走 Redis 段缓存，写后失效对应段。实时广播（WebSocket comet）在 V1 接入。
package danmaku

import "time"

// SegmentMS 弹幕分段长度（毫秒）= 6 分钟。
const SegmentMS = 6 * 60 * 1000

// 弹幕状态
const (
	StatusNormal  = 0
	StatusShadow  = 1 // 影子屏蔽：仅发送者可见
	StatusDeleted = 2
)

// Danmaku 对应 danmaku 表。
type Danmaku struct {
	ID        int64 `gorm:"primaryKey"`
	VideoID   int64
	UserID    int64
	Content   string
	TimeMs    int
	Mode      int8
	Color     uint32
	FontSize  int8
	Status    int8
	CreatedAt time.Time
}

func (Danmaku) TableName() string { return "danmaku" }

// SendReq 发送弹幕请求。
type SendReq struct {
	Content  string `json:"content" binding:"required,max=100"`
	TimeMs   int    `json:"time_ms" binding:"min=0"`
	Mode     int8   `json:"mode" binding:"omitempty,oneof=1 2 3"` // 1滚动 2顶部 3底部
	Color    uint32 `json:"color"`                                // RGB，默认白色
	FontSize int8   `json:"font_size"`                            // 默认 25
}

// Item 对外弹幕条目（列表与发送响应共用）。
type Item struct {
	ID       string `json:"id"`
	Content  string `json:"content"`
	TimeMs   int    `json:"time_ms"`
	Mode     int8   `json:"mode"`
	Color    uint32 `json:"color"`
	FontSize int8   `json:"font_size"`
	IsSelf   bool   `json:"is_self,omitempty"`
}
