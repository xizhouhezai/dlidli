package live

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/dlidli/server/internal/pkg/errcode"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// testDB 连接测试库；未设置 DLIDLI_TEST_DSN 时跳过。
// 用真实 MySQL 验证唯一键/UPSERT/状态机语义，避免 mock 掩盖行为。
// 显式设置 DSN 却连不上时 Fatalf 而非 Skip（库挂了不得伪装成通过）。
func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("DLIDLI_TEST_DSN")
	if dsn == "" {
		t.Skip("DLIDLI_TEST_DSN 未设置，跳过 live 集成测试")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent), TranslateError: true})
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

// testRedis 测试 Redis；未设置 DLIDLI_TEST_REDIS 时返回 nil（Online 降级 DB 兜底路径同样被覆盖）。
func testRedis(t *testing.T) *redis.Client {
	t.Helper()
	addr := os.Getenv("DLIDLI_TEST_REDIS")
	if addr == "" {
		t.Log("DLIDLI_TEST_REDIS 未设置，在线计数走 DB 兜底路径")
		return nil
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("已设置 DLIDLI_TEST_REDIS 但无法连接: %v", err)
	}
	t.Cleanup(func() {
		_ = rdb.Close()
	})
	return rdb
}

// cleanup 清理本次测试数据（测试用户 ID 段 91000000+）。
func cleanup(t *testing.T, db *gorm.DB) {
	t.Helper()
	db.Exec("DELETE FROM live_room WHERE anchor_id BETWEEN 91000000 AND 91099999")
	db.Exec("DELETE FROM live_whitelist WHERE user_id BETWEEN 91000000 AND 91099999")
	db.Exec("DELETE FROM live_room_admin WHERE user_id BETWEEN 91000000 AND 91099999")
	db.Exec("DELETE FROM live_room_mute WHERE user_id BETWEEN 91000000 AND 91099999")
	db.Exec("DELETE FROM live_room_msg") // 弹幕历史窗口按房间隔离，测试房间回收时全量清理
	db.Exec("DELETE FROM `user` WHERE id BETWEEN 91000000 AND 91099999")
	db.Exec("DELETE FROM user_auth WHERE user_id BETWEEN 91000000 AND 91099999")
	db.Exec("DELETE FROM relation WHERE user_id BETWEEN 91000000 AND 91099999 OR target_id BETWEEN 91000000 AND 91099999")
}

// seedUser 造用户 + 手机号认证（实名基线）+ fans 个粉丝。
func seedUser(t *testing.T, db *gorm.DB, uid int64, fans int) {
	t.Helper()
	if err := db.Exec("INSERT INTO `user` (id, nickname, status, created_at, updated_at) VALUES (?, ?, 0, NOW(), NOW())", uid, fmt.Sprintf("livetest-%d", uid)).Error; err != nil {
		t.Fatalf("造用户失败: %v", err)
	}
	if err := db.Exec("INSERT INTO user_auth (user_id, identity_type, identifier, identifier_hash, credential, activated, created_at) VALUES (?, 1, ?, 'hash', 'cred', 1, NOW())", uid, fmt.Sprintf("enc-%d", uid)).Error; err != nil {
		t.Fatalf("造认证失败: %v", err)
	}
	for i := 0; i < fans; i++ {
		fid := 91000000 + 50000 + uid%1000*100 + int64(i)
		if err := db.Exec("INSERT IGNORE INTO relation (user_id, target_id, created_at) VALUES (?, ?, NOW())", fid, uid).Error; err != nil {
			t.Fatalf("造粉丝失败: %v", err)
		}
	}
}

// TestApplyQualification 资格三闸：未入白名单 / 未实名 / 粉丝不足 / 全通过。
func TestApplyQualification(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	t.Cleanup(func() { cleanup(t, db) })
	cleanup(t, db)

	repo := NewRepo(db)
	svc := &Service{repo: repo, online: NewOnline(nil, repo, zap.NewNop())}
	const uid = 91000001

	// 分支 1：未入白名单 → 91001
	if err := svc.CheckQualification(ctx, uid); err != errcode.ErrLiveNotWhitelisted {
		t.Fatalf("未入白名单应 91001, got %v", err)
	}
	// 入白名单但未实名（无 user_auth）→ 91002
	if err := db.Exec("INSERT INTO live_whitelist (user_id, note, created_at) VALUES (?, 'test', NOW())", uid).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.CheckQualification(ctx, uid); err != errcode.ErrLiveNotRealnamed {
		t.Fatalf("未实名应 91002, got %v", err)
	}
	// 实名但粉丝不足（0 粉）→ 91003
	seedUser(t, db, uid, 0)
	if err := svc.CheckQualification(ctx, uid); err != errcode.ErrLiveFansNotEnough {
		t.Fatalf("粉丝不足应 91003, got %v", err)
	}
	// 造满 100 粉（先补 99 个，seedUser 已造 0）→ 通过
	for i := 0; i < 100; i++ {
		db.Exec("INSERT IGNORE INTO relation (user_id, target_id, created_at) VALUES (?, ?, NOW())", 91000000+70000+int64(i), uid)
	}
	if err := svc.CheckQualification(ctx, uid); err != nil {
		t.Fatalf("三闸全过应放行, got %v", err)
	}
}

