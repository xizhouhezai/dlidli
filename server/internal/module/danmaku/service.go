package danmaku

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dlidli/server/internal/module/account"
	"github.com/dlidli/server/internal/module/growth"
	"github.com/dlidli/server/internal/module/video"
	"github.com/dlidli/server/internal/pkg/errcode"
	"github.com/dlidli/server/internal/pkg/moderate"
	"github.com/dlidli/server/internal/pkg/snowflake"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	sendCooldown = 5 * time.Second
	segCacheTTL  = 10 * time.Minute
)

type Service struct {
	repo       *Repo
	videoSvc   *video.Service
	accountSvc *account.Service
	growthSvc  *growth.Service
	rdb        *redis.Client
	log        *zap.Logger
}

func NewService(repo *Repo, videoSvc *video.Service, accountSvc *account.Service, growthSvc *growth.Service, rdb *redis.Client, log *zap.Logger) *Service {
	return &Service{repo: repo, videoSvc: videoSvc, accountSvc: accountSvc, growthSvc: growthSvc, rdb: rdb, log: log}
}

func segKey(videoID int64, seg int) string {
	return fmt.Sprintf("dm:v:%d:%d", videoID, seg)
}

// Send 发送弹幕：禁言/封禁拦截 → 等级/频控校验 → 敏感词影子屏蔽 → 落库 + 计数 + 段缓存失效。
func (s *Service) Send(ctx context.Context, uid int64, bv string, req *SendReq) (*Item, error) {
	if err := s.accountSvc.EnsureCanPublish(ctx, uid); err != nil {
		return nil, err
	}
	videoID, err := s.videoSvc.PublishedIDByBvid(ctx, bv)
	if err != nil {
		return nil, err
	}

	// Lv1 以上可发（手机注册即 Lv1）
	profile, err := s.accountSvc.Me(ctx, uid)
	if err != nil {
		return nil, err
	}
	if profile.Level < 1 {
		return nil, errcode.ErrDanmakuLevelLow
	}
	// 彩色/顶部/底部弹幕需 Lv3（M2-GRW-02）
	if profile.Level < 3 && (req.Mode == 2 || req.Mode == 3 || (req.Color != 0 && req.Color != 0xFFFFFF)) {
		return nil, errcode.ErrDanmakuPrivilege
	}

	// 频控：同用户同视频 5s 1 条
	cdKey := fmt.Sprintf("dm:cd:%d:%d", videoID, uid)
	ok, err := s.rdb.SetNX(ctx, cdKey, 1, sendCooldown).Result()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errcode.ErrDanmakuTooFrequent
	}

	d := &Danmaku{
		ID:       snowflake.NextID(),
		VideoID:  videoID,
		UserID:   uid,
		Content:  strings.TrimSpace(req.Content),
		TimeMs:   req.TimeMs,
		Mode:     req.Mode,
		Color:    req.Color,
		FontSize: req.FontSize,
	}
	if d.Content == "" {
		return nil, errcode.ErrInvalidParams
	}
	if d.Mode == 0 {
		d.Mode = 1
	}
	if d.Color == 0 {
		d.Color = 0xFFFFFF
	}
	if d.FontSize == 0 {
		d.FontSize = 25
	}

	// 命中敏感词 → 影子屏蔽（仅发送者本地可见，不报错）
	if moderate.Hit(d.Content) {
		d.Status = StatusShadow
	}

	if err := s.repo.Create(d); err != nil {
		return nil, err
	}

	if d.Status == StatusNormal {
		if err := s.videoSvc.IncDanmakuCount(ctx, videoID); err != nil {
			s.log.Warn("弹幕计数回写失败", zap.Error(err))
		}
		// 失效对应段缓存，下次拉取回源
		s.rdb.Del(ctx, segKey(videoID, d.TimeMs/SegmentMS))
		// 发送弹幕 +1 经验（每日上限 20 次，M2-GRW-01）
		if s.growthSvc != nil {
			s.growthSvc.AddExpWithLimit(ctx, uid, growth.ReasonDanmakuSend)
		}
	}

	return &Item{
		ID: fmt.Sprintf("%d", d.ID), Content: d.Content, TimeMs: d.TimeMs,
		Mode: d.Mode, Color: d.Color, FontSize: d.FontSize, IsSelf: true,
	}, nil
}

// ListSegment 分段拉取（Redis 缓存优先，miss 回源 MySQL 并回填）。
func (s *Service) ListSegment(ctx context.Context, bv string, seg int) ([]Item, error) {
	if seg < 0 {
		return nil, errcode.ErrInvalidParams
	}
	videoID, err := s.videoSvc.PublishedIDByBvid(ctx, bv)
	if err != nil {
		return nil, err
	}

	key := segKey(videoID, seg)
	if cached, err := s.rdb.Get(ctx, key).Result(); err == nil {
		var items []Item
		if json.Unmarshal([]byte(cached), &items) == nil {
			return items, nil
		}
	}

	list, err := s.repo.ListSegment(videoID, seg*SegmentMS, (seg+1)*SegmentMS)
	if err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(list))
	for _, d := range list {
		items = append(items, Item{
			ID: fmt.Sprintf("%d", d.ID), Content: d.Content, TimeMs: d.TimeMs,
			Mode: d.Mode, Color: d.Color, FontSize: d.FontSize,
		})
	}

	if buf, err := json.Marshal(items); err == nil {
		s.rdb.Set(ctx, key, buf, segCacheTTL)
	}
	return items, nil
}
