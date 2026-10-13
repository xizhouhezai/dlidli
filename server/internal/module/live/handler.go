package live

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

// RegisterRoutes 注册直播域路由（M4-LIV-02）。
func (h *Handler) RegisterRoutes(v1 *gin.RouterGroup, auth, optionalAuth gin.HandlerFunc) {
	g := v1.Group("/live")
	{
		g.POST("/apply", auth, h.apply)
		g.POST("/room/start", auth, h.start)
		g.POST("/room/stop", auth, h.stop)
		g.PUT("/room", auth, h.updateRoom)
		g.POST("/room/stream-key/reset", auth, h.resetStreamKey)
		g.GET("/room/mine", auth, h.myRoom)

		g.GET("/rooms", h.rooms)                        // 公开：直播中列表
		g.GET("/rooms/:id", optionalAuth, h.roomDetail) // 详情（未开播仅主播可见）

		g.POST("/rooms/:id/enter", auth, h.enter)
		g.POST("/rooms/:id/heartbeat", auth, h.heartbeat)
		g.POST("/rooms/:id/leave", auth, h.leave)

		g.POST("/rooms/:id/admins/:uid", auth, h.addAdmin)
		g.DELETE("/rooms/:id/admins/:uid", auth, h.removeAdmin)
		g.POST("/rooms/:id/mutes/:uid", auth, h.mute)
		g.DELETE("/rooms/:id/mutes/:uid", auth, h.unmute)
	}
}

