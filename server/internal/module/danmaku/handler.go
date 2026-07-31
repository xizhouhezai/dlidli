package danmaku

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

// RegisterRoutes 注册弹幕域路由。
func (h *Handler) RegisterRoutes(v1 *gin.RouterGroup, auth gin.HandlerFunc) {
	g := v1.Group("/videos/:bvid/danmaku")
	{
		g.GET("", h.list)
		g.POST("", auth, h.send)
	}
}

func (h *Handler) send(c *gin.Context) {
	var req SendReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	uid := c.GetInt64(middleware.CtxUserID)
	item, err := h.svc.Send(c.Request.Context(), uid, c.Param("bvid"), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, item)
}

func (h *Handler) list(c *gin.Context) {
	seg, err := strconv.Atoi(c.DefaultQuery("segment", "0"))
	if err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	items, err := h.svc.ListSegment(c.Request.Context(), c.Param("bvid"), seg)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"segment": seg, "segment_ms": SegmentMS, "list": items})
}
