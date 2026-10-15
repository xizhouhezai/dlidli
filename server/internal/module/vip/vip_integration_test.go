package vip

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dlidli/server/internal/module/pay"
	"github.com/dlidli/server/internal/pkg/errcode"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// M4-VIP-01 集成测试（真实 MySQL）：覆盖验收核心 —— 开通全链路、顺延（非覆盖）、
// 订单幂等、权益判定、过期判免费、月度赠币幂等、自动续期成功/失败/解约。
// 未设置 DLIDLI_TEST_DSN 时跳过（沿用 pay / live 包既有口径）。

// vipTestDSN 会员集成测试专用库的 DSN。
//
// **为什么要独立库而不是复用主测试库**：本包的测试会真实走支付链路并产生
// `pay_order` / `cash_log` 行，而 pay 包的对账用例按「当天 × 渠道」聚合
// `cash_log`（该表没有 channel 列，聚合口径是全局当日）。两个包并行跑时
// vip 的造数会让 pay 的 `diff_count` 断言凭空多出差异 —— 实测踩到
// （`go test ./...` 下 TestReconFindsIssues 失败，单包跑则通过）。
// 用独立库把两个包的测试数据彻底隔离，而不是靠「不要并行」这种口头约定。
func vipTestDSN(t *testing.T) string {
	t.Helper()
	base := os.Getenv("DLIDLI_TEST_DSN")
	if base == "" {
		t.Skip("DLIDLI_TEST_DSN 未设置，跳过 vip 集成测试")
	}
	if dsn := os.Getenv("DLIDLI_VIP_TEST_DSN"); dsn != "" {
		return dsn
	}
	// 从主测试库 DSN 派生：把库名替换为 dlidli_vip_test（见 README 的建库命令）。
	// 用「最后一个 / 之后」定位库名，避免误伤密码或 tcp(...) 里的斜杠。
	i := strings.LastIndex(base, "/")
	if i < 0 {
		t.Skipf("无法从 DLIDLI_TEST_DSN 派生独立测试库: %s", base)
	}
	rest := base[i+1:]
	j := strings.Index(rest, "?")
	if j < 0 {
		return base[:i+1] + "dlidli_vip_test"
	}
	return base[:i+1] + "dlidli_vip_test" + rest[j:]
}

func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := vipTestDSN(t)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), TranslateError: true,
	})
	if err != nil {
		t.Fatalf("已设置 DSN 但无法连接 vip 测试库: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("获取底层连接失败: %v", err)
	}
	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("vip 测试库 ping 失败（是否已建库并跑迁移？见 .dev-logs/vip01/README.md）: %v", err)
	}
	return db
}

// 测试用户段 92500xxx：刻意避开 92600xxx（pay 集成测试用）与 92000xxx（硬币测试用），
// 避免清理逻辑互相干扰（pay 的 cleanupPay 会按区间删 coin_log）。
const (
	testUIDBase = int64(92500000)
	testUIDEnd  = int64(92599999)
)

// seedUser 造测试用户。昵称必须带 uid：`user.uk_nickname` 唯一。
func seedUser(t *testing.T, db *gorm.DB, uid int64) {
	t.Helper()
	if err := db.Exec("INSERT INTO `user` (id, nickname, status, coin, created_at, updated_at) VALUES (?, ?, 0, 0, NOW(), NOW())",
		uid, "viptest-"+strconv.FormatInt(uid, 10)).Error; err != nil {
		t.Fatalf("造用户失败: %v", err)
	}
}

