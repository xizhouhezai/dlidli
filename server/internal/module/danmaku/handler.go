package danmaku

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

// RegisterRoutes 注册弹幕域路由。
// optionalAuth 用于分段/全量拉取（登录态过滤屏蔽）；发送与屏蔽设置需登录。
func (h *Handler) RegisterRoutes(v1 *gin.RouterGroup, auth, optionalAuth gin.HandlerFunc) {
	g := v1.Group("/videos/:bvid/danmaku")
	{
		g.GET("", optionalAuth, h.list)
		g.GET("/list", optionalAuth, h.listAll)
		g.POST("", auth, h.send)
		g.GET("/ws", optionalAuth, h.ws)
		// 屏蔽设置（M2-DM-02）
		g.GET("/blocks", auth, h.listBlocks)
		g.POST("/blocks", auth, h.addBlock)
		g.DELETE("/blocks/:id", auth, h.deleteBlock)
	}
}

// @Summary  发送弹幕
// @Tags     弹幕
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    bvid path string true "视频 BV 号"
// @Param    body body object true "text; time; color; mode"
// @Success  200 {object} response.Body
// @Router   /videos/{bvid}/danmaku [post]
func (h *Handler) send(c *gin.Context) {
	var req SendReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	uid := c.GetInt64(middleware.CtxUserID)
	item, err := h.svc.Send(c.Request.Context(), uid, c.Param("bvid"), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, item)
}

// @Summary  弹幕分段列表（登录用户过滤屏蔽项）
// @Tags     弹幕
// @Produce  json
// @Param    bvid path string true "视频 BV 号"
// @Param    segment query int false "分段号（默认0）"
// @Success  200 {object} response.Body
// @Router   /videos/{bvid}/danmaku [get]
func (h *Handler) list(c *gin.Context) {
	seg, err := strconv.Atoi(c.DefaultQuery("segment", "0"))
	if err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	uid := c.GetInt64(middleware.CtxUserID)
	items, err := h.svc.ListSegment(c.Request.Context(), c.Param("bvid"), seg, uid)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"segment": seg, "segment_ms": SegmentMS, "list": items})
}

// @Summary  弹幕全量列表（列表面板，分页）
// @Tags     弹幕
// @Produce  json
// @Param    bvid path string true "视频 BV 号"
// @Param    page query int false "页码（默认1）"
// @Param    size query int false "每页条数（默认50）"
// @Success  200 {object} response.Body
// @Router   /videos/{bvid}/danmaku/list [get]
func (h *Handler) listAll(c *gin.Context) {
	page, size := pagination(c, 50)
	uid := c.GetInt64(middleware.CtxUserID)
	items, total, err := h.svc.ListAll(c.Request.Context(), c.Param("bvid"), uid, page, size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"list": items, "total": total})
}

// @Summary  弹幕屏蔽列表
// @Tags     弹幕-屏蔽
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} response.Body
// @Router   /videos/{bvid}/danmaku/blocks [get]
func (h *Handler) listBlocks(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	items, err := h.svc.ListBlocks(c.Request.Context(), uid)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"list": items})
}

// @Summary  新增屏蔽（关键词或用户）
// @Tags     弹幕-屏蔽
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    body body object true "block_type: 1关键词 2用户; keyword / target_uid"
// @Success  200 {object} response.Body
// @Router   /videos/{bvid}/danmaku/blocks [post]
func (h *Handler) addBlock(c *gin.Context) {
	var req struct {
		BlockType int8   `json:"block_type" binding:"required,oneof=1 2"`
		Keyword   string `json:"keyword"`
		TargetUID string `json:"target_uid"`
		BlockHash string `json:"block_hash"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	uid := c.GetInt64(middleware.CtxUserID)
	if err := h.svc.AddBlock(c.Request.Context(), uid, req.BlockType, req.Keyword, req.TargetUID, req.BlockHash); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

// @Summary  删除屏蔽项
// @Tags     弹幕-屏蔽
// @Produce  json
// @Security BearerAuth
// @Param    id path string true "屏蔽项ID"
// @Success  200 {object} response.Body
// @Router   /videos/{bvid}/danmaku/blocks/{id} [delete]
func (h *Handler) deleteBlock(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	uid := c.GetInt64(middleware.CtxUserID)
	if err := h.svc.DeleteBlock(c.Request.Context(), uid, id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

func pagination(c *gin.Context, defaultSize int) (page, size int) {
	page = 1
	size = defaultSize
	if p := c.Query("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if s := c.Query("size"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 && v <= 200 {
			size = v
		}
	}
	return page, size
}
