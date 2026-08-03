package danmaku

import (
	"errors"

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

// FindByID 查弹幕；不存在返回 (nil, nil)。
func (r *Repo) FindByID(id int64) (*Danmaku, error) {
	var d Danmaku
	err := r.db.First(&d, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// MarkDeleted 弹幕置删除状态（举报处理用）。
func (r *Repo) MarkDeleted(id int64) error {
	return r.db.Model(&Danmaku{}).Where("id = ?", id).UpdateColumn("status", StatusDeleted).Error
}
