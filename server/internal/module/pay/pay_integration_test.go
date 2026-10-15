package pay

import (
	"context"
	"errors"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/dlidli/server/internal/pkg/errcode"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// M4-PAY-01 集成测试（真实 MySQL）：覆盖验收核心 —— 全链路、幂等、伪造、并发、
// 状态机、退款、对账异常。未设置 DLIDLI_TEST_DSN 时跳过（沿用 live 包既有口径）。

func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("DLIDLI_TEST_DSN")
	if dsn == "" {
		t.Skip("DLIDLI_TEST_DSN 未设置，跳过 pay 集成测试")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), TranslateError: true,
	})
	if err != nil {
		t.Fatalf("已设置 DLIDLI_TEST_DSN 但无法连接测试库: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("获取底层连接失败: %v", err)
	}
	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("测试库 ping 失败: %v", err)
	}
	return db
}

// 测试用户段 92600xxx：刻意避开 92000xxx（既有 daily_login 硬币流水测试用过该段，
// 复用会让「支付不得写虚拟币」的断言被陈年数据污染，属于测试自身的坑）。
const (
	testUIDBase = int64(92600000)
	testUIDEnd  = int64(92699999)
)

func newTestSvc(t *testing.T, db *gorm.DB) *Service {
	t.Helper()
	cfg := DefaultConfig()
	cfg.MockSecret = "test-integration-secret"
	cfg.OrderTimeoutMinutes = 15
	return NewService(NewRepo(db), cfg, zap.NewNop())
}

// seedUser 造测试用户。昵称必须带 uid：`user.uk_nickname` 唯一，
// 同一昵称会让第二个测试用户插入失败（本卡实测踩到）。
func seedUser(t *testing.T, db *gorm.DB, uid int64) {
	t.Helper()
	if err := db.Exec("INSERT INTO `user` (id, nickname, status, created_at, updated_at) VALUES (?, ?, 0, NOW(), NOW())",
		uid, "paytest-"+strconv.FormatInt(uid, 10)).Error; err != nil {
		t.Fatalf("造用户失败: %v", err)
	}
}

// cleanupPay 清理 pay 域测试数据。
//
// 顺序有讲究：回调日志与对账异常都按 order_no 关联订单，必须在删订单**之前**清掉，
// 否则残留让「验签失败日志应恰 1 条」这类计数断言失败（本卡实测踩到）。
// 硬币/金瓜子一并清理：它们的残留（如 daily_login 赠币）会让「支付不写虚拟币」的
// 断言变成假阳性来源，清空后该断言才真正在测「本次支付流程没写虚拟币」。
func cleanupPay(t *testing.T, db *gorm.DB) {
	t.Helper()
	_ = db.Exec(`DELETE FROM pay_callback_log WHERE order_no IN
		(SELECT order_no FROM pay_order WHERE user_id BETWEEN ? AND ?)`, testUIDBase, testUIDEnd).Error
	_ = db.Exec(`DELETE FROM pay_recon_issue WHERE order_no IN
		(SELECT order_no FROM pay_order WHERE user_id BETWEEN ? AND ?)`, testUIDBase, testUIDEnd).Error
	_ = db.Exec("DELETE FROM cash_log WHERE user_id BETWEEN ? AND ?", testUIDBase, testUIDEnd).Error
	_ = db.Exec("DELETE FROM cash_account WHERE user_id BETWEEN ? AND ?", testUIDBase, testUIDEnd).Error
	_ = db.Exec("DELETE FROM pay_order WHERE user_id BETWEEN ? AND ?", testUIDBase, testUIDEnd).Error
	_ = db.Exec("DELETE FROM coin_log WHERE user_id BETWEEN ? AND ?", testUIDBase, testUIDEnd).Error
	_ = db.Exec("DELETE FROM melon_seed_log WHERE user_id BETWEEN ? AND ?", testUIDBase, testUIDEnd).Error
	_ = db.Exec("DELETE FROM melon_seed_account WHERE user_id BETWEEN ? AND ?", testUIDBase, testUIDEnd).Error
	_ = db.Exec("DELETE FROM `user` WHERE id BETWEEN ? AND ?", testUIDBase, testUIDEnd).Error
	// 日对账单按 (日期,渠道) 唯一，测试只写 mock 渠道的当日行，清掉避免跨测试串味。
	_ = db.Exec("DELETE FROM pay_recon_daily WHERE channel = ? AND recon_date = CURDATE()", ChannelMock).Error
}

// newOrder 建一张待支付订单（金额由调用方给定）。
func newOrder(t *testing.T, svc *Service, uid, amount int64) *OrderView {
	t.Helper()
	v, err := svc.CreateOrder(context.Background(), &CreateOrderReq{
		UserID: uid, Amount: amount, Channel: ChannelMock, Subject: "测试商品", ClientIP: "127.0.0.1",
	})
	if err != nil {
		t.Fatalf("下单失败: %v", err)
	}
	return v
}

