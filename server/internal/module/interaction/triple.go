package interaction

import (
	"context"
	"errors"

	"github.com/dlidli/server/internal/module/video"
	"github.com/dlidli/server/internal/pkg/errcode"
	"github.com/dlidli/server/internal/pkg/snowflake"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// State 播放页互动状态聚合。
type State struct {
	Liked     bool `json:"liked"`
	CoinCount int  `json:"coin_count"` // 已投币数（0=未投）
	Faved     bool `json:"faved"`
}

// TripleResult 一键三连结果（Delta 供前端本地修正计数）。
type TripleResult struct {
	State
	LikeDelta int `json:"like_delta"`
	CoinDelta int `json:"coin_delta"`
	FavDelta  int `json:"fav_delta"`
}

// CoinVideo 投币：自制 ≤2 枚 / 转载 1 枚，不可重复投、不可投自己。
func (s *Service) CoinVideo(ctx context.Context, uid int64, bv string, count int) error {
	videoID, ownerID, copyright, err := s.videoSvc.PublishedMetaByBvid(ctx, bv)
	if err != nil {
		return err
	}
	if uid == ownerID {
		return errcode.ErrCoinSelf
	}
	limit := 1
	if copyright == 1 {
		limit = 2
	}
	if count < 1 || count > limit {
		return errcode.ErrCoinCount
	}
	if exist, err := s.repo.GetAction(uid, videoID, ObjVideo, ActionCoin); err != nil {
		return err
	} else if exist != nil {
		return errcode.ErrAlreadyCoined
	}

	// 先扣币（account 内事务保证余额一致），后续失败则退款补偿
	if err := s.accountSvc.SpendCoins(ctx, uid, count, "coin_video"); err != nil {
		return err
	}
	if err := s.repo.CreateAction(&UserAction{
		UserID: uid, Oid: videoID, ObjType: ObjVideo, Action: ActionCoin, Extra: count,
	}); err != nil {
		if refundErr := s.accountSvc.GrantCoins(ctx, uid, count, "coin_refund"); refundErr != nil {
			s.log.Error("投币退款失败", zap.Int64("uid", uid), zap.Error(refundErr))
		}
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return errcode.ErrAlreadyCoined
		}
		return err
	}

	if err := s.videoSvc.AddStat(ctx, videoID, "coin_cnt", count); err != nil {
		s.log.Warn("投币计数回写失败", zap.Error(err))
	}
	return nil
}

// ToggleFavorite 收藏开关（指定收藏夹；MVP 默认收藏夹）。
func (s *Service) ToggleFavorite(ctx context.Context, uid int64, bv string, collectionID int64) (bool, error) {
	videoID, err := s.videoSvc.PublishedIDByBvid(ctx, bv)
	if err != nil {
		return false, err
	}
	// 确保默认收藏夹存在
	if collectionID == 0 {
		collectionID, err = s.repo.EnsureDefaultCollection(uid)
		if err != nil {
			return false, err
		}
	} else {
		c, err := s.repo.FindCollection(uid, collectionID)
		if err != nil {
			return false, err
		}
		if c == nil {
			return false, errcode.ErrNotFound.WithMsg("收藏夹不存在")
		}
	}

	// 检查是否已收藏（任意夹）
	exist, _ := s.repo.HasAction(uid, videoID, ObjVideo, ActionFav)
	if exist {
		// 已收藏 → 取消
		_, err = s.repo.ToggleAction(uid, videoID, ObjVideo, ActionFav)
		if err != nil {
			return false, err
		}
		_ = s.videoSvc.AddStat(ctx, videoID, "fav_cnt", -1)
		return false, nil
	}
	// 未收藏 → 新增（落入指定收藏夹）
	if err := s.repo.CreateAction(&UserAction{
		UserID: uid, Oid: videoID, ObjType: ObjVideo, Action: ActionFav, CollectionID: collectionID,
	}); err != nil {
		return false, err
	}
	_ = s.videoSvc.AddStat(ctx, videoID, "fav_cnt", 1)
	return true, nil
}

