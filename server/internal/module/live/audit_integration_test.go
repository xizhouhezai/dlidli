package live

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/dlidli/server/internal/pkg/errcode"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// M4-LIV-05 处置链集成测试（真实 MySQL）。
// 覆盖 audit_test.go 无法覆盖的行为级路径：CutRoom 的状态前置、流水顺序、降级路径、
// 封禁幂等、恢复，以及截帧机审落库与 fail-open。

// seedAuditRoom 造一个房间并置指定状态（不走资格流程，直接落库）。
func seedAuditRoom(t *testing.T, db *gorm.DB, uid int64, status int8) *LiveRoom {
	t.Helper()
	room := &LiveRoom{AnchorID: uid, Title: "audit-test", Status: status,
		StreamKeyHash: "x", CategoryID: 1}
	if err := db.Create(room).Error; err != nil {
		t.Fatalf("造房间失败: %v", err)
	}
	if status == RoomStatusLive {
		now := time.Now()
		if err := db.Model(&LiveRoom{}).Where("id = ?", room.ID).
			Update("started_at", now).Error; err != nil {
			t.Fatal(err)
		}
	}
	return room
}

// auditSvc 构造只依赖 DB 的审计服务（SRS 指向一个必然不可达的地址 → 走降级路径）。
func auditSvc(db *gorm.DB) *Service {
	cfg := streamTestCfg()
	cfg.Live.Audit.SrsAPIBase = "http://127.0.0.1:1" // 不可达，强制降级
	cfg.Live.Audit.FrameDir = filepath.Join("testdata-audit")
	cfg.Live.Audit.FlvTimeoutSeconds = 1
	return &Service{
		repo:   NewRepo(db),
		online: NewOnline(nil, NewRepo(db), zap.NewNop()),
		cfg:    cfg,
		log:    zap.NewNop(),
	}
}

// TestCutRoomRequiresLive 未在直播中的房间不得切断，且**不产生违规流水**（spec §5E.3）。
func TestCutRoomRequiresLive(t *testing.T) {
	db := testDB(t)
	cleanup(t, db)
	defer cleanup(t, db)

	// 一主播一房（uk_anchor），两个状态用例必须用不同 user_id 各建一房
	for i, st := range []int8{RoomStatusIdle, RoomStatusEnded} {
		room := seedAuditRoom(t, db, 91000501+int64(i), st)
		svc := auditSvc(db)

		_, err := svc.CutRoom(context.Background(), room.ID, ViolationSourceManual, "x", 0, 1)
		if !errors.Is(err, errcode.ErrLiveNotLive) {
			t.Fatalf("status=%d 应返回 91006 未在直播，got %v", st, err)
		}
		var n int64
		if err := db.Model(&LiveViolationLog{}).Where("room_id = ?", room.ID).Count(&n).Error; err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Fatalf("status=%d 被拒时不得产生违规流水，got %d 条", st, n)
		}
		// 房间不得被封禁
		var got LiveRoom
		if err := db.First(&got, room.ID).Error; err != nil {
			t.Fatal(err)
		}
		if got.BanStatus != RoomBanNone {
			t.Fatalf("status=%d 被拒时不得封禁，got ban_status=%d", st, got.BanStatus)
		}
	}
}

