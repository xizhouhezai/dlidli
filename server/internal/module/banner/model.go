// Package banner 运营位（M3-OPS-01）：首页轮播 Banner 配置与公开读取。
package banner

import "time"

// Banner 对应 banner 表。
type Banner struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id,string"`
	Title     string    `json:"title"`
	Image     string    `json:"image"` // 图片 URL（空则用稿件封面）
	Bvid      string    `json:"bvid"`  // 跳转稿件（空则不跳）
	Sort      int       `json:"sort"`
	Status    int8      `json:"status"` // 0启用 1停用
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Banner) TableName() string { return "banner" }

// Item 公开 Banner（含稿件封面兜底）。
type Item struct {
	ID    int64  `json:"id,string"`
	Title string `json:"title"`
	Image string `json:"image"`
	Bvid  string `json:"bvid"`
}
