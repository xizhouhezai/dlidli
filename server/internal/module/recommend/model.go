// Package recommend 推荐域（M3-REC）：行为采集、热度榜、混合召回推荐、负反馈、推荐开关。
// 本地务实版：MySQL 落库 + Redis 缓存；规模化后行为日志迁 ClickHouse、召回换向量模型，接口保持兼容。
package recommend

import "time"

// 行为类型
const (
	ActionExpose  int8 = 1 // 曝光
	ActionClick   int8 = 2 // 点击
	ActionPlay    int8 = 3 // 播放（>5s 有效）
	ActionInteract int8 = 4 // 互动（赞/币/藏/评）
)

// 负反馈维度
const (
	DislikeVideo    int8 = 1 // 内容
	DislikeUP       int8 = 2 // UP 主
	DislikeCategory int8 = 3 // 分区
)

// UserBehavior 对应 user_behavior 表。
type UserBehavior struct {
	ID        int64 `gorm:"primaryKey;autoIncrement"`
	UserID    int64
	VideoID   int64
	Action    int8
	CreatedAt time.Time
}

func (UserBehavior) TableName() string { return "user_behavior" }

// UserDislike 对应 user_dislike 表。
type UserDislike struct {
	ID         int64 `gorm:"primaryKey;autoIncrement"`
	UserID     int64
	TargetType int8
	TargetID   int64
	CreatedAt  time.Time
}

func (UserDislike) TableName() string { return "user_dislike" }

// RecItem 推荐结果条目。
type RecItem struct {
	Bvid       string `json:"bvid"`
	Title      string `json:"title"`
	Cover      string `json:"cover"`
	Duration   int    `json:"duration"`
	CategoryID int    `json:"category_id"`
	UPID       int64  `json:"up_id,string"`
	Score      int64  `json:"score"`
	IsNew      bool   `json:"is_new"` // 新稿扶持池命中
}

// BehaviorItem 行为上报条目（ID 字符串化防 JS 精度丢失）。
type BehaviorItem struct {
	VideoID string `json:"video_id" binding:"required"`
	Action  int8   `json:"action" binding:"required,oneof=1 2 3 4"` // 1曝光 2点击 3播放 4互动
}
