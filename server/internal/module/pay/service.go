package pay

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/dlidli/server/internal/pkg/errcode"
	"go.uber.org/zap"
)

// Config 支付域配置（router 装配注入，缺省值由 config.Load 兜底）。
type Config struct {
	// OrderTimeoutMinutes 订单超时分钟数（默认 15）。
	OrderTimeoutMinutes int
	// StuckHours 对账「长期挂起」阈值小时数（默认 24）。
	StuckHours int
	// MockSecret mock 渠道共享密钥（回调验签）；为空时 mock 回调一律拒绝（fail-closed）。
	MockSecret string
	// MaxAmount 单笔金额上限（分，默认 100 万即 1 万元）；防误传以「元」当「分」。
	MaxAmount int64
	// NotifyURLBase 渠道回调地址前缀（部署配置决定，业务侧不硬编码域名）。
	NotifyURLBase string
}

// DefaultConfig 缺省配置（测试与未配置环境的兜底）。
func DefaultConfig() Config {
	return Config{
		OrderTimeoutMinutes: 15,
		StuckHours:          24,
		MockSecret:          "dev-mock-pay-secret-do-not-use-in-prod",
		MaxAmount:           1_000_000,
		NotifyURLBase:       "http://127.0.0.1:8000",
	}
}

// ConsumptionGuard 消费限制钩子（spec §6.6 合规钩子）。
//
// **实现程度声明**：默认实现放行。本项目 user_auth 无出生日期字段、实名判定仅
// 「存在手机/邮箱记录」，故**未成年人识别的数据基础不具备**——此处只固定调用点，
// 真实规则待 ACC-23 补全后替换。**不得据此宣称合规达标**（spec §6.6 / plan §2E.7）。
type ConsumptionGuard interface {
	Check(ctx context.Context, userID, amount int64) error
}

// allowAllGuard 默认钩子：放行一切（显式替换点）。
type allowAllGuard struct{}

func (allowAllGuard) Check(context.Context, int64, int64) error { return nil }

// Service 支付域服务：订单状态机、渠道网关、回调验签与幂等、退款、对账。
type Service struct {
	repo  *repo
	cfg   Config
	log   *zap.Logger
	guard ConsumptionGuard
}

// NewService 构造（cfg 零值时用 DefaultConfig 兜底）。
func NewService(r *repo, cfg Config, log *zap.Logger) *Service {
	def := DefaultConfig()
	if cfg.OrderTimeoutMinutes <= 0 {
		cfg.OrderTimeoutMinutes = def.OrderTimeoutMinutes
	}
	if cfg.StuckHours <= 0 {
		cfg.StuckHours = def.StuckHours
	}
	if cfg.MaxAmount <= 0 {
		cfg.MaxAmount = def.MaxAmount
	}
	if cfg.NotifyURLBase == "" {
		cfg.NotifyURLBase = def.NotifyURLBase
	}
	if cfg.MockSecret == "" {
		cfg.MockSecret = def.MockSecret
	}
	if log == nil {
		log = zap.NewNop()
	}
	// mock 渠道密钥在此统一下发（唯一注入点，避免测试与运行时口径分裂）。
	if a, err := Get(ChannelMock); err == nil {
		if m, ok := a.(*mockAdapter); ok {
			m.SetSecret(cfg.MockSecret)
		}
	}
	return &Service{repo: r, cfg: cfg, log: log, guard: allowAllGuard{}}
}

// SetGuard 注入消费限制钩子（替换点；nil 表示用默认放行实现）。
func (s *Service) SetGuard(g ConsumptionGuard) {
	if g == nil {
		s.guard = allowAllGuard{}
		return
	}
	s.guard = g
}

// MockSecret 暴露 mock 密钥（dev 回调端点构造签名用；与验签口径单一来源）。
func (s *Service) MockSecret() string { return s.cfg.MockSecret }

// ---- 订单 ----

// CreateOrderReq 下单请求。
type CreateOrderReq struct {
	UserID   int64
	Amount   int64  // 分
	Channel  string // mock / alipay / wechat
	Subject  string
	ClientIP string
}