// TestE2EFullChain 全链路（验收项 ①）：下单 → mock 支付 → 回调 → 订单已支付 + 现金流水正确。
// DB 逐条核对：订单状态/渠道交易号/paid_at、账户余额、流水（delta/balance_after/biz_no）。
func TestE2EFullChain(t *testing.T) {
	db := testDB(t)
	cleanupPay(t, db)
	t.Cleanup(func() { cleanupPay(t, db) })

	svc := newTestSvc(t, db)
	uid := testUIDBase + 1
	seedUser(t, db, uid)

	const amount = int64(1999) // 19.99 元，用分
	o := newOrder(t, svc, uid, amount)
	if o.Status != StatusPending {
		t.Fatalf("新订单应待支付, got %d", o.Status)
	}
	if o.Amount != amount {
		t.Fatalf("金额应为整数分 %d, got %d", amount, o.Amount)
	}

	// 模拟渠道回调（走与真实渠道同一条链）
	res, err := svc.DevCallback(o.OrderNo, amount, "success")
	if err != nil {
		t.Fatalf("回调失败: %v", err)
	}
	if !res.Applied || res.Status != StatusPaid {
		t.Fatalf("回调应生效并置已支付: %+v", res)
	}

	// DB 逐条核对：订单
	var dbOrder PayOrder
	if err := db.Where("order_no = ?", o.OrderNo).First(&dbOrder).Error; err != nil {
		t.Fatal(err)
	}
	if dbOrder.Status != StatusPaid {
		t.Fatalf("DB 订单状态应 1, got %d", dbOrder.Status)
	}
	if dbOrder.ChannelTradeNo == nil || *dbOrder.ChannelTradeNo == "" {
		t.Fatal("DB 订单应记录渠道交易号")
	}
	if dbOrder.PaidAt == nil {
		t.Fatal("DB 订单应记录 paid_at")
	}
	if dbOrder.Amount != amount {
		t.Fatalf("DB 金额应 %d, got %d", amount, dbOrder.Amount)
	}

	// DB 逐条核对：余额与流水
	bal, err := svc.CashBalance(uid)
	if err != nil {
		t.Fatal(err)
	}
	if bal != amount {
		t.Fatalf("现金余额应 %d, got %d", amount, bal)
	}
	logs, err := NewRepo(db).CashLogsByUser(uid)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 {
		t.Fatalf("现金流水应恰 1 条, got %d", len(logs))
	}
	l := logs[0]
	if l.Delta != amount || l.BalanceAfter != amount || l.BizType != CashBizPay {
		t.Fatalf("流水字段不符: %+v", l)
	}
	if l.BizNo != "pay:"+o.OrderNo || l.OrderNo != o.OrderNo {
		t.Fatalf("流水业务单号不符: %+v", l)
	}

	// 现金与虚拟币严格分离（spec §6.1）：硬币/金瓜子流水**零新增**
	var coinCnt, melonCnt int64
	db.Model(&coinLogProbe{}).Where("user_id = ?", uid).Count(&coinCnt)
	db.Model(&melonLogProbe{}).Where("user_id = ?", uid).Count(&melonCnt)
	if coinCnt != 0 || melonCnt != 0 {
		t.Fatalf("支付不得写虚拟币账户：coin_log=%d melon_seed_log=%d", coinCnt, melonCnt)
	}
}

// coinLogProbe / melonLogProbe 只读探针表（验证「支付不写虚拟币」这一硬约束）。
type coinLogProbe struct{}

func (coinLogProbe) TableName() string { return "coin_log" }

type melonLogProbe struct{}

func (melonLogProbe) TableName() string { return "melon_seed_log" }

type melonAcctProbe struct{}

func (melonAcctProbe) TableName() string { return "melon_seed_account" }

// TestCallbackIdempotent 幂等（验收项 ②）：同一回调重放 N 次 → 订单状态与流水只有一份。
func TestCallbackIdempotent(t *testing.T) {
	db := testDB(t)
	cleanupPay(t, db)
	t.Cleanup(func() { cleanupPay(t, db) })

	svc := newTestSvc(t, db)
	uid := testUIDBase + 2
	seedUser(t, db, uid)
	o := newOrder(t, svc, uid, 500)

	const replays = 10
	appliedCnt := 0
	for i := 0; i < replays; i++ {
		res, err := svc.DevCallback(o.OrderNo, 500, "success")
		if err != nil {
			t.Fatalf("第 %d 次回调不应报错（重放属常态）: %v", i+1, err)
		}
		if res.Applied {
			appliedCnt++
		}
		if res.Status != StatusPaid {
			t.Fatalf("第 %d 次回调后状态应为已支付, got %d", i+1, res.Status)
		}
	}
	if appliedCnt != 1 {
		t.Fatalf("10 次重放应恰好 1 次生效, got %d", appliedCnt)
	}
	// 流水只有一份
	logs, _ := NewRepo(db).CashLogsByUser(uid)
	if len(logs) != 1 {
		t.Fatalf("重放后流水应恰 1 条, got %d", len(logs))
	}
	// 余额只加一次
	bal, _ := svc.CashBalance(uid)
	if bal != 500 {
		t.Fatalf("余额应 500（只入账一次）, got %d", bal)
	}
	// 回调日志应有 10 条（每次回调都留痕，取证要求）
	var cbCnt int64
	db.Model(&PayCallbackLog{}).Where("order_no = ? AND sign_ok = 1", o.OrderNo).Count(&cbCnt)
	if cbCnt != replays {
		t.Fatalf("回调日志应留 %d 条, got %d", replays, cbCnt)
	}
}

