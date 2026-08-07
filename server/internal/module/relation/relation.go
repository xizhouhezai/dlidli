// Package relation 关系链域：关注/取关、关注列表、粉丝列表。
// 黑名单、互关标记、悄悄关注随 M2-FLW 后续任务补充。
package relation

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/dlidli/server/internal/module/account"
	"github.com/dlidli/server/internal/module/notify"
	"github.com/dlidli/server/internal/pkg/errcode"
	"gorm.io/gorm"
)

// Relation 对应 relation 表。
type Relation struct {
	ID        int64 `gorm:"primaryKey;autoIncrement"`
	UserID    int64
	TargetID  int64
	CreatedAt time.Time
}

func (Relation) TableName() string { return "relation" }

// Stat 关系状态聚合（播放页 UP 主卡片 / 空间页头部用）。
type Stat struct {
	Following    bool  `json:"following"`     // 当前登录用户是否已关注
	FollowerCnt  int64 `json:"follower_cnt"`  // 粉丝数
	FollowingCnt int64 `json:"following_cnt"` // 关注数
}

type Repo struct {
	db *gorm.DB
}

func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) Toggle(uid, target int64) (following bool, err error) {
	err = r.db.Transaction(func(tx *gorm.DB) error {
		res := tx.Where("user_id = ? AND target_id = ?", uid, target).Delete(&Relation{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected > 0 {
			following = false
			return nil
		}
		following = true
		return tx.Create(&Relation{UserID: uid, TargetID: target}).Error
	})
	return following, err
}

func (r *Repo) IsFollowing(uid, target int64) (bool, error) {
	var cnt int64
	err := r.db.Model(&Relation{}).
		Where("user_id = ? AND target_id = ?", uid, target).Count(&cnt).Error
	return cnt > 0, err
}

func (r *Repo) CountFollowers(target int64) (int64, error) {
	var cnt int64
	err := r.db.Model(&Relation{}).Where("target_id = ?", target).Count(&cnt).Error
	return cnt, err
}

func (r *Repo) CountFollowings(uid int64) (int64, error) {
	var cnt int64
	err := r.db.Model(&Relation{}).Where("user_id = ?", uid).Count(&cnt).Error
	return cnt, err
}

// ListFollowingIDs 关注的人（关注时间倒序）。
func (r *Repo) ListFollowingIDs(uid int64, page, size int) ([]int64, int64, error) {
	q := r.db.Model(&Relation{}).Where("user_id = ?", uid)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var ids []int64
	err := q.Order("created_at DESC").Offset((page-1)*size).Limit(size).
		Pluck("target_id", &ids).Error
	return ids, total, err
}

// AllFollowingIDs 全量关注 ID（Feed 拉模式用，上限 500）。
func (r *Repo) AllFollowingIDs(uid int64) ([]int64, error) {
	var ids []int64
	err := r.db.Model(&Relation{}).Where("user_id = ?", uid).Limit(500).
		Pluck("target_id", &ids).Error
	return ids, err
}

// ListFollowerIDs 粉丝（关注时间倒序）。
func (r *Repo) ListFollowerIDs(target int64, page, size int) ([]int64, int64, error) {
	q := r.db.Model(&Relation{}).Where("target_id = ?", target)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var ids []int64
	err := q.Order("created_at DESC").Offset((page-1)*size).Limit(size).
		Pluck("user_id", &ids).Error
	return ids, total, err
}

// UserBlock 对应 user_block 表（MSG-12 私信拉黑拦截）。
type UserBlock struct {
	ID         int64 `gorm:"primaryKey;autoIncrement"`
	UID        int64
	BlockedUID int64
	CreatedAt  time.Time
}

func (UserBlock) TableName() string { return "user_block" }

// ToggleBlock 拉黑/取消拉黑（幂等切换）。
func (r *Repo) ToggleBlock(uid, blocked int64) (blockedNow bool, err error) {
	err = r.db.Transaction(func(tx *gorm.DB) error {
		res := tx.Where("uid = ? AND blocked_uid = ?", uid, blocked).Delete(&UserBlock{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected > 0 {
			blockedNow = false
			return nil
		}
		blockedNow = true
		return tx.Create(&UserBlock{UID: uid, BlockedUID: blocked}).Error
	})
	return blockedNow, err
}

// IsBlocked uid 是否拉黑了 blocked。
func (r *Repo) IsBlocked(uid, blocked int64) (bool, error) {
	var cnt int64
	err := r.db.Model(&UserBlock{}).
		Where("uid = ? AND blocked_uid = ?", uid, blocked).Count(&cnt).Error
	return cnt > 0, err
}

type Service struct {
	repo       *Repo
	accountSvc *account.Service
	notifySvc  *notify.Service
}

func NewService(repo *Repo, accountSvc *account.Service, notifySvc *notify.Service) *Service {
	return &Service{repo: repo, accountSvc: accountSvc, notifySvc: notifySvc}
}

// Toggle 关注/取关。
// IsMutual 是否互关（私信发送限制用，PRD MSG-11）。
func (s *Service) IsMutual(ctx context.Context, a, b int64) (bool, error) {
	ab, err := s.repo.IsFollowing(a, b)
	if err != nil {
		return false, err
	}
	if !ab {
		return false, nil
	}
	return s.repo.IsFollowing(b, a)
}

// IsFollowing 是否已关注对方（私信提示语区分关注方向用）。
func (s *Service) IsFollowing(_ context.Context, a, b int64) (bool, error) {
	return s.repo.IsFollowing(a, b)
}

// ToggleBlock 拉黑/取消拉黑对方（MSG-12）。
func (s *Service) ToggleBlock(_ context.Context, uid, target int64) (bool, error) {
	if uid == target {
		return false, errcode.ErrInvalidParams.WithMsg("不能拉黑自己")
	}
	return s.repo.ToggleBlock(uid, target)
}

// BlockStatus 双向拉黑状态（私信会话页用）。
func (s *Service) BlockStatus(_ context.Context, uid, target int64) (iBlocked, blockedMe bool, err error) {
	if iBlocked, err = s.repo.IsBlocked(uid, target); err != nil {
		return false, false, err
	}
	blockedMe, err = s.repo.IsBlocked(target, uid)
	return iBlocked, blockedMe, err
}

// IsBlocked uid 是否拉黑了 blocked（私信发送拦截用）。
func (s *Service) IsBlocked(_ context.Context, uid, blocked int64) (bool, error) {
	return s.repo.IsBlocked(uid, blocked)
}

func (s *Service) Toggle(ctx context.Context, uid, target int64) (bool, error) {
	if uid == target {
		return false, errcode.ErrFollowSelf
	}
	// 目标用户必须存在
	briefs, err := s.accountSvc.Briefs(ctx, []int64{target})
	if err != nil {
		return false, err
	}
	if len(briefs) == 0 {
		return false, errcode.ErrNotFound.WithMsg("用户不存在")
	}

	following, err := s.repo.Toggle(uid, target)
	if err != nil && errors.Is(err, gorm.ErrDuplicatedKey) {
		return true, nil // 并发重复关注视为已关注
	}
	if err == nil && following {
		s.notifySvc.Push(target, uid, notify.TypeFollow, "关注了你", "/space/"+strconv.FormatInt(uid, 10))
	}
	return following, err
}

// Stat 关系状态聚合（viewer<=0 表示游客，仅返回计数）。
func (s *Service) Stat(_ context.Context, viewer, target int64) (*Stat, error) {
	st := &Stat{}
	var err error
	if st.FollowerCnt, err = s.repo.CountFollowers(target); err != nil {
		return nil, err
	}
	if st.FollowingCnt, err = s.repo.CountFollowings(target); err != nil {
		return nil, err
	}
	if viewer > 0 && viewer != target {
		if st.Following, err = s.repo.IsFollowing(viewer, target); err != nil {
			return nil, err
		}
	}
	return st, nil
}

// PublicProfile 公开用户资料（个人空间头部）。
func (s *Service) PublicProfile(ctx context.Context, target int64) (*account.Profile, error) {
	m, err := s.accountSvc.Briefs(ctx, []int64{target})
	if err != nil {
		return nil, err
	}
	p, ok := m[target]
	if !ok {
		return nil, errcode.ErrNotFound.WithMsg("用户不存在")
	}
	return &p, nil
}

// FollowingIDs 全量关注 ID（供 dynamic 等模块拉 Feed）。
func (s *Service) FollowingIDs(_ context.Context, uid int64) ([]int64, error) {
	return s.repo.AllFollowingIDs(uid)
}

// Followings 关注列表（用户卡片）。
func (s *Service) Followings(ctx context.Context, uid int64, page, size int) ([]account.Profile, int64, error) {
	ids, total, err := s.repo.ListFollowingIDs(uid, page, size)
	if err != nil {
		return nil, 0, err
	}
	briefs, err := s.briefsInOrder(ctx, ids)
	return briefs, total, err
}

// Followers 粉丝列表（用户卡片）。
func (s *Service) Followers(ctx context.Context, target int64, page, size int) ([]account.Profile, int64, error) {
	ids, total, err := s.repo.ListFollowerIDs(target, page, size)
	if err != nil {
		return nil, 0, err
	}
	briefs, err := s.briefsInOrder(ctx, ids)
	return briefs, total, err
}

func (s *Service) briefsInOrder(ctx context.Context, ids []int64) ([]account.Profile, error) {
	if len(ids) == 0 {
		return []account.Profile{}, nil
	}
	m, err := s.accountSvc.Briefs(ctx, ids)
	if err != nil {
		return nil, err
	}
	ordered := make([]account.Profile, 0, len(ids))
	for _, id := range ids {
		if b, ok := m[id]; ok {
			ordered = append(ordered, b)
		}
	}
	return ordered, nil
}
