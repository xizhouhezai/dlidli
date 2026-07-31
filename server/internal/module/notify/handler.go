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

// RegisterRoutes 注册通知路由（全部需登录）。
func (h *Handler) RegisterRoutes(v1 *gin.RouterGroup, auth gin.HandlerFunc) {
	g := v1.Group("/notifications", auth)
	{
		g.GET("", h.list)
		g.GET("/unread-count", h.unreadCount)
		g.POST("/read", h.markAllRead)
	}
}

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

func (h *Handler) unreadCount(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	cnt, err := h.svc.UnreadCount(c.Request.Context(), uid)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"count": cnt})
}

func (h *Handler) markAllRead(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	if err := h.svc.MarkAllRead(c.Request.Context(), uid); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}
