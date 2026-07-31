package interaction

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

// RegisterRoutes 注册互动域路由。
func (h *Handler) RegisterRoutes(v1 *gin.RouterGroup, auth, optionalAuth gin.HandlerFunc) {
	videos := v1.Group("/videos/:bvid")
	{
		videos.GET("/comments", optionalAuth, h.listComments)
		videos.POST("/comments", auth, h.addComment)
		videos.GET("/like", optionalAuth, h.videoLiked)
		videos.POST("/like", auth, h.toggleVideoLike)
		videos.GET("/interaction", optionalAuth, h.interactionState)
		videos.POST("/coin", auth, h.coinVideo)
		videos.POST("/favorite", auth, h.toggleFavorite)
		videos.POST("/triple", auth, h.triple)
	}

	v1.GET("/users/me/favorites", auth, h.favorites)
	v1.GET("/users/me/collections", auth, h.listCollections)
	v1.POST("/users/me/collections", auth, h.createCollection)
	v1.PUT("/users/me/collections/:id", auth, h.renameCollection)
	v1.DELETE("/users/me/collections/:id", auth, h.deleteCollection)

	comments := v1.Group("/comments/:id")
	{
		comments.GET("/replies", optionalAuth, h.listReplies)
		comments.POST("/like", auth, h.toggleCommentLike)
		comments.DELETE("", auth, h.deleteComment)
	}
}

func (h *Handler) addComment(c *gin.Context) {
	var req AddCommentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	uid := c.GetInt64(middleware.CtxUserID)
	item, err := h.svc.AddComment(c.Request.Context(), uid, c.Param("bvid"), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, item)
}

func (h *Handler) listComments(c *gin.Context) {
	sort := c.DefaultQuery("sort", "hot")
	page, size := pagination(c)
	uid := c.GetInt64(middleware.CtxUserID)

	items, total, err := h.svc.ListComments(c.Request.Context(), uid, c.Param("bvid"), sort, page, size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"list": items, "total": total})
}

func (h *Handler) listReplies(c *gin.Context) {
	rootID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	page, size := pagination(c)
	uid := c.GetInt64(middleware.CtxUserID)

	items, total, err := h.svc.ListReplies(c.Request.Context(), uid, rootID, page, size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"list": items, "total": total})
}

func (h *Handler) deleteComment(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	uid := c.GetInt64(middleware.CtxUserID)
	if err := h.svc.DeleteComment(c.Request.Context(), uid, id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

func (h *Handler) toggleVideoLike(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	liked, err := h.svc.ToggleVideoLike(c.Request.Context(), uid, c.Param("bvid"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"liked": liked})
}

func (h *Handler) videoLiked(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	liked, err := h.svc.VideoLiked(c.Request.Context(), uid, c.Param("bvid"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"liked": liked})
}

func (h *Handler) toggleCommentLike(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	uid := c.GetInt64(middleware.CtxUserID)
	liked, err := h.svc.ToggleCommentLike(c.Request.Context(), uid, id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"liked": liked})
}

func (h *Handler) interactionState(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	st, err := h.svc.InteractionState(c.Request.Context(), uid, c.Param("bvid"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, st)
}

func (h *Handler) coinVideo(c *gin.Context) {
	var req struct {
		Count int `json:"count" binding:"required,min=1,max=2"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	uid := c.GetInt64(middleware.CtxUserID)
	if err := h.svc.CoinVideo(c.Request.Context(), uid, c.Param("bvid"), req.Count); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

func (h *Handler) toggleFavorite(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	var req struct {
		// 雪花 ID 用字符串传输，避免 JS Number 精度丢失
		CollectionID string `json:"collection_id"`
	}
	_ = c.ShouldBindJSON(&req) // optional body
	colID, _ := strconv.ParseInt(req.CollectionID, 10, 64)
	faved, err := h.svc.ToggleFavorite(c.Request.Context(), uid, c.Param("bvid"), colID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"faved": faved})
}

func (h *Handler) triple(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	res, err := h.svc.Triple(c.Request.Context(), uid, c.Param("bvid"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

func (h *Handler) favorites(c *gin.Context) {
	page, size := pagination(c)
	uid := c.GetInt64(middleware.CtxUserID)
	cards, total, err := h.svc.Favorites(c.Request.Context(), uid, page, size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"list": cards, "total": total})
}

func (h *Handler) listCollections(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	list, err := h.svc.ListCollections(c.Request.Context(), uid)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

func (h *Handler) createCollection(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	uid := c.GetInt64(middleware.CtxUserID)
	col, err := h.svc.CreateCollection(c.Request.Context(), uid, req.Name)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, col)
}

func (h *Handler) renameCollection(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	uid := c.GetInt64(middleware.CtxUserID)
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.svc.RenameCollection(c.Request.Context(), uid, id, req.Name); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

func (h *Handler) deleteCollection(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.svc.DeleteCollection(c.Request.Context(), uid, id); err != nil {
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
