package live

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/dlidli/server/internal/module/account"
	"github.com/dlidli/server/internal/pkg/config"
	"github.com/dlidli/server/internal/pkg/snowflake"
	"github.com/dlidli/server/internal/pkg/wsorigin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// TestDanmakuVipBadge 会员身份标识（M4-VIP-01 权益落地点之二）：直播弹幕的**历史回放**
// 与**广播帧**都必须带 `is_vip` / `nickname_color`，且由服务端注入的会员标识提供者决定。
//
// 用 fake provider 而非真实 membership 表：本测试固定的是「弹幕通道是否消费会员标识」，
// 会员判定本身的正确性由 vip 包覆盖。若这里也查库，失败时分不清是
// 「弹幕没带徽章」还是「会员判定错了」。
type fakeVipBadge struct {
	colors map[int64]string
	fail   bool
}

func (f *fakeVipBadge) VipColors(ids []int64) (map[int64]string, error) {
	if f.fail {
		return nil, errVipProbe
	}
	out := map[int64]string{}
	for _, id := range ids {
		if c, ok := f.colors[id]; ok {
			out[id] = c
		}
	}
	return out, nil
}

type vipProbeErr struct{}

func (vipProbeErr) Error() string { return "vip provider 故障" }

var errVipProbe = vipProbeErr{}

// vipTestSvc 构造带会员标识的 live Service（复用 audit 测试的轻量构造口径）。
func vipTestSvc(t *testing.T, db *gorm.DB, vip VipBadgeProvider) *Service {
	t.Helper()
	// SendDanmaku 用 snowflake 生成消息 ID；node 未初始化会 panic。
	if err := snowflake.Init(1); err != nil {
		t.Fatalf("snowflake 初始化失败: %v", err)
	}
	cfg := &config.Config{}
	acct := account.NewService(account.NewRepo(db), nil, cfg, zap.NewNop(), nil)
	repo := NewRepo(db)
	hub := NewHub(wsorigin.Config{}, zap.NewNop())
	svc := &Service{
		repo:   repo,
		online: NewOnline(nil, repo, zap.NewNop()),
		log:    zap.NewNop(),
		hub:    hub,
		deps:   &LiveDeps{Account: acct, RDB: nil, Vip: vip},
	}
	hub.SetDeps(
		func(ctx context.Context, roomID, uid int64, content string) error {
			return svc.SendDanmaku(ctx, uid, roomID, content)
		},
		func(context.Context, int64, int64) {},
		func(context.Context, int64, int64, bool) {},
	)
	return svc
}

// joinRoomFake 把 uid 记为「已进房」（SendDanmaku 的 InRoom 前置校验），
// 并记录该连接收到的广播帧，用于断言广播负载内容。
func joinRoomFake(h *Hub, roomID, uid int64) chan DanmakuInfo {
	ch := make(chan DanmakuInfo, 16)
	h.mu.Lock()
	r, ok := h.rooms[roomID]
	if !ok {
		r = &room{clients: map[*client]bool{}}
		h.rooms[roomID] = r
	}
	c := &client{hub: h, uid: uid, roomID: roomID, send: make(chan []byte, 16)}
	r.clients[c] = true
	h.mu.Unlock()
	// 消费 send 通道，把 danmaku 帧解出来（模拟 WS 客户端）
	go func() {
		for b := range c.send {
			var msg struct {
				Type string      `json:"type"`
				Data DanmakuInfo `json:"data"`
			}
			if json.Unmarshal(b, &msg) == nil && msg.Type == "danmaku" {
				select {
				case ch <- msg.Data:
				default:
				}
			}
		}
	}()
	return ch
}

