package video

import (
	"context"
	"time"

	"github.com/dlidli/server/internal/pkg/metrics"
	"go.uber.org/zap"
)

// 积压快照（转码积压治理卡）：回答「当前排队多少、正在转几个、最老的等了多久、平均耗时多少」。
//
// 两个载体并存，理由不同：
//   - Prometheus 指标（/metrics）：给监控/告警与趋势看，多实例各自上报、由 Prometheus 聚合；
//   - 本结构体（管理端接口）：给运营一次性看全貌，"最老等待"这类**当前瞬时值**在
//     多实例各自上报 Gauge 时会被最后上报者覆盖，单点快照反而更准。
type QueueSnapshot struct {
	Pending int64 `json:"pending"` // 排队中（可认领）
	Running int64 `json:"running"` // 正在转码
	Success int64 `json:"success"` // 历史成功
	Failed  int64 `json:"failed"`  // 历史终态失败（死信，已达重试上限）

	// OldestPendingSeconds 最老待处理任务已等待秒数；无待处理任务时为 0。
	OldestPendingSeconds int64 `json:"oldest_pending_seconds"`
	// OldestPendingJobID 最老的待处理任务 ID（排障直接可查）；无则为 0。
	OldestPendingJobID int64 `json:"oldest_pending_job_id"`
	// RunningOverdue 租约已过期但仍在"处理中"的任务数：
	// >0 意味着有 Worker 崩溃/被 kill 且尚未被回收——这是积压的真正来源，必须告警。
	RunningOverdue int64 `json:"running_overdue"`

	// AvgDurationSeconds 已成功任务的平均耗时（秒），由 SUM(finished_at-started_at)/COUNT 得到。
	AvgDurationSeconds float64 `json:"avg_duration_seconds"`

	UpdatedAt time.Time `json:"updated_at"`
}

// StartQueueMetricsRefresher 周期把 DB 队列口径刷进 Prometheus 指标。
//
// 为什么需要它：任务失败/崩溃回收不会经过"任务结束"这条路径，只靠 Worker 在
// 任务边界喂值，异常积压恰恰不会及时反映到 /metrics。周期刷新保证"看到的就是现在的"。
// 刷新本身是 4 条只读聚合查询（都走 idx_claim/idx_lease），15s 一次的代价可忽略。
func (s *Service) StartQueueMetricsRefresher(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 15 * time.Second
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				s.log.Error("队列指标刷新协程 panic，已隔离", zap.Any("recover", r))
			}
		}()
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if _, err := s.QueueSnapshot(); err != nil {
					s.log.Debug("刷新转码队列指标失败", zap.Error(err))
				}
			}
		}
	}()
}

// QueueSnapshot 读取当前转码队列快照（同时刷新 Prometheus 指标）。
// 管理端接口与 Worker 轮询共用：任何一次查询都会把指标推到最新，避免指标滞后。
func (s *Service) QueueSnapshot() (*QueueSnapshot, error) {
	snap, err := s.repo.QueueSnapshot()
	if err != nil {
		return nil, err
	}
	metrics.SetTranscodeQueue(snap.Pending, snap.Running, snap.Success, snap.Failed, float64(snap.OldestPendingSeconds))
	if snap.RunningOverdue > 0 {
		// 回收动作是幂等的（条件 UPDATE），查询路径顺手回收：
		// 保证"看到积压"与"积压被清理"之间没有额外的时间窗。
		if n, err := s.repo.ReclaimExpired(); err == nil && n > 0 {
			metrics.AddTranscodeReclaimed(n)
			s.log.Warn("回收过期租约的转码任务", zap.Int64("count", n))
		}
	}
	return snap, nil
}

// QueueSnapshot 一次查询出全部积压口径，避免 4 次往返。
// 时间口径：
//   - 最老等待 = now - MIN(created_at)（pending 行；退避中的任务用 next_retry_at 会更准，
//     但 created_at 是用户真正感知的"我投稿后等了多久"，故选它）；
//   - 平均耗时 = AVG(finished_at - started_at)（仅成功行；failed 行耗时无产品意义）。
func (r *Repo) QueueSnapshot() (*QueueSnapshot, error) {
	snap := &QueueSnapshot{UpdatedAt: time.Now()}

	type statusCount struct {
		Status int8
		Cnt    int64
	}
	var counts []statusCount
	if err := r.db.Model(&TranscodeJob{}).
		Select("status, COUNT(*) AS cnt").Group("status").Scan(&counts).Error; err != nil {
		return nil, err
	}
	for _, c := range counts {
		switch c.Status {
		case JobPending:
			snap.Pending = c.Cnt
		case JobRunning:
			snap.Running = c.Cnt
		case JobSuccess:
			snap.Success = c.Cnt
		case JobFailed:
			snap.Failed = c.Cnt
		}
	}

	var oldest struct {
		ID        int64
		CreatedAt *time.Time
	}
	if err := r.db.Model(&TranscodeJob{}).
		Select("id, created_at").Where("status = ?", JobPending).
		Order("id").Limit(1).Scan(&oldest).Error; err != nil {
		return nil, err
	}
	if oldest.CreatedAt != nil {
		snap.OldestPendingJobID = oldest.ID
		if d := time.Since(*oldest.CreatedAt); d > 0 {
			snap.OldestPendingSeconds = int64(d.Seconds())
		}
	}

	if err := r.db.Model(&TranscodeJob{}).
		Where("status = ? AND lease_until IS NOT NULL AND lease_until < ?", JobRunning, time.Now()).
		Count(&snap.RunningOverdue).Error; err != nil {
		return nil, err
	}

	var agg struct {
		Avg float64
	}
	if err := r.db.Model(&TranscodeJob{}).
		Select("COALESCE(AVG(TIMESTAMPDIFF(SECOND, started_at, finished_at)), 0) AS avg").
		Where("status = ? AND started_at IS NOT NULL AND finished_at IS NOT NULL", JobSuccess).
		Scan(&agg).Error; err != nil {
		return nil, err
	}
	snap.AvgDurationSeconds = agg.Avg

	return snap, nil
}
