package upload

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dlidli/server/internal/pkg/snowflake"
	"github.com/dlidli/server/internal/pkg/storage"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// 集成测试：用真实 MySQL + Redis 验证「跨用户上传同一文件」的归属语义。
// 这两条用例对应 2026-09-30 发现的缺陷①（complete 返回他人 file_id）与缺陷②
// （init 复用他人上传会话）。用真实依赖而非 mock，因为缺陷恰恰出在
// 「唯一键冲突 + 归属校验」的真实交互上，mock 会把它们掩盖掉。
//
// DLIDLI_TEST_DSN 未设置时跳过；设置却连不上则直接失败（避免「库挂了」伪装成「测试通过」）。

func testEnv(t *testing.T) (*Service, *gorm.DB, func()) {
	t.Helper()
	// 雪花 ID 是包级全局，Init 未调用时 NextID 会空指针 panic
	if err := snowflake.Init(1); err != nil {
		t.Fatalf("初始化雪花 ID 失败: %v", err)
	}
	dsn := os.Getenv("DLIDLI_TEST_DSN")
	if dsn == "" {
		t.Skip("DLIDLI_TEST_DSN 未设置，跳过上传模块集成测试")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("已设置 DLIDLI_TEST_DSN 但无法连接测试库: %v", err)
	}
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS upload_file (
		id BIGINT UNSIGNED PRIMARY KEY,
		user_id BIGINT UNSIGNED NOT NULL,
		file_name VARCHAR(255) NOT NULL DEFAULT '',
		file_hash CHAR(64) NOT NULL,
		file_size BIGINT NOT NULL DEFAULT 0,
		store_key VARCHAR(255) NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		UNIQUE KEY uk_user_hash (user_id, file_hash),
		KEY idx_user (user_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`).Error; err != nil {
		t.Fatalf("建 upload_file 表失败: %v", err)
	}
	// 测试库可能残留旧的全局唯一键（真实线上 schema 是 uk_hash），
	// 显式对齐到「有 uk_hash 就删掉」，以便用例能同时验证修复前后两种 schema。
	db.Exec("ALTER TABLE upload_file DROP INDEX uk_hash")
	if os.Getenv("DLIDLI_TEST_LEGACY_SCHEMA") == "1" {
		// 复现缺陷①用：还原为全局唯一键
		db.Exec("ALTER TABLE upload_file DROP INDEX uk_user_hash")
		if err := db.Exec("ALTER TABLE upload_file ADD UNIQUE KEY uk_hash (file_hash)").Error; err != nil {
			t.Fatalf("还原 uk_hash 失败: %v", err)
		}
	}

	rdbAddr := os.Getenv("DLIDLI_TEST_REDIS")
	if rdbAddr == "" {
		rdbAddr = "127.0.0.1:6379"
	}
	rdb := redis.NewClient(&redis.Options{Addr: rdbAddr, DB: 15})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("测试 Redis ping 失败（%s）: %v", rdbAddr, err)
	}

	tmpDir, err := os.MkdirTemp("", "upload-test-*")
	if err != nil {
		t.Fatalf("建临时目录失败: %v", err)
	}
	store, err := storage.NewLocal(tmpDir, "http://localhost/static")
	if err != nil {
		t.Fatalf("建本地存储失败: %v", err)
	}

	svc := NewService(NewRepo(db), rdb, store, tmpDir, zap.NewNop())
	cleanup := func() {
		db.Exec("DELETE FROM upload_file WHERE file_name LIKE 'itest-%'")
		os.RemoveAll(tmpDir)
		rdb.Close()
	}
	return svc, db, cleanup
}

// putFile 走完整上传链路：init → 分片 → complete。
func putFile(t *testing.T, svc *Service, uid int64, name string, data []byte) (*CompleteResp, error) {
	t.Helper()
	ctx := context.Background()
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])

	init, err := svc.Init(ctx, uid, &InitReq{FileName: name, FileSize: int64(len(data)), FileHash: hash})
	if err != nil {
		return nil, err
	}
	if init.Fast {
		return &CompleteResp{FileID: init.FileID, StoreKey: init.StoreKey}, nil
	}
	for i := 0; i < init.ChunkCount; i++ {
		start := int64(i) * init.ChunkSize
		end := start + init.ChunkSize
		if end > int64(len(data)) {
			end = int64(len(data))
		}
		if err := svc.UploadPart(ctx, uid, init.UploadID, i, strings.NewReader(string(data[start:end]))); err != nil {
			return nil, fmt.Errorf("上传分片 %d 失败: %w", i, err)
		}
	}
	return svc.Complete(ctx, uid, init.UploadID)
}

// 缺陷①：A 上传后，B 上传同一文件应得到「属于 B 自己」的 file_id。
// 修复前：complete 命中 uk_hash 冲突 → Create 回退 FindByHash → 返回 A 的记录，
// 于是 B 拿到的 file_id 归属 A，投稿时被 VID-24 拒绝。
func TestComplete_CrossUserSameFile_ReturnsOwnFileID(t *testing.T) {
	svc, _, cleanup := testEnv(t)
	defer cleanup()

	data := []byte("itest-cross-user-payload-" + time.Now().Format(time.RFC3339Nano))

	resA, err := putFile(t, svc, 1001, "itest-a.mp4", data)
	if err != nil {
		t.Fatalf("用户A上传失败: %v", err)
	}
	resB, err := putFile(t, svc, 2002, "itest-b.mp4", data)
	if err != nil {
		t.Fatalf("用户B上传同一文件失败（修复前会命中他人记录）: %v", err)
	}

	if resA.FileID == resB.FileID {
		t.Fatalf("跨用户上传同一文件返回了同一个 file_id（%s）：B 拿到的是 A 的记录，投稿必被 VID-24 拒绝", resA.FileID)
	}

	// 关键断言：B 拿到的 file_id 必须通过「属主为 B」的归属校验
	fileID := parseID(t, resB.FileID)
	if _, err := svc.GetUserFile(context.Background(), 2002, fileID); err != nil {
		t.Fatalf("用户B用自己的 file_id 校验归属失败: %v（说明返回的不是 B 的文件）", err)
	}
	// 且 A 不能用 B 的 file_id
	if _, err := svc.GetUserFile(context.Background(), 1001, fileID); err == nil {
		t.Fatal("用户A 不应能使用用户B 的 file_id")
	}
}

// 缺陷②：A 的会话未完成时，B 上传同一文件应拿到「自己的新会话」，
// 而不是复用 A 的 uploadID（修复前复用 → B 在上传分片时撞 ErrForbidden）。
func TestInit_CrossUserSameFile_DoesNotReuseOthersSession(t *testing.T) {
	svc, _, cleanup := testEnv(t)
	defer cleanup()

	ctx := context.Background()
	data := []byte("itest-session-payload-" + time.Now().Format(time.RFC3339Nano))
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])

	// A 建会话但不完成
	initA, err := svc.Init(ctx, 3003, &InitReq{FileName: "itest-s1.mp4", FileSize: int64(len(data)), FileHash: hash})
	if err != nil {
		t.Fatalf("用户A init 失败: %v", err)
	}
	if initA.Fast {
		t.Fatal("首次上传不应命中秒传")
	}

	// B 上传同一文件：必须拿到属于自己的会话
	initB, err := svc.Init(ctx, 4004, &InitReq{FileName: "itest-s2.mp4", FileSize: int64(len(data)), FileHash: hash})
	if err != nil {
		t.Fatalf("用户B init 失败: %v", err)
	}
	if initB.UploadID == initA.UploadID {
		t.Fatalf("用户B 复用了用户A 的上传会话 %s（修复前行为）：B 传分片会撞 ErrForbidden", initB.UploadID)
	}

	// B 应能正常完成整个上传
	if _, err := putFile(t, svc, 4004, "itest-s2.mp4", data); err != nil {
		t.Fatalf("用户B 完整上传失败: %v", err)
	}
}

// 同一用户重复上传同一文件：应命中秒传并复用，不得新建记录（保持幂等语义）。
func TestInit_SameUserSameFile_HitsFastUpload(t *testing.T) {
	svc, _, cleanup := testEnv(t)
	defer cleanup()

	data := []byte("itest-same-user-payload-" + time.Now().Format(time.RFC3339Nano))

	res1, err := putFile(t, svc, 5005, "itest-u1.mp4", data)
	if err != nil {
		t.Fatalf("首次上传失败: %v", err)
	}
	res2, err := putFile(t, svc, 5005, "itest-u2.mp4", data)
	if err != nil {
		t.Fatalf("同用户重复上传失败: %v", err)
	}
	if res1.FileID != res2.FileID {
		t.Fatalf("同用户重复上传同一文件应秒传复用，实际得到不同 file_id: %s vs %s", res1.FileID, res2.FileID)
	}
}

func parseID(t *testing.T, s string) int64 {
	t.Helper()
	var id int64
	if _, err := fmt.Sscanf(s, "%d", &id); err != nil {
		t.Fatalf("file_id 不是合法数字: %q", s)
	}
	return id
}
