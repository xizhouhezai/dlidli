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

			// 权限点全集（角色分配树用）
			authed.GET("/permissions", perm("role:view"), h.listPermissions)
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
		}
	}
}

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

func (h *Handler) listWords(c *gin.Context) {
	list, err := h.svc.ListWords()
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"list": list})
}

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
