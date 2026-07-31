package admin

import (
	"strconv"

	"github.com/dlidli/server/internal/middleware"
	"github.com/dlidli/server/internal/module/video"
	"github.com/dlidli/server/internal/pkg/errcode"
	"github.com/dlidli/server/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// @Summary  分区列表（含停用）
// @Tags     管理后台-分区
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} response.Body
// @Router   /admin/categories [get]
func (h *Handler) listCategories(c *gin.Context) {
	list, err := h.svc.ListCategories(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"list": list})
}

// @Summary  新建分区
// @Tags     管理后台-分区
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    body body video.SaveCategoryReq true "parent_id; name; sort; status"
// @Success  200 {object} response.Body
// @Router   /admin/categories [post]
func (h *Handler) createCategory(c *gin.Context) {
	var req video.SaveCategoryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	adminID := c.GetInt64(middleware.CtxAdminID)
	cat, err := h.svc.CreateCategory(c.Request.Context(), adminID, &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, cat)
}

// @Summary  编辑分区
// @Tags     管理后台-分区
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    id path int true "分区ID"
// @Param    body body video.SaveCategoryReq true "name; sort; status"
// @Success  200 {object} response.Body
// @Router   /admin/categories/{id} [put]
func (h *Handler) updateCategory(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	var req video.SaveCategoryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	adminID := c.GetInt64(middleware.CtxAdminID)
	if err := h.svc.UpdateCategory(c.Request.Context(), adminID, id, &req); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

// @Summary  删除分区（有子分区或稿件时禁删）
// @Tags     管理后台-分区
// @Produce  json
// @Security BearerAuth
// @Param    id path int true "分区ID"
// @Success  200 {object} response.Body
// @Router   /admin/categories/{id} [delete]
func (h *Handler) deleteCategory(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	adminID := c.GetInt64(middleware.CtxAdminID)
	if err := h.svc.DeleteCategory(c.Request.Context(), adminID, id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}
