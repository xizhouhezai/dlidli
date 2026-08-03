package danmaku

import (
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

// ListAll 全量弹幕分页（弹幕列表面板用；含已删除过滤），新→旧。
func (r *Repo) ListAll(videoID int64, page, size int) ([]Danmaku, int64, error) {
	q := r.db.Model(&Danmaku{}).Where("video_id = ? AND status = ?", videoID, StatusNormal)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []Danmaku
	err := q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, total, err
}

// ---- 弹幕屏蔽（M2-DM-02） ----

// CreateBlock 新增屏蔽（幂等：重复插入忽略）。
func (r *Repo) CreateBlock(b *DanmakuBlock) error {
	return r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(b).Error
}

// ListBlocks 某用户全部屏蔽项（新→旧）。
func (r *Repo) ListBlocks(uid int64) ([]DanmakuBlock, error) {
	var list []DanmakuBlock
	err := r.db.Where("user_id = ?", uid).Order("id DESC").Find(&list).Error
	return list, err
}

// CountBlocks 某用户某类型屏蔽项数量。
func (r *Repo) CountBlocks(uid int64, blockType int8) (int64, error) {
	var n int64
	err := r.db.Model(&DanmakuBlock{}).Where("user_id = ? AND block_type = ?", uid, blockType).Count(&n).Error
	return n, err
}

// DeleteBlock 删除屏蔽项（仅本人）。
func (r *Repo) DeleteBlock(uid, id int64) error {
	return r.db.Where("id = ? AND user_id = ?", id, uid).Delete(&DanmakuBlock{}).Error
}