// TestRoomLifecycle 状态机：申请建房 → 重复申请拦截 → 开播 → 重复开播拦截 → 下播 → 重复下播拦截 → 回环。
func TestRoomLifecycle(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	t.Cleanup(func() { cleanup(t, db) })
	cleanup(t, db)

	repo := NewRepo(db)
	svc := &Service{repo: repo, online: NewOnline(nil, repo, zap.NewNop())}
	const uid = 91000002
	seedUser(t, db, uid, 100)
	db.Exec("INSERT INTO live_whitelist (user_id, note, created_at) VALUES (?, 't', NOW())", uid)

	res, err := svc.Apply(ctx, uid)
	if err != nil {
		t.Fatalf("申请开播失败: %v", err)
	}
	if res.Room.Status != RoomStatusIdle || res.PushURL == "" || len(res.StreamKey) != 32 {
		t.Fatalf("初始态/推流地址/密钥不符合: %+v", res)
	}
	if _, err := svc.Apply(ctx, uid); err != errcode.ErrLiveRoomExists {
		t.Fatalf("重复申请应 91004, got %v", err)
	}
	info, err := svc.Start(ctx, uid)
	if err != nil {
		t.Fatalf("开播失败: %v", err)
	}
	if info.Status != RoomStatusLive || info.StartedAt == nil {
		t.Fatalf("开播态不符合: %+v", info)
	}
	if _, err := svc.Start(ctx, uid); err != errcode.ErrLiveAlreadyLive {
		t.Fatalf("重复开播应 91005, got %v", err)
	}
	info, err = svc.Stop(ctx, uid)
	if err != nil {
		t.Fatalf("下播失败: %v", err)
	}
	if info.Status != RoomStatusEnded || info.EndedAt == nil {
		t.Fatalf("下播态不符合: %+v", info)
	}
	if _, err := svc.Stop(ctx, uid); err != errcode.ErrLiveNotLive {
		t.Fatalf("重复下播应 91006, got %v", err)
	}
	// 回环：再开播回到直播中
	info, err = svc.Start(ctx, uid)
	if err != nil || info.Status != RoomStatusLive {
		t.Fatalf("再开播应回直播中: %v %+v", err, info)
	}
	// 密钥校验：原密钥通过，重置后旧密钥失效、新密钥通过
	ok, err := svc.VerifyStreamKey(res.Room.ID, res.StreamKey)
	if err != nil || !ok {
		t.Fatalf("原密钥应通过校验: %v %v", ok, err)
	}
	res2, err := svc.ResetStreamKey(ctx, uid)
	if err != nil {
		t.Fatalf("重置密钥失败: %v", err)
	}
	if ok, _ := svc.VerifyStreamKey(res.Room.ID, res.StreamKey); ok {
		t.Fatal("重置后旧密钥应失效")
	}
	if ok, _ := svc.VerifyStreamKey(res.Room.ID, res2.StreamKey); !ok {
		t.Fatal("重置后新密钥应通过")
	}
}

