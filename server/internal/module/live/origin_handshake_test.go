package live

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dlidli/server/internal/pkg/jwtx"
	"github.com/dlidli/server/internal/pkg/wsorigin"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// TestHubRealHandshakeOrigin 走**真实 WS 握手**（httptest + gin + gorilla Upgrader，
// 真实 101 vs 403），固定 live hub 的 Origin 策略与其余三个 hub 一致。
// 直接驱动 hub 的 upgrader（而非完整 Serve）以避开「必须是直播中房间 + DB 种子」的前置，
// 从而把断言精确落在「握手是否被 Origin 策略放行」这一件事上。
func TestHubRealHandshakeOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const secret = "handshake-secret"
	token, err := jwtx.Generate(secret, 2105223266137280512, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	h := NewHub(wsorigin.Config{
		AllowOrigins:       []string{"http://localhost:5173"},
		JWTSecret:          secret,
		AllowNativeClients: true,
	}, zap.NewNop())

	e := gin.New()
	e.GET("/ws", func(c *gin.Context) {
		conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return // Upgrader 已回 403
		}
		defer conn.Close()
		_ = conn.WriteMessage(websocket.TextMessage, []byte("ok"))
	})
	srv := httptest.NewServer(e)
	defer srv.Close()
	wsURL := strings.Replace(srv.URL, "http://", "ws://", 1) + "/ws"

	cases := []struct {
		name   string
		origin string
		token  string
		wantOK bool
	}{
		{"白名单来源 → 101", "http://localhost:5173", "", true},
		{"非白名单来源无 token → 403", "https://evil.example", "", false},
		{"非白名单来源乱造 token → 403", "https://evil.example", "bogus", false},
		{"ArkTS 自生成形态 + 有效 token → 101", "http://192.168.77.66", token, true},
		{"ArkTS 自生成形态无 token → 403", "http://192.168.77.66", "", false},
		{"file:// + 有效 token → 101", "file://", token, true},
		{"无 Origin 头 → 101", "", "", true},
	}

	for _, c := range cases {
		url := wsURL
		if c.token != "" {
			url += "?token=" + c.token
		}
		hdr := http.Header{}
		if c.origin != "" {
			hdr.Set("Origin", c.origin)
		}
		d := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
		conn, resp, err := d.Dial(url, hdr)
		ok := err == nil
		if ok {
			defer conn.Close()
		}
		if ok != c.wantOK {
			status := 0
			if resp != nil {
				status = resp.StatusCode
			}
			t.Errorf("%s: 握手成功=%v (status=%d), want %v", c.name, ok, status, c.wantOK)
			continue
		}
		// 反过来确认被拒时确实是 403（而不是其它错误）
		if !c.wantOK && resp != nil && resp.StatusCode != http.StatusForbidden {
			t.Errorf("%s: 拒绝状态码应为 403, got %d", c.name, resp.StatusCode)
		}
	}
}
