package video

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dlidli/server/internal/pkg/config"
	"github.com/dlidli/server/internal/pkg/storage"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// 转码积压治理（水平扩容）验收测试：真 MySQL。
// 口径与 pay/charge/live 的集成测试一致：未设置 DLIDLI_TEST_DSN 时跳过。
func transcaleDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("DLIDLI_TEST_DSN")
	if dsn == "" {
		t.Skip("DLIDLI_TEST_DSN 未设置，跳过转码扩容集成测试")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), TranslateError: true,
	})
	if err != nil {
		t.Fatalf("无法连接测试库: %v", err)
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

// 测试数据段 92800xxx：与 pay(92600xxx)/charge(92700xxx)/live(91000xxx) 隔离，
// 避免跨包清理互相踩踏。
const (
	tsUIDBase = int64(92800000)
	tsUIDEnd  = int64(92800999)
	tsVideoID = int64(928000000001)
)

func cleanupTranscodeScale(t *testing.T, db *gorm.DB) {
	t.Helper()
	// transcode_job 只被 video 域使用，且本文件的断言直接对比**全表**口径
	// （QueueSnapshot 是全表统计），故整表清理，避免历史运行残留把计数带偏。
	_ = db.Exec("DELETE FROM transcode_job").Error
	_ = db.Exec("DELETE FROM video_stream WHERE video_id BETWEEN ? AND ?", tsVideoID, tsVideoID+9999).Error
	_ = db.Exec("DELETE FROM video_stat WHERE video_id BETWEEN ? AND ?", tsVideoID, tsVideoID+9999).Error
	_ = db.Exec("DELETE FROM video WHERE user_id BETWEEN ? AND ?", tsUIDBase, tsUIDEnd).Error
}

// seedVideo 造一个处于"转码中"的稿件 + 原画流（转码任务的前置数据）。
func seedVideo(t *testing.T, db *gorm.DB, id int64) {
	t.Helper()
	v := &Video{
		ID: id, Bvid: fmt.Sprintf("TS%d", id%100000), UserID: tsUIDBase,
		Title: "transcode-scale", CategoryID: 1, Tags: "[]", Copyright: 1,
		Status: StatusTranscoding,
	}
	if err := db.Create(v).Error; err != nil {
		t.Fatalf("造稿件失败: %v", err)
	}
	st := &Stream{VideoID: id, PartIndex: 0, Quality: 0, Format: "mp4", PlayPath: "videos/src.mp4"}
	if err := db.Create(st).Error; err != nil {
		t.Fatalf("造原画流失败: %v", err)
	}
}

// seedJobs 造 n 个待处理转码任务，返回任务 ID 列表。
func seedJobs(t *testing.T, db *gorm.DB, videoID int64, n int) []int64 {
	t.Helper()
	ids := make([]int64, 0, n)
	for i := 0; i < n; i++ {
		j := &TranscodeJob{VideoID: videoID, PartIndex: 0, Quality: int16(360 + i), Status: JobPending}
		if err := db.Create(j).Error; err != nil {
			t.Fatalf("造任务失败: %v", err)
		}
		ids = append(ids, j.ID)
	}
	return ids
}

// TestClaimJob_NoDuplicateUnderConcurrency 核心验收：多消费者并发认领同一队列，
// **每个任务恰好被认领一次**。
//
// 造 40 个任务，起 8 个互相独立的 Repo（模拟 8 个 worker 进程各自持有连接池），
// 同时开工认领，直到队列空。断言：
//   - 认领到的任务总数 == 40（没有漏认，也没有重复）；
//   - 每个任务 ID 只出现一次（无重复处理）；
//   - DB 里 40 行全部为 running 且 worker_id 非空。
func TestClaimJob_NoDuplicateUnderConcurrency(t *testing.T) {
	db := transcaleDB(t)
	cleanupTranscodeScale(t, db)
	defer cleanupTranscodeScale(t, db)

	seedVideo(t, db, tsVideoID)
	seedJobs(t, db, tsVideoID, 40)

	const consumers = 8
	repos := make([]*Repo, consumers)
	for i := range repos {
		repos[i] = NewRepo(db)
	}

	var (
		mu     sync.Mutex
		claims []int64
		wg     sync.WaitGroup
		done   atomic.Int32
	)
	start := make(chan struct{})
	for i := 0; i < consumers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start // 同时起跑，最大化竞态窗口
			workerID := fmt.Sprintf("test-host/%d/w%d", os.Getpid(), idx)
			for {
				job, err := repos[idx].ClaimJob(workerID, time.Minute)
				if err != nil {
					t.Errorf("认领出错: %v", err)
					return
				}
				if job == nil {
					done.Add(1)
					return // 队列空
				}
				mu.Lock()
				claims = append(claims, job.ID)
				mu.Unlock()
			}
		}(i)
	}
	close(start)
	wg.Wait()

	if len(claims) != 40 {
		t.Fatalf("认领总数应为 40，实际 %d（漏认或多认）", len(claims))
	}
	seen := make(map[int64]int, len(claims))
	for _, id := range claims {
		seen[id]++
	}
	for id, n := range seen {
		if n != 1 {
			t.Fatalf("任务 %d 被认领 %d 次（重复处理，违反水平扩容安全性）", id, n)
		}
	}

	var running int64
	if err := db.Model(&TranscodeJob{}).
		Where("video_id = ? AND status = ?", tsVideoID, JobRunning).Count(&running).Error; err != nil {
		t.Fatalf("统计 running 失败: %v", err)
	}
	if running != 40 {
		t.Fatalf("DB 中 running 应为 40，实际 %d", running)
	}
	var noOwner int64
	if err := db.Model(&TranscodeJob{}).
		Where("video_id = ? AND worker_id = ''", tsVideoID).Count(&noOwner).Error; err != nil {
		t.Fatalf("统计 worker_id 失败: %v", err)
	}
	if noOwner != 0 {
		t.Fatalf("有 %d 个任务没有持有者标识（排障不可用）", noOwner)
	}
}

