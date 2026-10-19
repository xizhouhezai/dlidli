// dlidli-worker 独立转码 Worker 入口（转码积压治理卡）。
//
// 为什么需要独立进程：
//   - API 进程重启会打断转码并留下租约未过期的僵尸任务（虽有回收兜底，但重启越频繁越糟）；
//   - 水平扩容的最小单位应该是**进程**，而不是把单进程的协程数调大——后者受 CPU 核数限制，
//     且与 API 共享 GC/连接池，扩容时会把 API 一起拖慢。
//
// 与 API 内嵌模式（transcode.embedInAPI=true）的关系：共用同一套配置语义
// （transcode.workers / lease / maxRetries / retryBackoff / pollInterval / jobTimeout），
// 共用同一个认领实现（repo.ClaimJob），因此两种模式可以**同时**跑而不会重复处理：
// 认领靠数据库行锁 + SKIP LOCKED + 条件 UPDATE + 租约，进程数再多也只有一个赢家。
//
// 用法：
//
//	go run ./cmd/worker -config configs/dev.yaml -workers 2 -id-prefix w1
//	DLIDLI_TRANSCODE_WORKERS=4 ./dlidli-worker -config configs/prod.yaml
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/dlidli/server/internal/infra"
	"github.com/dlidli/server/internal/module/video"
	"github.com/dlidli/server/internal/pkg/config"
	"github.com/dlidli/server/internal/pkg/logger"
	"github.com/dlidli/server/internal/pkg/snowflake"
	"go.uber.org/zap"
)

func main() {
	configPath := flag.String("config", "configs/dev.yaml", "配置文件路径（与 API 共用同一份）")
	workers := flag.Int("workers", 0, "覆盖 transcode.workers（0=用配置文件的值）")
	idPrefix := flag.String("id-prefix", "", "Worker 标识前缀（默认 hostname/pid）；同机多进程请显式区分")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}
	if *workers > 0 {
		cfg.Transcode.Workers = *workers
	}

	log, err := logger.New(cfg.Log.Level, cfg.App.Env)
	if err != nil {
		fmt.Fprintf(os.Stderr, "初始化日志失败: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = log.Sync() }()

	// 独立 Worker 不发布任何对外接口，也不需要 HTTP 端口；
	// 但雪花 ID 与 API 一致初始化，避免转码回写里任何用到 ID 的路径 panic。
	if err := snowflake.Init(2); err != nil { // 节点号与 API(1) 区分，避免跨进程 ID 相撞
		log.Fatal("初始化 ID 生成器失败", zap.Error(err))
	}

	res := infra.Init(cfg, log)
	defer res.Close()
	if res.DB == nil {
		log.Fatal("MySQL 不可用：转码队列在 DB 中，独立 Worker 无法运行")
	}

	svc := video.NewService(video.NewRepo(res.DB), nil, nil, nil, res.Redis, cfg, log)
	svc.SetStorage(res.Storage)
	// 档位配置是转码参数的来源；缺表/读失败不阻断启动（转码侧对空档位会按"未知档位"失败进死信，
	// 而不是让整个 Worker 起不来——起不来才是真积压）。
	if set, err := svc.LoadTiers(); err != nil {
		log.Warn("清晰度档位配置加载失败，转码参数可能不全", zap.Error(err))
	} else {
		svc.SetTierSet(set)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 退出信号：优雅停机。收到信号后停止认领新任务，
	// 正在跑的任务最多跑到 JobTimeout；未跑完的任务租约过期后被其他实例回收。
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-quit
		log.Info("收到退出信号，停止认领新任务（在跑任务由租约兜底回收）")
		cancel()
	}()

	log.Info("独立转码 Worker 启动",
		zap.String("config", *configPath),
		zap.Int("workers", cfg.Transcode.Workers),
		zap.Duration("lease", cfg.Transcode.Lease),
		zap.Int("max_retries", cfg.Transcode.MaxRetries),
		zap.Int("max_duration_seconds", cfg.Transcode.MaxDurationSeconds),
	)

	// 独立 Worker 自带的指标端点（默认关闭）：水平扩容后每个实例都要被 Prometheus 抓到，
	// 否则 /metrics 只有一个实例的数据，"排队多少、正在转几个"会被任何一次抓取带偏。
	// 与 API 的 /metrics 共用同一个 prometheus 注册表实现，指标名完全一致，可直接相加。
	if cfg.Transcode.MetricsAddr != "" {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		metricsSrv := &http.Server{Addr: cfg.Transcode.MetricsAddr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
		go func() {
			log.Info("Worker 指标端点已启动", zap.String("addr", cfg.Transcode.MetricsAddr))
			if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Error("指标端点启动失败", zap.Error(err))
			}
		}()
		defer func() {
			sctx, scancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer scancel()
			_ = metricsSrv.Shutdown(sctx)
		}()
	}
	svc.StartQueueMetricsRefresher(ctx, 15*time.Second)

	if err := svc.RunTranscodeWorkers(ctx, res.Storage, *idPrefix); err != nil {
		log.Fatal("转码 Worker 启动失败", zap.Error(err))
	}

	<-ctx.Done()
	// 给在跑的任务一点收尾时间（超时兜底在上层 JobTimeout，这里只是让日志落盘）。
	time.Sleep(200 * time.Millisecond)
	log.Info("独立转码 Worker 已退出")
}
