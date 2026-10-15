package charge

import (
	"strconv"
	"strings"

	"github.com/dlidli/server/internal/middleware"
	"github.com/dlidli/server/internal/pkg/errcode"
	"github.com/dlidli/server/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// Handler 充电域 HTTP 层。
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// RegisterRoutes C 端路由（spec CHG-01/03）。
//
// g 已挂 optionalAuth：档位/鸣谢是公开读（游客也应看得见「谁充过电」），
// 下单与提现需要登录，故这些端点额外挂 authed（鉴权 + 写接口限流）。
func (h *Handler) RegisterRoutes(g *gin.RouterGroup, authed gin.HandlerFunc) {
	g.GET("/tiers", h.tiers)
	g.GET("/thanks", h.thanks)
	g.POST("/orders", authed, h.createCharge)
	g.GET("/mine", authed, h.myCharges)
	g.GET("/balance", authed, h.balance)
	g.POST("/withdraws", authed, h.applyWithdraw)
	g.GET("/withdraws", authed, h.myWithdraws)
	g.GET("/settlements", authed, h.mySettlements)
}

// RegisterAdminRoutes 管理端路由（spec CHG-02/03、CRT-32）。
//
// g 必须已挂 AdminAuth；perm 逐路由挂权限点——查询与「审核/打款/月结」必须是
// 独立的权限点（给了查询权限等于给了动钱权限是财务系统的典型缺陷，与 PAY-01 同口径）。
func (h *Handler) RegisterAdminRoutes(g *gin.RouterGroup, perm func(code string) gin.HandlerFunc) {
	g.GET("/charges", perm("charge:view"), h.adminCharges)
	g.GET("/settlements", perm("charge:view"), h.adminSettlements)
	g.POST("/settlements/run", perm("charge:settle"), h.adminSettleRun)
	g.GET("/withdraws", perm("charge:view"), h.adminWithdraws)
	g.POST("/withdraws/:withdrawNo/audit", perm("charge:audit"), h.adminAuditWithdraw)
	g.POST("/withdraws/:withdrawNo/payout", perm("charge:payout"), h.adminPayout)
}

// ---- C 端 ----

// @Summary  充电档位列表
// @Tags     充电
// @Produce  json
// @Success  200 {object} response.Body{data=[]charge.TierView}
// @Router   /charge/tiers [get]
// @Description 金额一律整数「分」（spec §6.1）。档位可运营改价与上下架。
func (h *Handler) tiers(c *gin.Context) {
	list, err := h.svc.Tiers()
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"list": list})
}

// @Summary  鸣谢名单（充电者名单，可匿名）
// @Tags     充电
// @Produce  json
// @Param    anchor_id query int false "UP 主 user.id"
// @Param    video_id  query int false "稿件 id（可选，按稿筛选）"
// @Param    limit     query int false "条数（默认 50，上限 200）"
// @Success  200 {object} response.Body{data=[]charge.ThanksItem}
// @Router   /charge/thanks [get]
// @Description 只列已支付的充电（spec CHG-03）。匿名条目只返回金额与时间，
// @Description 不返回 user_id 与昵称——「可匿名」若在接口层泄露身份等于没有匿名。
func (h *Handler) thanks(c *gin.Context) {
	anchor := atoi64(c.Query("anchor_id"))
	video := atoi64(c.Query("video_id"))
	limit := 0
	if v := c.Query("limit"); v != "" {
		limit, _ = strconv.Atoi(v)
	}
	if anchor <= 0 && video <= 0 {
		response.Fail(c, errcode.ErrInvalidParams.WithMsg("需指定 anchor_id 或 video_id"))
		return
	}
	list, err := h.svc.ThanksList(anchor, video, limit)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"list": list})
}

// idArg 兼容 JSON 数字与字符串两种形式的 64 位 ID。
//
// **为什么必须支持字符串**：雪花 ID 超过 2^53，JS 端 JSON.parse 会丢精度，
// 故前端一律以字符串传 ID（与响应侧 `json:"...,string"` 的口径一致）。
// 只接受数字会让真实前端调用直接落到 10002「参数不合法」——本卡 E2E 实测踩到：
// 驱动器按字符串传 anchor_id，全部下单请求被 binding 拦下。
type idArg int64

func (a *idArg) int64() int64 {
	if a == nil {
		return 0
	}
	return int64(*a)
}

// UnmarshalJSON 接受 92710003 / "92710003" / "" / null。
func (a *idArg) UnmarshalJSON(b []byte) error {
	s := strings.Trim(strings.TrimSpace(string(b)), `"`)
	if s == "" || s == "null" {
		*a = 0
		return nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}
	*a = idArg(v)
	return nil
}

