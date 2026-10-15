package interaction

import (
	"context"
	"os"
	"testing"

	"github.com/dlidli/server/internal/module/account"
	"github.com/dlidli/server/internal/pkg/config"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestToItemsVipBadge 会员身份标识（M4-VIP-01 权益落地点之一）：评论读模型
// `UserBrief.is_vip` / `nickname_color` 由注入的 VipBadgeProvider 决定，免费用户不带。
//
// 用 fake provider 固定「读模型是否消费会员标识」这一行为；会员判定本身由 vip 包覆盖。
type fakeVipColors struct {
	colors map[int64]string
	fail   bool
}

func (f *fakeVipColors) VipColors([]int64) (map[int64]string, error) {
	if f.fail {
		return nil, vipProbeErr{}
	}
	return f.colors, nil
}

type vipProbeErr struct{}

func (vipProbeErr) Error() string { return "vip provider 故障" }

func interactionTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("DLIDLI_TEST_DSN")
	if dsn == "" {
		t.Skip("DLIDLI_TEST_DSN 未设置，跳过 interaction 集成测试")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), TranslateError: true,
	})
	if err != nil {
		t.Fatalf("连接测试库失败: %v", err)
	}
	return db
}

const (
	badgeUIDBase = int64(92400000)
	badgeUIDEnd  = int64(92499999)
)

func TestToItemsVipBadge(t *testing.T) {
	db := interactionTestDB(t)
	clean := func() {
		db.Exec("DELETE FROM comment WHERE user_id BETWEEN ? AND ?", badgeUIDBase, badgeUIDEnd)
		db.Exec("DELETE FROM `user` WHERE id BETWEEN ? AND ?", badgeUIDBase, badgeUIDEnd)
	}
	clean()
	t.Cleanup(clean)

	const vipUID, freeUID = 92400001, 92400002
	for _, u := range []int64{vipUID, freeUID} {
		if err := db.Exec("INSERT INTO `user` (id, nickname, status, created_at, updated_at) VALUES (?, ?, 0, NOW(), NOW())",
			u, "badge-"+string(rune('a'+u%26))).Error; err != nil {
			t.Fatalf("造用户失败: %v", err)
		}
	}
	// 同一视频下两条评论：一条会员、一条免费（对照）
	rows := []Comment{
		{ID: 924000001, Oid: 999001, ObjType: ObjVideo, UserID: vipUID, Content: "会员评论"},
		{ID: 924000002, Oid: 999001, ObjType: ObjVideo, UserID: freeUID, Content: "免费评论"},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("造评论失败: %v", err)
	}

	svc := &Service{
		repo:       NewRepo(db),
		accountSvc: account.NewService(account.NewRepo(db), nil, &config.Config{}, zap.NewNop(), nil),
		log:        zap.NewNop(),
		vipBadge:   &fakeVipColors{colors: map[int64]string{vipUID: "pink"}},
	}
	items, err := svc.toItems(context.Background(), rows, freeUID)
	if err != nil {
		t.Fatalf("toItems 失败: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("应返回 2 条, got %d", len(items))
	}
	byContent := map[string]CommentItem{}
	for _, it := range items {
		byContent[it.Content] = it
	}
	vip := byContent["会员评论"]
	if !vip.User.IsVip || vip.User.NickColor != "pink" {
		t.Fatalf("会员评论应带徽章与粉色昵称, got %+v", vip.User)
	}
	free := byContent["免费评论"]
	if free.User.IsVip || free.User.NickColor != "" {
		t.Fatalf("免费用户评论不应带徽章, got %+v", free.User)
	}
	if !free.IsSelf {
		t.Fatalf("viewerUID 为该评论作者时应标记 is_self")
	}

	// 会员标识查询失败 → 降级为无徽章，但**评论列表必须仍能返回**（展示增强不阻断读）
	svc.vipBadge = &fakeVipColors{fail: true}
	items2, err := svc.toItems(context.Background(), rows, 0)
	if err != nil {
		t.Fatalf("会员标识故障时评论列表仍应可返回: %v", err)
	}
	for _, it := range items2 {
		if it.User.IsVip {
			t.Fatalf("降级时不应出现徽章, got %+v", it.User)
		}
	}
}
