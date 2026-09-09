package searchindex

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// DocLoader 加载稿件检索文档（由 video 域实现，避免 searchindex 依赖具体存储）。
type DocLoader interface {
	// IndexDoc 返回稿件文档；稿件不存在或未发布返回 (nil, nil)。
	IndexDoc(ctx context.Context, videoID int64) (*Doc, error)
}

// OutboxRepo Outbox 数据访问（便于测试注入 Fake）。
type OutboxRepo interface {
	Enqueue(ctx context.Context, videoID int64, action string) error
	PendingBatch(ctx context.Context, limit int) ([]Outbox, error)
	MarkDone(ctx context.Context, ids []int64) error
	MarkFailed(ctx context.Context, ids []int64) error
	IncrAttempts(ctx context.Context, id int64) (int, error)
}

// Service 搜索索引同步服务：登记变更 + 后台 Worker 消费 Outbox 推送 ES + 检索入口。
type Service struct {
	repo        OutboxRepo
	indexer     Indexer
	loader      DocLoader
	log         *zap.Logger
	maxAttempts int
}

func NewService(repo OutboxRepo, indexer Indexer, loader DocLoader, log *zap.Logger) *Service {
	return &Service{repo: repo, indexer: indexer, loader: loader, log: log, maxAttempts: 5}
}

// Enabled ES 是否启用（未启用时搜索走 MySQL LIKE 兜底）。
func (s *Service) Enabled() bool { return s.indexer.IsEnabled() }

// Enqueue 登记一次索引变更（供稿件发布/下架/删除旁路调用）。
// ES 未启用时直接跳过，不产生 Outbox 记录。
func (s *Service) Enqueue(ctx context.Context, videoID int64, action string) error {
	if !s.indexer.IsEnabled() {
		return nil
	}
	if action != ActionUpsert && action != ActionDelete {
		return fmt.Errorf("未知索引动作: %s", action)
	}
	return s.repo.Enqueue(ctx, videoID, action)
}

// SearchVideos 走 ES 检索视频；ES 异常时返回错误，由调用方降级 MySQL LIKE。
func (s *Service) SearchVideos(ctx context.Context, keyword string, page, size int) ([]int64, int64, error) {
	return s.indexer.Search(ctx, keyword, page, size)
}

// StartWorker 启动 Outbox 消费 Worker。ES 未启用时不启动（仅日志）。
func (s *Service) StartWorker(ctx context.Context, interval time.Duration) {
	if !s.indexer.IsEnabled() {
		s.log.Info("Elasticsearch 未启用，跳过索引同步 Worker（搜索走 MySQL LIKE 兜底）")
		return
	}
	if interval <= 0 {
		interval = 3 * time.Second
	}
	if err := s.indexer.EnsureIndex(ctx); err != nil {
		// 不阻塞启动：索引未就绪时 Worker 每轮重试
		s.log.Warn("ES 索引初始化失败，Worker 将继续重试", zap.Error(err))
	}
	go func() {
		s.drain(ctx)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				s.log.Info("搜索索引同步 Worker 退出")
				return
			case <-ticker.C:
				s.drain(ctx)
			}
		}
	}()
	s.log.Info("搜索索引同步 Worker 已启动", zap.Duration("interval", interval))
}

// drain 消费一批待处理记录；成功标记完成，失败自增重试次数，超限标记失败。
func (s *Service) drain(ctx context.Context) {
	batch, err := s.repo.PendingBatch(ctx, 100)
	if err != nil {
		s.log.Error("读取 Outbox 失败", zap.Error(err))
		return
	}
	if len(batch) == 0 {
		return
	}
	var done, failed []int64
	for _, o := range batch {
		if err := s.syncOne(ctx, o); err != nil {
			s.log.Warn("索引同步失败",
				zap.Int64("outbox", o.ID), zap.Int64("video", o.VideoID),
				zap.String("action", o.Action), zap.Error(err))
			n, err2 := s.repo.IncrAttempts(ctx, o.ID)
			if err2 != nil {
				s.log.Error("Outbox 重试计数失败", zap.Int64("outbox", o.ID), zap.Error(err2))
				continue
			}
			if n >= s.maxAttempts {
				failed = append(failed, o.ID)
			}
			continue
		}
		done = append(done, o.ID)
	}
	if len(done) > 0 {
		if err := s.repo.MarkDone(ctx, done); err != nil {
			s.log.Error("Outbox 标记完成失败", zap.Error(err))
		}
	}
	if len(failed) > 0 {
		if err := s.repo.MarkFailed(ctx, failed); err != nil {
			s.log.Error("Outbox 标记失败失败", zap.Error(err))
		}
	}
}

// syncOne 同步单条：upsert 需加载文档（未发布则幂等删除）；delete 直接删。
func (s *Service) syncOne(ctx context.Context, o Outbox) error {
	switch o.Action {
	case ActionUpsert:
		doc, err := s.loader.IndexDoc(ctx, o.VideoID)
		if err != nil {
			return err
		}
		if doc == nil {
			// 稿件已不存在/未发布 → 幂等删除残留文档
			return s.indexer.Delete(ctx, o.VideoID)
		}
		return s.indexer.Upsert(ctx, *doc)
	case ActionDelete:
		return s.indexer.Delete(ctx, o.VideoID)
	default:
		return fmt.Errorf("未知动作: %s", o.Action)
	}
}
