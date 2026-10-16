package live

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dlidli/server/internal/module/video"
	"github.com/dlidli/server/internal/pkg/config"
	"github.com/dlidli/server/internal/pkg/errcode"
	"github.com/dlidli/server/internal/pkg/storage"
	"go.uber.org/zap"
)

// 直播回放（M4-LIV-14，spec LIV-14 / plan liv14 §3）。
//
// 链路：SRS DVR 落盘（FLV，plan liv14 D1）→ 下播（on_unpublish）登记素材（live_replay）
// → 转码源就绪（FLV remux 成 MP4 进存储）→ 调 video 内部路径建**草稿**稿件
// → 主播可选发布（未发布只有主播可见）。
//
// 三条不可动摇的顺序约束（spec LIV-14 失败处理条款）：
//  1. **房间状态先落定，回放生成后置**：录制缺失/损坏/转码失败都不得影响下播。
//  2. 生成路径的全部错误只落在 live_replay.status/fail_reason（可追溯），不上抛。
//  3. 可见性由 video.status 决定：草稿不推进、不索引、不外泄。

// replayLookupWait 录制文件落盘等待上限：on_unpublish 与 SRS 关闭 FLV 文件之间有毫秒级
// 竞争（SRS 先回 2xx 再 flush），文件尚未落盘就判定「缺失」会把正常的回放误报为失败。
const replayLookupWait = 5 * time.Second

// replayLookupPoll 落盘等待的轮询间隔。
const replayLookupPoll = 250 * time.Millisecond

// replayRemuxTimeout 单场录制 remux 上限（`-c copy` 只搬容器，分钟级素材秒级完成）。
const replayRemuxTimeout = 30 * time.Minute

// replayProbeTimeout ffprobe 探测上限。
const replayProbeTimeout = 60 * time.Second

// LiveReplayDeps 回放的跨域依赖（router 装配注入，live 不反向构造 video 内部结构）。
type LiveReplayDeps struct {
	Video *video.Service
	Store storage.Storage
}

// SetReplayDeps 注入回放依赖（router 装配）。
func (s *Service) SetReplayDeps(deps *LiveReplayDeps) { s.replayDeps = deps }

// replayCfg 回放配置（cfg 为空时返回安全的零配置，避免装配遗漏导致 panic）。
func (s *Service) replayCfg() config.LiveReplay {
	if s.cfg == nil {
		return config.LiveReplay{}
	}
	return s.cfg.Live.Replay
}

// OnSessionEnded 场次结束时的回放素材登记入口（stopRoom 调用）。
//
// 调用点必须在**房间状态落库之后**：本方法内部任何失败都不能回滚房间状态。
// 开关关闭时直接返回（dev 默认关，实测无副作用）。
func (s *Service) OnSessionEnded(ctx context.Context, room *LiveRoom) {
	rc := s.replayCfg()
	if !rc.Enabled {
		return
	}
	if room == nil || room.StartedAt == nil || room.EndedAt == nil {
		// 缺少场次时刻 → 无法构造幂等键（uk_room_session），登记出来的是脏数据
		s.logger().Warn("回放登记跳过：场次时刻缺失", zap.Int64("room", roomIDOrZero(room)))
		return
	}
	rep := &LiveReplay{
		RoomID:           room.ID,
		AnchorID:         room.AnchorID,
		SessionStartedAt: *room.StartedAt,
		SessionEndedAt:   *room.EndedAt,
		Status:           ReplayStatusPending,
	}
	created, err := s.repo.CreateReplay(rep)
	if err != nil {
		s.logger().Error("回放素材登记失败", zap.Int64("room", room.ID), zap.Error(err))
		return
	}
	if !created {
		// 同场次重复投递（SRS 回调重试）：幂等退化为无操作，不重复生成稿件
		s.logger().Info("回放素材已登记过，忽略重复投递", zap.Int64("room", room.ID))
		return
	}
	s.logger().Info("回放素材已登记", zap.Int64("room", room.ID),
		zap.Int64("replay", rep.ID), zap.Int64("anchor", room.AnchorID))

	if !rc.AutoGenerate {
		// 只登记不生成：运维可事后手动补生成（plan liv14 §3.6 开关语义）
		s.logger().Info("回放自动生成未启用，仅登记素材", zap.Int64("replay", rep.ID))
		return
	}
	s.startReplayGenerate(rep.ID)
}

func roomIDOrZero(r *LiveRoom) int64 {
	if r == nil {
		return 0
	}
	return r.ID
}

