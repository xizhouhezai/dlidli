// Package config 负责加载多环境配置（yaml + 环境变量覆盖）。
// 环境变量前缀 DLIDLI_，如 DLIDLI_MYSQL_DSN 覆盖 mysql.dsn。
package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	App       App
	Log       Log
	MySQL     MySQL
	Redis     Redis
	JWT       JWT
	Storage   Storage
	RateLimit RateLimit
	Transcode Transcode
}

type App struct {
	Name         string
	Env          string // dev / test / staging / prod
	Port         int
	AllowOrigins []string
	// AutoApprove 仅限开发环境：稿件提交后自动过审（审核工作台 M1-ADM 上线前的联调便利）
	AutoApprove bool
}

type Log struct {
	Level string // debug / info / warn / error
}

type MySQL struct {
	DSN          string
	MaxOpenConns int
	MaxIdleConns int
}

type Redis struct {
	Addr     string
	Password string
	DB       int
}

type JWT struct {
	Secret       string
	AccessTTLMin int
}

type Storage struct {
	Driver   string // local / minio（M1-VID-01 接入）
	LocalDir string // local 驱动：磁盘目录
	BaseURL  string // 对外访问前缀，如 http://localhost:8000/static
}

type RateLimit struct {
	Enabled   bool // 全局写接口限流开关
	PerMinute int  // 每个 key（用户/IP）每分钟写请求上限；<=0 表示禁用
}

type Transcode struct {
	Enabled     bool   // dev 内嵌 Worker；部署环境由独立 cmd/worker 承担
	Workers     int    // 并发转码协程数
	FfmpegPath  string // 缺省 "ffmpeg"（依赖 PATH）
	FfprobePath string // 缺省 "ffprobe"
}

// Load 读取指定路径的 yaml 配置，并允许环境变量覆盖。
func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetEnvPrefix("DLIDLI")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件 %s 失败: %w", path, err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}
	if cfg.App.Port == 0 {
		cfg.App.Port = 8000
	}
	if cfg.RateLimit.PerMinute == 0 && cfg.RateLimit.Enabled {
		cfg.RateLimit.PerMinute = 30 // 默认每个写接口 30 次/分钟
	}
	if cfg.Transcode.Workers <= 0 {
		cfg.Transcode.Workers = 1
	}
	if cfg.Transcode.FfmpegPath == "" {
		cfg.Transcode.FfmpegPath = "ffmpeg"
	}
	if cfg.Transcode.FfprobePath == "" {
		cfg.Transcode.FfprobePath = "ffprobe"
	}
	return &cfg, nil
}
