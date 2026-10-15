package live

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg" // 截帧图与外部图审回传可能是 jpeg
	_ "image/png"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/dlidli/server/internal/pkg/contentmod"
	"github.com/dlidli/server/internal/pkg/errcode"
	"go.uber.org/zap"
)

// 截帧机审与快速切断（M4-LIV-05，spec §5E / plan §2D）。
//
// 设计要点：
//  1. 截帧源选 HTTP-FLV（实时转封装，首帧即截得到），不选 HLS（切片式，要多等一个切片周期）；
//     SRS 5 无截图 API，故必须由后端调 ffmpeg（plan §2D.1）。
//  2. 机审走 pkg/contentmod；机审不可用时 fail-open —— 不切断，只记 degraded（plan §2D.2）。
//  3. 切断优先 SRS API 踢流（成功即由 on_unpublish 回调置房间已下播，LIV-01 既有链路）；
//     不可达则降级为仅封禁 + 流水标 cut_mode=degraded（plan §2D.3）。
//  4. 封禁（ban_status）与场次状态（status）正交，不复用枚举，避免污染列表查询与开播校验。

// srsHTTPTimeout SRS API 单次调用上限：踢流是快速切断的一环，宁可快速降级也不能挂住。
const srsHTTPTimeout = 5 * time.Second

// srsCodeNoTarget SRS 对「目标不存在」的返回码（实测 SRS 5.0.225）。
const srsCodeNoTarget = 2049

// roomStreamName 房间的流名（plan liv01 D1：公开、不含密钥）。
func roomStreamName(roomID int64) string { return fmt.Sprintf("room_%d", roomID) }

// CutResult 切断处置结果。
type CutResult struct {
	Kicked      bool  `json:"kicked"`       // SRS 侧是否确已踢掉推流
	CutMode     int8  `json:"cut_mode"`     // 1 srs_api / 2 degraded
	ViolationID int64 `json:"violation_id"` // 违规流水 id
}

// ---- SRS HTTP API 客户端 ----

// srsClient SRS HTTP API 抽象（接口化便于测试注入桩）。
type srsClient interface {
	// FindPublisher 找该房间当前推流客户端标识；无推流返回 ""。
	FindPublisher(ctx context.Context, streamName string) (string, error)
	// KickPublisher 踢掉指定客户端；目标已不存在视为成功（幂等）。
	KickPublisher(ctx context.Context, clientID string) error
}

type srsHTTP struct {
	base string
	http *http.Client
}

// srsStreamsResp GET /api/v1/streams/ 响应。
type srsStreamsResp struct {
	Code    int         `json:"code"`
	Streams []srsStream `json:"streams"`
}

// srsStream 一个流。字段名按 SRS 5.0.225 实测填写：
// name=流名、publish.active=是否推流中、publish.cid=推流客户端 id。
// SRS 各版本间存在 name/stream 与 cid/id 的差异，故两类都留兜底取值。
type srsStream struct {
	Name    string `json:"name"`
	Stream  string `json:"stream"`
	Publish struct {
		Active bool   `json:"active"`
		CID    string `json:"cid"`
		ID     string `json:"id"`
	} `json:"publish"`
}

func (st srsStream) clientID() string {
	if st.Publish.CID != "" {
		return st.Publish.CID
	}
	return st.Publish.ID
}

func (st srsStream) streamName() string {
	if st.Name != "" {
		return st.Name
	}
	return st.Stream
}

// FindPublisher 查该房间的推流客户端 id。
// 实测依据：GET /api/v1/streams/（**尾斜杠必需**，无斜杠 302）→ {"code":0,"streams":[...]}。
func (s *srsHTTP) FindPublisher(ctx context.Context, streamName string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, srsHTTPTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.base+"/api/v1/streams/", nil)
	if err != nil {
		return "", err
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	var out srsStreamsResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.Code != 0 {
		return "", fmt.Errorf("SRS streams 接口 code=%d", out.Code)
	}
	for _, st := range out.Streams {
		if !st.Publish.Active {
			continue
		}
		if !matchesRoomStream(st.streamName(), streamName) {
			continue
		}
		if cid := st.clientID(); cid != "" {
			return cid, nil
		}
	}
	return "", nil
}

// matchesRoomStream 流名匹配：精确相等，或流名后带查询串（SRS 把推流 URL 查询串并进流名的版本）。
// 前缀必须整段匹配——room_10 不得命中 room_102。
func matchesRoomStream(name, streamName string) bool {
	if name == streamName {
		return true
	}
	return strings.HasPrefix(name, streamName+"?")
}

// srsKickResp DELETE /api/v1/clients/<id> 响应：成功 {"code":0}，目标不存在 {"code":2049}。
type srsKickResp struct {
	Code int `json:"code"`
}