// cleanupVip 清理会员域测试数据（顺序：先台账后主表，避免外键式残留影响计数断言）。
func cleanupVip(t *testing.T, db *gorm.DB) {
	t.Helper()
	_ = db.Exec(`DELETE FROM vip_coin_grant WHERE user_id BETWEEN ? AND ?`, testUIDBase, testUIDEnd).Error
	_ = db.Exec(`DELETE FROM vip_subscription WHERE user_id BETWEEN ? AND ?`, testUIDBase, testUIDEnd).Error
	_ = db.Exec(`DELETE FROM vip_order WHERE user_id BETWEEN ? AND ?`, testUIDBase, testUIDEnd).Error
	_ = db.Exec(`DELETE FROM membership WHERE user_id BETWEEN ? AND ?`, testUIDBase, testUIDEnd).Error
	_ = db.Exec(`DELETE FROM coin_log WHERE user_id BETWEEN ? AND ?`, testUIDBase, testUIDEnd).Error
	// 测试产生的支付订单一并清掉（否则 pay 的订单查询类断言会被本测试数据污染）。
	_ = db.Exec(`DELETE FROM cash_log WHERE user_id BETWEEN ? AND ?`, testUIDBase, testUIDEnd).Error
	_ = db.Exec(`DELETE FROM cash_account WHERE user_id BETWEEN ? AND ?`, testUIDBase, testUIDEnd).Error
	_ = db.Exec(`DELETE FROM pay_callback_log WHERE order_no IN
		(SELECT order_no FROM pay_order WHERE user_id BETWEEN ? AND ?)`, testUIDBase, testUIDEnd).Error
	_ = db.Exec(`DELETE FROM pay_order WHERE user_id BETWEEN ? AND ?`, testUIDBase, testUIDEnd).Error
	_ = db.Exec(`DELETE FROM `+"`user`"+` WHERE id BETWEEN ? AND ?`, testUIDBase, testUIDEnd).Error
}

// seedPlans 显式 upsert 测试套餐。
//
// 不依赖 router 装配时的 SeedDefaultPlans：那是「表空才播种」的启动逻辑，
// 一旦表里已有运营改过的套餐（或跑过 API 的库），测试就会因取不到 fixture 而失败——
// 测试必须能自证其前置条件，不能依赖另一个进程曾经启动过。
func seedPlans(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, p := range DefaultPlans() {
		if err := db.Exec(`INSERT INTO vip_plan (code, name, months, price, original_price, first_month_price, auto_renew, sort, status, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
			ON DUPLICATE KEY UPDATE name=VALUES(name), months=VALUES(months), price=VALUES(price),
			  original_price=VALUES(original_price), first_month_price=VALUES(first_month_price),
			  auto_renew=VALUES(auto_renew), sort=VALUES(sort), status=VALUES(status)`,
			p.Code, p.Name, p.Months, p.Price, p.OriginalPrice, p.FirstMonthPrice,
			p.AutoRenew, p.Sort, p.Status).Error; err != nil {
			t.Fatalf("播种套餐 %s 失败: %v", p.Code, err)
		}
	}
}

// newVipSvc 构造「pay + vip」联动装配（与 router 装配一致：钩子双向注入）。
func newVipSvc(t *testing.T, db *gorm.DB) (*Service, *pay.Service) {
	t.Helper()
	seedPlans(t, db)
	payCfg := pay.DefaultConfig()
	payCfg.MockSecret = "vip-integration-secret"
	paySvc := pay.NewService(pay.NewRepo(db), payCfg, zap.NewNop())
	cfg := DefaultConfig()
	cfg.RenewEnabled = false
	svc := NewService(NewRepo(db), paySvc, cfg, zap.NewNop())
	paySvc.SetSkuHook(svc)
	paySvc.SetPaidHook(svc.ApplyPaidHook)
	return svc, paySvc
}

// payAndCallback 走完整支付链：下单 → mock 渠道回调（与真实渠道同一处理链）。
func payAndCallback(t *testing.T, svc *Service, paySvc *pay.Service, uid int64, planCode string) *pay.OrderView {
	t.Helper()
	o, err := svc.CreateVipOrder(context.Background(), uid, planCode, pay.ChannelMock)
	if err != nil {
		t.Fatalf("会员下单失败: %v", err)
	}
	res, err := paySvc.DevCallback(o.OrderNo, o.Amount, "success")
	if err != nil {
		t.Fatalf("回调失败: %v", err)
	}
	if !res.Applied {
		t.Fatalf("回调未生效: %+v", res)
	}
	return o
}

