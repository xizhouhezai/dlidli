// Package search 搜索域：视频标题 / UP 主昵称检索。
// MVP 走 MySQL LIKE；配置 Elasticsearch 后优先走 ES（分词/相关度），
// ES 异常自动降级 LIKE（M2-SRH-01/02 平滑切换）。
package search

import (
	"context"
	"strconv"
	"strings"

	"github.com/dlidli/server/internal/module/account"
	"github.com/dlidli/server/internal/module/video"
	"github.com/dlidli/server/internal/pkg/errcode"
	"github.com/dlidli/server/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// Reader 检索索引读取侧（searchindex.Service 实现；未启用时可为 nil）。
type Reader interface {
	Enabled() bool
	SearchVideos(ctx context.Context, keyword string, page, size int) ([]int64, int64, error)
}

type Handler struct {
	videoSvc   *video.Service
	accountSvc *account.Service
	index      Reader // 可为 nil → 恒走 MySQL LIKE
}

func NewHandler(videoSvc *video.Service, accountSvc *account.Service) *Handler {
	return &Handler{videoSvc: videoSvc, accountSvc: accountSvc}
}

// SetIndex 注入检索索引（router 装配时调用；nil 表示禁用）。
func (h *Handler) SetIndex(idx Reader) { h.index = idx }

// searchVideos 视频搜索：ES 优先，失败/未启用降级 MySQL LIKE。
func (h *Handler) searchVideos(ctx context.Context, keyword string, page, size int) (any, error) {
	if h.index != nil && h.index.Enabled() {
		ids, total, err := h.index.SearchVideos(ctx, keyword, page, size)
		if err == nil {
			cards, err2 := h.videoSvc.CardsByIDs(ctx, ids)
			if err2 != nil {
				return nil, err2
			}
			return gin.H{"list": cards, "total": total}, nil
		}
		// ES 异常 → 降级 LIKE（不返回错误，保证搜索可用）
	}
	list, total, err := h.videoSvc.Search(ctx, keyword, page, size)
	if err != nil {
		return nil, err
	}
	return gin.H{"list": list, "total": total}, nil
}

// RegisterRoutes 注册搜索路由（公开接口）。
func (h *Handler) RegisterRoutes(v1 *gin.RouterGroup) {
	v1.GET("/search", h.search)
}

// @Summary  搜索（视频/用户）
// @Tags     搜索
// @Produce  json
// @Param    kw query string true "关键词"
// @Param    type query string false "类型 video|user"
// @Param    page query int false "页码"
// @Success  200 {object} response.Body
// @Router   /search [get]
func (h *Handler) search(c *gin.Context) {
	keyword := strings.TrimSpace(c.Query("keyword"))
	if keyword == "" || len([]rune(keyword)) > 50 {
		response.Fail(c, errcode.ErrInvalidParams.WithMsg("关键词需在 1~50 字之间"))
		return
	}
	searchType := c.DefaultQuery("type", "video")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 50 {
		size = 20
	}

	switch searchType {
	case "user":
		list, total, err := h.accountSvc.SearchUsers(c.Request.Context(), keyword, page, size)
		if err != nil {
			response.Fail(c, err)
			return
		}
		response.OK(c, gin.H{"list": list, "total": total})
	default: // video
		res, err := h.searchVideos(c.Request.Context(), keyword, page, size)
		if err != nil {
			response.Fail(c, err)
			return
		}
		response.OK(c, res)
	}
}
