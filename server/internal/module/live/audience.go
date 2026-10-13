package live

import (
	"context"
	"time"

	"github.com/dlidli/server/internal/module/account"
	"github.com/dlidli/server/internal/pkg/errcode"
)

// ListLive 直播中房间列表（公开侧只看 status=1）。
func (s *Service) ListLive(ctx context.Context, categoryID int64, page, size int) ([]*RoomInfo, int64, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 || size > 50 {
		size = 20
	}
	var (
		list  []LiveRoom
		total int64
		err   error
	)
	if categoryID > 0 {
		list, total, err = s.repo.ListLiveByCategory(categoryID, page, size)
	} else {
		list, total, err = s.repo.ListLive(page, size)
	}
	if err != nil {
		return nil, 0, err
	}
	out := make([]*RoomInfo, 0, len(list))
	for i := range list {
		info, err := s.roomInfo(ctx, &list[i])
		if err != nil {
			return nil, 0, err
		}
		out = append(out, info)
	}
	return out, total, nil
}

// RoomDetail 房间详情（optionalAuth：游客可看；仅 status=1 进公开可见，0/2 仅主播本人可见）。
func (s *Service) RoomDetail(ctx context.Context, viewer, roomID int64) (*RoomInfo, error) {
	room, err := s.repo.RoomByID(roomID)
	if err != nil {
		return nil, err
	}
	if room == nil {
		return nil, errcode.ErrLiveRoomNotFound
	}
	if room.Status != RoomStatusLive && room.AnchorID != viewer {
		return nil, errcode.ErrLiveRoomNotFound // 未开播房间对外不可见
	}
	return s.roomInfo(ctx, room)
}

// Enter 进房（登录态；计数 +1，重复进房不重复计数）。
func (s *Service) Enter(ctx context.Context, uid, roomID int64) (*RoomInfo, error) {
	room, err := s.activeRoom(roomID)
	if err != nil {
		return nil, err
	}
	if err := s.online.Enter(ctx, room.ID, uid); err != nil {
		return nil, err
	}
	return s.roomInfo(ctx, room)
}

// Heartbeat 心跳（刷新 TTL ≤15s 一次；30s 无心跳判离线）。
func (s *Service) Heartbeat(ctx context.Context, uid, roomID int64) error {
	_, err := s.activeRoom(roomID)
	if err != nil {
		return err
	}
	return s.online.Heartbeat(ctx, roomID, uid)
}

// Leave 显式离房（计数 −1，幂等）。
func (s *Service) Leave(ctx context.Context, uid, roomID int64) (*RoomInfo, error) {
	room, err := s.repo.RoomByID(roomID)
	if err != nil {
		return nil, err
	}
	if room == nil {
		return nil, errcode.ErrLiveRoomNotFound
	}
	if err := s.online.Leave(ctx, room.ID, uid); err != nil {
		return nil, err
	}
	return s.roomInfo(ctx, room)
}

// activeRoom 直播中的房间（观众侧进房/心跳仅限 status=1）。
func (s *Service) activeRoom(roomID int64) (*LiveRoom, error) {
	room, err := s.repo.RoomByID(roomID)
	if err != nil {
		return nil, err
	}
	if room == nil {
		return nil, errcode.ErrLiveRoomNotFound
	}
	if room.Status != RoomStatusLive {
		return nil, errcode.ErrLiveNotLive
	}
	return room, nil
}

// ---- 房管（LIV-04）----

// AddAdmin 任命房管（仅主播；不能任命自己，不能任命已开黑名单外的人——本卡仅要求用户存在）。
func (s *Service) AddAdmin(ctx context.Context, anchor, roomID, target int64) error {
	room, err := s.mustOwnedRoom(anchor)
	if err != nil {
		return err
	}
	if room.ID != roomID {
		return errcode.ErrLiveRoomNotFound
	}
	if anchor == target {
		return errcode.ErrLiveAdminSelf
	}
	ok, err := s.repo.UserExists(target)
	if err != nil {
		return err
	}
	if !ok {
		return errcode.ErrNotFound.WithMsg("用户不存在")
	}
	_, err = s.repo.AddAdmin(roomID, target, anchor)
	return err
}

// RemoveAdmin 撤销房管（仅主播）。
func (s *Service) RemoveAdmin(ctx context.Context, anchor, roomID, target int64) error {
	room, err := s.mustOwnedRoom(anchor)
	if err != nil {
		return err
	}
	if room.ID != roomID {
		return errcode.ErrLiveRoomNotFound
	}
	_, err = s.repo.RemoveAdmin(roomID, target)
	return err
}

// ---- 房管/禁言列表（LIV-06 开播管理前端读端点；观众侧亦可看房管与禁言名单）----

// AdminBrief 房管条目（uid 字符串避免 JS 精度丢失 + 账号域简况）。
type AdminBrief struct {
	UserID     string `json:"user_id"`
	Nickname   string `json:"nickname"`
	Avatar     string `json:"avatar,omitempty"`
	AppointedAt time.Time `json:"appointed_at"`
}

// AdminListResult 房管列表。
type AdminListResult struct {
	List []AdminBrief `json:"list"`
}