// TestOpenAndExtend 开通 + 顺延（验收项 ①②）。
//
// 核心断言：第二次购买后 expire_at **不是**覆盖成 now+1月，而是 首次到期时间 + 1月。
// 这是「顺延语义」与「覆盖语义」唯一可区分的观测点。
func TestOpenAndExtend(t *testing.T) {
	db := testDB(t)
	cleanupVip(t, db)
	t.Cleanup(func() { cleanupVip(t, db) })
	svc, paySvc := newVipSvc(t, db)
	uid := testUIDBase + 1
	seedUser(t, db, uid)

	// 首次开通（月度）
	o1 := payAndCallback(t, svc, paySvc, uid, "vip_month")
	m1, err := svc.repo.GetMembership(uid)
	if err != nil || m1 == nil {
		t.Fatalf("开通后应存在 membership 行: %v", err)
	}
	first := m1.ExpireAt
	// 首次开通 ≈ now + 1 月（允许 2 分钟误差）
	if d := time.Until(first); d < 27*24*time.Hour || d > 32*24*time.Hour {
		t.Fatalf("首次到期时间应约为 1 个月后, got %v (剩余 %v)", first, d)
	}

	st, err := svc.Status(uid)
	if err != nil {
		t.Fatalf("status 失败: %v", err)
	}
	if !st.IsVip || st.Rights.MaxResolution != "4k" || !st.Rights.Badge || st.Rights.CoinPerMonth != 5 {
		t.Fatalf("会员权益位不正确: %+v", st)
	}

	// 未过期时再买一个月 → 顺延（不是覆盖）
	payAndCallback(t, svc, paySvc, uid, "vip_month")
	m2, err := svc.repo.GetMembership(uid)
	if err != nil || m2 == nil {
		t.Fatalf("续期后应存在 membership 行: %v", err)
	}
	extended := m2.ExpireAt
	delta := extended.Sub(first)
	// 顺延一个月 ≈ 28~31 天；若为覆盖，delta 会 ≈ 0（甚至是负值）
	if delta < 27*24*time.Hour || delta > 32*24*time.Hour {
		t.Fatalf("顺延失败：到期时间应从 %v 顺延约 1 个月，实际 %v（增量 %v）", first, extended, delta)
	}
	if m2.TotalMonths != 2 {
		t.Fatalf("累计月数应为 2, got %d", m2.TotalMonths)
	}

	// 台账：两笔订单都已生效，且记录了顺延前后的精确值（审计凭据）
	row1, _ := svc.repo.GetOrderRow(o1.OrderNo)
	if row1 == nil || row1.ApplyStatus != ApplyDone || row1.ExpireBefore != nil {
		t.Fatalf("首单台账应为已生效且 expire_before=NULL: %+v", row1)
	}
	if row1.ExpireAfter == nil || !row1.ExpireAfter.Equal(first) {
		t.Fatalf("首单台账 expire_after 应为 %v, got %v", first, row1.ExpireAfter)
	}
}

// TestApplyIdempotent 订单幂等（验收项 ③）：同一订单重放回调只生效一次。
func TestApplyIdempotent(t *testing.T) {
	db := testDB(t)
	cleanupVip(t, db)
	t.Cleanup(func() { cleanupVip(t, db) })
	svc, paySvc := newVipSvc(t, db)
	uid := testUIDBase + 2
	seedUser(t, db, uid)

	o, err := svc.CreateVipOrder(context.Background(), uid, "vip_month", pay.ChannelMock)
	if err != nil {
		t.Fatalf("下单失败: %v", err)
	}
	// 首次回调
	res, err := paySvc.DevCallback(o.OrderNo, o.Amount, "success")
	if err != nil || !res.Applied {
		t.Fatalf("首次回调应生效: %+v err=%v", res, err)
	}
	m1, _ := svc.repo.GetMembership(uid)
	first := m1.ExpireAt
	coins1 := coinOf(t, db, uid)
	grants1 := countGrants(t, db, uid)

	// 重放 5 次
	for i := 0; i < 5; i++ {
		res, err := paySvc.DevCallback(o.OrderNo, o.Amount, "success")
		if err != nil {
			t.Fatalf("第 %d 次重放报错: %v", i+1, err)
		}
		if res.Applied {
			t.Fatalf("第 %d 次重放不应再次生效（applied 应为 false）", i+1)
		}
	}

	m2, _ := svc.repo.GetMembership(uid)
	if !m2.ExpireAt.Equal(first) {
		t.Fatalf("重放后到期时间不应变化: %v → %v", first, m2.ExpireAt)
	}
	if got := coinOf(t, db, uid); got != coins1 {
		t.Fatalf("重放后硬币不应变化: %d → %d", coins1, got)
	}
	if got := countGrants(t, db, uid); got != grants1 || got != 1 {
		t.Fatalf("重放后赠币台账应恰 1 条, got %d", got)
	}
	// 该订单的现金流水也应恰 1 条（PAY-01 幂等未被本卡破坏）
	var n int64
	if err := db.Model(&pay.CashLog{}).Where("order_no = ?", o.OrderNo).Count(&n).Error; err != nil {
		t.Fatalf("统计现金流水失败: %v", err)
	}
	if n != 1 {
		t.Fatalf("同一订单现金流水应恰 1 条, got %d", n)
	}
}

