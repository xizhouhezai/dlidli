// Package middleware 提供全局 HTTP 中间件。
package middleware

import (
	"strings"
	"time"

	"github.com/dlidli/server/internal/pkg/errcode"
	"github.com/dlidli/server/internal/pkg/jwtx"
	"github.com/dlidli/server/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// CtxUserID 是认证中间件写入的当前登录用户 ID 键。
const CtxUserID = "user_id"

// TraceID 为每个请求生成/透传 trace id。
func TraceID() gin.HandlerFunc {
	return func(c *gin.Context) {
		tid := c.GetHeader("X-Request-Id")
		if tid == "" {
			tid = uuid.NewString()
		}
		c.Set(response.CtxTraceID, tid)
		c.Header("X-Request-Id", tid)
		c.Next()
	}
}

// AccessLog 记录访问日志。
func AccessLog(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		fields := []zap.Field{
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", time.Since(start)),
			zap.String("ip", c.ClientIP()),
			zap.String("trace_id", c.GetString(response.CtxTraceID)),
		}
		if len(c.Errors) > 0 {
			fields = append(fields, zap.String("errors", c.Errors.String()))
			log.Error("access", fields...)
			return
		}
		log.Info("access", fields...)
	}
}

// Recovery 捕获 panic，记录堆栈并返回统一错误。
func Recovery(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Error("panic recovered",
					zap.Any("error", err),
					zap.String("path", c.Request.URL.Path),
					zap.String("trace_id", c.GetString(response.CtxTraceID)),
					zap.Stack("stack"),
				)
				response.Fail(c, errcode.ErrInternal)
				c.Abort()
			}
		}()
		c.Next()
	}
}

// CORS 跨域支持；allowOrigins 为空时仅允许同源（不下发 CORS 头）。
func CORS(allowOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]bool, len(allowOrigins))
	allowAll := false
	for _, o := range allowOrigins {
		if o == "*" {
			allowAll = true
		}
		allowed[o] = true
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && (allowAll || allowed[origin]) {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-Id, Idempotency-Key")
			c.Header("Access-Control-Max-Age", "86400")
		}
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

// Auth 校验 Bearer token，写入 user_id；用于需要登录的路由组。
func Auth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		if token == "" {
			response.Fail(c, errcode.ErrUnauthorized)
			c.Abort()
			return
		}
		uid, err := jwtx.Parse(secret, token)
		if err != nil {
			response.Fail(c, errcode.ErrUnauthorized)
			c.Abort()
			return
		}
		c.Set(CtxUserID, uid)
		c.Next()
	}
}

// OptionalAuth 尝试解析 token，成功则注入 user_id，失败不拦截（游客可访问的接口用）。
func OptionalAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if token := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer "); token != "" {
			if uid, err := jwtx.Parse(secret, token); err == nil {
				c.Set(CtxUserID, uid)
			}
		}
		c.Next()
	}
}

// CtxAdminID 是后台认证中间件写入的管理员 ID 键。
const CtxAdminID = "admin_id"

// AdminAuth 校验后台管理员令牌。
func AdminAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		if token == "" {
			response.Fail(c, errcode.ErrUnauthorized)
			c.Abort()
			return
		}
		adminID, err := jwtx.ParseAdmin(secret, token)
		if err != nil {
			response.Fail(c, errcode.ErrUnauthorized)
			c.Abort()
			return
		}
		c.Set(CtxAdminID, adminID)
		c.Next()
	}
}

// PermChecker 判断管理员是否拥有指定权限码。
type PermChecker func(adminID int64, code string) (bool, error)

// RequirePerm 按权限码鉴权（需置于 AdminAuth 之后）。
func RequirePerm(check PermChecker, code string) gin.HandlerFunc {
	return func(c *gin.Context) {
		adminID := c.GetInt64(CtxAdminID)
		ok, err := check(adminID, code)
		if err != nil {
			response.Fail(c, errcode.ErrInternal)
			c.Abort()
			return
		}
		if !ok {
			response.Fail(c, errcode.ErrForbidden)
			c.Abort()
			return
		}
		c.Next()
	}
}