// MuteBrief 禁言条目。
type MuteBrief struct {
	UserID     string     `json:"user_id"`
	Nickname   string     `json:"nickname"`
	Avatar     string     `json:"avatar,omitempty"`
	Reason     string     `json:"reason"`
	MutedUntil time.Time  `json:"muted_until"`
	Muted      bool       `json:"muted"` // muted_until 是否仍未到期
}

// MuteListResult 禁言列表。
type MuteListResult struct {
	List []MuteBrief `json:"list"`
}

// ListAdmins 房管列表（房间必须存在；登录即可看）。
func (s *Service) ListAdmins(ctx context.Context, roomID int64) (*AdminListResult, error) {
	room, err := s.repo.RoomByID(roomID)
	if err != nil {
		return nil, err
	}
	if room == nil {
		return nil, errcode.ErrLiveRoomNotFound
	}
	rows, err := s.repo.ListAdmins(roomID)
	if err != nil {
		return nil, err
	}
	return &AdminListResult{List: s.adminBriefs(ctx, rows)}, nil
}

// adminBriefs 拼装房管简况（账号域取不到时昵称降级空串）。
func (s *Service) adminBriefs(ctx context.Context, rows []LiveRoomAdmin) []AdminBrief {
	out := make([]AdminBrief, 0, len(rows))
	profiles := map[int64]account.Profile{}
	if s.deps != nil && s.deps.Account != nil && len(rows) > 0 {
		ids := make([]int64, 0, len(rows))
		for _, r := range rows {
			ids = append(ids, r.UserID)
		}
		if m, err := s.deps.Account.Briefs(ctx, ids); err == nil {
			profiles = m
		}
	}
	for _, r := range rows {
		b := AdminBrief{UserID: itoa(r.UserID), AppointedAt: r.CreatedAt}
		if p, ok := profiles[r.UserID]; ok {
			b.Nickname, b.Avatar = p.Nickname, p.Avatar
		}
		out = append(out, b)
	}
	return out
}

// ListMutes 禁言列表（房间必须存在；登录即可看）。
func (s *Service) ListMutes(ctx context.Context, roomID int64) (*MuteListResult, error) {
	room, err := s.repo.RoomByID(roomID)
	if err != nil {
		return nil, err
	}
	if room == nil {
		return nil, errcode.ErrLiveRoomNotFound
	}
	rows, err := s.repo.ListMutes(roomID)
	if err != nil {
		return nil, err
	}
	out := make([]MuteBrief, 0, len(rows))
	profiles := map[int64]account.Profile{}
	if s.deps != nil && s.deps.Account != nil && len(rows) > 0 {
		ids := make([]int64, 0, len(rows))
		for _, r := range rows {
			ids = append(ids, r.UserID)
		}
		if m, err := s.deps.Account.Briefs(ctx, ids); err == nil {
			profiles = m
		}
	}
	now := time.Now()
	for _, r := range rows {
		b := MuteBrief{
			UserID: itoa(r.UserID), Reason: r.Reason,
			MutedUntil: r.MutedUntil, Muted: r.MutedUntil.After(now),
		}
		if p, ok := profiles[r.UserID]; ok {
			b.Nickname, b.Avatar = p.Nickname, p.Avatar
		}
		out = append(out, b)
	}
	return &MuteListResult{List: out}, nil
}

// ---- 禁言（LIV-04 数据与接口；执行在 LIV-03）----

// Mute 禁言（主播或房管；时长 minutes 1min-720h；不能禁言主播）。
func (s *Service) Mute(ctx context.Context, operator, roomID, target int64, minutes int64, reason string) error {
	room, err := s.repo.RoomByID(roomID)
	if err != nil {
		return err
	}
	if room == nil {
		return errcode.ErrLiveRoomNotFound
	}
	if !s.canModerate(room, operator) {
		return errcode.ErrLiveNotAdmin
	}
	if minutes <= 0 {
		return errcode.ErrLiveMuteRange
	}
	dur := time.Duration(minutes) * time.Minute
	if dur < muteMin || dur > muteMax {
		return errcode.ErrLiveMuteRange
	}
	if target == room.AnchorID {
		return errcode.ErrLiveAdminSelf // 不能禁言主播
	}
	ok, err := s.repo.UserExists(target)
	if err != nil {
		return err
	}
	if !ok {
		return errcode.ErrNotFound.WithMsg("用户不存在")
	}
	return s.repo.UpsertMute(&LiveRoomMute{
		RoomID: roomID, UserID: target,
		MutedUntil: time.Now().Add(dur), Reason: reason, CreatedBy: operator,
	})
}

// Unmute 解禁（主播或房管；不能解主播——主播本就不可被禁言）。
func (s *Service) Unmute(ctx context.Context, operator, roomID, target int64) error {
	room, err := s.repo.RoomByID(roomID)
	if err != nil {
		return err
	}
	if room == nil {
		return errcode.ErrLiveRoomNotFound
	}
	if !s.canModerate(room, operator) {
		return errcode.ErrLiveNotAdmin
	}
	if target == room.AnchorID {
		return errcode.ErrLiveAdminSelf // 主播本就不可被禁言
	}
	_, err = s.repo.Unmute(roomID, target)
	return err
}

// canModerate 主播或该房间房管。
func (s *Service) canModerate(room *LiveRoom, uid int64) bool {
	if uid == room.AnchorID {
		return true
	}
	ok, err := s.repo.IsAdmin(room.ID, uid)
	return err == nil && ok
}
