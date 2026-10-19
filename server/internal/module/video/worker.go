package video

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/dlidli/server/internal/pkg/metrics"
	"github.com/dlidli/server/internal/pkg/storage"
	"go.uber.org/zap"
)

// 转码 Worker 运行器（转码积压治理卡）。
//
// 水平扩容模型：**进程级扩容，不依赖进程内协程数**。
// 每个进程跑 cfg.Transcode.Workers 个协程，N 个进程 → N*Workers 个并发消费者，
// 全部经 repo.ClaimJob 的原子认领取任务（行锁 + SKIP LOCKED + 条件 UPDATE + 租约），
// 因此"扩容"只是多起几个进程，不需要任何协调组件。
//
// 与前缀的兼容：StartTranscodeWorkers(ctx, store) 签名与行为不变（API 内嵌 dev 模式），
// 内部改为复用同一套运行器，避免 dev 与部署两套逻辑漂移。
const (
	// leaseRenewDivisor 续租周期 = Lease/N：留出 N 次续租机会，单次续租抖动不会导致误回收。
	leaseRenewDivisor = 3
	// 空闲轮询的下限：配置写成 0 时会回落到 3s（见 config.Load），这里再兜一次底。
	minPollInterval = 500 * time.Millisecond
)

// WorkerID 生成本进程稳定的持有者标识：hostname/pid（同机多进程可区分）。
// 一个进程内所有协程共享该前缀，协程号由调用方追加，便于在 DB 中直接看出"谁在处理"。
func WorkerID() (string, error) {
	host, err := os.Hostname()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%d", host, os.Getpid()), nil
}

// StartTranscodeWorkers 启动进程内转码 Worker（dev 便利；部署环境改由独立 cmd/worker 运行）。
// 依赖本地存储（storage.PathResolver）；其他驱动下直接告警退出。
func (s *Service) StartTranscodeWorkers(ctx context.Context, store storage.Storage) {
	if err := s.RunTranscodeWorkers(ctx, store, ""); err != nil {
		s.log.Warn("转码 Worker 未启动", zap.Error(err))
	}
}

// RunTranscodeWorkers 阻塞到 ctx 结束；idPrefix 为空时按 hostname/pid 生成。
// cmd/worker 与 API 内嵌模式共用本方法——**同一份认领与重试语义**，避免两套代码漂移。
func (s *Service) RunTranscodeWorkers(ctx context.Context, store storage.Storage, idPrefix string) error {
	resolver, ok := store.(storage.PathResolver)
	if !ok {
		return fmt.Errorf("当前存储驱动不支持本地转码")
	}
	if idPrefix == "" {
		id, err := WorkerID()
		if err != nil {
			return fmt.Errorf("生成 worker 标识失败: %w", err)
		}
		idPrefix = id
	}
	n := s.cfg.Transcode.Workers
	if n <= 0 {
		n = 1
	}
	for i := 0; i < n; i++ {
		go s.workerLoop(ctx, resolver, fmt.Sprintf("%s/w%d", idPrefix, i), i)
	}
	s.log.Info("转码 Worker 已启动",
		zap.Int("workers", n),
		zap.String("worker_id", idPrefix),
		zap.Duration("poll", s.pollInterval()),
		zap.Duration("lease", s.cfg.Transcode.Lease),
		zap.Int("max_retries", s.cfg.Transcode.MaxRetries),
	)
	return nil
}

func (s *Service) pollInterval() time.Duration {
	if s.cfg.Transcode.PollInterval < minPollInterval {
		return minPollInterval
	}
	return s.cfg.Transcode.PollInterval
}

func (s *Service) workerLoop(ctx context.Context, resolver storage.PathResolver, workerID string, id int) {
	ticker := time.NewTicker(s.pollInterval())
	defer ticker.Stop()
	for {
		// 启动即尝试一轮：不等第一个 tick，缩短"服务刚起来队列满着却干等 3s"的空窗。
		s.drainQueue(ctx, resolver, workerID, id)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// drainQueue 连续认领直到队列空或 ctx 结束。
func (s *Service) drainQueue(ctx context.Context, resolver storage.PathResolver, workerID string, id int) {
	for {
		if ctx.Err() != nil {
			return
		}
		job, err := s.repo.ClaimJob(workerID, s.cfg.Transcode.Lease)
		if err != nil {
			// 认领失败（DB 抖动等）不退出循环，但要退避，避免把日志刷爆。
			s.log.Error("认领转码任务失败", zap.Error(err), zap.String("worker_id", workerID))
			return
		}
		if job == nil {
			s.refreshQueueMetrics()
			return // 队列空，等下个周期
		}
		s.processJob(ctx, resolver, job, id, workerID)
	}
}

// refreshQueueMetrics 队列空/满之间刷新一次指标，使积压曲线在无流量时也不会陈旧。
func (s *Service) refreshQueueMetrics() {
	if _, err := s.QueueSnapshot(); err != nil {
		s.log.Debug("刷新转码队列指标失败", zap.Error(err))
	}
}

// startLeaseKeeper 周期续租；返回 stop 用于任务结束。
// 若续租发现"租约已不属于自己"（被回收），立即 cancel 任务 ctx：
// 此时产物已不可信（另一 Worker 可能已在写同一路径），必须尽快让出，避免重复处理同一任务。
func (s *Service) startLeaseKeeper(ctx context.Context, cancel context.CancelFunc, job *TranscodeJob, workerID string) (stop func()) {
	lease := s.cfg.Transcode.Lease
	if lease <= 0 {
		return func() {}
	}
	interval := lease / leaseRenewDivisor
	if interval < time.Second {
		interval = time.Second
	}
	done := make(chan struct{})
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-t.C:
				ok, err := s.repo.RenewLease(job.ID, workerID, lease)
				if err != nil {
					// 续租出错不代表失去租约（DB 抖动），下个周期再试；
					// 只要 Lease 足够长（默认 10min，远超 3 次续租抖动），不会误回收。
					s.log.Warn("续租转码任务失败", zap.Error(err), zap.Int64("job_id", job.ID))
					continue
				}
				if !ok {
					metrics.IncTranscodeLeaseLost()
					s.log.Warn("转码任务租约已丢失，放弃本次处理",
						zap.Int64("job_id", job.ID), zap.String("worker_id", workerID))
					cancel()
					return
				}
			}
		}
	}()
	return func() { close(done) }
}
