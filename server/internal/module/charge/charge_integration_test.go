package charge

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/dlidli/server/internal/module/pay"
	"github.com/dlidli/server/internal/pkg/errcode"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// M4-PAY-02 集成测试（真实 MySQL）：覆盖验收核心 —— 充电全链路 + 70/30 精确分成、
// 口径与 LIV-04 一致性、**并发提现不得超提**、提现状态机与重复打款拦截、月结聚合、
// 鸣谢名单匿名口径。未设置 DLIDLI_TEST_DSN 时跳过（沿用 pay/live 包既有口径）。

func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("DLIDLI_TEST_DSN")
	if dsn == "" {
		t.Skip("DLIDLI_TEST_DSN 未设置，跳过 charge 集成测试")
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

// 测试用户段 92700xxx：避开 pay(92600xxx) 与 live(91000xxx) 的段，
// 避免跨包清理互相踩踏（PAY-01/VIP-01 实测过这类污染）。
//
// 上界必须**收在 92700999**：接口级 E2E（.dev-logs/pay02/e2e/drive.mjs）的账号在
// 92710xxx 段，而 Go 用例的 cleanup 会 DELETETE 整个区间的 user 行——区间开得太宽
// 会把 E2E 账号一起删掉，表现为后续 E2E 全部报 92204「充电对象不存在」（本卡实测踩到）。
const (
	testUIDBase = int64(92700000)
	testUIDEnd  = int64(92700999)
)

// cleanupCharge 清理 charge 域测试数据（顺序：先子后父，避免外键式残留断言失败）。
func cleanupCharge(t *testing.T, db *gorm.DB) {
	t.Helper()
	_ = db.Exec(`DELETE FROM creator_balance_log WHERE user_id BETWEEN ? AND ?`, testUIDBase, testUIDEnd).Error
	_ = db.Exec(`DELETE FROM creator_balance WHERE user_id BETWEEN ? AND ?`, testUIDBase, testUIDEnd).Error
	_ = db.Exec(`DELETE FROM withdraw_payout_log WHERE user_id BETWEEN ? AND ?`, testUIDBase, testUIDEnd).Error
	_ = db.Exec(`DELETE FROM withdraw_order WHERE user_id BETWEEN ? AND ?`, testUIDBase, testUIDEnd).Error
	_ = db.Exec(`DELETE FROM creator_income WHERE user_id BETWEEN ? AND ?`, testUIDBase, testUIDEnd).Error
	_ = db.Exec(`DELETE FROM creator_charge_settlement WHERE user_id BETWEEN ? AND ?`, testUIDBase, testUIDEnd).Error
	_ = db.Exec(`DELETE FROM creator_settlement WHERE user_id BETWEEN ? AND ? AND source = 'charge'`, testUIDBase, testUIDEnd).Error
	_ = db.Exec(`DELETE FROM charge_log WHERE user_id BETWEEN ? AND ? OR anchor_id BETWEEN ? AND ?`,
		testUIDBase, testUIDEnd, testUIDBase, testUIDEnd).Error
	_ = db.Exec(`DELETE FROM cash_log WHERE user_id BETWEEN ? AND ?`, testUIDBase, testUIDEnd).Error
	_ = db.Exec(`DELETE FROM cash_account WHERE user_id BETWEEN ? AND ?`, testUIDBase, testUIDEnd).Error
	_ = db.Exec(`DELETE FROM pay_callback_log WHERE order_no IN
		(SELECT order_no FROM pay_order WHERE user_id BETWEEN ? AND ?)`, testUIDBase, testUIDEnd).Error
	_ = db.Exec(`DELETE FROM pay_order WHERE user_id BETWEEN ? AND ?`, testUIDBase, testUIDEnd).Error
	// 硬约束取证：充电流程不得写虚拟币（清空后该断言才真正在测本次流程）。
	_ = db.Exec(`DELETE FROM coin_log WHERE user_id BETWEEN ? AND ?`, testUIDBase, testUIDEnd).Error
	_ = db.Exec(`DELETE FROM melon_seed_log WHERE user_id BETWEEN ? AND ?`, testUIDBase, testUIDEnd).Error
	_ = db.Exec(`DELETE FROM melon_seed_account WHERE user_id BETWEEN ? AND ?`, testUIDBase, testUIDEnd).Error
	_ = db.Exec(`DELETE FROM video WHERE id BETWEEN ? AND ?`, testUIDBase, testUIDEnd).Error
	_ = db.Exec(`DELETE FROM `+"`user`"+` WHERE id BETWEEN ? AND ?`, testUIDBase, testUIDEnd).Error
}

// seedUser 造测试用户（昵称带 uid：`user.uk_nickname` 唯一）。
func seedUser(t *testing.T, db *gorm.DB, uid int64) {
	t.Helper()
	if err := db.Exec("INSERT INTO `user` (id, nickname, status, created_at, updated_at) VALUES (?, ?, 0, NOW(), NOW())",
		uid, "chargetest-"+strconv.FormatInt(uid, 10)).Error; err != nil {
		t.Fatalf("造用户失败: %v", err)
	}
}

// seedVideo 造稿件（鸣谢名单按稿筛选与「按稿聚合结算」需要真实 video 行）。
func seedVideo(t *testing.T, db *gorm.DB, id, uid int64) {
	t.Helper()
	if err := db.Exec(`INSERT INTO video (id, bvid, user_id, title, cover, category_id, copyright, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, '', 1, 1, 0, NOW(), NOW())`,
		id, "BVP02"+strconv.FormatInt(id, 10), uid, "充电测试稿").Error; err != nil {
		t.Fatalf("造稿件失败: %v", err)
	}
}

// errCode 取业务错误码（0 表示成功或非业务错误）。
//
// 为什么不用 errors.Is 比错误值：errcode.Error 是**指针**类型，Service 用 WithMsg
// 返回的是「同码不同文案」的新对象，指针比较会把正确实现判成失败。
func errCode(err error) int {
	if err == nil {
		return 0
	}
	var e *errcode.Error
	if errors.As(err, &e) {
		return e.Code
	}
	return -1
}

// newTestSvc 构造服务（真实 pay 域，mock 渠道）。
func newTestSvc(t *testing.T, db *gorm.DB) (*Service, *pay.Service) {
	t.Helper()
	payCfg := pay.DefaultConfig()
	payCfg.MockSecret = "test-charge-secret"
	paySvc := pay.NewService(pay.NewRepo(db), payCfg, zap.NewNop())
	cfg := DefaultConfig()
	cfg.MinWithdrawAmount = 1 // 测试按小额验证（真实门槛 100 元见 dev.yaml）
	svc := NewService(NewRepo(db), paySvc, cfg, zap.NewNop())
	// 把充电分成挂在支付入账事务内（与 router 装配同构：会员 + 充电组合钩子）。
	paySvc.SetPaidHook(func(tx *gorm.DB, orderNo string, userID int64) error {
		return svc.ApplyPaidHook(tx, orderNo, userID)
	})
	return svc, paySvc
}

// chargeOnce 走一次完整充电：下单 → mock 支付回调。
func chargeOnce(t *testing.T, svc *Service, paySvc *pay.Service, uid, anchor, video int64, tier string, amount int64) *ChargeLog {
	t.Helper()
	order, err := svc.CreateCharge(context.Background(), &CreateChargeReq{
		UserID: uid, AnchorID: anchor, VideoID: video, TierCode: tier, Amount: amount,
		ClientIP: "127.0.0.1", Channel: pay.ChannelMock,
	})
	if err != nil {
		t.Fatalf("充电下单失败: %v", err)
	}
	res, err := paySvc.DevCallback(order.OrderNo, order.Amount, "success")
	if err != nil {
		t.Fatalf("mock 支付回调失败: %v", err)
	}
	if !res.Applied {
		t.Fatalf("首次回调应生效，实际 applied=false: %s", res.Message)
	}
	c, err := svc.repo.ChargeByOrder(order.OrderNo)
	if err != nil {
		t.Fatalf("取充电记录失败: %v", err)
	}
	return c
}

// TestE2EChargeFullChain 验收项 ①：下单 → mock 支付 → 回调 → 充电记录 + 分成入账，
// DB 逐条核对 70/30 精确值（900 分 → 创作者 630 / 平台 270）。
func TestE2EChargeFullChain(t *testing.T) {
	db := testDB(t)
	cleanupCharge(t, db)
	t.Cleanup(func() { cleanupCharge(t, db) })

	svc, paySvc := newTestSvc(t, db)
	const viewer, anchor, video = int64(92700001), int64(92700002), int64(92700011)
	seedUser(t, db, viewer)
	seedUser(t, db, anchor)
	seedVideo(t, db, video, anchor)

	c := chargeOnce(t, svc, paySvc, viewer, anchor, video, "tier_10", 0)
	if c.Status != ChargePaid {
		t.Fatalf("充电记录应为已支付，实际 status=%d", c.Status)
	}
	if c.Amount != 1000 {
		t.Fatalf("档位 tier_10 金额应为 1000 分，实际 %d", c.Amount)
	}
	if c.ShareRate != 700 {
		t.Fatalf("分成比例快照应为 700‰，实际 %d", c.ShareRate)
	}
	if c.PaidAt == nil {
		t.Fatalf("已支付充电必须有 paid_at（跨月归属依据）")
	}

	// 分成明细：1000 分 → 700 / 300（整数分，精确）。
	inc, err := svc.repo.IncomeByOrder(c.OrderNo)
	if err != nil {
		t.Fatalf("取分成明细失败: %v", err)
	}
	if inc.Amount != 700 || inc.PlatformAmount != 300 {
		t.Fatalf("分成应为创作者 700 / 平台 300，实际 %d / %d", inc.Amount, inc.PlatformAmount)
	}
	if inc.UserID != anchor {
		t.Fatalf("分成应记给 UP 主 %d，实际 %d", anchor, inc.UserID)
	}
	if inc.OrderNo != c.OrderNo {
		t.Fatalf("分成记录必须可追溯到来源订单：期望 %s 实际 %s", c.OrderNo, inc.OrderNo)
	}

	// 余额：可用 = 700，冻结 = 0。
	bal, err := svc.Balance(anchor)
	if err != nil {
		t.Fatalf("取余额失败: %v", err)
	}
	if bal.Balance != 700 || bal.Frozen != 0 {
		t.Fatalf("创作者余额应为 700/0，实际 %d/%d", bal.Balance, bal.Frozen)
	}

	// 硬约束：**充电是现金**，绝不写虚拟币（硬币 / 金瓜子）。
	var coinCnt, melonCnt int64
	_ = db.Table("coin_log").Where("user_id IN ?", []int64{viewer, anchor}).Count(&coinCnt).Error
	_ = db.Table("melon_seed_log").Where("user_id IN ?", []int64{viewer, anchor}).Count(&melonCnt).Error
	if coinCnt != 0 || melonCnt != 0 {
		t.Fatalf("充电不得写虚拟币：coin_log=%d melon_seed_log=%d（必须为 0）", coinCnt, melonCnt)
	}

	// 幂等：重复回调不重复记分成（余额与分成笔数不变）。
	if _, err := paySvc.DevCallback(c.OrderNo, 1000, "success"); err != nil {
		t.Fatalf("重放回调不应报错: %v", err)
	}
	var incCnt int64
	_ = db.Table("creator_income").Where("order_no = ?", c.OrderNo).Count(&incCnt).Error
	bal2, _ := svc.Balance(anchor)
	if incCnt != 1 || bal2.Balance != 700 {
		t.Fatalf("重放回调不得重复入账：分成笔数=%d（期望 1）余额=%d（期望 700）", incCnt, bal2.Balance)
	}
}

// TestShareRateMatchesLive 验收项 ②：口径一致性 —— 充电分成比例必须与 LIV-04 礼物一致。
//
// 判据取**数据**而非常量：读 live 域已落库的礼物分成流水（gift_share_log），
// 反算其实际生效比例，与充电实际落库的比例对照。这样「两边常量各自漂移」不会被漏掉。
func TestShareRateMatchesLive(t *testing.T) {
	db := testDB(t)
	cleanupCharge(t, db)
	t.Cleanup(func() { cleanupCharge(t, db) })

	svc, paySvc := newTestSvc(t, db)
	const viewer, anchor, video = int64(92700021), int64(92700022), int64(92700023)
	seedUser(t, db, viewer)
	seedUser(t, db, anchor)
	seedVideo(t, db, video, anchor)
	c := chargeOnce(t, svc, paySvc, viewer, anchor, video, "tier_10", 0)

	inc, _ := svc.repo.IncomeByOrder(c.OrderNo)
	chargeRate := inc.ShareRate

	// 判据取**两边实际落库的快照值**（LIV-04 在 gift_log.share_rate 快照、本卡在
	// charge_log.share_rate 快照），而不是各自包里的常量——常量各自漂移测不出来。
	if chargeRate != 700 {
		t.Fatalf("充电分成比例应为 700‰，实际 %d‰", chargeRate)
	}
	type giftRow struct {
		ShareRate int64
	}
	var gifts []giftRow
	if err := db.Raw("SELECT DISTINCT share_rate FROM gift_log").Scan(&gifts).Error; err != nil {
		t.Fatalf("读礼物分成快照失败: %v", err)
	}
	if len(gifts) == 0 {
		t.Skip("库中无礼物流水（live 用例未跑过）；DB 级对照见 .dev-logs/pay02 的 E2E 证据")
	}
	for _, g := range gifts {
		if g.ShareRate != int64(chargeRate) {
			t.Fatalf("口径不一致：充电 %d‰ vs LIV-04 礼物快照 %d‰（spec CHG-02 要求 70/30 一致）",
				chargeRate, g.ShareRate)
		}
	}
}

// TestConcurrentWithdrawNoOverdraft 验收项 ③（**本卡核心**）：
// 余额 100 分，20 并发各提 10 分 → **恰好 10 笔成功、余额归零、冻结 100、无超提**。
//
// 与 LIV-04 `TestGiftConcurrentSpend` 同写法（同一口径要求）：
// 行锁把并发请求在余额行上串行化，故成功笔数恰为 floor(余额 / 单笔)。
func TestConcurrentWithdrawNoOverdraft(t *testing.T) {
	db := testDB(t)
	cleanupCharge(t, db)
	t.Cleanup(func() { cleanupCharge(t, db) })

	svc, _ := newTestSvc(t, db)
	const creator = int64(92700031)
	seedUser(t, db, creator)
	if err := svc.repo.CreditBalanceForTest(creator, 100); err != nil {
		t.Fatalf("造初始余额失败: %v", err)
	}

	const workers, each = 20, 10
	var wg sync.WaitGroup
	var mu sync.Mutex
	success, insufficient := 0, 0
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			w := &WithdrawOrder{
				WithdrawNo: fmt.Sprintf("WTESTCONC%02d", i), UserID: creator, Amount: each,
				Channel: pay.ChannelMock, Account: "a**z", AccountName: "测试", Status: WithdrawAuditing,
			}
			// maxPerDay=0：本用例只验证「余额行锁不超提」这一不变量，
			// 故意关掉频次限制，让 20 笔全部参与对余额的竞争（频次限制另有专门用例）。
			err := svc.repo.ApplyWithdrawTx(w, 0)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				success++
			case errors.Is(err, errcode.ErrCreatorBalanceNotEnough):
				insufficient++
			default:
				t.Errorf("意外错误: %v", err)
			}
		}(i)
	}
	wg.Wait()

	if success != 10 {
		t.Fatalf("并发提现成功笔数应为 10（余额 100 / 单笔 10），实际 %d", success)
	}
	if insufficient != workers-10 {
		t.Fatalf("余额不足笔数应为 %d，实际 %d", workers-10, insufficient)
	}
	bal, err := svc.Balance(creator)
	if err != nil {
		t.Fatalf("取余额失败: %v", err)
	}
	if bal.Balance != 0 {
		t.Fatalf("成功 10 笔后可用余额应归零，实际 %d（超提！）", bal.Balance)
	}
	if bal.Frozen != 100 {
		t.Fatalf("冻结金额应为 100，实际 %d", bal.Frozen)
	}
	// 成功笔数 × 单笔 = 冻结金额（资金守恒：不可能有第 11 笔的余地）。
	var wCnt int64
	_ = db.Table("withdraw_order").Where("user_id = ?", creator).Count(&wCnt).Error
	if wCnt != 10 {
		t.Fatalf("落库提现单应为 10 张，实际 %d", wCnt)
	}
}

