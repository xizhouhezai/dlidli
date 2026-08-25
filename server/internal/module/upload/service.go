package upload

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dlidli/server/internal/pkg/errcode"
	"github.com/dlidli/server/internal/pkg/snowflake"
	"github.com/dlidli/server/internal/pkg/storage"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	ChunkSize   = 5 << 20        // 5MB
	maxFileSize = 8 << 30        // 8GB
	sessionTTL  = 24 * time.Hour // 超时未完成的上传由会话过期 + 目录清理兜底
)

var allowedExts = map[string]bool{".mp4": true, ".mov": true, ".mkv": true, ".flv": true, ".avi": true}

type Service struct {
	repo   *Repo
	rdb    *redis.Client
	store  storage.Storage
	tmpDir string // 分片暂存目录
	log    *zap.Logger
}

func NewService(repo *Repo, rdb *redis.Client, store storage.Storage, tmpDir string, log *zap.Logger) *Service {
	return &Service{repo: repo, rdb: rdb, store: store, tmpDir: tmpDir, log: log}
}

func sessKey(id string) string  { return "up:sess:" + id }
func partsKey(id string) string { return "up:parts:" + id }

// Init 初始化上传：秒传命中直接返回文件；否则创建/恢复上传会话。
func (s *Service) Init(ctx context.Context, uid int64, req *InitReq) (*InitResp, error) {
	ext := strings.ToLower(filepath.Ext(req.FileName))
	if !allowedExts[ext] {
		return nil, errcode.ErrFileTypeNotAllowed
	}
	if req.FileSize > maxFileSize {
		return nil, errcode.ErrFileTooLarge
	}
	hash := strings.ToLower(req.FileHash)

	// 秒传：同 hash 文件已存在
	if f, err := s.repo.FindByHash(hash); err != nil {
		return nil, err
	} else if f != nil {
		return &InitResp{Fast: true, FileID: fmt.Sprintf("%d", f.ID), StoreKey: f.StoreKey}, nil
	}

	// 恢复未完成会话（断点续传）：以 hash 作为会话索引
	if existing, _ := s.rdb.Get(ctx, "up:byhash:"+hash).Result(); existing != "" {
		if uploaded, err := s.uploadedParts(ctx, existing); err == nil {
			chunkCount, _ := s.rdb.HGet(ctx, sessKey(existing), "chunk_count").Int()
			return &InitResp{
				UploadID: existing, ChunkSize: ChunkSize,
				ChunkCount: chunkCount, Uploaded: uploaded,
			}, nil
		}
	}

	uploadID := fmt.Sprintf("%d", snowflake.NextID())
	chunkCount := int((req.FileSize + ChunkSize - 1) / ChunkSize)

	// 用 HMSET 兼容老版本 Redis（多字段 HSET 需 Redis >= 4.0）
	if err := s.rdb.HMSet(ctx, sessKey(uploadID), map[string]any{
		"user_id":     uid,
		"file_name":   req.FileName,
		"file_size":   req.FileSize,
		"file_hash":   hash,
		"chunk_count": chunkCount,
		"ext":         ext,
	}).Err(); err != nil {
		return nil, err
	}
	s.rdb.Expire(ctx, sessKey(uploadID), sessionTTL)
	s.rdb.Set(ctx, "up:byhash:"+hash, uploadID, sessionTTL)

	return &InitResp{
		UploadID: uploadID, ChunkSize: ChunkSize,
		ChunkCount: chunkCount, Uploaded: []int{},
	}, nil
}

