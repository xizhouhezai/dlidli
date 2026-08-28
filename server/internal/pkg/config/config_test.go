package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "cfg.yaml")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadDefaults(t *testing.T) {
	p := writeTemp(t, "app:\n  name: t\n  env: dev\n")
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if cfg.App.Port != 8000 {
		t.Errorf("默认端口应为 8000, got %d", cfg.App.Port)
	}
	if cfg.Transcode.Workers != 1 {
		t.Errorf("默认转码协程应为 1, got %d", cfg.Transcode.Workers)
	}
	if cfg.Transcode.FfmpegPath != "ffmpeg" || cfg.Transcode.FfprobePath != "ffprobe" {
		t.Errorf("默认 ffmpeg 路径应走 PATH: %q %q", cfg.Transcode.FfmpegPath, cfg.Transcode.FfprobePath)
	}
}

func TestLoadRateLimitDefaultWhenEnabled(t *testing.T) {
	p := writeTemp(t, "app:\n  name: t\nratelimit:\n  enabled: true\n")
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if !cfg.RateLimit.Enabled || cfg.RateLimit.PerMinute != 30 {
		t.Errorf("开启限流时默认应为 30/min, got %+v", cfg.RateLimit)
	}
}

func TestLoadRateLimitDisabledStaysZero(t *testing.T) {
	p := writeTemp(t, "app:\n  name: t\n")
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if cfg.RateLimit.Enabled || cfg.RateLimit.PerMinute != 0 {
		t.Errorf("未开启限流应保持禁用, got %+v", cfg.RateLimit)
	}
}

func TestLoadExplicitValuesPreserved(t *testing.T) {
	p := writeTemp(t, "app:\n  port: 9000\ntranscode:\n  workers: 4\nratelimit:\n  enabled: true\n  perMinute: 60\n")
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if cfg.App.Port != 9000 || cfg.Transcode.Workers != 4 {
		t.Errorf("显式配置被覆盖: port=%d workers=%d", cfg.App.Port, cfg.Transcode.Workers)
	}
	if cfg.RateLimit.PerMinute != 60 {
		t.Errorf("显式限流值应保留, got %d", cfg.RateLimit.PerMinute)
	}
}

func TestLoadEnvOverride(t *testing.T) {
	// viper AutomaticEnv：DLIDLI_ 前缀环境变量覆盖 yaml（键名点→下划线）
	t.Setenv("DLIDLI_TRANSCODE_FFMPEGPATH", "/opt/ffmpeg/bin/ffmpeg")
	t.Setenv("DLIDLI_APP_PORT", "9100")
	p := writeTemp(t, "app:\n  name: t\ntranscode:\n  workers: 2\n")
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if cfg.Transcode.FfmpegPath != "/opt/ffmpeg/bin/ffmpeg" {
		t.Errorf("环境变量应覆盖 ffmpeg 路径, got %q", cfg.Transcode.FfmpegPath)
	}
	if cfg.App.Port != 9100 {
		t.Errorf("环境变量应覆盖端口, got %d", cfg.App.Port)
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope.yaml")); err == nil {
		t.Error("配置文件不存在应报错")
	}
}

func TestLoadProdGuard(t *testing.T) {
	// prod 环境缺 secret/DSN（或用 dev 占位密钥）必须拒绝启动
	cases := []struct {
		name string
		yaml string
	}{
		{"缺secret", "app:\n  env: prod\nmysql:\n  dsn: \"u:p@tcp(h)/d\"\n"},
		{"dev占位密钥", "app:\n  env: prod\njwt:\n  secret: \"dev-secret-do-not-use-in-prod\"\nmysql:\n  dsn: \"u:p@tcp(h)/d\"\n"},
		{"缺DSN", "app:\n  env: prod\njwt:\n  secret: \"real-secret\"\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := Load(writeTemp(t, c.yaml)); err == nil {
				t.Errorf("prod 缺安全配置应报错")
			}
		})
	}
	// 正式注入后应放行
	t.Setenv("DLIDLI_JWT_SECRET", "real-secret")
	t.Setenv("DLIDLI_MYSQL_DSN", "u:p@tcp(h)/d")
	if _, err := Load(writeTemp(t, "app:\n  env: prod\n")); err != nil {
		t.Errorf("prod 注入环境变量后应通过: %v", err)
	}
}