// TestForgedCallbackRejected 伪造回调（验收项 ③）：错签名 → 拒绝且订单与流水零变化。
func TestForgedCallbackRejected(t *testing.T) {
	db := testDB(t)
	cleanupPay(t, db)
	t.Cleanup(func() { cleanupPay(t, db) })

	svc := newTestSvc(t, db)
	uid := testUIDBase + 3
	seedUser(t, db, uid)
	o := newOrder(t, svc, uid, 800)

	// 用攻击者自己的密钥签一条「支付成功」报文
	body, sig, err := MockBuildCallback("attacker-secret", o.OrderNo, "FAKE-TRADE", 800, "success")
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.HandleCallback(context.Background(), ChannelMock, body, sig)
	if !errors.Is(err, errcode.ErrPayCallbackSign) {
		t.Fatalf("伪造回调应 92005, got %v", err)
	}
	// 订单必须纹丝不动
	var dbOrder PayOrder
	db.Where("order_no = ?", o.OrderNo).First(&dbOrder)
	if dbOrder.Status != StatusPending {
		t.Fatalf("伪造回调后订单状态不得变化, got %d", dbOrder.Status)
	}
	if dbOrder.ChannelTradeNo != nil {
		t.Fatal("伪造回调后不得写入渠道交易号")
	}
	if dbOrder.PaidAt != nil {
		t.Fatal("伪造回调后不得写入 paid_at")
	}
	// 余额与流水必须为零
	bal, _ := svc.CashBalance(uid)
	if bal != 0 {
		t.Fatalf("伪造回调后余额应为 0, got %d", bal)
	}
	logs, _ := NewRepo(db).CashLogsByUser(uid)
	if len(logs) != 0 {
		t.Fatalf("伪造回调不得产生流水, got %d", len(logs))
	}
	// 验签失败也必须留痕（否则伪造行为不可观测）
	var failCnt int64
	db.Model(&PayCallbackLog{}).Where("order_no = ? AND sign_ok = 0", o.OrderNo).Count(&failCnt)
	if failCnt != 1 {
		t.Fatalf("验签失败应落 1 条 sign_ok=0 日志, got %d", failCnt)
	}
}

// TestConcurrentCallbackExactlyOnce 并发（验收项 ④，集成测试证据）：
// 同一订单 20 并发回调 → 恰好一次生效，余额与流水只有一份。
func TestConcurrentCallbackExactlyOnce(t *testing.T) {
	db := testDB(t)
	cleanupPay(t, db)
	t.Cleanup(func() { cleanupPay(t, db) })

	svc := newTestSvc(t, db)
	uid := testUIDBase + 4
	seedUser(t, db, uid)
	const amount = int64(3000)
	o := newOrder(t, svc, uid, amount)

	// 所有并发请求携带**同一条**报文（同一渠道交易号 = 同一笔支付），
	// 这正是渠道重复投递的真实形态。
	body, sig, err := MockBuildCallback(svc.MockSecret(), o.OrderNo, "MOCK-CONCURRENT-1", amount, "success")
	if err != nil {
		t.Fatal(err)
	}

	const workers = 20
	var wg sync.WaitGroup
	var mu sync.Mutex
	applied, idem, failed := 0, 0, 0
	var firstErr error
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := svc.HandleCallback(context.Background(), ChannelMock, body, sig)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err != nil:
				failed++
				if firstErr == nil {
					firstErr = err
				}
			case res.Applied:
				applied++
			default:
				idem++
			}
		}()
	}
	wg.Wait()

	if failed != 0 {
		t.Fatalf("并发回调不应报错（重放/竞争是常态）: %d 次失败, 首个错误 %v", failed, firstErr)
	}
	if applied != 1 {
		t.Fatalf("20 并发应恰好 1 次生效, got %d（幂等=%d）", applied, idem)
	}
	if applied+idem != workers {
		t.Fatalf("总处理数不符: %d+%d ≠ %d", applied, idem, workers)
	}
	bal, _ := svc.CashBalance(uid)
	if bal != amount {
		t.Fatalf("余额应 %d, got %d", amount, bal)
	}
	logs, _ := NewRepo(db).CashLogsByUser(uid)
	if len(logs) != 1 {
		t.Fatalf("并发后流水应恰 1 条, got %d", len(logs))
	}
	if logs[0].Delta != amount || logs[0].BalanceAfter != amount {
		t.Fatalf("流水金额不符: %+v", logs[0])
	}
}

