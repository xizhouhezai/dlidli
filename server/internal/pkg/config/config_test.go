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

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope.yaml")); err == nil {
		t.Error("配置文件不存在应报错")
	}
}
