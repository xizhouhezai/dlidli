// Package wsorigin 提供四个 WS hub（danmaku/notify/im/live）共用的 Origin 校验策略。
//
// 为什么需要它：浏览器同源策略不约束 WebSocket，故服务端必须自行校验 Origin，
// 防「跨站 WS 劫持」（CSWSH：恶意页面借浏览器**自动携带的凭据**连接受害站点）。
// 本仓库的 WS 凭据只有 Bearer / query token —— **全仓无 Cookie 鉴权**，
// 即凭据不由浏览器环境自动携带：恶意页面无法让浏览器附上受害者的 token，
// 而已经持有 token 的攻击者本就能在浏览器之外直接调用 API。
// 故 Origin 校验在本仓库防的是「环境自动凭据」这一类攻击，而这类攻击在此不可成立。
//
// 但 native 放行仍须有明确判据，不能裸放行。策略（四 hub 共用同一实现，杜绝语义漂移）：
//  1. 无 Origin 头 → 放行。非浏览器客户端（原生 App、脚本、服务端）本就不发该头；
//  2. Origin 命中白名单 → 放行，浏览器端防护与升级前逐字一致；
//  3. Origin 未命中白名单，但请求携带**经 JWT 校验有效的** access token
//     且 AllowNativeClients 已开启 → 放行。移动端/native（如鸿蒙 ArkTS WebSocket）
//     自生成 `http://<host>`（不带端口）、`file://` 一类浏览器不会产生的来源，
//     其放行判据是「显式凭据」而非来源形态——非法来源仍被拒；
//  4. 其余 → 拒绝。
package wsorigin

import (
	"net/http"
	"strings"

	"github.com/dlidli/server/internal/pkg/jwtx"
	"go.uber.org/zap"
)

// Config 是 Origin 校验所需的全部输入，由 router 装配时从应用配置映射而来。
type Config struct {
	AllowOrigins []string // app.allowOrigins：浏览器来源白名单
	JWTSecret    string   // jwt.secret：用于校验请求携带的 access token
	// AllowNativeClients 开启「携带有效 token 即放行非白名单 Origin」（app.allowNativeClients）。
	// 默认关闭：漏配时按拒绝处理（fail closed），不会静默放宽。
	AllowNativeClients bool
}

// New 构建 CheckOrigin 判定函数（websocket.Upgrader.CheckOrigin 的签名）。
// 判定与拒绝都会写日志：放行 native 例外与拒绝非法来源都可被运维检索。
func New(cfg Config, log *zap.Logger) func(*http.Request) bool {
	allow := make(map[string]bool, len(cfg.AllowOrigins))
	for _, o := range cfg.AllowOrigins {
		if o == "" {
			continue
		}
		allow[normalize(o)] = true
	}
	return func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true // 非浏览器客户端：无 Origin 头
		}
		if allow[normalize(origin)] {
			return true // 白名单来源
		}
		if cfg.AllowNativeClients {
			if uid, ok := nativeUID(cfg.JWTSecret, r); ok {
				if log != nil {
					log.Info("WS Origin 放行（native 客户端：有效 token）",
						zap.String("origin", origin),
						zap.String("path", r.URL.Path),
						zap.Int64("uid", uid))
				}
				return true
			}
		}
		if log != nil {
			log.Warn("WS Origin 拒绝",
				zap.String("origin", origin),
				zap.String("path", r.URL.Path))
		}
		return false
	}
}

// nativeUID 校验请求携带的 access token；有效则返回用户 ID。
// 仅接受 C 端用户令牌——jwtx.Parse 会拒绝管理员令牌（两条令牌互不通用）。
func nativeUID(secret string, r *http.Request) (int64, bool) {
	token := credential(r)
	if token == "" || secret == "" {
		return 0, false
	}
	uid, err := jwtx.Parse(secret, token)
	if err != nil {
		return 0, false
	}
	return uid, true
}

// credential 提取访问令牌：优先 Authorization 头，其次 query（token/access_token）。
// 语义须与 internal/middleware.bearerToken 保持一致（WebSocket 无法自定义请求头，
// 客户端以 query 携带 token），两侧任一改动都应同步。
func credential(r *http.Request) string {
	if t := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "); t != "" {
		return t
	}
	q := r.URL.Query()
	if t := q.Get("token"); t != "" {
		return t
	}
	return q.Get("access_token")
}

// normalize 归一切片来源：去尾斜杠（与升级前 allow[strings.TrimRight(o, "/")] 逐字一致）。
func normalize(origin string) string {
	return strings.TrimRight(origin, "/")
}
