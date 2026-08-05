package im

import (
	"strconv"

	"github.com/dlidli/server/internal/middleware"
	"github.com/dlidli/server/internal/pkg/errcode"
	"github.com/dlidli/server/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes 注册私信路由（全部需登录；WS 经 Auth 中间件支持 query token）。
func (h *Handler) RegisterRoutes(v1 *gin.RouterGroup, auth gin.HandlerFunc) {
	g := v1.Group("/messages", auth)
	{
		g.POST("", h.send)
		g.GET("/conversations", h.conversations)
		g.GET("/:peer", h.messages)
		g.GET("/unread-count", h.unreadCount)
		g.GET("/ws", h.ws)
	}
}

// @Summary  发送私信（机审 + 未互关每日限制）
// @Tags     私信
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    body body SendReq true "消息内容"
// @Success  200 {object} response.Body
// @Router   /messages [post]
func (h *Handler) send(c *gin.Context) {
	var req SendReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	uid := c.GetInt64(middleware.CtxUserID)
	item, err := h.svc.Send(c.Request.Context(), uid, &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, item)
}

// @Summary  会话列表（按最新消息排序，含未读数）
// @Tags     私信
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} response.Body
// @Router   /messages/conversations [get]
func (h *Handler) conversations(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	items, err := h.svc.Conversations(c.Request.Context(), uid)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"list": items})
}

// @Summary  与某用户的消息记录（分页，读取即已读）
// @Tags     私信
// @Produce  json
// @Security BearerAuth
// @Param    peer path int true "对方 uid"
// @Param    page query int false "页码（默认1）"
// @Param    page_size query int false "每页条数（默认20）"
// @Success  200 {object} response.Body
// @Router   /messages/{peer} [get]
func (h *Handler) messages(c *gin.Context) {
	peer, err := strconv.ParseInt(c.Param("peer"), 10, 64)
	if err != nil || peer <= 0 {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	uid := c.GetInt64(middleware.CtxUserID)
	items, total, err := h.svc.Messages(c.Request.Context(), uid, peer, page, size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"list": items, "total": total})
}

// @Summary  私信总未读数
// @Tags     私信
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} response.Body
// @Router   /messages/unread-count [get]
func (h *Handler) unreadCount(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	n, err := h.svc.UnreadTotal(c.Request.Context(), uid)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"unread": n})
}

// @Summary  私信实时连接（WebSocket，query token）
// @Tags     私信
// @Security BearerAuth
// @Success  101 {string} string "Switching Protocols"
// @Router   /messages/ws [get]
func (h *Handler) ws(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	h.svc.hub.Serve(c, uid)
}
