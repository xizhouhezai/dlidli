// dlidli-api 核心业务 API 服务入口。
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

	"github.com/dlidli/server/internal/infra"
	"github.com/dlidli/server/internal/pkg/config"
	"github.com/dlidli/server/internal/pkg/logger"
	"github.com/dlidli/server/internal/pkg/snowflake"
	"github.com/dlidli/server/internal/router"
	"go.uber.org/zap"
)

func main() {
	configPath := flag.String("config", "configs/dev.yaml", "配置文件路径")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	log, err := logger.New(cfg.Log.Level, cfg.App.Env)
	if err != nil {
		fmt.Fprintf(os.Stderr, "初始化日志失败: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = log.Sync() }()

	if err := snowflake.Init(1); err != nil {
		log.Fatal("初始化 ID 生成器失败", zap.Error(err))
	}

	res := infra.Init(cfg, log)
	defer res.Close()

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.App.Port),
		Handler:           router.New(cfg, log, res),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Info("api 服务启动",
			zap.String("app", cfg.App.Name),
			zap.String("env", cfg.App.Env),
			zap.Int("port", cfg.App.Port),
		)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("服务启动失败", zap.Error(err))
		}
	}()

	// 优雅退出
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("收到退出信号，开始优雅关闭...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("优雅关闭超时", zap.Error(err))
	}
	log.Info("服务已退出")
}
