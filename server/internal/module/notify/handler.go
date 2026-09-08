package notify

import (
	"strconv"

	"github.com/dlidli/server/internal/middleware"
	"github.com/dlidli/server/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes 注册通知路由（全部需登录；WS 经 Auth 中间件支持 query token）。
func (h *Handler) RegisterRoutes(v1 *gin.RouterGroup, auth gin.HandlerFunc) {
	g := v1.Group("/notifications", auth)
	{
		g.GET("", h.list)
		g.GET("/unread-count", h.unreadCount)
		g.POST("/read", h.markAllRead)
		g.GET("/ws", h.ws)
	}
}

// @Summary  通知实时连接（WebSocket，query token）
// @Tags     通知
// @Security BearerAuth
// @Success  101 {string} string "Switching Protocols"
// @Router   /notifications/ws [get]
func (h *Handler) ws(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	h.svc.WS(c, uid)
}

// @Summary  通知列表
// @Tags     通知
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} response.Body
// @Router   /notifications [get]
func (h *Handler) list(c *gin.Context) {
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if size < 1 || size > 50 {
		size = 20
	}
	uid := c.GetInt64(middleware.CtxUserID)
	items, next, hasMore, err := h.svc.List(c.Request.Context(), uid, c.Query("cursor"), size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"list": items, "next_cursor": next, "has_more": hasMore})
}

// @Summary  未读数
// @Tags     通知
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} response.Body
// @Router   /notifications/unread-count [get]
func (h *Handler) unreadCount(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	cnt, err := h.svc.UnreadCount(c.Request.Context(), uid)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"count": cnt})
}

// @Summary  全部标记已读
// @Tags     通知
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} response.Body
// @Router   /notifications/read [post]
func (h *Handler) markAllRead(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	if err := h.svc.MarkAllRead(c.Request.Context(), uid); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}