// OrderView 订单对外视图（金额一律整数分，spec §6.1）。
type OrderView struct {
	OrderNo        string            `json:"order_no"`
	UserID         string            `json:"user_id"`
	Amount         int64             `json:"amount"` // 分
	Channel        string            `json:"channel"`
	Subject        string            `json:"subject"`
	Status         int8              `json:"status"`
	StatusText     string            `json:"status_text"`
	ChannelTradeNo string            `json:"channel_trade_no,omitempty"`
	RefundAmount   int64             `json:"refund_amount"`
	ExpireAt       string            `json:"expire_at"`
	PaidAt         string            `json:"paid_at,omitempty"`
	ClosedAt       string            `json:"closed_at,omitempty"`
	RefundedAt     string            `json:"refunded_at,omitempty"`
	CreatedAt      string            `json:"created_at"`
	PayParams      map[string]string `json:"pay_params,omitempty"`
}

func toOrderView(o *PayOrder) *OrderView {
	v := &OrderView{
		OrderNo: o.OrderNo, UserID: itoa(o.UserID), Amount: o.Amount,
		Channel: o.Channel, Subject: o.Subject, Status: o.Status,
		StatusText: statusText(o.Status), RefundAmount: o.RefundAmount,
		ExpireAt:  o.ExpireAt.Format("2006-01-02 15:04:05"),
		CreatedAt: o.CreatedAt.Format("2006-01-02 15:04:05"),
	}
	if o.ChannelTradeNo != nil {
		v.ChannelTradeNo = *o.ChannelTradeNo
	}
	if o.PaidAt != nil {
		v.PaidAt = o.PaidAt.Format("2006-01-02 15:04:05")
	}
	if o.ClosedAt != nil {
		v.ClosedAt = o.ClosedAt.Format("2006-01-02 15:04:05")
	}
	if o.RefundedAt != nil {
		v.RefundedAt = o.RefundedAt.Format("2006-01-02 15:04:05")
	}
	return v
}

// CreateOrder 下单（spec §6.2）：校验渠道与金额 → 落订单 → 调渠道下单拿预付凭据。
//
// 惰性关单在入口执行一次（plan §2E.3）：保证「过期订单不会被支付」这一不变量
// 不依赖任何后台 Worker 存活。
func (s *Service) CreateOrder(ctx context.Context, req *CreateOrderReq) (*OrderView, error) {
	if err := s.closeExpired(); err != nil {
		s.log.Warn("惰性关单失败（不阻断下单）", zap.Error(err))
	}
	if req.Amount <= 0 || req.Amount > s.cfg.MaxAmount {
		return nil, errcode.ErrPayAmountInvalid
	}
	adapter, err := Get(req.Channel)
	if err != nil {
		return nil, err
	}
	// 消费限制钩子调用点 ①（spec §6.6）：下单前判定，拒绝则不产生订单。
	if err := s.guard.Check(ctx, req.UserID, req.Amount); err != nil {
		return nil, err
	}
	now := time.Now()
	o := &PayOrder{
		OrderNo:  genOrderNo(),
		UserID:   req.UserID,
		Amount:   req.Amount,
		Channel:  req.Channel,
		Subject:  req.Subject,
		Status:   StatusPending,
		ClientIP: req.ClientIP,
		ExpireAt: now.Add(time.Duration(s.cfg.OrderTimeoutMinutes) * time.Minute),
	}
	if err := s.repo.CreateOrder(o); err != nil {
		return nil, err
	}
	resp, err := adapter.CreatePayment(ctx, &CreatePaymentReq{
		OrderNo:   o.OrderNo,
		Amount:    o.Amount,
		Subject:   o.Subject,
		ClientIP:  o.ClientIP,
		NotifyURL: s.cfg.NotifyURLBase + "/api/v1/pay/callback/" + req.Channel,
	})
	if err != nil {
		// 渠道下单失败：订单保留为待支付（用户可重试唤起），但**不得**假装成功。
		// 真实渠道此处应落一条渠道调用失败日志；本卡只回错误，订单由超时机制自然关闭。
		s.log.Warn("渠道下单失败", zap.String("order_no", o.OrderNo),
			zap.String("channel", req.Channel), zap.Error(err))
		return nil, err
	}
	v := toOrderView(o)
	v.PayParams = resp.Params
	if resp.ChannelTradeNo != "" {
		v.ChannelTradeNo = resp.ChannelTradeNo
	}
	return v, nil
}

