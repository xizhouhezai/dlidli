package admin

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// CountByDay 通用按日计数聚合（表/时间列/统计表达式 → map[日期]计数）。
// 仅内部白名单使用，table/column 来自本模块硬编码调用，不接受外部输入。
func (r *Repo) CountByDay(table, column, from, expr string) (map[string]int64, error) {
	type row struct {
		Date string
		N    int64
	}
	var list []row
	err := r.db.Table(table).
		Select("DATE_FORMAT("+column+", '%Y-%m-%d') AS date, "+expr+" AS n").
		Where(column+" >= ?", from).
		Group("date").
		Scan(&list).Error
	if err != nil {
		return nil, err
	}
	m := make(map[string]int64, len(list))
	for _, it := range list {
		m[it.Date] = it.N
	}
	return m, nil
}

// CountBehaviorByDay 近 N 日有效播放按日计数（user_behavior action=3）。
func (r *Repo) CountBehaviorByDay(_ context.Context, from string) (map[string]int64, error) {
	type row struct {
		Date string
		N    int64
	}
	var list []row
	err := r.db.Table("user_behavior").
		Select("DATE_FORMAT(created_at, '%Y-%m-%d') AS date, COUNT(*) AS n").
		Where("action = 3 AND created_at >= ?", from).
		Group("date").
		Scan(&list).Error
	if err != nil {
		return nil, err
	}
	m := make(map[string]int64, len(list))
	for _, it := range list {
		m[it.Date] = it.N
	}
	return m, nil
}

// AvgReviewHours 近 7 日审核通过稿件的平均审核耗时（小时）。
func (r *Repo) AvgReviewHours(_ context.Context) (float64, error) {
	var avg float64
	err := r.db.Table("video").
		Select("COALESCE(AVG(TIMESTAMPDIFF(HOUR, created_at, published_at)), 0)").
		Where("status = 4 AND published_at >= ?", time.Now().AddDate(0, 0, -6)).
		Scan(&avg).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return 0, err
	}
	return avg, nil
}

// CountPendingReview 待审稿件数。
func (r *Repo) CountPendingReview(_ context.Context) (int64, error) {
	var n int64
	err := r.db.Table("video").Where("status = 3").Count(&n).Error
	return n, err
}
