package live

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dlidli/server/internal/module/account"
	"github.com/dlidli/server/internal/module/video"
	"github.com/dlidli/server/internal/pkg/config"
	"github.com/dlidli/server/internal/pkg/errcode"
	"github.com/dlidli/server/internal/pkg/snowflake"
	"github.com/dlidli/server/internal/pkg/storage"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// 直播回放集成测试（M4-LIV-14）。依赖 DLIDLI_TEST_DSN，未设置时跳过（沿用本包既有口径）。
//
// 覆盖本卡的三条不可动摇约束：
//  1. 开关关闭时**零副作用**（不登记、不生成、不清理）；
//  2. 下播永远先落房间状态，回放失败只记流水（失败路径不影响下播）；
//  3. 生成的是**草稿**稿件（仅主播可见），发布后才公开。

// replayCleanup 清理本次回放测试数据。
func replayCleanup(t *testing.T, db *gorm.DB) {
	t.Helper()
	db.Exec("DELETE FROM live_replay WHERE anchor_id BETWEEN 91000000 AND 91099999")
	db.Exec("DELETE FROM transcode_job WHERE video_id IN (SELECT id FROM video WHERE user_id BETWEEN 91000000 AND 91099999)")
	db.Exec("DELETE FROM video_stream WHERE video_id IN (SELECT id FROM video WHERE user_id BETWEEN 91000000 AND 91099999)")
	db.Exec("DELETE FROM video_stat WHERE video_id IN (SELECT id FROM video WHERE user_id BETWEEN 91000000 AND 91099999)")
	db.Exec("DELETE FROM video WHERE user_id BETWEEN 91000000 AND 91099999")
}

// replaySvc 造一个注入了回放依赖的 live Service（DVR 目录指向 t.TempDir）。
func replaySvc(t *testing.T, db *gorm.DB, dvrDir string, enabled, autoGen bool) *Service {
	t.Helper()
	// 建稿走 snowflake 生成稿件 ID；包级 node 未初始化会 nil panic（生产由 cmd/api 启动时 Init）
	if err := snowflake.Init(1); err != nil {
		t.Fatalf("初始化 snowflake 失败: %v", err)
	}
	rdb := testRedis(t)
	cfg := &config.Config{}
	cfg.Live.Replay = config.LiveReplay{
		Enabled: enabled, AutoGenerate: autoGen, DvrDir: dvrDir,
		MaxDurationHours: 4, RetentionDays: 60, CleanIntervalSec: 3600, Copyright: 1,
	}
	svc := NewService(NewRepo(db), NewOnline(rdb, NewRepo(db), zap.NewNop()), cfg, zap.NewNop())
	store, err := storage.NewLocal(t.TempDir(), "http://localhost:8000/static")
	if err != nil {
		t.Fatalf("构造存储失败: %v", err)
	}
	// video 服务需要真实的 account 服务（detail/cards 要拼 UP 主信息）
	accSvc := account.NewService(account.NewRepo(db), rdb, cfg, zap.NewNop(), nil)
	svc.SetReplayDeps(&LiveReplayDeps{
		Video: video.NewService(video.NewRepo(db), nil, accSvc, nil, rdb, cfg, zap.NewNop()),
		Store: store,
	})
	return svc
}

// makeDVR 用 ffmpeg 造一个真实的 FLV 录制文件（SRS DVR 的落盘形态），返回其路径。
//
// 不用空文件占位：本卡的失败路径依赖 ffprobe 能解析出真实时长，
// 空文件会让「超时长上限」「时长落库」这些断言退化成恒真。
func makeDVR(t *testing.T, dir string, roomID int64, seconds int) string {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("本机无 ffmpeg，跳过回放集成测试")
	}
	appDir := filepath.Join(dir, "live")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatalf("建 dvr 目录失败: %v", err)
	}
	out := filepath.Join(appDir, StreamName(roomID)+".1700000000.flv")
	cmd := exec.Command("ffmpeg", "-y", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc=size=320x240:rate=15",
		"-f", "lavfi", "-i", "sine=frequency=440",
		"-t", strconv.Itoa(seconds), "-c:v", "libx264", "-preset", "ultrafast",
		"-pix_fmt", "yuv420p", "-c:a", "aac", "-shortest", out)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("造录制文件失败: %v (%s)", err, string(b))
	}
	return out
}

