package report

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

type Repo struct {
	db *gorm.DB
}

func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

// Create 新增举报。
func (r *Repo) Create(rep *Report) error {
	return r.db.Create(rep).Error
}

// FindDuplicated 同举报人对同对象是否已举报（防刷）；不存在返回 (nil, nil)。
func (r *Repo) FindDuplicated(reporterID, targetID int64, targetType int8) (*Report, error) {
	var rep Report
	err := r.db.Where("reporter_id = ? AND target_id = ? AND target_type = ?",
		reporterID, targetID, targetType).First(&rep).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &rep, nil
}

// FindByID 按 ID 查举报；不存在返回 (nil, nil)。
func (r *Repo) FindByID(id int64) (*Report, error) {
	var rep Report
	err := r.db.First(&rep, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &rep, nil
}

// ListByStatus 举报列表（status=-1 全部），新→旧。
func (r *Repo) ListByStatus(status int8, page, size int) ([]Report, int64, error) {
	q := r.db.Model(&Report{})
	if status >= 0 {
		q = q.Where("status = ?", status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []Report
	err := q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, total, err
}

// UpdateHandle 写入处理结果（幂等由调用侧保证仅待处理可处理）。
func (r *Repo) UpdateHandle(id, handlerID int64, status int8, result string) error {
	return r.db.Model(&Report{}).Where("id = ?", id).Updates(map[string]any{
		"status":        status,
		"handler_id":    handlerID,
		"handle_result": result,
		"handled_at":    time.Now(),
	}).Error
}