type createChargeReq struct {
	AnchorID  idArg  `json:"anchor_id"`
	VideoID   idArg  `json:"video_id"`
	TierCode  string `json:"tier_code" binding:"required"`
	Amount    int64  `json:"amount"`
	Anonymous bool   `json:"anonymous"`
	Message   string `json:"message"`
	Channel   string `json:"channel"`
}

// @Summary  发起充电（下单）
// @Tags     充电
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    body body charge.createChargeReq true "anchor_id 被充电 UP 主；tier_code 档位；自定义档传 amount（分）"
// @Success  200 {object} response.Body{data=pay.OrderView}
// @Failure  200 {object} response.Body "92201 档位不存在 / 92202 金额不合法 / 92203 不能给自己充电 / 92204 对象不存在"
// @Router   /charge/orders [post]
// @Description 金额由**服务端**按档位计算（spec CHG-01），客户端只传档位 code，
// @Description 自定义档须落在档位配置的 min/max 内。复用 PAY-01 支付链路（订单状态机 +
// @Description mock 渠道 + 回调验签幂等）；真实渠道为换入点。
func (h *Handler) createCharge(c *gin.Context) {
	var req createChargeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	uid := c.GetInt64(middleware.CtxUserID)
	order, err := h.svc.CreateCharge(c.Request.Context(), &CreateChargeReq{
		UserID: uid, AnchorID: req.AnchorID.int64(), VideoID: req.VideoID.int64(),
		TierCode: req.TierCode, Amount: req.Amount,
		Anonymous: req.Anonymous, Message: req.Message,
		ClientIP: c.ClientIP(), Channel: req.Channel,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, order)
}

// @Summary  我的充电记录
// @Tags     充电
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} response.Body "list/total/page/size"
// @Router   /charge/mine [get]
func (h *Handler) myCharges(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	res, err := h.svc.MyCharges(uid, page, size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// @Summary  创作者收益账户（现金负债口径）
// @Tags     充电
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} response.Body{data=charge.CreatorBalanceView}
// @Router   /charge/balance [get]
// @Description **这是创作者收益的现金负债账户**，与硬币（coin_log）/金瓜子
// @Description （melon_seed_*）无任何关系（spec §6.1 硬约束）。
func (h *Handler) balance(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	v, err := h.svc.Balance(uid)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, v)
}

type applyWithdrawReq struct {
	Amount      int64  `json:"amount" binding:"required"`
	Account     string `json:"account" binding:"required"`
	AccountName string `json:"account_name" binding:"required"`
}

// @Summary  发起提现申请
// @Tags     充电
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    body body charge.applyWithdrawReq true "amount 金额（分）；account 收款账户；account_name 收款人"
// @Success  200 {object} response.Body{data=charge.WithdrawView}
// @Failure  200 {object} response.Body "92302 金额不合法 / 92304 频次超限 / 92306 余额不足 / 92307 账户不合法"
// @Router   /charge/withdraws [post]
// @Description 申请即冻结（事务 + 行锁，防超提）；审核通过后才可以打款。
// @Description 门槛与频次见 plan §2H.7（最低 100 元、24h 最多 3 次）。
func (h *Handler) applyWithdraw(c *gin.Context) {
	var req applyWithdrawReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	uid := c.GetInt64(middleware.CtxUserID)
	v, err := h.svc.ApplyWithdraw(uid, req.Amount, req.Account, req.AccountName)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, v)
}

