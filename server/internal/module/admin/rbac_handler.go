package admin

import (
	"strconv"

	"github.com/dlidli/server/internal/middleware"
	"github.com/dlidli/server/internal/pkg/errcode"
	"github.com/dlidli/server/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// myPermissions 当前登录管理员的权限码与可见菜单。
// @Summary  当前登录者权限码与可见菜单
// @Tags     管理后台-RBAC
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} response.Body
// @Router   /admin/me/permissions [get]
func (h *Handler) myPermissions(c *gin.Context) {
	adminID := c.GetInt64(middleware.CtxAdminID)
	res, err := h.svc.CurrentPerm(c.Request.Context(), adminID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// listPermissions 权限点全集。
// @Summary  权限点全集
// @Tags     管理后台-RBAC
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} response.Body
// @Router   /admin/permissions [get]
func (h *Handler) listPermissions(c *gin.Context) {
	list, err := h.svc.AllPermissions()
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"list": list})
}

// ---- 角色 ----

// @Summary  角色列表
// @Tags     管理后台-RBAC
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} response.Body
// @Router   /admin/roles [get]
func (h *Handler) listRoles(c *gin.Context) {
	list, err := h.svc.ListRoles()
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"list": list})
}

// @Summary  新建角色
// @Tags     管理后台-RBAC
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    body body SaveRoleReq true "name; code; remark; perms"
// @Success  200 {object} response.Body
// @Router   /admin/roles [post]
func (h *Handler) createRole(c *gin.Context) {
	var req SaveRoleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	adminID := c.GetInt64(middleware.CtxAdminID)
	role, err := h.svc.CreateRole(adminID, &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, role)
}

// @Summary  编辑角色/分配权限
// @Tags     管理后台-RBAC
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    id path string true "角色ID"
// @Param    body body SaveRoleReq true "name; remark; perms"
// @Success  200 {object} response.Body
// @Router   /admin/roles/{id} [put]
func (h *Handler) updateRole(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	var req SaveRoleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	adminID := c.GetInt64(middleware.CtxAdminID)
	if err := h.svc.UpdateRole(adminID, id, &req); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

// @Summary  删除角色（内置角色禁删）
// @Tags     管理后台-RBAC
// @Produce  json
// @Security BearerAuth
// @Param    id path string true "角色ID"
// @Success  200 {object} response.Body
// @Router   /admin/roles/{id} [delete]
func (h *Handler) deleteRole(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	adminID := c.GetInt64(middleware.CtxAdminID)
	if err := h.svc.DeleteRole(adminID, id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

// ---- 账号 ----

// @Summary  管理员账号列表
// @Tags     管理后台-RBAC
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} response.Body
// @Router   /admin/admins [get]
func (h *Handler) listAdmins(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 50 {
		size = 20
	}
	list, total, err := h.svc.ListAdmins(page, size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"list": list, "total": total})
}

// @Summary  新建管理员账号
// @Tags     管理后台-RBAC
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    body body SaveAdminReq true "username; nickname; password; role_ids"
// @Success  200 {object} response.Body
// @Router   /admin/admins [post]
func (h *Handler) createAdmin(c *gin.Context) {
	var req SaveAdminReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	adminID := c.GetInt64(middleware.CtxAdminID)
	u, err := h.svc.CreateAdmin(adminID, &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"id": strconv.FormatInt(u.ID, 10)})
}

// @Summary  编辑管理员账号
// @Tags     管理后台-RBAC
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    id path string true "账号ID"
// @Param    body body SaveAdminReq true "nickname; role_ids"
// @Success  200 {object} response.Body
// @Router   /admin/admins/{id} [put]
func (h *Handler) updateAdmin(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	var req SaveAdminReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	adminID := c.GetInt64(middleware.CtxAdminID)
	if err := h.svc.UpdateAdmin(adminID, id, &req); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

// @Summary  启用/停用账号
// @Tags     管理后台-RBAC
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    id path string true "账号ID"
// @Param    body body object true "status: 0启用/1停用"
// @Success  200 {object} response.Body
// @Router   /admin/admins/{id}/toggle [post]
func (h *Handler) toggleAdmin(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	var body struct {
		Status int8 `json:"status"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	adminID := c.GetInt64(middleware.CtxAdminID)
	if err := h.svc.ToggleAdmin(adminID, id, body.Status); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

// @Summary  重置账号密码
// @Tags     管理后台-RBAC
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    id path string true "账号ID"
// @Param    body body ResetPwdReq true "password"
// @Success  200 {object} response.Body
// @Router   /admin/admins/{id}/reset-password [post]
func (h *Handler) resetAdminPwd(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	var req ResetPwdReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	adminID := c.GetInt64(middleware.CtxAdminID)
	if err := h.svc.ResetAdminPwd(adminID, id, req.Password); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

// @Summary  删除管理员账号
// @Tags     管理后台-RBAC
// @Produce  json
// @Security BearerAuth
// @Param    id path string true "账号ID"
// @Success  200 {object} response.Body
// @Router   /admin/admins/{id} [delete]
func (h *Handler) deleteAdmin(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	adminID := c.GetInt64(middleware.CtxAdminID)
	if err := h.svc.DeleteAdmin(adminID, id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}
