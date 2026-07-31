package admin

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

// RegisterRoutes 注册后台路由（/api/v1/admin）。
func (h *Handler) RegisterRoutes(v1 *gin.RouterGroup, adminAuth gin.HandlerFunc) {
	// 鉴权中间件工厂：按权限码校验
	perm := func(code string) gin.HandlerFunc {
		return middleware.RequirePerm(h.svc.HasPerm, code)
	}
	g := v1.Group("/admin")
	{
		g.POST("/login", h.login)

		authed := g.Group("", adminAuth)
		{
			// 当前登录者权限/菜单（无需特定权限）
			authed.GET("/me/permissions", h.myPermissions)

			authed.GET("/videos/review", perm("review:view"), h.reviewList)
			authed.POST("/videos/:bvid/review", perm("review:approve"), h.review)
			authed.GET("/sensitive-words", perm("sensitive:view"), h.listWords)
			authed.POST("/sensitive-words", perm("sensitive:edit"), h.addWord)
			authed.DELETE("/sensitive-words/:id", perm("sensitive:edit"), h.deleteWord)
			authed.GET("/users", perm("user:view"), h.listUsers)
			authed.POST("/users/:id/punish", perm("user:punish"), h.punishUser)

			// 权限点：目录为元数据（角色分配树与权限管理页共用），读取仅需登录；写操作需 permission:edit
			authed.GET("/permissions", h.listPermissions)
			authed.POST("/permissions", perm("permission:edit"), h.createPermission)
			authed.PUT("/permissions/:id", perm("permission:edit"), h.updatePermission)
			authed.DELETE("/permissions/:id", perm("permission:edit"), h.deletePermission)
			// 角色管理
			authed.GET("/roles", perm("role:view"), h.listRoles)
			authed.POST("/roles", perm("role:edit"), h.createRole)
			authed.PUT("/roles/:id", perm("role:edit"), h.updateRole)
			authed.DELETE("/roles/:id", perm("role:edit"), h.deleteRole)
			// 账号管理
			authed.GET("/admins", perm("admin:view"), h.listAdmins)
			authed.POST("/admins", perm("admin:edit"), h.createAdmin)
			authed.PUT("/admins/:id", perm("admin:edit"), h.updateAdmin)
			authed.POST("/admins/:id/toggle", perm("admin:edit"), h.toggleAdmin)
			authed.POST("/admins/:id/reset-password", perm("admin:edit"), h.resetAdminPwd)
			authed.DELETE("/admins/:id", perm("admin:edit"), h.deleteAdmin)
			// 分区管理
			authed.GET("/categories", perm("category:view"), h.listCategories)
			authed.POST("/categories", perm("category:edit"), h.createCategory)
			authed.PUT("/categories/:id", perm("category:edit"), h.updateCategory)
			authed.DELETE("/categories/:id", perm("category:edit"), h.deleteCategory)
		}
	}
}

// @Summary  后台登录
// @Tags     管理后台
// @Accept   json
// @Produce  json
// @Param    body body LoginReq true "管理员账密"
// @Success  200 {object} response.Body "data: {token, username, role}"
// @Router   /admin/login [post]
func (h *Handler) login(c *gin.Context) {
	var req LoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	resp, err := h.svc.Login(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, resp)
}

// @Summary  待审稿件队列
// @Tags     管理后台
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} response.Body
// @Router   /admin/videos/review [get]
func (h *Handler) reviewList(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 50 {
		size = 20
	}
	items, total, err := h.svc.ReviewList(c.Request.Context(), page, size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"list": items, "total": total})
}

// @Summary  审核稿件（通过/驳回）
// @Tags     管理后台
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    bvid path string true "视频 BV 号"
// @Param    body body ReviewReq true "approve; reason"
// @Success  200 {object} response.Body
// @Router   /admin/videos/{bvid}/review [post]
func (h *Handler) review(c *gin.Context) {
	var req ReviewReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	adminID := c.GetInt64(middleware.CtxAdminID)
	if err := h.svc.Review(c.Request.Context(), adminID, c.Param("bvid"), &req); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

// @Summary  敏感词列表
// @Tags     管理后台
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} response.Body
// @Router   /admin/sensitive-words [get]
func (h *Handler) listWords(c *gin.Context) {
	list, err := h.svc.ListWords()
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"list": list})
}

// @Summary  新增敏感词
// @Tags     管理后台
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    body body AddWordReq true "word"
// @Success  200 {object} response.Body
// @Router   /admin/sensitive-words [post]
func (h *Handler) addWord(c *gin.Context) {
	var req AddWordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	adminID := c.GetInt64(middleware.CtxAdminID)
	w, err := h.svc.AddWord(adminID, req.Word)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, w)
}

// @Summary  删除敏感词
// @Tags     管理后台
// @Produce  json
// @Security BearerAuth
// @Param    id path string true "敏感词ID"
// @Success  200 {object} response.Body
// @Router   /admin/sensitive-words/{id} [delete]
func (h *Handler) deleteWord(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	adminID := c.GetInt64(middleware.CtxAdminID)
	if err := h.svc.DeleteWord(adminID, id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

// @Summary  用户查询（UID/手机号/昵称+状态）
// @Tags     管理后台
// @Produce  json
// @Security BearerAuth
// @Param    keyword query string false "UID/手机号/昵称"
// @Param    status query int false "状态 -1全部/0正常/1禁言/2封禁"
// @Success  200 {object} response.Body
// @Router   /admin/users [get]
func (h *Handler) listUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	status, _ := strconv.Atoi(c.DefaultQuery("status", "-1"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 50 {
		size = 20
	}
	list, total, err := h.svc.ListUsers(c.Request.Context(), c.Query("keyword"), status, page, size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"list": list, "total": total})
}

// @Summary  用户处罚（禁言/封禁/解除）
// @Tags     管理后台
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    id path string true "用户ID"
// @Param    body body PunishReq true "action; days; reason"
// @Success  200 {object} response.Body
// @Router   /admin/users/{id}/punish [post]
func (h *Handler) punishUser(c *gin.Context) {
	uid, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	var req PunishReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	adminID := c.GetInt64(middleware.CtxAdminID)
	if err := h.svc.PunishUser(c.Request.Context(), adminID, uid, req.Action, req.Days, req.Reason); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}
