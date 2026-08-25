package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dlidli/server/internal/pkg/jwtx"
	"github.com/dlidli/server/internal/pkg/playsign"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const secret = "test-secret"

func init() {
	gin.SetMode(gin.TestMode)
}

func newRouter(h gin.HandlerFunc) *gin.Engine {
	e := gin.New()
	e.Use(h)
	e.GET("/ok", func(c *gin.Context) { c.String(200, "ok") })
	return e
}

func doReq(e *gin.Engine, method, target string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, target, nil)
	e.ServeHTTP(w, req)
	return w
}

func doReqAuth(e *gin.Engine, method, target, token string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, target, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	e.ServeHTTP(w, req)
	return w
}

func TestTraceIDGeneratedAndEchoed(t *testing.T) {
	e := newRouter(TraceID())
	w1 := doReq(e, "GET", "/ok")
	w2 := doReq(e, "GET", "/ok")
	if w1.Header().Get("X-Request-Id") == "" {
		t.Error("未携带 X-Request-Id 时应生成")
	}
	if w1.Header().Get("X-Request-Id") == w2.Header().Get("X-Request-Id") {
		t.Error("两次请求 trace id 不应相同")
	}
}

func TestTraceIDPassthrough(t *testing.T) {
	e := newRouter(TraceID())
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/ok", nil)
	req.Header.Set("X-Request-Id", "fixed-trace")
	e.ServeHTTP(w, req)
	if got := w.Header().Get("X-Request-Id"); got != "fixed-trace" {
		t.Errorf("应透传外部 trace id, got %q", got)
	}
}

func TestAuth(t *testing.T) {
	var gotUID int64
	e := gin.New()
	e.Use(Auth(secret))
	e.GET("/me", func(c *gin.Context) { gotUID = c.GetInt64(CtxUserID); c.String(200, "ok") })

	token, _ := jwtx.Generate(secret, 42, time.Minute)
	w := doReqAuth(e, "GET", "/me", token)
	if w.Code != 200 || gotUID != 42 {
		t.Errorf("有效 token 应放行: code=%d uid=%d", w.Code, gotUID)
	}
	if w2 := doReq(e, "GET", "/me?token="+token); w2.Code != 200 {
		t.Errorf("query token 应放行(WebSocket 场景): %d", w2.Code)
	}
	if w3 := doReq(e, "GET", "/me"); w3.Code != http.StatusUnauthorized {
		t.Errorf("无 token 应 401, got %d", w3.Code)
	}
	if w4 := doReq(e, "GET", "/me?token=bad.token.here"); w4.Code != http.StatusUnauthorized {
		t.Errorf("非法 token 应 401, got %d", w4.Code)
	}
}

func TestOptionalAuth(t *testing.T) {
	var gotUID int64
	e := gin.New()
	e.Use(OptionalAuth(secret))
	e.GET("/view", func(c *gin.Context) { gotUID = c.GetInt64(CtxUserID); c.String(200, "ok") })
	if w := doReq(e, "GET", "/view"); w.Code != 200 || gotUID != 0 {
		t.Errorf("匿名应放行且无 uid: code=%d uid=%d", w.Code, gotUID)
	}
	if w := doReq(e, "GET", "/view?token=junk"); w.Code != 200 || gotUID != 0 {
		t.Errorf("非法 token 应按游客放行: code=%d uid=%d", w.Code, gotUID)
	}
	token, _ := jwtx.Generate(secret, 7, time.Minute)
	if w := doReq(e, "GET", "/view?token="+token); w.Code != 200 || gotUID != 7 {
		t.Errorf("有效 token 应注入 uid: code=%d uid=%d", w.Code, gotUID)
	}
}

func TestAdminAuthIsolation(t *testing.T) {
	e := gin.New()
	e.Use(AdminAuth(secret))
	e.GET("/admin", func(c *gin.Context) { c.String(200, "ok") })
	adminToken, _ := jwtx.GenerateAdmin(secret, 1, time.Minute)
	userToken, _ := jwtx.Generate(secret, 2, time.Minute)

	if w := doReqAuth(e, "GET", "/admin", adminToken); w.Code != 200 {
		t.Errorf("管理员 token 应放行: %d", w.Code)
	}
	if w := doReqAuth(e, "GET", "/admin", userToken); w.Code != http.StatusUnauthorized {
		t.Errorf("用户 token 访问后台应 401, got %d", w.Code)
	}
}

