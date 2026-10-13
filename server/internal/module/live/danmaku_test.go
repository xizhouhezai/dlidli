package live

import (
	"context"
	"testing"

	"github.com/dlidli/server/internal/pkg/errcode"
	"go.uber.org/zap"
)

// TestDanmakuWindowHistory 历史窗口：追加 > 50 条只留最近 50 条、正序回放、下播清空。
func TestDanmakuWindowHistory(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	t.Cleanup(func() { cleanup(t, db) })
	cleanup(t, db)

	repo := NewRepo(db)
	svc := &Service{repo: repo, online: NewOnline(nil, repo, zap.NewNop())}
	const anchor = 91000101
	seedUser(t, db, anchor, 100)
	db.Exec("INSERT INTO live_whitelist (user_id, note, created_at) VALUES (?, 't', NOW())", anchor)
	res, err := svc.Apply(ctx, anchor)
	if err != nil {
		t.Fatalf("建房失败: %v", err)
	}
	if _, err := svc.Start(ctx, anchor); err != nil {
		t.Fatalf("开播失败: %v", err)
	}
	roomID := res.Room.ID

	for i := 0; i < 60; i++ {
		m := &LiveRoomMsg{ID: int64(1000 + i), RoomID: roomID, UserID: anchor,
			Nickname: "u", Content: string(rune('a'+i%26)) + string(rune('a'+i%26))}
		if err := repo.AddRoomMsg(m); err != nil {
			t.Fatalf("追加弹幕 %d 失败: %v", i, err)
		}
	}
	rows, err := repo.ListRoomMsg(roomID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != historyWindow {
		t.Fatalf("窗口应保留 %d 条, got %d", historyWindow, len(rows))
	}
	// 正序回放：首条应是最近 50 条里最早的一条（i=10），末条是最新（i=59）
	if rows[0].ID != 1010 || rows[len(rows)-1].ID != 1059 {
		t.Fatalf("回放序错误: first=%d last=%d", rows[0].ID, rows[len(rows)-1].ID)
	}
	// History 服务层：含在线人数
	hist, err := svc.History(ctx, roomID)
	if err != nil {
		t.Fatal(err)
	}
	if len(hist.List) != historyWindow || hist.Online != 0 {
		t.Fatalf("History 不符合: n=%d online=%d", len(hist.List), hist.Online)
	}
	// 下播清空
	if _, err := svc.Stop(ctx, anchor); err != nil {
		t.Fatal(err)
	}
	rows, _ = repo.ListRoomMsg(roomID)
	if len(rows) != 0 {
		t.Fatalf("下播后历史应清空, got %d", len(rows))
	}
}

// TestDanmakuSendChain 校验链拦截序（Redis 不可用侧不放行频控——直接断言错误码）：
// 未进房 91017；进房后禁言 91015；Redis 缺失时频控放行（降级口径）。
func TestDanmakuSendChain(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	t.Cleanup(func() { cleanup(t, db) })
	cleanup(t, db)

	repo := NewRepo(db)
	svc := &Service{repo: repo, online: NewOnline(nil, repo, zap.NewNop()), log: zap.NewNop(), hub: NewHub(nil, zap.NewNop())}
	const anchor, viewer = 91000201, 91000202
	seedUser(t, db, anchor, 100)
	seedUser(t, db, viewer, 0)
	db.Exec("INSERT INTO live_whitelist (user_id, note, created_at) VALUES (?, 't', NOW())", anchor)
	res, err := svc.Apply(ctx, anchor)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Start(ctx, anchor); err != nil {
		t.Fatal(err)
	}
	roomID := res.Room.ID

	// 未进房 → 91017
	if err := svc.SendDanmaku(ctx, viewer, roomID, "hello"); err != errcode.ErrLiveDanmakuNotIn {
		t.Fatalf("未进房应 91017, got %v", err)
	}
	// 模拟已进房（向 hub 注入连接）→ deps 缺失应走装配缺陷保护 ErrInternal
	svc.hub.mu.Lock()
	svc.hub.rooms[roomID] = &room{clients: map[*client]bool{
		{hub: svc.hub, uid: viewer, roomID: roomID}: true}}
	svc.hub.mu.Unlock()
	if err := svc.SendDanmaku(ctx, viewer, roomID, "hello"); err != errcode.ErrInternal {
		t.Fatalf("deps 缺失应 ErrInternal, got %v", err)
	}
}

// TestDanmakuHistoryEmptyRoom 未开播房间拉历史应 91006/91007。
func TestDanmakuHistoryErrors(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	t.Cleanup(func() { cleanup(t, db) })
	cleanup(t, db)

	repo := NewRepo(db)
	svc := &Service{repo: repo, online: NewOnline(nil, repo, zap.NewNop()), log: zap.NewNop()}
	if _, err := svc.History(ctx, 999999); err != errcode.ErrLiveRoomNotFound {
		t.Fatalf("不存在房间应 91007, got %v", err)
	}
	const anchor = 91000301
	seedUser(t, db, anchor, 100)
	db.Exec("INSERT INTO live_whitelist (user_id, note, created_at) VALUES (?, 't', NOW())", anchor)
	res, _ := svc.Apply(ctx, anchor) // 未开播（status=0）
	if _, err := svc.History(ctx, res.Room.ID); err != errcode.ErrLiveNotLive {
		t.Fatalf("未开播应 91006, got %v", err)
	}
}
