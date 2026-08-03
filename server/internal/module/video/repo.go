package video

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

// Categories 启用中的分区列表。
func (r *Repo) Categories() ([]Category, error) {
	var list []Category
	err := r.db.Where("status = 0").Order("parent_id, sort").Find(&list).Error
	return list, err
}

func (r *Repo) CategoryExists(id int) (bool, error) {
	var cnt int64
	err := r.db.Model(&Category{}).Where("id = ? AND status = 0", id).Count(&cnt).Error
	return cnt > 0, err
}

// AllCategories 后台：全部分区（含停用），按 parent_id, sort 排序。
func (r *Repo) AllCategories() ([]Category, error) {
	var list []Category
	err := r.db.Order("parent_id, sort, id").Find(&list).Error
	return list, err
}

func (r *Repo) CreateCategory(c *Category) error {
	return r.db.Create(c).Error
}

func (r *Repo) UpdateCategory(id int, fields map[string]any) error {
	return r.db.Model(&Category{}).Where("id = ?", id).Updates(fields).Error
}

func (r *Repo) DeleteCategory(id int) error {
	return r.db.Delete(&Category{}, id).Error
}

// CategoryChildCount 子分区数（删除一级分区前校验）。
func (r *Repo) CategoryChildCount(parentID int) (int64, error) {
	var cnt int64
	err := r.db.Model(&Category{}).Where("parent_id = ?", parentID).Count(&cnt).Error
	return cnt, err
}

// CategoryVideoCount 分区下稿件数（删除前校验）。
func (r *Repo) CategoryVideoCount(id int) (int64, error) {
	var cnt int64
	err := r.db.Model(&Video{}).Where("category_id = ?", id).Count(&cnt).Error
	return cnt, err
}

