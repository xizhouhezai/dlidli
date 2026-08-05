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

// RegisterRoutes 注册稿件域路由。
func (h *Handler) RegisterRoutes(v1 *gin.RouterGroup, auth, optionalAuth gin.HandlerFunc) {
	v1.GET("/categories", h.categories)

	g := v1.Group("/videos")
	{
		g.GET("", h.publicList)
		g.GET("/mine", auth, h.mine) // 需在 /:bvid 之前注册同级静态路由
		g.GET("/:bvid", h.publicDetail)
		g.GET("/:bvid/parts", h.parts)
		g.POST("", auth, h.submit)
		g.POST("/cover", auth, h.uploadCover)
		g.POST("/:bvid/view", optionalAuth, h.addView) // 游客也计播放
		g.GET("/:bvid/progress", auth, h.getProgress)
		g.POST("/:bvid/progress", auth, h.saveProgress)
		g.DELETE("/:bvid", auth, h.remove)
	}
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
	items, err := h.svc.Parts(c.Request.Context(), c.Param("bvid"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"list": items})
}

// @Summary  视频详情（含播放地址/流）
// @Tags     视频
// @Produce  json
// @Param    bvid path string true "视频 BV 号"
// @Success  200 {object} response.Body
// @Router   /videos/{bvid} [get]
func (h *Handler) publicDetail(c *gin.Context) {
	detail, err := h.svc.PublicDetail(c.Request.Context(), c.Param("bvid"))
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
