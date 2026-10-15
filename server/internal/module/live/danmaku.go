package live

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dlidli/server/internal/module/account"
	"github.com/dlidli/server/internal/pkg/contentmod"
	"github.com/dlidli/server/internal/pkg/errcode"
	"github.com/dlidli/server/internal/pkg/snowflake"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// 直播弹幕频控（plan live §2B.3）：直播节奏快于视频稿件，
// 发送冷却 2s（视频 5s）；重复内容去重 10s（视频 30s）。
const (
	sendCooldown = 2 * time.Second
	dupCooldown  = 10 * time.Second
	badgeMax     = 30
)

// LiveDeps 弹幕通道的跨域只读依赖（router 装配注入，live 不反向依赖 account 内部 repo）。
type LiveDeps struct {
	Account *account.Service
	RDB     *redis.Client
	// Vip 会员标识提供者（M4-VIP-01 权益落地：直播弹幕带会员徽章）。
	// 同样用最小接口注入，live 不 import 会员域（plan §2F.6）。
	Vip VipBadgeProvider
}

// VipBadgeProvider 会员标识只读提供者（M4-VIP-01）。
type VipBadgeProvider interface {
	// VipColors 返回「生效中会员」的 uid → 昵称颜色（非会员不在返回值中）。
	VipColors(userIDs []int64) (map[int64]string, error)
}

// SendDanmaku 直播弹幕校验链（WS 上行走 HTTP 语义等价校验，服务端判定不靠前端）：
// 账号可发言（封禁/禁言）→ 房间直播中 → 发送者已进房 → 房间禁言执行（LIV-04 落地）
// → 内容机审 → 频控（Redis SETNX）→ 落库（历史窗口裁剪）→ 广播。
func (s *Service) SendDanmaku(ctx context.Context, uid, roomID int64, content string) error {
	content = strings.TrimSpace(content)
	if content == "" || len([]rune(content)) > maxDanmakuLen {
		return errcode.ErrInvalidParams.WithMsg("弹幕需 1-50 字")
	}
	room, err := s.activeRoom(roomID)
	if err != nil {
		return err
	}
	// 未进房拒绝（connSet 内存判定；hub 断开即离房，与连接集合强一致）
	if !s.hub.InRoom(roomID, uid) {
		return errcode.ErrLiveDanmakuNotIn
	}
	if s.deps == nil || s.deps.Account == nil {
		return errcode.ErrInternal
	}
	if err := s.deps.Account.EnsureCanPublish(ctx, uid); err != nil {
		return err
	}
	// 房间禁言执行（spec live §3 / §5C：被禁言观众发言在弹幕通道拒绝）
	muted, err := s.online.IsMuted(ctx, room.ID, uid)
	if err != nil {
		return err
	}
	if muted {
		return errcode.ErrLiveDanmakuMuted
	}
	// 机审：命中敏感词 → 影子屏蔽（仅落库不广播，对齐 danmaku 口径）
	shadow := !contentmod.CheckText(contentmod.SceneDanmaku, content).Pass

	// 频控：同房间同用户 2s 一条（Redis 不可用时放行——在线弹幕非资金数据）
	if s.deps.RDB != nil {
		cdKey := fmt.Sprintf("live:cd:%d:%d", roomID, uid)
		ok, err := s.deps.RDB.SetNX(ctx, cdKey, 1, sendCooldown).Result()
		if err != nil {
			s.log.Warn("直播弹幕频控 Redis 不可用，放行", zap.Error(err))
		} else if !ok {
			return errcode.ErrLiveDanmakuTooFast
		}
		dupKey := fmt.Sprintf("live:dup:%d:%d", roomID, uid)
		if ok, err := s.deps.RDB.SetNX(ctx, dupKey, 1, dupCooldown).Result(); err == nil && !ok {
			return errcode.ErrLiveDanmakuDuplicate
		}
	}

	profile, err := s.deps.Account.Me(ctx, uid)
	if err != nil {
		return err
	}
	// 会员标识（M4-VIP-01）：服务端判定后随帧下发；查询失败降级为无徽章（不阻断弹幕）。
	nickColor, isVip := s.vipColor(ctx, uid)
	msg := &LiveRoomMsg{
		ID: snowflake.NextID(), RoomID: roomID, UserID: uid,
		Nickname: profile.Nickname, Avatar: profile.Avatar, Content: content,
	}
	if err := s.repo.AddRoomMsg(msg); err != nil {
		return err
	}
	item := DanmakuInfo{
		ID: msg.ID, UserID: itoa(uid), Nickname: profile.Nickname,
		Avatar: profile.Avatar, Content: content,
		IsVip: isVip, NickColor: nickColor,
	}
	if !shadow {
		s.hub.BroadcastDanmaku(roomID, item)
	} else {
		s.log.Warn("直播弹幕机审命中，影子屏蔽", zap.Int64("room", roomID), zap.Int64("uid", uid))
	}
	return nil
}