// TestManualMarkPaidRecordsChargeIncome 管理端「手动补单」对充电同样生效。
//
// 补单（PAY-01 的 `POST /admin/pay/orders/:orderNo/mark-paid`）与真实回调走**同一条**
// 入账链（同一 PaidHook），故补单也必须落充电记录状态与分成收益——
// 否则「用户付了钱但渠道没回调、客服手工补单」这条最常见的兜底路径会漏记创作者收益。
// 本卡不新增补单接口，直接复用 PAY-01 的既有能力（权限点 pay:order），此处证明其可用。
func TestManualMarkPaidRecordsChargeIncome(t *testing.T) {
	db := testDB(t)
	cleanupCharge(t, db)
	t.Cleanup(func() { cleanupCharge(t, db) })

	svc, paySvc := newTestSvc(t, db)
	const viewer, anchor = int64(92700081), int64(92700082)
	seedUser(t, db, viewer)
	seedUser(t, db, anchor)

	order, err := svc.CreateCharge(context.Background(), &CreateChargeReq{
		UserID: viewer, AnchorID: anchor, TierCode: "tier_30", Channel: pay.ChannelMock,
	})
	if err != nil {
		t.Fatalf("下单失败: %v", err)
	}
	// 下单后仍是待支付：既不记收益，也不进鸣谢。
	c, _ := svc.repo.ChargeByOrder(order.OrderNo)
	if c.Status != ChargeUnpaid {
		t.Fatalf("下单后应为待支付，实际 status=%d", c.Status)
	}
	bal, _ := svc.Balance(anchor)
	if bal.Balance != 0 {
		t.Fatalf("未支付不得记收益，实际 %d", bal.Balance)
	}

	// 管理端手动补单。
	res, err := paySvc.ManualMarkPaid(order.OrderNo, 2082340429419778048)
	if err != nil {
		t.Fatalf("手动补单失败: %v", err)
	}
	if !res.Applied {
		t.Fatalf("首次补单应生效，实际 applied=false: %s", res.Message)
	}
	c, _ = svc.repo.ChargeByOrder(order.OrderNo)
	if c.Status != ChargePaid || c.PaidAt == nil {
		t.Fatalf("补单后充电记录应为已支付且有 paid_at，实际 status=%d paid_at=%v", c.Status, c.PaidAt)
	}
	// tier_30 = 3000 分 → 创作者 2100。
	bal, _ = svc.Balance(anchor)
	if bal.Balance != 2100 {
		t.Fatalf("补单后创作者收益应为 2100（3000 的 70%%），实际 %d", bal.Balance)
	}
	inc, err := svc.repo.IncomeByOrder(order.OrderNo)
	if err != nil {
		t.Fatalf("补单未记分成明细: %v", err)
	}
	if inc.Amount != 2100 || inc.PlatformAmount != 900 {
		t.Fatalf("补单分成应 2100/900，实际 %d/%d", inc.Amount, inc.PlatformAmount)
	}
	// 重复补单：**必须被拒（92002）而不是静默幂等**。
	// 与回调重放的口径差异是有意的：渠道重投是常态，故回调必须幂等返回成功；
	// 而补单是运营人员的显式动作，第二次点击应当明确告知「该单已支付」，
	// 否则操作者无法判断自己的操作是否生效。两种路径的资金结果一致：绝不重复入账。
	_, err = paySvc.ManualMarkPaid(order.OrderNo, 2082340429419778048)
	if errCode(err) != 92002 {
		t.Fatalf("重复补单应报 92002（状态不允许），实际 %v", err)
	}
	bal, _ = svc.Balance(anchor)
	var incCnt int64
	_ = db.Table("creator_income").Where("order_no = ?", order.OrderNo).Count(&incCnt).Error
	if bal.Balance != 2100 || incCnt != 1 {
		t.Fatalf("重复补单不得重复入账：余额=%d（期望 2100）分成笔数=%d（期望 1）", bal.Balance, incCnt)
	}
}

