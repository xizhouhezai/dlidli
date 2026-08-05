package abtest

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

// RegisterRoutes 注册实验管理路由（admin，experiment:view/edit 权限）。
func (h *Handler) RegisterRoutes(v1 *gin.RouterGroup, adminAuth gin.HandlerFunc, perm func(code string) gin.HandlerFunc) {
	g := v1.Group("/admin/experiments", adminAuth)
	{
		g.GET("", perm("experiment:view"), h.list)
		g.POST("", perm("experiment:edit"), h.create)
		g.PUT("/:id", perm("experiment:edit"), h.update)
		g.DELETE("/:id", perm("experiment:edit"), h.delete)
	}
}

// @Summary  实验列表
// @Tags     管理后台-运营
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} response.Body
// @Router   /admin/experiments [get]
func (h *Handler) list(c *gin.Context) {
	list, err := h.svc.AdminList(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"list": list})
}

// @Summary  新建实验
// @Tags     管理后台-运营
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    body body SaveReq true "实验信息"
// @Success  200 {object} response.Body
// @Router   /admin/experiments [post]
func (h *Handler) create(c *gin.Context) {
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

// @Summary  编辑实验
// @Tags     管理后台-运营
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    id path int true "实验 ID"
// @Param    body body SaveReq true "实验信息"
// @Success  200 {object} response.Body
// @Router   /admin/experiments/{id} [put]
func (h *Handler) update(c *gin.Context) {
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

// @Summary  删除实验
// @Tags     管理后台-运营
// @Produce  json
// @Security BearerAuth
// @Param    id path int true "实验 ID"
// @Success  200 {object} response.Body
// @Router   /admin/experiments/{id} [delete]
func (h *Handler) delete(c *gin.Context) {
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
