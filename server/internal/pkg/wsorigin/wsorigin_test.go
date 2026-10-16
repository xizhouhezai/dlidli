package wsorigin

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/dlidli/server/internal/pkg/jwtx"
	"go.uber.org/zap"
)

const testSecret = "test-secret"

// originReq 构造带 Origin 头（可选带 query token）的请求。
func originReq(origin, token string) *http.Request {
	target := "/ws"
	if token != "" {
		target += "?token=" + url.QueryEscape(token)
	}
	r := httptest.NewRequest("GET", target, nil)
	r.Header.Set("Origin", origin)
	return r
}

// TestNoOriginAllowed 非浏览器客户端（无 Origin 头）放行——升级前口径，保持不变。
func TestNoOriginAllowed(t *testing.T) {
	check := New(Config{AllowOrigins: []string{"http://localhost:5173"}}, zap.NewNop())
	if !check(httptest.NewRequest("GET", "/ws", nil)) {
		t.Fatal("无 Origin 头应放行（非浏览器客户端）")
	}
}

// TestWhitelistedOriginAllowed 白名单来源放行；尾斜杠归一（与升级前 TrimRight 逐字一致）。
func TestWhitelistedOriginAllowed(t *testing.T) {
	check := New(Config{AllowOrigins: []string{"http://localhost:5173"}}, zap.NewNop())
	for _, o := range []string{"http://localhost:5173", "http://localhost:5173/"} {
		r := httptest.NewRequest("GET", "/ws", nil)
		r.Header.Set("Origin", o)
		if check(r) {
			continue
		}
		t.Fatalf("白名单来源应放行: %s", o)
	}
}

// TestForeignOriginRejected 不在白名单且无 token → 拒绝（浏览器端防护不变）。
func TestForeignOriginRejected(t *testing.T) {
	check := New(Config{AllowOrigins: []string{"http://localhost:5173"}}, zap.NewNop())
	r := httptest.NewRequest("GET", "/ws", nil)
	r.Header.Set("Origin", "https://evil.example")
	if check(r) {
		t.Fatal("非白名单来源应拒绝")
	}
}

// TestForeignOriginRejectedEvenWithNativeEnabled 开了 native 例外，
// 但**未携带 token** 的外来 Origin 仍必须被拒——防「裸放行」。
func TestForeignOriginRejectedEvenWithNativeEnabled(t *testing.T) {
	check := New(Config{
		AllowOrigins:       []string{"http://localhost:5173"},
		JWTSecret:          testSecret,
		AllowNativeClients: true,
	}, zap.NewNop())
	r := httptest.NewRequest("GET", "/ws", nil)
	r.Header.Set("Origin", "https://evil.example")
	if check(r) {
		t.Fatal("native 例外开启时，无 token 的外来 Origin 仍应拒绝")
	}
}

// TestNativeClientWithValidTokenAllowed 核心新增路径：鸿蒙 ArkTS 自生成 Origin
// （`http://10.0.2.2` 不带端口 / `file://`）携带有效 token → 放行。
func TestNativeClientWithValidTokenAllowed(t *testing.T) {
	token, err := jwtx.Generate(testSecret, 2105223266137280512, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	check := New(Config{
		AllowOrigins:       []string{"http://localhost:5173"},
		JWTSecret:          testSecret,
		AllowNativeClients: true,
	}, zap.NewNop())
	for _, o := range []string{"http://10.0.2.2", "http://127.0.0.1", "file://", "https://10.0.2.2"} {
		r := httptest.NewRequest("GET", "/ws?token="+token, nil)
		r.Header.Set("Origin", o)
		if !check(r) {
			t.Fatalf("携带有效 token 的 native 来源应放行: %s", o)
		}
	}
}

// TestNativeClientWithBadTokenRejected token 无效 / 缺失 / 错误 → 拒绝。
func TestNativeClientWithBadTokenRejected(t *testing.T) {
	valid, err := jwtx.Generate(testSecret, 7, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	expired, err := jwtx.Generate(testSecret, 7, -time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	otherSecret, err := jwtx.Generate("another-secret", 7, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	check := New(Config{
		AllowOrigins:       []string{"http://localhost:5173"},
		JWTSecret:          testSecret,
		AllowNativeClients: true,
	}, zap.NewNop())
	cases := []struct {
		name string
		tk   string
	}{
		{"无 token", ""},
		{"乱造 token", "not-a-jwt"},
		{"过期 token", expired},
		{"他人密钥签名", otherSecret},
		{"篡改 token", valid + "x"},
	}
	for _, c := range cases {
		if check(originReq("http://10.0.2.2", c.tk)) {
			t.Fatalf("%s：应拒绝", c.name)
		}
	}
}

// TestAdminTokenNotAcceptedForNative 管理员令牌不是 C 端用户令牌，不得用于 native 例外
// （jwtx.Parse 明确拒绝 Admin 令牌）。
func TestAdminTokenNotAcceptedForNative(t *testing.T) {
	admin, err := jwtx.GenerateAdmin(testSecret, 1, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	check := New(Config{
		AllowOrigins:       []string{"http://localhost:5173"},
		JWTSecret:          testSecret,
		AllowNativeClients: true,
	}, zap.NewNop())
	r := httptest.NewRequest("GET", "/ws?token="+admin, nil)
	r.Header.Set("Origin", "http://10.0.2.2")
	if check(r) {
		t.Fatal("管理员令牌不得用于 native 例外")
	}
}

// TestNativeDisabledIsFailClosed 开关关闭（默认）时，即使带了有效 token，
// 非白名单 Origin 仍拒绝——漏配不会静默放宽。
func TestNativeDisabledIsFailClosed(t *testing.T) {
	token, err := jwtx.Generate(testSecret, 7, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	check := New(Config{
		AllowOrigins:       []string{"http://localhost:5173"},
		JWTSecret:          testSecret,
		AllowNativeClients: false,
	}, zap.NewNop())
	r := httptest.NewRequest("GET", "/ws?token="+token, nil)
	r.Header.Set("Origin", "http://10.0.2.2")
	if check(r) {
		t.Fatal("allowNativeClients 关闭时，非白名单 Origin 应一律拒绝")
	}
}

// TestAuthorizationHeaderAlsoAccepted 无 Origin 头时本就放行；此例固定
// Authorization 头这条 token 通道对 native 例外同样有效（WS 无法自定义头，
// 但原生客户端可自设；两种载体都须被识别）。
func TestAuthorizationHeaderAlsoAccepted(t *testing.T) {
	token, err := jwtx.Generate(testSecret, 7, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	check := New(Config{
		AllowOrigins:       []string{"http://localhost:5173"},
		JWTSecret:          testSecret,
		AllowNativeClients: true,
	}, zap.NewNop())
	r := httptest.NewRequest("GET", "/ws", nil)
	r.Header.Set("Origin", "http://10.0.2.2")
	r.Header.Set("Authorization", "Bearer "+token)
	if !check(r) {
		t.Fatal("Authorization 头携带有效 token 应放行")
	}
}

// TestEmptyWhitelistRejectsAllOrigins 白名单为空时不放行任何带 Origin 的请求
// （除 native 例外路径外），保证「未配置 = 最严」。
func TestEmptyWhitelistRejectsAllOrigins(t *testing.T) {
	check := New(Config{AllowOrigins: nil}, zap.NewNop())
	r := httptest.NewRequest("GET", "/ws", nil)
	r.Header.Set("Origin", "http://localhost:5173")
	if check(r) {
		t.Fatal("白名单为空时应拒绝带 Origin 的请求")
	}
}
