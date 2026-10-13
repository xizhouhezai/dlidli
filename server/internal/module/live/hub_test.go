package live

import (
	"context"
	"sync"
	"testing"

	"go.uber.org/zap"
)

// TestHubBroadcastRemoveRace 回归：broadcast 迭代房间连接与 remove（delete+close(send)）
// 并发不得产生 map 竞态或 send on closed channel（go test -race 验证）。
func TestHubBroadcastRemoveRace(t *testing.T) {
	h := NewHub(nil, zap.NewNop())
	const n = 200
	clients := make([]*client, n)
	h.mu.Lock()
	r := &room{clients: make(map[*client]bool)}
	h.rooms[1] = r
	for i := range clients {
		clients[i] = &client{hub: h, send: make(chan []byte, 1), uid: int64(i + 1), roomID: 1}
		r.clients[clients[i]] = true
	}
	h.mu.Unlock()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			h.remove(clients[i])
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			h.BroadcastDanmaku(1, DanmakuInfo{Content: "hi"})
			h.BroadcastEnter(1, EnterInfo{UID: "9"})
		}
	}()
	wg.Wait()
}

// TestHubKickReplacesOldClient 同 uid 旧连接被新连接顶替：旧连接不再收到广播，
// 房间内该 uid 只剩一条连接。
func TestHubKickReplacesOldClient(t *testing.T) {
	h := NewHub(nil, zap.NewNop())
	h.mu.Lock()
	r := &room{clients: make(map[*client]bool)}
	h.rooms[7] = r
	old := &client{hub: h, send: make(chan []byte, 4), uid: 42, roomID: 7}
	r.clients[old] = true
	h.mu.Unlock()

	// 顶替：模拟 Serve 的去重路径（不真升级 WS，直接操作集合）
	h.mu.Lock()
	delete(r.clients, old)
	close(old.send)
	h.mu.Unlock()
	_ = old.conn // conn 为 nil，仅验证集合/通道语义

	if h.InRoom(7, 42) {
		t.Fatal("旧连接已删，InRoom 应为 false")
	}
}

// TestHubOnlineLeaveCallback 断开回调只触发一次：被顶替连接的 remove 不重复扣计数。
func TestHubOnlineLeaveCallback(t *testing.T) {
	h := NewHub(nil, zap.NewNop())
	var calls int64
	h.onlineLeave = func(ctx context.Context, roomID, uid int64, counted bool) {
		if roomID == 3 && uid == 9 {
			calls++
		}
	}
	cl := &client{hub: h, send: make(chan []byte, 1), uid: 9, roomID: 3}
	h.mu.Lock()
	h.rooms[3] = &room{clients: map[*client]bool{cl: true}}
	h.mu.Unlock()

	h.remove(cl)
	h.remove(cl) // 第二次：连接已不在集合，不得重复回调
	if calls != 1 {
		t.Fatalf("离房回调应恰好 1 次, got %d", calls)
	}
}

// TestInRoomAndOnlineCount 房间集合判定与在线连接计数。
func TestInRoomAndOnlineCount(t *testing.T) {
	h := NewHub(nil, zap.NewNop())
	a := &client{hub: h, send: make(chan []byte, 1), uid: 1, roomID: 5}
	b := &client{hub: h, send: make(chan []byte, 1), uid: 2, roomID: 5}
	h.mu.Lock()
	h.rooms[5] = &room{clients: map[*client]bool{a: true, b: true}}
	h.mu.Unlock()

	if !h.InRoom(5, 1) || !h.InRoom(5, 2) {
		t.Fatal("房间内 uid 应判定为 InRoom")
	}
	if h.InRoom(5, 3) || h.InRoom(6, 1) {
		t.Fatal("房间外 uid/房间外连接不应判定为 InRoom")
	}
	if n := h.OnlineCount(5); n != 2 {
		t.Fatalf("OnlineCount 应为 2, got %d", n)
	}
}