// TestCutRoomDegradedAndIdempotent 直播中房间：SRS 不可达 → 降级封禁 + 流水 cut_mode=2；
// 重复切断返回 91028 且不重复落流水。
func TestCutRoomDegradedAndIdempotent(t *testing.T) {
	db := testDB(t)
	cleanup(t, db)
	defer cleanup(t, db)

	room := seedAuditRoom(t, db, 91000502, RoomStatusLive)
	svc := auditSvc(db)

	res, err := svc.CutRoom(context.Background(), room.ID, ViolationSourceAuto, "黑屏", 0, 0)
	if err != nil {
		t.Fatalf("切断失败: %v", err)
	}
	if res.CutMode != CutModeDegraded || res.Kicked {
		t.Fatalf("SRS 不可达应降级：cut_mode=%d kicked=%v", res.CutMode, res.Kicked)
	}

	// 违规流水已落且 cut_mode 回填为 2（先落流水再切断）
	var vlog LiveViolationLog
	if err := db.First(&vlog, res.ViolationID).Error; err != nil {
		t.Fatalf("违规流水不存在: %v", err)
	}
	if vlog.CutMode != CutModeDegraded || vlog.Source != ViolationSourceAuto {
		t.Fatalf("流水内容不符: %+v", vlog)
	}

	// 房间被封禁，但 status 不被伪造（仍为 1，由 on_unpublish 回调负责置 2）
	var got LiveRoom
	if err := db.First(&got, room.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.BanStatus != RoomBanApplied {
		t.Fatalf("房间应被封禁，got ban_status=%d", got.BanStatus)
	}
	if got.Status != RoomStatusLive {
		t.Fatalf("CutRoom 不得改 status（正交），got %d", got.Status)
	}
	if got.BanReason != "黑屏" || got.BanAt == nil {
		t.Fatalf("封禁元数据不全: %+v", got)
	}

	// 重复切断：91028 且流水不增加
	if _, err := svc.CutRoom(context.Background(), room.ID, ViolationSourceAuto, "黑屏", 0, 0); !errors.Is(err, errcode.ErrLiveRoomBanned) {
		t.Fatalf("重复切断应返回 91028，got %v", err)
	}
	var n int64
	if err := db.Model(&LiveViolationLog{}).Where("room_id = ?", room.ID).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("重复切断不得重复落流水，got %d 条", n)
	}
}

