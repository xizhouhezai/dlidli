// Package storage 对象存储抽象：dev 用本地磁盘，生产切换 MinIO/OSS（M1-VID-01 接入）。
package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Storage 对象存储接口。
type Storage interface {
	// Save 保存对象并返回可访问 URL。key 形如 "avatars/123.png"。
	Save(ctx context.Context, key string, r io.Reader) (url string, err error)
}

// PathResolver 本地存储可解析磁盘绝对路径（转码 Worker 直接读写文件用；
// MinIO 驱动不实现此接口，Worker 届时改为下载到临时目录处理）。
type PathResolver interface {
	LocalPath(key string) string
}

// Local 本地磁盘实现：文件写入 dir，URL 为 baseURL/key。
type Local struct {
	dir     string
	baseURL string
}

func NewLocal(dir, baseURL string) (*Local, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("创建存储目录失败: %w", err)
	}
	return &Local{dir: dir, baseURL: strings.TrimRight(baseURL, "/")}, nil
}

func (l *Local) Save(_ context.Context, key string, r io.Reader) (string, error) {
	// 防路径穿越
	clean := filepath.Clean(key)
	if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
		return "", fmt.Errorf("非法存储 key: %s", key)
	}

	path := filepath.Join(l.dir, clean)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}

	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if _, err := io.Copy(f, r); err != nil {
		return "", err
	}
	return l.baseURL + "/" + filepath.ToSlash(clean), nil
}

// LocalPath 返回 key 对应的磁盘绝对路径；强制根定位消除 ..，保证结果始终在 dir 内。
func (l *Local) LocalPath(key string) string {
	clean := path.Clean("/" + filepath.ToSlash(key))
	return filepath.Join(l.dir, filepath.FromSlash(clean))
}

// URL 返回 key 的对外访问地址。
func (l *Local) URL(key string) string {
	return l.baseURL + path.Clean("/"+filepath.ToSlash(key))
}
