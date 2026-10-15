package vip

import (
	"strconv"

	"github.com/dlidli/server/internal/middleware"
	"github.com/dlidli/server/internal/pkg/errcode"
	"github.com/dlidli/server/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// Handler 会员域路由（M4-VIP-01）：C 端套餐/开通/权益/签约 + 管理端套餐配置与会员查询。
type Handler struct {
	svc *Service
}

// NewHandler 构造。
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// RegisterRoutes C 端路由（挂在 /api/v1/vip 下）。
//   - plans 与 status 允许匿名：套餐价格是公开信息，/vip/status 对游客必须能返回
//     「不是会员」而不是 401（否则首页无法决定是否显示会员入口）；
//   - 开通/解约/签约需登录（资金与订阅变更）。
func (h *Handler) RegisterRoutes(g *gin.RouterGroup, authed gin.HandlerFunc) {
	g.GET("/plans", h.plans)
	g.GET("/status", h.status)
	g.POST("/orders", authed, h.createOrder)
	g.GET("/subscribe", authed, h.subscribeView)
	g.POST("/subscribe/cancel", authed, h.cancelSubscribe)
	// dev/联调用：手动触发一次续期扫描（与 Worker 走同一函数，不是旁路实现）。
	g.POST("/dev/renew-once", h.devRenewOnce)
	// dev/联调用：手动触发月度赠币补发扫描（同上）。
	g.POST("/dev/grant-monthly", h.devGrantMonthly)
}

// RegisterAdminRoutes 管理端路由（g 必须已挂 AdminAuth；perm 逐个路由挂权限点）。
//
// 权限点分离：vip:view 只能看，vip:plan 能改价（改价直接影响收入），
// vip:grant 能手工补发硬币（等价于凭空发币）。三者不得混为一个权限点。
func (h *Handler) RegisterAdminRoutes(g *gin.RouterGroup, perm func(code string) gin.HandlerFunc) {
	g.GET("/plans", perm("vip:view"), h.adminPlans)
	g.POST("/plans", perm("vip:plan"), h.adminUpsertPlan)
	g.GET("/members", perm("vip:view"), h.adminMembers)
	g.POST("/members/:uid/grant-coins", perm("vip:grant"), h.adminGrantCoins)
}

// ---- C 端 ----

// @Summary  会员套餐列表
// @Tags     会员
// @Produce  json
// @Success  200 {object} response.Body "data: {list: []vip.PlanView}"
// @Router   /vip/plans [get]
// @Description 金额一律整数「分」。匿名可访问（价格是公开信息）。
func (h *Handler) plans(c *gin.Context) {
	list, err := h.svc.Plans()
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"list": list})
}

// @Summary  会员状态与权益位
// @Tags     会员
// @Produce  json
// @Success  200 {object} response.Body{data=vip.StatusView}
// @Router   /vip/status [get]
// @Description **服务端判定**：is_vip 完全由 membership.expire_at > NOW() 决定，
// @Description 不接受任何客户端输入，前端无法绕过（plan §2F.1）。匿名访问返回免费权益位。
// @Description rights.max_resolution 为清晰度权益位，实际鉴权由 M4-VIP-02 消费（本卡只下发）。
func (h *Handler) status(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	res, err := h.svc.Status(uid)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

type createOrderReq struct {
	PlanCode string `json:"plan_code" binding:"required"`
	Channel  string `json:"channel"`
}

// @Summary  会员下单（复用支付域订单链路）
// @Tags     会员
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    body body vip.createOrderReq true "plan_code 套餐 code；channel 默认 mock"
// @Success  200 {object} response.Body "data: pay.OrderView"
// @Failure  200 {object} response.Body "92101 套餐不存在或已下架 / 92008 金额不合法 / 92003 渠道不存在"
// @Router   /vip/orders [post]
// @Description **金额由服务端计算**（客户端不传价格）；首次开通连续包月套餐按首月优惠价。
func (h *Handler) createOrder(c *gin.Context) {
	var req createOrderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	channel := req.Channel
	if channel == "" {
		channel = "mock"
	}
	res, err := h.svc.CreateVipOrder(c.Request.Context(), c.GetInt64(middleware.CtxUserID), req.PlanCode, channel)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// @Summary  我的连续包月签约
// @Tags     会员
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} response.Body{data=vip.SubscribeView}
// @Router   /vip/subscribe [get]
func (h *Handler) subscribeView(c *gin.Context) {
	res, err := h.svc.SubscribeView(c.GetInt64(middleware.CtxUserID))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// @Summary  解约连续包月（可随时解约）
// @Tags     会员
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} response.Body
// @Failure  200 {object} response.Body "92105 无有效的连续包月签约"
// @Router   /vip/subscribe/cancel [post]
// @Description 解约后不再自动续期，但**已开通的会员时长不受影响**（到期自然失效，
// @Description 不做按比例退款——退款走支付域的退款通道，与本接口分离）。
func (h *Handler) cancelSubscribe(c *gin.Context) {
	if err := h.svc.CancelSubscribe(c.GetInt64(middleware.CtxUserID)); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"cancelled": true})
}