// TestClaimJob_LeaseRecovery 租约回收：进程崩溃留下的 status=1 任务，
// 租约过期后可被其他 Worker 重新认领；未过期时不可抢。
func TestClaimJob_LeaseRecovery(t *testing.T) {
	db := transcaleDB(t)
	cleanupTranscodeScale(t, db)
	defer cleanupTranscodeScale(t, db)

	seedVideo(t, db, tsVideoID)
	ids := seedJobs(t, db, tsVideoID, 1)

	repo := NewRepo(db)
	first, err := repo.ClaimJob("dead-worker/1", time.Minute)
	if err != nil || first == nil {
		t.Fatalf("首次认领失败: job=%v err=%v", first, err)
	}
	if first.ID != ids[0] {
		t.Fatalf("认领到错误任务: %d", first.ID)
	}

	// 租约仍有效 → 别人抢不到
	if j, err := NewRepo(db).ClaimJob("other-worker/1", time.Minute); err != nil || j != nil {
		t.Fatalf("租约未过期时不应被抢走: job=%v err=%v", j, err)
	}

	// 模拟崩溃：把租约改到过去
	if err := db.Model(&TranscodeJob{}).Where("id = ?", ids[0]).
		Update("lease_until", time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatalf("改写租约失败: %v", err)
	}

	second, err := NewRepo(db).ClaimJob("alive-worker/1", time.Minute)
	if err != nil || second == nil {
		t.Fatalf("租约过期后应可被回收认领: job=%v err=%v", second, err)
	}
	if second.ID != ids[0] {
		t.Fatalf("回收认领到错误任务: %d", second.ID)
	}
	if second.WorkerID != "alive-worker/1" {
		t.Fatalf("持有者未更新: %q", second.WorkerID)
	}

	// 旧持有者的续租必须失败（防止两个 Worker 同时写同一任务）
	ok, err := repo.RenewLease(ids[0], "dead-worker/1", time.Minute)
	if err != nil {
		t.Fatalf("续租出错: %v", err)
	}
	if ok {
		t.Fatal("已被回收的任务不应续租成功（会与接管方重复处理）")
	}
}