// GetOrder 查单（仅本人，spec §6.7 越权口径：不泄露他人订单存在性）。
func (s *Service) GetOrder(ctx context.Context, orderNo string, userID int64) (*OrderView, error) {
	if err := s.closeExpired(); err != nil {
		s.log.Warn("惰性关单失败（不阻断查单）", zap.Error(err))
	}
	o, err := s.repo.GetUserOrder(orderNo, userID)
	if err != nil {
		return nil, err
	}
	return toOrderView(o), nil
}

// MyOrders 我的订单列表。
func (s *Service) MyOrders(userID int64, page, size int) (map[string]any, error) {
	page, size = normPage(page, size)
	list, total, err := s.repo.ListUserOrders(userID, page, size)
	if err != nil {
		return nil, err
	}
	views := make([]*OrderView, 0, len(list))
	for i := range list {
		views = append(views, toOrderView(&list[i]))
	}
	return map[string]any{"list": views, "total": total, "page": page, "size": size}, nil
}

// CashBalance 现金余额（分）。注意：这是**现金**账户，与硬币/金瓜子无关（spec §6.1）。
func (s *Service) CashBalance(userID int64) (int64, error) { return s.repo.CashBalance(userID) }

// ---- 回调（验签 + 幂等，spec §6.4） ----

// CallbackOutcome 回调处理结果（对外返回，便于渠道/联调方诊断）。
type CallbackOutcome struct {
	OrderNo string `json:"order_no"`
	Applied bool   `json:"applied"` // true=本次真正生效；false=幂等（此前已生效）
	Status  int8   `json:"status"`
	Message string `json:"message"`
}

// HandleCallback 回调处理主链（spec §6.3/§6.4）。**所有回调入口（真实渠道、dev 模拟）
// 必须走这一条链**——旁路实现会让「验签 + 幂等」的保证在联调路径上失效，
// 而联调路径恰恰是最容易被当成可用实现的地方。
//
// 顺序（每一步失败都不得留下部分副作用）：
//  1. 取渠道适配器（不存在 → 92003）。
//  2. 验签（失败 → 92005，落 sign_ok=0 的日志，订单与资金**零变化**）。
//  3. 惰性关单 + 解析（失败 → 92006）。
//  4. 金额核对（不符 → 落日志 + 拒绝；**绝不入账一个可疑金额**）。
//  5. 幂等入账（条件 UPDATE + 同事务入账；重放/并发只生效一次）。
func (s *Service) HandleCallback(ctx context.Context, channel string, rawBody []byte, signature string) (*CallbackOutcome, error) {
	adapter, err := Get(channel)
	if err != nil {
		return nil, err
	}
	logRow := &PayCallbackLog{Channel: channel, RawBody: string(rawBody), Signature: signature}

	res, err := adapter.VerifyCallback(ctx, rawBody, signature)
	if err != nil {
		// 验签失败也落库（spec §6.4）：否则「有人持续伪造回调」在系统内不可观测。
		logRow.SignOK = 0
		// 尽力而为地从**未经验证**的报文里抠出订单号与金额：验签失败恰恰是最需要定位
		// 「在伪造哪一笔订单」的场景，而报文本身不可信，故在 result 里显式标注未经验证。
		if hint := peekCallback(rawBody); hint != nil {
			logRow.OrderNo = hint.OrderNo
			logRow.ChannelTradeNo = hint.ChannelTradeNo
			logRow.Amount = hint.Amount
		}
		logRow.Result = "验签失败（报文自述内容未经验证）：" + err.Error()
		if e := s.repo.InsertCallbackLog(logRow); e != nil {
			s.log.Warn("回调日志落库失败", zap.Error(e))
		}
		return nil, err
	}
	logRow.SignOK = 1
	logRow.OrderNo = res.OrderNo
	logRow.ChannelTradeNo = res.ChannelTradeNo
	logRow.Amount = res.Amount

	// 支付失败回调：验签通过但渠道侧未成功。不推进订单，记日志即可（订单由超时机制关闭）。
	if !res.Success {
		logRow.Result = "渠道侧支付失败，订单不推进"
		if e := s.repo.InsertCallbackLog(logRow); e != nil {
			s.log.Warn("回调日志落库失败", zap.Error(e))
		}
		return &CallbackOutcome{OrderNo: res.OrderNo, Applied: false, Message: "渠道侧支付失败"}, nil
	}

	if err := s.closeExpired(); err != nil {
		s.log.Warn("惰性关单失败（不阻断回调）", zap.Error(err))
	}
	order, err := s.repo.GetOrder(res.OrderNo)
	if err != nil {
		logRow.Result = "订单不存在：" + err.Error()
		if e := s.repo.InsertCallbackLog(logRow); e != nil {
			s.log.Warn("回调日志落库失败", zap.Error(e))
		}
		return nil, err
	}
	// 金额核对：回调所报金额必须与订单金额一致。
	// 放在入账前，是因为一旦入账，账实不符就已经发生；金额是回调报文里唯一被篡改后
	// 直接造成损失（或占便宜）的字段。
	if res.Amount != order.Amount {
		logRow.Result = fmt.Sprintf("金额不符：回调 %d 分 ≠ 订单 %d 分，拒绝入账", res.Amount, order.Amount)
		if e := s.repo.InsertCallbackLog(logRow); e != nil {
			s.log.Warn("回调日志落库失败", zap.Error(e))
		}
		return nil, errcode.ErrPayAmountInvalid
	}
	// 消费限制钩子调用点 ②（spec §6.6）：入账前二次判定。
	if err := s.guard.Check(ctx, order.UserID, order.Amount); err != nil {
		logRow.Result = "消费限制拒绝：" + err.Error()
		if e := s.repo.InsertCallbackLog(logRow); e != nil {
			s.log.Warn("回调日志落库失败", zap.Error(e))
		}
		return nil, err
	}

	applied, err := s.repo.MarkPaidTx(res.OrderNo, res.ChannelTradeNo)
	if err != nil {
		logRow.Result = "入账失败：" + err.Error()
		if e := s.repo.InsertCallbackLog(logRow); e != nil {
			s.log.Warn("回调日志落库失败", zap.Error(e))
		}
		return nil, err
	}
	// mock 渠道同步渠道侧状态，使查单口径与业务口径一致（真实渠道由渠道侧自持）。
	if m, ok := adapter.(*mockAdapter); ok {
		m.markPaid(res.OrderNo, res.ChannelTradeNo, res.Amount)
	}
	if applied {
		logRow.Result = "支付成功，已入账"
	} else {
		logRow.Result = "幂等：此前已入账，本次无副作用"
	}
	if e := s.repo.InsertCallbackLog(logRow); e != nil {
		s.log.Warn("回调日志落库失败", zap.Error(e))
	}
	cur, err := s.repo.GetOrder(res.OrderNo)
	if err != nil {
		return nil, err
	}
	return &CallbackOutcome{
		OrderNo: res.OrderNo, Applied: applied, Status: cur.Status,
		Message: logRow.Result,
	}, nil
}

