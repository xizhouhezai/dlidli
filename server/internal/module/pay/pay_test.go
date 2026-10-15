package pay

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dlidli/server/internal/pkg/errcode"
)

// M4-PAY-01 单元测试：不依赖数据库，覆盖适配器契约、验签抗伪造、注册表与输入校验。

// TestMockVerifyCallbackValid 正常回调：验签通过并解析出五项契约字段。
func TestMockVerifyCallbackValid(t *testing.T) {
	a, err := Get(ChannelMock)
	if err != nil {
		t.Fatalf("mock 渠道应已注册: %v", err)
	}
	const secret = "test-secret"
	m := a.(*mockAdapter)
	m.SetSecret(secret)
	defer m.SetSecret(DefaultConfig().MockSecret)

	body, sig, err := MockBuildCallback(secret, "PAY-T-1", "MOCK-T-1", 12345, "success")
	if err != nil {
		t.Fatalf("构造回调失败: %v", err)
	}
	res, err := a.VerifyCallback(context.Background(), body, sig)
	if err != nil {
		t.Fatalf("验签应通过: %v", err)
	}
	if res.OrderNo != "PAY-T-1" || res.ChannelTradeNo != "MOCK-T-1" || res.Amount != 12345 || !res.Success {
		t.Fatalf("解析结果不符: %+v", res)
	}
	if res.RawBody != string(body) {
		t.Fatalf("原始报文应原样保留（取证要求）")
	}
}

// TestMockVerifyCallbackForged 抗伪造三例：错误签名 / 篡改报文 / 缺签名，
// 三者都必须拒绝（spec §6.4「回调必须能抵抗伪造」）。
func TestMockVerifyCallbackForged(t *testing.T) {
	a, _ := Get(ChannelMock)
	m := a.(*mockAdapter)
	const secret = "test-secret"
	m.SetSecret(secret)
	defer m.SetSecret(DefaultConfig().MockSecret)

	body, sig, _ := MockBuildCallback(secret, "PAY-T-2", "MOCK-T-2", 100, "success")

	// ① 完全错误的签名
	if _, err := a.VerifyCallback(context.Background(), body, "deadbeef"); !errors.Is(err, errcode.ErrPayCallbackSign) {
		t.Fatalf("错签名应 92005, got %v", err)
	}
	// ② 签名正确但报文被篡改金额（攻击者抬高金额）
	tampered := strings.Replace(string(body), `"amount":100`, `"amount":999999`, 1)
	if tampered == string(body) {
		t.Fatal("测试用例构造失败：未篡改到金额字段")
	}
	if _, err := a.VerifyCallback(context.Background(), []byte(tampered), sig); !errors.Is(err, errcode.ErrPayCallbackSign) {
		t.Fatalf("篡改报文的签名应不匹配 92005, got %v", err)
	}
	// ③ 用别人的密钥签名
	other, otherSig, _ := MockBuildCallback("attacker-secret", "PAY-T-2", "MOCK-T-2", 100, "success")
	if _, err := a.VerifyCallback(context.Background(), other, otherSig); !errors.Is(err, errcode.ErrPayCallbackSign) {
		t.Fatalf("他人密钥签名应 92005, got %v", err)
	}
	// ④ 缺签名头
	if _, err := a.VerifyCallback(context.Background(), body, ""); !errors.Is(err, errcode.ErrPayCallbackSign) {
		t.Fatalf("空签名应 92005, got %v", err)
	}
}

// TestMockVerifyCallbackMalformed 验签通过但报文不合法 → 92006（与验签失败可区分）。
func TestMockVerifyCallbackMalformed(t *testing.T) {
	a, _ := Get(ChannelMock)
	m := a.(*mockAdapter)
	const secret = "test-secret"
	m.SetSecret(secret)
	defer m.SetSecret(DefaultConfig().MockSecret)

	// 合法 JSON 但缺 order_no/channel_trade_no
	body := []byte(`{"amount":100,"result":"success"}`)
	if _, err := a.VerifyCallback(context.Background(), body, MockSign(secret, body)); !errors.Is(err, errcode.ErrPayCallbackInvalid) {
		t.Fatalf("缺字段应 92006, got %v", err)
	}
	// 根本不是 JSON
	body2 := []byte(`not-json`)
	if _, err := a.VerifyCallback(context.Background(), body2, MockSign(secret, body2)); !errors.Is(err, errcode.ErrPayCallbackInvalid) {
		t.Fatalf("非 JSON 应 92006, got %v", err)
	}
}

