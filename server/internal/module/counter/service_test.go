package counter

import (
	"context"
	"fmt"
	"os"
	"testing"

	counterv1 "github.com/dlidli/server/internal/gen/counter/v1"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestColumnFromProtoAllowlist 校验计数列白名单：
// 已登记枚举必须映射到预期列名，未登记值（含 UNSPECIFIED）必须报错。
// 这是"禁止把调用方输入带入 SQL 列名"的回归保护。
func TestColumnFromProtoAllowlist(t *testing.T) {
	cases := map[counterv1.CounterColumn]string{
		// 播放列在既有 schema 中名为 view_cnt（0001_init.up.sql），
		// proto 枚举是 COUNTER_COLUMN_PLAY，此处锁定这层映射不被改错。
		counterv1.CounterColumn_COUNTER_COLUMN_PLAY:    "view_cnt",
		counterv1.CounterColumn_COUNTER_COLUMN_LIKE:    "like_cnt",
		counterv1.CounterColumn_COUNTER_COLUMN_COIN:    "coin_cnt",
		counterv1.CounterColumn_COUNTER_COLUMN_FAV:     "fav_cnt",
		counterv1.CounterColumn_COUNTER_COLUMN_COMMENT: "comment_cnt",
		counterv1.CounterColumn_COUNTER_COLUMN_DANMAKU: "danmaku_cnt",
		counterv1.CounterColumn_COUNTER_COLUMN_SHARE:   "share_cnt",
	}
	for col, want := range cases {
		got, err := ColumnFromProto(col)
		if err != nil {
			t.Fatalf("column %v should be allowed: %v", col, err)
		}
		if got != want {
			t.Fatalf("column %v: got=%q want=%q", col, got, want)
		}
		if _, ok := allowColumns[got]; !ok {
			t.Fatalf("mapped column %q missing from allowColumns", got)
		}
	}

	for _, bad := range []counterv1.CounterColumn{
		counterv1.CounterColumn_COUNTER_COLUMN_UNSPECIFIED,
		counterv1.CounterColumn(99),
		counterv1.CounterColumn(-1),
	} {
		if _, err := ColumnFromProto(bad); err == nil {
			t.Fatalf("column %v should be rejected", bad)
		}
	}
}

// testDB 连接测试库；未设置 DLIDLI_TEST_DSN 时跳过。
// 用真实 MySQL 验证幂等语义，避免用 mock 掩盖事务/唯一键行为。
//
// 注意：显式设置了 DSN 却连不上时用 Fatalf 而非 Skipf——
// 否则 CI 上"库挂了"会伪装成"测试通过"，是更危险的静默失败。
func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("DLIDLI_TEST_DSN")
	if dsn == "" {
		t.Skip("DLIDLI_TEST_DSN 未设置，跳过计数服务集成测试")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
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
	if err := db.Exec("CREATE TABLE IF NOT EXISTS counter_event (\n" +
		"event_id VARCHAR(128) NOT NULL PRIMARY KEY,\n" +
		"video_id BIGINT UNSIGNED NOT NULL,\n" +
		"`column` VARCHAR(32) NOT NULL,\n" + // column 为 MySQL 保留字，必须反引号包裹
		"delta BIGINT NOT NULL DEFAULT 0,\n" +
		"created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,\n" +
		"KEY idx_video (video_id)\n" +
		") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4").Error; err != nil {
		t.Fatalf("建 counter_event 表失败: %v", err)
	}
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS video_stat (
		video_id BIGINT UNSIGNED NOT NULL PRIMARY KEY,
		view_cnt BIGINT NOT NULL DEFAULT 0,
		like_cnt BIGINT NOT NULL DEFAULT 0,
		coin_cnt BIGINT NOT NULL DEFAULT 0,
		fav_cnt BIGINT NOT NULL DEFAULT 0,
		danmaku_cnt BIGINT NOT NULL DEFAULT 0,
		comment_cnt BIGINT NOT NULL DEFAULT 0,
		share_cnt BIGINT NOT NULL DEFAULT 0
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`).Error; err != nil {
		t.Fatalf("建 video_stat 表失败: %v", err)
	}
	return db
}

// TestApplyDeltaIdempotent 验收标准：相同 event_id 重复调用只累加一次。
func TestApplyDeltaIdempotent(t *testing.T) {
	db := testDB(t)
	svc := NewService(db)
	ctx := context.Background()

	const videoID = int64(900000001)
	db.Exec("DELETE FROM counter_event WHERE video_id = ?", videoID)
	db.Exec("DELETE FROM video_stat WHERE video_id = ?", videoID)

	evt := "test-evt-idem-1"
	applied, cur, err := svc.ApplyDelta(ctx, evt, videoID, counterv1.CounterColumn_COUNTER_COLUMN_LIKE, 1)
	if err != nil {
		t.Fatalf("首次 ApplyDelta 失败: %v", err)
	}
	if !applied || cur != 1 {
		t.Fatalf("首次应生效且计数为 1: applied=%v current=%d", applied, cur)
	}

	// 重复投递同一事件
	for i := 0; i < 3; i++ {
		applied, cur, err = svc.ApplyDelta(ctx, evt, videoID, counterv1.CounterColumn_COUNTER_COLUMN_LIKE, 1)
		if err != nil {
			t.Fatalf("重复 ApplyDelta 失败: %v", err)
		}
		if applied {
			t.Fatalf("第 %d 次重复投递不应再次生效", i+1)
		}
		if cur != 1 {
			t.Fatalf("重复投递后计数应保持 1: got=%d", cur)
		}
	}

	snap, err := svc.GetStat(ctx, videoID)
	if err != nil {
		t.Fatalf("GetStat 失败: %v", err)
	}
	if snap.GetLikeCnt() != 1 {
		t.Fatalf("最终 like_cnt 应为 1: got=%d", snap.GetLikeCnt())
	}
}

// TestApplyDeltaFloorAtZero 计数下限保护：负向扣减不得为负。
func TestApplyDeltaFloorAtZero(t *testing.T) {
	db := testDB(t)
	svc := NewService(db)
	ctx := context.Background()

	const videoID = int64(900000002)
	db.Exec("DELETE FROM counter_event WHERE video_id = ?", videoID)
	db.Exec("DELETE FROM video_stat WHERE video_id = ?", videoID)

	// 先建行（+1），再扣 5，期望下限 0
	if _, _, err := svc.ApplyDelta(ctx, "test-floor-a", videoID, counterv1.CounterColumn_COUNTER_COLUMN_COIN, 1); err != nil {
		t.Fatalf("ApplyDelta(+1) 失败: %v", err)
	}
	applied, cur, err := svc.ApplyDelta(ctx, "test-floor-b", videoID, counterv1.CounterColumn_COUNTER_COLUMN_COIN, -5)
	if err != nil {
		t.Fatalf("ApplyDelta(-5) 失败: %v", err)
	}
	if !applied || cur != 0 {
		t.Fatalf("扣减后应下限为 0: applied=%v current=%d", applied, cur)
	}
}

// TestApplyDeltaRejectsBadInput 非法入参必须被拒绝，且不写入任何数据。
func TestApplyDeltaRejectsBadInput(t *testing.T) {
	db := testDB(t)
	svc := NewService(db)
	ctx := context.Background()
	const videoID = int64(900000003)

	cases := []struct {
		name  string
		event string
		vid   int64
		col   counterv1.CounterColumn
	}{
		{"空 event_id", "", videoID, counterv1.CounterColumn_COUNTER_COLUMN_PLAY},
		{"非法 video_id", "bad-vid", 0, counterv1.CounterColumn_COUNTER_COLUMN_PLAY},
		{"未登记列", "bad-col", videoID, counterv1.CounterColumn_COUNTER_COLUMN_UNSPECIFIED},
		{"越界列", "bad-col-2", videoID, counterv1.CounterColumn(404)},
	}
	for _, tc := range cases {
		if _, _, err := svc.ApplyDelta(ctx, tc.event, tc.vid, tc.col, 1); err == nil {
			t.Fatalf("用例 %q 应返回错误", tc.name)
		}
	}
	var cnt int64
	db.Model(&counterEvent{}).Where("event_id LIKE ?", "bad-%").Count(&cnt)
	if cnt != 0 {
		t.Fatalf("非法入参不应落库: count=%d", cnt)
	}
}

// TestBatchGetStatsOrderAndZeroFill 批量读取需按入参顺序补齐零值快照。
func TestBatchGetStatsOrderAndZeroFill(t *testing.T) {
	db := testDB(t)
	svc := NewService(db)
	ctx := context.Background()

	const videoA = int64(900000010)
	const videoB = int64(900000011) // 故意不写入，验证零值补齐
	db.Exec("DELETE FROM counter_event WHERE video_id IN ?", []int64{videoA, videoB})
	db.Exec("DELETE FROM video_stat WHERE video_id IN ?", []int64{videoA, videoB})

	if _, _, err := svc.ApplyDelta(ctx, "test-batch-a", videoA, counterv1.CounterColumn_COUNTER_COLUMN_DANMAKU, 7); err != nil {
		t.Fatalf("ApplyDelta 失败: %v", err)
	}

	stats, err := svc.BatchGetStats(ctx, []int64{videoB, videoA})
	if err != nil {
		t.Fatalf("BatchGetStats 失败: %v", err)
	}
	if len(stats) != 2 {
		t.Fatalf("期望 2 条，得到 %d", len(stats))
	}
	if stats[0].GetVideoId() != videoB || stats[0].GetDanmakuCnt() != 0 {
		t.Fatalf("第 1 条应为 %d 的零值快照: %+v", videoB, stats[0])
	}
	if stats[1].GetVideoId() != videoA || stats[1].GetDanmakuCnt() != 7 {
		t.Fatalf("第 2 条应为 %d 且 danmaku_cnt=7: %+v", videoA, stats[1])
	}

	// 空入参应返回空切片而非 nil
	empty, err := svc.BatchGetStats(ctx, nil)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("空入参应返回空切片: len=%d err=%v", len(empty), err)
	}
}

// TestConcurrentApplyDeltaSameEvent 并发投递同一事件只允许一次生效。
func TestConcurrentApplyDeltaSameEvent(t *testing.T) {
	db := testDB(t)
	svc := NewService(db)
	ctx := context.Background()

	const videoID = int64(900000020)
	const event = "test-evt-concurrent"
	db.Exec("DELETE FROM counter_event WHERE video_id = ?", videoID)
	db.Exec("DELETE FROM video_stat WHERE video_id = ?", videoID)

	const workers = 8
	results := make(chan bool, workers)
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		go func(n int) {
			applied, _, err := svc.ApplyDelta(ctx, event, videoID, counterv1.CounterColumn_COUNTER_COLUMN_FAV, 1)
			if err != nil {
				errs <- fmt.Errorf("worker %d: %w", n, err)
				return
			}
			results <- applied
		}(i)
	}

	appliedCount := 0
	for i := 0; i < workers; i++ {
		select {
		case err := <-errs:
			t.Fatalf("并发 ApplyDelta 出错: %v", err)
		case ok := <-results:
			if ok {
				appliedCount++
			}
		}
	}
	if appliedCount != 1 {
		t.Fatalf("并发下应恰好 1 次生效: got=%d", appliedCount)
	}

	snap, err := svc.GetStat(ctx, videoID)
	if err != nil {
		t.Fatalf("GetStat 失败: %v", err)
	}
	if snap.GetFavCnt() != 1 {
		t.Fatalf("并发后 fav_cnt 应为 1: got=%d", snap.GetFavCnt())
	}
}