// TestConcurrentWithdrawRespectsDailyLimit 并发下**日限不可被绕过**。
//
// 余额充裕（10000 分）而日限为 3：20 个并发申请必须恰好成功 3 笔。
// 这条用例的价值在于它测的是「事务外预检」的经典漏洞——若频次判定不在持锁事务内，
// 20 个请求会同时读到「0 次」而全部通过，日限形同虚设。
func TestConcurrentWithdrawRespectsDailyLimit(t *testing.T) {
	db := testDB(t)
	cleanupCharge(t, db)
	t.Cleanup(func() { cleanupCharge(t, db) })

	payCfg := pay.DefaultConfig()
	payCfg.MockSecret = "test-charge-secret"
	paySvc := pay.NewService(pay.NewRepo(db), payCfg, zap.NewNop())
	cfg := DefaultConfig()
	cfg.MinWithdrawAmount = 1
	cfg.MaxWithdrawPerDay = 3
	svc := NewService(NewRepo(db), paySvc, cfg, zap.NewNop())

	const creator = int64(92700035)
	seedUser(t, db, creator)
	if err := svc.repo.CreditBalanceForTest(creator, 10000); err != nil {
		t.Fatalf("造初始余额失败: %v", err)
	}

	const workers = 20
	var wg sync.WaitGroup
	var mu sync.Mutex
	success, limited := 0, 0
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			w := &WithdrawOrder{
				WithdrawNo: fmt.Sprintf("WTESTLIM%02d", i), UserID: creator, Amount: 100,
				Channel: pay.ChannelMock, Account: "a**z", AccountName: "测试", Status: WithdrawAuditing,
			}
			err := svc.repo.ApplyWithdrawTx(w, cfg.MaxWithdrawPerDay)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				success++
			case errCode(err) == 92304:
				limited++
			default:
				t.Errorf("意外错误: %v", err)
			}
		}(i)
	}
	wg.Wait()

	if success != 3 {
		t.Fatalf("日限 3 时并发申请应恰成功 3 笔，实际 %d（日限被绕过）", success)
	}
	if limited != workers-3 {
		t.Fatalf("被日限拒绝的应为 %d 笔，实际 %d", workers-3, limited)
	}
	var cnt int64
	_ = db.Table("withdraw_order").Where("user_id = ?", creator).Count(&cnt).Error
	if cnt != 3 {
		t.Fatalf("落库提现单应恰 3 张，实际 %d", cnt)
	}
	bal, _ := svc.Balance(creator)
	if bal.Balance != 9700 || bal.Frozen != 300 {
		t.Fatalf("余额应为 9700/300，实际 %d/%d", bal.Balance, bal.Frozen)
	}
}

