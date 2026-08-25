// Package middleware 提供通用 HTTP 中间件。
// ratelimit.go 为写接口提供基于 Redis 的固定窗口限流（防刷）。
package middleware

import (
	"context"
	"fmt"
	"time"

	"github.com/dlidli/server/internal/pkg/errcode"
	"github.com/dlidli/server/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// RateLimiter 基于 Redis 固定窗口的通用写接口限流器。
type RateLimiter struct {
	rdb       *redis.Client
	perMinute int
	log       *zap.Logger
}

// NewRateLimiter 创建限流器；perMinute<=0 时 Allow 恒放行（等效禁用）。
func NewRateLimiter(rdb *redis.Client, perMinute int, log *zap.Logger) *RateLimiter {
	return &RateLimiter{rdb: rdb, perMinute: perMinute, log: log}
}

// Allow 对 (key, path) 计数；超限返回 false，调用方返回 429。
// scope 采用固定窗口：key = rl:w:{scope}:{path}，窗口 60s 自然滚动。
func (rl *RateLimiter) Allow(ctx context.Context, scope, path string) bool {
	if rl == nil || rl.perMinute <= 0 || rl.rdb == nil {
		return true
	}
	key := fmt.Sprintf("rl:w:%s:%s", scope, path)
	// INCR + 首次设置过期，利用 Redis 单线程保证原子性
	n, err := rl.rdb.Incr(ctx, key).Result()
	if err != nil {
		rl.log.Warn("限流计数失败（放行）", zap.String("key", key), zap.Error(err))
		return true
	}
	if n == 1 {
		_ = rl.rdb.Expire(ctx, key, time.Minute).Err()
	}
	return n <= int64(rl.perMinute)
}

// WriteLimiter 限制写请求（POST/PUT/PATCH/DELETE），挂到需登录的写接口路由组前。
// scope 优先取登录 user_id，未登录退回客户端 IP。
func WriteLimiter(rl *RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if rl == nil || rl.perMinute <= 0 {
			c.Next()
			return
		}
		method := c.Request.Method
		if method != "POST" && method != "PUT" && method != "PATCH" && method != "DELETE" {
			c.Next() // 读请求不限流
			return
		}
		scope := fmt.Sprintf("%d", c.GetInt64(CtxUserID))
		if scope == "0" {
			scope = "ip:" + c.ClientIP()
		}
		if !rl.Allow(c.Request.Context(), scope, c.Request.URL.Path) {
			response.Fail(c, errcode.ErrTooManyRequests.WithMsg("操作太频繁，请稍后再试"))
			c.Abort()
			return
		}
		c.Next()
	}
}

// Chain 按顺序串行执行多个 gin 中间件（先执行者在前、后执行者续接）。
// 用于在路由组上组合"先鉴权再限流"等有依赖的中间件顺序。
func Chain(ms ...gin.HandlerFunc) gin.HandlerFunc {
	if len(ms) == 0 {
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		runChain(c, ms, 0)
	}
}

func runChain(c *gin.Context, ms []gin.HandlerFunc, i int) {
	if i >= len(ms) {
		c.Next()
		return
	}
	m := ms[i]
	m(c)
	if c.IsAborted() {
		return
	}
	runChain(c, ms, i+1)
}
