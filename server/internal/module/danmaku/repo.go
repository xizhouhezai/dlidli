package danmaku

import (
	"gorm.io/gorm"
)

type Repo struct {
	db *gorm.DB
}

func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) Create(d *Danmaku) error {
	return r.db.Create(d).Error
}

// ListSegment 拉取某段内正常状态的弹幕（按时间升序，段内上限 1000 条）。
func (r *Repo) ListSegment(videoID int64, fromMs, toMs int) ([]Danmaku, error) {
	var list []Danmaku
	err := r.db.Where("video_id = ? AND time_ms >= ? AND time_ms < ? AND status = ?",
		videoID, fromMs, toMs, StatusNormal).
		Order("time_ms").Limit(1000).Find(&list).Error
	return list, err
}