// TestWithdrawStateMachine 验收项 ④：审核通过/拒绝/打款路径正确；重复打款与重复审核被拦。
func TestWithdrawStateMachine(t *testing.T) {
	db := testDB(t)
	cleanupCharge(t, db)
	t.Cleanup(func() { cleanupCharge(t, db) })

	svc, _ := newTestSvc(t, db)
	const creator = int64(92700041)
	seedUser(t, db, creator)
	if err := svc.repo.CreditBalanceForTest(creator, 1000); err != nil {
		t.Fatalf("造初始余额失败: %v", err)
	}
	const adminID = int64(2082340429419778048)

	// 路径 A：申请 → 通过 → 打款（终态）。
	wA, err := svc.ApplyWithdraw(creator, 300, "alipay@test", "测试创作者")
	if err != nil {
		t.Fatalf("提现申请失败: %v", err)
	}
	if wA.Status != WithdrawAuditing {
		t.Fatalf("申请后状态应为审核中，实际 %s", wA.StatusText)
	}
	bal, _ := svc.Balance(creator)
	if bal.Balance != 700 || bal.Frozen != 300 {
		t.Fatalf("申请后应 700/300，实际 %d/%d", bal.Balance, bal.Frozen)
	}
	// 重复审核：第二个请求必须被拦（否则会二次解冻/二次出款）。
	if _, err := svc.AdminAuditWithdraw(wA.WithdrawNo, true, adminID, "通过"); err != nil {
		t.Fatalf("首次审核应成功: %v", err)
	}
	if _, err := svc.AdminAuditWithdraw(wA.WithdrawNo, true, adminID, "重复"); errCode(err) != 92303 {
		t.Fatalf("重复审核应被拦为 92303，实际 %v", err)
	}
	paid, err := svc.AdminPayout(wA.WithdrawNo, false, "")
	if err != nil {
		t.Fatalf("打款失败: %v", err)
	}
	if paid.Status != WithdrawPaid {
		t.Fatalf("打款后应为已打款，实际 %s", paid.StatusText)
	}
	// 重复打款：必须被拦，且打款流水恰 1 条（双闸）。
	if _, err := svc.AdminPayout(wA.WithdrawNo, false, ""); errCode(err) != 92303 {
		t.Fatalf("重复打款应被拦为 92303，实际 %v", err)
	}
	n, _ := svc.repo.PayoutLogCount(wA.WithdrawNo)
	if n != 1 {
		t.Fatalf("打款流水应恰 1 条，实际 %d（重复出款！）", n)
	}
	bal, _ = svc.Balance(creator)
	if bal.Balance != 700 || bal.Frozen != 0 {
		t.Fatalf("打款后应 700/0（冻结出清），实际 %d/%d", bal.Balance, bal.Frozen)
	}

	// 路径 B：申请 → 驳回 → 冻结解冻返还。
	wB, err := svc.ApplyWithdraw(creator, 200, "bank@test", "测试创作者")
	if err != nil {
		t.Fatalf("第二次提现申请失败: %v", err)
	}
	bal, _ = svc.Balance(creator)
	if bal.Balance != 500 || bal.Frozen != 200 {
		t.Fatalf("第二次申请后应 500/200，实际 %d/%d", bal.Balance, bal.Frozen)
	}
	rejected, err := svc.AdminAuditWithdraw(wB.WithdrawNo, false, adminID, "账户信息有误")
	if err != nil {
		t.Fatalf("驳回失败: %v", err)
	}
	if rejected.Status != WithdrawRejected {
		t.Fatalf("应为已拒绝，实际 %s", rejected.StatusText)
	}
	bal, _ = svc.Balance(creator)
	if bal.Balance != 700 || bal.Frozen != 0 {
		t.Fatalf("驳回后应解冻返还为 700/0，实际 %d/%d", bal.Balance, bal.Frozen)
	}
	// 已拒绝的单不可打款。
	if _, err := svc.AdminPayout(wB.WithdrawNo, false, ""); errCode(err) != 92303 {
		t.Fatalf("已拒绝单打款应被拦，实际 %v", err)
	}

	// 路径 C：通过后打款失败 → 解冻返还。
	wC, err := svc.ApplyWithdraw(creator, 100, "bank@test", "测试创作者")
	if err != nil {
		t.Fatalf("第三次提现申请失败: %v", err)
	}
	if _, err := svc.AdminAuditWithdraw(wC.WithdrawNo, true, adminID, ""); err != nil {
		t.Fatalf("审核失败: %v", err)
	}
	failed, err := svc.AdminPayout(wC.WithdrawNo, true, "通道超时")
	if err != nil {
		t.Fatalf("打款失败路径不应报错: %v", err)
	}
	if failed.Status != WithdrawPayFail {
		t.Fatalf("应为打款失败，实际 %s", failed.StatusText)
	}
	bal, _ = svc.Balance(creator)
	if bal.Balance != 700 || bal.Frozen != 0 {
		t.Fatalf("打款失败后应解冻返还 700/0，实际 %d/%d", bal.Balance, bal.Frozen)
	}

	// 门槛与频次：低于门槛、超出 24h 次数上限都要被拦。
	if _, err := svc.ApplyWithdraw(creator, 0, "a@b", "x"); errCode(err) != 92302 {
		t.Fatalf("低于门槛应报 92302，实际 %v", err)
	}
	if _, err := svc.ApplyWithdraw(creator, 100, "a@b", "x"); errCode(err) != 92304 {
		t.Fatalf("超出 24h 次数上限应报 92304，实际 %v", err)
	}
}

