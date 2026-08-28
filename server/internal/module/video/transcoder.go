package video

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dlidli/server/internal/pkg/storage"
	"go.uber.org/zap"
)

const pollInterval = 3 * time.Second

// transcodeJobTimeout 单个转码任务上限：ffmpeg/ffprobe 挂死时由 ctx 超时兜底，避免永久占用 Worker。
const transcodeJobTimeout = 30 * time.Minute

// 转码档位参数（对齐 docs/architecture/video-pipeline.md §3）
var presets = map[int16]struct {
	height  int
	vBits   string
	maxRate string
	bufSize string
	aBits   string
}{
	360: {360, "500k", "600k", "1000k", "96k"},
	720: {720, "2000k", "2400k", "4000k", "128k"},
}

// StartTranscodeWorkers 启动进程内转码 Worker（dev 便利；部署环境改由独立 cmd/worker 运行）。
// 依赖本地存储（storage.PathResolver）；其他驱动下直接告警退出。
func (s *Service) StartTranscodeWorkers(ctx context.Context, store storage.Storage) {
	resolver, ok := store.(storage.PathResolver)
	if !ok {
		s.log.Warn("当前存储驱动不支持本地转码，Worker 未启动")
		return
	}
	for i := 0; i < s.cfg.Transcode.Workers; i++ {
		go s.workerLoop(ctx, resolver, i)
	}
	s.log.Info("转码 Worker 已启动", zap.Int("workers", s.cfg.Transcode.Workers))
}

func (s *Service) workerLoop(ctx context.Context, resolver storage.PathResolver, id int) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for {
				job, err := s.repo.ClaimJob()
				if err != nil {
					s.log.Error("认领转码任务失败", zap.Error(err))
					break
				}
				if job == nil {
					break // 队列空，等下个周期
				}
				s.processJob(ctx, resolver, job, id)
			}
		}
	}
}

func (s *Service) processJob(ctx context.Context, resolver storage.PathResolver, job *TranscodeJob, workerID int) {
	// 单任务超时 + panic 自隔离：任一任务异常只回写失败状态，不能拖死 Worker 或击穿进程
	ctx, cancel := context.WithTimeout(ctx, transcodeJobTimeout)
	defer cancel()
	completed := false
	defer func() {
		if r := recover(); r != nil {
			s.log.Error("转码任务 panic，已隔离", zap.Any("recover", r), zap.Int64("job_id", job.ID))
			if !completed {
				if err := s.repo.FailJob(job, fmt.Sprintf("worker panic: %v", r)); err != nil {
					s.log.Error("panic 后回写失败状态出错", zap.Error(err), zap.Int64("job_id", job.ID))
				}
			}
		}
	}()
	log := s.log.With(
		zap.Int64("job_id", job.ID),
		zap.Int64("video_id", job.VideoID),
		zap.Int16("quality", job.Quality),
		zap.Int("worker", workerID),
	)
	start := time.Now()

	if err := s.doTranscode(ctx, resolver, job); err != nil {
		log.Error("转码失败", zap.Error(err), zap.Int8("retry", job.RetryCnt))
		if err := s.repo.FailJob(job, err.Error()); err != nil {
			log.Error("回写失败状态出错", zap.Error(err))
		}
		return
	}

	if err := s.repo.CompleteJob(job.ID); err != nil {
		log.Error("回写完成状态出错", zap.Error(err))
		return
	}
	completed = true
	log.Info("转码完成", zap.Duration("cost", time.Since(start)))

	// 全部档位完成 → 推进稿件状态机：转码中 → 待审核（dev autoApprove 直接发布）
	left, err := s.repo.UnfinishedJobCount(job.VideoID)
	if err != nil || left > 0 {
		return
	}
	fields := map[string]any{"status": StatusReviewing}
	if s.cfg.App.AutoApprove {
		fields["status"] = StatusPublished
		fields["published_at"] = time.Now()
	}
	if err := s.repo.UpdateVideoFields(job.VideoID, fields); err != nil {
		log.Error("推进稿件状态失败", zap.Error(err))
		return
	}
	log.Info("稿件转码全部完成，状态已推进", zap.Any("status", fields["status"]))

	// dev 自动过审直接发布 → 触发发布钩子（动态生成等旁路逻辑）
	if s.cfg.App.AutoApprove {
		if v, err := s.repo.FindVideoByID(job.VideoID); err == nil && v != nil {
			s.firePublish(v.ID, v.UserID)
		}
	}
}

