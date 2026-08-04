package recommend

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

// RegisterRoutes 注册推荐域路由。
// 推荐列表/热度榜游客可访问（optionalAuth）；行为上报与负反馈需登录（游客不上报）。
func (h *Handler) RegisterRoutes(v1 *gin.RouterGroup, auth, optionalAuth gin.HandlerFunc) {
	v1.GET("/recommend/videos", optionalAuth, h.recommend)
	v1.GET("/videos/hot", optionalAuth, h.hot)

	v1.POST("/behaviors", auth, h.reportBehavior)
	v1.POST("/dislikes", auth, h.addDislike)
	v1.GET("/users/me/recommend-settings", auth, h.setting)
	v1.PUT("/users/me/recommend-settings", auth, h.setSetting)
}

// @Summary  首页推荐信息流（混合召回：热度+兴趣分区+新稿池，过滤已看/负反馈后打散）
// @Tags     推荐
// @Produce  json
// @Param    page query int false "页码（默认1）"
// @Param    size query int false "每页条数（默认20，最大50）"
// @Success  200 {object} response.Body
// @Router   /recommend/videos [get]
func (h *Handler) recommend(c *gin.Context) {
	page, size := pagination(c)
	uid := c.GetInt64(middleware.CtxUserID)
	cards, err := h.svc.Recommend(c.Request.Context(), uid, page, size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"list": cards})
}

// @Summary  全站/分区热度榜（加权分）
// @Tags     推荐
// @Produce  json
// @Param    category_id query int false "分区ID（0=全站）"
// @Param    page query int false "页码"
// @Param    size query int false "每页条数"
// @Success  200 {object} response.Body
// @Router   /videos/hot [get]
func (h *Handler) hot(c *gin.Context) {
	categoryID, _ := strconv.Atoi(c.DefaultQuery("category_id", "0"))
	page, size := pagination(c)
	cards, err := h.svc.HotVideos(c.Request.Context(), categoryID, page, size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"list": cards})
}

// @Summary  行为上报（曝光/点击/播放/互动，批量）
// @Tags     推荐
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    body body object true "items: [{video_id, action}]"
// @Success  200 {object} response.Body
// @Router   /behaviors [post]
func (h *Handler) reportBehavior(c *gin.Context) {
	var req struct {
		Items []BehaviorItem `json:"items" binding:"required,min=1,max=50"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	uid := c.GetInt64(middleware.CtxUserID)
	h.svc.ReportBehavior(c.Request.Context(), uid, req.Items)
	response.OK(c, nil)
}

// @Summary  负反馈（不感兴趣：1内容 2UP主 3分区）
// @Tags     推荐
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    body body object true "target_type, target_id"
// @Success  200 {object} response.Body
// @Router   /dislikes [post]
func (h *Handler) addDislike(c *gin.Context) {
	var req struct {
		TargetType int8   `json:"target_type" binding:"required,oneof=1 2 3"`
		TargetID   string `json:"target_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	uid := c.GetInt64(middleware.CtxUserID)
	if err := h.svc.AddDislike(c.Request.Context(), uid, req.TargetType, req.TargetID); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

// @Summary  个性化推荐开关状态
// @Tags     推荐
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} response.Body
// @Router   /users/me/recommend-settings [get]
func (h *Handler) setting(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	on, err := h.svc.RecommendSetting(c.Request.Context(), uid)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"enabled": on})
}

// @Summary  开关个性化推荐（关闭后退化为热度榜，合规）
// @Tags     推荐
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    body body object true "enabled: 是否开启"
// @Success  200 {object} response.Body
// @Router   /users/me/recommend-settings [put]
func (h *Handler) setSetting(c *gin.Context) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	uid := c.GetInt64(middleware.CtxUserID)
	if err := h.svc.SetRecommendSetting(c.Request.Context(), uid, req.Enabled); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
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
		if v, err := strconv.Atoi(s); err == nil && v > 0 && v <= 50 {
			size = v
		}
	}
	return page, size
}