// TestExpiredUserIsFree 过期用户判为免费（验收项 ④）。
func TestExpiredUserIsFree(t *testing.T) {
	db := testDB(t)
	cleanupVip(t, db)
	t.Cleanup(func() { cleanupVip(t, db) })
	svc, paySvc := newVipSvc(t, db)
	uid := testUIDBase + 3
	seedUser(t, db, uid)

	// 未开通 → 免费
	st, err := svc.Status(uid)
	if err != nil {
		t.Fatalf("status 失败: %v", err)
	}
	if st.IsVip || st.Rights.MaxResolution != "1080p" || st.Rights.Badge {
		t.Fatalf("未开通用户应为免费权益: %+v", st.Rights)
	}

	// 开通后手动把到期时间改到过去 → 必须判为免费
	payAndCallback(t, svc, paySvc, uid, "vip_month")
	if err := db.Exec("UPDATE membership SET expire_at = DATE_SUB(NOW(), INTERVAL 1 DAY) WHERE user_id = ?", uid).Error; err != nil {
		t.Fatalf("改到期时间失败: %v", err)
	}
	st2, err := svc.Status(uid)
	if err != nil {
		t.Fatalf("status 失败: %v", err)
	}
	if st2.IsVip {
		t.Fatalf("已过期用户必须判为免费, got %+v", st2)
	}
	if st2.Rights.MaxResolution != "1080p" || st2.Rights.HDR {
		t.Fatalf("过期用户权益位应为免费档: %+v", st2.Rights)
	}
	// 过期后购买：从 now() 起算，而不是从过去的到期时间顺延
	payAndCallback(t, svc, paySvc, uid, "vip_month")
	m, _ := svc.repo.GetMembership(uid)
	if d := time.Until(m.ExpireAt); d < 27*24*time.Hour || d > 32*24*time.Hour {
		t.Fatalf("过期后购买应从 now() 起算约 1 个月, got %v (剩余 %v)", m.ExpireAt, d)
	}
}