// startReplayGenerate 异步生成回放稿件（panic 自隔离：worker goroutine 不受 HTTP Recovery 保护）。
func (s *Service) startReplayGenerate(replayID int64) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				s.logger().Error("回放生成 panic，已隔离", zap.Any("recover", r), zap.Int64("replay", replayID))
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), replayRemuxTimeout+5*time.Minute)
		defer cancel()
		if err := s.GenerateReplay(ctx, replayID); err != nil {
			s.logger().Error("回放生成失败（不影响下播）", zap.Int64("replay", replayID), zap.Error(err))
		}
	}()
}

// GenerateReplay 生成回放稿件：找录制 → 校验时长 → remux 进存储 → 建草稿稿件 → 回填登记。
//
// 失败一律落在 live_replay.status/fail_reason 并返回 nil 之外的 error 供日志；
// **调用方不得据此改变房间状态**（spec LIV-14 失败处理）。
func (s *Service) GenerateReplay(ctx context.Context, replayID int64) error {
	rep, err := s.repo.ReplayByID(replayID)
	if err != nil {
		return err
	}
	if rep == nil {
		return fmt.Errorf("回放登记不存在: %d", replayID)
	}
	if rep.Status != ReplayStatusPending {
		return nil // 已处理过（幂等）
	}
	rc := s.replayCfg()
	if s.replayDeps == nil || s.replayDeps.Video == nil {
		return s.failReplay(rep, "回放依赖未装配（video 服务缺失）")
	}
	resolver, ok := s.replayDeps.Store.(storage.PathResolver)
	if !ok || resolver == nil {
		return s.failReplay(rep, "存储驱动不支持本地回放（需 PathResolver）")
	}

	// 1) 找本场录制文件（等待 SRS flush，见 replayLookupWait）
	rel, abs, err := s.lookupDvrFile(rep.RoomID, replayLookupWait)
	if err != nil {
		return s.failReplay(rep, err.Error())
	}
	fi, err := os.Stat(abs)
	if err != nil || fi.Size() == 0 {
		return s.failReplay(rep, "录制文件缺失或为空")
	}

	// 2) 时长上限（plan liv14 D2）：超限直接拒绝，不做无界转码/上传
	dur, err := s.probeDuration(ctx, abs)
	if err != nil {
		return s.failReplay(rep, "录制文件不可解析: "+err.Error())
	}
	maxSec := rc.MaxDurationHours * 3600
	if maxSec > 0 && dur > maxSec {
		reason := fmt.Sprintf("直播时长 %d 秒超出上限 %d 小时，未生成回放", dur, rc.MaxDurationHours)
		if uErr := s.repo.UpdateReplayFields(rep.ID, map[string]any{
			"status": ReplayStatusRejected, "fail_reason": reason,
			"dvr_key": rel, "duration": dur, "file_size": fi.Size(),
		}); uErr != nil {
			return uErr
		}
		s.logger().Info("回放超时长上限已拒绝登记", zap.Int64("replay", rep.ID), zap.Int("duration", dur))
		return nil
	}

	// 3) 录制文件 → 转码源（FLV 原地保留；remux 成 MP4 进存储）。
	//    必须 remux 而不是把 .flv 直接当播放源：/static 的签名闸只覆盖 videos/**/*.{m3u8,mp4}，
	//    .flv 路径**不需要签名**即被 8180 直出——未发布回放的原始画面会因此对全站公开
	//    （plan liv14 §3.4 的可见性边界）。
	srcKey := fmt.Sprintf("videos/replay/%d/%d.mp4", rep.RoomID, rep.SessionEndedAt.Unix())
	srcAbs := resolver.LocalPath(srcKey)
	if err := os.MkdirAll(filepath.Dir(srcAbs), 0o755); err != nil {
		return s.failReplay(rep, "创建转码源目录失败: "+err.Error())
	}
	if err := s.remuxToMP4(ctx, abs, srcAbs); err != nil {
		return s.failReplay(rep, "录制文件 remux 失败: "+err.Error())
	}
	sfi, err := os.Stat(srcAbs)
	if err != nil || sfi.Size() == 0 {
		return s.failReplay(rep, "转码源产物缺失或为空")
	}

	// 4) 建草稿稿件（D5：owner=主播、标题自动生成、分区取配置或首个启用分区）
	title := s.replayTitle(rep, dur)
	desc := s.replayDesc(rep, rel)
	detail, videoID, err := s.replayDeps.Video.CreateReplayDraft(ctx, &video.CreateReplayDraftReq{
		UserID:     rep.AnchorID,
		Title:      title,
		Desc:       desc,
		SourceKey:  srcKey,
		SourceSize: sfi.Size(),
		CategoryID: int(rc.CategoryID),
	})
	if err != nil {
		return s.failReplay(rep, "生成回放稿件失败: "+err.Error())
	}

	if err := s.repo.UpdateReplayFields(rep.ID, map[string]any{
		"status": ReplayStatusDrafted, "dvr_key": rel, "source_key": srcKey,
		"video_id": videoID, "duration": dur, "file_size": fi.Size(), "fail_reason": "",
	}); err != nil {
		return err
	}
	s.logger().Info("回放草稿已生成（等待主播发布）",
		zap.Int64("replay", rep.ID), zap.Int64("video", videoID),
		zap.String("bvid", detail.Bvid), zap.Int("duration", dur))
	return nil
}

