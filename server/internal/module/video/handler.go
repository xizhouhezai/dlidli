package video

import (
	"fmt"
	"strconv"

	"github.com/dlidli/server/internal/middleware"
	"github.com/dlidli/server/internal/pkg/errcode"
	"github.com/dlidli/server/internal/pkg/response"
	"github.com/dlidli/server/internal/pkg/storage"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc   *Service
	store storage.Storage
}

func NewHandler(svc *Service, store storage.Storage) *Handler {
	return &Handler{svc: svc, store: store}
}

// RegisterRoutes 注册稿件域路由（C 端）。
func (h *Handler) RegisterRoutes(v1 *gin.RouterGroup, auth, optionalAuth gin.HandlerFunc) {
	v1.GET("/categories", h.categories)

	g := v1.Group("/videos")
	{
		g.GET("", h.publicList)
		g.GET("/mine", auth, h.mine)       // 需在 /:bvid 之前注册同级静态路由
		g.GET("/history", auth, h.history) // 观看历史，同样需在 /:bvid 之前
		// 公开读接口挂 optionalAuth（M4-VIP-02）：不拦截游客，但登录时把 uid 注入，
		// 供「按会员状态过滤清晰度档位」使用（未登录按免费处理）。前端无法绕过该判定。
		g.GET("/:bvid", optionalAuth, h.publicDetail)
		g.GET("/:bvid/parts", optionalAuth, h.parts)
		g.POST("", auth, h.submit)
		g.POST("/cover", auth, h.uploadCover)
		g.POST("/:bvid/view", optionalAuth, h.addView) // 游客也计播放
		g.GET("/:bvid/progress", auth, h.getProgress)
		g.POST("/:bvid/progress", auth, h.saveProgress)
		g.DELETE("/:bvid", auth, h.remove)
	}
}

// RegisterAdminRoutes 注册清晰度档位配置（管理端，M4-VIP-02 spec §8.6）。
//
// 单独一个方法而不是塞进 RegisterRoutes：管理端要 AdminAuth + 权限点，
// 而权限检查器（RequirePerm(adminSvc.HasPerm, ...)）在 router 里的构建时机晚于稿件域装配，
// 故由装配方在 adminSvc 就绪后再调（与 pay/vip 的 RegisterAdminRoutes 同口径）。
func (h *Handler) RegisterAdminRoutes(g *gin.RouterGroup, perm func(string) gin.HandlerFunc) {
	// 查询与改配置分两个权限点：能看到档位不等于能改「谁能看多清晰」。
	g.GET("/quality-tiers", perm("video:view"), h.adminTiers)
	g.PUT("/quality-tiers/:quality", perm("video:tier"), h.adminUpdateTier)
	// 转码队列积压快照（转码积压治理卡）：只读，沿用 video:view。
	// 只出接口，管理端页面由另一张卡负责；同时刷新 Prometheus 指标（见 QueueSnapshot）。
	g.GET("/transcode/queue", perm("video:view"), h.adminTranscodeQueue)
}

// @Summary  转码队列积压快照
// @Tags     视频
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} response.Body
// @Router   /admin/videos/transcode/queue [get]
func (h *Handler) adminTranscodeQueue(c *gin.Context) {
	snap, err := h.svc.QueueSnapshot()
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, snap)
}

// @Summary  分区列表
// @Tags     视频
// @Produce  json
// @Success  200 {object} response.Body
// @Router   /categories [get]
func (h *Handler) categories(c *gin.Context) {
	list, err := h.svc.Categories(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

// @Summary  投稿（登记稿件）
// @Tags     视频
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    body body SubmitReq true "投稿信息"
// @Success  200 {object} response.Body
// @Router   /videos [post]
func (h *Handler) submit(c *gin.Context) {
	var req SubmitReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	uid := c.GetInt64(middleware.CtxUserID)
	detail, err := h.svc.Submit(c.Request.Context(), uid, &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, detail)
}

// @Summary  上传封面
// @Tags     视频
// @Accept   multipart/form-data
// @Produce  json
// @Security BearerAuth
// @Param    file formData file true "封面图片"
// @Success  200 {object} response.Body
// @Router   /videos/cover [post]
func (h *Handler) uploadCover(c *gin.Context) {
	fh, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, errcode.ErrInvalidParams.WithMsg("请选择封面文件"))
		return
	}
	uid := c.GetInt64(middleware.CtxUserID)
	url, err := h.svc.UploadCover(c.Request.Context(), uid, fh, h.store)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"cover": url})
}

