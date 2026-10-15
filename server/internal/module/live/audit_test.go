package live

import (
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dlidli/server/internal/pkg/errcode"
	"go.uber.org/zap"
)

// SRS 客户端解析（M4-LIV-05）==============================================

// realStreamsJSON 是 SRS 5.0.225 的 GET /api/v1/streams/ 实测响应原文
// （2026-10-15 真实推流 room_102 时抓取，见 .dev-logs/liv05/recon/）。
// 关键字段名：name=流名（**不是** stream，后者是内部 sid）、
// publish.active=是否推流中、publish.cid=客户端 id（DELETE 踢流用这个 id）。
const realStreamsJSON = `{"code":0,"server":"vid-02sei34","service":"1x8gk01n","pid":"1",
 "streams":[{"id":"vid-87m2t74","name":"room_102","vhost":"vid-4i5526w","app":"live",
 "tcUrl":"rtmp://127.0.0.1:1935/live","url":"/live/room_102","live_ms":1792033214352,
 "clients":1,"frames":0,"send_bytes":4292,"recv_bytes":18890,
 "kbps":{"recv_30s":0,"send_30s":0},
 "publish":{"active":true,"cid":"5i55qu40"},
 "video":{"codec":"H264","profile":"Baseline","level":"3","width":640,"height":360},
 "audio":null}]}`

// TestFindPublisherParsesRealSRSResponse 用真实 SRS 响应验证字段名与取值路径。
func TestFindPublisherParsesRealSRSResponse(t *testing.T) {
	var out srsStreamsResp
	if err := json.Unmarshal([]byte(realStreamsJSON), &out); err != nil {
		t.Fatalf("解析真实 SRS 响应失败: %v", err)
	}
	if out.Code != 0 || len(out.Streams) != 1 {
		t.Fatalf("code=%d streams=%d，want 0/1", out.Code, len(out.Streams))
	}
	st := out.Streams[0]

	// 流名取 name（实测 stream 字段是内部 sid "vid-87m2t74"，不是流名）
	if got := st.streamName(); got != "room_102" {
		t.Fatalf("流名 %q，want room_102（实测字段是 name，stream 是内部 sid）", got)
	}
	if !st.Publish.Active {
		t.Fatal("publish.active 应为 true")
	}
	if got := st.clientID(); got != "5i55qu40" {
		t.Fatalf("客户端 id %q，want 5i55qu40（实测字段是 publish.cid）", got)
	}
}

// TestMatchesRoomStream 流名匹配边界：只认精确或「后带查询串」，前缀必须整段。
func TestMatchesRoomStream(t *testing.T) {
	cases := []struct {
		name, stream string
		want         bool
	}{
		{"room_102", "room_102", true},
		{"room_102?key=abc", "room_102", true},
		{"room_10", "room_102", false}, // 前缀不得跨房间误命中
		{"room_1023", "room_102", false},
		{"other_102", "room_102", false},
		{"", "room_102", false},
	}
	for _, c := range cases {
		if got := matchesRoomStream(c.name, c.stream); got != c.want {
			t.Errorf("matchesRoomStream(%q,%q)=%v，want %v", c.name, c.stream, got, c.want)
		}
	}
}

// TestSRSKickCodeNoTarget SRS 对「目标不存在」返回 2049，须视为踢流成功（幂等）。
func TestSRSKickCodeNoTarget(t *testing.T) {
	if srsCodeNoTarget != 2049 {
		t.Fatalf("srsCodeNoTarget=%d，want 2049（实测 SRS 5.0.225 DELETE 未知 id 返回 {\"code\":2049}）", srsCodeNoTarget)
	}
}

// TestNewSRSClient 基址为空返回 nil —— 调用方据此走「降级为仅封禁」路径。
func TestNewSRSClient(t *testing.T) {
	if c := newSRSClient(""); c != nil {
		t.Fatal("空基址应返回 nil（降级）")
	}
	if c := newSRSClient("   "); c != nil {
		t.Fatal("空白基址应返回 nil（降级）")
	}
	if c := newSRSClient("http://127.0.0.1:1985/"); c == nil {
		t.Fatal("有效基址应返回客户端")
	}
}

// TestFlvURL FLV 拉流地址（截帧源，plan §2D.1）。
func TestFlvURL(t *testing.T) {
	svc := &Service{cfg: streamTestCfg()}
	svc.cfg.Live.PlayDomain = "127.0.0.1:8180"
	if got, want := svc.flvURL(102), "http://127.0.0.1:8180/live/room_102.flv"; got != want {
		t.Fatalf("FLV 地址 %q，want %q", got, want)
	}
	// 未配置 playDomain：不可截帧 → 返回空，调用方记 degraded 且不切断
	empty := &Service{cfg: streamTestCfg()}
	empty.cfg.Live.PlayDomain = ""
	if got := empty.flvURL(1); got != "" {
		t.Fatalf("未配置 playDomain 应返回空，got %q", got)
	}
}