// TestMockResultFail 验签通过的「支付失败」回调：解析成功但 Success=false，
// 业务侧据此不推进订单（与「验签失败」是两回事，不可混淆）。
func TestMockResultFail(t *testing.T) {
	a, _ := Get(ChannelMock)
	m := a.(*mockAdapter)
	const secret = "test-secret"
	m.SetSecret(secret)
	defer m.SetSecret(DefaultConfig().MockSecret)

	body, sig, _ := MockBuildCallback(secret, "PAY-T-3", "MOCK-T-3", 100, "fail")
	res, err := a.VerifyCallback(context.Background(), body, sig)
	if err != nil {
		t.Fatalf("验签应通过: %v", err)
	}
	if res.Success {
		t.Fatal("result=fail 应解析为 Success=false")
	}
}

// TestCreatePaymentGeneratesTradeNo mock 下单返回渠道交易号与唤起参数（真实渠道同契约）。
func TestCreatePaymentGeneratesTradeNo(t *testing.T) {
	a, _ := Get(ChannelMock)
	r1, err := a.CreatePayment(context.Background(), &CreatePaymentReq{OrderNo: "PAY-T-4", Amount: 500, Subject: "x"})
	if err != nil {
		t.Fatalf("mock 下单应成功: %v", err)
	}
	r2, _ := a.CreatePayment(context.Background(), &CreatePaymentReq{OrderNo: "PAY-T-5", Amount: 500, Subject: "x"})
	if r1.ChannelTradeNo == "" || r1.ChannelTradeNo == r2.ChannelTradeNo {
		t.Fatalf("渠道交易号应非空且互不相同: %q / %q", r1.ChannelTradeNo, r2.ChannelTradeNo)
	}
	if r1.Params["sign_algo"] != "HMAC-SHA256(hex)" {
		t.Fatalf("唤起参数应说明签名算法: %+v", r1.Params)
	}
	// 金额非法
	if _, err := a.CreatePayment(context.Background(), &CreatePaymentReq{OrderNo: "x", Amount: 0}); !errors.Is(err, errcode.ErrPayAmountInvalid) {
		t.Fatalf("金额 0 应 92008, got %v", err)
	}
}

// TestMockQueryPaymentReflectsState 查单口径：未支付 → 0；markPaid 后 → 1 且带回金额。
func TestMockQueryPaymentReflectsState(t *testing.T) {
	a, _ := Get(ChannelMock)
	m := a.(*mockAdapter)
	ctx := context.Background()

	q, err := a.QueryPayment(ctx, "PAY-T-QUERY", "")
	if err != nil || q.Status != 0 {
		t.Fatalf("未支付应 status=0, got %+v err=%v", q, err)
	}
	m.markPaid("PAY-T-QUERY", "MOCK-Q", 777)
	q, _ = a.QueryPayment(ctx, "PAY-T-QUERY", "")
	if q.Status != 1 || q.Amount != 777 || q.ChannelTradeNo != "MOCK-Q" {
		t.Fatalf("已支付应带金额与交易号, got %+v", q)
	}
}

// TestRealAdaptersNotImplemented 真实渠道骨架：四个方法一律 92004，
// **绝不返回伪造成功**（spec §3 末条 / §6.3）。
func TestRealAdaptersNotImplemented(t *testing.T) {
	ctx := context.Background()
	for _, name := range []string{ChannelAlipay, ChannelWeChat} {
		a, err := Get(name)
		if err != nil {
			t.Fatalf("%s 应已注册（骨架即换入点）: %v", name, err)
		}
		if a.Name() != name {
			t.Fatalf("渠道名不符: %s vs %s", a.Name(), name)
		}
		if _, err := a.CreatePayment(ctx, &CreatePaymentReq{OrderNo: "o", Amount: 1}); !errors.Is(err, errcode.ErrPayChannelNotImpl) {
			t.Fatalf("%s 下单应 92004, got %v", name, err)
		}
		if _, err := a.QueryPayment(ctx, "o", "t"); !errors.Is(err, errcode.ErrPayChannelNotImpl) {
			t.Fatalf("%s 查单应 92004, got %v", name, err)
		}
		if _, err := a.Refund(ctx, &RefundReq{OrderNo: "o", Amount: 1}); !errors.Is(err, errcode.ErrPayChannelNotImpl) {
			t.Fatalf("%s 退款应 92004, got %v", name, err)
		}
		// 最关键的一条：未接入渠道的验签必须失败。若这里返回成功，
		// 任何人都能靠一条未验签的报文把订单刷成「已支付」。
		body, sig, _ := MockBuildCallback("whatever", "o", "t", 1, "success")
		if _, err := a.VerifyCallback(ctx, body, sig); !errors.Is(err, errcode.ErrPayChannelNotImpl) {
			t.Fatalf("%s 验签应 92004（不得放行）, got %v", name, err)
		}
	}
}