// ---- 退款 ----

// Refund 退款（spec §6.2）：仅已支付订单可退；渠道退款成功后事务内扣回现金余额。
func (s *Service) Refund(ctx context.Context, orderNo string, userID int64) (*CallbackOutcome, error) {
	o, err := s.repo.GetUserOrder(orderNo, userID)
	if err != nil {
		return nil, err
	}
	if o.Status != StatusPaid {
		return nil, errcode.ErrPayOrderStatus
	}
	adapter, err := Get(o.Channel)
	if err != nil {
		return nil, err
	}
	tradeNo := ""
	if o.ChannelTradeNo != nil {
		tradeNo = *o.ChannelTradeNo
	}
	rr, err := adapter.Refund(ctx, &RefundReq{
		OrderNo: o.OrderNo, ChannelTradeNo: tradeNo, Amount: o.Amount, Reason: "用户申请退款",
	})
	if err != nil {
		return nil, err
	}
	if !rr.Success {
		// 渠道退款未成功：**不得**把订单置为已退款（否则本地账实不符）。
		return nil, errcode.ErrPayOrderStatus.WithMsg("渠道退款未成功，订单保持已支付")
	}
	applied, err := s.repo.RefundTx(orderNo)
	if err != nil {
		return nil, err
	}
	cur, err := s.repo.GetOrder(orderNo)
	if err != nil {
		return nil, err
	}
	msg := "退款完成"
	if !applied {
		msg = "幂等：此前已退款，本次无副作用"
	}
	return &CallbackOutcome{OrderNo: orderNo, Applied: applied, Status: cur.Status, Message: msg}, nil
}

// ---- 管理端 ----

// AdminOrders 管理端订单查询。
func (s *Service) AdminOrders(f OrderFilter) (map[string]any, error) {
	f.Page, f.Size = normPage(f.Page, f.Size)
	list, total, err := s.repo.ListOrders(f)
	if err != nil {
		return nil, err
	}
	views := make([]*OrderView, 0, len(list))
	for i := range list {
		views = append(views, toOrderView(&list[i]))
	}
	return map[string]any{"list": views, "total": total, "page": f.Page, "size": f.Size}, nil
}

