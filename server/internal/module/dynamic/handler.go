package dynamic

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

// RegisterRoutes 注册动态路由。
func (h *Handler) RegisterRoutes(v1 *gin.RouterGroup, auth gin.HandlerFunc) {
	v1.POST("/dynamics", auth, h.post)
	v1.POST("/dynamics/share", auth, h.share)
	v1.GET("/feed", auth, h.feed)
}

// @Summary  发布图文动态
// @Tags     动态
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    body body object true "content: 动态内容"
// @Success  200 {object} response.Body
// @Router   /dynamics [post]
func (h *Handler) post(c *gin.Context) {
	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	uid := c.GetInt64(middleware.CtxUserID)
	item, err := h.svc.PostText(c.Request.Context(), uid, req.Content)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, item)
}

// @Summary  转发视频到动态
// @Tags     动态
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    body body object true "bvid: 视频号; content: 转发语"
// @Success  200 {object} response.Body
// @Router   /dynamics/share [post]
func (h *Handler) share(c *gin.Context) {
	var req struct {
		Bvid    string `json:"bvid" binding:"required"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	uid := c.GetInt64(middleware.CtxUserID)
	item, err := h.svc.ShareVideo(c.Request.Context(), uid, req.Bvid, req.Content)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, item)
}

// @Summary  动态信息流（关注的 UP 主）
// @Tags     动态
// @Produce  json
// @Security BearerAuth
// @Param    cursor query string false "游标"
// @Success  200 {object} response.Body
// @Router   /feed [get]
func (h *Handler) feed(c *gin.Context) {
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if size < 1 || size > 50 {
		size = 20
	}
	uid := c.GetInt64(middleware.CtxUserID)
	items, next, hasMore, err := h.svc.Feed(c.Request.Context(), uid, c.Query("cursor"), size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"list": items, "next_cursor": next, "has_more": hasMore})
}