// KickPublisher 踢掉推流客户端。code==0 或 2049（目标不存在）均视为成功（幂等）。
func (s *srsHTTP) KickPublisher(ctx context.Context, clientID string) error {
	if clientID == "" {
		return errcode.ErrLiveNoActiveStream
	}
	ctx, cancel := context.WithTimeout(ctx, srsHTTPTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete,
		s.base+"/api/v1/clients/"+url.PathEscape(clientID), nil)
	if err != nil {
		return err
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	var out srsKickResp
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if out.Code == 0 || out.Code == srsCodeNoTarget {
		return nil
	}
	return fmt.Errorf("SRS 踢流失败 HTTP %d code=%d", resp.StatusCode, out.Code)
}

// newSRSClient 按配置构造 SRS API 客户端；基址为空返回 nil（调用方按降级处理）。
func newSRSClient(base string) srsClient {
	if strings.TrimSpace(base) == "" {
		return nil
	}
	return &srsHTTP{
		base: strings.TrimSuffix(base, "/"),
		http: &http.Client{Timeout: srsHTTPTimeout},
	}
}

// ---- 截帧 ----

// flvURL 该房间的 HTTP-FLV 拉流地址（与房间详情下发的 play_urls.flv 同口径）。
func (s *Service) flvURL(roomID int64) string {
	if s.cfg == nil || s.cfg.Live.PlayDomain == "" {
		return ""
	}
	domain := s.cfg.Live.PlayDomain
	scheme := "http://"
	if strings.HasPrefix(domain, "http://") || strings.HasPrefix(domain, "https://") {
		scheme = ""
	}
	return fmt.Sprintf("%s%s/%s/%s.flv", scheme, domain, s.streamApp(), roomStreamName(roomID))
}

// ffmpegBin 截帧用 ffmpeg 可执行名（复用转码配置，缺省走 PATH）。
func (s *Service) ffmpegBin() string {
	if s.cfg != nil && s.cfg.Transcode.FfmpegPath != "" {
		return s.cfg.Transcode.FfmpegPath
	}
	return "ffmpeg"
}

// captureFrame 从该房间的 HTTP-FLV 拉流截取单帧。
//
// 返回落盘路径与图片尺寸；文件名含房间与纳秒时间戳，避免相邻周期互相覆盖。
// **图片不删除**：spec §5E.5 要求截帧图留存 ≥60 天（image_key 即其存储键）。
func (s *Service) captureFrame(ctx context.Context, roomID int64) (path string, w, h int, err error) {
	auditCfg := s.cfg.Live.Audit
	flv := s.flvURL(roomID)
	if flv == "" {
		return "", 0, 0, fmt.Errorf("拉流地址不可用（live.playDomain 未配置）")
	}
	if err := os.MkdirAll(auditCfg.FrameDir, 0o755); err != nil {
		return "", 0, 0, err
	}
	out := filepath.Join(auditCfg.FrameDir, fmt.Sprintf("room_%d_%d.png", roomID, time.Now().UnixNano()))

	cctx, cancel := context.WithTimeout(ctx, time.Duration(auditCfg.FlvTimeoutSeconds)*time.Second)
	defer cancel()

	// -frames:v 1：收到首个可解码视频帧即退出（无需 -t 限时）
	cmd := exec.CommandContext(cctx, s.ffmpegBin(),
		"-y", "-loglevel", "error",
		"-i", flv,
		"-frames:v", "1", "-f", "image2", out,
	)
	if b, cErr := cmd.CombinedOutput(); cErr != nil {
		return "", 0, 0, fmt.Errorf("ffmpeg 截帧失败: %w (%s)", cErr, strings.TrimSpace(string(b)))
	}
	fi, sErr := os.Stat(out)
	if sErr != nil || fi.Size() == 0 {
		return "", 0, 0, fmt.Errorf("截帧产物缺失或为空: %v", sErr)
	}
	w, h = imageSize(out)
	return out, w, h, nil
}

// imageSize 读图片像素尺寸；失败返回 0,0（尺寸仅用于审计留痕，不影响判定）。
func imageSize(path string) (int, int) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer func() { _ = f.Close() }()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0
	}
	return cfg.Width, cfg.Height
}

// ---- 审计一轮 ----