// TestMonthlySettlement 验收项 ⑤：造跨月数据 → 结算单聚合正确（金额 = Σ 各笔分成）。
func TestMonthlySettlement(t *testing.T) {
	db := testDB(t)
	cleanupCharge(t, db)
	t.Cleanup(func() { cleanupCharge(t, db) })

	svc, _ := newTestSvc(t, db)
	const creator, viewer = int64(92700051), int64(92700052)
	const videoA, videoB = int64(92700053), int64(92700054)
	seedUser(t, db, creator)
	seedUser(t, db, viewer)
	seedVideo(t, db, videoA, creator)
	seedVideo(t, db, videoB, creator)

	// 造跨月数据：上月 2 笔（10 元 + 30 元）、本月 1 笔（10 元）。
	// 归属按 charge_log.paid_at / creator_income.paid_at，故直接改写 paid_at 模拟历史月份。
	lastM := time.Now().AddDate(0, -1, 0)
	mkIncome := func(videoID, amount int64, paidAt time.Time, tag string) {
		t.Helper()
		c := &ChargeLog{
			ChargeNo: "CSETTLE" + tag, OrderNo: "OSETTLE" + tag,
			UserID: viewer, AnchorID: creator, VideoID: videoID,
			Amount: amount, ShareRate: 700, Status: ChargePaid, PaidAt: &paidAt,
		}
		if err := svc.repo.CreateCharge(c); err != nil {
			t.Fatalf("造充电记录失败: %v", err)
		}
		inc := &CreatorIncome{
			UserID: creator, SourceType: IncomeSourceCharge, SourceID: c.ID,
			OrderNo: c.OrderNo, VideoID: videoID,
			Amount: amount * 700 / 1000, PlatformAmount: amount - amount*700/1000,
			ShareRate: 700, PaidAt: &paidAt,
		}
		if err := db.Create(inc).Error; err != nil {
			t.Fatalf("造分成明细失败: %v", err)
		}
	}
	mkIncome(videoA, 1000, lastM, "A")      // 上月稿 A 10 元 → 700
	mkIncome(videoB, 3000, lastM, "B")      // 上月稿 B 30 元 → 2100
	mkIncome(videoA, 1000, time.Now(), "C") // 本月稿 A 10 元 → 700

	// 结算上一自然月：应恰 2 笔、创作者 2800 分、平台 1200 分。
	res, err := svc.AdminSettleMonth(lastM.Format("2006-01"))
	if err != nil {
		t.Fatalf("月结失败: %v", err)
	}
	if res.ChargeCount != 2 || res.Amount != 2800 || res.Platform != 1200 {
		t.Fatalf("上月结算应 2 笔/2800/1200，实际 %d/%d/%d", res.ChargeCount, res.Amount, res.Platform)
	}
	// 结算单金额 = Σ 该月各笔分成（独立 SQL 核对，不信自report）。
	sum, err := svc.repo.SettledChargeSum(creator, lastM.Format("2006-01"))
	if err != nil {
		t.Fatalf("核对结算金额失败: %v", err)
	}
	if sum != 2800 {
		t.Fatalf("结算单金额应等于该月分成合计 2800，实际 %d", sum)
	}
	// 按稿聚合写入 creator_settlement（source='charge'）：稿 A 700、稿 B 2100。
	type row struct {
		VideoID int64
		Amount  int64
		Cnt     int64
	}
	var rows []row
	_ = db.Raw(`SELECT video_id, amount, charge_count AS cnt FROM creator_settlement
		WHERE user_id = ? AND source = 'charge' AND settle_date = ?`,
		creator, lastM.Format("2006-01")+"-01").Scan(&rows).Error
	if len(rows) != 2 {
		t.Fatalf("应按稿聚合出 2 行，实际 %d 行", len(rows))
	}
	total := int64(0)
	for _, r := range rows {
		total += r.Amount
	}
	if total != 2800 {
		t.Fatalf("按稿聚合合计应 2800，实际 %d", total)
	}
	// 幂等：重跑同一月不重复计额（本月那笔不得被并入）。
	res2, err := svc.AdminSettleMonth(lastM.Format("2006-01"))
	if err != nil {
		t.Fatalf("月结重跑失败: %v", err)
	}
	if res2.Amount != res.Amount || res2.ChargeCount != res.ChargeCount {
		t.Fatalf("月结可重放：重跑应得同值 %d/%d，实际 %d/%d",
			res.Amount, res.ChargeCount, res2.Amount, res2.ChargeCount)
	}
	// 未结算余额可查：本月那笔仍未结算 → unsettled = 700。
	bal, _ := svc.Balance(creator)
	if bal.Unsettled != 700 {
		t.Fatalf("未结算余额应 700（本月那笔），实际 %d", bal.Unsettled)
	}
}

