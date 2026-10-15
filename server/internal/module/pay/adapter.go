package pay

import (
	"context"
	"errors"
	"sync"

	"github.com/dlidli/server/internal/pkg/errcode"
)

// ChannelMock 是内置 mock 渠道标识（全链路联调与验收用，spec §6.3）。
const ChannelMock = "mock"

// ChannelAlipay / ChannelWeChat 是真实渠道标识（骨架已留，未接入，spec §6.3）。
const (
	ChannelAlipay = "alipay"
	ChannelWeChat = "wechat"
)

// CreatePaymentReq 下单请求（金额单位：分）。
type CreatePaymentReq struct {
	OrderNo string // 商户订单号（对外业务单号）
	Amount  int64  // 分
	Subject string // 商品描述
	ClientIP string
	NotifyURL string // 渠道异步回调地址（部署配置决定，业务侧不硬编码域名）
}

// CreatePaymentResp 下单响应：渠道预支付凭据（唤起支付所需的一切）。
type CreatePaymentResp struct {
	ChannelTradeNo string            // 渠道交易号（mock 立即生成；真实渠道可能下单后才回）
	Params         map[string]string // 唤起支付参数（真实渠道为签名后的表单/JSON）
}

// QueryPaymentResp 查单响应。
type QueryPaymentResp struct {
	Status         int8   // 1 已支付 0 未支付（渠道侧口径）
	ChannelTradeNo string
	Amount         int64 // 分
}

// RefundReq 退款请求（金额单位：分）。
type RefundReq struct {
	OrderNo        string
	ChannelTradeNo string
	Amount         int64 // 分
	Reason         string
}

// RefundResp 退款响应。
type RefundResp struct {
	Success       bool
	RefundNo      string // 渠道退款单号
	RefundedAmount int64 // 实际退款金额（分）
}

// CallbackResult 回调解析结果（验签通过后才有意义）。
type CallbackResult struct {
	OrderNo        string
	ChannelTradeNo string
	Amount         int64  // 分：对账的「金额不符」判据来源
	Success        bool   // 支付是否成功
	RawBody        string // 原始报文（落库取证）
}

// Adapter 支付渠道适配器契约（spec §6.3）：下单 / 查单 / 退款 / 回调验签 四方法。
//
// 业务侧（订单服务）只依赖本接口，不依赖任何具体渠道实现——这是「将来接真渠道只换适配器」
// 的结构保证。**接口签名即契约，真实渠道实现不得改变签名**（plan §2E.2）。
type Adapter interface {
	// Name 渠道标识（与 pay_order.channel 取值一致）。
	Name() string
	// CreatePayment 下单：返回唤起支付所需凭据。
	CreatePayment(ctx context.Context, req *CreatePaymentReq) (*CreatePaymentResp, error)
	// QueryPayment 查单：以渠道侧为准的状态查询（支付结果兜底确认）。
	QueryPayment(ctx context.Context, orderNo, channelTradeNo string) (*QueryPaymentResp, error)
	// Refund 退款。
	Refund(ctx context.Context, req *RefundReq) (*RefundResp, error)
	// VerifyCallback 回调验签：校验原始报文与签名，返回解析结果。
	// 验签失败必须返回错误（ErrPayCallbackSign），不得返回「成功但金额为 0」这类含糊结果。
	VerifyCallback(ctx context.Context, rawBody []byte, signature string) (*CallbackResult, error)
}

// ---- 适配器注册表 ----

var (
	adaptersMu sync.RWMutex
	adapters   = map[string]Adapter{}
)

// Register 注册渠道适配器（重复注册同渠道覆盖，便于测试注入）。
func Register(a Adapter) {
	adaptersMu.Lock()
	defer adaptersMu.Unlock()
	adapters[a.Name()] = a
}

// Get 取渠道适配器；不存在返回 92003（spec §6.3 / §6.2 下单拒绝）。
func Get(channel string) (Adapter, error) {
	adaptersMu.RLock()
	defer adaptersMu.RUnlock()
	a, ok := adapters[channel]
	if !ok {
		return nil, errcode.ErrPayChannelUnknown
	}
	return a, nil
}

// IsRegistered 渠道是否已注册（下单前置校验与错误码区分用）。
func IsRegistered(channel string) bool {
	adaptersMu.RLock()
	defer adaptersMu.RUnlock()
	_, ok := adapters[channel]
	return ok
}

// notImplemented 真实渠道骨架的统一返回：92004「渠道未实现」。
// **不得返回伪造成功**（spec §3 末条 / §6.3）——静默成功会让业务侧把未接入的渠道
// 当成可用渠道，进而产生「订单已支付但没有真实收款」的账实不符。
func notImplemented() error { return errcode.ErrPayChannelNotImpl }

// errNotImplemented 供骨架内部用 errors.Is 判定。
var errNotImplemented = errors.New("channel not implemented")

func init() {
	// 内置渠道注册：mock 可用；支付宝/微信为换入点（骨架实现，调用即 92004）。
	Register(newMockAdapter())
	Register(newAlipayAdapter())
	Register(newWechatAdapter())
}