// failReplay 记录失败并返回原错误（登记必须落库，否则「这场直播为何没回放」无从追溯）。
func (s *Service) failReplay(rep *LiveReplay, reason string) error {
	if err := s.repo.UpdateReplayFields(rep.ID, map[string]any{
		"status": ReplayStatusFailed, "fail_reason": truncateReason(reason),
	}); err != nil {
		s.logger().Error("回放失败状态回写失败", zap.Int64("replay", rep.ID), zap.Error(err))
	}
	s.logger().Warn("回放生成失败", zap.Int64("replay", rep.ID), zap.String("reason", reason))
	return fmt.Errorf("%s", reason)
}

// truncateReason fail_reason 列宽 255（迁移 0042），超长截断而非让整条回写失败。
func truncateReason(s string) string {
	r := []rune(s)
	if len(r) <= 255 {
		return s
	}
	return string(r[:255])
}

// replayTitle 自动标题：直播回放 · <房间标题> · <日期>（plan liv14 D5）。
//
// 房间标题在登记后可能被主播改掉，故优先用**登记时的房间标题**（当前值）；
// 房间已被删除时退化为 stream 名，标题仍然可用。
func (s *Service) replayTitle(rep *LiveReplay, duration int) string {
	title := ""
	if room, err := s.repo.RoomByID(rep.RoomID); err == nil && room != nil {
		title = strings.TrimSpace(room.Title)
	}
	if title == "" {
		title = fmt.Sprintf("直播间 %d", rep.RoomID)
	}
	t := fmt.Sprintf("直播回放 · %s · %s", title, rep.SessionEndedAt.Format("2006-01-02"))
	if len([]rune(t)) > 80 { // video.title VARCHAR(80)
		t = string([]rune(t)[:80])
	}
	return t
}

// replayDesc 简介：写明来源房间、场次与录制来源，便于合规追溯（spec LIV-14 关联条款）。
func (s *Service) replayDesc(rep *LiveReplay, dvrRel string) string {
	return fmt.Sprintf("本视频为直播间 %d 于 %s 至 %s 的直播回放（自动生成）。\n录制文件: %s",
		rep.RoomID,
		rep.SessionStartedAt.Format("2006-01-02 15:04:05"),
		rep.SessionEndedAt.Format("2006-01-02 15:04:05"),
		dvrRel)
}

// ---- DVR 录制文件定位 ----

// dvrDir 配置的 DVR 落盘目录（相对路径按后端工作目录解析）。
func (s *Service) dvrDir() string {
	dir := s.replayCfg().DvrDir
	if dir == "" {
		dir = "./deploy/srs/dvr"
	}
	return dir
}

// lookupDvrFile 找该房间最新的一段录制（返回相对 dvrDir 的路径与绝对路径）。
//
// SRS 的 dvr_path 模式是 `[app]/[stream].[timestamp].flv`，故文件名形如
// `<dvrDir>/live/room_<id>.<ts>.flv`。同一房间的场次是**串行**的（一主播一房间、
// 同时只有一个推流），因此「该房间最新的录制文件」就是刚结束这场——按 mtime 取最新即可，
// 不需要（也无法可靠地）从文件名反推场次。
//
// waitFor > 0 时轮询等待文件出现且非空：SRS 回 on_unpublish 2xx 与关闭 FLV 文件之间
// 存在毫秒级竞争，立即读取会把正常回放误判为「录制缺失」。
func (s *Service) lookupDvrFile(roomID int64, waitFor time.Duration) (rel, abs string, err error) {
	deadline := time.Now().Add(waitFor)
	for {
		rel, abs, err = s.findNewestDvr(roomID)
		if err == nil {
			return rel, abs, nil
		}
		if time.Now().After(deadline) {
			return "", "", err
		}
		time.Sleep(replayLookupPoll)
	}
}

