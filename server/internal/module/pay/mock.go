package pay

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/dlidli/server/internal/pkg/errcode"
)

// mockAdapter 内置 mock 渠道（spec §6.3）：让全链路（下单 → 支付 → 回调 → 入账 → 退款）
// 在无真实渠道资质的前提下可跑通、可验收。
//
// 设计与真实渠道同构，因此它同时是适配器契约的参考实现：
//   - 回调鉴权用 **HMAC-SHA256 共享密钥**（不是明文 token 相等）：密钥未泄漏时无法伪造，
//     且报文任何一字节被篡改都会导致签名不匹配。与 LIV-01 SRS 回调鉴权同思路，
//     但此处覆盖「报文完整性」而不只是「来源可信」（spec §6.4 抗伪造要求）。
//   - 渠道交易号由渠道侧生成（mock 用随机数），商户不可预测 —— 与支付宝/微信口径一致。
//   - 退款在 mock 中恒成功；真实渠道的失败降级由各渠道实现自行决定。
type mockAdapter struct {
	mu     sync.Mutex
	secret string
	// paid 记录 mock 侧「渠道认为已支付」的交易（查单口径）。
	paid map[string]*mockPaid
}

type mockPaid struct {
	TradeNo string
	Amount  int64
	At      time.Time
}

func newMockAdapter() *mockAdapter {
	return &mockAdapter{secret: "dev-mock-pay-secret-do-not-use-in-prod", paid: map[string]*mockPaid{}}
}

func (m *mockAdapter) Name() string { return ChannelMock }

// SetSecret 设置 mock 共享密钥（由 Service 装配时从配置注入）。
func (m *mockAdapter) SetSecret(s string) {
	if s == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.secret = s
}

func (m *mockAdapter) getSecret() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.secret
}

// Sign 用当前密钥计算报文签名（hex），供 dev 回调端点与测试签名使用。
func (m *mockAdapter) Sign(rawBody []byte) string {
	return MockSign(m.getSecret(), rawBody)
}

// MockSign HMAC-SHA256(secret, rawBody) 的 hex 表示（签名算法即契约，测试与端点共用）。
func MockSign(secret string, rawBody []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(rawBody)
	return hex.EncodeToString(mac.Sum(nil))
}

// CreatePayment 下单：立即返回渠道交易号（真实渠道可能下单后才回交易号，
// 故业务侧不得假设 CreatePayment 一定带回 trade_no）。
func (m *mockAdapter) CreatePayment(_ context.Context, req *CreatePaymentReq) (*CreatePaymentResp, error) {
	if req == nil || req.Amount <= 0 {
		return nil, errcode.ErrPayAmountInvalid
	}
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return nil, err
	}
	tradeNo := "MOCK" + strconv.FormatInt(time.Now().Unix(), 10) + hex.EncodeToString(buf)
	return &CreatePaymentResp{
		ChannelTradeNo: tradeNo,
		Params: map[string]string{
			// 真实渠道此处是签名后的支付表单/调起参数；mock 给出「如何模拟」的提示。
			// 联调时用 POST /api/v1/pay/callback/mock（或 /admin/pay/dev/pay-callback）
			// 携带按 MockSign 计算出的 X-Pay-Signature 模拟渠道回调。
			"channel":  ChannelMock,
			"order_no": req.OrderNo,
			"hint":     "POST 到回调端点并带 X-Pay-Signature=HMAC-SHA256(secret, body) 模拟渠道回调",
			"sign_algo": "HMAC-SHA256(hex)",
		},
	}, nil
}

// QueryPayment 查单：以 mock 侧记录为准（未记录即未支付）。
func (m *mockAdapter) QueryPayment(_ context.Context, orderNo, _ string) (*QueryPaymentResp, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.paid[orderNo]
	if !ok {
		return &QueryPaymentResp{Status: 0}, nil
	}
	return &QueryPaymentResp{Status: 1, ChannelTradeNo: p.TradeNo, Amount: p.Amount}, nil
}

// Refund 退款：mock 恒成功（金额由调用方校验，适配器不重复校验业务规则）。
func (m *mockAdapter) Refund(_ context.Context, req *RefundReq) (*RefundResp, error) {
	if req == nil || req.Amount <= 0 {
		return nil, errcode.ErrPayRefundInvalid
	}
	return &RefundResp{
		Success:        true,
		RefundNo:       "MOCKREFUND" + req.OrderNo,
		RefundedAmount: req.Amount,
	}, nil
}

// mockCallbackBody mock 回调报文（字段名即契约；真实渠道报文结构各不同，
// 但都必须能解析出这五项交给业务侧）。
type mockCallbackBody struct {
	OrderNo        string `json:"order_no"`
	ChannelTradeNo string `json:"channel_trade_no"`
	Amount         int64  `json:"amount"` // 分
	Result         string `json:"result"` // success / fail
}

// VerifyCallback 回调验签（spec §6.4）：
//   - 签名用常量时间比较（hmac.Equal），避免时序侧信道。
//   - 报文解析失败 → 92006；验签失败 → 92005；两者**都不得**让订单或资金发生变化。
//   - result=fail 表示「渠道侧支付失败」：验签通过但你方业务应拒绝推进订单。
func (m *mockAdapter) VerifyCallback(_ context.Context, rawBody []byte, signature string) (*CallbackResult, error) {
	if signature == "" {
		return nil, errcode.ErrPayCallbackSign
	}
	want := m.Sign(rawBody)
	if !hmac.Equal([]byte(want), []byte(signature)) {
		return nil, errcode.ErrPayCallbackSign
	}
	var body mockCallbackBody
	if err := json.Unmarshal(rawBody, &body); err != nil {
		return nil, errcode.ErrPayCallbackInvalid
	}
	if body.OrderNo == "" || body.ChannelTradeNo == "" {
		return nil, errcode.ErrPayCallbackInvalid
	}
	if body.Amount < 0 {
		return nil, errcode.ErrPayCallbackInvalid
	}
	return &CallbackResult{
		OrderNo:        body.OrderNo,
		ChannelTradeNo: body.ChannelTradeNo,
		Amount:         body.Amount,
		Success:        body.Result == "success",
		RawBody:        string(rawBody),
	}, nil
}

// markPaid 记录 mock 侧支付状态（回调处理成功后调用，使查单口径与业务口径一致）。
func (m *mockAdapter) markPaid(orderNo, tradeNo string, amount int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.paid[orderNo] = &mockPaid{TradeNo: tradeNo, Amount: amount, At: time.Now()}
}

// MockBuildCallback 构造并签名一条 mock 回调报文（dev 回调端点与集成测试共用，
// 保证「模拟渠道」只有一处口径，不出现测试签名与端点签名不一致）。
func MockBuildCallback(secret, orderNo, tradeNo string, amount int64, result string) (body []byte, signature string, err error) {
	body, err = json.Marshal(mockCallbackBody{
		OrderNo: orderNo, ChannelTradeNo: tradeNo, Amount: amount, Result: result,
	})
	if err != nil {
		return nil, "", fmt.Errorf("构造 mock 回调报文失败: %w", err)
	}
	return body, MockSign(secret, body), nil
}