// TestDanmakuVipBadge 历史回放 + 广播帧都带会员标识；免费用户不带（对照）。
func TestDanmakuVipBadge(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	t.Cleanup(func() { cleanup(t, db) })
	cleanup(t, db)

	const anchor, vipUser, freeUser = 91000301, 91000302, 91000303
	seedUser(t, db, anchor, 100)
	seedUser(t, db, vipUser, 0)
	seedUser(t, db, freeUser, 0)
	db.Exec("INSERT INTO live_whitelist (user_id, note, created_at) VALUES (?, 't', NOW())", anchor)

	svc := vipTestSvc(t, db, &fakeVipBadge{colors: map[int64]string{vipUser: "pink"}})
	res, err := svc.Apply(ctx, anchor)
	if err != nil {
		t.Fatalf("建房失败: %v", err)
	}
	if _, err := svc.Start(ctx, anchor); err != nil {
		t.Fatalf("开播失败: %v", err)
	}
	roomID := res.Room.ID

	got := joinRoomFake(svc.hub, roomID, vipUser)

	if err := svc.SendDanmaku(ctx, vipUser, roomID, "会员弹幕"); err != nil {
		t.Fatalf("会员发送弹幕失败: %v", err)
	}
	// 广播是异步的（写入 client.send 后由消费 goroutine 解码），故等待而非立即断言。
	select {
	case item := <-got:
		if !item.IsVip || item.NickColor != "pink" {
			t.Fatalf("会员弹幕广播帧应带徽章与粉色昵称, got is_vip=%v color=%q", item.IsVip, item.NickColor)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("未收到会员弹幕广播帧")
	}

	// 免费用户：已进房但不在会员标识返回值中 → 广播帧不带徽章
	joinRoomFake(svc.hub, roomID, freeUser)
	if err := svc.SendDanmaku(ctx, freeUser, roomID, "免费弹幕"); err != nil {
		t.Fatalf("免费用户发送弹幕失败: %v", err)
	}

	// 历史回放：两条都在，且徽章状态与发送者身份一致（对照证明徽章不是恒真）
	hist, err := svc.History(ctx, roomID)
	if err != nil {
		t.Fatalf("History 失败: %v", err)
	}
	var vipSeen, freeSeen int
	for _, m := range hist.List {
		switch m.Content {
		case "会员弹幕":
			vipSeen++
			if !m.IsVip || m.NickColor != "pink" {
				t.Fatalf("历史回放中会员弹幕应带徽章, got %+v", m)
			}
		case "免费弹幕":
			freeSeen++
			if m.IsVip || m.NickColor != "" {
				t.Fatalf("历史回放中免费用户弹幕不应带徽章, got %+v", m)
			}
		}
	}
	if vipSeen != 1 || freeSeen != 1 {
		t.Fatalf("历史回放条数不符: vip=%d free=%d", vipSeen, freeSeen)
	}
}

// TestDanmakuVipBadgeDegrade 会员标识查询失败时降级为无徽章，**不得阻断弹幕**。
//
// 理由：徽章是展示增强而非数据完整性的一部分；若查询失败就发不出弹幕，
// 会员服务的一次抖动会放大成「全站直播弹幕不可用」。
func TestDanmakuVipBadgeDegrade(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	t.Cleanup(func() { cleanup(t, db) })
	cleanup(t, db)

	const anchor, user = 91000311, 91000312
	seedUser(t, db, anchor, 100)
	seedUser(t, db, user, 0)
	db.Exec("INSERT INTO live_whitelist (user_id, note, created_at) VALUES (?, 't', NOW())", anchor)

	svc := vipTestSvc(t, db, &fakeVipBadge{fail: true})
	res, err := svc.Apply(ctx, anchor)
	if err != nil {
		t.Fatalf("建房失败: %v", err)
	}
	if _, err := svc.Start(ctx, anchor); err != nil {
		t.Fatalf("开播失败: %v", err)
	}
	roomID := res.Room.ID
	joinRoomFake(svc.hub, roomID, user)

	if err := svc.SendDanmaku(ctx, user, roomID, "降级弹幕"); err != nil {
		t.Fatalf("会员服务故障时弹幕仍应可发送（降级为无徽章）: %v", err)
	}
	hist, err := svc.History(ctx, roomID)
	if err != nil {
		t.Fatalf("History 失败: %v", err)
	}
	if len(hist.List) == 0 {
		t.Fatalf("降级时弹幕仍应落库")
	}
	for _, m := range hist.List {
		if m.IsVip {
			t.Fatalf("降级时不应出现徽章, got %+v", m)
		}
	}
}
