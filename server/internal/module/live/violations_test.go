package live

import (
	"strconv"
	"testing"
)

// 违规记录查询（管理端巡查台的数据源）集成测试。
//
// 覆盖 repo.ListViolations 的两处易错点：
//  1. **LEFT JOIN 而非 INNER JOIN**：机审流水 cut_by=0，没有对应的 admin_user 行。
//     写成 INNER JOIN 会让所有机审记录从列表里消失 —— 而机审命中的那几条恰恰是
//     巡台最该看到的（人工切了自己知道，机审切了才需要查）。
//  2. 操作人用户名必须解析出来：表里只有 cut_by（雪花 ID），对人不可读，
//     而「切断是谁下的」正是追责时要回答的问题。
//
// 另覆盖 room_id 过滤与分页（列表页筛选/翻页的数据源）。
//
// **断言一律按本用例自己的 roomID 收敛**：live_violation_log 是全库表，
// 同包其它用例（audit_integration_test）也会写它，按全库计数会被残留数据污染。
// 用户/房间 ID 取 91000000+ 段，与 live_test.go 的 cleanup 口径一致。
func TestListViolations(t *testing.T) {
	db := testDB(t)
	cleanup(t, db)
	defer cleanup(t, db)

	// 管理员 ID 也取 91000001：live_test.go 的 cleanup 只清用户段，不管 admin_user，
	// 故这里显式回收；插入前先删，保证用例**可重复运行**（残留账号会让本次直接 dirty fail）。
	adminID := int64(91000001)
	if err := db.Exec("DELETE FROM admin_user WHERE id = ?", adminID).Error; err != nil {
		t.Fatalf("清理旧管理员失败: %v", err)
	}
	if err := db.Exec("INSERT INTO admin_user (id, username, password, nickname, role, status, created_at) VALUES (?,?,?,?,?,0,NOW())",
		adminID, "patrol-op", "x", "巡查员", "super").Error; err != nil {
		t.Fatalf("造管理员失败: %v", err)
	}
	t.Cleanup(func() { _ = db.Exec("DELETE FROM admin_user WHERE id = ?", adminID).Error })

	roomA := seedAuditRoom(t, db, 91000201, RoomStatusLive)
	roomB := seedAuditRoom(t, db, 91000202, RoomStatusLive)
	// 违规流水不随房间级联删除，需显式回收（否则污染后续全库计数类断言）。
	t.Cleanup(func() {
		_ = db.Exec("DELETE FROM live_violation_log WHERE room_id IN (?,?)", roomA.ID, roomB.ID).Error
	})

	// 人工切断（带操作人）两条 + 机审（cut_by=0）一条。
	for i := 0; i < 2; i++ {
		if err := db.Create(&LiveViolationLog{
			RoomID: roomA.ID, AnchorID: roomA.AnchorID, Source: ViolationSourceManual,
			Reason: "巡查切断", CutMode: 1, CutBy: adminID,
		}).Error; err != nil {
			t.Fatalf("造人工违规失败: %v", err)
		}
	}
	if err := db.Create(&LiveViolationLog{
		RoomID: roomB.ID, AnchorID: roomB.AnchorID, Source: ViolationSourceAuto,
		Reason: "黑屏", CutMode: CutModeDegraded, CutBy: 0,
	}).Error; err != nil {
		t.Fatalf("造机审违规失败: %v", err)
	}

	// (1) 机审记录必须可见（INNER JOIN 写法会让它整行消失）。
	auto, n, err := NewRepo(db).ListViolations(roomB.ID, 1, 20)
	if err != nil {
		t.Fatalf("查询机审违规失败: %v", err)
	}
	if n != 1 || len(auto) != 1 {
		t.Fatalf("roomB 应见 1 条机审记录，实际 total=%d len=%d", n, len(auto))
	}
	if auto[0].Source != ViolationSourceAuto {
		t.Fatalf("记录来源应为机审(%d)，实际 %d", ViolationSourceAuto, auto[0].Source)
	}
	if auto[0].Operator != "" {
		t.Fatalf("机审记录不应有操作人，实际 %q", auto[0].Operator)
	}

	// (2) 人工切断必须带出操作人用户名（而非 ID）。
	manual, n, err := NewRepo(db).ListViolations(roomA.ID, 1, 20)
	if err != nil {
		t.Fatalf("查询人工违规失败: %v", err)
	}
	if n != 2 || len(manual) != 2 {
		t.Fatalf("roomA 应见 2 条人工切断，实际 total=%d len=%d", n, len(manual))
	}
	if manual[0].Operator != "patrol-op" {
		t.Fatalf("人工切断的操作人应为 patrol-op，实际 %q", manual[0].Operator)
	}
	if manual[0].CutBy != strconv.FormatInt(adminID, 10) {
		t.Fatalf("cut_by 应为字符串 ID，实际 %q", manual[0].CutBy)
	}

	// (3) 分页：roomA 两条，每页 1 条 → 当页 1 条、总数 2。
	page1, total, err := NewRepo(db).ListViolations(roomA.ID, 1, 1)
	if err != nil {
		t.Fatalf("分页查询失败: %v", err)
	}
	if total != 2 || len(page1) != 1 {
		t.Fatalf("roomA 应 total=2 / 当页 1 条，实际 total=%d len=%d", total, len(page1))
	}
	// 倒序：第 1 页应是较新的那条（id 更大）。
	if page1[0].ID <= manual[1].ID {
		t.Fatalf("应按 id 倒序返回，第 1 页 id=%d 不应小于第 2 条 id=%d", page1[0].ID, manual[1].ID)
	}
}