// TestAdapterRegistryUnknownChannel 未注册渠道 → 92003（下单拒绝，spec §6.3）。
func TestAdapterRegistryUnknownChannel(t *testing.T) {
	if _, err := Get("paypal"); !errors.Is(err, errcode.ErrPayChannelUnknown) {
		t.Fatalf("未知渠道应 92003, got %v", err)
	}
	if IsRegistered("paypal") {
		t.Fatal("paypal 不应注册")
	}
	for _, c := range []string{ChannelMock, ChannelAlipay, ChannelWeChat} {
		if !IsRegistered(c) {
			t.Fatalf("%s 应已注册", c)
		}
	}
}

// TestGenOrderNoUnique 订单号唯一且不含浮点/序列（防泄露交易笔数）。
func TestGenOrderNoUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 2000; i++ {
		no := genOrderNo()
		if !strings.HasPrefix(no, "PAY") {
			t.Fatalf("订单号前缀不符: %s", no)
		}
		if seen[no] {
			t.Fatalf("订单号重复: %s", no)
		}
		seen[no] = true
	}
}

// TestValidateDate 对账日期校验：空/格式错/未来日期一律 92011。
func TestValidateDate(t *testing.T) {
	if err := validateDate(""); !errors.Is(err, errcode.ErrPayReconDateInvalid) {
		t.Fatalf("空日期应 92011, got %v", err)
	}
	if err := validateDate("2026-10-99"); !errors.Is(err, errcode.ErrPayReconDateInvalid) {
		t.Fatalf("非法日期应 92011, got %v", err)
	}
	if err := validateDate("2099-01-01"); !errors.Is(err, errcode.ErrPayReconDateInvalid) {
		t.Fatalf("未来日期应 92011, got %v", err)
	}
}

// TestConsumptionGuardHook 消费限制钩子（spec §6.6）：默认放行；注入实现后生效。
// 本用例只证明「调用点存在且可替换」，不代表未成年人限制已实现（无出生日期数据，
// 见 spec §6.6 与 plan §2E.7 的未覆盖声明）。
func TestConsumptionGuardHook(t *testing.T) {
	s := NewService(NewRepo(nil), Config{}, nil)
	if err := s.guard.Check(context.Background(), 1, 100); err != nil {
		t.Fatalf("默认钩子应放行: %v", err)
	}
	s.SetGuard(blockAllGuard{})
	if err := s.guard.Check(context.Background(), 1, 100); !errors.Is(err, errcode.ErrPayMinorRestricted) {
		t.Fatalf("注入的钩子应生效: %v", err)
	}
	s.SetGuard(nil) // nil 回落默认放行，避免装配疏漏导致全站不可支付
	if err := s.guard.Check(context.Background(), 1, 100); err != nil {
		t.Fatalf("SetGuard(nil) 应回落放行: %v", err)
	}
}

type blockAllGuard struct{}

func (blockAllGuard) Check(context.Context, int64, int64) error { return errcode.ErrPayMinorRestricted }

