// Package searchindex 稿件搜索索引同步（M2-SRH-01/02）：
//
// 以 MySQL Outbox 模式解耦稿件写路径与 Elasticsearch 索引同步：
//   - 稿件发布/下架/删除时异步写入 search_index_outbox（不阻塞主流程）；
//   - 进程内 Worker 周期轮询 outbox，批量推送 ES（upsert/delete），
//     失败重试（指数退避），超限标记失败便于人工对账；
//   - ES 未配置/不可用时搜索自动降级 MySQL LIKE（接口层平滑切换）。
package searchindex

import "time"

// Outbox 对应 search_index_outbox 表。
type Outbox struct {
	ID        int64     `gorm:"primaryKey;autoIncrement"`
	VideoID   int64     `gorm:"column:video_id"`
	Action    string    `gorm:"column:action"` // upsert | delete
	Attempts  int       `gorm:"column:attempts"`
	Status    int8      `gorm:"column:status"` // 0待处理 1完成 2失败(超限)
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

// TableName 表名。
func (Outbox) TableName() string { return "search_index_outbox" }

// 状态常量。
const (
	OutboxPending = 0
	OutboxDone    = 1
	OutboxFailed  = 2
)

// 动作常量。
const (
	ActionUpsert = "upsert"
	ActionDelete = "delete"
)

// Doc Elasticsearch 稿件文档（与 video.Card 对齐的检索字段）。
type Doc struct {
	VideoID     int64    `json:"video_id"`
	Bvid        string   `json:"bvid"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Cover       string   `json:"cover"`
	CategoryID  int      `json:"category_id"`
	Tags        []string `json:"tags"`
	OwnerID     int64    `json:"owner_id"`
	OwnerName   string   `json:"owner_name"`
	Duration    int      `json:"duration"`
	PublishedAt int64    `json:"published_at"` // Unix 秒
	ViewCnt     int64    `json:"view_cnt"`
	LikeCnt     int64    `json:"like_cnt"`
}