// TestExpireThenCallbackRejected 状态机（验收项 ⑤）：超时关单 + 关单后回调被拒。
func TestExpireThenCallbackRejected(t *testing.T) {
	db := testDB(t)
	cleanupPay(t, db)
	t.Cleanup(func() { cleanupPay(t, db) })

	svc := newTestSvc(t, db)
	uid := testUIDBase + 5
	seedUser(t, db, uid)
	o := newOrder(t, svc, uid, 1200)

	// 手工把过期时间推到过去（模拟超时），触发惰性关单
	if err := db.Exec("UPDATE pay_order SET expire_at = DATE_SUB(NOW(), INTERVAL 1 MINUTE) WHERE order_no = ?",
		o.OrderNo).Error; err != nil {
		t.Fatal(err)
	}
	// 关单必须由「下一次读/写入口」的惰性关单完成（plan §2E.3）
	if _, err := svc.GetOrder(context.Background(), o.OrderNo, uid); err != nil {
		t.Fatal(err)
	}
	var dbOrder PayOrder
	db.Where("order_no = ?", o.OrderNo).First(&dbOrder)
	if dbOrder.Status != StatusClosed {
		t.Fatalf("超时订单应被关单为已关闭, got %d", dbOrder.Status)
	}
	if dbOrder.ClosedAt == nil {
		t.Fatal("关单应写 closed_at")
	}

	// 关单后的支付成功回调必须被拒且不入账（否则「过期后支付」会静默生效）
	_, err := svc.DevCallback(o.OrderNo, 1200, "success")
	if !errors.Is(err, errcode.ErrPayOrderExpired) {
		t.Fatalf("关单后回调应 92007, got %v", err)
	}
	bal, _ := svc.CashBalance(uid)
	if bal != 0 {
		t.Fatalf("关单后回调不得入账, 余额 %d", bal)
	}
	logs, _ := NewRepo(db).CashLogsByUser(uid)
	if len(logs) != 0 {
		t.Fatalf("关单后回调不得产生流水, got %d", len(logs))
	}
	// 幂等关单：重复执行不再改动（RowsAffected=0）
	n, err := svc.AdminCloseExpired()
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("已关单的订单不应被二次关闭, got %d", n)
	}
}

// TestRepeatPayRejected 状态机：已支付订单再收成功回调 → 幂等返回，不再入账；退款后再回调 → 92002。
func TestRepeatPayRejected(t *testing.T) {
	db := testDB(t)
	cleanupPay(t, db)
	t.Cleanup(func() { cleanupPay(t, db) })

	svc := newTestSvc(t, db)
	uid := testUIDBase + 6
	seedUser(t, db, uid)
	o := newOrder(t, svc, uid, 600)

	if _, err := svc.DevCallback(o.OrderNo, 600, "success"); err != nil {
		t.Fatal(err)
	}
	// 已支付后再回调：幂等，Applied=false
	res, err := svc.DevCallback(o.OrderNo, 600, "success")
	if err != nil {
		t.Fatalf("已支付后回调应幂等不报错: %v", err)
	}
	if res.Applied {
		t.Fatal("已支付后回调不应再次生效")
	}
	bal, _ := svc.CashBalance(uid)
	if bal != 600 {
		t.Fatalf("重复支付回调后余额应仍为 600, got %d", bal)
	}

	// 退款路径
	rr, err := svc.Refund(context.Background(), o.OrderNo, uid)
	if err != nil {
		t.Fatalf("退款失败: %v", err)
	}
	if !rr.Applied || rr.Status != StatusRefunded {
		t.Fatalf("退款应生效并置已退款: %+v", rr)
	}
	bal, _ = svc.CashBalance(uid)
	if bal != 0 {
		t.Fatalf("退款后余额应回到 0, got %d", bal)
	}
	logs, _ := NewRepo(db).CashLogsByUser(uid)
	if len(logs) != 2 {
		t.Fatalf("应有入账 + 退款两条流水, got %d", len(logs))
	}
	if logs[1].Delta != -600 || logs[1].BalanceAfter != 0 || logs[1].BizType != CashBizRefund {
		t.Fatalf("退款流水不符: %+v", logs[1])
	}
	// 重复退款：幂等（第二次状态已非已支付 → 92002）
	if _, err := svc.Refund(context.Background(), o.OrderNo, uid); !errors.Is(err, errcode.ErrPayOrderStatus) {
		t.Fatalf("重复退款应 92002, got %v", err)
	}
	// 退款后的成功回调必须被拒（否则钱退了又入账）
	if _, err := svc.DevCallback(o.OrderNo, 600, "success"); !errors.Is(err, errcode.ErrPayOrderStatus) {
		t.Fatalf("已退款订单的回调应 92002, got %v", err)
	}
	bal, _ = svc.CashBalance(uid)
	if bal != 0 {
		t.Fatalf("已退款后的回调不得入账, 余额 %d", bal)
	}
}

// TestRefundRequiresPaid 未支付订单不得退款。
func TestRefundRequiresPaid(t *testing.T) {
	db := testDB(t)
	cleanupPay(t, db)
	t.Cleanup(func() { cleanupPay(t, db) })

	svc := newTestSvc(t, db)
	uid := testUIDBase + 7
	seedUser(t, db, uid)
	o := newOrder(t, svc, uid, 100)

	if _, err := svc.Refund(context.Background(), o.OrderNo, uid); !errors.Is(err, errcode.ErrPayOrderStatus) {
		t.Fatalf("未支付订单退款应 92002, got %v", err)
	}
}

