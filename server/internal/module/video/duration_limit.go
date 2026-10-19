package video

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/dlidli/server/internal/pkg/errcode"
	"go.uber.org/zap"
)

// 投稿时长限制（roadmap 缓解措施原文：「限制内测投稿时长 ≤ 30min」）。
//
// 为什么在投稿阶段拒绝而不是转码阶段失败：
//   - 转码阶段失败＝用户已上传完整文件、排了队、等了才知道被拒，体验最差，
//     而且失败要占用 Worker 一次执行（30min 素材的转码开销正是要避免的）；
//   - 投稿阶段拒绝＝同步、立刻、可读原因，且**根本不会产生转码任务**，从源头削减积压。
//
// 时长来源：ffprobe 读上传源文件的真实时长（不信任前端上报）。
// 探测失败时**放行**（与 transcoder 里"探测失败不阻断转码"同口径）：
// 时长限制是体验/容量护栏，不是安全边界；让探测器抖动挡住投稿是更坏的故障。
// 若要让它变成硬边界，应在上传完成时落 duration 列，而不是把这里的策略改成"探测失败即拒"。

// formatSeconds 把秒数渲染成 "30分00秒" 便于错误提示。
func formatSeconds(total int) string {
	if total <= 0 {
		return "0秒"
	}
	m, sec := total/60, total%60
	if m == 0 {
		return fmt.Sprintf("%d秒", sec)
	}
	return fmt.Sprintf("%d分%02d秒", m, sec)
}

// probeLocalDuration 读本地文件时长（秒）。文件不存在或 ffprobe 不可用时返回错误。
func (s *Service) probeLocalDuration(ctx context.Context, path string) (int, error) {
	if path == "" {
		return 0, fmt.Errorf("存储不支持本地路径解析")
	}
	if _, err := os.Stat(path); err != nil {
		return 0, err
	}
	cmd := exec.CommandContext(ctx, s.cfg.Transcode.FfprobePath,
		"-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", path)
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	raw := strings.TrimSpace(string(out))
	if i := strings.IndexByte(raw, '\n'); i >= 0 {
		raw = raw[:i]
	}
	sec, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 0, err
	}
	return int(sec + 0.5), nil
}

// ensureWithinDurationLimit 校验单P时长；超限返回 ErrVideoTooLong。
// partLabel 用于错误提示（多P时指出是哪一P）。
func (s *Service) ensureWithinDurationLimit(ctx context.Context, path, partLabel string) error {
	limit := s.cfg.Transcode.MaxDurationSeconds
	if limit <= 0 {
		return nil // 未配置上限＝不限制（config.Load 默认 30min，此处只兜底测试/特殊环境）
	}
	dur, err := s.probeLocalDuration(ctx, path)
	if err != nil {
		// 探测失败放行（见上方注释）；留下告警线索，避免"限制静默失效"。
		s.log.Warn("投稿时长探测失败，按时长限制放行",
			zap.String("path", path), zap.String("part", partLabel), zap.Error(err))
		return nil
	}
	if dur > limit {
		return errcode.ErrVideoTooLong.WithMsg(fmt.Sprintf(
			"%s时长 %s 超过投稿上限 %s，请剪辑后再投稿",
			partLabel, formatSeconds(dur), formatSeconds(limit)))
	}
	return nil
}