// findNewestDvr 扫 dvrDir 下匹配 room_<id>.* 的录制文件，取 mtime 最新的非空文件。
func (s *Service) findNewestDvr(roomID int64) (rel, abs string, err error) {
	dir := s.dvrDir()
	root, err := filepath.Abs(dir)
	if err != nil {
		return "", "", fmt.Errorf("录制目录不可用: %w", err)
	}
	pattern := filepath.Join(root, "*", StreamName(roomID)+".*")
	matches, gErr := filepath.Glob(pattern)
	if gErr != nil {
		return "", "", fmt.Errorf("扫描录制目录失败: %w", gErr)
	}
	type cand struct {
		path string
		mod  time.Time
		size int64
	}
	cands := make([]cand, 0, len(matches))
	for _, m := range matches {
		fi, sErr := os.Stat(m)
		if sErr != nil || fi.IsDir() || fi.Size() == 0 {
			continue // 仅 0 字节的占位文件不算录制成品
		}
		cands = append(cands, cand{path: m, mod: fi.ModTime(), size: fi.Size()})
	}
	if len(cands) == 0 {
		return "", "", fmt.Errorf("未找到房间 %d 的录制文件（%s，请确认 SRS DVR 已启用）", roomID, pattern)
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].mod.After(cands[j].mod) })
	best := cands[0]
	r, rErr := filepath.Rel(root, best.path)
	if rErr != nil {
		r = best.path
	}
	return filepath.ToSlash(r), best.path, nil
}

// ---- ffmpeg 调用 ----

// ffprobeBin ffprobe 可执行名（复用转码配置，缺省走 PATH）。
func (s *Service) ffprobeBin() string {
	if s.cfg != nil && s.cfg.Transcode.FfprobePath != "" {
		return s.cfg.Transcode.FfprobePath
	}
	return "ffprobe"
}

// probeDuration 读录制时长（秒）；不可解析时返回错误（损坏素材据此登记失败）。
func (s *Service) probeDuration(ctx context.Context, src string) (int, error) {
	cctx, cancel := context.WithTimeout(ctx, replayProbeTimeout)
	defer cancel()
	out, err := exec.CommandContext(cctx, s.ffprobeBin(),
		"-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", src).Output()
	if err != nil {
		return 0, err
	}
	sec, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 0, err
	}
	return int(sec + 0.5), nil
}