// TestMonthlyCoinGrantIdempotent 月度赠币幂等（验收项 ⑦）。
func TestMonthlyCoinGrantIdempotent(t *testing.T) {
	db := testDB(t)
	cleanupVip(t, db)
	t.Cleanup(func() { cleanupVip(t, db) })
	svc, paySvc := newVipSvc(t, db)
	uid := testUIDBase + 4
	seedUser(t, db, uid)

	payAndCallback(t, svc, paySvc, uid, "vip_month")
	if got := coinOf(t, db, uid); got != 5 {
		t.Fatalf("开通后应赠 5 枚硬币, got %d", got)
	}
	if got := countGrants(t, db, uid); got != 1 {
		t.Fatalf("赠币台账应恰 1 条, got %d", got)
	}

	// 直接重复调用发放（模拟定时补发与运维手工补发并发/重复触发）
	granted, err := svc.GrantCoinsFor(uid, GrantByWorker)
	if err != nil {
		t.Fatalf("重复发放报错: %v", err)
	}
	if granted {
		t.Fatalf("同月重复发放应被幂等拒绝（granted=false）")
	}
	granted, err = svc.GrantCoinsFor(uid, GrantByAdmin)
	if err != nil || granted {
		t.Fatalf("管理端重复补发也应幂等: granted=%v err=%v", granted, err)
	}
	if got := coinOf(t, db, uid); got != 5 {
		t.Fatalf("重复发放后硬币不应变化, got %d", got)
	}
	if got := countGrants(t, db, uid); got != 1 {
		t.Fatalf("重复发放后台账仍应恰 1 条, got %d", got)
	}
	// 月度补发扫描：该用户本月已发 → 不应被扫出
	g2, _, err := svc.GrantMonthlyOnce()
	if err != nil {
		t.Fatalf("月度补发扫描失败: %v", err)
	}
	if g2 != 0 {
		t.Fatalf("本月已发放的用户不应被补发, got granted=%d", g2)
	}
	// coin_log 恰 1 条（硬币流水口径）
	var n int64
	if err := db.Table("coin_log").Where("user_id = ? AND reason = ?", uid, CoinReasonVIP).Count(&n).Error; err != nil {
		t.Fatalf("统计 coin_log 失败: %v", err)
	}
	if n != 1 {
		t.Fatalf("会员赠币 coin_log 应恰 1 条, got %d", n)
	}
	// **不得**写金瓜子（spec §2.1 两者不同）
	var ms int64
	if err := db.Table("melon_seed_log").Where("user_id = ?", uid).Count(&ms).Error; err == nil && ms != 0 {
		t.Fatalf("会员赠币不得写金瓜子, got %d", ms)
	}
}

// TestAutoRenew 连续包月：签约 → 到期扫描 → 自动续期成功顺延；解约后不再续（验收项 ⑧）。
func TestAutoRenew(t *testing.T) {
	db := testDB(t)
	cleanupVip(t, db)
	t.Cleanup(func() { cleanupVip(t, db) })
	svc, paySvc := newVipSvc(t, db)
	uid := testUIDBase + 5
	seedUser(t, db, uid)

	// 首次开通连续包月套餐（vip_month 带 auto_renew）
	payAndCallback(t, svc, paySvc, uid, "vip_month")
	sub, err := svc.repo.GetSubscription(uid)
	if err != nil || sub == nil {
		t.Fatalf("开通连续包月套餐后应自动签约: %v", err)
	}
	if sub.Status != SubActive {
		t.Fatalf("签约状态应为签约中, got %d", sub.Status)
	}
	m1, _ := svc.repo.GetMembership(uid)
	if !sub.NextChargeAt.Equal(m1.ExpireAt) {
		t.Fatalf("下次扣款时间应等于会员到期时间: %v vs %v", sub.NextChargeAt, m1.ExpireAt)
	}
	coinsBefore := coinOf(t, db, uid)

	// 把到期时间与下次扣款时间推到过去 → 模拟「已到期」
	if err := db.Exec("UPDATE membership SET expire_at = DATE_SUB(NOW(), INTERVAL 1 HOUR) WHERE user_id = ?", uid).Error; err != nil {
		t.Fatalf("改到期时间失败: %v", err)
	}
	if err := db.Exec("UPDATE vip_subscription SET next_charge_at = DATE_SUB(NOW(), INTERVAL 1 HOUR) WHERE user_id = ?", uid).Error; err != nil {
		t.Fatalf("改扣款时间失败: %v", err)
	}

	due, ok, failed, err := svc.RenewOnce(context.Background())
	if err != nil {
		t.Fatalf("续期扫描失败: %v", err)
	}
	if due != 1 || ok != 1 || failed != 0 {
		t.Fatalf("续期应 1 到期 1 成功 0 失败, got due=%d ok=%d failed=%d", due, ok, failed)
	}
	m2, _ := svc.repo.GetMembership(uid)
	if !m2.ExpireAt.After(time.Now()) {
		t.Fatalf("续期后到期时间应在未来, got %v", m2.ExpireAt)
	}
	// 顺延语义：从 max(now, 过期值) 起算一个月
	if d := time.Until(m2.ExpireAt); d < 27*24*time.Hour || d > 32*24*time.Hour {
		t.Fatalf("续期后应顺延约 1 个月, got %v (剩余 %v)", m2.ExpireAt, d)
	}
	// 续期订单同样触发赠币发放，但本月已发 → 幂等不重复
	if got := coinOf(t, db, uid); got != coinsBefore {
		t.Fatalf("同月续期不应重复赠币: %d → %d", coinsBefore, got)
	}
	// 二次扫描：扣款时间已推进到未来 → 不应重复扣
	due2, ok2, _, err := svc.RenewOnce(context.Background())
	if err != nil {
		t.Fatalf("二次扫描失败: %v", err)
	}
	if due2 != 0 || ok2 != 0 {
		t.Fatalf("扣款时间推进后不应再次续期, got due=%d ok=%d", due2, ok2)
	}

	// 解约后不再续
	if err := svc.CancelSubscribe(uid); err != nil {
		t.Fatalf("解约失败: %v", err)
	}
	if err := db.Exec("UPDATE vip_subscription SET next_charge_at = DATE_SUB(NOW(), INTERVAL 1 HOUR) WHERE user_id = ?", uid).Error; err != nil {
		t.Fatalf("改扣款时间失败: %v", err)
	}
	due3, ok3, _, err := svc.RenewOnce(context.Background())
	if err != nil {
		t.Fatalf("解约后扫描失败: %v", err)
	}
	if due3 != 0 || ok3 != 0 {
		t.Fatalf("解约后不应再续期, got due=%d ok=%d", due3, ok3)
	}
	// 重复解约应报 92105（无有效签约），不是静默成功
	if err := svc.CancelSubscribe(uid); !isCode(err, 92105) {
		t.Fatalf("重复解约应返回 92105, got %v", err)
	}
}