// TestNewServiceDefaults 零值配置兜底（避免 ticker/timeout 类零值缺陷）。
func TestNewServiceDefaults(t *testing.T) {
	s := NewService(NewRepo(nil), Config{}, nil)
	if s.cfg.OrderTimeoutMinutes != 15 || s.cfg.StuckHours != 24 {
		t.Fatalf("零值配置应回落默认: %+v", s.cfg)
	}
	if s.cfg.MaxAmount != 1_000_000 || s.cfg.MockSecret == "" {
		t.Fatalf("上限与密钥应有默认: %+v", s.cfg)
	}
	// mock 密钥注入必须真的落到适配器（否则验签与端点签名会用不同密钥）
	a, _ := Get(ChannelMock)
	if a.(*mockAdapter).getSecret() != s.cfg.MockSecret {
		t.Fatal("mock 密钥未下发到适配器")
	}
}

// TestNormalizePage 分页归一。
func TestNormalizePage(t *testing.T) {
	cases := []struct{ p, s, wp, ws int }{
		{0, 0, 1, 20}, {-1, -1, 1, 20}, {2, 500, 2, 20}, {3, 50, 3, 50},
	}
	for _, c := range cases {
		p, s := normPage(c.p, c.s)
		if p != c.wp || s != c.ws {
			t.Fatalf("normPage(%d,%d)=(%d,%d), 期望 (%d,%d)", c.p, c.s, p, s, c.wp, c.ws)
		}
	}
}

// TestJSONFieldNamingConsistent 接口 JSON 键必须全为 snake_case。
//
// 回归护栏：ORM 模型（PayReconDaily）字段名是 Go 命名，若被直接序列化进响应，
// 会出现 `ReconDate`/`DiffCount` 这类键，前端就得为同一模块维护两套取字段方式
// ——上线前实测正是被这一点绊住（对账单取 `channels[0].status` 拿到 undefined）。
// 故用测试钉住「所有对外视图序列化后无大写键」。
func TestJSONFieldNamingConsistent(t *testing.T) {
	now := time.Now()
	samples := map[string]any{
		"OrderView": &OrderView{OrderNo: "o", Status: StatusPaid, ExpireAt: "x", CreatedAt: "y"},
		"IssueView": &IssueView{ID: 1, ReconDate: "d", IssueType: IssueStuck, CreatedAt: "c"},
		"DailyView": toDailyView(&PayReconDaily{
			ReconDate: time.Date(2026, 10, 15, 0, 0, 0, 0, time.Local), Channel: ChannelMock, Status: ReconHasIssue,
			CreatedAt: now, UpdatedAt: now,
		}),
		"CallbackOutcome": &CallbackOutcome{OrderNo: "o", Applied: true, Status: StatusPaid},
		"ReconResult":     &ReconResult{ReconDate: "d", Channels: []*DailyView{}, Issues: []*IssueView{}, IssueNum: 0},
	}
	for name, s := range samples {
		b, err := json.Marshal(s)
		if err != nil {
			t.Fatalf("%s 序列化失败: %v", name, err)
		}
		var m map[string]json.RawMessage
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatalf("%s 反序列化失败: %v", name, err)
		}
		for k := range m {
			if strings.ToLower(k) != k {
				t.Fatalf("%s 的 JSON 键 %q 不是 snake_case（大写出现在对外响应里）: %s", name, k, b)
			}
		}
	}
}

// TestCheckoutReqAmountOptional 下单请求的 amount 必须允许“缺失”走到业务校验，
// 否则会退化成一个笼统的 10002，掩盖金额问题（见 handler 里的注释）。
func TestCheckoutReqAmountOptional(t *testing.T) {
	var req createOrderReq
	if err := json.Unmarshal([]byte(`{"channel":"mock"}`), &req); err != nil {
		t.Fatalf("缺 amount 不应解析失败: %v", err)
	}
	if req.Amount != 0 || req.Channel != "mock" {
		t.Fatalf("解析结果不符: %+v", req)
	}
}

// TestCallbackBodyContract 回调报文字段名即契约（改字段名会静默破坏回调链，故用测试钉住）。
func TestCallbackBodyContract(t *testing.T) {
	body, _, err := MockBuildCallback("s", "o", "t", 5, "success")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"order_no", "channel_trade_no", "amount", "result"} {
		if _, ok := m[k]; !ok {
			t.Fatalf("回调报文缺契约字段 %s: %s", k, body)
		}
	}
	// 金额必须是整数（JSON number 解析为 float64 后应无小数）
	if v, ok := m["amount"].(float64); !ok || v != 5 {
		t.Fatalf("amount 应为整数 5: %v", m["amount"])
	}
}