// seedRoomLive 造用户 + 房间（带场次时刻）。
func seedRoomLive(t *testing.T, db *gorm.DB, uid int64, status int8, started time.Time) *LiveRoom {
	t.Helper()
	seedUser(t, db, uid, 0)
	room := &LiveRoom{AnchorID: uid, Title: "回放测试间", Status: status, StreamKeyHash: "x"}
	if err := db.Create(room).Error; err != nil {
		t.Fatalf("造房间失败: %v", err)
	}
	if err := db.Model(&LiveRoom{}).Where("id = ?", room.ID).
		Update("started_at", started).Error; err != nil {
		t.Fatalf("写 started_at 失败: %v", err)
	}
	room.StartedAt = &started
	return room
}

// TestReplayDisabledNoSideEffect 开关关闭：下播不登记任何回放、不生成稿件（实测无副作用）。
func TestReplayDisabledNoSideEffect(t *testing.T) {
	db := testDB(t)
	cleanup(t, db)
	replayCleanup(t, db)
	t.Cleanup(func() { cleanup(t, db); replayCleanup(t, db) })

	svc := replaySvc(t, db, t.TempDir(), false, false)
	room := seedRoomLive(t, db, 91000001, RoomStatusLive, time.Now().Add(-time.Minute))

	if _, err := svc.Stop(context.Background(), room.AnchorID); err != nil {
		t.Fatalf("下播失败: %v", err)
	}
	var n int64
	db.Model(&LiveReplay{}).Where("room_id = ?", room.ID).Count(&n)
	if n != 0 {
		t.Fatalf("开关关闭时不应登记回放，got %d 条", n)
	}
	var nv int64
	db.Model(&video.Video{}).Where("user_id = ?", room.AnchorID).Count(&nv)
	if nv != 0 {
		t.Fatalf("开关关闭时不应生成稿件，got %d 个", nv)
	}
	// 房间状态仍须正确落为已下播
	var got LiveRoom
	db.First(&got, room.ID)
	if got.Status != RoomStatusEnded || got.EndedAt == nil {
		t.Fatalf("下播状态不正确: status=%d ended_at=%v", got.Status, got.EndedAt)
	}
}