// TestCallbackAmountMismatch 金额不符：拒绝入账 + 订单不变 + 落回调日志（供对账查出）。
func TestCallbackAmountMismatch(t *testing.T) {
	db := testDB(t)
	cleanupPay(t, db)
	t.Cleanup(func() { cleanupPay(t, db) })

	svc := newTestSvc(t, db)
	uid := testUIDBase + 8
	seedUser(t, db, uid)
	o := newOrder(t, svc, uid, 1000)

	// 验签合法（我们自己的密钥），但报文金额与订单不符
	_, err := svc.DevCallback(o.OrderNo, 100, "success")
	if !errors.Is(err, errcode.ErrPayAmountInvalid) {
		t.Fatalf("金额不符应 92008, got %v", err)
	}
	var dbOrder PayOrder
	db.Where("order_no = ?", o.OrderNo).First(&dbOrder)
	if dbOrder.Status != StatusPending {
		t.Fatalf("金额不符不得推进订单, got %d", dbOrder.Status)
	}
	bal, _ := svc.CashBalance(uid)
	if bal != 0 {
		t.Fatalf("金额不符不得入账, 余额 %d", bal)
	}
}

// TestOrderAccessControl 越权：只能查自己的订单（他人订单返回 92001，不泄露存在性）。
func TestOrderAccessControl(t *testing.T) {
	db := testDB(t)
	cleanupPay(t, db)
	t.Cleanup(func() { cleanupPay(t, db) })

	svc := newTestSvc(t, db)
	uidA, uidB := testUIDBase+9, testUIDBase+10
	seedUser(t, db, uidA)
	seedUser(t, db, uidB)
	o := newOrder(t, svc, uidA, 100)

	if _, err := svc.GetOrder(context.Background(), o.OrderNo, uidB); !errors.Is(err, errcode.ErrPayOrderNotFound) {
		t.Fatalf("跨用户查单应 92001, got %v", err)
	}
	if _, err := svc.Refund(context.Background(), o.OrderNo, uidB); !errors.Is(err, errcode.ErrPayOrderNotFound) {
		t.Fatalf("跨用户退款应 92001, got %v", err)
	}
	// 本人可查
	if _, err := svc.GetOrder(context.Background(), o.OrderNo, uidA); err != nil {
		t.Fatalf("本人查单应成功: %v", err)
	}
}

// TestUnknownChannelRejected 未知渠道下单被拒（spec §6.3）。
func TestUnknownChannelRejected(t *testing.T) {
	db := testDB(t)
	svc := newTestSvc(t, db)
	_, err := svc.CreateOrder(context.Background(), &CreateOrderReq{
		UserID: testUIDBase + 11, Amount: 100, Channel: "paypal", Subject: "x",
	})
	if !errors.Is(err, errcode.ErrPayChannelUnknown) {
		t.Fatalf("未知渠道应 92003, got %v", err)
	}
	// 真实渠道（未实现）下单应 92004，而不是静默成功
	uid := testUIDBase + 12
	seedUser(t, db, uid)
	t.Cleanup(func() { cleanupPay(t, db) })
	_, err = svc.CreateOrder(context.Background(), &CreateOrderReq{
		UserID: uid, Amount: 100, Channel: ChannelAlipay, Subject: "x",
	})
	if !errors.Is(err, errcode.ErrPayChannelNotImpl) {
		t.Fatalf("未实现渠道应 92004, got %v", err)
	}
}

// TestAmountLimits 金额边界：≤0 与超上限一律 92008。
func TestAmountLimits(t *testing.T) {
	db := testDB(t)
	svc := newTestSvc(t, db)
	uid := testUIDBase + 13
	for _, amt := range []int64{0, -1, svc.cfg.MaxAmount + 1} {
		_, err := svc.CreateOrder(context.Background(), &CreateOrderReq{
			UserID: uid, Amount: amt, Channel: ChannelMock, Subject: "x",
		})
		if !errors.Is(err, errcode.ErrPayAmountInvalid) {
			t.Fatalf("金额 %d 应 92008, got %v", amt, err)
		}
	}
}

// TestManualMarkPaid 手动补单（spec §6.7）：对待支付订单生效；对已支付订单拒绝；幂等。
func TestManualMarkPaid(t *testing.T) {
	db := testDB(t)
	cleanupPay(t, db)
	t.Cleanup(func() { cleanupPay(t, db) })

	svc := newTestSvc(t, db)
	uid := testUIDBase + 14
	seedUser(t, db, uid)
	o := newOrder(t, svc, uid, 250)

	res, err := svc.ManualMarkPaid(o.OrderNo, 1)
	if err != nil {
		t.Fatalf("补单失败: %v", err)
	}
	if !res.Applied || res.Status != StatusPaid {
		t.Fatalf("补单应生效: %+v", res)
	}
	bal, _ := svc.CashBalance(uid)
	if bal != 250 {
		t.Fatalf("补单后余额应 250, got %d", bal)
	}
	// 已支付再补单必须拒绝（不得双倍入账）
	if _, err := svc.ManualMarkPaid(o.OrderNo, 1); !errors.Is(err, errcode.ErrPayOrderStatus) {
		t.Fatalf("已支付订单补单应 92002, got %v", err)
	}
	bal, _ = svc.CashBalance(uid)
	if bal != 250 {
		t.Fatalf("重复补单不得双倍入账, 余额 %d", bal)
	}
}

