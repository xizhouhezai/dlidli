// Package collection UP 主合集（M3-CRT-05 合集部分）：创建/管理/视频归集。
package collection

import "time"

// Collection 对应 collection 表。
type Collection struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id,string"`
	UserID      int64     `json:"user_id,string"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Cover       string    `json:"cover"`
	CreatedAt   time.Time `json:"created_at"`
}

func (Collection) TableName() string { return "video_collection" }

// CollectionItem 对应 collection_item 表。
type CollectionItem struct {
	ID           int64 `gorm:"primaryKey;autoIncrement"`
	CollectionID int64
	VideoID      int64
	Sort         int
	CreatedAt    time.Time
}

func (CollectionItem) TableName() string { return "video_collection_item" }

// Card 合集卡片（含首视频封面兜底与稿件数）。
type Card struct {
	ID          int64     `json:"id,string"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Cover       string    `json:"cover"`
	VideoCount  int64     `json:"video_count"`
	CreatedAt   time.Time `json:"created_at"`
}