// ManualMarkPaid 手动补单（spec §6.7，人工确认收款后入账）。
func (s *Service) ManualMarkPaid(orderNo string, adminID int64) (*CallbackOutcome, error) {
	if err := s.closeExpired(); err != nil {
		s.log.Warn("惰性关单失败（不阻断补单）", zap.Error(err))
	}
	applied, err := s.repo.MarkPaidManualTx(orderNo, adminID, "")
	if err != nil {
		return nil, err
	}
	cur, err := s.repo.GetOrder(orderNo)
	if err != nil {
		return nil, err
	}
	msg := "补单成功，已入账"
	if !applied {
		msg = "补单未生效（订单已支付或状态不允许）"
	}
	return &CallbackOutcome{OrderNo: orderNo, Applied: applied, Status: cur.Status, Message: msg}, nil
}

// AdminCloseExpired 手动触发关单（运维与验收用；返回关闭笔数）。
func (s *Service) AdminCloseExpired() (int64, error) { return s.repo.CloseExpired() }

// DevCallback 模拟渠道回调（spec §6.7）：构造并签名一条 mock 报文，
// **走与真实渠道完全相同的 HandleCallback 链路**。
func (s *Service) DevCallback(orderNo string, amount int64, result string) (*CallbackOutcome, error) {
	o, err := s.repo.GetOrder(orderNo)
	if err != nil {
		return nil, err
	}
	if amount <= 0 {
		amount = o.Amount
	}
	// 渠道交易号：已存在则复用（重放场景必须复用同一交易号，否则测不出幂等键的作用），
	// 否则生成一个新的。
	tradeNo := "MOCKDEV" + orderNo
	if o.ChannelTradeNo != nil && *o.ChannelTradeNo != "" {
		tradeNo = *o.ChannelTradeNo
	}
	channel := o.Channel
	body, sig, err := MockBuildCallback(s.cfg.MockSecret, orderNo, tradeNo, amount, result)
	if err != nil {
		return nil, err
	}
	return s.HandleCallback(context.Background(), channel, body, sig)
}

// ---- 内部 ----

// closeExpired 惰性关单（plan §2E.3）。
func (s *Service) closeExpired() error {
	n, err := s.repo.CloseExpired()
	if err != nil {
		return err
	}
	if n > 0 {
		s.log.Info("惰性关单完成", zap.Int64("closed", n))
	}
	return nil
}

// genOrderNo 订单号：时间前缀 + 加密随机，便于人工识别与排序；
// 不用递增序列是为了**不泄露业务量**（订单号对外可见，连续编号等于公开交易笔数）。
func genOrderNo() string {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		// rand 失败极罕见；退化用纳秒时间戳，唯一性仍由 DB 唯一键兜底。
		return "PAY" + strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return "PAY" + time.Now().Format("20060102150405") + hex.EncodeToString(buf)
}

// peekCallback 尽力从**未经验证**的报文中抠出订单号/交易号/金额，仅用于验签失败时的
// 日志留痕（定位「有人在伪造哪一笔订单」）。返回值可能为 nil，且其内容**绝不可**用于
// 任何业务判定 —— 调用方必须把它当作「报文的自我声明」而非事实。
func peekCallback(rawBody []byte) *CallbackResult {
	var body struct {
		OrderNo        string `json:"order_no"`
		ChannelTradeNo string `json:"channel_trade_no"`
		Amount         int64  `json:"amount"`
	}
	if err := json.Unmarshal(rawBody, &body); err != nil {
		return nil
	}
	if body.OrderNo == "" && body.ChannelTradeNo == "" {
		return nil
	}
	// 单号长度兜底：截断超长输入，避免伪造者用超长字符串撑爆日志列。
	return &CallbackResult{
		OrderNo:        truncate(body.OrderNo, 40),
		ChannelTradeNo: truncate(body.ChannelTradeNo, 64),
		Amount:         body.Amount,
	}
}

// truncate 按字节截断（列宽兜底；订单号与交易号均为 ASCII）。
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

func itoa(v int64) string { return strconv.FormatInt(v, 10) }

func normPage(page, size int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 || size > 100 {
		size = 20
	}
	return page, size
}