// @Summary  公开视频列表（分页/分区/排序）
// @Tags     视频
// @Produce  json
// @Param    category_id query int false "分区ID"
// @Param    sort query string false "排序 new|hot"
// @Param    page query int false "页码"
// @Param    page_size query int false "每页数"
// @Success  200 {object} response.Body
// @Router   /videos [get]
func (h *Handler) publicList(c *gin.Context) {
	categoryID, _ := strconv.Atoi(c.DefaultQuery("category_id", "0"))
	uid, _ := strconv.ParseInt(c.DefaultQuery("uid", "0"), 10, 64)
	sort := c.DefaultQuery("sort", "new")
	page, size := pagination(c)

	cards, err := h.svc.PublicList(c.Request.Context(), categoryID, uid, sort, page, size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"list": cards})
}

// @Summary  稿件分P列表（含各 P 播放流）
// @Tags     视频
// @Produce  json
// @Param    bvid path string true "视频 BV 号"
// @Success  200 {object} response.Body
// @Router   /videos/{bvid}/parts [get]
func (h *Handler) parts(c *gin.Context) {
	ctx := WithViewer(c.Request.Context(), c.GetInt64(middleware.CtxUserID))
	items, err := h.svc.Parts(ctx, c.Param("bvid"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"list": items})
}

// @Summary  清晰度档位配置列表（管理端）
// @Tags     视频
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} response.Body
// @Router   /admin/videos/quality-tiers [get]
func (h *Handler) adminTiers(c *gin.Context) {
	response.OK(c, gin.H{"list": h.svc.AdminTiers()})
}

// @Summary  更新清晰度档位配置（管理端：开关/码率/会员限定/排序）
// @Tags     视频
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    quality path int true "档位编号"
// @Success  200 {object} response.Body
// @Router   /admin/videos/quality-tiers/{quality} [put]
func (h *Handler) adminUpdateTier(c *gin.Context) {
	q, err := strconv.ParseInt(c.Param("quality"), 10, 16)
	if err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	var req TierUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	view, err := h.svc.AdminUpdateTier(int16(q), &req, c.GetInt64(middleware.CtxAdminID))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, view)
}

// @Summary  视频详情（含播放地址/流）
// @Tags     视频
// @Produce  json
// @Param    bvid path string true "视频 BV 号"
// @Success  200 {object} response.Body
// @Router   /videos/{bvid} [get]
func (h *Handler) publicDetail(c *gin.Context) {
	ctx := WithViewer(c.Request.Context(), c.GetInt64(middleware.CtxUserID))
	detail, err := h.svc.PublicDetail(ctx, c.Param("bvid"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, detail)
}

// @Summary  我的稿件列表
// @Tags     视频
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} response.Body
// @Router   /videos/mine [get]
func (h *Handler) mine(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	page, size := pagination(c)
	cards, total, err := h.svc.Mine(c.Request.Context(), uid, page, size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"list": cards, "total": total})
}

// history 观看历史（最近观看倒序）
// @Summary  观看历史
// @Tags     视频
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} response.Body
// @Router   /videos/history [get]
func (h *Handler) history(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	page, size := pagination(c)
	cards, err := h.svc.History(c.Request.Context(), uid, page, size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"list": cards})
}

// @Summary  上报播放（游客也计数）
// @Tags     视频
// @Produce  json
// @Param    bvid path string true "视频 BV 号"
// @Success  200 {object} response.Body
// @Router   /videos/{bvid}/view [post]
func (h *Handler) addView(c *gin.Context) {
	// 登录用户按 UID 去重，游客按 IP 去重
	viewer := c.ClientIP()
	if uid := c.GetInt64(middleware.CtxUserID); uid > 0 {
		viewer = fmt.Sprintf("u%d", uid)
	}
	if err := h.svc.AddView(c.Request.Context(), c.Param("bvid"), viewer); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

// @Summary  获取播放进度
// @Tags     视频
// @Produce  json
// @Security BearerAuth
// @Param    bvid path string true "视频 BV 号"
// @Success  200 {object} response.Body
// @Router   /videos/{bvid}/progress [get]
func (h *Handler) getProgress(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	pos, err := h.svc.GetProgress(c.Request.Context(), uid, c.Param("bvid"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"position": pos})
}

// @Summary  保存播放进度
// @Tags     视频
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    bvid path string true "视频 BV 号"
// @Success  200 {object} response.Body
// @Router   /videos/{bvid}/progress [post]
func (h *Handler) saveProgress(c *gin.Context) {
	var req struct {
		Position int `json:"position" binding:"min=0"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	uid := c.GetInt64(middleware.CtxUserID)
	if err := h.svc.SaveProgress(c.Request.Context(), uid, c.Param("bvid"), req.Position); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

// @Summary  删除稿件
// @Tags     视频
// @Produce  json
// @Security BearerAuth
// @Param    bvid path string true "视频 BV 号"
// @Success  200 {object} response.Body
// @Router   /videos/{bvid} [delete]
func (h *Handler) remove(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	if err := h.svc.Delete(c.Request.Context(), uid, c.Param("bvid")); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
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
	return
}