// @Summary  申请开播（LIV-01/LIV-02）
// @Tags     直播
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} response.Body{data=live.ApplyResult}
// @Failure  200 {object} response.Body "91001 未入白名单 / 91002 未实名 / 91003 粉丝不足 / 91004 已有房间"
// @Router   /live/apply [post]
func (h *Handler) apply(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	res, err := h.svc.Apply(c.Request.Context(), uid)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// @Summary  开播
// @Tags     直播
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} response.Body{data=live.RoomInfo}
// @Failure  200 {object} response.Body "91005 房间已开播 / 91007 房间不存在"
// @Router   /live/room/start [post]
func (h *Handler) start(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	res, err := h.svc.Start(c.Request.Context(), uid)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// @Summary  下播
// @Tags     直播
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} response.Body{data=live.RoomInfo}
// @Failure  200 {object} response.Body "91006 房间未在直播 / 91007 房间不存在"
// @Router   /live/room/stop [post]
func (h *Handler) stop(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	res, err := h.svc.Stop(c.Request.Context(), uid)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

type updateRoomReq struct {
	Title      string `json:"title" binding:"required"`
	Cover      string `json:"cover"`
	CategoryID int64  `json:"category_id" binding:"required"`
}

// @Summary  修改房间信息（标题/封面/分区）
// @Tags     直播
// @Produce  json
// @Security BearerAuth
// @Param    body body updateRoomReq true "房间信息"
// @Success  200 {object} response.Body{data=live.RoomInfo}
// @Failure  200 {object} response.Body "91012 分区不存在 / 91014 标题非法 / 91007 房间不存在"
// @Router   /live/room [put]
func (h *Handler) updateRoom(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	var req updateRoomReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	res, err := h.svc.UpdateRoom(c.Request.Context(), uid, req.Title, req.Cover, req.CategoryID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// @Summary  重置推流密钥（旧密钥立即失效；明文仅此一次返回）
// @Tags     直播
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} response.Body{data=live.ApplyResult}
// @Failure  200 {object} response.Body "91007 房间不存在"
// @Router   /live/room/stream-key/reset [post]
func (h *Handler) resetStreamKey(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	res, err := h.svc.ResetStreamKey(c.Request.Context(), uid)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// @Summary  我的房间（含推流地址；密钥不回传）
// @Tags     直播
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} response.Body{data=live.ApplyResult}
// @Failure  200 {object} response.Body "91007 房间不存在"
// @Router   /live/room/mine [get]
func (h *Handler) myRoom(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	res, err := h.svc.MyRoom(c.Request.Context(), uid)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// @Summary  直播中房间列表（公开）
// @Tags     直播
// @Produce  json
// @Param    category_id query int false "分区过滤"
// @Param    page query int false "页码，默认 1"
// @Param    size query int false "每页，默认 20，上限 50"
// @Success  200 {object} response.Body{data=live.RoomListResult}
// @Router   /live/rooms [get]
func (h *Handler) rooms(c *gin.Context) {
	categoryID, _ := strconv.ParseInt(c.Query("category_id"), 10, 64)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	list, total, err := h.svc.ListLive(c.Request.Context(), categoryID, page, size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, RoomListResult{List: list, Total: total})
}

// @Summary  房间详情（未开播房间仅主播本人可见）
// @Tags     直播
// @Produce  json
// @Param    id path int true "房间ID"
// @Success  200 {object} response.Body{data=live.RoomInfo}
// @Failure  200 {object} response.Body "91007 房间不存在"
// @Router   /live/rooms/{id} [get]
func (h *Handler) roomDetail(c *gin.Context) {
	viewer := c.GetInt64(middleware.CtxUserID)
	id, ok := pathID(c)
	if !ok {
		return
	}
	res, err := h.svc.RoomDetail(c.Request.Context(), viewer, id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// @Summary  进房（在线计数 +1；重复进房不重复计数）
// @Tags     直播
// @Produce  json
// @Security BearerAuth
// @Param    id path int true "房间ID"
// @Success  200 {object} response.Body{data=live.RoomInfo}
// @Failure  200 {object} response.Body "91006 未开播 / 91007 房间不存在"
// @Router   /live/rooms/{id}/enter [post]
func (h *Handler) enter(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	id, ok := pathID(c)
	if !ok {
		return
	}
	res, err := h.svc.Enter(c.Request.Context(), uid, id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// @Summary  心跳（≤15s 一次；30s 无心跳判离线）
// @Tags     直播
// @Produce  json
// @Security BearerAuth
// @Param    id path int true "房间ID"
// @Success  200 {object} response.Body
// @Router   /live/rooms/{id}/heartbeat [post]
func (h *Handler) heartbeat(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.svc.Heartbeat(c.Request.Context(), uid, id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{})
}

// @Summary  离房（在线计数 −1，幂等）
// @Tags     直播
// @Produce  json
// @Security BearerAuth
// @Param    id path int true "房间ID"
// @Success  200 {object} response.Body{data=live.RoomInfo}
// @Router   /live/rooms/{id}/leave [post]
func (h *Handler) leave(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	id, ok := pathID(c)
	if !ok {
		return
	}
	res, err := h.svc.Leave(c.Request.Context(), uid, id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// @Summary  任命房管（仅主播）
// @Tags     直播
// @Produce  json
// @Security BearerAuth
// @Param    id  path int true "房间ID"
// @Param    uid path int true "目标用户ID"
// @Success  200 {object} response.Body
// @Failure  200 {object} response.Body "91008 非主播 / 91010 不能对自己 / 10005 用户不存在"
// @Router   /live/rooms/{id}/admins/{uid} [post]
func (h *Handler) addAdmin(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	id, ok := pathID(c)
	if !ok {
		return
	}
	target, ok := pathUID(c)
	if !ok {
		return
	}
	if err := h.svc.AddAdmin(c.Request.Context(), uid, id, target); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"added": true})
}

// @Summary  撤销房管（仅主播）
// @Tags     直播
// @Produce  json
// @Security BearerAuth
// @Param    id  path int true "房间ID"
// @Param    uid path int true "房管用户ID"
// @Success  200 {object} response.Body
// @Failure  200 {object} response.Body "91008 非主播 / 91007 房间不存在"
// @Router   /live/rooms/{id}/admins/{uid} [delete]
func (h *Handler) removeAdmin(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	id, ok := pathID(c)
	if !ok {
		return
	}
	target, ok := pathUID(c)
	if !ok {
		return
	}
	if err := h.svc.RemoveAdmin(c.Request.Context(), uid, id, target); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"removed": true})
}

type muteReq struct {
	Minutes int64  `json:"minutes" binding:"required"` // 禁言分钟数，1 ~ 43200
	Reason  string `json:"reason"`
}

// @Summary  禁言用户（主播/房管；时长 1min-720h）
// @Tags     直播
// @Produce  json
// @Security BearerAuth
// @Param    id   path int      true "房间ID"
// @Param    uid  path int      true "目标用户ID"
// @Param    body body muteReq true "禁言参数"
// @Success  200 {object} response.Body
// @Failure  200 {object} response.Body "91009 非主播/房管 / 91013 时长非法 / 91010 不能禁言主播"
// @Router   /live/rooms/{id}/mutes/{uid} [post]
func (h *Handler) mute(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	id, ok := pathID(c)
	if !ok {
		return
	}
	target, ok := pathUID(c)
	if !ok {
		return
	}
	var req muteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	if err := h.svc.Mute(c.Request.Context(), uid, id, target, req.Minutes, req.Reason); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"muted": true})
}

// @Summary  解除禁言（主播/房管）
// @Tags     直播
// @Produce  json
// @Security BearerAuth
// @Param    id  path int true "房间ID"
// @Param    uid path int true "目标用户ID"
// @Success  200 {object} response.Body
// @Failure  200 {object} response.Body "91009 非主播/房管"
// @Router   /live/rooms/{id}/mutes/{uid} [delete]
func (h *Handler) unmute(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	id, ok := pathID(c)
	if !ok {
		return
	}
	target, ok := pathUID(c)
	if !ok {
		return
	}
	if err := h.svc.Unmute(c.Request.Context(), uid, id, target); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"unmuted": true})
}

// pathID 解析路径房间 ID。
func pathID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Fail(c, errcode.ErrInvalidParams)
		return 0, false
	}
	return id, true
}

// pathUID 解析路径用户 ID。
func pathUID(c *gin.Context) (int64, bool) {
	uid, err := strconv.ParseInt(c.Param("uid"), 10, 64)
	if err != nil || uid <= 0 {
		response.Fail(c, errcode.ErrInvalidParams)
		return 0, false
	}
	return uid, true
}
