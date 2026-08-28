// 用户治理（ADM-03）：后台查询、封禁/禁言处罚、发言校验。
// 到期采用"懒解除"：登录/发言时检查到期时间，过期自动恢复正常，无需定时任务。
package account

import (
	"context"
	"time"

	"github.com/dlidli/server/internal/pkg/encrypt"
	"github.com/dlidli/server/internal/pkg/errcode"
	"go.uber.org/zap"
)

// isPhonePlain 判断是否为历史明文手机号（11 位数字，1 开头；区别于 base64 密文）。
func isPhonePlain(s string) bool {
	if len(s) != 11 || s[0] != '1' {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

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
	var phoneHash string
	if isPhonePlain(keyword) {
		phoneHash = encrypt.IdentifierHash(int8(IdentityPhone), keyword)
	}
	users, total, err := s.repo.AdminSearchUsers(keyword, phoneHash, status, page, size)
	if err != nil {
		return nil, 0, err
	}
	ids := make([]int64, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.ID)
	}
	phones := s.repo.PhoneByUsers(ids) // 密文（ACC-43）
	items := make([]AdminUserItem, 0, len(users))
	for _, u := range users {
		items = append(items, AdminUserItem{
			ID: u.ID, Nickname: u.Nickname, Avatar: u.Avatar,
			Phone:     s.decryptPhone(phones[u.ID]),
			Signature: u.Signature, Gender: u.Gender, Level: u.Level, Exp: u.Exp, Coin: u.Coin,
			Status: u.Status, MutedUntil: u.MutedUntil, BannedUntil: u.BannedUntil,
			CreatedAt: u.CreatedAt,
		})
	}
	return items, total, nil
}

// decryptPhone 解密手机号密文；空值/历史明文/解密失败均返回空串（后台展示兜底，不阻塞列举）。
func (s *Service) decryptPhone(ciphertext string) string {
	if ciphertext == "" {
		return ""
	}
	// 兼容未迁移的明文历史数据：无前缀且非 base64 密文特征时原样返回
	out, err := encrypt.Decrypt(s.phKey(), ciphertext)
	if err != nil {
		// 历史明文：11 位纯数字，直接展示
		if isPhonePlain(ciphertext) {
			return ciphertext
		}
		s.log.Warn("后台手机号解密失败", zap.Int("len", len(ciphertext)), zap.Error(err))
		return ""
	}
	return out
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