// mustClaimEventually 轮询直到认领成功或超时（退避是时间相关的，不能用固定 sleep 断言）。
func mustClaimEventually(t *testing.T, db *gorm.DB, workerID string, timeout time.Duration) *TranscodeJob {
	t.Helper()
	repo := NewRepo(db)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		job, err := repo.ClaimJob(workerID, time.Minute)
		if err != nil {
			t.Fatalf("认领出错: %v", err)
		}
		if job != nil {
			return job
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("等待 %s 仍无法认领（退避未按时释放）", timeout)
	return nil
}

// TestFailJob_RetryBackoffThenDeadLetter 重试语义：
// 未达上限 → 回 pending 且带未来 next_retry_at（退避期内不可认领）；
// 达上限 → 终态 failed 并记 finished_at（死信）。
func TestFailJob_RetryBackoffThenDeadLetter(t *testing.T) {
	db := transcaleDB(t)
	cleanupTranscodeScale(t, db)
	defer cleanupTranscodeScale(t, db)

	seedVideo(t, db, tsVideoID)
	seedJobs(t, db, tsVideoID, 1)

	repo := NewRepo(db)
	job, err := repo.ClaimJob("w/1", time.Minute)
	if err != nil || job == nil {
		t.Fatalf("认领失败: %v", err)
	}

	const maxRetries = 2
	// 退避基数取 2s：next_retry_at 落 DATETIME(0)（秒级），过小的基数会让"退避中"窗口
	// 与秒边界重叠而让断言变得不可靠（生产默认是 30s，见 config.Load）。
	base := 2 * time.Second

	// 第 1 次失败：应回到 pending（未达上限），但退避期内不可认领
	terminal, err := repo.FailJob(job, "boom-1", maxRetries, base)
	if err != nil {
		t.Fatalf("回写失败出错: %v", err)
	}
	if terminal {
		t.Fatalf("第 1 次失败不应进终态（上限 %d）", maxRetries)
	}
	if j, err := NewRepo(db).ClaimJob("w/2", time.Minute); err != nil || j != nil {
		t.Fatalf("退避期内不应被认领: job=%v err=%v", j, err)
	}

	// 等退避过去 → 可再次认领。
	// 用轮询而不是固定 sleep：next_retry_at 落库是 DATETIME(0)（秒级精度，MySQL 会四舍五入），
	// 固定 sleep 在 200ms 量级上会因秒边界抖动而偶发失败。
	job2 := mustClaimEventually(t, db, "w/2", 10*time.Second)

	// 第 2 次失败（retry=1 → n=2 <= 上限）：仍应重排
	terminal, err = repo.FailJob(job2, "boom-2", maxRetries, base)
	if err != nil {
		t.Fatalf("回写失败出错: %v", err)
	}
	if terminal {
		t.Fatalf("第 2 次失败不应进终态（上限 %d）", maxRetries)
	}

	job3 := mustClaimEventually(t, db, "w/3", 10*time.Second)
	// 第 3 次失败（n=3 > 上限 2）→ 终态死信
	terminal, err = repo.FailJob(job3, "boom-3", maxRetries, base)
	if err != nil {
		t.Fatalf("回写失败出错: %v", err)
	}
	if !terminal {
		t.Fatalf("超过重试上限应进终态（上限 %d）", maxRetries)
	}

	var row TranscodeJob
	if err := db.First(&row, job3.ID).Error; err != nil {
		t.Fatalf("读取任务失败: %v", err)
	}
	if row.Status != JobFailed {
		t.Fatalf("终态应为 failed(3)，实际 %d", row.Status)
	}
	if row.FinishedAt == nil {
		t.Fatal("终态必须记录 finished_at（积压平均耗时口径依赖它）")
	}
	if row.LeaseUntil != nil {
		t.Fatalf("终态不应残留租约: %v", row.LeaseUntil)
	}
	if row.RetryCnt != 3 {
		t.Fatalf("retry_cnt 应为 3，实际 %d", row.RetryCnt)
	}
}

// TestQueueSnapshot 积压指标：排队数/正在转/最老等待/平均耗时四个口径。
func TestQueueSnapshot(t *testing.T) {
	db := transcaleDB(t)
	cleanupTranscodeScale(t, db)
	defer cleanupTranscodeScale(t, db)

	seedVideo(t, db, tsVideoID)
	seedJobs(t, db, tsVideoID, 10)

	repo := NewRepo(db)
	// 把最老的一条的创建时间推到 100 秒前，验证"最老的等了多久"
	if err := db.Exec("UPDATE transcode_job SET created_at = ? WHERE id = (SELECT id FROM (SELECT MIN(id) AS id FROM transcode_job WHERE video_id = ?) x)",
		time.Now().Add(-100*time.Second), tsVideoID).Error; err != nil {
		t.Fatalf("改写 created_at 失败: %v", err)
	}

	snap, err := repo.QueueSnapshot()
	if err != nil {
		t.Fatalf("队列快照失败: %v", err)
	}
	if snap.Pending != 10 {
		t.Fatalf("排队数应为 10，实际 %d", snap.Pending)
	}
	if snap.OldestPendingSeconds < 90 {
		t.Fatalf("最老等待应 >=90s，实际 %d", snap.OldestPendingSeconds)
	}
	if snap.OldestPendingJobID == 0 {
		t.Fatal("应返回最老任务 ID")
	}

	// 认领 3 个 → running=3, pending=7
	for i := 0; i < 3; i++ {
		if j, err := repo.ClaimJob(fmt.Sprintf("w/%d", i), time.Minute); err != nil || j == nil {
			t.Fatalf("认领失败: %v", err)
		}
	}
	snap2, err := repo.QueueSnapshot()
	if err != nil {
		t.Fatalf("队列快照失败: %v", err)
	}
	if snap2.Pending != 7 || snap2.Running != 3 {
		t.Fatalf("认领后应为 pending=7 running=3，实际 pending=%d running=%d", snap2.Pending, snap2.Running)
	}

	// 全部处理完 → 排队归零：剩余 7 个先被认领再标记成功。
	var jobs []TranscodeJob
	if err := db.Where("status = ?", JobPending).Order("id").Find(&jobs).Error; err != nil {
		t.Fatalf("读取任务失败: %v", err)
	}
	for range jobs {
		claimed, err := repo.ClaimJob("drainer/1", time.Minute)
		if err != nil {
			t.Fatalf("认领失败: %v", err)
		}
		if claimed == nil {
			t.Fatal("仍有 pending 但认领不到（口径不一致）")
		}
		if err := repo.CompleteJob(claimed.ID, "drainer/1"); err != nil {
			t.Fatalf("标记完成失败: %v", err)
		}
	}
	// 已 running 的 3 个也标记成功
	var running []TranscodeJob
	if err := db.Where("status = ?", JobRunning).Find(&running).Error; err != nil {
		t.Fatalf("读取 running 失败: %v", err)
	}
	for _, j := range running {
		if err := repo.CompleteJob(j.ID, j.WorkerID); err != nil {
			t.Fatalf("标记完成失败: %v", err)
		}
	}
	snap3, err := repo.QueueSnapshot()
	if err != nil {
		t.Fatalf("队列快照失败: %v", err)
	}
	if snap3.Pending != 0 || snap3.Running != 0 {
		t.Fatalf("处理完应归零，实际 pending=%d running=%d", snap3.Pending, snap3.Running)
	}
	if snap3.Success != 10 {
		t.Fatalf("成功数应为 10，实际 %d", snap3.Success)
	}
}

// TestEmbeddedWorkerConsumesJobs 回归：StartTranscodeWorkers 仍是可用入口（dev 内嵌模式）。
// 只验证 worker 会认领并把任务推进到终态（用一个必然失败的档位，走完重试→死信），
// 不依赖 ffmpeg：档位 360 在测试库存在，但源文件不存在 → 转码失败 → 重试到上限。
func TestEmbeddedWorkerConsumesJobs(t *testing.T) {
	db := transcaleDB(t)
	cleanupTranscodeScale(t, db)
	defer cleanupTranscodeScale(t, db)

	seedVideo(t, db, tsVideoID)
	seedJobs(t, db, tsVideoID, 1)

	cfg := &config.Config{}
	cfg.Transcode.Workers = 1
	cfg.Transcode.MaxRetries = 0 // 一次失败即终态，测试更快
	cfg.Transcode.RetryBackoff = 10 * time.Millisecond
	cfg.Transcode.Lease = time.Minute
	cfg.Transcode.PollInterval = 50 * time.Millisecond
	cfg.Transcode.JobTimeout = 5 * time.Second
	cfg.Transcode.FfmpegPath = "ffmpeg"
	cfg.Transcode.FfprobePath = "ffprobe"

	svc := NewService(NewRepo(db), nil, nil, nil, nil, cfg, zap.NewNop())
	dir := t.TempDir()
	store, err := storage.NewLocal(dir, "http://localhost/static")
	if err != nil {
		t.Fatalf("初始化本地存储失败: %v", err)
	}
	svc.SetStorage(store)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.StartTranscodeWorkers(ctx, store)

	// 等待任务被消费到终态（源文件不存在 → 失败 → 死信）
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		var row TranscodeJob
		if err := db.First(&row).Error; err == nil && row.Status == JobFailed {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	var row TranscodeJob
	_ = db.First(&row).Error
	t.Fatalf("内嵌 Worker 未把任务推进到终态: status=%d retry=%d err=%s", row.Status, row.RetryCnt, row.ErrorMsg)
}

// 保证 filepath 被使用（某些构建下 storage 解析路径依赖它）
var _ = filepath.Join