// remuxToMP4 把 DVR 的 FLV 原样搬进 MP4 容器（`-c copy`，不重编码）。
//
// faststart 把 moov 前置：回放稿件发布后由浏览器顺序拉流播放，moov 在尾部时
// 播放器必须先跳到文件末尾才能起播（长直播的观感差异很明显）。
func (s *Service) remuxToMP4(ctx context.Context, src, dst string) error {
	cctx, cancel := context.WithTimeout(ctx, replayRemuxTimeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, s.ffmpegBin(),
		"-y", "-loglevel", "error",
		"-i", src,
		"-c", "copy", "-movflags", "+faststart",
		"-f", "mp4", dst,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// ---- 主播侧：发布 / 删除 / 列表 ----

// ReplayItem 主播「我的回放」条目（关联稿件状态，供前端区分草稿/已发布）。
type ReplayItem struct {
	ID         int64  `json:"id"`
	RoomID     int64  `json:"room_id"`
	VideoID    int64  `json:"video_id"`
	Bvid       string `json:"bvid"`
	Title      string `json:"title"`
	VideoState int8   `json:"video_status"` // video 表状态（0 草稿 / 4 已发布）——可见性权威
	Duration   int    `json:"duration"`
	FileSize   int64  `json:"file_size"`
	EndedAt    string `json:"session_ended_at"`
	Status     int8   `json:"status"`      // 回放生成状态（live_replay.status）
	FailReason string `json:"fail_reason"` // 生成失败原因（可追溯）
}

// MyReplays 主播的回放列表（含草稿与已发布，供「可选发布」的界面选人）。
func (s *Service) MyReplays(ctx context.Context, uid int64, limit int) ([]ReplayItem, error) {
	if !s.replayCfg().Enabled {
		return nil, errcode.ErrLiveReplayDisabled
	}
	room, err := s.repo.RoomByAnchor(uid)
	if err != nil {
		return nil, err
	}
	if room == nil {
		return nil, errcode.ErrLiveRoomNotFound
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	list, err := s.repo.ReplaysByAnchor(uid, limit)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return []ReplayItem{}, nil
	}
	ids := make([]int64, 0, len(list))
	for _, r := range list {
		ids = append(ids, r.VideoID)
	}
	metas := map[int64]video.ReplayBrief{}
	if s.replayDeps != nil && s.replayDeps.Video != nil {
		metas, err = s.replayDeps.Video.ReplayBriefs(ids)
		if err != nil {
			return nil, err
		}
	}
	items := make([]ReplayItem, 0, len(list))
	for _, r := range list {
		it := ReplayItem{
			ID: r.ID, RoomID: r.RoomID, VideoID: r.VideoID, Duration: r.Duration,
			FileSize: r.FileSize, EndedAt: r.SessionEndedAt.Format("2006-01-02 15:04:05"),
			Status: r.Status, FailReason: r.FailReason,
		}
		if m, ok := metas[r.VideoID]; ok {
			it.Bvid, it.Title, it.VideoState = m.Bvid, m.Title, m.Status
		}
		items = append(items, it)
	}
	return items, nil
}

// ReplayOwner 回放归属判定：稿件必须由本主播的某个直播间生成（防越权发布/删除他人回放）。
func (s *Service) ReplayOwner(videoID, uid int64) (*LiveReplay, error) {
	rep, err := s.repo.ReplayByVideo(videoID)
	if err != nil {
		return nil, err
	}
	if rep == nil || rep.AnchorID != uid {
		// 不区分「不存在」与「非本人」：避免探测他人稿件是否为直播回放
		return nil, errcode.ErrLiveReplayNotFound
	}
	return rep, nil
}

// PublishReplay 主播发布自己的回放（草稿 → 已发布，spec LIV-14「主播可选发布」）。
func (s *Service) PublishReplay(ctx context.Context, uid, videoID int64) error {
	if !s.replayCfg().Enabled {
		return errcode.ErrLiveReplayDisabled
	}
	rep, err := s.ReplayOwner(videoID, uid)
	if err != nil {
		return err
	}
	if s.replayDeps == nil || s.replayDeps.Video == nil {
		return errcode.ErrLiveReplayDisabled
	}
	// 先判稿件是否仍为草稿：重复发布要返回**回放域**的 91033，
	// 而不是让 video 层抛通用的参数错误（10002）——直播域的错误码要自洽。
	if metas, mErr := s.replayDeps.Video.ReplayBriefs([]int64{rep.VideoID}); mErr == nil {
		if m, ok := metas[rep.VideoID]; ok && m.Status != video.StatusDraft {
			return errcode.ErrLiveReplayNotDraft
		}
	}
	return s.replayDeps.Video.PublishDraft(ctx, videoID, uid)
}

// DeleteReplay 主播删除自己的回放（软删；登记行保留以维追溯）。
func (s *Service) DeleteReplay(ctx context.Context, uid, videoID int64) error {
	if !s.replayCfg().Enabled {
		return errcode.ErrLiveReplayDisabled
	}
	if _, err := s.ReplayOwner(videoID, uid); err != nil {
		return err
	}
	if s.replayDeps == nil || s.replayDeps.Video == nil {
		return errcode.ErrLiveReplayDisabled
	}
	return s.replayDeps.Video.DeleteReplay(ctx, videoID, uid)
}

// RoomReplays 某房间的回放登记（合规追溯：管理端/巡查按房间查「这场直播留了什么」）。
func (s *Service) RoomReplays(roomID int64) ([]LiveReplay, error) {
	return s.repo.ReplaysByRoom(roomID)
}

// ---- 保留期清理（spec §4：直播内容存档 ≥60 天）----

// replayCleanBatch 单轮清理处理的登记数上限（避免一次删太多拖住 ticker）。
const replayCleanBatch = 200

// StartReplayCleaner 启动保留期清理任务（进程内定时器，与截帧机审 worker 同风格）。
//
// **刻意放在 go 里跑**：StartAuditWorker 曾在启动路径内联跑循环，导致 router.New 永不返回、
// HTTP 服务从未监听（M4-LIV-05 实测踩到）。此处不再重犯。
func (s *Service) StartReplayCleaner(ctx context.Context) {
	rc := s.replayCfg()
	interval := time.Duration(rc.CleanIntervalSec) * time.Second
	if interval <= 0 {
		interval = time.Hour
	}
	s.logger().Info("回放保留期清理任务已启动",
		zap.Int("retention_days", rc.RetentionDays), zap.Duration("interval", interval))
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if n, err := s.CleanExpiredReplays(ctx); err != nil {
					s.logger().Error("回放保留期清理失败", zap.Error(err))
				} else if n > 0 {
					s.logger().Info("回放录制素材已按保留期清理", zap.Int("count", n))
				}
			}
		}
	}()
}