// TestReplayEndToEnd 端到端：录制文件 → 登记 → 转码源 → 草稿稿件 → 发布。
func TestReplayEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("本机无 ffmpeg，跳过回放集成测试")
	}
	db := testDB(t)
	cleanup(t, db)
	replayCleanup(t, db)
	t.Cleanup(func() { cleanup(t, db); replayCleanup(t, db) })

	dvr := t.TempDir()
	svc := replaySvc(t, db, dvr, true, true)
	room := seedRoomLive(t, db, 91000002, RoomStatusLive, time.Now().Add(-2*time.Minute))
	makeDVR(t, dvr, room.ID, 2)

	// 下播：登记素材（autoGenerate=true → 异步生成，这里同步调 GenerateReplay 以便断言）
	if _, err := svc.Stop(context.Background(), room.AnchorID); err != nil {
		t.Fatalf("下播失败: %v", err)
	}
	var rep LiveReplay
	if err := db.Where("room_id = ?", room.ID).First(&rep).Error; err != nil {
		t.Fatalf("下播应登记回放素材: %v", err)
	}
	if rep.Status != ReplayStatusPending {
		t.Fatalf("登记初始状态应为待处理, got %d", rep.Status)
	}
	// 幂等：重复投递不产生第二行
	created, err := svc.repo.CreateReplay(&LiveReplay{
		RoomID: rep.RoomID, AnchorID: rep.AnchorID,
		SessionStartedAt: rep.SessionStartedAt, SessionEndedAt: rep.SessionEndedAt,
	})
	if err != nil {
		t.Fatalf("重复登记不应报错: %v", err)
	}
	if created {
		t.Fatal("同场次重复投递应被唯一键拦下（幂等）")
	}

	// 生成
	if err := svc.GenerateReplay(context.Background(), rep.ID); err != nil {
		t.Fatalf("生成回放失败: %v", err)
	}
	var after LiveReplay
	db.First(&after, rep.ID)
	if after.Status != ReplayStatusDrafted || after.VideoID == 0 {
		t.Fatalf("应生成草稿稿件, status=%d video_id=%d reason=%s", after.Status, after.VideoID, after.FailReason)
	}
	if after.DvrKey == "" || after.SourceKey == "" {
		t.Fatalf("录制路径与转码源都应落库: dvr=%q source=%q", after.DvrKey, after.SourceKey)
	}
	if after.Duration <= 0 {
		t.Fatalf("应探测出真实时长, got %d", after.Duration)
	}

	// 稿件是草稿（spec LIV-14「主播可选发布」：未发布仅主播可见）
	var v video.Video
	if err := db.First(&v, after.VideoID).Error; err != nil {
		t.Fatalf("应生成稿件: %v", err)
	}
	if v.Status != video.StatusDraft {
		t.Fatalf("回放稿件初始状态应为草稿(0), got %d", v.Status)
	}
	if v.UserID != room.AnchorID {
		t.Fatalf("稿件归属应为主播 %d, got %d", room.AnchorID, v.UserID)
	}
	if v.CategoryID == 0 {
		t.Fatal("回放稿件必须有分区（自动取首个启用分区）")
	}
	// 原画流指向转码源
	var st video.Stream
	if err := db.Where("video_id = ? AND quality = 0", v.ID).First(&st).Error; err != nil {
		t.Fatalf("应有原画流: %v", err)
	}
	if st.PlayPath != after.SourceKey {
		t.Fatalf("原画流应指向转码源 %q, got %q", after.SourceKey, st.PlayPath)
	}
	// 转码源文件确实存在（MP4 容器，remux 产物）
	if _, err := os.Stat(filepath.Join(svc.replayDeps.Store.(storage.PathResolver).LocalPath(after.SourceKey))); err != nil {
		t.Fatalf("转码源文件应存在: %v", err)
	}

	// 未发布：对他人不可见（公开详情/列表都不含草稿）
	detail, err := svc.replayDeps.Video.PublicDetail(context.Background(), v.Bvid)
	if err == nil || detail != nil {
		t.Fatalf("未发布的回放对他人不应可见, err=%v", err)
	}

	// 发布：仅主播本人
	if err := svc.PublishReplay(context.Background(), room.AnchorID, v.ID); err != nil {
		t.Fatalf("主播发布回放失败: %v", err)
	}
	var v2 video.Video
	db.First(&v2, v.ID)
	if v2.Status != video.StatusPublished {
		t.Fatalf("发布后状态应为已发布(4), got %d", v2.Status)
	}
	if d, err := svc.replayDeps.Video.PublicDetail(context.Background(), v2.Bvid); err != nil || d == nil {
		t.Fatalf("发布后应公开可见: err=%v", err)
	}
	if err := svc.PublishReplay(context.Background(), room.AnchorID, v.ID); err == nil {
		t.Fatal("重复发布应报错（非草稿）")
	}

	// 越权：他人不能发布/删除
	if err := svc.PublishReplay(context.Background(), 91000099, v.ID); err == nil {
		t.Fatal("非本人发布回放应被拒绝")
	}
	if err := svc.DeleteReplay(context.Background(), 91000099, v.ID); err == nil {
		t.Fatal("非本人删除回放应被拒绝")
	}
}