// TestThanksListAnonymous 验收项 ⑥：鸣谢名单返回最近充电者；匿名条目不得泄露身份。
func TestThanksListAnonymous(t *testing.T) {
	db := testDB(t)
	cleanupCharge(t, db)
	t.Cleanup(func() { cleanupCharge(t, db) })

	svc, paySvc := newTestSvc(t, db)
	const anchor, viewer1, viewer2, video = int64(92700061), int64(92700062), int64(92700063), int64(92700064)
	seedUser(t, db, anchor)
	seedUser(t, db, viewer1)
	seedUser(t, db, viewer2)
	seedVideo(t, db, video, anchor)

	// 实名一笔 + 匿名一笔 + 未支付一笔（不应出现在名单）。
	chargeOnce(t, svc, paySvc, viewer1, anchor, video, "tier_5", 0)
	if err := db.Exec(`UPDATE `+"`user`"+` SET nickname = ? WHERE id = ?`, "充电者一号", viewer1).Error; err != nil {
		t.Fatalf("改昵称失败: %v", err)
	}
	order, err := svc.CreateCharge(context.Background(), &CreateChargeReq{
		UserID: viewer2, AnchorID: anchor, VideoID: video, TierCode: "tier_5",
		Anonymous: true, Message: "加油", Channel: pay.ChannelMock,
	})
	if err != nil {
		t.Fatalf("匿名充电下单失败: %v", err)
	}
	if _, err := paySvc.DevCallback(order.OrderNo, order.Amount, "success"); err != nil {
		t.Fatalf("匿名充电回调失败: %v", err)
	}
	// 未支付一笔（只下单不回调）。
	if _, err := svc.CreateCharge(context.Background(), &CreateChargeReq{
		UserID: viewer1, AnchorID: anchor, VideoID: video, TierCode: "tier_5", Channel: pay.ChannelMock,
	}); err != nil {
		t.Fatalf("未支付充电下单失败: %v", err)
	}

	list, err := svc.ThanksList(anchor, video, 50)
	if err != nil {
		t.Fatalf("鸣谢名单失败: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("鸣谢名单应恰 2 条（未支付不计入），实际 %d", len(list))
	}
	var anon, named int
	for _, it := range list {
		if it.Anonymous {
			anon++
			if it.Nickname != "匿名用户" {
				t.Fatalf("匿名条目昵称应为「匿名用户」，实际 %q", it.Nickname)
			}
		} else {
			named++
			if it.Nickname != "充电者一号" {
				t.Fatalf("实名条目应带昵称，实际 %q", it.Nickname)
			}
		}
	}
	if anon != 1 || named != 1 {
		t.Fatalf("应 1 匿名 + 1 实名，实际 %d/%d", anon, named)
	}
	// 鸣谢金额：tier_5 = 500 分。
	if list[0].Amount != 500 {
		t.Fatalf("鸣谢金额应为档位金额 500 分，实际 %d", list[0].Amount)
	}
}

// TestChargeValidation 边界：自充/档位/金额/稿件归属/留言长度。
func TestChargeValidation(t *testing.T) {
	db := testDB(t)
	cleanupCharge(t, db)
	t.Cleanup(func() { cleanupCharge(t, db) })

	svc, _ := newTestSvc(t, db)
	const anchor, viewer, video = int64(92700071), int64(92700072), int64(92700073)
	seedUser(t, db, anchor)
	seedUser(t, db, viewer)
	seedVideo(t, db, video, anchor)
	const video2 = int64(92700074) // 属于 viewer 而非 anchor 的稿件
	seedVideo(t, db, video2, viewer)

	// 断言按**错误码**而非指针相等：Service 用 WithMsg 附加了上下文文案
	// （码相同、文案更具体），指针比较会把正确实现误判为失败。
	cases := []struct {
		name string
		req  CreateChargeReq
		want int
	}{
		{"自充被拒", CreateChargeReq{UserID: anchor, AnchorID: anchor, TierCode: "tier_5"}, 92203},
		{"档位不存在", CreateChargeReq{UserID: viewer, AnchorID: anchor, TierCode: "nope"}, 92201},
		{"自定义档低于下限", CreateChargeReq{UserID: viewer, AnchorID: anchor, TierCode: TierCustomCode, Amount: 10}, 92202},
		{"自定义档超上限", CreateChargeReq{UserID: viewer, AnchorID: anchor, TierCode: TierCustomCode, Amount: 999999}, 92202},
		{"UP 主不存在", CreateChargeReq{UserID: viewer, AnchorID: 99999999, TierCode: "tier_5"}, 92204},
		{"稿件不存在", CreateChargeReq{UserID: viewer, AnchorID: anchor, VideoID: 92700099, TierCode: "tier_5"}, 92204},
		// 稿件存在但归属他人：拿别人的稿件给 A 充电会让收益记错人，必须拒绝（10002 参数不合法）。
		{"稿件不属于该 UP 主", CreateChargeReq{UserID: viewer, AnchorID: anchor, VideoID: video2, TierCode: "tier_5"}, 10002},
	}
	for _, tc := range cases {
		req := tc.req
		req.Channel = pay.ChannelMock
		_, err := svc.CreateCharge(context.Background(), &req)
		if got := errCode(err); got != tc.want {
			t.Errorf("%s: 期望码 %d，实际码 %d（%v）", tc.name, tc.want, got, err)
		}
	}
	// 合法自定义档应通过并取客户端金额。
	_, err := svc.CreateCharge(context.Background(), &CreateChargeReq{
		UserID: viewer, AnchorID: anchor, TierCode: TierCustomCode, Amount: 1234, Channel: pay.ChannelMock,
	})
	if err != nil {
		t.Fatalf("合法自定义充电应成功，实际 %v", err)
	}
	// 留言超长。
	long := make([]rune, 201)
	for i := range long {
		long[i] = 'a'
	}
	_, err = svc.CreateCharge(context.Background(), &CreateChargeReq{
		UserID: viewer, AnchorID: anchor, TierCode: "tier_5", Message: string(long), Channel: pay.ChannelMock,
	})
	if !errors.Is(err, errcode.ErrChargeMessageTooLong) {
		t.Fatalf("超长留言应报 92207，实际 %v", err)
	}
}
