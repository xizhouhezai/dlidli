package middleware

import (
	"context"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestWriteLimiterNilPasses(t *testing.T) {
	e := gin.New()
	e.Use(WriteLimiter(nil))
	e.POST("/w", func(c *gin.Context) { c.String(200, "ok") })
	if w := doReq(e, "POST", "/w"); w.Code != 200 {
		t.Errorf("未配置限流器应放行, got %d", w.Code)
	}
}

func TestRateLimiterFailsOpenWithoutRedis(t *testing.T) {
	// rdb 为 nil（Redis 不可用）时必须放行，避免限流故障放大为服务不可用
	rl := NewRateLimiter(nil, 1, zap.NewNop())
	if !rl.Allow(context.Background(), "u:1", "/x") {
		t.Error("Redis 缺失时应放行(fail-open)")
	}
}

func TestWriteLimiterSkipsReadRequests(t *testing.T) {
	// perMinute=0 表示禁用：即使 POST 也放行；GET 恒放行
	rl := &RateLimiter{perMinute: 0}
	e := gin.New()
	e.Use(WriteLimiter(rl))
	e.GET("/r", func(c *gin.Context) { c.String(200, "ok") })
	e.POST("/w", func(c *gin.Context) { c.String(200, "ok") })
	if w := doReq(e, "GET", "/r"); w.Code != 200 {
		t.Errorf("GET 应恒放行, got %d", w.Code)
	}
	if w := doReq(e, "POST", "/w"); w.Code != 200 {
		t.Errorf("限流禁用时 POST 应放行, got %d", w.Code)
	}
}

func TestNewRateLimiterDisabledWhenPerMinuteNonPositive(t *testing.T) {
	if rl := NewRateLimiter(nil, 0, zap.NewNop()); !rl.Allow(context.Background(), "k", "/p") {
		t.Error("perMinute<=0 应视为禁用并放行")
	}
	if rl := NewRateLimiter(nil, -5, zap.NewNop()); !rl.Allow(context.Background(), "k", "/p") {
		t.Error("perMinute 负值应视为禁用并放行")
	}
}
