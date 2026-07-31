package admin

import (
	"strconv"

	"github.com/dlidli/server/internal/middleware"
	"github.com/dlidli/server/internal/pkg/errcode"
	"github.com/dlidli/server/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// @Summary  新建权限点（页面 menu / 按钮 button）
// @Tags     管理后台-权限点
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    body body SavePermissionReq true "code; name; type=menu|button; parent; path; icon; sort"
// @Success  200 {object} response.Body
// @Router   /admin/permissions [post]
func (h *Handler) createPermission(c *gin.Context) {
	var req SavePermissionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	adminID := c.GetInt64(middleware.CtxAdminID)
	p, err := h.svc.CreatePermission(adminID, &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, p)
}

// @Summary  编辑权限点（code 锁定）
// @Tags     管理后台-权限点
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    id path string true "权限点ID"
// @Param    body body SavePermissionReq true "name; type; parent; path; icon; sort"
// @Success  200 {object} response.Body
// @Router   /admin/permissions/{id} [put]
func (h *Handler) updatePermission(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	var req SavePermissionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	adminID := c.GetInt64(middleware.CtxAdminID)
	if err := h.svc.UpdatePermission(adminID, id, &req); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

// @Summary  删除权限点（有子节点或被角色引用时禁删）
// @Tags     管理后台-权限点
// @Produce  json
// @Security BearerAuth
// @Param    id path string true "权限点ID"
// @Success  200 {object} response.Body
// @Router   /admin/permissions/{id} [delete]
func (h *Handler) deletePermission(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	adminID := c.GetInt64(middleware.CtxAdminID)
	if err := h.svc.DeletePermission(adminID, id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}
