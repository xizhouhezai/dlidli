package pay

import (
	"context"
	"fmt"
)

// alipayAdapter 支付宝渠道适配器 —— **真实渠道的显式换入点**（spec §6.3 / plan §2E.2）。
//
// 现状：**未接入**。本文件把接入所需的一切位置固定下来（接口签名、配置项、签名算法、
// 回调地址、查单/退款/对账文件入口），使后续接入真实渠道时**只改本文件**——
// 业务侧（订单/回调/对账服务）与表结构零改动。
//
// 接入步骤（按序执行，每步都可独立验证）：
//
//  1. **资质与账户**：签约支付宝开放平台（需要增值电信/支付通道资质），拿到
//     APPID、应用私钥、支付宝公钥。资质是本项目当前不具备的前置条件
//     （monetization spec §2.5）。
//
//  2. **配置项**：在 config.Pay 下补 alipay 段（appId / privateKey / alipayPublicKey /
//     gatewayURL / notifyURL / signType），以环境变量注入敏感值，禁止落 YAML 明文：
//     DLIDLI_PAY_ALIPAY_APPID / _PRIVATEKEY / _ALIPAYPUBLICKEY。
//     装配点见 router 中 pay.NewService 的 NewServiceWithConfig。
//
//  3. **签名算法**：RSA2（SHA256withRSA）。请求参数按 key 字典序拼接
//     `k1=v1&k2=v2`（剔除 sign 与空值）后用应用私钥签名，Base64 后作为 sign 参数；
//     回调验签用支付宝公钥验证**原始表单参数**（不是整个 body 字符串，与 mock 的 HMAC
//     覆盖原始 body 不同——这是各渠道的固有差异，故 VerifyCallback 放在适配器内实现）。
//
//  4. **下单 CreatePayment**：调用 `alipay.trade.page.pay`（PC）或
//     `alipay.trade.wap.pay`（H5）/ `alipay.trade.app.pay`（App）：
//     out_trade_no 用 OrderNo、total_amount 用 **Amount/100 的两位小数字符串**
//     （支付宝以「元」为单位，这是唯一需要把「分」换算成元的位置；
//     **换算只在适配器边界发生**，业务侧与 DB 始终是整数分 —— 换算后即为字符串比较，
//     不得引入 float64 中间量）。
//
//  5. **回调地址备案**：notify_url 必须是公网可达的 HTTPS 地址且已在开放平台备案。
//     本项目端点为 `POST /api/v1/pay/callback/alipay`。备案是运营动作，不在代码内。
//
//  6. **回调 VerifyCallback**：解析 `application/x-www-form-urlencoded` 表单，
//     取 out_trade_no / trade_no / total_amount / trade_status，
//     验签通过且 trade_status ∈ {TRADE_SUCCESS, TRADE_FINISHED} 才算支付成功；
//     把 total_amount（元字符串）**转换回分**填进 CallbackResult.Amount。
//
//  7. **查单 QueryPayment**：`alipay.trade.query`，用 out_trade_no 或 trade_no 查询。
//     真实渠道的支付结果以**渠道侧为准**（回调可能丢失），故必须实现本方法。
//
//  8. **退款 Refund**：`alipay.trade.refund`，out_request_no 用 OrderNo 保证退款幂等
//     （同一 out_request_no 重复退款不会重复出款——与本地 cash_log.uk_biz_no 是两道独立的闸）。
//
//  9. **对账文件（换入点，本卡不做）**：`alipay.data.dataservice.bill.downloadurl.query`
//     获取日账单下载链接，解析后与本地 pay_recon_daily 逐笔比对。
//     解析入口挂在 ReconService，见 plan §2E.6 末尾的边界说明。
type alipayAdapter struct{}

func newAlipayAdapter() *alipayAdapter { return &alipayAdapter{} }

func (a *alipayAdapter) Name() string { return ChannelAlipay }

// CreatePayment 支付宝下单（未接入：返回 92004，不返回伪造成功）。
func (a *alipayAdapter) CreatePayment(_ context.Context, _ *CreatePaymentReq) (*CreatePaymentResp, error) {
	// TODO(M4-PAY 后续卡): 见本文件头部「接入步骤 2/3/4」。
	// 接入后：构造 biz_content(out_trade_no/total_amount/subject/product_code) →
	// RSA2 签名 → 拼装网关跳转表单/返回 trade_no。凭据从 config.Pay.Alipay 读取。
	return nil, notImplemented()
}

// QueryPayment 支付宝查单（未接入）。
func (a *alipayAdapter) QueryPayment(_ context.Context, _, _ string) (*QueryPaymentResp, error) {
	// TODO(M4-PAY 后续卡): alipay.trade.query，见接入步骤 7。
	// 真实渠道必须实现本方法：回调可能丢失，支付结果以渠道侧为准。
	return nil, notImplemented()
}

// Refund 支付宝退款（未接入）。
func (a *alipayAdapter) Refund(_ context.Context, _ *RefundReq) (*RefundResp, error) {
	// TODO(M4-PAY 后续卡): alipay.trade.refund，out_request_no=OrderNo 保证渠道侧退款幂等，
	// 见接入步骤 8。
	return nil, notImplemented()
}

// VerifyCallback 支付宝回调验签（未接入）。
func (a *alipayAdapter) VerifyCallback(_ context.Context, _ []byte, _ string) (*CallbackResult, error) {
	// TODO(M4-PAY 后续卡): 支付宝公钥验签原始表单参数，trade_status 判定成功，
	// total_amount「元」→「分」转换，见接入步骤 6。
	//
	// 注意：**未接入时本方法必须返回错误**。若这里返回 nil,nil，回调链会把
	// 一条未验签的报文当成合法回调推进订单，等于给任意人伪造支付结果的能力。
	return nil, notImplemented()
}

// String 便于日志与调试输出渠道状态。
func (a *alipayAdapter) String() string {
	return fmt.Sprintf("alipay(not implemented: %v)", errNotImplemented)
}
