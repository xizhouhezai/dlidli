package video

import (
	"errors"
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

// CreateWithParts 多P投稿事务：稿件 + 统计 + 各分P原画流 + 转码任务 + video_part 记录。
func (r *Repo) CreateWithParts(v *Video, parts []VideoPart, streams []Stream, jobQualities []int16) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(v).Error; err != nil {
			return err
		}
		if err := tx.Create(&Stat{VideoID: v.ID}).Error; err != nil {
			return err
		}
		for i := range streams {
			streams[i].VideoID = v.ID
			if err := tx.Create(&streams[i]).Error; err != nil {
				return err
			}
		}
		for i := range parts {
			parts[i].VideoID = v.ID
			if err := tx.Create(&parts[i]).Error; err != nil {
				return err
			}
		}
		for _, q := range jobQualities {
			for _, p := range parts {
				if err := tx.Create(&TranscodeJob{VideoID: v.ID, PartIndex: p.PartIndex, Quality: q}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// PartsByVideo 稿件分P列表（按 part_index）。
func (r *Repo) PartsByVideo(videoID int64) ([]VideoPart, error) {
	list := make([]VideoPart, 0)
	err := r.db.Where("video_id = ?", videoID).Order("part_index").Find(&list).Error
	return list, err
}

// StreamsByVideoPart 指定分P的流（按质量升序）。
func (r *Repo) StreamsByVideoPart(videoID int64, partIndex int8) ([]Stream, error) {
	list := make([]Stream, 0)
	err := r.db.Where("video_id = ? AND part_index = ?", videoID, partIndex).Order("quality").Find(&list).Error
	return list, err
}

// UpdatePartDuration 更新分P时长（转码探帧后）。
func (r *Repo) UpdatePartDuration(videoID int64, partIndex int8, duration int) error {
	return r.db.Model(&VideoPart{}).
		Where("video_id = ? AND part_index = ?", videoID, partIndex).
		Update("duration", duration).Error
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

// retryBackoff 第 n 次失败（n 从 1 起）后的退避时长：base * 2^(n-1)，上限 1 小时。
// 退避不是"慢一点"，而是防止坏任务在被回收的瞬间被其他 Worker 再次抢走，
// 把"多实例并发"变成"多个实例轮流撞同一个坏文件"的热循环（积压只增不减的主因之一）。
func retryBackoff(base time.Duration, n int) time.Duration {
	if n < 1 {
		n = 1
	}
	if n > 6 { // 2^6=64 倍，封顶避免长尾
		n = 6
	}
	d := base * time.Duration(1<<uint(n-1))
	if d > time.Hour {
		d = time.Hour
	}
	return d
}

// ClaimJob 原子认领一个可处理任务；无任务返回 (nil, nil)。
//
// 并发安全的三道保证（水平扩容的技术核心）：
//  1. **行锁 + SKIP LOCKED**：事务内 SELECT ... FOR UPDATE SKIP LOCKED，两个 Worker 同时取时
//     第二个直接跳过（不阻塞、不等待）；MySQL 8 原生支持。
//  2. **条件 UPDATE 兜底**：认领写回带 status=0 条件并校验 RowsAffected，即使行锁语义被
//     隔离级别削弱（例如外部把隔离级别调低），也只会有一条 UPDATE 生效。
//  3. **租约过期回收**：WHERE 里额外放开「status=1 但 lease_until 已过期」的行，
//     使崩溃进程留下的僵尸任务能回到队列，且回收同样经由行锁 + 条件 UPDATE 去重。
//
// workerID 由调用方提供（hostname/pid/协程号），落库后可直接回答"这个任务被谁在处理"。
func (r *Repo) ClaimJob(workerID string, lease time.Duration) (*TranscodeJob, error) {
	if lease <= 0 {
		lease = 10 * time.Minute
	}
	var job TranscodeJob
	err := r.db.Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		claimable := "status = ? AND (next_retry_at IS NULL OR next_retry_at <= ?) OR (status = ? AND lease_until IS NOT NULL AND lease_until < ?)"
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where(claimable, JobPending, now, JobRunning, now).
			Order("id").First(&job).Error; err != nil {
			return err
		}
		leaseUntil := now.Add(lease)
		res := tx.Model(&TranscodeJob{}).
			Where("id = ? AND status = ?", job.ID, job.Status).
			Updates(map[string]any{
				"status":        JobRunning,
				"lease_until":   leaseUntil,
				"worker_id":     workerID,
				"started_at":    now,
				"next_retry_at": nil,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			// 被别的 Worker 抢先（理论上 SKIP LOCKED 已挡住，这里是兜底）：
			// 返回哨兵错误让调用方重试下一轮，绝不能把同一个任务处理两次。
			return errClaimLost
		}
		job.Status = JobRunning
		job.LeaseUntil = &leaseUntil
		job.WorkerID = workerID
		job.StartedAt = &now
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, errClaimLost) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &job, nil
}

// ceilSecond 向上取整到秒。
//
// 为什么需要：next_retry_at 落的是 DATETIME(0)（秒级精度，MySQL 对小数部分是"四舍五入"，
// 即 22.35s 会被存成 22s）。不向上取整的话，配置一个 <1.5s 的退避会被静默抹平——
// 退避期形同不存在，坏任务照样被多实例反复抢。向上取整保证落库值**不早于**
// 配置的退避时刻（代价是退避可能被延长不到 1 秒）。
func ceilSecond(t time.Time) time.Time {
	if trunc := t.Truncate(time.Second); trunc.Before(t) {
		return trunc.Add(time.Second)
	}
	return t
}

// errClaimLost 认领时条件 UPDATE 未命中（任务已被他人认领），视为"队列暂时没有可认领任务"。
var errClaimLost = errors.New("claim lost")

// RenewLease 续租。返回 (false, nil) 表示租约已不属于自己（被别人回收），
// 调用方必须立刻停止转码并丢弃产物，否则会与回收方重复处理同一任务。
func (r *Repo) RenewLease(id int64, workerID string, lease time.Duration) (bool, error) {
	res := r.db.Model(&TranscodeJob{}).
		Where("id = ? AND status = ? AND worker_id = ?", id, JobRunning, workerID).
		Update("lease_until", time.Now().Add(lease))
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

// ReclaimExpired 把租约已过期且仍在"处理中"的任务放回队列（进程崩溃/被 kill 的兜底）。
// 不单独维护后台清扫协程：认领语句本身就接纳过期租约（见 ClaimJob），
// 这里的显式回收只用于统计"回收了多少次"，供指标/排障观察。
func (r *Repo) ReclaimExpired() (int64, error) {
	res := r.db.Model(&TranscodeJob{}).
		Where("status = ? AND lease_until IS NOT NULL AND lease_until < ?", JobRunning, time.Now()).
		Updates(map[string]any{"status": JobPending, "lease_until": nil, "worker_id": ""})
	return res.RowsAffected, res.Error
}

// CompleteJob 成功终态：清租约、记终态时间。
func (r *Repo) CompleteJob(id int64, workerID string) error {
	now := time.Now()
	return r.db.Model(&TranscodeJob{}).
		Where("id = ? AND status = ? AND worker_id = ?", id, JobRunning, workerID).
		Updates(map[string]any{
			"status":      JobSuccess,
			"error_msg":   "",
			"lease_until": nil,
			"finished_at": now,
		}).Error
}

// FailJob 失败处理：未超重试上限则带退避重新入队，否则进终态 failed（死信）。
// 返回是否已进入终态，供调用方区分"还会重试"与"这个任务到此为止"。
func (r *Repo) FailJob(job *TranscodeJob, msg string, maxRetries int, backoffBase time.Duration) (bool, error) {
	if len(msg) > 500 {
		msg = msg[:500]
	}
	now := time.Now()
	n := int(job.RetryCnt) + 1
	next := map[string]any{
		"error_msg":   msg,
		"retry_cnt":   job.RetryCnt + 1,
		"lease_until": nil,
		"worker_id":   "",
	}
	terminal := n > maxRetries
	if terminal {
		next["status"] = JobFailed
		next["finished_at"] = now
		next["next_retry_at"] = nil
	} else {
		backoff := retryBackoff(backoffBase, n)
		next["status"] = JobPending
		next["next_retry_at"] = ceilSecond(now.Add(backoff))
	}
	if err := r.db.Model(&TranscodeJob{}).Where("id = ?", job.ID).Updates(next).Error; err != nil {
		return terminal, err
	}
	job.RetryCnt = job.RetryCnt + 1
	return terminal, nil
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

// SetStreamHeight 回填某条流的实际画面高度（M4-VIP-02）。
// 用于转码时把**原画行**的真实分辨率写库——原画直出源文件，其高度决定免费用户
// 能否拿到该流（见 qualitytier 包注释）。
func (r *Repo) SetStreamHeight(videoID int64, partIndex int8, quality int16, height int) error {
	return r.db.Model(&Stream{}).
		Where("video_id = ? AND part_index = ? AND quality = ?", videoID, partIndex, quality).
		Update("height", height).Error
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

// ListAdmin 管理端全状态列表（稿件管理用）：可选状态/分区/标题关键词筛选，排除已删除，新→旧。
func (r *Repo) ListAdmin(categoryID int, status int8, keyword string, page, size int) ([]Video, int64, error) {
	q := r.db.Model(&Video{}).Where("status <> ?", StatusDeleted)
	if status > 0 {
		q = q.Where("status = ?", status)
	}
	if categoryID > 0 {
		q = q.Where("category_id = ?", categoryID)
	}
	if keyword != "" {
		q = q.Where("title LIKE ?", "%"+keyword+"%")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []Video
	err := q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error
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