// UploadPart 接收一个分片并暂存磁盘。
func (s *Service) UploadPart(ctx context.Context, uid int64, uploadID string, index int, r io.Reader) error {
	sess, err := s.session(ctx, uid, uploadID)
	if err != nil {
		return err
	}
	chunkCount, _ := strconv.Atoi(sess["chunk_count"])
	if index < 0 || index >= chunkCount {
		return errcode.ErrChunkIndexInvalid
	}

	dir := filepath.Join(s.tmpDir, uploadID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(dir, fmt.Sprintf("%06d.part", index)))
	if err != nil {
		return err
	}
	defer f.Close()

	// 单分片限长：防止恶意超大请求体
	n, err := io.Copy(f, io.LimitReader(r, ChunkSize+1))
	if err != nil {
		return err
	}
	if n == 0 || n > ChunkSize {
		return errcode.ErrInvalidParams.WithMsg("分片大小不合法")
	}

	if err := s.rdb.SAdd(ctx, partsKey(uploadID), index).Err(); err != nil {
		return err
	}
	s.rdb.Expire(ctx, partsKey(uploadID), sessionTTL)
	return nil
}

// Progress 返回已上传分片下标。
func (s *Service) Progress(ctx context.Context, uid int64, uploadID string) (*InitResp, error) {
	sess, err := s.session(ctx, uid, uploadID)
	if err != nil {
		return nil, err
	}
	uploaded, err := s.uploadedParts(ctx, uploadID)
	if err != nil {
		return nil, err
	}
	chunkCount, _ := strconv.Atoi(sess["chunk_count"])
	return &InitResp{UploadID: uploadID, ChunkSize: ChunkSize, ChunkCount: chunkCount, Uploaded: uploaded}, nil
}

// Complete 校验分片完整性，按序合并、SHA-256 校验后落对象存储并登记。
func (s *Service) Complete(ctx context.Context, uid int64, uploadID string) (*CompleteResp, error) {
	sess, err := s.session(ctx, uid, uploadID)
	if err != nil {
		return nil, err
	}
	chunkCount, _ := strconv.Atoi(sess["chunk_count"])
	fileSize, _ := strconv.ParseInt(sess["file_size"], 10, 64)

	cnt, err := s.rdb.SCard(ctx, partsKey(uploadID)).Result()
	if err != nil {
		return nil, err
	}
	if int(cnt) != chunkCount {
		return nil, errcode.ErrUploadIncomplete
	}

	dir := filepath.Join(s.tmpDir, uploadID)

	// 合并到临时文件，同时计算 SHA-256
	merged, err := os.CreateTemp(s.tmpDir, "merge-*")
	if err != nil {
		return nil, err
	}
	mergedPath := merged.Name()
	defer os.Remove(mergedPath)

	hasher := sha256.New()
	w := io.MultiWriter(merged, hasher)
	var total int64
	for i := 0; i < chunkCount; i++ {
		part, err := os.Open(filepath.Join(dir, fmt.Sprintf("%06d.part", i)))
		if err != nil {
			merged.Close()
			return nil, errcode.ErrUploadIncomplete
		}
		n, err := io.Copy(w, part)
		part.Close()
		if err != nil {
			merged.Close()
			return nil, err
		}
		total += n
	}
	merged.Close()

	if total != fileSize || hex.EncodeToString(hasher.Sum(nil)) != sess["file_hash"] {
		return nil, errcode.ErrFileHashMismatch
	}

	// 落对象存储
	src, err := os.Open(mergedPath)
	if err != nil {
		return nil, err
	}
	defer src.Close()

	key := "videos/source/" + sess["file_hash"] + sess["ext"]
	if _, err := s.store.Save(ctx, key, src); err != nil {
		return nil, err
	}

	userID, _ := strconv.ParseInt(sess["user_id"], 10, 64)
	record, err := s.repo.Create(&UploadFile{
		ID:       snowflake.NextID(),
		UserID:   userID,
		FileName: sess["file_name"],
		FileHash: sess["file_hash"],
		FileSize: fileSize,
		StoreKey: key,
	})
	if err != nil {
		return nil, err
	}

	// 清理会话与分片
	s.rdb.Del(ctx, sessKey(uploadID), partsKey(uploadID), "up:byhash:"+sess["file_hash"])
	if err := os.RemoveAll(dir); err != nil {
		s.log.Warn("清理分片目录失败", zap.String("dir", dir), zap.Error(err))
	}

	return &CompleteResp{FileID: fmt.Sprintf("%d", record.ID), StoreKey: record.StoreKey}, nil
}