func (s *Service) doTranscode(ctx context.Context, resolver storage.PathResolver, job *TranscodeJob) error {
	preset, ok := presets[job.Quality]
	if !ok {
		return fmt.Errorf("未知转码档位: %d", job.Quality)
	}

	v, err := s.repo.FindVideoByID(job.VideoID)
	if err != nil {
		return err
	}
	if v == nil || v.Status == StatusDeleted {
		return fmt.Errorf("稿件不存在或已删除")
	}

	// 源文件 = 原画流（quality 0，按分P）
	streams, err := s.repo.StreamsByVideoPart(v.ID, job.PartIndex)
	if err != nil {
		return err
	}
	var srcKey string
	for _, st := range streams {
		if st.Quality == 0 {
			srcKey = st.PlayPath
			break
		}
	}
	if srcKey == "" {
		return fmt.Errorf("找不到源文件流")
	}
	src := resolver.LocalPath(srcKey)

	// 时长与封面：单P写 video.duration，多P写 video_part.duration；封面仅单P抽帧
	if v.Duration == 0 && job.PartIndex == 0 {
		if dur, err := s.probeDuration(ctx, src); err == nil && dur > 0 {
			_ = s.repo.UpdateVideoFields(v.ID, map[string]any{"duration": dur})
			v.Duration = dur
		} else if err != nil {
			s.log.Warn("ffprobe 取时长失败", zap.Error(err))
		}
	}
	if job.PartIndex > 0 {
		if dur, err := s.probeDuration(ctx, src); err == nil && dur > 0 {
			_ = s.repo.UpdatePartDuration(v.ID, job.PartIndex, dur)
		}
	}
	if v.Cover == "" && job.PartIndex == 0 {
		if coverURL, err := s.extractCover(ctx, resolver, src, v); err == nil && coverURL != "" {
			_ = s.repo.UpdateVideoFields(v.ID, map[string]any{"cover": coverURL})
		} else if err != nil {
			s.log.Warn("抽帧封面失败", zap.Error(err))
		}
	}

	// HLS 输出目录（分P隔离防覆盖）
	outKey := fmt.Sprintf("videos/hls/%d/%d/%d", v.ID, job.PartIndex, job.Quality)
	outDir := resolver.LocalPath(outKey)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	args := []string{
		"-y", "-i", src,
		"-c:v", "libx264", // 不指定 profile：兼容低码率/低分辨率测试源（指定 main 时部分源报 -22）
		"-b:v", preset.vBits, "-maxrate", preset.maxRate, "-bufsize", preset.bufSize,
		"-vf", fmt.Sprintf("scale=-2:%d", preset.height),
		"-r", "30", "-g", "60",
		"-c:a", "aac", "-b:a", preset.aBits, "-ac", "2",
		"-hls_time", "6", "-hls_playlist_type", "vod",
		"-hls_segment_filename", filepath.Join(outDir, "seg_%05d.ts"),
		filepath.Join(outDir, "index.m3u8"),
	}
	cmd := exec.CommandContext(ctx, s.cfg.Transcode.FfmpegPath, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg: %v: %s", err, tail(string(out), 300))
	}

	return s.repo.AddStream(&Stream{
		VideoID:   v.ID,
		PartIndex: job.PartIndex,
		Quality:   job.Quality,
		Format:    "hls",
		PlayPath:  outKey + "/index.m3u8",
		FileSize:  dirSize(outDir),
	})
}

// probeDuration 用 ffprobe 读取视频时长（秒）。
func (s *Service) probeDuration(ctx context.Context, src string) (int, error) {
	cmd := exec.CommandContext(ctx, s.cfg.Transcode.FfprobePath,
		"-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", src)
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	sec, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 0, err
	}
	return int(sec + 0.5), nil
}

// extractCover 在 10% 进度处（最多 3s）抽帧作为封面。
func (s *Service) extractCover(ctx context.Context, resolver storage.PathResolver, src string, v *Video) (string, error) {
	seek := float64(v.Duration) * 0.1
	if seek > 3 {
		seek = 3
	}
	key := fmt.Sprintf("covers/auto_%d.jpg", v.ID)
	out := resolver.LocalPath(key)
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, s.cfg.Transcode.FfmpegPath,
		"-y", "-ss", fmt.Sprintf("%.2f", seek), "-i", src,
		"-vframes", "1", "-vf", "scale=-2:720", out)
	if outMsg, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("ffmpeg 抽帧: %v: %s", err, tail(string(outMsg), 200))
	}
	return strings.TrimRight(s.cfg.Storage.BaseURL, "/") + "/" + key, nil
}

func dirSize(dir string) int64 {
	var total int64
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
