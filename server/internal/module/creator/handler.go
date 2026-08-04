package creator

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

// RegisterRoutes 注册创作者中心路由（均需登录）。
func (h *Handler) RegisterRoutes(v1 *gin.RouterGroup, auth gin.HandlerFunc) {
	g := v1.Group("/creator", auth)
	{
		g.GET("/overview", h.overview)
		g.GET("/videos", h.videoStats)
		g.GET("/trend", h.trend)
		g.GET("/settlements", h.settlements)
	}
}

// @Summary  创作者总览（播放/互动/粉丝/收益/近7日播放）
// @Tags     创作者中心
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} response.Body
// @Router   /creator/overview [get]
func (h *Handler) overview(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	ov, err := h.svc.Overview(c.Request.Context(), uid)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, ov)
}

// @Summary  稿件数据列表（含有效播放与收益）
// @Tags     创作者中心
// @Produce  json
// @Security BearerAuth
// @Param    page query int false "页码（默认1）"
// @Param    size query int false "每页条数（默认10）"
// @Success  200 {object} response.Body
// @Router   /creator/videos [get]
func (h *Handler) videoStats(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	page, size := pagination(c)
	items, total, err := h.svc.VideoStats(c.Request.Context(), uid, page, size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"list": items, "total": total})
}

// @Summary  近 N 天数据趋势（指标可切换：play/interact/click/expose）
// @Tags     创作者中心
// @Produce  json
// @Security BearerAuth
// @Param    days query int false "天数（默认7，最大30）"
// @Param    metric query string false "指标：play 播放 / interact 互动 / click 点击 / expose 曝光（默认 play）"
// @Success  200 {object} response.Body
// @Router   /creator/trend [get]
func (h *Handler) trend(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "7"))
	metric := c.DefaultQuery("metric", "play")
	uid := c.GetInt64(middleware.CtxUserID)
	list, err := h.svc.Trend(c.Request.Context(), uid, days, metric)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"list": list, "metric": metric})
}

// @Summary  收益明细分页
// @Tags     创作者中心
// @Produce  json
// @Security BearerAuth
// @Param    page query int false "页码（默认1）"
// @Param    size query int false "每页条数（默认10）"
// @Success  200 {object} response.Body
// @Router   /creator/settlements [get]
func (h *Handler) settlements(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	page, size := pagination(c)
	items, total, err := h.svc.Settlements(c.Request.Context(), uid, page, size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"list": items, "total": total})
}

func pagination(c *gin.Context) (page, size int) {
	page = 1
	size = 10
	if p := c.Query("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if s := c.Query("size"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 && v <= 50 {
			size = v
		}
	}
	return page, size
}