// GetUserFile 按 ID 获取当前用户可用的已完成文件（供 video 模块投稿时校验）。
// 秒传场景下文件首传者可能不是当前用户，故不限制 user_id。
func (s *Service) GetUserFile(_ context.Context, fileID int64) (*UploadFile, error) {
	f, err := s.repo.FindByID(fileID)
	if err != nil {
		return nil, err
	}
	if f == nil {
		return nil, errcode.ErrNotFound.WithMsg("上传文件不存在")
	}
	return f, nil
}

// session 读取并校验上传会话归属。
func (s *Service) session(ctx context.Context, uid int64, uploadID string) (map[string]string, error) {
	sess, err := s.rdb.HGetAll(ctx, sessKey(uploadID)).Result()
	if err != nil {
		return nil, err
	}
	if len(sess) == 0 {
		return nil, errcode.ErrUploadNotFound
	}
	if owner, _ := strconv.ParseInt(sess["user_id"], 10, 64); owner != uid {
		return nil, errcode.ErrForbidden
	}
	return sess, nil
}

func (s *Service) uploadedParts(ctx context.Context, uploadID string) ([]int, error) {
	members, err := s.rdb.SMembers(ctx, partsKey(uploadID)).Result()
	if err != nil {
		return nil, err
	}
	uploaded := make([]int, 0, len(members))
	for _, m := range members {
		if v, err := strconv.Atoi(m); err == nil {
			uploaded = append(uploaded, v)
		}
	}
	return uploaded, nil
}

// CleanupOrphans 扫描 tmpDir 下的孤儿分片目录，删除超过 maxAge 未活动（目录 mtime 久于 maxAge）的会话残留，
// 并同步清理其 Redis 会话/分片 key。返回删除的目录数。
// 安全依据：目录 mtime 由每次分片写入刷新，活跃上传目录必然较新，不会误删。
func (s *Service) CleanupOrphans(ctx context.Context, maxAge time.Duration) (int, error) {
	if s.tmpDir == "" {
		return 0, nil
	}
	entries, err := os.ReadDir(s.tmpDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil // tmpDir 尚未创建，无残留
		}
		return 0, err
	}
	cutoff := time.Now().Add(-maxAge)
	removed := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue // 跳过 merge-* 等临时文件；此类文件由 Complete 自行清理
		}
		uploadID := e.Name()
		dir := filepath.Join(s.tmpDir, uploadID)
		info, err := e.Info()
		if err != nil {
			s.log.Warn("读取分片目录信息失败，跳过", zap.String("dir", dir), zap.Error(err))
			continue
		}
		if info.ModTime().After(cutoff) {
			continue // 仍有活动（最近写入过分片）或较新的会话
		}
		// 已过期：清理磁盘目录与残留 Redis 会话
		if err := os.RemoveAll(dir); err != nil {
			s.log.Warn("清理过期分片目录失败", zap.String("dir", dir), zap.Error(err))
			continue
		}
		s.rdb.Del(ctx, sessKey(uploadID), partsKey(uploadID))
		s.log.Info("清理过期分片目录", zap.String("upload_id", uploadID))
		removed++
	}
	if removed > 0 {
		s.log.Info("分片孤儿清理完成", zap.Int("removed", removed))
	}
	return removed, nil
}

// StartCleanupWorker 周期性清理孤儿分片目录（后台 goroutine）。
// interval 为执行间隔；进程退出（ctx 取消）时停止。默认为 30 分钟。
func (s *Service) StartCleanupWorker(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Minute
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := s.CleanupOrphans(ctx, sessionTTL); err != nil {
					s.log.Warn("孤儿分片清理失败", zap.Error(err))
				}
			}
		}
	}()
}
