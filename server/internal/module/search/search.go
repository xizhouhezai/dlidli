// Package search 搜索域：视频标题 / UP 主昵称检索。
// MVP 走 MySQL LIKE；SRH 后续任务切 Elasticsearch（分词/高亮/相关度）。
package search

import (
	"strconv"
	"strings"

	"github.com/dlidli/server/internal/module/account"
	"github.com/dlidli/server/internal/module/video"
	"github.com/dlidli/server/internal/pkg/errcode"
	"github.com/dlidli/server/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	videoSvc   *video.Service
	accountSvc *account.Service
}

func NewHandler(videoSvc *video.Service, accountSvc *account.Service) *Handler {
	return &Handler{videoSvc: videoSvc, accountSvc: accountSvc}
}

// RegisterRoutes 注册搜索路由（公开接口）。
func (h *Handler) RegisterRoutes(v1 *gin.RouterGroup) {
	v1.GET("/search", h.search)
}

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
		list, total, err := h.videoSvc.Search(c.Request.Context(), keyword, page, size)
		if err != nil {
			response.Fail(c, err)
			return
		}
		response.OK(c, gin.H{"list": list, "total": total})
	}
}