// TestOnlineCounting 在线计数：进房判重 / 心跳续期与过期补记 / 离房幂等 / 下播清零。
func TestOnlineCounting(t *testing.T) {
	db := testDB(t)
	rdb := testRedis(t)
	ctx := context.Background()
	t.Cleanup(func() { cleanup(t, db) })
	cleanup(t, db)

	repo := NewRepo(db)
	online := NewOnline(rdb, repo, zap.NewNop())
	svc := &Service{repo: repo, online: online}
	const uid = 91000003
	// 清掉上一轮残留的 live:* 键，保证计数从 0 起
	if rdb != nil {
		rdb.FlushDB(ctx)
	}
	seedUser(t, db, uid, 100)
	db.Exec("INSERT INTO live_whitelist (user_id, note, created_at) VALUES (?, 't', NOW())", uid)

	res, err := svc.Apply(ctx, uid)
	if err != nil {
		t.Fatalf("申请失败: %v", err)
	}
	roomID := res.Room.ID
	if _, err := svc.Start(ctx, uid); err != nil {
		t.Fatalf("开播失败: %v", err)
	}

	// 3 用户进房（91000101-103，与主播 uid 不重叠）→ 3；同用户重复进房不重复计数
	for _, v := range []int64{101, 102, 103} {
		if _, err := svc.Enter(ctx, 91000000+v, roomID); err != nil {
			t.Fatalf("进房失败: %v", err)
		}
	}
	if _, err := svc.Enter(ctx, 91000101, roomID); err != nil {
		t.Fatal(err)
	}
	info, err := svc.RoomDetail(ctx, uid, roomID)
	if err != nil {
		t.Fatal(err)
	}
	if info.Online != 3 {
		t.Fatalf("重复进房应判重，在线应为 3, got %d", info.Online)
	}
	// 主播也计入 → 4
	if _, err := svc.Enter(ctx, uid, roomID); err != nil {
		t.Fatal(err)
	}
	if info, _ = svc.RoomDetail(ctx, uid, roomID); info.Online != 4 {
		t.Fatalf("主播进房后应为 4, got %d", info.Online)
	}
	// 心跳续期不增计数
	if err := svc.Heartbeat(ctx, 91000101, roomID); err != nil {
		t.Fatal(err)
	}
	if info, _ = svc.RoomDetail(ctx, uid, roomID); info.Online != 4 {
		t.Fatalf("心跳不应改变计数, got %d", info.Online)
	}
	// 模拟 30s 无心跳（DEL 心跳键，计数不减）→ 心跳视为重进房，计数不变（键已重建）
	if rdb != nil {
		rdb.Del(ctx, hbKey(roomID, 91000101))
		if err := svc.Heartbeat(ctx, 91000101, roomID); err != nil {
			t.Fatal(err)
		}
		if info, _ = svc.RoomDetail(ctx, uid, roomID); info.Online != 4 {
			t.Fatalf("过期重进后应为 4, got %d", info.Online)
		}
	}
	// 2 人离房 → 2；重复离房幂等
	if _, err := svc.Leave(ctx, 91000101, roomID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Leave(ctx, 91000102, roomID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Leave(ctx, 91000102, roomID); err != nil {
		t.Fatal(err)
	}
	if info, _ = svc.RoomDetail(ctx, uid, roomID); info.Online != 2 {
		t.Fatalf("离房后应为 2, got %d", info.Online)
	}
	// 下播清零
	if _, err := svc.Stop(ctx, uid); err != nil {
		t.Fatal(err)
	}
	if info, _ = svc.RoomDetail(ctx, uid, roomID); info.Online != 0 {
		t.Fatalf("下播后在线应清零, got %d", info.Online)
	}
}

// TestAdminAndMute 房管任命/撤销 + 禁言权限/范围/覆盖/解禁。
func TestAdminAndMute(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	t.Cleanup(func() { cleanup(t, db) })
	cleanup(t, db)

	repo := NewRepo(db)
	svc := &Service{repo: repo, online: NewOnline(nil, repo, zap.NewNop())}
	const anchor, viewer = 91000004, 91000005
	seedUser(t, db, anchor, 100)
	seedUser(t, db, viewer, 0)
	seedUser(t, db, 91000106, 0) // 被禁言的普通观众
	db.Exec("INSERT INTO live_whitelist (user_id, note, created_at) VALUES (?, 't', NOW())", anchor)

	res, _ := svc.Apply(ctx, anchor)
	roomID := res.Room.ID

	// 非主播不能任命（mustOwnedRoom 找不到其房间）
	if err := svc.AddAdmin(ctx, viewer, roomID, viewer); err != errcode.ErrLiveRoomNotFound {
		t.Fatalf("非主播任命应被拒, got %v", err)
	}
	// 主播不能任命自己
	if err := svc.AddAdmin(ctx, anchor, roomID, anchor); err != errcode.ErrLiveAdminSelf {
		t.Fatalf("主播任命自己应 91010, got %v", err)
	}
	// 任命 viewer 为房管（幂等）
	if err := svc.AddAdmin(ctx, anchor, roomID, viewer); err != nil {
		t.Fatal(err)
	}
	if ok, _ := repo.IsAdmin(roomID, viewer); !ok {
		t.Fatal("任命后应是房管")
	}
	if err := svc.AddAdmin(ctx, anchor, roomID, viewer); err != nil {
		t.Fatalf("重复任命应幂等: %v", err)
	}
	// 房管可禁言普通用户（先造一个普通用户 91000106），但不能禁言主播
	if err := svc.Mute(ctx, viewer, roomID, anchor, 10, "test"); err != errcode.ErrLiveAdminSelf {
		t.Fatalf("禁言主播应 91010, got %v", err)
	}
	if err := svc.Mute(ctx, viewer, roomID, 91000106, 10, "test"); err != nil {
		t.Fatalf("房管禁言失败: %v", err)
	}
	muted, err := svc.online.IsMuted(ctx, roomID, 91000106)
	if err != nil || !muted {
		t.Fatalf("禁言中应判定为 muted: %v %v", muted, err)
	}
	// 时长越界：0 与 720h+1min
	if err := svc.Mute(ctx, anchor, roomID, viewer, 0, ""); err != errcode.ErrLiveMuteRange {
		t.Fatalf("0 分钟应 91013, got %v", err)
	}
	if err := svc.Mute(ctx, anchor, roomID, viewer, 720*60+1, ""); err != errcode.ErrLiveMuteRange {
		t.Fatalf("超 720h 应 91013, got %v", err)
	}
	// 重复禁言覆盖刷新到期（对 viewer 先禁 10min 再禁 60min，到期应更晚）
	if err := svc.Mute(ctx, anchor, roomID, viewer, 10, "first"); err != nil {
		t.Fatal(err)
	}
	until1, _ := repo.MutedUntil(roomID, viewer)
	_ = svc.Mute(ctx, anchor, roomID, viewer, 60, "again")
	until2, _ := repo.MutedUntil(roomID, viewer)
	if until1 == nil || until2 == nil || !until2.After(*until1) {
		t.Fatalf("重复禁言应刷新到期: %v -> %v", until1, until2)
	}
	// 解禁
	if err := svc.Unmute(ctx, anchor, roomID, viewer); err != nil {
		t.Fatal(err)
	}
	if muted, _ = svc.online.IsMuted(ctx, roomID, viewer); muted {
		t.Fatal("解禁后不应 muted")
	}
	// 撤销房管后失去禁言权限
	if err := svc.RemoveAdmin(ctx, anchor, roomID, viewer); err != nil {
		t.Fatal(err)
	}
	if err := svc.Mute(ctx, viewer, roomID, 91000106, 10, ""); err != errcode.ErrLiveNotAdmin {
		t.Fatalf("撤销后禁言应 91009, got %v", err)
	}
}

// TestUpdateRoomFields 信息修改：标题长度/分区存在性校验与生效。
func TestUpdateRoomFields(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	t.Cleanup(func() { cleanup(t, db) })
	cleanup(t, db)

	repo := NewRepo(db)
	svc := &Service{repo: repo, online: NewOnline(nil, repo, zap.NewNop())}
	const uid = 91000007
	seedUser(t, db, uid, 100)
	db.Exec("INSERT INTO live_whitelist (user_id, note, created_at) VALUES (?, 't', NOW())", uid)

	if _, err := svc.Apply(ctx, uid); err != nil {
		t.Fatalf("申请失败: %v", err)
	}

	longTitle := make([]rune, 81)
	for i := range longTitle {
		longTitle[i] = '标'
	}
	if _, err := svc.UpdateRoom(ctx, uid, string(longTitle), "", 1); err != errcode.ErrLiveTitleRequired {
		t.Fatalf("81 字标题应 91014, got %v", err)
	}
	if _, err := svc.UpdateRoom(ctx, uid, "标题", "", 999999); err != errcode.ErrLiveCategoryInvalid {
		t.Fatalf("不存在分区应 91012, got %v", err)
	}
	var catID int64
	if err := db.Raw("SELECT id FROM category WHERE status = 0 LIMIT 1").Scan(&catID).Error; err != nil || catID == 0 {
		t.Skip("category 表无启用分区，跳过更新生效断言")
	}
	info, err := svc.UpdateRoom(ctx, uid, "测试标题", "https://cdn.example/c.jpg", catID)
	if err != nil {
		t.Fatalf("正常更新失败: %v", err)
	}
	if info.Title != "测试标题" || info.CategoryID != catID || info.Cover == "" {
		t.Fatalf("更新未生效: %+v", info)
	}
}