// CleanExpiredReplays 按配置天数删除过期录制素材（返回清理条数）。
//
// 删除范围**仅是录制文件**（dvr 原片 + 转码源），稿件与登记行都保留：
//   - 稿件是主播的内容资产，保留期针对的是「推流全量录制」这份合规存档义务；
//   - 登记行保留才能回答「这场直播的录制何时被清理」（plan liv14 §3.5）。
//
// 稿件已发布时**不删转码源**：它可能仍是该稿件原画流的播放源
// （转码关闭时原画是唯一产物），删掉等于把已发布稿件打坏。
func (s *Service) CleanExpiredReplays(ctx context.Context) (int, error) {
	rc := s.replayCfg()
	if !rc.Enabled || rc.RetentionDays <= 0 {
		return 0, nil
	}
	before := time.Now().AddDate(0, 0, -rc.RetentionDays)
	list, err := s.repo.ExpiredReplaySources(before, replayCleanBatch)
	if err != nil {
		return 0, err
	}
	if len(list) == 0 {
		return 0, nil
	}
	// 发布态判定需要 video 侧读模型；未装配时保守起见不删转码源（只删 DVR 原片）
	var published map[int64]bool
	if s.replayDeps != nil && s.replayDeps.Video != nil {
		ids := make([]int64, 0, len(list))
		for _, r := range list {
			if r.VideoID > 0 {
				ids = append(ids, r.VideoID)
			}
		}
		if len(ids) > 0 {
			if metas, mErr := s.replayDeps.Video.ReplayBriefs(ids); mErr == nil {
				published = make(map[int64]bool, len(metas))
				for id, m := range metas {
					published[id] = m.Status == video.StatusPublished
				}
			}
		}
	}

	var resolver storage.PathResolver
	if s.replayDeps != nil && s.replayDeps.Store != nil {
		resolver, _ = s.replayDeps.Store.(storage.PathResolver)
	}

	cleaned := 0
	for _, r := range list {
		removed := make([]string, 0, 2)
		// 1) DVR 原片（相对 dvrDir）
		if r.DvrKey != "" {
			p := filepath.Join(s.dvrDir(), filepath.FromSlash(r.DvrKey))
			if err := os.Remove(p); err != nil {
				if !os.IsNotExist(err) {
					s.logger().Warn("删除录制原片失败", zap.String("path", p), zap.Error(err))
					continue // 删不掉就不标记已清理，下轮重试
				}
				// 文件已不在（人工清理/上轮删了一半）：仍算已达清理目标，照常标记
				removed = append(removed, "dvr(已缺失)")
			} else {
				removed = append(removed, "dvr")
			}
		}
		// 2) 转码源（存储 key）：仅当稿件未发布时删
		if resolver != nil && r.SourceKey != "" && r.VideoID > 0 && !published[r.VideoID] {
			p := resolver.LocalPath(r.SourceKey)
			if err := os.Remove(p); err != nil {
				if !os.IsNotExist(err) {
					s.logger().Warn("删除回放转码源失败", zap.String("path", p), zap.Error(err))
				} else {
					removed = append(removed, "source(已缺失)")
				}
			} else {
				removed = append(removed, "source")
			}
		} else if r.VideoID > 0 && published[r.VideoID] {
			// 稿件已发布：转码源可能仍是其原画流播放源，保留（plan liv14 §3.5）
			removed = append(removed, "source(已发布稿件保留)")
		}
		reason := fmt.Sprintf("录制素材已按保留期 %d 天清理（%s）",
			rc.RetentionDays, strings.Join(removed, "+"))
		if err := s.repo.UpdateReplayFields(r.ID, map[string]any{
			"status": ReplayStatusPurged, "fail_reason": reason,
		}); err != nil {
			return cleaned, err
		}
		cleaned++
	}
	return cleaned, nil
}