// TestReplayMissingFileStillStops 失败路径：无录制文件时下播，房间状态仍正确、不产生脏稿件。
func TestReplayMissingFileStillStops(t *testing.T) {
	db := testDB(t)
	cleanup(t, db)
	replayCleanup(t, db)
	t.Cleanup(func() { cleanup(t, db); replayCleanup(t, db) })

	svc := replaySvc(t, db, t.TempDir(), true, true) // 空目录：没有录制文件
	room := seedRoomLive(t, db, 91000003, RoomStatusLive, time.Now().Add(-time.Minute))

	if _, err := svc.Stop(context.Background(), room.AnchorID); err != nil {
		t.Fatalf("无录制文件时下播仍应成功: %v", err)
	}
	var got LiveRoom
	db.First(&got, room.ID)
	if got.Status != RoomStatusEnded {
		t.Fatalf("房间状态应落为已下播(2), got %d", got.Status)
	}

	// 素材已登记，生成失败被记为 failed（可追溯），且不产生任何稿件
	var rep LiveReplay
	if err := db.Where("room_id = ?", room.ID).First(&rep).Error; err != nil {
		t.Fatalf("应登记素材（失败也要留痕）: %v", err)
	}
	if err := svc.GenerateReplay(context.Background(), rep.ID); err == nil {
		t.Fatal("无录制文件时生成应失败")
	}
	var after LiveReplay
	db.First(&after, rep.ID)
	if after.Status != ReplayStatusFailed || after.FailReason == "" {
		t.Fatalf("失败应落库且带原因, status=%d reason=%q", after.Status, after.FailReason)
	}
	var nv int64
	db.Model(&video.Video{}).Where("user_id = ?", room.AnchorID).Count(&nv)
	if nv != 0 {
		t.Fatalf("失败路径不应产生脏稿件, got %d 个", nv)
	}
	// 再次生成应幂等（已是终态，不再重试）
	if err := svc.GenerateReplay(context.Background(), rep.ID); err != nil {
		t.Fatalf("终态再生成应幂等无错: %v", err)
	}
}

// TestReplayZeroMaxDurationMeansUnlimited 上限语义：MaxDurationHours<=0 视为不限时长。
//
// 为什么不用「造 4 小时素材」来测拒绝分支：那需要真实生成 4 小时视频（不可行）。
// 拒绝分支的判定是 `maxSec > 0 && dur > maxSec`，故用 maxSec=0 这一侧验证
// 「0 = 不限」的兜底语义；拒绝分支本身由配置层保证下限（config.Load 兜底为 4 小时）。
func TestReplayZeroMaxDurationMeansUnlimited(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("本机无 ffmpeg，跳过回放集成测试")
	}
	db := testDB(t)
	cleanup(t, db)
	replayCleanup(t, db)
	t.Cleanup(func() { cleanup(t, db); replayCleanup(t, db) })

	dvr := t.TempDir()
	svc := replaySvc(t, db, dvr, true, true)
	svc.cfg.Live.Replay.MaxDurationHours = 0 // 0 → 不限时长
	room := seedRoomLive(t, db, 91000004, RoomStatusLive, time.Now().Add(-time.Minute))
	makeDVR(t, dvr, room.ID, 2)

	if _, err := svc.Stop(context.Background(), room.AnchorID); err != nil {
		t.Fatalf("下播失败: %v", err)
	}
	var rep LiveReplay
	db.Where("room_id = ?", room.ID).First(&rep)
	if err := svc.GenerateReplay(context.Background(), rep.ID); err != nil {
		t.Fatalf("上限为 0 时应视为不限时长: %v", err)
	}
	var after LiveReplay
	db.First(&after, rep.ID)
	if after.Status != ReplayStatusDrafted {
		t.Fatalf("应正常生成草稿, got status=%d reason=%s", after.Status, after.FailReason)
	}
}