// TestManualMarkPaidOnClosed 补单可救回被超时关单的订单（人工确认收款后的特权）。
func TestManualMarkPaidOnClosed(t *testing.T) {
	db := testDB(t)
	cleanupPay(t, db)
	t.Cleanup(func() { cleanupPay(t, db) })

	svc := newTestSvc(t, db)
	uid := testUIDBase + 15
	seedUser(t, db, uid)
	o := newOrder(t, svc, uid, 400)
	if err := db.Exec("UPDATE pay_order SET expire_at = DATE_SUB(NOW(), INTERVAL 1 MINUTE) WHERE order_no = ?",
		o.OrderNo).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AdminCloseExpired(); err != nil {
		t.Fatal(err)
	}
	res, err := svc.ManualMarkPaid(o.OrderNo, 7)
	if err != nil {
		t.Fatalf("对已关单订单补单应成功: %v", err)
	}
	if !res.Applied {
		t.Fatal("补单应生效")
	}
	// 流水备注应记录操作人
	logs, _ := NewRepo(db).CashLogsByUser(uid)
	if len(logs) != 1 || !contains(logs[0].Remark, "admin:7") {
		t.Fatalf("补单流水应记录操作人: %+v", logs)
	}
}

// TestReconFindsIssues 对账（验收项 ⑥）：造四类异常 → 异常清单能查出；重跑幂等。
func TestReconFindsIssues(t *testing.T) {
	db := testDB(t)
	cleanupPay(t, db)
	t.Cleanup(func() { cleanupPay(t, db) })

	svc := newTestSvc(t, db)
	recon := NewReconService(NewRepo(db), svc.cfg, zap.NewNop())
	uid := testUIDBase + 20
	seedUser(t, db, uid)

	// 正常单（对照组）
	ok := newOrder(t, svc, uid, 100)
	if _, err := svc.DevCallback(ok.OrderNo, 100, "success"); err != nil {
		t.Fatal(err)
	}
	// ① 金额不符
	mism := newOrder(t, svc, uid, 1000)
	if _, err := svc.DevCallback(mism.OrderNo, 100, "success"); err == nil {
		t.Fatal("金额不符应被拒（用于造异常证据）")
	}
	// ② 长期挂起：伪造一条 48 小时前的待支付订单
	stuck := newOrder(t, svc, uid, 300)
	if err := db.Exec("UPDATE pay_order SET created_at = DATE_SUB(NOW(), INTERVAL 48 HOUR) WHERE order_no = ?",
		stuck.OrderNo).Error; err != nil {
		t.Fatal(err)
	}
	// ③ 流水缺失：造一条已支付但删掉其流水
	miss := newOrder(t, svc, uid, 700)
	if _, err := svc.DevCallback(miss.OrderNo, 700, "success"); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("DELETE FROM cash_log WHERE biz_no = ?", "pay:"+miss.OrderNo).Error; err != nil {
		t.Fatal(err)
	}
	// ④ 重复回调：同一订单收到两次验签通过的回调
	dup := newOrder(t, svc, uid, 200)
	if _, err := svc.DevCallback(dup.OrderNo, 200, "success"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.DevCallback(dup.OrderNo, 200, "success"); err != nil {
		t.Fatal(err)
	}

	today := time.Now().Format("2006-01-02")
	res, err := recon.Run(today, ChannelMock)
	if err != nil {
		t.Fatalf("对账失败: %v", err)
	}
	if len(res.Channels) != 1 {
		t.Fatalf("应产出 1 个渠道对账单, got %d", len(res.Channels))
	}
	daily := res.Channels[0]
	if daily.Status != ReconHasIssue || daily.DiffCount == 0 {
		t.Fatalf("有异常的日子应标记为有异常: %+v", daily)
	}

	// 四类异常都能查出。
	// 断言按**本测试创建的订单号**取，而不是按类型取第一条：历史上其它测试在同一天
	// 也会留下同类异常（异常清单是全局的，不清空就无法按类型唯一断言）。
	byOrder := map[string]*IssueView{}
	for _, i := range res.Issues {
		byOrder[i.OrderNo] = i
	}
	for _, exp := range []struct {
		orderNo string
		typ     int8
		name    string
	}{
		{mism.OrderNo, IssueAmountMismatch, "金额不符"},
		{stuck.OrderNo, IssueStuck, "长期挂起"},
		{miss.OrderNo, IssueMissingLog, "流水缺失"},
		{dup.OrderNo, IssueDuplicateCb, "重复回调"},
	} {
		got, ok := byOrder[exp.orderNo]
		if !ok {
			t.Fatalf("应查出%s单 %s，实际清单: %+v", exp.name, exp.orderNo, res.Issues)
		}
		if got.IssueType != exp.typ {
			t.Fatalf("%s 的异常类型应为 %d, got %d", exp.orderNo, exp.typ, got.IssueType)
		}
	}
	// 正常单不得被列为异常
	for _, i := range res.Issues {
		if i.OrderNo == ok.OrderNo {
			t.Fatalf("正常单 %s 不应出现在异常清单", ok.OrderNo)
		}
	}

	// 重复回调项的 detail 必须区分「只是重投」与「幂等被击穿」：dup 单只重投未重复入账，
	// 故应写明流水 1 条且**不含**「幂等被击穿」字样。
	if !contains(byOrder[dup.OrderNo].Detail, "现金流水 1 条") {
		t.Fatalf("重复回调 detail 应写明流水条数: %q", byOrder[dup.OrderNo].Detail)
	}
	if contains(byOrder[dup.OrderNo].Detail, "幂等被击穿") {
		t.Fatalf("仅重投未重复入账，不应判为幂等被击穿: %q", byOrder[dup.OrderNo].Detail)
	}

	// 重跑幂等：异常清单条数不膨胀（人工反复核对的前提）
	before := len(res.Issues)
	res2, err := recon.Run(today, ChannelMock)
	if err != nil {
		t.Fatal(err)
	}
	if len(res2.Issues) != before {
		t.Fatalf("重跑对账不应膨胀异常清单: %d → %d", before, len(res2.Issues))
	}
	for _, o := range []string{mism.OrderNo, stuck.OrderNo, miss.OrderNo, dup.OrderNo} {
		var n int64
		db.Model(&PayReconIssue{}).Where("recon_date = ? AND order_no = ?", today, o).Count(&n)
		if n != 1 {
			t.Fatalf("订单 %s 的异常记录应恰 1 条（UPSERT 幂等）, got %d", o, n)
		}
	}
}

// TestReconIssueHandle 异常处理：处理人、时间、备注落库；重复处理只更新备注。
func TestReconIssueHandle(t *testing.T) {
	db := testDB(t)
	cleanupPay(t, db)
	t.Cleanup(func() { cleanupPay(t, db) })

	svc := newTestSvc(t, db)
	recon := NewReconService(NewRepo(db), svc.cfg, zap.NewNop())
	uid := testUIDBase + 21
	seedUser(t, db, uid)
	stuck := newOrder(t, svc, uid, 100)
	if err := db.Exec("UPDATE pay_order SET created_at = DATE_SUB(NOW(), INTERVAL 48 HOUR) WHERE order_no = ?",
		stuck.OrderNo).Error; err != nil {
		t.Fatal(err)
	}
	today := time.Now().Format("2006-01-02")
	res, err := recon.Run(today, ChannelMock)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Issues) == 0 {
		t.Fatal("应至少有一个异常")
	}
	var issue *IssueView
	for _, i := range res.Issues {
		if i.IssueType == IssueStuck {
			issue = i
			break
		}
	}
	if issue == nil {
		t.Fatal("应查出长期挂起异常")
	}
	got, err := recon.Handle(issue.ID, 42, "已人工确认，订单作废")
	if err != nil {
		t.Fatalf("异常处理失败: %v", err)
	}
	if got.Status != IssueHandled || got.HandledBy != "42" || got.HandledAt == "" {
		t.Fatalf("处理结果不符: %+v", got)
	}
	if got.HandleNote != "已人工确认，订单作废" {
		t.Fatalf("备注不符: %q", got.HandleNote)
	}
	// 重复处理：只更新备注
	got2, err := recon.Handle(issue.ID, 43, "复核：已作废")
	if err != nil {
		t.Fatal(err)
	}
	if got2.HandleNote != "复核：已作废" || got2.HandledBy != "43" {
		t.Fatalf("重复处理应更新: %+v", got2)
	}
	// 不存在的异常
	if _, err := recon.Handle(999999999, 1, "x"); !errors.Is(err, errcode.ErrNotFound) {
		t.Fatalf("不存在应 10005, got %v", err)
	}
}

