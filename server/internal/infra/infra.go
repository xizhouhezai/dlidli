// Package infra 管理基础设施客户端（MySQL / Redis，后续扩展 Kafka / OSS / ES）。
//
// M0 阶段基础设施允许缺失（降级启动，便于本地无 docker 时开发框架代码）；
// M1 业务模块接入后应将所需依赖改为强制（连接失败直接退出）。
package infra

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/dlidli/server/internal/pkg/config"
	"github.com/dlidli/server/internal/pkg/storage"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

type Resources struct {
	DB      *gorm.DB
	Redis   *redis.Client
	Storage storage.Storage
	log     *zap.Logger
}

// Init 按配置初始化基础设施，失败仅告警不中断（见包注释）。
func Init(cfg *config.Config, log *zap.Logger) *Resources {
	r := &Resources{log: log}

	if cfg.MySQL.DSN != "" {
		db, err := newMySQL(cfg.MySQL)
		if err != nil {
			log.Warn("MySQL 连接失败，降级启动", zap.Error(err))
		} else {
			r.DB = db
			log.Info("MySQL 已连接")
		}
	}

	if cfg.Redis.Addr != "" {
		rdb := redis.NewClient(&redis.Options{
			Addr:     cfg.Redis.Addr,
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
		})
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := rdb.Ping(ctx).Err(); err != nil {
			log.Warn("Redis 连接失败，降级启动", zap.Error(err))
			_ = rdb.Close()
		} else {
			r.Redis = rdb
			log.Info("Redis 已连接")
		}
	}

	// 对象存储（dev 默认本地磁盘；minio 驱动在 M1-VID-01 接入）
	if cfg.Storage.Driver == "local" || cfg.Storage.Driver == "" {
		local, err := storage.NewLocal(cfg.Storage.LocalDir, cfg.Storage.BaseURL)
		if err != nil {
			log.Warn("本地存储初始化失败", zap.Error(err))
		} else {
			r.Storage = local
		}
	}

	return r
}

func newMySQL(cfg config.MySQL) (*gorm.DB, error) {
	// IgnoreRecordNotFoundError：未命中是正常业务分支（如转码 Worker 空轮询、按 ID 查无），
	// 由调用方自行处理 gorm.ErrRecordNotFound，不当错误日志刷屏
	db, err := gorm.Open(gormmysql.Open(cfg.DSN), &gorm.Config{
		Logger: gormlogger.New(
			log.New(os.Stdout, "\r\n", log.LstdFlags),
			gormlogger.Config{
				SlowThreshold:             200 * time.Millisecond,
				LogLevel:                  gormlogger.Warn,
				IgnoreRecordNotFoundError: true,
			},
		),
		TranslateError: true, // 将驱动错误转为 gorm.ErrDuplicatedKey 等语义错误
	})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	if cfg.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	sqlDB.SetConnMaxLifetime(time.Hour)
	return db, sqlDB.Ping()
}

// Health 返回各组件健康状态，供 /health 使用。
func (r *Resources) Health(ctx context.Context) map[string]string {
	status := map[string]string{"mysql": "down", "redis": "down"}
	if r.DB != nil {
		if sqlDB, err := r.DB.DB(); err == nil && sqlDB.PingContext(ctx) == nil {
			status["mysql"] = "up"
		}
	}
	if r.Redis != nil && r.Redis.Ping(ctx).Err() == nil {
		status["redis"] = "up"
	}
	return status
}

// Close 释放所有连接。
func (r *Resources) Close() {
	if r.Redis != nil {
		_ = r.Redis.Close()
	}
	if r.DB != nil {
		if sqlDB, err := r.DB.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}
}
