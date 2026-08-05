package admin

import (
	"context"
	"time"
)

// DashboardStats 数据大盘（M3-OPS-02）：全站实时聚合，本地务实版不做 T+1 数仓。
type DashboardStats struct {
	Today DashboardToday `json:"today"`
	Trend []DashboardTrend `json:"trend"`
	// ReviewHours 近 7 日审核通过稿件平均审核耗时（小时，0 表示无数据）
	ReviewHours float64 `json:"review_hours"`
	// PendingReview 待审稿件数
	PendingReview int64 `json:"pending_review"`
}

// DashboardToday 今日实时指标。
type DashboardToday struct {
	Dau      int64 `json:"dau"`       // 今日活跃用户（行为去重）
	NewUsers int64 `json:"new_users"` // 今日注册
	Uploads  int64 `json:"uploads"`   // 今日投稿
	Views    int64 `json:"views"`     // 今日有效播放
}

// DashboardTrend 近 7 日趋势点。
type DashboardTrend struct {
	Date     string `json:"date"`
	Dau      int64  `json:"dau"`
	NewUsers int64  `json:"new_users"`
	Uploads  int64  `json:"uploads"`
	Views    int64  `json:"views"`
}

// DashboardStats 数据大盘（全站：活跃/新增/投稿/播放 + 审核时效）。
func (s *Service) DashboardStats(ctx context.Context) (*DashboardStats, error) {
	from := time.Now().AddDate(0, 0, -6).Format("2006-01-02")

	dauMap, err := s.repo.CountByDay("user_behavior", "created_at", from, "COUNT(DISTINCT user_id)")
	if err != nil {
		return nil, err
	}
	newUserMap, err := s.repo.CountByDay("user", "created_at", from, "COUNT(*)")
	if err != nil {
		return nil, err
	}
	uploadMap, err := s.repo.CountByDay("video", "created_at", from, "COUNT(*)")
	if err != nil {
		return nil, err
	}
	viewMap, err := s.repo.CountBehaviorByDay(ctx, from)
	if err != nil {
		return nil, err
	}
	reviewHours, err := s.repo.AvgReviewHours(ctx)
	if err != nil {
		return nil, err
	}
	pending, err := s.repo.CountPendingReview(ctx)
	if err != nil {
		return nil, err
	}

	out := &DashboardStats{ReviewHours: reviewHours, PendingReview: pending}
	// 补零对齐近 7 日（含今天）
	for i := 6; i >= 0; i-- {
		d := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		t := DashboardTrend{Date: d[5:]}
		if i == 0 {
			out.Today = DashboardToday{
				Dau: dauMap[d], NewUsers: newUserMap[d], Uploads: uploadMap[d], Views: viewMap[d],
			}
		}
		t.Dau, t.NewUsers, t.Uploads, t.Views = dauMap[d], newUserMap[d], uploadMap[d], viewMap[d]
		out.Trend = append(out.Trend, t)
	}
	return out, nil
}