// @Summary  我的提现记录
// @Tags     充电
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} response.Body "list/total/page/size"
// @Router   /charge/withdraws [get]
func (h *Handler) myWithdraws(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	res, err := h.svc.MyWithdraws(uid, page, size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// @Summary  我的结算单
// @Tags     充电
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} response.Body "list/total/page/size"
// @Router   /charge/settlements [get]
func (h *Handler) mySettlements(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	res, err := h.svc.Settlements(uid, c.Query("month"), page, size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// ---- 管理端 ----

// @Summary  充电记录查询（管理端）
// @Tags     充电管理
// @Produce  json
// @Security BearerAuth
// @Param    user_id   query int false "充电者"
// @Param    anchor_id query int false "被充电 UP 主"
// @Param    video_id  query int false "稿件"
// @Success  200 {object} response.Body "list/total/page/size"
// @Router   /admin/charge/charges [get]
func (h *Handler) adminCharges(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	f := ChargeFilter{
		UserID: atoi64(c.Query("user_id")), AnchorID: atoi64(c.Query("anchor_id")),
		VideoID: atoi64(c.Query("video_id")), Page: page, Size: size,
	}
	res, err := h.svc.AdminCharges(f)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// @Summary  结算单查询（管理端）
// @Tags     充电管理
// @Produce  json
// @Security BearerAuth
// @Param    user_id query int false "创作者"
// @Param    month   query string false "结算月 YYYY-MM"
// @Success  200 {object} response.Body "list/total/page/size"
// @Router   /admin/charge/settlements [get]
func (h *Handler) adminSettlements(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	res, err := h.svc.AdminSettlements(atoi64(c.Query("user_id")), c.Query("month"), page, size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

type settleRunReq struct {
	Month string `json:"month"`
}

// @Summary  手动触发月度结算（管理端补跑）
// @Tags     充电管理
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    body body charge.settleRunReq false "month 结算月 YYYY-MM（缺省为上一自然月）"
// @Success  200 {object} response.Body{data=charge.SettleResult}
// @Failure  200 {object} response.Body "92401 月份不合法"
// @Router   /admin/charge/settlements/run [post]
// @Description 与月度 Worker 同一实现（幂等，可重复跑）。
func (h *Handler) adminSettleRun(c *gin.Context) {
	var req settleRunReq
	_ = c.ShouldBindJSON(&req)
	res, err := h.svc.AdminSettleMonth(req.Month)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// @Summary  提现审核台列表（管理端）
// @Tags     充电管理
// @Produce  json
// @Security BearerAuth
// @Param    user_id query int false "创作者"
// @Param    status  query int false "状态 0审核中 2已通过 3已拒绝 4已打款 5打款失败"
// @Success  200 {object} response.Body "list/total/page/size"
// @Router   /admin/charge/withdraws [get]
func (h *Handler) adminWithdraws(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	f := WithdrawFilter{UserID: atoi64(c.Query("user_id")), Page: page, Size: size}
	if s := c.Query("status"); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			st := int8(v)
			f.Status = &st
		}
	}
	res, err := h.svc.AdminWithdraws(f)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

type auditWithdrawReq struct {
	Approve bool   `json:"approve"`
	Note    string `json:"note"`
}

// @Summary  提现审核（通过/驳回）
// @Tags     充电管理
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    withdrawNo path string true "提现单号"
// @Param    body body charge.auditWithdrawReq true "approve=true 通过；false 驳回（解冻返还）"
// @Success  200 {object} response.Body{data=charge.WithdrawView}
// @Failure  200 {object} response.Body "92301 单不存在 / 92303 状态不允许（含重复审核）"
// @Router   /admin/charge/withdraws/{withdrawNo}/audit [post]
// @Description 状态迁移用条件 UPDATE + RowsAffected 保证并发安全：重复审核第二个请求
// @Description 返回 92303，**不会二次解冻**（否则余额会被凭空增加）。
func (h *Handler) adminAuditWithdraw(c *gin.Context) {
	var req auditWithdrawReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	adminID := c.GetInt64(middleware.CtxAdminID)
	v, err := h.svc.AdminAuditWithdraw(c.Param("withdrawNo"), req.Approve, adminID, req.Note)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, v)
}

type payoutReq struct {
	Fail   bool   `json:"fail"`
	Remark string `json:"remark"`
}

// @Summary  提现打款（mock 通道）
// @Tags     充电管理
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    withdrawNo path string true "提现单号"
// @Param    body body charge.payoutReq false "fail=true 模拟打款失败（解冻返还）"
// @Success  200 {object} response.Body{data=charge.WithdrawView}
// @Failure  200 {object} response.Body "92303 状态不允许 / 92305 通道未接入"
// @Router   /admin/charge/withdraws/{withdrawNo}/payout [post]
// @Description **真实出款需支付通道资质（换入点）**：当前走 mock 通道，但
// @Description 状态机、冻结扣减、打款流水与幂等闸全部按真实契约实现。
// @Description 双闸防重复出款：条件 UPDATE（2→4）+ withdraw_payout_log 唯一键。
func (h *Handler) adminPayout(c *gin.Context) {
	var req payoutReq
	_ = c.ShouldBindJSON(&req)
	v, err := h.svc.AdminPayout(c.Param("withdrawNo"), req.Fail, req.Remark)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, v)
}

// atoi64 解析 query 中的 64 位整数（雪花 ID 超 int32，故不经过 Atoi）。
func atoi64(s string) int64 {
	if s == "" {
		return 0
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return v
}