func TestCORSAllowedOrigin(t *testing.T) {
	e := newRouter(CORS([]string{"http://localhost:5173"}))
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/ok", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	e.ServeHTTP(w, req)
	if w.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Error("允许来源应回显 ACAO")
	}
	if w.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Error("应允许凭据")
	}
}

func TestCORSDisallowedOrigin(t *testing.T) {
	e := newRouter(CORS([]string{"http://localhost:5173"}))
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/ok", nil)
	req.Header.Set("Origin", "https://evil.example")
	e.ServeHTTP(w, req)
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("非允许来源不应下发 CORS 头")
	}
}

func TestCORSPreflight(t *testing.T) {
	e := newRouter(CORS([]string{"*"}))
	w := doReq(e, "OPTIONS", "/ok")
	if w.Code != http.StatusNoContent {
		t.Errorf("预检应返回 204, got %d", w.Code)
	}
}

func TestRecovery(t *testing.T) {
	e := gin.New()
	e.Use(Recovery(zap.NewNop()))
	e.GET("/boom", func(c *gin.Context) { panic("boom") })
	w := doReq(e, "GET", "/boom")
	if w.Code != http.StatusInternalServerError {
		t.Errorf("panic 应转为 500, got %d", w.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if int(body["code"].(float64)) != 10001 {
		t.Errorf("panic 响应码应为内部错误 10001, got %v", body["code"])
	}
}

func TestPlaySignGuard(t *testing.T) {
	guard := PlaySignGuard(secret)
	e := gin.New()
	e.GET("/static/*path", guard, func(c *gin.Context) { c.String(200, "served") })

	rel := "videos/hls/a/index.m3u8"
	signed := playsign.Query(secret, rel, time.Minute)
	if w := doReq(e, "GET", "/static/"+rel+"?"+signed); w.Code != 200 {
		t.Errorf("合法签名应放行, got %d", w.Code)
	}
	if w := doReq(e, "GET", "/static/"+rel); w.Code != 403 {
		t.Errorf("缺签名应 403, got %d", w.Code)
	}
	if w := doReq(e, "GET", "/static/"+rel+"?e=99999999999&s=deadbeef"); w.Code != 403 {
		t.Errorf("篡改签名应 403, got %d", w.Code)
	}
	if w := doReq(e, "GET", "/static/covers/x.jpg"); w.Code != 200 {
		t.Errorf("封面应放行, got %d", w.Code)
	}
	if w := doReq(e, "GET", "/static/videos/hls/a/0.ts"); w.Code != 200 {
		t.Errorf(".ts 分片应放行, got %d", w.Code)
	}
}

func TestAuthedRateLimited(t *testing.T) {
	handlerRan := 0
	newE := func(rl *RateLimiter) *gin.Engine {
		e := gin.New()
		e.POST("/w", AuthedRateLimited(secret, rl), func(c *gin.Context) {
			handlerRan++
			c.String(200, "ok")
		})
		return e
	}
	token, _ := jwtx.Generate(secret, 9, time.Minute)

	// 无 token：401 且业务 handler 不执行（限流/鉴权先于业务）
	handlerRan = 0
	if w := doReq(newE(nil), "POST", "/w"); w.Code != http.StatusUnauthorized || handlerRan != 0 {
		t.Errorf("无 token 应 401 且 handler 不执行: code=%d ran=%d", w.Code, handlerRan)
	}
	// 非法 token 同理
	handlerRan = 0
	if w := doReqAuth(newE(nil), "POST", "/w", "bad.token"); w.Code != http.StatusUnauthorized || handlerRan != 0 {
		t.Errorf("非法 token 应 401 且 handler 不执行: code=%d ran=%d", w.Code, handlerRan)
	}
	// 有效 token + 未配置限流器：放行且注入 uid
	eOK := gin.New()
	var gotUID int64
	eOK.POST("/w", AuthedRateLimited(secret, nil), func(c *gin.Context) {
		gotUID = c.GetInt64(CtxUserID)
		c.String(200, "ok")
	})
	if w := doReqAuth(eOK, "POST", "/w", token); w.Code != 200 || gotUID != 9 {
		t.Errorf("有效 token 应放行并注入 uid: code=%d uid=%d", w.Code, gotUID)
	}
	// 有效 token + Redis 缺失(fail-open)：写请求仍放行
	if w := doReqAuth(newE(NewRateLimiter(nil, 30, zap.NewNop())), "POST", "/w", token); w.Code != 200 {
		t.Errorf("Redis 缺失时应 fail-open, got %d", w.Code)
	}
}
