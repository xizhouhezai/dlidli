package live

import (
	"sync"
	"testing"

	"github.com/dlidli/server/internal/pkg/errcode"
	"github.com/dlidli/server/internal/pkg/snowflake"
	"gorm.io/gorm"
)

// TestGiftConcurrentSpend 强一致并发（spec LIV-13 / 验收核心）：
// 余额 600，20 并发各扣 100（单价 100 的礼物 ×1）→ 恰好 6 笔成功、
// 最终余额 0、无负数、无丢单（成功笔数 × 100 + 失败笔数 = 20，余额 + Σdelta 守恒）。
func TestGiftConcurrentSpend(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { cleanupGift(t, db) })
	cleanupGift(t, db)

	repo := NewGiftRepo(db)
	if err := snowflake.Init(1); err != nil {
		t.Fatalf("snowflake 初始化失败: %v", err)
	}
	const uid = int64(91000090)
	if err := db.Exec("INSERT INTO `user` (id, nickname, status, created_at, updated_at) VALUES (?, 'gifttest', 0, NOW(), NOW())", uid).Error; err != nil {
		t.Fatalf("造用户失败: %v", err)
	}
	// dev 直充通道造初始余额 600（同时覆盖 RechargeTx 正常路径）
	biz := "dev-test-concurrent"
	if _, _, err := repo.RechargeTx(uid, 600, biz, "测试"); err != nil {
		t.Fatalf("充值失败: %v", err)
	}

	room := &LiveRoom{ID: 990001, AnchorID: 91000091}
	gift := &Gift{ID: 1, Name: "辣条", Price: 100, Effect: GiftEffectDanmaku, Level: GiftLevelNormal}

	const workers = 20
	var wg sync.WaitGroup
	var mu sync.Mutex
	success, insufficient := 0, 0
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := repo.SendGiftTx(uid, room, gift, 1)
			mu.Lock()
			defer mu.Unlock()
			switch err {
			case nil:
				success++
			case errcode.ErrLiveMelonNotEnough:
				insufficient++
			default:
				t.Errorf("意外错误: %v", err)
			}
		}()
	}
	wg.Wait()

	if success != 6 || insufficient != workers-6 {
		t.Fatalf("成功笔数=%d 余额不足=%d，期望 6/%d", success, insufficient, workers-6)
	}
	balance, err := repo.MelonBalance(uid)
	if err != nil {
		t.Fatalf("查余额失败: %v", err)
	}
	if balance != 0 {
		t.Fatalf("最终余额=%d，期望 0（无负数、无丢单）", balance)
	}
	logs, err := repo.GiftLogsByRoom(room.ID)
	if err != nil {
		t.Fatalf("查流水失败: %v", err)
	}
	if len(logs) != success {
		t.Fatalf("礼物流水 %d 条 ≠ 成功笔数 %d（丢单）", len(logs), success)
	}
	var cnt int64
	db.Model(&MelonSeedLog{}).Where("user_id = ?", uid).Count(&cnt)
	if cnt != int64(1+success) { // 1 笔充值 + success 笔扣减
		t.Fatalf("金瓜子流水 %d 条，期望 %d", cnt, 1+success)
	}
	var shareSum int64
	db.Model(&GiftShareLog{}).Where("anchor_id = ?", room.AnchorID).
		Select("COALESCE(SUM(amount),0)").Take(&shareSum)
	if shareSum != int64(success)*100*giftShareDefault/1000 {
		t.Fatalf("分成累计 %d ≠ 期望 %d（70‰）", shareSum, int64(success)*100*giftShareDefault/1000)
	}
}