// AuditOnce 执行一轮截帧机审：遍历所有直播中房间，逐房间**串行**截帧 → 机审 → 落记录
// → 命中则走处置链。返回本轮审计房间数与命中数。
//
// 单房间任何错误只记一条 degraded 记录并继续（fail-open，plan §2D.2）：
// 不得中断整轮，不得因异常切断。
func (s *Service) AuditOnce(ctx context.Context) (audited, violations int, err error) {
	rooms, err := s.repo.ListLiveRoomsAll()
	if err != nil {
		return 0, 0, err
	}
	for i := range rooms {
		room := &rooms[i]
		if room.BanStatus == RoomBanApplied {
			continue // 已封禁房间不再重复判定
		}
		audited++
		hitted, rErr := s.auditRoom(ctx, &rooms[i])
		if rErr != nil {
			s.logger().Warn("截帧机审房间处理异常（fail-open，不切断）",
				zap.Int64("room", room.ID), zap.Error(rErr))
			continue
		}
		if hitted {
			violations++
		}
	}
	return audited, violations, nil
}

// auditRoom 单房间一轮：截帧 → 机审 → 落 live_frame_audit → 命中且非降级则切断。
// 返回是否命中违规；异常路径一律落 degraded 记录并返回 (false, err)。
func (s *Service) auditRoom(ctx context.Context, room *LiveRoom) (bool, error) {
	start := time.Now()
	auditCfg := s.cfg.Live.Audit

	rec := &LiveFrameAudit{
		RoomID:   room.ID,
		AnchorID: room.AnchorID,
		Pass:     true,
	}
	path, w, h, capErr := s.captureFrame(ctx, room.ID)
	rec.Width, rec.Height = w, h
	if capErr != nil {
		// 截帧失败 = 无法判定：记 degraded，不切断（fail-open）
		rec.Degraded = true
		rec.HitWords = "截帧失败"
		rec.CostMs = int(time.Since(start).Milliseconds())
		if err := s.repo.CreateFrameAudit(rec); err != nil {
			return false, err
		}
		return false, capErr
	}
	// 截帧图**留存**（spec §5E.5：截帧图与判定记录留存 ≥60 天）——不删原始帧，
	// image_key 即其存储键，供合规追溯与人工复核。
	res := contentmod.CheckImage(contentmod.SceneLive, path, auditCfg.BlackYAvgMax, auditCfg.FlatRangeMax)
	rec.ImageKey = filepath.Base(path)
	rec.Pass = res.Pass
	rec.Degraded = res.Degraded
	rec.HitWords = strings.Join(res.Words, ",")
	rec.CostMs = int(time.Since(start).Milliseconds())

	if err := s.repo.CreateFrameAudit(rec); err != nil {
		return false, err
	}
	// 降级（机审不可用）或通过 → 不切断（直播可用性优先，plan §2D.2 待替换点）
	if res.Degraded || res.Pass {
		return false, nil
	}
	if _, err := s.CutRoom(ctx, room.ID, ViolationSourceAuto, rec.HitWords, rec.ID, 0); err != nil {
		return true, err
	}
	return true, nil
}

// ---- 处置链（快速切断）----

// CutRoom 违规处置：落违规流水 → SRS 踢流 → 房间封禁。
//
// 顺序刻意是「先落流水再切断」：保证「判定发生过」不被切断失败吞掉（spec §5E.5 可审计）。
// SRS API 不可达时降级为仅封禁 + 流水标 cut_mode=degraded，不静默（plan §2D.3）。
// 房间 status 不在此处改：踢流成功后由 SRS on_unpublish 回调置已下播（LIV-01 既有链路）。
func (s *Service) CutRoom(ctx context.Context, roomID int64, source int8, reason string,
	frameAuditID, cutBy int64) (*CutResult, error) {
	room, err := s.repo.RoomByID(roomID)
	if err != nil {
		return nil, err
	}
	if room == nil {
		return nil, errcode.ErrLiveRoomNotFound
	}
	if room.BanStatus == RoomBanApplied {
		return nil, errcode.ErrLiveRoomBanned
	}
	// 房间未在直播中：无可切断对象，返回可区分错误且**不产生违规流水**（spec §5E.3）。
	// 注意顺序：先判封禁再判场次状态——已封禁房间重复切断要报 91028 而非 91006。
	if room.Status != RoomStatusLive {
		return nil, errcode.ErrLiveNotLive
	}
	if strings.TrimSpace(reason) == "" {
		reason = "内容违规"
	}

	// 1) 先落违规流水（cut_mode 先置 0，实际结果随后回填——流水必须存在）
	vlog := &LiveViolationLog{
		RoomID:   roomID,
		AnchorID: room.AnchorID,
		Source:   source,
		Reason:   reason,
		CutBy:    cutBy,
	}
	if frameAuditID > 0 {
		id := frameAuditID
		vlog.FrameAuditID = &id
	}
	if err := s.repo.CreateViolation(vlog); err != nil {
		return nil, err
	}

	// 2) SRS API 踢流（确定性主通道）；失败/不可达则降级
	cutMode := int8(CutModeDegraded)
	kicked := false
	if cli := newSRSClient(s.cfg.Live.Audit.SrsAPIBase); cli != nil {
		switch cid, fErr := cli.FindPublisher(ctx, roomStreamName(roomID)); {
		case fErr != nil:
			s.logger().Warn("SRS API 查流失败，降级为仅封禁",
				zap.Int64("room", roomID), zap.Error(fErr))
		case cid == "":
			// 无推流：无处可踢，封禁照做。这是「流本就不在」，非 SRS 不可达，不算降级。
			s.logger().Info("切断时无在推流客户端，仅封禁", zap.Int64("room", roomID))
			cutMode = CutModeSrsAPI
		default:
			if kErr := cli.KickPublisher(ctx, cid); kErr != nil {
				s.logger().Warn("SRS API 踢流失败，降级为仅封禁",
					zap.Int64("room", roomID), zap.String("cid", cid), zap.Error(kErr))
			} else {
				kicked = true
				cutMode = CutModeSrsAPI
			}
		}
	} else {
		s.logger().Warn("SRS API 未配置，降级为仅封禁", zap.Int64("room", roomID))
	}

	// 3) 回填真实切断方式
	if err := s.repo.SetViolationCutMode(vlog.ID, cutMode); err != nil {
		s.logger().Error("回填违规流水切断方式失败", zap.Int64("violation", vlog.ID), zap.Error(err))
	}

	// 4) 房间封禁（与 status 正交；status 由 on_unpublish 回调置已下播）
	if err := s.repo.UpdateRoomFields(roomID, map[string]any{
		"ban_status": RoomBanApplied,
		"ban_reason": reason,
		"ban_by":     cutBy,
		"ban_at":     time.Now(),
	}); err != nil {
		return nil, err
	}

	s.logger().Info("直播快速切断已执行",
		zap.Int64("room", roomID), zap.Int8("source", source), zap.String("reason", reason),
		zap.Bool("kicked", kicked), zap.Int8("cut_mode", cutMode))
	return &CutResult{Kicked: kicked, CutMode: cutMode, ViolationID: vlog.ID}, nil
}