// 切断处置链 =============================================================

// TestCutErrorsBannedIdempotent 封禁/解封/未启用的错误码语义（91527-91030）。
func TestCutErrorsBannedIdempotent(t *testing.T) {
	cases := []struct {
		name string
		got  int
		want int
	}{
		{"机审未启用", errcode.ErrLiveAuditDisabled.Code, 91027},
		{"房间已封禁", errcode.ErrLiveRoomBanned.Code, 91028},
		{"房间未被封禁", errcode.ErrLiveRoomNotBanned.Code, 91029},
		{"无推流", errcode.ErrLiveNoActiveStream.Code, 91030},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: code=%d，want %d", c.name, c.got, c.want)
		}
	}
}

// TestCutModeConstants 切断方式常量与 DB 注释一致（1 srs_api / 2 degraded）。
func TestCutModeConstants(t *testing.T) {
	if CutModeSrsAPI != 1 || CutModeDegraded != 2 {
		t.Fatalf("cut_mode 常量 %d/%d，want 1/2", CutModeSrsAPI, CutModeDegraded)
	}
	if ViolationSourceAuto != 1 || ViolationSourceManual != 2 {
		t.Fatalf("violation source 常量 %d/%d，want 1/2", ViolationSourceAuto, ViolationSourceManual)
	}
	if RoomBanNone != 0 || RoomBanApplied != 1 {
		t.Fatalf("ban_status 常量 %d/%d，want 0/1", RoomBanNone, RoomBanApplied)
	}
}

// TestAuthorizePublishHasBanGate 封禁闸必须留在 on_publish 鉴权路径上（plan §2D.3）：
// 否则被切断的主播立刻重推，快速切断形同虚设。用源码断言防止后续重构悄悄丢掉。
func TestAuthorizePublishHasBanGate(t *testing.T) {
	src, err := os.ReadFile("stream.go")
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(src), "RoomBanApplied") {
		t.Fatal("stream.go 的 AuthorizePublish 未见封禁判定（plan §2D.3 要求封禁房间拒推）")
	}
	if !contains(string(src), "ErrLiveRoomBanned") {
		t.Fatal("封禁拒绝必须返回 91028")
	}
}

func contains(hay, needle string) bool {
	for i := 0; i+len(needle) <= len(hay); i++ {
		if hay[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

// 截帧图尺寸 =============================================================

// TestImageSize 真实 PNG 读尺寸；缺失/损坏返回 0,0（尺寸仅留痕，不影响判定）。
func TestImageSize(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.png")
	writeTestPNG(t, good, 640, 360, color.RGBA{R: 0, G: 0, B: 0, A: 255})
	if w, h := imageSize(good); w != 640 || h != 360 {
		t.Fatalf("尺寸 %dx%d，want 640x360", w, h)
	}
	if w, h := imageSize(filepath.Join(dir, "missing.png")); w != 0 || h != 0 {
		t.Fatalf("缺失文件应为 0,0，got %dx%d", w, h)
	}
	bad := filepath.Join(dir, "bad.png")
	if err := os.WriteFile(bad, []byte("not a png"), 0o644); err != nil {
		t.Fatal(err)
	}
	if w, h := imageSize(bad); w != 0 || h != 0 {
		t.Fatalf("损坏文件应为 0,0，got %dx%d", w, h)
	}
}

// Worker 开关 ============================================================

// TestAuditWorkerDisabledReturnsImmediately 未启用时立即返回（dev 默认关闭，spec §5E.1）。
func TestAuditWorkerDisabledReturnsImmediately(t *testing.T) {
	cfg := streamTestCfg()
	cfg.Live.Audit.Enabled = false
	svc := &Service{cfg: cfg, log: zap.NewNop()}
	done := make(chan struct{})
	go func() { svc.StartAuditWorker(context.Background()); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("未启用时 StartAuditWorker 应立即返回")
	}
}

// TestAuditWorkerStopsOnCtxCancel 启用时随 ctx 取消退出（不泄漏 goroutine）。
func TestAuditWorkerStopsOnCtxCancel(t *testing.T) {
	cfg := streamTestCfg()
	cfg.Live.Audit.Enabled = true
	cfg.Live.Audit.FrameIntervalSeconds = 3600 // 不真的跑一轮
	svc := &Service{cfg: cfg, log: zap.NewNop()}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { svc.StartAuditWorker(ctx); close(done) }()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("ctx 取消后 Worker 应退出")
	}
}

func writeTestPNG(t *testing.T, path string, w, h int, c color.Color) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}