// @Summary  手动触发一次自动续期扫描（联调用，与 Worker 同一实现）
// @Tags     会员
// @Produce  json
// @Success  200 {object} response.Body "data: {due, ok, failed}"
// @Router   /vip/dev/renew-once [post]
// @Description 与进程内定时 Worker **调用同一函数**，不是旁路实现，故联调结果可信。
func (h *Handler) devRenewOnce(c *gin.Context) {
	due, ok, failed, err := h.svc.RenewOnce(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"due": due, "ok": ok, "failed": failed})
}

// @Summary  手动触发月度赠币补发扫描（联调用）
// @Tags     会员
// @Produce  json
// @Success  200 {object} response.Body "data: {granted, skipped}"
// @Router   /vip/dev/grant-monthly [post]
func (h *Handler) devGrantMonthly(c *gin.Context) {
	granted, skipped, err := h.svc.GrantMonthlyOnce()
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"granted": granted, "skipped": skipped})
}

// ---- 管理端 ----

// @Summary  套餐列表（管理端，含下架项）
// @Tags     会员管理
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} response.Body "data: {list}"
// @Router   /admin/vip/plans [get]
func (h *Handler) adminPlans(c *gin.Context) {
	list, err := h.svc.Plans()
	if err != nil {
		response.Fail(c, err)
		return
	}
	all, err := h.svc.allPlans()
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"list": list, "all": all})
}

type upsertPlanReq struct {
	Code            string `json:"code" binding:"required"`
	Name            string `json:"name" binding:"required"`
	Months          int    `json:"months" binding:"required"`
	Price           int64  `json:"price"`
	OriginalPrice   int64  `json:"original_price"`
	FirstMonthPrice int64  `json:"first_month_price"`
	AutoRenew       bool   `json:"auto_renew"`
	Sort            int    `json:"sort"`
	Status          int8   `json:"status"`
}

// @Summary  套餐配置（新建或改价/上下架）
// @Tags     会员管理
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} response.Body
// @Failure  200 {object} response.Body "92109 套餐参数不合法"
// @Router   /admin/vip/plans [post]
func (h *Handler) adminUpsertPlan(c *gin.Context) {
	var req upsertPlanReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	autoRenew := int8(0)
	if req.AutoRenew {
		autoRenew = 1
	}
	p := &VipPlan{
		Code: req.Code, Name: req.Name, Months: req.Months, Price: req.Price,
		OriginalPrice: req.OriginalPrice, FirstMonthPrice: req.FirstMonthPrice,
		AutoRenew: autoRenew, Sort: req.Sort, Status: req.Status,
	}
	if err := h.svc.AdminUpsertPlan(p, c.GetInt64(middleware.CtxUserID)); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"ok": true})
}

// @Summary  会员查询
// @Tags     会员管理
// @Produce  json
// @Security BearerAuth
// @Param    user_id query int false "按用户筛选"
// @Param    only_vip query int false "1=只看生效中"
// @Success  200 {object} response.Body "data: {list, total, page, size}"
// @Router   /admin/vip/members [get]
func (h *Handler) adminMembers(c *gin.Context) {
	res, err := h.svc.AdminMembers(MemberFilter{
		UserID:  qInt64(c, "user_id"),
		OnlyVip: c.Query("only_vip") == "1",
		Page:    qInt(c, "page"),
		Size:    qInt(c, "size"),
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// @Summary  手工补发会员硬币（幂等）
// @Tags     会员管理
// @Produce  json
// @Security BearerAuth
// @Param    uid path int true "用户 ID"
// @Success  200 {object} response.Body "data: {granted}"
// @Router   /admin/vip/members/{uid}/grant-coins [post]
// @Description 幂等：同一用户同一月重复调用只会发放一次（uk_user_period 唯一闸），
// @Description 故「点两次怕多发」不成立，运维可放心重试。
func (h *Handler) adminGrantCoins(c *gin.Context) {
	uid, err := strconv.ParseInt(c.Param("uid"), 10, 64)
	if err != nil || uid <= 0 {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	granted, err := h.svc.AdminGrantCoins(uid, c.GetInt64(middleware.CtxUserID))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"granted": granted})
}

// qInt 查询参数整数（缺省 0）。
func qInt(c *gin.Context, key string) int {
	v, err := strconv.Atoi(c.Query(key))
	if err != nil {
		return 0
	}
	return v
}

// qInt64 查询参数 int64（缺省 0）。
func qInt64(c *gin.Context, key string) int64 {
	v, err := strconv.ParseInt(c.Query(key), 10, 64)
	if err != nil {
		return 0
	}
	return v
}