// TestAuthorizePublishRejectsBannedRoom 封禁房间即使密钥正确也拒绝推流（plan §2D.3）。
func TestAuthorizePublishRejectsBannedRoom(t *testing.T) {
	db := testDB(t)
	cleanup(t, db)
	defer cleanup(t, db)

	room := seedAuditRoom(t, db, 91000503, RoomStatusLive)
	svc := auditSvc(db)

	// 直接写入一个已知明文密钥的 bcrypt 哈希，确保拒绝来自封禁而非密钥（不依赖资格流程）
	const key = "known-stream-key-1234567890abcdef"
	hash, err := bcrypt.GenerateFromPassword([]byte(key), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.repo.UpdateRoomFields(room.ID, map[string]any{"stream_key_hash": string(hash)}); err != nil {
		t.Fatal(err)
	}
	req := &PublishAuthReq{App: "live", Stream: roomStreamName(room.ID), Param: "?key=" + key}
	if _, err := svc.AuthorizePublish(context.Background(), req); err != nil {
		t.Fatalf("未封禁时应放行，got %v", err)
	}

	// 封禁后再推：必须 91028
	if err := svc.repo.UpdateRoomFields(room.ID, map[string]any{"ban_status": RoomBanApplied}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AuthorizePublish(context.Background(), req); !errors.Is(err, errcode.ErrLiveRoomBanned) {
		t.Fatalf("封禁房间应拒绝推流（91028），got %v", err)
	}
}

// TestRestoreRoom 解封：1→0 清空元数据；非封禁房间返回 91029。
func TestRestoreRoom(t *testing.T) {
	db := testDB(t)
	cleanup(t, db)
	defer cleanup(t, db)

	room := seedAuditRoom(t, db, 91000504, RoomStatusLive)
	svc := auditSvc(db)

	// 未封禁 → 91029
	if err := svc.RestoreRoom(context.Background(), room.ID, 1); !errors.Is(err, errcode.ErrLiveRoomNotBanned) {
		t.Fatalf("未封禁应返回 91029，got %v", err)
	}

	if _, err := svc.CutRoom(context.Background(), room.ID, ViolationSourceManual, "巡查", 0, 77); err != nil {
		t.Fatal(err)
	}
	if err := svc.RestoreRoom(context.Background(), room.ID, 77); err != nil {
		t.Fatalf("解封失败: %v", err)
	}
	var got LiveRoom
	if err := db.First(&got, room.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.BanStatus != RoomBanNone || got.BanReason != "" || got.BanBy != 0 || got.BanAt != nil {
		t.Fatalf("解封应清空封禁元数据: %+v", got)
	}
}

// TestCutRoomUnknownRoom 房间不存在返回 91007。
func TestCutRoomUnknownRoom(t *testing.T) {
	db := testDB(t)
	svc := auditSvc(db)
	if _, err := svc.CutRoom(context.Background(), 99999999999, ViolationSourceManual, "x", 0, 1); !errors.Is(err, errcode.ErrLiveRoomNotFound) {
		t.Fatalf("未知房间应返回 91007，got %v", err)
	}
}

// TestAuditRoomFailOpenOnCaptureError 截帧失败 → degraded=true、pass=true、不切断（fail-open）。
func TestAuditRoomFailOpenOnCaptureError(t *testing.T) {
	db := testDB(t)
	cleanup(t, db)
	defer cleanup(t, db)

	room := seedAuditRoom(t, db, 91000505, RoomStatusLive)
	svc := auditSvc(db)
	svc.cfg.Live.Audit.FrameDir = t.TempDir()
	svc.cfg.Live.PlayDomain = "" // 拉流地址不可用 → 截帧必然失败

	hitted, err := svc.auditRoom(context.Background(), room)
	if err == nil {
		t.Fatal("截帧应失败（playDomain 未配置）")
	}
	if hitted {
		t.Fatal("fail-open：截帧失败不得判定命中")
	}

	var rec LiveFrameAudit
	if err := db.Where("room_id = ?", room.ID).Order("id DESC").First(&rec).Error; err != nil {
		t.Fatalf("应落一条降级截帧记录: %v", err)
	}
	if !rec.Degraded || !rec.Pass {
		t.Fatalf("降级记录应 degraded=true pass=true，got %+v", rec)
	}
	// 不得切断：房间未被封禁，且无违规流水
	var got LiveRoom
	if err := db.First(&got, room.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.BanStatus != RoomBanNone {
		t.Fatal("fail-open：机审不可用时不得封禁房间")
	}
	var n int64
	if err := db.Model(&LiveViolationLog{}).Where("room_id = ?", room.ID).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("fail-open：不得产生违规流水，got %d", n)
	}
}

// TestAuditOnceSkipsBanned 已封禁房间不再重复判定。
func TestAuditOnceSkipsBanned(t *testing.T) {
	db := testDB(t)
	cleanup(t, db)
	defer cleanup(t, db)

	banned := seedAuditRoom(t, db, 91000506, RoomStatusLive)
	if err := db.Model(&LiveRoom{}).Where("id = ?", banned.ID).
		Update("ban_status", RoomBanApplied).Error; err != nil {
		t.Fatal(err)
	}
	svc := auditSvc(db)
	svc.cfg.Live.Audit.FrameDir = t.TempDir()
	svc.cfg.Live.PlayDomain = ""

	audited, _, err := svc.AuditOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// 审计范围 = 直播中且未封禁的房间（库里可能有其它真实房间，故按条件计数而非硬编码）
	var expect int64
	if err := db.Model(&LiveRoom{}).
		Where("status = ? AND ban_status = ?", RoomStatusLive, RoomBanNone).
		Count(&expect).Error; err != nil {
		t.Fatal(err)
	}
	if int64(audited) != expect {
		t.Fatalf("audited=%d，期望=未封禁直播中房间数 %d（已封禁房间应被跳过）", audited, expect)
	}
	var n int64
	if err := db.Model(&LiveFrameAudit{}).Where("room_id = ?", banned.ID).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("已封禁房间不应产生截帧记录，got %d", n)
	}
}