// RestoreRoom 解封（人工巡查的恢复动作）：ban_status 1→0。
// 已非封禁状态返回 91029，不产生副作用。
func (s *Service) RestoreRoom(ctx context.Context, roomID, adminID int64) error {
	room, err := s.repo.RoomByID(roomID)
	if err != nil {
		return err
	}
	if room == nil {
		return errcode.ErrLiveRoomNotFound
	}
	if room.BanStatus != RoomBanApplied {
		return errcode.ErrLiveRoomNotBanned
	}
	if err := s.repo.UpdateRoomFields(roomID, map[string]any{
		"ban_status": RoomBanNone,
		"ban_reason": "",
		"ban_by":     int64(0),
		"ban_at":     nil,
	}); err != nil {
		return err
	}
	s.logger().Info("直播房间已解封", zap.Int64("room", roomID), zap.Int64("admin", adminID))
	return nil
}

// ---- Worker 调度 ----

// StartAuditWorker 启动进程内截帧机审定时器（plan §2D.1）。
//
// 与 video/transcoder.go 的 StartTranscodeWorkers 同风格：**立即返回**，实际循环在
// 独立 goroutine 里跑（`go s.auditLoop(ctx)`）——否则会在 router 装配阶段阻塞住整个进程。
func (s *Service) StartAuditWorker(ctx context.Context) {
	go s.auditLoop(ctx)
}

// auditLoop 截帧机审主循环：time.Ticker + ctx.Done 退出，不引入新中间件。
// 未启用时直接返回并打日志（dev 默认关闭，spec §5E.1）。
func (s *Service) auditLoop(ctx context.Context) {
	auditCfg := s.cfg.Live.Audit
	if !auditCfg.Enabled {
		s.logger().Info("截帧机审未启用（live.audit.enabled=false）")
		return
	}
	interval := time.Duration(auditCfg.FrameIntervalSeconds) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	s.logger().Info("直播截帧机审 Worker 已启动",
		zap.Duration("interval", interval), zap.String("srs_api", auditCfg.SrsAPIBase))

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 单轮 panic 自隔离：一轮异常不能拖死 Worker（与转码 worker 同口径）
			func() {
				defer func() {
					if r := recover(); r != nil {
						s.logger().Error("截帧机审单轮 panic，已隔离", zap.Any("recover", r))
					}
				}()
				start := time.Now()
				audited, violations, err := s.AuditOnce(ctx)
				if err != nil {
					s.logger().Error("截帧机审一轮失败", zap.Error(err))
					return
				}
				s.logger().Info("截帧机审一轮完成",
					zap.Int("audited", audited), zap.Int("violations", violations),
					zap.Duration("cost", time.Since(start)))
			}()
		}
	}
}
