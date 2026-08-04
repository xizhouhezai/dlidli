package recommend

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

// HotEntry 热度榜条目。
type HotEntry struct {
	VideoID int64
	Score   int64
}

// 热度加权分（互动权重高于播放；share 量少权重低）
const hotScoreSQL = `(video_stat.view_cnt + video_stat.like_cnt*3 + video_stat.coin_cnt*5 +
	video_stat.fav_cnt*4 + video_stat.comment_cnt*4 + video_stat.danmaku_cnt*2 + video_stat.share_cnt*3)`

// ListHot 全站/分区热度榜 TopN（按加权分降序）。
func (r *Repo) ListHot(categoryID int, limit int) ([]HotEntry, error) {
	q := r.db.Model(&VideoStatAlias{}).
		Select("video_stat.video_id", hotScoreSQL+" AS score").
		Joins("JOIN video ON video.id = video_stat.video_id").
		Where("video.status = ? AND video.published_at IS NOT NULL", 4)
	if categoryID > 0 {
		q = q.Where("video.category_id = ?", categoryID)
	}
	var entries []HotEntry
	err := q.Order("score DESC").Limit(limit).Scan(&entries).Error
	return entries, err
}

// NewPool 新稿扶持池：发布 48h 内、播放 < 100（冷启动保底曝光）。
func (r *Repo) NewPool(limit int) ([]int64, error) {
	var ids []int64
	err := r.db.Model(&VideoAlias{}).
		Select("video.id").
		Joins("LEFT JOIN video_stat ON video_stat.video_id = video.id").
		Where("video.status = 4 AND video.published_at >= ? AND COALESCE(video_stat.view_cnt,0) < 100", time.Now().Add(-48*time.Hour)).
		Order("video.published_at DESC").Limit(limit).Pluck("video.id", &ids).Error
	return ids, err
}

// RecentWatchCategories 最近观看/点击视频的分区（按最近行为取前 N 个去重）。
func (r *Repo) RecentWatchCategories(uid int64, n int) ([]int, error) {
	var cats []int
	err := r.db.Raw(`SELECT DISTINCT v.category_id FROM user_behavior ub
		JOIN video v ON v.id = ub.video_id
		WHERE ub.user_id = ? AND ub.action IN (?, ?)
		ORDER BY ub.id DESC LIMIT ?`, uid, ActionClick, ActionPlay, n*3).Scan(&cats).Error
	return cats, err
}

// RecentWatchedIDs 最近有效观看的稿件（已看过滤用）。
func (r *Repo) RecentWatchedIDs(uid int64, limit int) ([]int64, error) {
	var ids []int64
	err := r.db.Model(&UserBehavior{}).
		Where("user_id = ? AND action = ?", uid, ActionPlay).
		Order("id DESC").Limit(limit).Pluck("video_id", &ids).Error
	return ids, err
}

// DislikesOf 用户负反馈（内容/UP主/分区）。
func (r *Repo) DislikesOf(uid int64) ([]UserDislike, error) {
	var list []UserDislike
	err := r.db.Where("user_id = ?", uid).Find(&list).Error
	return list, err
}

// AddDislike 负反馈（幂等）。
func (r *Repo) AddDislike(d *UserDislike) error {
	return r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(d).Error
}

// AddBehaviors 批量写行为日志（旁路，单条失败不影响整体）。
func (r *Repo) AddBehaviors(list []UserBehavior) error {
	return r.db.Create(&list).Error
}

// RecommendOn 个性化推荐开关（user 表）。
func (r *Repo) RecommendOn(uid int64) (bool, error) {
	var v int8
	err := r.db.Table("user").Select("recommend_on").Where("id = ?", uid).Scan(&v).Error
	return v == 1, err
}

// SetRecommendOn 开关个性化推荐。
func (r *Repo) SetRecommendOn(uid int64, on bool) error {
	v := int8(0)
	if on {
		v = 1
	}
	return r.db.Table("user").Where("id = ?", uid).Update("recommend_on", v).Error
}

// VideoAlias / VideoStatAlias 仅用于推荐模块查询 video/video_stat 表（避免依赖 video 包模型）。
type VideoAlias struct{ ID int64 }

func (VideoAlias) TableName() string { return "video" }

type VideoStatAlias struct{ VideoID int64 }

func (VideoStatAlias) TableName() string { return "video_stat" }