// TestReconCashVsOrderMismatch 对账盲区保护：两条口径对不上但四类判据都没命中时，
// 对账单仍必须标异常（否则差异会无声消失）。
func TestReconCashVsOrderMismatch(t *testing.T) {
	db := testDB(t)
	cleanupPay(t, db)
	t.Cleanup(func() { cleanupPay(t, db) })

	svc := newTestSvc(t, db)
	recon := NewReconService(NewRepo(db), svc.cfg, zap.NewNop())
	uid := testUIDBase + 22
	seedUser(t, db, uid)
	o := newOrder(t, svc, uid, 500)
	if _, err := svc.DevCallback(o.OrderNo, 500, "success"); err != nil {
		t.Fatal(err)
	}
	// 手工把流水金额改小（模拟流水侧被篡改：订单口径 500，流水口径 100）
	if err := db.Exec("UPDATE cash_log SET delta = 100 WHERE biz_no = ?", "pay:"+o.OrderNo).Error; err != nil {
		t.Fatal(err)
	}
	today := time.Now().Format("2006-01-02")
	res, err := recon.Run(today, ChannelMock)
	if err != nil {
		t.Fatal(err)
	}
	if res.Channels[0].Status != ReconHasIssue {
		t.Fatalf("订单口径 %d ≠ 流水口径 %d，对账单必须标异常: %+v",
			res.Channels[0].PaidAmount, res.Channels[0].CashLogAmount, res.Channels[0])
	}
}

// TestReconRejectsFutureDate 未来日期对账拒绝（92011）。
func TestReconRejectsFutureDate(t *testing.T) {
	db := testDB(t)
	recon := NewReconService(NewRepo(db), DefaultConfig(), zap.NewNop())
	if _, err := recon.Run("2099-01-01", ChannelMock); !errors.Is(err, errcode.ErrPayReconDateInvalid) {
		t.Fatalf("未来日期应 92011, got %v", err)
	}
	if _, err := recon.Run("not-a-date", ChannelMock); !errors.Is(err, errcode.ErrPayReconDateInvalid) {
		t.Fatalf("非法日期应 92011, got %v", err)
	}
}

