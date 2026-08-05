package collection

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

func (r *Repo) Create(c *Collection) error {
	return r.db.Create(c).Error
}

// ListByUser 某 UP 主的合集（新→旧），带稿件数与首封面兜底。
func (r *Repo) ListByUser(uid int64) ([]Card, error) {
	cards := make([]Card, 0)
	err := r.db.Raw(`SELECT c.id, c.title, c.description,
			COALESCE(NULLIF(c.cover, ''), (SELECT v.cover FROM video_collection_item ci
				JOIN video v ON v.id = ci.video_id
				WHERE ci.collection_id = c.id ORDER BY ci.sort, ci.id LIMIT 1), '') AS cover,
			(SELECT COUNT(*) FROM video_collection_item ci2 WHERE ci2.collection_id = c.id) AS video_count,
			c.created_at
		FROM video_collection c WHERE c.user_id = ? ORDER BY c.id DESC`, uid).Scan(&cards).Error
	return cards, err
}

// FindByID 合集详情；不存在返回 (nil, nil)。
func (r *Repo) FindByID(id int64) (*Collection, error) {
	var c Collection
	err := r.db.First(&c, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// VideoIDs 合集内稿件 id（按 sort）。
func (r *Repo) VideoIDs(collectionID int64) ([]int64, error) {
	var ids []int64
	err := r.db.Model(&CollectionItem{}).
		Where("collection_id = ?", collectionID).
		Order("sort, id").Pluck("video_id", &ids).Error
	return ids, err
}

// AddVideo 添加稿件（校验唯一）。
func (r *Repo) AddVideo(collectionID, videoID int64, sort int) error {
	return r.db.Create(&CollectionItem{CollectionID: collectionID, VideoID: videoID, Sort: sort}).Error
}

// RemoveVideo 移除稿件。
func (r *Repo) RemoveVideo(collectionID, videoID int64) error {
	return r.db.Where("collection_id = ? AND video_id = ?", collectionID, videoID).Delete(&CollectionItem{}).Error
}

func (r *Repo) Delete(id int64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("collection_id = ?", id).Delete(&CollectionItem{}).Error; err != nil {
			return err
		}
		return tx.Delete(&Collection{}, id).Error
	})
}