// InteractionState 聚合当前用户对稿件的互动状态（游客全为零值）。
func (s *Service) InteractionState(ctx context.Context, uid int64, bv string) (*State, error) {
	st := &State{}
	if uid <= 0 {
		return st, nil
	}
	videoID, err := s.videoSvc.PublishedIDByBvid(ctx, bv)
	if err != nil {
		return nil, err
	}

	if liked, err := s.repo.HasAction(uid, videoID, ObjVideo, ActionLike); err == nil {
		st.Liked = liked
	}
	if coin, err := s.repo.GetAction(uid, videoID, ObjVideo, ActionCoin); err == nil && coin != nil {
		st.CoinCount = coin.Extra
	}
	if faved, err := s.repo.HasAction(uid, videoID, ObjVideo, ActionFav); err == nil {
		st.Faved = faved
	}
	return st, nil
}

// Triple 一键三连：点赞 + 投 1 枚币 + 收藏（已完成项跳过；投币失败不阻塞其余动作）。
func (s *Service) Triple(ctx context.Context, uid int64, bv string) (*TripleResult, error) {
	videoID, err := s.videoSvc.PublishedIDByBvid(ctx, bv)
	if err != nil {
		return nil, err
	}
	res := &TripleResult{}

	// 点赞（未赞则赞）
	if liked, _ := s.repo.HasAction(uid, videoID, ObjVideo, ActionLike); !liked {
		if active, err := s.repo.ToggleAction(uid, videoID, ObjVideo, ActionLike); err == nil && active {
			_ = s.videoSvc.AddStat(ctx, videoID, "like_cnt", 1)
			res.LikeDelta = 1
		}
	}
	res.Liked = true

	// 投币 1 枚（自己的稿件/已投/余额不足则静默跳过）
	if err := s.CoinVideo(ctx, uid, bv, 1); err == nil {
		res.CoinDelta = 1
	}
	if coin, _ := s.repo.GetAction(uid, videoID, ObjVideo, ActionCoin); coin != nil {
		res.CoinCount = coin.Extra
	}

	// 收藏（未藏则藏）
	if faved, _ := s.repo.HasAction(uid, videoID, ObjVideo, ActionFav); !faved {
		if active, err := s.repo.ToggleAction(uid, videoID, ObjVideo, ActionFav); err == nil && active {
			_ = s.videoSvc.AddStat(ctx, videoID, "fav_cnt", 1)
			res.FavDelta = 1
		}
	}
	res.Faved = true

	return res, nil
}

// Favorites 我的收藏列表（默认收藏夹）。
func (s *Service) Favorites(ctx context.Context, uid int64, page, size int) ([]video.Card, int64, error) {
	ids, total, err := s.repo.ListFavOids(uid, page, size)
	if err != nil {
		return nil, 0, err
	}
	cards, err := s.videoSvc.CardsByIDs(ctx, ids)
	if err != nil {
		return nil, 0, err
	}
	return cards, total, nil
}

// ---- 收藏夹 CRUD ----

func (s *Service) ListCollections(_ context.Context, uid int64) ([]Collection, error) {
	return s.repo.ListCollections(uid)
}

func (s *Service) CreateCollection(_ context.Context, uid int64, name string) (*Collection, error) {
	if name == "" || len([]rune(name)) > 50 {
		return nil, errcode.ErrInvalidParams.WithMsg("收藏夹名称 1~50 字")
	}
	c := &Collection{ID: snowflake.NextID(), UserID: uid, Name: name}
	if err := s.repo.CreateCollection(c); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *Service) RenameCollection(_ context.Context, uid, id int64, name string) error {
	if name == "" || len([]rune(name)) > 50 {
		return errcode.ErrInvalidParams.WithMsg("收藏夹名称 1~50 字")
	}
	if err := s.repo.RenameCollection(uid, id, name); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.ErrNotFound.WithMsg("收藏夹不存在或为默认夹")
		}
		return err
	}
	return nil
}

func (s *Service) DeleteCollection(_ context.Context, uid, id int64) error {
	if err := s.repo.DeleteCollection(uid, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.ErrNotFound.WithMsg("收藏夹不存在或为默认夹")
		}
		return err
	}
	return nil
}
