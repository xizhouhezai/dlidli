package live

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/dlidli/server/internal/pkg/config"
	"github.com/dlidli/server/internal/pkg/errcode"
	"go.uber.org/zap"
)

// streamTestCfg 回调相关测试配置（对齐 configs/dev.yaml 的 live 段）。
func streamTestCfg() *config.Config {
	return &config.Config{Live: config.Live{
		PushDomain: "127.0.0.1:1935", PlayDomain: "127.0.0.1:8080",
		StreamApp: "live", CallbackToken: "dev-live-cb-token",
	}}
}

// TestParsePublish 回调寻址解析：标准 param / stream 自带查询串 / 各种非法形态。
func TestParsePublish(t *testing.T) {
	svc := &Service{cfg: streamTestCfg(), log: zap.NewNop()}
	cases := []struct {
		name    string
		req     PublishAuthReq
		wantID  int64
		wantKey string
		wantErr bool
	}{
		{"param 携带密钥（SRS 5 形态）", PublishAuthReq{App: "live", Stream: "room_12", Param: "?key=abc"}, 12, "abc", false},
		{"stream 自带查询串", PublishAuthReq{Stream: "room_7?key=xyz"}, 7, "xyz", false},
		{"param 无前导问号", PublishAuthReq{Stream: "room_3", Param: "key=k3"}, 3, "k3", false},
		{"无密钥", PublishAuthReq{Stream: "room_5"}, 5, "", false},
		{"app 缺省容错", PublishAuthReq{Stream: "room_9"}, 9, "", false},
		{"空体", PublishAuthReq{}, 0, "", true},
		{"非本域流名", PublishAuthReq{Stream: "movie_1"}, 0, "", true},
		{"流名非数字", PublishAuthReq{Stream: "room_abc"}, 0, "", true},
		{"流名零号", PublishAuthReq{Stream: "room_0"}, 0, "", true},
		{"app 不匹配", PublishAuthReq{App: "vod", Stream: "room_1"}, 0, "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := svc.parsePublish(&c.req)
			if c.wantErr {
				if err == nil {
					t.Fatalf("应报错，got %+v", got)
				}
				if err != errcode.ErrLiveStreamFormatInvalid {
					t.Fatalf("应为 91026，got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("不应报错: %v", err)
			}
			if got.roomID != c.wantID || got.key != c.wantKey {
				t.Fatalf("解析结果 %+v，want id=%d key=%q", got, c.wantID, c.wantKey)
			}
		})
	}
}

// TestCheckCallbackToken token 常量时间比较与 fail-closed（未配置/缺失/错误一律拒绝）。
func TestCheckCallbackToken(t *testing.T) {
	svc := &Service{cfg: streamTestCfg()}
	cases := []struct {
		name  string
		query string
		head  string
		want  bool
	}{
		{"查询串正确", "dev-live-cb-token", "", true},
		{"头部正确", "", "dev-live-cb-token", true},
		{"查询串错误", "wrong", "", false},
		{"前缀伪造", "dev-live-cb-toke", "", false},
		{"超长伪造", "dev-live-cb-token-extra", "", false},
		{"缺 token", "", "", false},
	}
	for _, c := range cases {
		if got := svc.CheckCallbackToken(c.query, c.head); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
	// 未配置 callbackToken：即使请求带来任意 token 也拒绝
	unset := &Service{cfg: &config.Config{}}
	if unset.CheckCallbackToken("anything", "") {
		t.Fatal("未配置 callbackToken 时必须拒绝（fail-closed）")
	}
}

// TestPushAndPlayURLs 推拉流寻址契约（D1/D4）：流名公开、密钥只在推流查询串、拉流不含密钥。
func TestPushAndPlayURLs(t *testing.T) {
	svc := &Service{cfg: streamTestCfg()}
	if got, want := svc.basePushURL(12), "rtmp://127.0.0.1:1935/live/room_12"; got != want {
		t.Fatalf("基址 %q, want %q", got, want)
	}
	if got, want := svc.pushURL(12, "abc123"), "rtmp://127.0.0.1:1935/live/room_12?key=abc123"; got != want {
		t.Fatalf("带密钥推流地址 %q, want %q", got, want)
	}
	p := svc.playURLs(12)
	if p.HLS != "http://127.0.0.1:8080/live/room_12.m3u8" || p.FLV != "http://127.0.0.1:8080/live/room_12.flv" {
		t.Fatalf("拉流地址异常: %+v", p)
	}
	if strings.Contains(p.HLS+p.FLV, "key") {
		t.Fatal("拉流地址不得含密钥")
	}
	// 未配置域时退回占位域（不产生空地址）
	bare := &Service{}
	if bare.basePushURL(1) == "" || bare.playURLs(1).HLS == "" {
		t.Fatal("未配置域名时也应返回占位地址")
	}
}

// TestPublishAuthReqUnmarshal SRS 回调体解码容错：client_id 字符串/数字两态、字段缺失。
func TestPublishAuthReqUnmarshal(t *testing.T) {
	var req PublishAuthReq
	body := `{"action":"on_publish","client_id":123,"ip":"1.2.3.4","vhost":"__defaultVhost__","app":"live","stream":"room_2","param":"?key=k"}`
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatalf("数字 client_id 解码失败: %v", err)
	}
	if req.Stream != "room_2" || req.Param != "?key=k" {
		t.Fatalf("字段解码异常: %+v", req)
	}
	var sreq PublishAuthReq
	if err := json.Unmarshal([]byte(`{"client_id":"abc"}`), &sreq); err != nil {
		t.Fatalf("字符串 client_id 解码失败: %v", err)
	}
}

// TestAuthorizePublishFlow 集成：鉴权正/反例（房间状态、密钥）与 on_unpublish 幂等下播。
// 依赖 DLIDLI_TEST_DSN；未设置时跳过（沿用 live 包既有测试口径）。
func TestAuthorizePublishFlow(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	t.Cleanup(func() { cleanup(t, db) })
	cleanup(t, db)

	repo := NewRepo(db)
	svc := &Service{repo: repo, online: NewOnline(nil, repo, zap.NewNop()), cfg: streamTestCfg(), log: zap.NewNop()}
	const uid = 91000010
	seedUser(t, db, uid, 100)
	db.Exec("INSERT INTO live_whitelist (user_id, note, created_at) VALUES (?, 't', NOW())", uid)

	applied, err := svc.Apply(ctx, uid)
	if err != nil {
		t.Fatalf("申请开播失败: %v", err)
	}
	roomID, key := applied.Room.ID, applied.StreamKey
	authReq := func(k string) *PublishAuthReq {
		return &PublishAuthReq{Action: "on_publish", App: "live", Stream: StreamName(roomID), Param: "?key=" + k}
	}

	// 未开播（status=0）→ 91006
	if _, err := svc.AuthorizePublish(ctx, authReq(key)); err != errcode.ErrLiveNotLive {
		t.Fatalf("未开播应 91006, got %v", err)
	}
	if _, err := svc.Start(ctx, uid); err != nil {
		t.Fatalf("开播失败: %v", err)
	}
	// 开播后：正确密钥放行
	gotID, err := svc.AuthorizePublish(ctx, authReq(key))
	if err != nil || gotID != roomID {
		t.Fatalf("正确密钥应放行, got id=%d err=%v", gotID, err)
	}
	// 错误密钥 → 91025
	if _, err := svc.AuthorizePublish(ctx, authReq("deadbeef")); err != errcode.ErrLiveStreamKeyInvalid {
		t.Fatalf("错误密钥应 91025, got %v", err)
	}
	// 缺密钥 → 91025
	if _, err := svc.AuthorizePublish(ctx, &PublishAuthReq{Stream: StreamName(roomID)}); err != errcode.ErrLiveStreamKeyInvalid {
		t.Fatalf("缺密钥应 91025, got %v", err)
	}
	// 不存在的房间 → 91007
	if _, err := svc.AuthorizePublish(ctx, &PublishAuthReq{Stream: "room_99999999", Param: "?key=x"}); err != errcode.ErrLiveRoomNotFound {
		t.Fatalf("未知房间应 91007, got %v", err)
	}
	// on_unpublish：下播 → status=2；重复回调和未知流幂等
	stopReq := &PublishAuthReq{Action: "on_unpublish", App: "live", Stream: StreamName(roomID)}
	stopped, err := svc.StopByStream(ctx, stopReq)
	if err != nil || !stopped {
		t.Fatalf("首次 on_unpublish 应下播, got stopped=%v err=%v", stopped, err)
	}
	room, err := repo.RoomByID(roomID)
	if err != nil || room == nil {
		t.Fatalf("查房失败: %v", err)
	}
	if room.Status != RoomStatusEnded || room.EndedAt == nil {
		t.Fatalf("下播后应为 status=2 且 ended_at 非空: %+v", room)
	}
	if stopped, err := svc.StopByStream(ctx, stopReq); err != nil || stopped {
		t.Fatalf("重复回调应幂等无动作, got stopped=%v err=%v", stopped, err)
	}
	if stopped, err := svc.StopByStream(ctx, &PublishAuthReq{Stream: "room_99999999"}); err != nil || stopped {
		t.Fatalf("未知流应幂等无动作, got stopped=%v err=%v", stopped, err)
	}
	// 下播后推流重新鉴权 → 91006
	if _, err := svc.AuthorizePublish(ctx, authReq(key)); err != errcode.ErrLiveNotLive {
		t.Fatalf("下播后应 91006, got %v", err)
	}
}
