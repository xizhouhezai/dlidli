package video

import (
	"context"
	"fmt"
	"io/fs"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dlidli/server/internal/pkg/qualitytier"
	"github.com/dlidli/server/internal/pkg/storage"
	"go.uber.org/zap"
)

const pollInterval = 3 * time.Second

// transcodeJobTimeout 单个转码任务上限：ffmpeg/ffprobe 挂死时由 ctx 超时兜底，避免永久占用 Worker。
// 4K 档位（M4-VIP-02）单机 CPU 转码远慢于 360/720，故放宽到 2 小时：
// 仍是有界超时（不会永久占位），但给 4K 留出可完成的窗口（验收用短视频实测）。
const transcodeJobTimeout = 2 * time.Hour

// preset 单个档位的转码参数（取自 video_quality_tier 配置，见 tier.go）。
type preset struct {
	height  int
	vBits   string
	maxRate string
	bufSize string
	aBits   string
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
	// 草稿态不推进（M4-LIV-14 直播回放）：回放稿件建成即为**草稿**，发布与否由主播决定
	// （spec LIV-14「主播可选发布」）。若在这里一律推到待审/已发布，
	// 「未发布只有主播可见」当场失效——稿件的可见性由 status 决定，绝不能由转码完成时间决定。
	if v, err := s.repo.FindVideoByID(job.VideoID); err == nil && v != nil && v.Status == StatusDraft {
		log.Info("草稿稿件转码完成，状态保持草稿（待作者发布）")
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
			s.firePublish(ctx, v.ID, v.UserID)
		}
	}
}

func (s *Service) doTranscode(ctx context.Context, resolver storage.PathResolver, job *TranscodeJob) error {
	tier, ok := s.tiers.Set().Get(job.Quality)
	if !ok || job.Quality == qualitytier.Origin {
		return fmt.Errorf("未知转码档位: %d", job.Quality)
	}
	preset := preset{height: tier.Height, vBits: tier.VBits, maxRate: tier.MaxRate, bufSize: tier.BufSize, aBits: tier.ABits}
	if preset.vBits == "" {
		return fmt.Errorf("档位 %d 缺少码率配置", job.Quality)
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
	originHeight := 0
	for _, st := range streams {
		if st.Quality == qualitytier.Origin {
			srcKey = st.PlayPath
			originHeight = st.Height
			break
		}
	}
	if srcKey == "" {
		return fmt.Errorf("找不到源文件流")
	}
	src := resolver.LocalPath(srcKey)

	// 源分辨率：既用于 HLS 播放列表的 RESOLUTION 标注，更用于**流的实际高度落库**——
	// 免费用户的 1080P 上限判定靠它（4K 源的原画行 height=3840 > 1080，下发给免费用户即 4K 越权）。
	// 探测失败不阻断转码（height 记 0，判定侧对未知高度只按档位配置判定）。
	srcW, srcH, err := s.probeResolution(ctx, src)
	if err != nil {
		s.log.Warn("ffprobe 取源分辨率失败，流高度将记 0", zap.Error(err), zap.Int64("video_id", v.ID))
	}
	// 回填原画行的真实高度：原画直出的是上传的源文件，4K 源在这一行上就是 4K。
	// 不回填的话，免费用户会通过「原画」这条路径拿到超上限分辨率（spec §8.2 的核心缺口）。
	// 回填时机选在这里（而不是投稿请求里）有两个理由：① 投稿是同步请求，探测会给它加一次
	// ffprobe 往返；② 转码本来就持有本地路径与 ffprobe 调用，零额外依赖。
	// 仅当该行尚未探测（height=0）或与实测不一致时才写 —— 多档位任务并发时也只是重复写同一个值。
	// 边界：转码总开关关闭时没有任何任务运行，原画行高度保持 0（此时也不存在任何档位产物，
	// 与升级前的行为一致）。
	if srcH > 0 && originHeight != srcH {
		if err := s.repo.SetStreamHeight(v.ID, job.PartIndex, qualitytier.Origin, srcH); err != nil {
			s.log.Warn("回填原画流高度失败", zap.Error(err), zap.Int64("video_id", v.ID))
		}
	}
	// 实际上限：档位目标高于源分辨率时按源输出（放大只会浪费码率且无收益）。
	outH := preset.height
	if srcH > 0 && outH > srcH {
		outH = srcH
	}
	// 目标高度须为偶数（libx264 要求，奇数会导致 -22）。
	if outH <= 0 {
		return fmt.Errorf("档位 %d 目标高度非法: %d", job.Quality, outH)
	}
	if outH%2 != 0 {
		outH--
	}

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

	// 目标宽度：按源宽高比换算并置偶数。不用 `scale=-2:<h>` 让 ffmpeg 自己算宽度，
	// 是因为部分低码率测试源的宽高比会在取整后得到奇数宽度，libx264 直接报 -22
	// （与「不指定 profile」同一个踩坑来源，见下方注释）。
	scale := fmt.Sprintf("scale=-2:%d", outH)
	outW := 0
	if srcW > 0 && srcH > 0 {
		outW = int(math.Round(float64(srcW) * float64(outH) / float64(srcH)))
		if outW < 2 {
			outW = 2
		}
		if outW%2 != 0 {
			outW--
		}
		// setsar=1：源带非方形像素（SAR≠1）时，否则播放器会把画面拉伸。
		scale = fmt.Sprintf("scale=%d:%d,setsar=1", outW, outH)
	}

	// HLS 输出目录（分P隔离防覆盖）。
	// 目标宽度已知时把它编入目录名：**同一档位可能因源分辨率不同产出不同画幅**，
	// 目录不含宽度会让「先转 4K、换源再转 4K」的产物互相覆盖（同 (video,part,quality) 唯一键下
	// 是同一行 yet 不同内容）。宽度未知（探测失败）时退回旧目录名，保持向后兼容。
	outKey := fmt.Sprintf("videos/hls/%d/%d/%d", v.ID, job.PartIndex, job.Quality)
	if outW > 0 {
		outKey = fmt.Sprintf("videos/hls/%d/%d/%d/%d", v.ID, job.PartIndex, job.Quality, outW)
	}
	outDir := resolver.LocalPath(outKey)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	args := []string{
		"-y", "-i", src,
		"-c:v", "libx264", // 不指定 profile：兼容低码率/低分辨率测试源（指定 main 时部分源报 -22）
		"-b:v", preset.vBits, "-maxrate", preset.maxRate, "-bufsize", preset.bufSize,
		"-vf", scale,
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
		Height:    outH,
	})
}

// probeResolution 用 ffprobe 读取视频流的宽高（像素）。
func (s *Service) probeResolution(ctx context.Context, src string) (int, int, error) {
	cmd := exec.CommandContext(ctx, s.cfg.Transcode.FfprobePath,
		"-v", "error", "-select_streams", "v:0",
		"-show_entries", "stream=width,height", "-of", "csv=p=0:s=x", src)
	out, err := cmd.Output()
	if err != nil {
		return 0, 0, err
	}
	line := strings.TrimSpace(string(out))
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	parts := strings.Split(strings.TrimSpace(line), "x")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("无法解析分辨率: %q", line)
	}
	w, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, err
	}
	h, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, err
	}
	return w, h, nil
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
