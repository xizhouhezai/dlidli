package pay

import (
	"context"
	"fmt"
)

// wechatAdapter 微信支付渠道适配器 —— **真实渠道的显式换入点**（spec §6.3 / plan §2E.2）。
//
// 现状：**未接入**。与 alipayAdapter 同构：把接入位置固定下来，业务侧零改动。
//
// 接入步骤（按序执行）：
//
//  1. **资质与账户**：微信支付商户号（mchid）+ 商户 API 证书（apiclient_cert.pem /
//     apiclient_key.pem）+ APIv3 密钥。同样需要支付通道资质，当前不具备。
//
//  2. **配置项**：config.Pay 补 wechat 段（mchId / appId / apiV3Key / serialNo /
//     privateKeyPath / notifyURL），敏感值用环境变量：
//     DLIDLI_PAY_WECHAT_MCHID / _APIV3KEY / _PRIVATEKEYPATH。
//
//  3. **签名算法（APIv3）**：请求签名用 `METHOD\nURL\nTIMESTAMP\nNONCE\nBODY\n`
//     拼串后用商户私钥 SHA256withRSA 签名（Authorization 头）；
//     回调验签要用**微信平台证书**验证 `Wechatpay-Signature` 头，
//     签名串为 `timestamp\nnonce\nbody\n`（与支付宝的表单参数拼接完全不同——
//     这正是把验签放进适配器的原因）。平台证书需定期下载轮换。
//
//  4. **下单 CreatePayment**：JSAPI（公众号/小程序）`POST /v3/pay/transactions/jsapi`
//     返回 prepay_id，再用商户私钥签出 `paySign` 等调起参数；App 走 `/v3/pay/transactions/app`。
//     金额字段 `amount.total` **本身就是整数分**（与本地口径一致，无需换算——
//     与支付宝的「元」形成对照，是本项目金额统一用分的有利之处）。
//
//  5. **回调地址备案**：notify_url 公网 HTTPS，端点为 `POST /api/v1/pay/callback/wechat`。
//
//  6. **回调 VerifyCallback**：验 `Wechatpay-Signature` + `Wechatpay-Timestamp`
//     （时间戳偏差超 5 分钟视为重放，拒绝），解密 `resource` 段（AES-256-GCM，
//     APIv3 密钥 + associated_data/nonce），取 out_trade_no / transaction_id /
//     amount.total / trade_state；trade_state=SUCCESS 才算成功。
//
//  7. **查单 QueryPayment**：`GET /v3/pay/transactions/out-trade-no/{out_trade_no}?mchid=`。
//
//  8. **退款 Refund**：`POST /v3/refund/domestic/refunds`，out_refund_no 用 OrderNo
//     保证渠道侧退款幂等。
//
//  9. **对账文件（换入点，本卡不做）**：`GET /v3/bill/tradebill` 下载账单，
//     解析后与本地 pay_recon_daily 比对。
type wechatAdapter struct{}

func newWechatAdapter() *wechatAdapter { return &wechatAdapter{} }

func (w *wechatAdapter) Name() string { return ChannelWeChat }

// CreatePayment 微信下单（未接入：返回 92004）。
func (w *wechatAdapter) CreatePayment(_ context.Context, _ *CreatePaymentReq) (*CreatePaymentResp, error) {
	// TODO(M4-PAY 后续卡): 见本文件头部「接入步骤 2/3/4」。
	// amount.total 直接取 req.Amount（分），无需换算。
	return nil, notImplemented()
}

// QueryPayment 微信查单（未接入）。
func (w *wechatAdapter) QueryPayment(_ context.Context, _, _ string) (*QueryPaymentResp, error) {
	// TODO(M4-PAY 后续卡): GET /v3/pay/transactions/out-trade-no/{out_trade_no}，见接入步骤 7。
	return nil, notImplemented()
}

// Refund 微信退款（未接入）。
func (w *wechatAdapter) Refund(_ context.Context, _ *RefundReq) (*RefundResp, error) {
	// TODO(M4-PAY 后续卡): POST /v3/refund/domestic/refunds，out_refund_no=OrderNo，
	// 见接入步骤 8。
	return nil, notImplemented()
}

// VerifyCallback 微信回调验签（未接入）。
func (w *wechatAdapter) VerifyCallback(_ context.Context, _ []byte, _ string) (*CallbackResult, error) {
	// TODO(M4-PAY 后续卡): 平台证书验签 + AES-256-GCM 解密 resource，见接入步骤 6。
	// 未接入时必须返回错误（见 alipayAdapter.VerifyCallback 的同类说明：
	// 返回成功即等于开放伪造支付结果的能力）。
	return nil, notImplemented()
}

// String 便于日志与调试输出渠道状态。
func (w *wechatAdapter) String() string {
	return fmt.Sprintf("wechat(not implemented: %v)", errNotImplemented)
}
