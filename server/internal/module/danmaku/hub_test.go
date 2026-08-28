package danmaku

import (
	"sync"
	"testing"

	"go.uber.org/zap"
)

// TestHubBroadcastRemoveRace 回归：Broadcast 迭代房间与 remove（delete+close(send)）
// 并发不得产生 map 竞态或 send on closed channel。用 go test -race 验证。
func TestHubBroadcastRemoveRace(t *testing.T) {
	h := NewHub(nil, zap.NewNop())
	const n = 200
	clients := make([]*client, n)
	h.mu.Lock()
	r := &room{clients: make(map[*client]bool)}
	h.rooms[1] = r
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
			h.remove(clients[i], 1)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			h.Broadcast(1, 0, &Item{})
		}
	}()
	wg.Wait()
}
