package searchindex

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Repo Outbox 数据访问。
type Repo struct {
	db *gorm.DB
}

func NewRepo(db *gorm.DB) *Repo { return &Repo{db: db} }

// Enqueue 登记一次索引变更。同稿件存在待处理记录时先折叠（删除旧待处理），
// 保证每稿件至多一条待处理、最新动作生效（latest-wins）。
func (r *Repo) Enqueue(ctx context.Context, videoID int64, action string) error {
	now := time.Now()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("video_id = ? AND status = ?", videoID, OutboxPending).
			Delete(&Outbox{}).Error; err != nil {
			return err
		}
		return tx.Create(&Outbox{
			VideoID: videoID, Action: action, Status: OutboxPending,
			CreatedAt: now, UpdatedAt: now,
		}).Error
	})
}

// PendingBatch 取一批待处理（按 id 升序，稳定顺序）。
func (r *Repo) PendingBatch(ctx context.Context, limit int) ([]Outbox, error) {
	if limit <= 0 {
		limit = 100
	}
	var list []Outbox
	err := r.db.WithContext(ctx).
		Where("status = ?", OutboxPending).
		Order("id ASC").Limit(limit).Find(&list).Error
	return list, err
}

// MarkDone 标记完成。
func (r *Repo) MarkDone(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Model(&Outbox{}).Where("id IN ?", ids).
		Update("status", OutboxDone).Error
}

// MarkFailed 标记失败（超限）。attempts 由调用方在同步前自增并回写。
func (r *Repo) MarkFailed(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Model(&Outbox{}).Where("id IN ?", ids).
		Updates(map[string]any{"status": OutboxFailed, "updated_at": time.Now()}).Error
}

// IncrAttempts 同步失败后自增重试次数（返回最新值，供超限判断）。
func (r *Repo) IncrAttempts(ctx context.Context, id int64) (int, error) {
	var o Outbox
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", id).First(&o).Error; err != nil {
			return err
		}
		o.Attempts++
		return tx.Model(&o).Update("attempts", o.Attempts).Error
	})
	return o.Attempts, err
}
