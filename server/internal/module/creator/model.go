// Package creator 创作者中心（M3-CRT）：数据看板聚合、单稿分析、创作激励结算。
// 本地务实版：实时聚合（video_stat + 行为日志），不做 T+1 数仓；结算按日增量（请求时结算）。
package creator

import "time"

// CreatorSettlement 对应 creator_settlement 表（创作激励日结算）。
type CreatorSettlement struct {
	ID         int64     `gorm:"primaryKey;autoIncrement"`
	SettleDate time.Time `gorm:"column:settle_date;type:date"`
	UserID     int64
	VideoID    int64
	ValidViews int
	Amount     int // 收益（分）
	CreatedAt  time.Time
}

func (CreatorSettlement) TableName() string { return "creator_settlement" }

// Overview 创作者总览。
type Overview struct {
	VideoCnt  int64 `json:"video_cnt"`  // 投稿数
	TotalView int64 `json:"total_view"` // 总播放
	TotalLike int64 `json:"total_like"` // 总点赞
	TotalCoin int64 `json:"total_coin"` // 总投币
	TotalFav  int64 `json:"total_fav"`  // 总收藏
	Fans      int64 `json:"fans"`       // 粉丝数
	WeekView  int64 `json:"week_view"`  // 近 7 日播放
	Earnings  int64 `json:"earnings"`   // 累计收益（分）
}

// TrendPoint 播放趋势点。
type TrendPoint struct {
	Date  string `json:"date"`
	Views int64  `json:"views"`
}

// VideoStatItem 单稿数据。
type VideoStatItem struct {
	Bvid        string     `json:"bvid"`
	Title       string     `json:"title"`
	Cover       string     `json:"cover"`
	Status      int8       `json:"status"`
	View        int64      `json:"view"`
	Like        int64      `json:"like"`
	Coin        int64      `json:"coin"`
	Fav         int64      `json:"fav"`
	Comment     int64      `json:"comment"`
	Danmaku     int64      `json:"danmaku"`
	ValidViews  int64      `json:"valid_views"` // 有效播放（>5s）
	Earnings    int64      `json:"earnings"`    // 该稿累计收益（分）
	PublishedAt *time.Time `json:"published_at"`
}

// SettlementItem 收益明细项。
type SettlementItem struct {
	VideoID    int64  `json:"-"` // 内部聚合用（不对外）
	Date       string `json:"date"`
	Bvid       string `json:"bvid"`
	Title      string `json:"title"`
	ValidViews int64  `json:"valid_views"`
	Amount     int64  `json:"amount"`
}
