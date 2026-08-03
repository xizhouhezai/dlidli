// 用户治理（ADM-03）：后台查询、封禁/禁言处罚、发言校验。
// 到期采用"懒解除"：登录/发言时检查到期时间，过期自动恢复正常，无需定时任务。
package account

import (
	"context"
	"time"

	"github.com/dlidli/server/internal/pkg/errcode"
)

// AdminUserItem 后台用户列表项。
type AdminUserItem struct {
	ID          int64      `json:"id,string"`
	Nickname    string     `json:"nickname"`
	Avatar      string     `json:"avatar"`
	Phone       string     `json:"phone"` // 绑定手机号（未绑定为空）
	Signature   string     `json:"signature"`
	Gender      int8       `json:"gender"` // 0 未知 1 男 2 女
	Level       int8       `json:"level"`
	Exp         int        `json:"exp"`
	Coin        int        `json:"coin"`
	Status      int8       `json:"status"`
	MutedUntil  *time.Time `json:"muted_until"`
	BannedUntil *time.Time `json:"banned_until"`
	CreatedAt   time.Time  `json:"created_at"`
}

// AdminListUsers 后台用户查询（UID/手机号/昵称，可按状态过滤）。
func (s *Service) AdminListUsers(_ context.Context, keyword string, status, page, size int) ([]AdminUserItem, int64, error) {
	users, total, err := s.repo.AdminSearchUsers(keyword, status, page, size)
	if err != nil {
		return nil, 0, err
	}
	ids := make([]int64, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.ID)
	}
	phones := s.repo.PhoneByUsers(ids)
	items := make([]AdminUserItem, 0, len(users))
	for _, u := range users {
		items = append(items, AdminUserItem{
			ID: u.ID, Nickname: u.Nickname, Avatar: u.Avatar, Phone: phones[u.ID],
			Signature: u.Signature, Gender: u.Gender, Level: u.Level, Exp: u.Exp, Coin: u.Coin,
			Status: u.Status, MutedUntil: u.MutedUntil, BannedUntil: u.BannedUntil,
			CreatedAt: u.CreatedAt,
		})
	}
	return items, total, nil
}

// PunishUser 处罚/解除：action = mute|unmute|ban|unban；days 处罚天数（ban 传 0 = 永久）。
func (s *Service) PunishUser(_ context.Context, uid int64, action string, days int) error {
	u, err := s.repo.FindUserByID(uid)
	if err != nil {
		return err
	}
	if u == nil {
		return errcode.ErrAccountNotExists
	}

	fields := map[string]any{}
	switch action {
	case "mute":
		if days <= 0 {
			return errcode.ErrInvalidParams.WithMsg("禁言必须指定天数")
		}
		until := time.Now().AddDate(0, 0, days)
		fields["status"] = UserStatusMuted
		fields["muted_until"] = &until
	case "unmute":
		fields["status"] = UserStatusNormal
		fields["muted_until"] = nil
	case "ban":
		fields["status"] = UserStatusBanned
		if days > 0 {
			until := time.Now().AddDate(0, 0, days)
			fields["banned_until"] = &until
		} else {
			fields["banned_until"] = nil // 永久
		}
	case "unban":
		fields["status"] = UserStatusNormal
		fields["banned_until"] = nil
	default:
		return errcode.ErrInvalidParams.WithMsg("未知处罚动作")
	}
	return s.repo.UpdateUserFields(uid, fields)
}

// EnsureCanPublish 发言前校验：封禁/禁言中的账号禁止发布内容（评论/弹幕/动态等）。
// 禁言到期时懒解除并放行。
func (s *Service) EnsureCanPublish(_ context.Context, uid int64) error {
	u, err := s.repo.FindUserByID(uid)
	if err != nil {
		return err
	}
	if u == nil {
		return errcode.ErrAccountNotExists
	}
	switch u.Status {
	case UserStatusMuted:
		if u.MutedUntil != nil && time.Now().After(*u.MutedUntil) {
			// 到期懒解除
			_ = s.repo.UpdateUserFields(uid, map[string]any{"status": UserStatusNormal, "muted_until": nil})
			return nil
		}
		return errcode.ErrUserMuted
	case UserStatusBanned:
		return errcode.ErrUserBanned
	}
	return nil
}

// resolveBanOnLogin 登录时封禁校验：到期自动解封放行，未到期拒绝。
func (s *Service) resolveBanOnLogin(u *User) error {
	if u.Status != UserStatusBanned {
		return nil
	}
	if u.BannedUntil != nil && time.Now().After(*u.BannedUntil) {
		if err := s.repo.UpdateUserFields(u.ID, map[string]any{"status": UserStatusNormal, "banned_until": nil}); err != nil {
			return err
		}
		u.Status = UserStatusNormal
		u.BannedUntil = nil
		return nil
	}
	return errcode.ErrUserBanned
}
