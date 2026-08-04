package banner

import (
	"strconv"

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

// RegisterRoutes 注册运营位路由：
// 公开 GET /banners（首页轮播）；admin /admin/banners（banner:view/edit 权限）。
func (h *Handler) RegisterRoutes(v1 *gin.RouterGroup, adminAuth gin.HandlerFunc, perm func(code string) gin.HandlerFunc) {
	v1.GET("/banners", h.list)

	g := v1.Group("/admin/banners", adminAuth)
	{
		g.GET("", perm("banner:view"), h.adminList)
		g.POST("", perm("banner:edit"), h.adminCreate)
		g.PUT("/:id", perm("banner:edit"), h.adminUpdate)
		g.DELETE("/:id", perm("banner:edit"), h.adminDelete)
	}
}

// @Summary  首页轮播 Banner（启用项）
// @Tags     运营
// @Produce  json
// @Success  200 {object} response.Body
// @Router   /banners [get]
func (h *Handler) list(c *gin.Context) {
	items, err := h.svc.Banners(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"list": items})
}

// @Summary  Banner 列表（admin）
// @Tags     管理后台-运营
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} response.Body
// @Router   /admin/banners [get]
func (h *Handler) adminList(c *gin.Context) {
	list, err := h.svc.AdminList(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"list": list})
}

// @Summary  新建 Banner
// @Tags     管理后台-运营
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    body body SaveReq true "Banner 信息"
// @Success  200 {object} response.Body
// @Router   /admin/banners [post]
func (h *Handler) adminCreate(c *gin.Context) {
	var req SaveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	if err := h.svc.AdminCreate(c.Request.Context(), &req); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

// @Summary  编辑 Banner
// @Tags     管理后台-运营
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    id path int true "Banner ID"
// @Param    body body SaveReq true "Banner 信息"
// @Success  200 {object} response.Body
// @Router   /admin/banners/{id} [put]
func (h *Handler) adminUpdate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	var req SaveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	if err := h.svc.AdminUpdate(c.Request.Context(), id, &req); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

// @Summary  删除 Banner
// @Tags     管理后台-运营
// @Produce  json
// @Security BearerAuth
// @Param    id path int true "Banner ID"
// @Success  200 {object} response.Body
// @Router   /admin/banners/{id} [delete]
func (h *Handler) adminDelete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	if err := h.svc.AdminDelete(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}
