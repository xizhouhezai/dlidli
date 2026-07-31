package upload

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

// RegisterRoutes 注册上传域路由（全部需要登录）。
func (h *Handler) RegisterRoutes(v1 *gin.RouterGroup, auth gin.HandlerFunc) {
	g := v1.Group("/upload", auth)
	{
		g.POST("/init", h.init)
		g.PUT("/:id/parts/:index", h.uploadPart)
		g.GET("/:id", h.progress)
		g.POST("/:id/complete", h.complete)
	}
}

func (h *Handler) init(c *gin.Context) {
	var req InitReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	uid := c.GetInt64(middleware.CtxUserID)
	resp, err := h.svc.Init(c.Request.Context(), uid, &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, resp)
}

func (h *Handler) uploadPart(c *gin.Context) {
	index, err := strconv.Atoi(c.Param("index"))
	if err != nil {
		response.Fail(c, errcode.ErrChunkIndexInvalid)
		return
	}
	uid := c.GetInt64(middleware.CtxUserID)
	if err := h.svc.UploadPart(c.Request.Context(), uid, c.Param("id"), index, c.Request.Body); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

func (h *Handler) progress(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	resp, err := h.svc.Progress(c.Request.Context(), uid, c.Param("id"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, resp)
}

func (h *Handler) complete(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	resp, err := h.svc.Complete(c.Request.Context(), uid, c.Param("id"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, resp)
}