// TestNoFloatInAmounts 金额链路的整数性：DB 列类型必须是整型（BIGINT），
// 且 Go 侧字段为 int64 —— 用 schema 断言钉住「禁用浮点」这一硬约束（spec §6.1）。
func TestNoFloatInAmounts(t *testing.T) {
	db := testDB(t)
	type col struct {
		TABLE_NAME  string
		COLUMN_NAME string
		DATA_TYPE   string
	}
	rows := []col{}
	if err := db.Raw(`SELECT TABLE_NAME, COLUMN_NAME, DATA_TYPE FROM information_schema.columns
		WHERE table_schema = DATABASE()
		  AND ((TABLE_NAME = 'pay_order' AND COLUMN_NAME IN ('amount','refund_amount'))
		    OR (TABLE_NAME = 'cash_account' AND COLUMN_NAME = 'balance')
		    OR (TABLE_NAME = 'cash_log' AND COLUMN_NAME IN ('delta','balance_after'))
		    OR (TABLE_NAME = 'pay_recon_daily' AND COLUMN_NAME LIKE '%amount%')
		    OR (TABLE_NAME = 'pay_callback_log' AND COLUMN_NAME = 'amount'))`).Scan(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) < 8 {
		t.Fatalf("金额列应至少 8 个, got %d: %+v", len(rows), rows)
	}
	for _, c := range rows {
		if c.DATA_TYPE != "bigint" {
			t.Fatalf("金额列 %s.%s 类型为 %s，必须为 bigint（禁止浮点/小数）",
				c.TABLE_NAME, c.COLUMN_NAME, c.DATA_TYPE)
		}
	}
}

// TestAccountSeparationSchema 现金与虚拟币表必须各自独立存在（不共用表）。
func TestAccountSeparationSchema(t *testing.T) {
	db := testDB(t)
	var cnt int64
	if err := db.Raw(`SELECT COUNT(*) FROM information_schema.tables
		WHERE table_schema = DATABASE() AND table_name IN ('cash_account','melon_seed_account','pay_order')`).
		Scan(&cnt).Error; err != nil {
		t.Fatal(err)
	}
	if cnt != 3 {
		t.Fatalf("现金账户/金瓜子账户/订单表应各自独立存在, got %d", cnt)
	}
	// 探针表可读（证明约束可验证）
	var n int64
	if err := db.Model(&coinLogProbe{}).Count(&n).Error; err != nil {
		t.Fatalf("coin_log 应可读: %v", err)
	}
	if err := db.Model(&melonAcctProbe{}).Count(&n).Error; err != nil {
		t.Fatalf("melon_seed_account 应可读: %v", err)
	}
}

// TestReconDateRendering 日期口径：对账单与异常项的 recon_date 必须是 YYYY-MM-DD。
//
// 回归护栏：DATE 列若用 string 字段承接，GORM 会读出 `2026-10-15T00:00:00+08:00`
// （实测），前端据此做日期比较/回填查询参数都会失配。
func TestReconDateRendering(t *testing.T) {
	db := testDB(t)
	cleanupPay(t, db)
	t.Cleanup(func() { cleanupPay(t, db) })

	svc := newTestSvc(t, db)
	recon := NewReconService(NewRepo(db), svc.cfg, zap.NewNop())
	uid := testUIDBase + 30
	seedUser(t, db, uid)
	o := newOrder(t, svc, uid, 100)
	if _, err := svc.DevCallback(o.OrderNo, 100, "success"); err != nil {
		t.Fatal(err)
	}
	today := time.Now().Format("2006-01-02")
	res, err := recon.Run(today, ChannelMock)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Channels) != 1 {
		t.Fatalf("应产出 1 条对账单, got %d", len(res.Channels))
	}
	if res.Channels[0].ReconDate != today {
		t.Fatalf("对账单日期应为 %q, got %q", today, res.Channels[0].ReconDate)
	}
	for _, i := range res.Issues {
		if i.ReconDate != today {
			t.Fatalf("异常项日期应为 %q, got %q", today, i.ReconDate)
		}
	}
	// 查询接口同样
	got, err := recon.Daily(today, ChannelMock, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if n := got["total"].(int64); n < 1 {
		t.Fatalf("按日期查询应命中, total=%d", n)
	}
	views := got["list"].([]*DailyView)
	if views[0].ReconDate != today {
		t.Fatalf("查询接口返回的日期应为 %q, got %q", today, views[0].ReconDate)
	}
	// 异常清单查询接口
	gotIss, err := recon.Issues(IssueFilter{Date: today, Channel: ChannelMock, Page: 1, Size: 20})
	if err != nil {
		t.Fatal(err)
	}
	ivs := gotIss["list"].([]*IssueView)
	if len(ivs) == 0 {
		t.Fatal("按日期查询异常应有结果")
	}
	for _, v := range ivs {
		if v.ReconDate != today {
			t.Fatalf("异常清单接口日期应为 %q, got %q", today, v.ReconDate)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
