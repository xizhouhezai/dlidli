package live

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/dlidli/server/internal/middleware"
	"github.com/dlidli/server/internal/pkg/errcode"
	"github.com/dlidli/server/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// callbackTokenHeader 回调 token 的备用传递头（SRS http_hooks 默认走查询串）。
const callbackTokenHeader = "X-Live-Callback-Token"

type Handler struct {
	svc *Service
	hub *Hub
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// WithHub 注入直播弹幕 Hub（M4-LIV-03；router 装配时调用）。
func (h *Handler) WithHub(hub *Hub) *Handler {
	h.hub = hub
	return h
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

		// SRS http_hooks 回调（M4-LIV-01）：无登录态，鉴权靠共享 token（见 CheckCallbackToken），
		// 失败一律非 2xx → SRS 拒绝推流。
		g.POST("/stream/auth", h.streamAuth)
		g.POST("/stream/stop", h.streamStop)

		g.POST("/rooms/:id/enter", auth, h.enter)
		g.POST("/rooms/:id/heartbeat", auth, h.heartbeat)
		g.POST("/rooms/:id/leave", auth, h.leave)

		g.GET("/rooms/:id/admins", auth, h.adminList)
		g.POST("/rooms/:id/admins/:uid", auth, h.addAdmin)
		g.DELETE("/rooms/:id/admins/:uid", auth, h.removeAdmin)
		g.GET("/rooms/:id/mutes", auth, h.muteList)
		g.POST("/rooms/:id/mutes/:uid", auth, h.mute)
		g.DELETE("/rooms/:id/mutes/:uid", auth, h.unmute)

		// 直播弹幕通道（M4-LIV-03）：WS 建连即进房（token 经 query 携带），
		// 历史拉取与弹幕发送不再走 HTTP（上行在 WS 内，见 hub readLoop）。
		g.GET("/rooms/:id/ws", auth, h.ws)
		g.GET("/rooms/:id/danmaku/history", auth, h.history)

		// 直播回放（M4-LIV-14，spec LIV-14）：主播可选发布自己的回放稿件。
		// 未发布（草稿）时稿件对他人不可见 —— 可见性由 video.status 保证
		// （草稿不进公开列表/搜索），这三个接口只是「发布/删除」的动作入口。
		g.GET("/replays/mine", auth, h.myReplays)
		g.POST("/replays/:videoId/publish", auth, h.publishReplay)
		g.DELETE("/replays/:videoId", auth, h.deleteReplay)
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

// @Summary  房管列表（登录即可看）
// @Tags     直播
// @Produce  json
// @Security BearerAuth
// @Param    id path int true "房间ID"
// @Success  200 {object} response.Body{data=live.AdminListResult}
// @Failure  200 {object} response.Body "91007 房间不存在"
// @Router   /live/rooms/{id}/admins [get]
func (h *Handler) adminList(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	res, err := h.svc.ListAdmins(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
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

// @Summary  禁言列表（登录即可看；muted=false 为已过期记录）
// @Tags     直播
// @Produce  json
// @Security BearerAuth
// @Param    id path int true "房间ID"
// @Success  200 {object} response.Body{data=live.MuteListResult}
// @Failure  200 {object} response.Body "91007 房间不存在"
// @Router   /live/rooms/{id}/mutes [get]
func (h *Handler) muteList(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	res, err := h.svc.ListMutes(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
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

// @Summary  直播弹幕实时连接（WebSocket，M4-LIV-03）
// @Tags     直播
// @Security BearerAuth
// @Param    id path int true "房间ID"
// @Success  101 {string} string "Switching Protocols"
// @Failure  200 {object} response.Body "91006 未开播 / 91007 房间不存在"
// @Router   /live/rooms/{id}/ws [get]
// @Description 客户端上行帧（JSON）: {"type":"heartbeat"} 心跳（≤15s 一次，40s 无上行断开）；
// @Description {"type":"danmaku","content":"文本"} 发弹幕（服务端频控/禁言/机审，失败经 admin 侧日志，
// @Description 拒绝不下发帧）。服务端下行帧: {"type":"enter",...} 进场 / {"type":"danmaku",...} 弹幕 / {"type":"pong",...} 心跳应答。
func (h *Handler) ws(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	id, ok := pathID(c)
	if !ok {
		return
	}
	room, nickname, avatar, info, err := h.svc.Join(c.Request.Context(), uid, id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	// WS 建连即进房（连接生命周期即在线状态）：计数 + 进场消息广播，随后升级连接
	counted := h.svc.online.EnterForce(context.Background(), room.ID, uid)
	h.hub.BroadcastEnter(room.ID, info)
	h.hub.ServeCounted(c, room.ID, uid, nickname, avatar, counted)
}

// @Summary  直播弹幕历史（进房回放最近 50 条）
// @Tags     直播
// @Produce  json
// @Security BearerAuth
// @Param    id path int true "房间ID"
// @Success  200 {object} response.Body{data=live.HistoryResult}
// @Failure  200 {object} response.Body "91006 未开播 / 91007 房间不存在"
// @Router   /live/rooms/{id}/danmaku/history [get]
func (h *Handler) history(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	res, err := h.svc.History(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
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

// ---- SRS http_hooks 回调（M4-LIV-01）----

// @Summary  SRS on_publish 回调（推流鉴权）
// @Tags     直播
// @Produce  json
// @Param    token query string false "回调共享 token（亦可用 X-Live-Callback-Token 头）"
// @Param    body  body  live.PublishAuthReq true "SRS 回调体（action/app/stream/param 等）"
// @Success  200 {object} response.Body{data=object} "code=0 放行推流"
// @Failure  403 {object} response.Body "91024 token 无效 / 91025 密钥错误 / 91006 未开播 / 91007 房间不存在 / 91026 流名不合法"
// @Router   /live/stream/auth [post]
// @Description 流名 room_<id>（公开、不含密钥），密钥在推流 URL 查询串 → SRS 放入 body.param；
// @Description 非 2xx 即 SRS 拒绝推流。
func (h *Handler) streamAuth(c *gin.Context) {
	if !h.svc.CheckCallbackToken(c.Query("token"), c.GetHeader(callbackTokenHeader)) {
		failCallback(c, errcode.ErrLiveCallbackTokenInvalid)
		return
	}
	var req PublishAuthReq
	if err := c.ShouldBindJSON(&req); err != nil {
		failCallback(c, errcode.ErrLiveStreamFormatInvalid)
		return
	}
	roomID, err := h.svc.AuthorizePublish(c.Request.Context(), &req)
	if err != nil {
		failCallback(c, err)
		return
	}
	response.OK(c, gin.H{"room_id": roomID})
}

// @Summary  SRS on_unpublish 回调（下播联动，幂等）
// @Tags     直播
// @Produce  json
// @Param    token query string false "回调共享 token（亦可用 X-Live-Callback-Token 头）"
// @Param    body  body  live.PublishAuthReq true "SRS 回调体（action/app/stream/param 等）"
// @Success  200 {object} response.Body{data=object} "stopped=true 本场已置下播；false 无动作（重复回调/未知流）"
// @Failure  403 {object} response.Body "91024 token 无效"
// @Router   /live/stream/stop [post]
// @Description 房间处于直播中(1) 时复用主播下播路径（status=2 + ended_at + 在线清零 + 清弹幕历史）；
// @Description 重复回调或未知流返回 200 且不改变状态。
func (h *Handler) streamStop(c *gin.Context) {
	if !h.svc.CheckCallbackToken(c.Query("token"), c.GetHeader(callbackTokenHeader)) {
		failCallback(c, errcode.ErrLiveCallbackTokenInvalid)
		return
	}
	var req PublishAuthReq
	if err := c.ShouldBindJSON(&req); err != nil {
		failCallback(c, errcode.ErrLiveStreamFormatInvalid)
		return
	}
	stopped, err := h.svc.StopByStream(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, errcode.ErrLiveStreamFormatInvalid) {
			// 不是本服务的房间流：无可下播对象，按幂等返回 200（避免 SRS 重试风暴）
			h.svc.logger().Warn("on_unpublish 流名不合法，忽略", zap.String("stream", req.Stream))
			response.OK(c, gin.H{"stopped": false})
			return
		}
		failCallback(c, err)
		return
	}
	if !stopped {
		h.svc.logger().Info("on_unpublish 无下播动作（重复回调或未知房间）", zap.String("stream", req.Stream))
	}
	response.OK(c, gin.H{"stopped": stopped})
}

// failCallback 回调失败响应：SRS 以 HTTP 状态码判定是否拒推，必须非 2xx，
// 故不能走 response.Fail（业务错误统一 200 + code）。业务错误用 403，其余用 500。
func failCallback(c *gin.Context, err error) {
	code, msg := errcode.ErrInternal.Code, errcode.ErrInternal.Msg
	status := http.StatusForbidden
	var biz *errcode.Error
	if errors.As(err, &biz) {
		code, msg = biz.Code, biz.Msg
	} else {
		status = http.StatusInternalServerError
	}
	_ = c.Error(err)
	c.JSON(status, response.Body{Code: code, Message: msg, Data: nil, TraceID: c.GetString(response.CtxTraceID)})
}

// ---- 直播回放（M4-LIV-14）----

// @Summary  我的回放列表（含草稿，供主播选择发布）
// @Tags     直播
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} response.Body{data=[]live.ReplayItem}
// @Failure  200 {object} response.Body "91031 回放未启用 / 91007 房间不存在"
// @Router   /live/replays/mine [get]
func (h *Handler) myReplays(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	list, err := h.svc.MyReplays(c.Request.Context(), uid, 50)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

// @Summary  发布我的回放（草稿 → 已发布，spec LIV-14「主播可选发布」）
// @Tags     直播
// @Produce  json
// @Security BearerAuth
// @Param    videoId path int true "回放稿件 ID"
// @Success  200 {object} response.Body
// @Failure  200 {object} response.Body "91031 未启用 / 91032 回放不存在或不属于您"
// @Router   /live/replays/{videoId}/publish [post]
func (h *Handler) publishReplay(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	videoID, err := strconv.ParseInt(c.Param("videoId"), 10, 64)
	if err != nil || videoID <= 0 {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	if err := h.svc.PublishReplay(c.Request.Context(), uid, videoID); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"published": true, "video_id": videoID})
}

// @Summary  删除我的回放（软删；录制登记保留以维追溯）
// @Tags     直播
// @Produce  json
// @Security BearerAuth
// @Param    videoId path int true "回放稿件 ID"
// @Success  200 {object} response.Body
// @Failure  200 {object} response.Body "91031 未启用 / 91032 回放不存在或不属于您"
// @Router   /live/replays/{videoId} [delete]
func (h *Handler) deleteReplay(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	videoID, err := strconv.ParseInt(c.Param("videoId"), 10, 64)
	if err != nil || videoID <= 0 {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	if err := h.svc.DeleteReplay(c.Request.Context(), uid, videoID); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"deleted": true, "video_id": videoID})
}