// TestRenewFailureAutoCancel 扣款失败达阈值 → 自动解约（连续包月的失败路径）。
func TestRenewFailureAutoCancel(t *testing.T) {
	db := testDB(t)
	cleanupVip(t, db)
	t.Cleanup(func() { cleanupVip(t, db) })
	svc, paySvc := newVipSvc(t, db)
	uid := testUIDBase + 6
	seedUser(t, db, uid)

	payAndCallback(t, svc, paySvc, uid, "vip_month")
	base, _ := svc.repo.GetMembership(uid)
	before := base.ExpireAt
	// 替换扣款实现为恒失败（模拟渠道拒付）
	svc.chargeFn = func(context.Context, int64, string, int64) error {
		return errcode.ErrVipRenewFailed
	}
	svc.cfg.MaxFailCount = 2

	for i := 1; i <= 2; i++ {
		if err := db.Exec("UPDATE vip_subscription SET next_charge_at = DATE_SUB(NOW(), INTERVAL 1 HOUR) WHERE user_id = ?", uid).Error; err != nil {
			t.Fatalf("改扣款时间失败: %v", err)
		}
		due, ok, failed, err := svc.RenewOnce(context.Background())
		if err != nil {
			t.Fatalf("第 %d 轮失败: %v", i, err)
		}
		if due != 1 || ok != 0 || failed != 1 {
			t.Fatalf("第 %d 轮应 1 到期 0 成功 1 失败, got due=%d ok=%d failed=%d", i, due, ok, failed)
		}
		sub, _ := svc.repo.GetSubscription(uid)
		if sub.FailCount != i {
			t.Fatalf("第 %d 轮失败计数应为 %d, got %d", i, i, sub.FailCount)
		}
		if i == 2 && sub.Status != SubCancelled {
			t.Fatalf("连续失败达阈值应自动解约, got status=%d", sub.Status)
		}
	}
	// 失败不发放权益：到期时间**与续期前完全一致**（不得被顺延）
	m, _ := svc.repo.GetMembership(uid)
	if !m.ExpireAt.Equal(before) {
		t.Fatalf("扣款失败不应顺延会员到期时间: %v → %v", before, m.ExpireAt)
	}
	// 失败也不得产生新订单/新流水（扣款未发生）
	var n int64
	if err := db.Model(&pay.PayOrder{}).Where("user_id = ? AND subject LIKE ?", uid, "%自动续期%").Count(&n).Error; err != nil {
		t.Fatalf("统计续期订单失败: %v", err)
	}
	if n != 0 {
		t.Fatalf("扣款失败不应产生续期订单, got %d", n)
	}
}

