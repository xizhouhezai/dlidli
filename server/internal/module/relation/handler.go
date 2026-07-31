package relation

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

// RegisterRoutes 注册关系链路由。
// 挂在 /space/:id 下（个人空间域），避开 /users/me 与 :id 的 gin 路由冲突。
func (h *Handler) RegisterRoutes(v1 *gin.RouterGroup, auth, optionalAuth gin.HandlerFunc) {
	g := v1.Group("/space/:id")
	{
		g.GET("/profile", h.profile)
		g.POST("/follow", auth, h.toggle)
		g.GET("/relation", optionalAuth, h.stat)
		g.GET("/followings", h.followings)
		g.GET("/followers", h.followers)
	}
}

func targetID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Fail(c, errcode.ErrInvalidParams)
		return 0, false
	}
	return id, true
}

// @Summary  用户空间资料
// @Tags     关系
// @Produce  json
// @Param    id path string true "用户ID"
// @Success  200 {object} response.Body
// @Router   /space/{id}/profile [get]
func (h *Handler) profile(c *gin.Context) {
	target, ok := targetID(c)
	if !ok {
		return
	}
	p, err := h.svc.PublicProfile(c.Request.Context(), target)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, p)
}

// @Summary  关注/取消关注
// @Tags     关系
// @Produce  json
// @Security BearerAuth
// @Param    id path string true "目标用户ID"
// @Success  200 {object} response.Body
// @Router   /space/{id}/follow [post]
func (h *Handler) toggle(c *gin.Context) {
	target, ok := targetID(c)
	if !ok {
		return
	}
	uid := c.GetInt64(middleware.CtxUserID)
	following, err := h.svc.Toggle(c.Request.Context(), uid, target)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"following": following})
}

// @Summary  关系统计（关注/粉丝数+是否已关注）
// @Tags     关系
// @Produce  json
// @Param    id path string true "用户ID"
// @Success  200 {object} response.Body
// @Router   /space/{id}/relation [get]
func (h *Handler) stat(c *gin.Context) {
	target, ok := targetID(c)
	if !ok {
		return
	}
	viewer := c.GetInt64(middleware.CtxUserID)
	st, err := h.svc.Stat(c.Request.Context(), viewer, target)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, st)
}

// @Summary  关注列表
// @Tags     关系
// @Produce  json
// @Param    id path string true "用户ID"
// @Success  200 {object} response.Body
// @Router   /space/{id}/followings [get]
func (h *Handler) followings(c *gin.Context) {
	target, ok := targetID(c)
	if !ok {
		return
	}
	page, size := pagination(c)
	list, total, err := h.svc.Followings(c.Request.Context(), target, page, size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"list": list, "total": total})
}

// @Summary  粉丝列表
// @Tags     关系
// @Produce  json
// @Param    id path string true "用户ID"
// @Success  200 {object} response.Body
// @Router   /space/{id}/followers [get]
func (h *Handler) followers(c *gin.Context) {
	target, ok := targetID(c)
	if !ok {
		return
	}
	page, size := pagination(c)
	list, total, err := h.svc.Followers(c.Request.Context(), target, page, size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"list": list, "total": total})
}

func pagination(c *gin.Context) (page, size int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ = strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 50 {
		size = 20
	}
	return page, size
}
