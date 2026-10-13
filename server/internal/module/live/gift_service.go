package live

import (
	"context"
	"fmt"

	"github.com/dlidli/server/internal/module/account"
	"github.com/dlidli/server/internal/pkg/errcode"
	"github.com/dlidli/server/internal/pkg/snowflake"
)

// GiftService 礼物域服务（M4-LIV-04）：金瓜子资产、礼物配置、赠送与分成、特效广播。
// 资产强一致在 giftRepo 事务层实现；广播在事务提交成功后进行。
type GiftService struct {
	repo *giftRepo
	hub  *Hub
	deps *LiveDeps // 跨域只读依赖（昵称尽力而为），router 装配注入
}

func NewGiftService(repo *giftRepo, hub *Hub) *GiftService {
	return &GiftService{repo: repo, hub: hub}
}

// SetLiveDeps 注入跨域依赖（router 装配）。
func (s *GiftService) SetLiveDeps(deps *LiveDeps) { s.deps = deps }

// profile 昵称/头像（账号域 Me；deps 未注入或失败返回错误，调用方降级）。
func (s *GiftService) profile(ctx context.Context, uid int64) (*account.Profile, error) {
	if s.deps == nil || s.deps.Account == nil {
		return nil, errcode.ErrLiveGiftNotFound
	}
	return s.deps.Account.Me(ctx, uid)
}

// GiftList 上架礼物列表（按单价正序）。
func (s *GiftService) GiftList() ([]GiftInfo, error) {
	var rows []Gift
	if err := s.repo.db.Where("status = 0").Order("price ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	list := make([]GiftInfo, 0, len(rows))
	for _, g := range rows {
		list = append(list, GiftInfo{
			ID: g.ID, Name: g.Name, Icon: g.Icon,
			Price: g.Price, Effect: g.Effect, Level: g.Level,
		})
	}
	return list, nil
}

// MelonBalance 查金瓜子余额（懒创建口径：无账户即 0）。
func (s *GiftService) MelonBalance(userID int64) (*MelonBalance, error) {
	b, err := s.repo.MelonBalance(userID)
	if err != nil {
		return nil, err
	}
	return &MelonBalance{Balance: b}, nil
}

// DevRecharge 开发直充（spec LIV-13 替换点：真实支付接入留 PAY-01，
// 届时以支付回调复用 RechargeTx 的 biz_no 幂等，本接口下线）。挂 admin 权限中间件。
func (s *GiftService) DevRecharge(userID, delta int64, operator int64) (*RechargeResult, error) {
	if delta <= 0 || delta > 1_000_000_000 {
		return nil, errcode.ErrLiveRechargeInvalid
	}
	bizNo := fmt.Sprintf("dev-%d-%d", snowflake.NextID(), userID)
	balance, _, err := s.repo.RechargeTx(userID, delta, bizNo, fmt.Sprintf("dev 直充（操作人 admin:%d）", operator))
	if err != nil {
		return nil, err
	}
	return &RechargeResult{UserID: userID, Balance: balance, Delta: delta}, nil
}

// SendGift 赠送礼物全链路：
// 校验（礼物上架/房间直播中/数量）→ 事务扣款入账 → WS 房间广播特效帧（type=gift）。
// 事务提交成功才广播：扣款失败（余额不足）不产生任何流水、不广播。
func (s *GiftService) SendGift(ctx context.Context, senderID, roomID, giftID, count int64) (*SendGiftResult, error) {
	if count < 1 || count > 999 {
		return nil, errcode.ErrLiveGiftCount
	}
	gift, err := s.giftByID(giftID)
	if err != nil {
		return nil, err
	}
	room, err := s.roomByID(roomID)
	if err != nil {
		return nil, err
	}
	if room.Status != RoomStatusLive {
		return nil, errcode.ErrLiveGiftNotLive
	}
	logID, _, err := s.repo.SendGiftTx(senderID, room, gift, count)
	if err != nil {
		return nil, err
	}
	balance, err := s.repo.MelonBalance(senderID)
	if err != nil {
		return nil, err
	}
	frame := &GiftFrame{
		LogID: logID, SenderID: itoa(senderID),
		GiftID: gift.ID, GiftName: gift.Name, Icon: gift.Icon,
		Count: count, Effect: gift.Effect, Level: gift.Level,
	}
	// 昵称尽力而为（取不到退化为 uid 展示）
	if p, err := s.profile(ctx, senderID); err == nil {
		frame.Nickname = p.Nickname
	} else {
		frame.Nickname = itoa(senderID)
	}
	if s.hub != nil {
		s.hub.BroadcastGift(roomID, *frame)
	}
	return &SendGiftResult{Balance: balance, GiftLog: frame}, nil
}

// AnchorShareSum 主播分成累计（验收/结算核对）。
func (s *GiftService) AnchorShareSum(anchorID int64) (int64, error) {
	return s.repo.AnchorShareSum(anchorID)
}

// giftByID 上架礼物；下架或不存在报 91019。
func (s *GiftService) giftByID(id int64) (*Gift, error) {
	var g Gift
	err := s.repo.db.Where("id = ? AND status = 0", id).First(&g).Error
	if err != nil {
		return nil, errcode.ErrLiveGiftNotFound
	}
	return &g, nil
}

// roomByID 房间存在性校验（复用主 Repo 语义：不存在报 91007）。
func (s *GiftService) roomByID(id int64) (*LiveRoom, error) {
	var room LiveRoom
	if err := s.repo.db.First(&room, id).Error; err != nil {
		return nil, errcode.ErrLiveRoomNotFound
	}
	return &room, nil
}
