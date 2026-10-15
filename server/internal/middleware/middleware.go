// Package middleware 提供全局 HTTP 中间件。
package middleware

import (
	"strconv"
	"strings"
	"time"

	"github.com/dlidli/server/internal/pkg/errcode"
	"github.com/dlidli/server/internal/pkg/jwtx"
	"github.com/dlidli/server/internal/pkg/playsign"
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

// bearerToken 提取访问令牌：优先 Authorization 头，其次 query（token/access_token）。
// WebSocket 无法自定义请求头，前端以 query 携带 token（弹幕实时连接等场景）。
func bearerToken(c *gin.Context) string {
	if t := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer "); t != "" {
		return t
	}
	if t := c.Query("token"); t != "" {
		return t
	}
	return c.Query("access_token")
}

// Auth 校验 Bearer token，写入 user_id；用于需要登录的路由组。
func Auth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := bearerToken(c)
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
		if token := bearerToken(c); token != "" {
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

// PlaySignGuard 校验播放地址签名（置于 /static 静态服务前）。
// 仅对 videos/ 下的播放入口（.m3u8/.mp4）强制签名；
// HLS 分片（.ts）与封面/头像等非播放入口资源放行。
//
// M4-VIP-02 起增加第二道闸：转码产物路径 `videos/hls/<视频>/<分P>/<档位>/...` 的请求，
// 除签名外还要**验会员**。两道闸互补：
//
//  1. **签名**（playsign.VerifyGated）把档位与用户签进待签内容 —— 免费用户把 URL 里的档位
//     从 360 改成 2160、或把会员拿到的 4K 签名串转给自己用，都在此处失败；
//  2. **会员判定**（gate.IsVip + tier.MemberOnly）—— 即使有人持有合法的 4K 门禁签名
//     （例如会员本人），校验时仍按**当前**会员状态再判一次，会员到期后旧签名立即失效。
//
// 两道闸都用 path 里解析出的档位（而非请求参数），参数侧无从伪造。
// gate 未填充时退化为升级前的仅签名口径（见 playgate.go 的说明）。
func PlaySignGuard(secret string, gate *PlayGate) gin.HandlerFunc {
	return func(c *gin.Context) {
		rel := strings.TrimPrefix(c.Request.URL.Path, "/static/")
		lower := strings.ToLower(rel)
		needSign := strings.Contains(lower, "videos/") &&
			(strings.HasSuffix(lower, ".m3u8") || strings.HasSuffix(lower, ".mp4"))
		if !needSign {
			c.Next()
			return
		}

		quality, gated := gatedTierQuality(rel)
		if !gated {
			// 非转码档位路径（原画源文件 videos/source/、审核预览等）：沿用历史签名口径，
			// 线上既有 URL 与签名算法一字未变。
			if !playsign.Verify(secret, rel, c.Query("e"), c.Query("s")) {
				c.AbortWithStatus(403)
				return
			}
			c.Next()
			return
		}

		// 门禁档位：档位取自**路径**（而不是请求参数），签名覆盖它，改 URL 参数无从生效。
		tier, tierKnown := gate.tiersSet().Get(quality)
		memberOnly := tierKnown && tier.MemberOnly

		// 会员限定档位：一律要求门禁签名（档位 + 用户都在待签内容里）。
		// 非会员限定档位：门禁签名与**历史签名（仅路径）都接受** ——
		// 后者是为「升级前已下发的 360/720 播放地址」留的兼容口：那些 URL 的签名里没有档位，
		// 若一律要求门禁签名，部署瞬间所有在播的旧 URL 会集体 403（TTL 6h 内无法自愈）。
		// 这不削弱安全性：非会员档位本就没有「改档位即可越权」的可图之处，
		// 而档位被运营改为会员限定后，此处的 memberOnly 立即为真 → 旧签名当场失效（fail closed）。
		// 档位未知（配置里没有该号，例如产物残留）同样按严格口径处理。
		if memberOnly || !tierKnown {
			if !verifyGatedRequest(secret, rel, quality, c) {
				c.AbortWithStatus(403)
				return
			}
			if memberOnly {
				uid, _ := strconv.ParseInt(c.Query("u"), 10, 64)
				if !gate.IsVip(c.Request.Context(), uid) {
					c.AbortWithStatus(403)
					return
				}
			}
			c.Next()
			return
		}

		// 非限档位：先试门禁签名，再退到历史签名。
		if verifyGatedRequest(secret, rel, quality, c) || playsign.Verify(secret, rel, c.Query("e"), c.Query("s")) {
			c.Next()
			return
		}
		c.AbortWithStatus(403)
	}
}

// verifyGatedRequest 校验门禁签名（档位参数须与路径一致、用户参数须可解析）。
func verifyGatedRequest(secret, rel string, quality int16, c *gin.Context) bool {
	qParam, ok := playsign.ParseQuality(c.Query("q"))
	if !ok || qParam != quality {
		return false
	}
	uid, err := strconv.ParseInt(c.Query("u"), 10, 64)
	if err != nil || uid < 0 {
		return false
	}
	return playsign.VerifyGated(secret, rel, c.Query("e"), quality, uid, c.Query("s"))
}

// gatedTierQuality 从播放路径解析转码档位号：
// `videos/hls/<videoID>/<partIndex>/<quality>/index.m3u8`（transcoder.go 的输出布局）。
// 命中返回 (档位, true)；非该布局（原画源文件 videos/source/、审核预览等）返回 false，
// 由调用方回落到历史签名口径。
func gatedTierQuality(rel string) (int16, bool) {
	segs := strings.Split(strings.TrimPrefix(rel, "/"), "/")
	// videos / hls / <videoID> / <partIndex> / <quality> / <file...>
	if len(segs) < 6 || segs[0] != "videos" || segs[1] != "hls" {
		return 0, false
	}
	q, err := strconv.ParseInt(segs[4], 10, 16)
	if err != nil {
		return 0, false
	}
	return int16(q), true
}
