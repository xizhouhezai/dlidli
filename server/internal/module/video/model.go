// Package video 稿件域：投稿、状态机、列表与详情。
package video

import "time"

// 稿件状态（对齐 docs/architecture/data-model.md）
const (
	StatusDraft       = 0
	StatusUploading   = 1
	StatusTranscoding = 2
	StatusReviewing   = 3
	StatusPublished   = 4
	StatusRejected    = 5
	StatusLocked      = 6
	StatusDeleted     = 7
)

// 机审风险等级（M2-AUD-02：低风险抽检/高风险全人审）
const (
	RiskLow    = 0
	RiskMedium = 1
	RiskHigh   = 2
)

// Video 对应 video 表。
type Video struct {
	ID           int64 `gorm:"primaryKey"`
	Bvid         string
	UserID       int64
	Title        string
	Cover        string
	Description  string
	CategoryID   int
	Tags         string `gorm:"type:json"` // JSON 数组字符串
	Copyright    int8
	Duration     int
	Status       int8
	RiskLevel    int8 // 0低 1中 2高（机审计算）
	RejectReason *string
	PublishedAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Version      int
}

func (Video) TableName() string { return "video" }

// Stream 对应 video_stream 表；quality=0 表示原画（未转码源文件）。
type Stream struct {
	ID       int64 `gorm:"primaryKey"`
	VideoID  int64
	Quality  int16
	Format   string
	PlayPath string
	FileSize int64
}

func (Stream) TableName() string { return "video_stream" }

// Stat 对应 video_stat 表。
type Stat struct {
	VideoID    int64 `gorm:"primaryKey"`
	ViewCnt    int64
	LikeCnt    int64
	CoinCnt    int64
	FavCnt     int64
	DanmakuCnt int64
	CommentCnt int64
	ShareCnt   int64
}

func (Stat) TableName() string { return "video_stat" }

// 转码任务状态
const (
	JobPending = 0
	JobRunning = 1
	JobSuccess = 2
	JobFailed  = 3
)

// TranscodeJob 对应 transcode_job 表（DB 任务队列，本地无 Kafka 的替代方案）。
type TranscodeJob struct {
	ID        int64 `gorm:"primaryKey"`
	VideoID   int64
	Quality   int16
	Status    int8
	RetryCnt  int8
	ErrorMsg  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (TranscodeJob) TableName() string { return "transcode_job" }

// Category 对应 category 表。
type Category struct {
	ID       int    `json:"id" gorm:"primaryKey"`
	ParentID int    `json:"parent_id"`
	Name     string `json:"name"`
	Sort     int    `json:"sort"`
	Status   int8   `json:"status"` // 0 正常 1 停用
}

func (Category) TableName() string { return "category" }

// ---- DTO ----

// SubmitReq 投稿请求。
type SubmitReq struct {
	FileID      string   `json:"file_id" binding:"required"`
	Title       string   `json:"title" binding:"required,max=80"`
	Description string   `json:"description" binding:"max=2000"`
	CategoryID  int      `json:"category_id" binding:"required"`
	Tags        []string `json:"tags" binding:"required,min=1,max=10"`
	Copyright   int8     `json:"copyright" binding:"required,oneof=1 2"` // 1自制 2转载
	Cover       string   `json:"cover"`                                  // 可选，封面 URL
}

// OwnerBrief 卡片中的 UP 主信息。
type OwnerBrief struct {
	ID       string `json:"id"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
}

// StatBrief 卡片计数。
type StatBrief struct {
	View    int64 `json:"view"`
	Like    int64 `json:"like"`
	Coin    int64 `json:"coin"`
	Fav     int64 `json:"fav"`
	Danmaku int64 `json:"danmaku"`
	Comment int64 `json:"comment"`
	Share   int64 `json:"share"`
}

// Card 列表卡片。
type Card struct {
	ID          string     `json:"id"` // 内部 ID（字符串化防 JS 精度丢失；负反馈等场景用）
	Bvid        string     `json:"bvid"`
	Title       string     `json:"title"`
	Cover       string     `json:"cover"`
	Duration    int        `json:"duration"`
	Status      int8       `json:"status"`
	RiskLevel   int8       `json:"risk_level"` // 0低 1中 2高
	CategoryID  int        `json:"category_id"`
	PublishedAt *time.Time `json:"published_at"`
	CreatedAt   time.Time  `json:"created_at"`
	Owner       OwnerBrief `json:"owner"`
	Stat        StatBrief  `json:"stat"`
}

// StreamItem 播放流。
type StreamItem struct {
	Quality int16  `json:"quality"` // 0=原画
	Format  string `json:"format"`
	URL     string `json:"url"`
}

// Detail 稿件详情。
type Detail struct {
	Card
	Description  string       `json:"description"`
	CategoryID   int          `json:"category_id"`
	Tags         []string     `json:"tags"`
	Copyright    int8         `json:"copyright"`
	RejectReason string       `json:"reject_reason,omitempty"`
	Streams      []StreamItem `json:"streams"`
}

// ReviewItem 审核队列条目（供后台工作台）。
type ReviewItem struct {
	Card
	Description string `json:"description"`
	PlayURL     string `json:"play_url"` // 原画预览地址
}
