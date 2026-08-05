package collection

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

// RegisterRoutes 注册合集路由：公开列表/详情；创建与归集需登录（仅本人可管理）。
func (h *Handler) RegisterRoutes(v1 *gin.RouterGroup, auth gin.HandlerFunc) {
	v1.GET("/collections", h.list)
	v1.GET("/collections/:id", h.detail)

	g := v1.Group("/collections", auth)
	{
		g.POST("", h.create)
		g.POST("/:id/videos", h.addVideo)
		g.DELETE("/:id/videos/:bvid", h.removeVideo)
		g.DELETE("/:id", h.delete)
	}
}

// @Summary  某 UP 主的合集列表
// @Tags     合集
// @Produce  json
// @Param    uid query int true "UP 主 uid"
// @Success  200 {object} response.Body
// @Router   /collections [get]
func (h *Handler) list(c *gin.Context) {
	uid, err := strconv.ParseInt(c.Query("uid"), 10, 64)
	if err != nil || uid <= 0 {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	cards, err := h.svc.ListByUser(c.Request.Context(), uid)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"list": cards})
}

// @Summary  合集详情（含稿件列表）
// @Tags     合集
// @Produce  json
// @Param    id path int true "合集 ID"
// @Success  200 {object} response.Body
// @Router   /collections/{id} [get]
func (h *Handler) detail(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	col, cards, err := h.svc.Detail(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"collection": col, "list": cards})
}

// @Summary  创建合集
// @Tags     合集
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    body body CreateReq true "合集信息"
// @Success  200 {object} response.Body
// @Router   /collections [post]
func (h *Handler) create(c *gin.Context) {
	var req CreateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	uid := c.GetInt64(middleware.CtxUserID)
	if err := h.svc.Create(c.Request.Context(), uid, &req); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

// @Summary  合集添加稿件
// @Tags     合集
// @Produce  json
// @Security BearerAuth
// @Param    id path int true "合集 ID"
// @Param    body body object true "{bvid: 稿件}"
// @Success  200 {object} response.Body
// @Router   /collections/{id}/videos [post]
func (h *Handler) addVideo(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	var req struct {
		Bvid string `json:"bvid" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	uid := c.GetInt64(middleware.CtxUserID)
	if err := h.svc.AddVideo(c.Request.Context(), uid, id, req.Bvid); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

// @Summary  合集移除稿件
// @Tags     合集
// @Produce  json
// @Security BearerAuth
// @Param    id path int true "合集 ID"
// @Param    bvid path string true "稿件 bvid"
// @Success  200 {object} response.Body
// @Router   /collections/{id}/videos/{bvid} [delete]
func (h *Handler) removeVideo(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	uid := c.GetInt64(middleware.CtxUserID)
	if err := h.svc.RemoveVideo(c.Request.Context(), uid, id, c.Param("bvid")); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

// @Summary  删除合集
// @Tags     合集
// @Produce  json
// @Security BearerAuth
// @Param    id path int true "合集 ID"
// @Success  200 {object} response.Body
// @Router   /collections/{id} [delete]
func (h *Handler) delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	uid := c.GetInt64(middleware.CtxUserID)
	if err := h.svc.Delete(c.Request.Context(), uid, id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}
