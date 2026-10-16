package live

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dlidli/server/internal/pkg/jwtx"
	"github.com/dlidli/server/internal/pkg/wsorigin"
	"go.uber.org/zap"
)

// TestHubCheckOriginPolicy 固定 live hub 的**实际接线**（upgrader.CheckOrigin），
// 逐条与 danmaku/notify/im 的同名用例对照，证明四个 hub 策略一致。
func TestHubCheckOriginPolicy(t *testing.T) {
	const secret = "hub-test-secret"
	token, err := jwtx.Generate(secret, 2105223266137280512, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	h := NewHub(wsorigin.Config{
		AllowOrigins:       []string{"http://localhost:5173"},
		JWTSecret:          secret,
		AllowNativeClients: true,
	}, zap.NewNop())
	check := h.upgrader.CheckOrigin

	cases := []struct {
		name   string
		origin string
		token  string
		want   bool
	}{
		{"白名单来源", "http://localhost:5173", "", true},
		{"无 Origin（非浏览器）", "", "", true},
		{"外来 Origin 无 token", "https://evil.example", "", false},
		{"外来 Origin 乱造 token", "https://evil.example", "bogus", false},
		{"鸿蒙自生成 Origin + 有效 token", "http://10.0.2.2", token, true},
		{"file:// + 有效 token", "file://", token, true},
		{"鸿蒙自生成 Origin 无 token", "http://10.0.2.2", "", false},
	}
	for _, c := range cases {
		target := "/ws"
		if c.token != "" {
			target += "?token=" + c.token
		}
		r := httptest.NewRequest("GET", target, nil)
		if c.origin != "" {
			r.Header.Set("Origin", c.origin)
		}
		if got := check(r); got != c.want {
			t.Errorf("%s: CheckOrigin=%v, want %v", c.name, got, c.want)
		}
	}
}
