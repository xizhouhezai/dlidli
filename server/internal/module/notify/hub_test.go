package notify

import (
	"sync"
	"testing"

	"go.uber.org/zap"
)

// TestHubPushRemoveRace 回归：Push 迭代用户房间与 remove（delete+close(send)）
// 并发不得产生 map 竞态或 send on closed channel（用 go test -race 验证）。
func TestHubPushRemoveRace(t *testing.T) {
	h := NewHub(nil, zap.NewNop())
	const n = 200
	clients := make([]*client, n)
	h.mu.Lock()
	r := &userRoom{clients: make(map[*client]bool)}
	h.users[7] = r
	for i := range clients {
		clients[i] = &client{hub: h, send: make(chan []byte, 1)}
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
			h.Push(7, Item{ID: "1", Content: "x"})
		}
	}()
	wg.Wait()
}

// TestHubPushOfflineNoop 无在线连接时 Push 不应 panic/阻塞（no-op）。
func TestHubPushOfflineNoop(t *testing.T) {
	h := NewHub(nil, zap.NewNop())
	h.Push(999, Item{ID: "1"})
	if len(h.users) != 0 {
		t.Fatalf("离线推送不应创建房间, got %d", len(h.users))
	}
}

// TestHubPushNilSafe hub 为 nil 时（未启用实时推送）Push 安全 no-op。
func TestHubPushNilSafe(t *testing.T) {
	var h *Hub
	h.Push(1, Item{ID: "1"}) // 不应 panic
}
