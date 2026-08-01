package growth

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

// RegisterRoutes 注册成长域路由（均需登录）。
func (h *Handler) RegisterRoutes(v1 *gin.RouterGroup, auth gin.HandlerFunc) {
	g := v1.Group("/growth", auth)
	{
		g.GET("/summary", h.summary)
		g.GET("/exp-logs", h.expLogs)
	}
}

// @Summary  成长总览（等级/经验进度/今日任务）
// @Tags     成长
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} response.Body
// @Router   /growth/summary [get]
func (h *Handler) summary(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	sum, err := h.svc.Summary(c.Request.Context(), uid)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, sum)
}

// @Summary  经验流水分页
// @Tags     成长
// @Produce  json
// @Security BearerAuth
// @Param    page query int false "页码（默认1）"
// @Param    size query int false "每页条数（默认20）"
// @Success  200 {object} response.Body
// @Router   /growth/exp-logs [get]
func (h *Handler) expLogs(c *gin.Context) {
	page, size := pagination(c)
	uid := c.GetInt64(middleware.CtxUserID)
	list, total, err := h.svc.ExpLogs(c.Request.Context(), uid, page, size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"list": list, "total": total})
}

func pagination(c *gin.Context) (page, size int) {
	page = 1
	size = 20
	if p := c.Query("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if s := c.Query("size"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 && v <= 100 {
			size = v
		}
	}
	return page, size
}
