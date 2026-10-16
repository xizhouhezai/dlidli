package pay

import (
	"strconv"

	"github.com/dlidli/server/internal/middleware"
	"github.com/dlidli/server/internal/pkg/errcode"
	"github.com/dlidli/server/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// Handler 支付域路由（M4-PAY-01）：C 端订单/退款/回调 + 管理端订单与对账台。
type Handler struct {
	svc   *Service
	recon *ReconService
}

// NewHandler 构造。
func NewHandler(svc *Service, recon *ReconService) *Handler {
	return &Handler{svc: svc, recon: recon}
}

// RegisterRoutes C 端路由（挂在 /api/v1/pay 下）。
//   - orders 系列需登录（资金接口）；
//   - callback/:channel **无用户鉴权**（回调来自渠道服务器，靠验签鉴权，spec §6.4）。
func (h *Handler) RegisterRoutes(g *gin.RouterGroup, authed gin.HandlerFunc) {
	g.POST("/orders", authed, h.createOrder)
	g.GET("/orders", authed, h.myOrders)
	g.GET("/orders/:orderNo", authed, h.getOrder)
	g.POST("/orders/:orderNo/refund", authed, h.refund)
	g.GET("/cash/balance", authed, h.cashBalance)
}

// RegisterCallbackRoute 注册回调端点。
//
// 单独注册而非挂在 auth 组内：回调来自渠道服务器，**没有也不可能有用户令牌**，
// 它的鉴权就是验签（spec §6.4）。若误挂登录中间件，真实渠道回调将一律 401，
// 表现为「线上支付永不生效」——这是接真实渠道时最容易踩的坑。
func (h *Handler) RegisterCallbackRoute(g *gin.RouterGroup) {
	g.POST("/callback/:channel", h.callback)
}

// RegisterAdminRoutes 管理端路由。
//
// g 必须已挂 AdminAuth（后台令牌鉴权）；perm 是权限点中间件工厂 —— **逐个路由挂对应权限点**
// 而不是整组挂一个：pay:view 只应能看，「手动补单」能凭空给账户加钱、「异常处理」能改对账
// 结论，二者必须是独立的权限点，否则给了查询权限等于给了改账权限（spec §6.7）。
func (h *Handler) RegisterAdminRoutes(g *gin.RouterGroup, perm func(code string) gin.HandlerFunc) {
	g.GET("/orders", perm("pay:view"), h.adminOrders)
	g.POST("/orders/:orderNo/mark-paid", perm("pay:order"), h.manualMarkPaid)
	g.POST("/close-expired", perm("pay:order"), h.closeExpired)
	g.GET("/recon/daily", perm("pay:view"), h.reconDaily)
	g.POST("/recon/run", perm("pay:recon"), h.reconRun)
	g.GET("/recon/issues", perm("pay:view"), h.reconIssues)
	g.POST("/recon/issues/:id/handle", perm("pay:recon"), h.handleIssue)
	// dev 模拟回调会真实入账，按补单同级权限控制（它等价于「宣布收款成功」）。
	g.POST("/dev/pay-callback", perm("pay:order"), h.devCallback)
}

// ---- C 端 ----

type createOrderReq struct {
	// Amount 不加 binding:"required"：required 会把 0 拦成通用的 10002「参数不合法」，
	// 而金额问题应当给出专门的 92008，便于调用方与对账定位（金额校验归 Service 一处）。
	Amount  int64  `json:"amount"`
	Channel string `json:"channel" binding:"required"`
	Subject string `json:"subject"`
	// Sku 商品标识（M4-VIP-01）。会员商品由 /vip/orders 下单（金额由服务端算），
	// 此处保留可选 sku 是为支付域自身也能受理「带商品」的通用下单（如后续 PAY-02）。
	Sku string `json:"sku"`
}

// @Summary  创建支付订单（金额单位：分）
// @Tags     支付
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    body body pay.createOrderReq true "amount 金额（分，整数）；channel mock/alipay/wechat；subject 描述"
// @Success  200 {object} response.Body{data=pay.OrderView}
// @Failure  200 {object} response.Body "92003 渠道不存在 / 92004 渠道未实现 / 92008 金额不合法 / 92012 未成年人受限"
// @Router   /pay/orders [post]
// @Description 金额一律整数「分」，禁止浮点（spec §6.1）。现金账户与虚拟币严格分离：
// @Description 支付只入现金账户，不写硬币（coin_log）与金瓜子（melon_seed_*）。
func (h *Handler) createOrder(c *gin.Context) {
	var req createOrderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	if req.Amount == 0 {
		// 缺省即「没传金额」——必须拒绝，不能默默当成 0 或某个默认值：
		// 支付金额的缺省值意味着一笔无人授权的收款。
		response.Fail(c, errcode.ErrPayAmountInvalid)
		return
	}
	res, err := h.svc.CreateOrder(c.Request.Context(), &CreateOrderReq{
		UserID:   c.GetInt64(middleware.CtxUserID),
		Amount:   req.Amount,
		Channel:  req.Channel,
		Subject:  req.Subject,
		Sku:      req.Sku,
		ClientIP: c.ClientIP(),
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// @Summary  我的订单列表
// @Tags     支付
// @Produce  json
// @Security BearerAuth
// @Param    page query int false "页码（默认1）"
// @Param    size query int false "每页条数（默认20，最大100）"
// @Success  200 {object} response.Body "data: {list, total, page, size}"
// @Router   /pay/orders [get]
func (h *Handler) myOrders(c *gin.Context) {
	res, err := h.svc.MyOrders(c.GetInt64(middleware.CtxUserID), qInt(c, "page"), qInt(c, "size"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// @Summary  订单详情（仅本人）
// @Tags     支付
// @Produce  json
// @Security BearerAuth
// @Param    orderNo path string true "订单号"
// @Success  200 {object} response.Body{data=pay.OrderView}
// @Failure  200 {object} response.Body "92001 订单不存在（含越权访问他人订单）"
// @Router   /pay/orders/{orderNo} [get]
func (h *Handler) getOrder(c *gin.Context) {
	res, err := h.svc.GetOrder(c.Request.Context(), c.Param("orderNo"), c.GetInt64(middleware.CtxUserID))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// @Summary  申请退款（全额，仅已支付订单）
// @Tags     支付
// @Produce  json
// @Security BearerAuth
// @Param    orderNo path string true "订单号"
// @Success  200 {object} response.Body{data=pay.CallbackOutcome}
// @Failure  200 {object} response.Body "92001 订单不存在 / 92002 订单状态不允许退款 / 92004 渠道未实现"
// @Router   /pay/orders/{orderNo}/refund [post]
func (h *Handler) refund(c *gin.Context) {
	res, err := h.svc.Refund(c.Request.Context(), c.Param("orderNo"), c.GetInt64(middleware.CtxUserID))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// @Summary  现金账户余额（分）
// @Tags     支付
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} response.Body "data: {balance}"
// @Router   /pay/cash/balance [get]
// @Description 注意：这是**现金**账户，与硬币（user.coin）和金瓜子（melon_seed_*）无关（spec §6.1）。
func (h *Handler) cashBalance(c *gin.Context) {
	balance, err := h.svc.CashBalance(c.GetInt64(middleware.CtxUserID))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"balance": balance})
}

// @Summary  支付回调（渠道异步通知，验签鉴权）
// @Tags     支付
// @Accept   plain
// @Produce  json
// @Param    channel path   string true  "渠道标识：mock/alipay/wechat"
// @Param    body    body   string true  "渠道原始报文（mock 为 JSON）"
// @Param    X-Pay-Signature header string false "渠道签名（mock 为 HMAC-SHA256(secret,body) 的 hex）"
// @Success  200 {object} response.Body{data=pay.CallbackOutcome}
// @Failure  200 {object} response.Body "92003 渠道不存在 / 92005 验签失败 / 92006 报文不合法 / 92007 订单已过期 / 92008 金额不符"
// @Router   /pay/callback/{channel} [post]
// @Description 幂等键为「渠道 + 渠道交易号」（DB 唯一索引）+ 状态机条件 UPDATE：
// @Description 同一回调重放 N 次，订单状态只迁移一次、现金流水只有一份（spec §6.4）。
func (h *Handler) callback(c *gin.Context) {
	raw, err := c.GetRawData()
	if err != nil {
		response.Fail(c, errcode.ErrPayCallbackInvalid)
		return
	}
	sig := c.GetHeader("X-Pay-Signature")
	res, err := h.svc.HandleCallback(c.Request.Context(), c.Param("channel"), raw, sig)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// ---- 管理端 ----

// @Summary  订单查询（管理端）
// @Tags     支付管理
// @Produce  json
// @Security BearerAuth
// @Param    order_no   query string false "订单号"
// @Param    user_id    query int    false "用户ID"
// @Param    channel    query string false "渠道"
// @Param    status     query int    false "状态 0待支付 1已支付 2已关闭 3已退款"
// @Param    min_amount query int    false "金额下限（分，含）"
// @Param    max_amount query int    false "金额上限（分，含）"
// @Param    start_date query string false "创建起始日 YYYY-MM-DD（含当日）"
// @Param    end_date   query string false "创建结束日 YYYY-MM-DD（含当日）"
// @Param    page       query int    false "页码"
// @Param    size       query int    false "每页条数"
// @Success  200 {object} response.Body "data: {list, total, page, size}"
// @Failure  401 {object} response.Body "未携带/无效管理员令牌"
// @Failure  403 {object} response.Body "缺少 pay:view 权限"
// @Router   /admin/pay/orders [get]
// bindOrderFilter 把管理端订单查询的 query 参数绑成筛选条件。
//
// 独立成函数以便用例直接断言绑定结果（金额下限/上限与日期边界是本次新增字段，
// 「SQL 写对了但 handler 忘了透传」是这类改动的典型失效模式）。
func bindOrderFilter(c *gin.Context) OrderFilter {
	return OrderFilter{
		OrderNo: c.Query("order_no"),
		UserID:  int64(qInt(c, "user_id")),
		Channel: c.Query("channel"),
		Page:    qInt(c, "page"),
		Size:    qInt(c, "size"),
		// 金额单位分、日期 YYYY-MM-DD：非法值直接判为「未筛选」，
		// 与既有 status 的处理保持一致（避免把非法值当成「金额=0」这种有效条件）。
		MinAmount: int64(qInt(c, "min_amount")),
		MaxAmount: int64(qInt(c, "max_amount")),
		StartDate: c.Query("start_date"),
		EndDate:   c.Query("end_date"),
	}
}

func (h *Handler) adminOrders(c *gin.Context) {
	f := bindOrderFilter(c)
	if s := c.Query("status"); s != "" {
		v, err := strconv.ParseInt(s, 10, 8)
		if err != nil {
			response.Fail(c, errcode.ErrInvalidParams)
			return
		}
		st := int8(v)
		f.Status = &st
	}
	res, err := h.svc.AdminOrders(f)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// @Summary  手动补单（人工确认收款后入账）
// @Tags     支付管理
// @Produce  json
// @Security BearerAuth
// @Param    orderNo path string true "订单号"
// @Success  200 {object} response.Body{data=pay.CallbackOutcome}
// @Failure  200 {object} response.Body "92001 订单不存在 / 92002 已支付或已退款不得补单"
// @Router   /admin/pay/orders/{orderNo}/mark-paid [post]
// @Description 与回调入账共用同一底层函数（条件 UPDATE + RowsAffected），故幂等与一致性保证一致；
// @Description 差别是允许对已关闭（超时）订单补单，并在流水备注中记录操作人。
func (h *Handler) manualMarkPaid(c *gin.Context) {
	res, err := h.svc.ManualMarkPaid(c.Param("orderNo"), c.GetInt64(middleware.CtxAdminID))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// @Summary  手动触发超时关单
// @Tags     支付管理
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} response.Body "data: {closed}"
// @Router   /admin/pay/close-expired [post]
func (h *Handler) closeExpired(c *gin.Context) {
	n, err := h.svc.AdminCloseExpired()
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"closed": n})
}

// @Summary  日对账单
// @Tags     支付管理
// @Produce  json
// @Security BearerAuth
// @Param    date    query string false "对账日期 YYYY-MM-DD"
// @Param    channel query string false "渠道"
// @Success  200 {object} response.Body "data: {list, total, page, size}"
// @Router   /admin/pay/recon/daily [get]
func (h *Handler) reconDaily(c *gin.Context) {
	res, err := h.recon.Daily(c.Query("date"), c.Query("channel"), qInt(c, "page"), qInt(c, "size"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// @Summary  执行对账（重跑幂等）
// @Tags     支付管理
// @Produce  json
// @Security BearerAuth
// @Param    date    query string true  "对账日期 YYYY-MM-DD"
// @Param    channel query string false "渠道（缺省为全部已注册渠道）"
// @Success  200 {object} response.Body{data=pay.ReconResult}
// @Failure  200 {object} response.Body "92011 对账日期不合法（格式错或未来日期）"
// @Router   /admin/pay/recon/run [post]
// @Description 口径为「本地订单 ↔ 本地现金流水」两条独立写入路径相互印证；
// @Description 真实渠道对账文件解析是换入点（不在本卡）。
func (h *Handler) reconRun(c *gin.Context) {
	res, err := h.recon.Run(c.Query("date"), c.Query("channel"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// @Summary  异常订单清单（人工处理台）
// @Tags     支付管理
// @Produce  json
// @Security BearerAuth
// @Param    date       query int false "对账日期 YYYY-MM-DD"
// @Param    channel    query int false "渠道"
// @Param    status     query int false "处理状态 0待处理 1已处理"
// @Param    issue_type query int false "异常类型 1金额不符 2长期挂起 3流水缺失 4重复回调"
// @Success  200 {object} response.Body "data: {list, total, page, size}"
// @Router   /admin/pay/recon/issues [get]
func (h *Handler) reconIssues(c *gin.Context) {
	f := IssueFilter{
		Date:    c.Query("date"),
		Channel: c.Query("channel"),
		Page:    qInt(c, "page"),
		Size:    qInt(c, "size"),
	}
	if s := c.Query("status"); s != "" {
		v, err := strconv.ParseInt(s, 10, 8)
		if err != nil {
			response.Fail(c, errcode.ErrInvalidParams)
			return
		}
		st := int8(v)
		f.Status = &st
	}
	if s := c.Query("issue_type"); s != "" {
		v, err := strconv.ParseInt(s, 10, 8)
		if err != nil {
			response.Fail(c, errcode.ErrInvalidParams)
			return
		}
		it := int8(v)
		f.IssueType = &it
	}
	res, err := h.recon.Issues(f)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

type handleIssueReq struct {
	Note string `json:"note"`
}

// @Summary  异常订单处理（备注 + 处理人）
// @Tags     支付管理
// @Produce  json
// @Security BearerAuth
// @Param    id   path int            true "异常记录ID"
// @Param    body body pay.handleIssueReq false "处理备注"
// @Success  200 {object} response.Body{data=pay.IssueView}
// @Router   /admin/pay/recon/issues/{id}/handle [post]
func (h *Handler) handleIssue(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	var req handleIssueReq
	_ = c.ShouldBindJSON(&req) // 备注可省
	res, err := h.recon.Handle(id, c.GetInt64(middleware.CtxAdminID), req.Note)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

type devCallbackReq struct {
	OrderNo string `json:"order_no" binding:"required"`
	Amount  int64  `json:"amount"` // 分；缺省取订单金额（造「金额不符」异常时显式传）
	Result  string `json:"result"` // success / fail，缺省 success
}

// @Summary  模拟渠道回调（dev 联调，挂 admin 权限）
// @Tags     支付管理
// @Produce  json
// @Security BearerAuth
// @Param    body body pay.devCallbackReq true "order_no；amount（分，可省略）；result success/fail（可省略）"
// @Success  200 {object} response.Body{data=pay.CallbackOutcome}
// @Router   /admin/pay/dev/pay-callback [post]
// @Description **与真实渠道回调走同一条处理链**（HandleCallback）：验签、幂等、金额核对、入账
// @Description 全部一致，不因为是联调入口而走旁路——旁路会让联调路径上的保证失效。
// @Description 金额单位分；传一个与订单不同的 amount 可复现「金额不符」异常用于对账验收。
func (h *Handler) devCallback(c *gin.Context) {
	var req devCallbackReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	if req.Result == "" {
		req.Result = "success"
	}
	res, err := h.svc.DevCallback(req.OrderNo, req.Amount, req.Result)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// qInt 取查询参数整数（非法或缺失返回 0，由业务层兜默认值）。
func qInt(c *gin.Context, key string) int {
	v, err := strconv.Atoi(c.Query(key))
	if err != nil {
		return 0
	}
	return v
}
