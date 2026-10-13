package live

import (
	"strconv"

	"github.com/dlidli/server/internal/middleware"
	"github.com/dlidli/server/internal/pkg/errcode"
	"github.com/dlidli/server/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// GiftHandler 礼物域路由（M4-LIV-04）：C 端礼物列表/余额/赠送 + 后台 dev 直充。
type GiftHandler struct {
	svc *GiftService
}

func NewGiftHandler(svc *GiftService) *GiftHandler { return &GiftHandler{svc: svc} }

// RegisterGiftRoutes C 端礼物路由（挂在 live 组内；礼物列表公开 optionalAuth，
// 余额/赠送需登录——登录校验在 handler 内按 CtxUserID 判定）。
func (h *GiftHandler) RegisterGiftRoutes(g *gin.RouterGroup) {
	g.GET("/gifts", h.giftList)
	g.GET("/melon/balance", authed(h.melonBalance))
	g.POST("/rooms/:id/gifts/:giftId", authed(h.sendGift))
}

// authed 简易登录校验：optionalAuth 组内要求已解出 uid（余额与赠送是资金接口，必须登录）。
func authed(h gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetInt64(middleware.CtxUserID) == 0 {
			response.Fail(c, errcode.ErrUnauthorized)
			return
		}
		h(c)
	}
}

// RegisterDevRoutes 开发直充（POST /admin/dev/recharge）：
// 挂 admin 中间件（AdminAuth + RequirePerm config:edit），
// 开发期临时通道——真实支付接入留 PAY-01，届时本接口下线（spec LIV-13 替换点）。
func (h *GiftHandler) RegisterDevRoutes(adminAuthed *gin.RouterGroup) {
	adminAuthed.POST("/dev/recharge", h.devRecharge)
}

type sendGiftReq struct {
	Count int64 `json:"count"`
}

// @Summary  礼物列表（上架）
// @Tags     直播礼物
// @Produce  json
// @Success  200 {object} response.Body{data=[]live.GiftInfo}
// @Router   /live/gifts [get]
func (h *GiftHandler) giftList(c *gin.Context) {
	list, err := h.svc.GiftList()
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

// @Summary  金瓜子余额
// @Tags     直播礼物
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} response.Body{data=live.MelonBalance}
// @Router   /live/melon/balance [get]
func (h *GiftHandler) melonBalance(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	res, err := h.svc.MelonBalance(uid)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// @Summary  赠送礼物（LIV-13 强一致：余额不足 91021 且不产生任何流水）
// @Tags     直播礼物
// @Produce  json
// @Security BearerAuth
// @Param    id     path int true "房间ID"
// @Param    giftId path int true "礼物ID"
// @Param    body   body live.sendGiftReq false "count 连击数量，默认 1（1-999）"
// @Success  200 {object} response.Body{data=live.SendGiftResult}
// @Failure  200 {object} response.Body "91007 房间不存在 / 91019 礼物不存在 / 91020 房间未开播 / 91021 余额不足 / 91022 数量不合法"
// @Router   /live/rooms/{id}/gifts/{giftId} [post]
func (h *GiftHandler) sendGift(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	roomID, ok := pathID(c)
	if !ok {
		return
	}
	giftID, err := strconv.ParseInt(c.Param("giftId"), 10, 64)
	if err != nil || giftID <= 0 {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	var req sendGiftReq
	_ = c.ShouldBindJSON(&req) // body 可省，默认 1
	if req.Count == 0 {
		req.Count = 1
	}
	res, err := h.svc.SendGift(c.Request.Context(), uid, roomID, giftID, req.Count)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

type devRechargeReq struct {
	UserID int64 `json:"user_id" binding:"required"`
	Delta  int64 `json:"delta" binding:"required"`
}

// @Summary  开发直充金瓜子（M4-LIV-04 临时通道，挂 admin 权限）
// @Tags     管理后台-开发工具
// @Produce  json
// @Security BearerAuth
// @Param    body body live.devRechargeReq true "user_id；delta 金瓜子数量（正数）"
// @Success  200 {object} response.Body{data=live.RechargeResult}
// @Failure  200 {object} response.Body "91023 金额不合法"
// @Router   /admin/dev/recharge [post]
// @Description 开发期临时通道（spec LIV-13 替换点）：真实支付接入 PAY-01 后本接口下线，
// @Description 支付回调复用 RechargeTx 的 biz_no 幂等设计。
func (h *GiftHandler) devRecharge(c *gin.Context) {
	var req devRechargeReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Delta <= 0 {
		response.Fail(c, errcode.ErrLiveRechargeInvalid)
		return
	}
	adminID := c.GetInt64(middleware.CtxAdminID)
	res, err := h.svc.DevRecharge(req.UserID, req.Delta, adminID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}