// Join WS 进房：房间校验 → 计数（连接生命周期）→ 进场消息广播（含粉丝徽章）。
// 昵称/头像取自账号域（取不到降级空串，进场消息退化为纯 uid——spec 允许轻量信息尽力而为）。
func (s *Service) Join(ctx context.Context, uid, roomID int64) (*LiveRoom, string, string, EnterInfo, error) {
	room, err := s.activeRoom(roomID)
	if err != nil {
		return nil, "", "", EnterInfo{}, err
	}
	nickname, avatar := "", ""
	var badge int64
	if s.deps != nil && s.deps.Account != nil {
		if p, err := s.deps.Account.Me(ctx, uid); err == nil {
			nickname, avatar = p.Nickname, p.Avatar
		} else {
			s.log.Warn("进场消息资料获取失败，降级空昵称", zap.Int64("uid", uid), zap.Error(err))
		}
		if s.repo != nil {
			badge = s.repo.FanBadge(room.AnchorID, uid, badgeMax)
		}
	}
	info := EnterInfo{UID: itoa(uid), Nickname: nickname, Avatar: avatar, FanBadge: badge}
	return room, nickname, avatar, info, nil
}

// History 进房拉历史（最近 50 条 + 当前在线人数）。
func (s *Service) History(ctx context.Context, roomID int64) (*HistoryResult, error) {
	if _, err := s.activeRoom(roomID); err != nil {
		return nil, err
	}
	rows, err := s.repo.ListRoomMsg(roomID)
	if err != nil {
		return nil, err
	}
	n, err := s.online.Count(ctx, roomID)
	if err != nil {
		return nil, err
	}
	// 会员标识批量补齐（M4-VIP-01）：历史回放同样要带徽章，否则「刷出来的弹幕有徽章、
	// 回放的历史弹幕没有」会让用户以为徽章是随机出现的。一次查询覆盖整页。
	uids := make([]int64, 0, len(rows))
	for _, m := range rows {
		uids = append(uids, m.UserID)
	}
	vipColors := s.vipColors(ctx, uids)
	list := make([]DanmakuInfo, 0, len(rows))
	for _, m := range rows {
		color, isVip := vipColors[m.UserID]
		list = append(list, DanmakuInfo{
			ID: m.ID, UserID: itoa(m.UserID), Nickname: m.Nickname,
			Avatar: m.Avatar, Content: m.Content,
			IsVip: isVip, NickColor: color,
		})
	}
	return &HistoryResult{List: list, Online: n, RoomID: roomID}, nil
}

// vipColor 单人会员标识（查询失败降级为无徽章，不阻断弹幕发送）。
func (s *Service) vipColor(ctx context.Context, uid int64) (string, bool) {
	m := s.vipColors(ctx, []int64{uid})
	c, ok := m[uid]
	return c, ok
}

// vipColors 批量会员标识；未注入或查询失败一律返回空 map（降级为无徽章）。
func (s *Service) vipColors(ctx context.Context, uids []int64) map[int64]string {
	if s.deps == nil || s.deps.Vip == nil || len(uids) == 0 {
		return map[int64]string{}
	}
	m, err := s.deps.Vip.VipColors(uids)
	if err != nil {
		s.log.Warn("直播弹幕会员标识查询失败，降级为无徽章", zap.Error(err))
		return map[int64]string{}
	}
	return m
}
