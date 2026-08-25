package storage

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestLocal(t *testing.T) *Local {
	t.Helper()
	l, err := NewLocal(t.TempDir(), "http://x/static/")
	if err != nil {
		t.Fatalf("NewLocal 失败: %v", err)
	}
	return l
}

func TestLocalSaveRoundTrip(t *testing.T) {
	l := newTestLocal(t)
	url, err := l.Save(context.Background(), "covers/a/b.png", strings.NewReader("hello"))
	if err != nil {
		t.Fatalf("Save 失败: %v", err)
	}
	if url != "http://x/static/covers/a/b.png" {
		t.Errorf("URL=%q", url)
	}
	data, err := os.ReadFile(filepath.Join(l.dir, "covers", "a", "b.png"))
	if err != nil || string(data) != "hello" {
		t.Errorf("读回内容异常: %q err=%v", data, err)
	}
}

func TestLocalSaveRejectsTraversal(t *testing.T) {
	l := newTestLocal(t)
	if _, err := l.Save(context.Background(), "../evil.txt", strings.NewReader("x")); err == nil {
		t.Error("../ 前缀 key 应被拒绝")
	}
	if _, err := l.Save(context.Background(), "/abs/x.png", strings.NewReader("x")); err == nil {
		t.Error("绝对路径 key 应被拒绝")
	}
}

func TestLocalPathStaysInsideDir(t *testing.T) {
	l := newTestLocal(t)
	p := l.LocalPath("../../etc/passwd")
	if strings.Contains(p, "..") {
		t.Errorf("LocalPath 应消除 ..: %q", p)
	}
	if !strings.HasPrefix(p, l.dir) {
		t.Errorf("LocalPath 应位于 dir 内: %q (dir=%q)", p, l.dir)
	}
}

func TestLocalURL(t *testing.T) {
	l := newTestLocal(t)
	if got := l.URL("videos/hls/x/index.m3u8"); got != "http://x/static/videos/hls/x/index.m3u8" {
		t.Errorf("URL=%q", got)
	}
}