func (r *Repo) FindCategory(id int) (*Category, error) {
	var c Category
	err := r.db.First(&c, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &c, err
}

// CreateWithStat 事务创建稿件 + 计数行 + 原画流 + 转码任务。
func (r *Repo) CreateWithStat(v *Video, stream *Stream, jobQualities []int16) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(v).Error; err != nil {
			return err
		}
		if err := tx.Create(&Stat{VideoID: v.ID}).Error; err != nil {
			return err
		}
		stream.VideoID = v.ID
		if err := tx.Create(stream).Error; err != nil {
			return err
		}
		for _, q := range jobQualities {
			if err := tx.Create(&TranscodeJob{VideoID: v.ID, Quality: q}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// FindByBvid 按 bvid 查稿件；不存在返回 (nil, nil)。
func (r *Repo) FindByBvid(bv string) (*Video, error) {
	var v Video
	err := r.db.Where("bvid = ?", bv).First(&v).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// ListMine 我的稿件（全状态，不含已删除），按创建时间倒序。
func (r *Repo) ListMine(uid int64, page, size int) ([]Video, int64, error) {
	q := r.db.Model(&Video{}).Where("user_id = ? AND status <> ?", uid, StatusDeleted)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []Video
	err := q.Order("created_at DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, total, err
}

// ListPublished 公开列表：sort = new（发布时间）/ hot（播放量）。
func (r *Repo) ListPublished(categoryID int, uid int64, sort string, page, size int) ([]Video, error) {
	q := r.db.Model(&Video{}).Where("video.status = ?", StatusPublished)
	if categoryID > 0 {
		q = q.Where("video.category_id = ?", categoryID)
	}
	if uid > 0 {
		q = q.Where("video.user_id = ?", uid) // 个人空间：某 UP 主的公开投稿
	}
	if sort == "hot" {
		q = q.Joins("LEFT JOIN video_stat ON video_stat.video_id = video.id").
			Order("video_stat.view_cnt DESC")
	} else {
		q = q.Order("video.published_at DESC")
	}
	var list []Video
	err := q.Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, err
}

// SearchPublished 标题模糊搜索（MVP LIKE；量级上来后切 ES）。
func (r *Repo) SearchPublished(keyword string, page, size int) ([]Video, int64, error) {
	q := r.db.Model(&Video{}).
		Where("status = ? AND title LIKE ?", StatusPublished, "%"+keyword+"%")
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []Video
	err := q.Order("published_at DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, total, err
}

// StatsByIDs 批量取计数。
func (r *Repo) StatsByIDs(ids []int64) (map[int64]Stat, error) {
	if len(ids) == 0 {
		return map[int64]Stat{}, nil
	}
	var list []Stat
	if err := r.db.Where("video_id IN ?", ids).Find(&list).Error; err != nil {
		return nil, err
	}
	m := make(map[int64]Stat, len(list))
	for _, s := range list {
		m[s.VideoID] = s
	}
	return m, nil
}

// StreamsByVideo 稿件的播放流。
func (r *Repo) StreamsByVideo(videoID int64) ([]Stream, error) {
	var list []Stream
	err := r.db.Where("video_id = ?", videoID).Order("quality DESC").Find(&list).Error
	return list, err
}

// IncView 播放量 +1。MVP 直接自增；高并发后改异步聚合回写。
func (r *Repo) IncView(videoID int64) error {
	return r.IncStatColumn(videoID, "view_cnt")
}

// IncStatColumn 计数列 +1。
func (r *Repo) IncStatColumn(videoID int64, column string) error {
	return r.AddStatColumn(videoID, column, 1)
}

// AddStatColumn 计数列增减（仅限白名单列防注入；下限保护不为负）。
func (r *Repo) AddStatColumn(videoID int64, column string, delta int) error {
	switch column {
	case "view_cnt", "like_cnt", "coin_cnt", "fav_cnt", "danmaku_cnt", "comment_cnt", "share_cnt":
	default:
		return errors.New("非法计数列: " + column)
	}
	return r.db.Model(&Stat{}).Where("video_id = ?", videoID).
		UpdateColumn(column, gorm.Expr("GREATEST(CAST("+column+" AS SIGNED) + ?, 0)", delta)).Error
}

// ---- 转码任务队列 ----

// FindVideoByID 按 ID 查稿件；不存在返回 (nil, nil)。
func (r *Repo) FindVideoByID(id int64) (*Video, error) {
	var v Video
	err := r.db.First(&v, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// ClaimJob 认领一个待处理任务（FOR UPDATE SKIP LOCKED，多 Worker 安全）；无任务返回 (nil, nil)。
func (r *Repo) ClaimJob() (*TranscodeJob, error) {
	var job TranscodeJob
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = ?", JobPending).Order("id").First(&job).Error; err != nil {
			return err
		}
		return tx.Model(&TranscodeJob{}).Where("id = ?", job.ID).Update("status", JobRunning).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	job.Status = JobRunning
	return &job, nil
}

func (r *Repo) CompleteJob(id int64) error {
	return r.db.Model(&TranscodeJob{}).Where("id = ?", id).
		Updates(map[string]any{"status": JobSuccess, "error_msg": ""}).Error
}

// FailJob 失败处理：未超重试上限则重新入队。
func (r *Repo) FailJob(job *TranscodeJob, msg string) error {
	if len(msg) > 500 {
		msg = msg[:500]
	}
	next := map[string]any{"error_msg": msg, "retry_cnt": job.RetryCnt + 1}
	if job.RetryCnt < 2 {
		next["status"] = JobPending
	} else {
		next["status"] = JobFailed
	}
	return r.db.Model(&TranscodeJob{}).Where("id = ?", job.ID).Updates(next).Error
}

// UnfinishedJobCount 稿件尚未完成（待处理/处理中）的任务数。
func (r *Repo) UnfinishedJobCount(videoID int64) (int64, error) {
	var cnt int64
	err := r.db.Model(&TranscodeJob{}).
		Where("video_id = ? AND status IN ?", videoID, []int8{JobPending, JobRunning}).
		Count(&cnt).Error
	return cnt, err
}

func (r *Repo) AddStream(st *Stream) error {
	return r.db.Create(st).Error
}

// UpdateVideoFields 按字段更新稿件（转码回写时长/封面/状态用）。
func (r *Repo) UpdateVideoFields(id int64, fields map[string]any) error {
	return r.db.Model(&Video{}).Where("id = ?", id).Updates(fields).Error
}

// FindPublishedByIDs 批量查已发布稿件。
func (r *Repo) FindPublishedByIDs(ids []int64) ([]Video, error) {
	var list []Video
	err := r.db.Where("id IN ? AND status = ?", ids, StatusPublished).Find(&list).Error
	return list, err
}

// ListByStatus 按状态分页（审核队列用，提交时间升序）。
func (r *Repo) ListByStatus(status int8, page, size int) ([]Video, int64, error) {
	q := r.db.Model(&Video{}).Where("status = ?", status)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []Video
	err := q.Order("risk_level DESC, created_at").Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, total, err
}

// SoftDelete 软删除（乐观锁）。
func (r *Repo) SoftDelete(v *Video) error {
	res := r.db.Model(&Video{}).
		Where("id = ? AND version = ?", v.ID, v.Version).
		Updates(map[string]any{"status": StatusDeleted, "version": v.Version + 1})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("并发冲突，请重试")
	}
	return nil
}