// TestSkuPriceGuard 客户端金额篡改防护：金额与套餐价不符必须拒绝。
func TestSkuPriceGuard(t *testing.T) {
	db := testDB(t)
	cleanupVip(t, db)
	t.Cleanup(func() { cleanupVip(t, db) })
	svc, paySvc := newVipSvc(t, db)
	uid := testUIDBase + 7
	seedUser(t, db, uid)

	// 直连支付域下单并伪造低金额 + 会员 sku → 应被 CheckSku 拒绝
	_, err := paySvc.CreateOrder(context.Background(), &pay.CreateOrderReq{
		UserID: uid, Amount: 1, Channel: pay.ChannelMock, Sku: "vip_year", Subject: "年卡",
	})
	if !isCode(err, 92008) {
		t.Fatalf("金额篡改应被 92008 拒绝, got %v", err)
	}
	// 未上架套餐 → 92101
	if err := db.Exec("UPDATE vip_plan SET status = 0 WHERE code = 'vip_year'").Error; err != nil {
		t.Fatalf("下架失败: %v", err)
	}
	defer db.Exec("UPDATE vip_plan SET status = 1 WHERE code = 'vip_year'")
	_, err = svc.CreateVipOrder(context.Background(), uid, "vip_year", pay.ChannelMock)
	if !isCode(err, 92101) {
		t.Fatalf("下架套餐下单应 92101, got %v", err)
	}
}

// TestFirstMonthDiscount 首月优惠：仅对从未开通过的用户生效。
func TestFirstMonthDiscount(t *testing.T) {
	db := testDB(t)
	cleanupVip(t, db)
	t.Cleanup(func() { cleanupVip(t, db) })
	svc, paySvc := newVipSvc(t, db)
	uid := testUIDBase + 8
	seedUser(t, db, uid)

	// 首次：按首月价原价 1500 → 首月 800
	o1, err := svc.CreateVipOrder(context.Background(), uid, "vip_month", pay.ChannelMock)
	if err != nil {
		t.Fatalf("首次下单失败: %v", err)
	}
	if o1.Amount != 800 {
		t.Fatalf("首次连续包月应为首月优惠价 800 分, got %d", o1.Amount)
	}
	if _, err := paySvc.DevCallback(o1.OrderNo, o1.Amount, "success"); err != nil {
		t.Fatalf("回调失败: %v", err)
	}
	// 第二次：已是会员 → 不再享首月优惠
	o2, err := svc.CreateVipOrder(context.Background(), uid, "vip_month", pay.ChannelMock)
	if err != nil {
		t.Fatalf("二次下单失败: %v", err)
	}
	if o2.Amount != 1500 {
		t.Fatalf("已开通过的用户不应再享首月优惠, got %d", o2.Amount)
	}
	// vip_order 台账必须按**实付时长**开通（快照），而非回头查套餐
	row, _ := svc.repo.GetOrderRow(o1.OrderNo)
	if row == nil || row.Months != 1 {
		t.Fatalf("台账应记录下单时长快照 1 月, got %+v", row)
	}
}

// ---- 辅助 ----

func coinOf(t *testing.T, db *gorm.DB, uid int64) int {
	t.Helper()
	var c int
	if err := db.Table("`user`").Where("id = ?", uid).Select("coin").Scan(&c).Error; err != nil {
		t.Fatalf("读硬币失败: %v", err)
	}
	return c
}

func countGrants(t *testing.T, db *gorm.DB, uid int64) int64 {
	t.Helper()
	var n int64
	if err := db.Table("vip_coin_grant").Where("user_id = ?", uid).Count(&n).Error; err != nil {
		t.Fatalf("统计赠币台账失败: %v", err)
	}
	return n
}

// isCode 判定错误码（沿用 pay 集成测试的 errors.Is 口径，而非自造 As 包装）。
func isCode(err error, code int) bool {
	if err == nil {
		return false
	}
	var e *errcode.Error
	if !errors.As(err, &e) {
		return false
	}
	return e.Code == code
}
