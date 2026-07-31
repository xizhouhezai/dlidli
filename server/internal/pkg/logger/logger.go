// Package logger 提供基于 zap 的结构化日志。
package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New 根据环境创建 logger：prod 使用 JSON 格式，其余环境使用控制台格式。
func New(level, env string) (*zap.Logger, error) {
	var cfg zap.Config
	if env == "prod" || env == "staging" {
		cfg = zap.NewProductionConfig()
	} else {
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	if lv, err := zapcore.ParseLevel(level); err == nil {
		cfg.Level = zap.NewAtomicLevelAt(lv)
	}
	return cfg.Build()
}