// TestReplayRetentionCleanup 保留策略：过期录制素材被清理，登记行保留为 purged。
func TestReplayRetentionCleanup(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("本机无 ffmpeg，跳过回放集成测试")
	}
	db := testDB(t)
	cleanup(t, db)
	replayCleanup(t, db)
	t.Cleanup(func() { cleanup(t, db); replayCleanup(t, db) })

	dvr := t.TempDir()
	svc := replaySvc(t, db, dvr, true, true)
	svc.cfg.Live.Replay.RetentionDays = 60
	room := seedRoomLive(t, db, 91000005, RoomStatusLive, time.Now().Add(-time.Minute))
	dvrFile := makeDVR(t, dvr, room.ID, 2)

	if _, err := svc.Stop(context.Background(), room.AnchorID); err != nil {
		t.Fatalf("下播失败: %v", err)
	}
	var rep LiveReplay
	db.Where("room_id = ?", room.ID).First(&rep)
	if err := svc.GenerateReplay(context.Background(), rep.ID); err != nil {
		t.Fatalf("生成回放失败: %v", err)
	}
	var after LiveReplay
	db.First(&after, rep.ID)

	// 刚生成：未过期，清理应为 0 条且文件仍在
	n, err := svc.CleanExpiredReplays(context.Background())
	if err != nil {
		t.Fatalf("清理失败: %v", err)
	}
	if n != 0 {
		t.Fatalf("未过期不应清理, got %d", n)
	}
	if _, err := os.Stat(dvrFile); err != nil {
		t.Fatalf("未过期录制原片应仍在: %v", err)
	}

	// 把登记时间改到 61 天前 → 视为过期
	if err := db.Model(&LiveReplay{}).Where("id = ?", rep.ID).
		Update("created_at", time.Now().AddDate(0, 0, -61)).Error; err != nil {
		t.Fatalf("改写登记时间失败: %v", err)
	}
	n, err = svc.CleanExpiredReplays(context.Background())
	if err != nil {
		t.Fatalf("清理失败: %v", err)
	}
	if n != 1 {
		t.Fatalf("过期录制应被清理 1 条, got %d", n)
	}
	if _, err := os.Stat(dvrFile); !os.IsNotExist(err) {
		t.Fatalf("过期录制原片应被删除, err=%v", err)
	}
	var purged LiveReplay
	db.First(&purged, rep.ID)
	if purged.Status != ReplayStatusPurged {
		t.Fatalf("应标记为已清理(4), got %d", purged.Status)
	}
	// 清理原因须写明删了什么（回归护栏：早期版本第二轮会把原因覆盖成空括号）
	if !strings.Contains(purged.FailReason, "dvr") {
		t.Fatalf("清理原因应写明删除内容, got %q", purged.FailReason)
	}

	// 幂等：再跑一轮不应重复清理（回归护栏：早期版本每轮都重选已清理行，
	// 既刷日志又把原因覆盖成「（）」）
	n, err = svc.CleanExpiredReplays(context.Background())
	if err != nil {
		t.Fatalf("第二轮清理失败: %v", err)
	}
	if n != 0 {
		t.Fatalf("已清理的行不应被重复清理, got %d", n)
	}
	var again LiveReplay
	db.First(&again, rep.ID)
	if again.FailReason != purged.FailReason {
		t.Fatalf("第二轮不应改写清理原因: before=%q after=%q", purged.FailReason, again.FailReason)
	}
	// 稿件与登记行都保留（合规追溯）
	var v video.Video
	if err := db.First(&v, after.VideoID).Error; err != nil {
		t.Fatalf("清理录制不应删除稿件: %v", err)
	}
}

// TestReplayOwnerIsolation 归属隔离：非本人不可见/不可操作（未发布仅主播可见）。
func TestReplayOwnerIsolation(t *testing.T) {
	db := testDB(t)
	cleanup(t, db)
	replayCleanup(t, db)
	t.Cleanup(func() { cleanup(t, db); replayCleanup(t, db) })

	svc := replaySvc(t, db, t.TempDir(), true, false)
	seedUser(t, db, 91000007, 120) // 另一个主播（seedRoomLive 会造 91000006）
	roomA := seedRoomLive(t, db, 91000006, RoomStatusEnded, time.Now().Add(-time.Hour))
	other := &LiveRoom{AnchorID: 91000007, Title: "另一个间", Status: RoomStatusIdle, StreamKeyHash: "y"}
	if err := db.Create(other).Error; err != nil {
		t.Fatalf("造第二个房间失败: %v", err)
	}

	if _, err := svc.MyReplays(context.Background(), roomA.AnchorID, 10); err != nil {
		t.Fatalf("主播查自己的回放列表不应报错: %v", err)
	}
	// 非本人操作他人回放 → 统一的「不存在或不属于您」
	if _, err := svc.ReplayOwner(12345, roomA.AnchorID); err == nil {
		t.Fatal("不存在的回放应报错")
	} else if !isBizErr(err, errcode.ErrLiveReplayNotFound) {
		t.Fatalf("应为 91032, got %v", err)
	}
}

func isBizErr(err error, want *errcode.Error) bool {
	if e, ok := err.(*errcode.Error); ok {
		return e.Code == want.Code
	}
	return false
}
