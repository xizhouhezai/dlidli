package creator

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repo struct {
	db *gorm.DB
}

func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

// VideoBase 单稿基础信息（标题/封面/状态/发布时间）。
type VideoBase struct {
	ID          int64
	Bvid        string
	Title       string
	Cover       string
	Status      int8
	PublishedAt *time.Time
}

// ListMineVideos 我的稿件分页（含已删除外的全部状态），新→旧。
func (r *Repo) ListMineVideos(uid int64, page, size int) ([]VideoBase, int64, error) {
	q := r.db.Table("video").Where("user_id = ? AND status != 7", uid)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	list := make([]VideoBase, 0)
	err := q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, total, err
}

// CountMineVideos 我的稿件总数。
func (r *Repo) CountMineVideos(uid int64) (int64, error) {
	var n int64
	err := r.db.Table("video").Where("user_id = ? AND status != 7", uid).Count(&n).Error
	return n, err
}

// StatsByVideos 稿件统计（join video 校验归属）。
func (r *Repo) StatsByVideos(uid int64) ([]struct {
	VideoID int64
	ViewCnt int64
	LikeCnt int64
	CoinCnt int64
	FavCnt  int64
	CommentCnt int64
	DanmakuCnt int64
}, error) {
	var list []struct {
		VideoID int64
		ViewCnt int64
		LikeCnt int64
		CoinCnt int64
		FavCnt  int64
		CommentCnt int64
		DanmakuCnt int64
	}
	err := r.db.Table("video_stat").
		Select("video_stat.video_id, video_stat.view_cnt, video_stat.like_cnt, video_stat.coin_cnt, video_stat.fav_cnt, video_stat.comment_cnt, video_stat.danmaku_cnt").
		Joins("JOIN video ON video.id = video_stat.video_id").
		Where("video.user_id = ?", uid).
		Scan(&list).Error
	return list, err
}

// ValidViewStats 有效播放统计（行为日志 action=3 按稿件）。
func (r *Repo) ValidViewStats(uid int64) ([]struct {
	VideoID int64
	Cnt     int64
}, error) {
	var list []struct {
		VideoID int64
		Cnt     int64
	}
	err := r.db.Table("user_behavior").
		Select("user_behavior.video_id, COUNT(*) AS cnt").
		Joins("JOIN video ON video.id = user_behavior.video_id").
		Where("user_behavior.action = 3 AND video.user_id = ?", uid).
		Group("user_behavior.video_id").
		Scan(&list).Error
	return list, err
}

// PlayTrend 近 N 天行为趋势（按 action 指标过滤，含当天）。
func (r *Repo) PlayTrend(uid int64, days int, action int8) ([]TrendPoint, error) {
	from := time.Now().AddDate(0, 0, -(days - 1)).Format("2006-01-02")
	var list []TrendPoint
	err := r.db.Table("user_behavior").
		Select("DATE_FORMAT(user_behavior.created_at, '%Y-%m-%d') AS date, COUNT(*) AS views").
		Joins("JOIN video ON video.id = user_behavior.video_id").
		Where("user_behavior.action = ? AND video.user_id = ? AND user_behavior.created_at >= ?", action, uid, from).
		Group("date").
		Order("date").
		Scan(&list).Error
	return list, err
}

// FanCount 粉丝数（relation target 计数）。
func (r *Repo) FanCount(uid int64) (int64, error) {
	var n int64
	err := r.db.Table("relation").Where("target_id = ? AND type = 1", uid).Count(&n).Error
	return n, err
}

// UpsertSettlement 日结算 upsert（按 date+video 唯一）。
func (r *Repo) UpsertSettlement(s *CreatorSettlement) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "settle_date"}, {Name: "video_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"valid_views", "amount"}),
	}).Create(s).Error
}

// ListSettlements 收益明细分页（含稿件标题）。
func (r *Repo) ListSettlements(uid int64, page, size int) ([]SettlementItem, int64, error) {
	q := r.db.Table("creator_settlement cs").
		Select("cs.video_id, DATE_FORMAT(cs.settle_date, '%Y-%m-%d') AS date, v.bvid, v.title, cs.valid_views, cs.amount").
		Joins("JOIN video v ON v.id = cs.video_id").
		Where("cs.user_id = ?", uid)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	list := make([]SettlementItem, 0)
	err := q.Order("cs.settle_date DESC, cs.id DESC").Offset((page - 1) * size).Limit(size).Scan(&list).Error
	return list, total, err
}

// SumSettlements 累计收益（分）。
func (r *Repo) SumSettlements(uid int64) (int64, error) {
	var n int64
	err := r.db.Table("creator_settlement").Where("user_id = ?", uid).Select("COALESCE(SUM(amount),0)").Scan(&n).Error
	return n, err
}

// SettleAll 全量结算（INSERT SELECT 聚合 + upsert，幂等）：按日期×稿件分组有效播放，收益=播放×单价（分）。
func (r *Repo) SettleAll(uid int64, rate int) error {
	return r.db.Exec(`INSERT INTO creator_settlement (settle_date, user_id, video_id, valid_views, amount)
		SELECT DATE(ub.created_at), v.user_id, ub.video_id, COUNT(*), COUNT(*) * ?
		FROM user_behavior ub
		JOIN video v ON v.id = ub.video_id
		WHERE ub.action = 3 AND v.user_id = ?
		GROUP BY DATE(ub.created_at), ub.video_id
		ON DUPLICATE KEY UPDATE valid_views = VALUES(valid_views), amount = VALUES(amount)`, rate, uid).Error
}