// TestGiftInsufficientNoLog 余额不足被拒且不产生任何流水（验收项）。
func TestGiftInsufficientNoLog(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { cleanupGift(t, db) })
	cleanupGift(t, db)

	repo := NewGiftRepo(db)
	const uid = int64(91000092)
	if err := db.Exec("INSERT INTO `user` (id, nickname, status, created_at, updated_at) VALUES (?, 'gifttest2', 0, NOW(), NOW())", uid).Error; err != nil {
		t.Fatalf("造用户失败: %v", err)
	}
	if _, _, err := repo.RechargeTx(uid, 50, "dev-test-poor", "测试"); err != nil {
		t.Fatalf("充值失败: %v", err)
	}
	room := &LiveRoom{ID: 990002, AnchorID: 91000093}
	gift := &Gift{ID: 1, Name: "辣条", Price: 100}
	if _, _, err := repo.SendGiftTx(uid, room, gift, 1); err != errcode.ErrLiveMelonNotEnough {
		t.Fatalf("余额不足应 91021, got %v", err)
	}
	var cnt int64
	db.Model(&MelonSeedLog{}).Where("user_id = ? AND biz_type = ?", uid, MelonBizGiftSpend).Count(&cnt)
	if cnt != 0 {
		t.Fatalf("余额不足不应产生送礼流水，实际 %d 条", cnt)
	}
	db.Model(&GiftLog{}).Where("sender_id = ?", uid).Count(&cnt)
	if cnt != 0 {
		t.Fatalf("余额不足不应产生礼物流水，实际 %d 条", cnt)
	}
	db.Model(&GiftShareLog{}).Where("anchor_id = ?", room.AnchorID).Count(&cnt)
	if cnt != 0 {
		t.Fatalf("余额不足不应产生分成流水，实际 %d 条", cnt)
	}
	balance, _ := repo.MelonBalance(uid)
	if balance != 50 {
		t.Fatalf("余额不应变动，实际 %d", balance)
	}
}

// TestRechargeIdempotent 充值幂等：同 biz_no 重复入账余额不变（PAY-01 回调防重放）。
func TestRechargeIdempotent(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { cleanupGift(t, db) })
	cleanupGift(t, db)

	repo := NewGiftRepo(db)
	const uid = int64(91000094)
	if err := db.Exec("INSERT INTO `user` (id, nickname, status, created_at, updated_at) VALUES (?, 'gifttest3', 0, NOW(), NOW())", uid).Error; err != nil {
		t.Fatalf("造用户失败: %v", err)
	}
	b1, _, err := repo.RechargeTx(uid, 1000, "pay-callback-001", "支付回调")
	if err != nil {
		t.Fatalf("首次充值失败: %v", err)
	}
	b2, applied, err := repo.RechargeTx(uid, 1000, "pay-callback-001", "支付回调重放")
	if err != nil {
		t.Fatalf("重放充值失败: %v", err)
	}
	if b1 != b2 || applied {
		t.Fatalf("幂等失败: b1=%d b2=%d applied=%v（重放不应再次入账）", b1, b2, applied)
	}
	var cnt int64
	db.Model(&MelonSeedLog{}).Where("user_id = ? AND biz_no = ?", uid, "pay-callback-001").Count(&cnt)
	if cnt != 1 {
		t.Fatalf("流水应恰好 1 条，实际 %d", cnt)
	}
}

// cleanupGift 清理礼物域测试数据（测试用户/房间段 91000xxx，礼物测试房间 990xxx）。
func cleanupGift(t *testing.T, db *gorm.DB) {
	t.Helper()
	_ = db.Exec("DELETE FROM gift_log WHERE sender_id BETWEEN 91000000 AND 91099999 OR anchor_id BETWEEN 91000000 AND 91099999 OR room_id BETWEEN 990000 AND 991999").Error
	_ = db.Exec("DELETE FROM gift_share_log WHERE anchor_id BETWEEN 91000000 AND 91099999").Error
	_ = db.Exec("DELETE FROM melon_seed_log WHERE user_id BETWEEN 91000000 AND 91099999").Error
	_ = db.Exec("DELETE FROM melon_seed_account WHERE user_id BETWEEN 91000000 AND 91099999").Error
	_ = db.Exec("DELETE FROM `user` WHERE id BETWEEN 91000000 AND 91099999").Error
}
