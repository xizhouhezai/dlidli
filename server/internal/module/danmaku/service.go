package danmaku

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dlidli/server/internal/module/account"
	"github.com/dlidli/server/internal/module/growth"
	"github.com/dlidli/server/internal/module/video"
	"github.com/dlidli/server/internal/pkg/contentmod"
	"github.com/dlidli/server/internal/pkg/errcode"
	"github.com/dlidli/server/internal/pkg/snowflake"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	sendCooldown  = 5 * time.Second
	dupCooldown   = 30 * time.Second
	segCacheTTL   = 10 * time.Minute
	maxBlockWords = 200
)

// Service 弹幕域服务：发送（频控/去重/机审）、分段拉取、屏蔽设置、实时广播。
type Service struct {
	repo       *Repo
	videoSvc   *video.Service
	accountSvc *account.Service
	growthSvc  *growth.Service
	rdb        *redis.Client
	hub        *Hub
	secret     string // UserHash 签名密钥（JWT secret）
	log        *zap.Logger
}

func NewService(repo *Repo, videoSvc *video.Service, accountSvc *account.Service,
	growthSvc *growth.Service, rdb *redis.Client, hub *Hub, secret string, log *zap.Logger,
) *Service {
	return &Service{repo: repo, videoSvc: videoSvc, accountSvc: accountSvc,
		growthSvc: growthSvc, rdb: rdb, hub: hub, secret: secret, log: log}
}

func segKey(videoID int64, seg int) string {
	return fmt.Sprintf("dm:v:%d:%d", videoID, seg)
}

// Send 发送弹幕：禁言/封禁拦截 → 等级/频控/去重校验 → 机审影子屏蔽 → 落库 + 计数 + 实时广播。
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

	content := strings.TrimSpace(req.Content)
	if content == "" {
		return nil, errcode.ErrInvalidParams
	}
	// 重复内容 30s 去重（同视频同内容，M2-DM-01）
	sum := sha256.Sum256([]byte(content))
	dupKey := fmt.Sprintf("dm:dup:%d:%x", videoID, sum[:8])
	if ok, err := s.rdb.SetNX(ctx, dupKey, 1, dupCooldown).Result(); err != nil {
		return nil, err
	} else if !ok {
		return nil, errcode.ErrDanmakuDuplicate
	}

	d := &Danmaku{
		ID:       snowflake.NextID(),
		VideoID:  videoID,
		UserID:   uid,
		Content:  content,
		TimeMs:   req.TimeMs,
		Mode:     req.Mode,
		Color:    req.Color,
		FontSize: req.FontSize,
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

	// 机审（M2-AUD-01）：命中敏感词/联系方式规则 → 影子屏蔽（仅发送者可见，不报错）
	if !contentmod.CheckText(contentmod.SceneDanmaku, d.Content).Pass {
		d.Status = StatusShadow
	}

	if err := s.repo.Create(d); err != nil {
		return nil, err
	}

	item := &Item{
		ID: fmt.Sprintf("%d", d.ID), Content: d.Content, TimeMs: d.TimeMs,
		Mode: d.Mode, Color: d.Color, FontSize: d.FontSize,
		SenderHash: s.UserHash(uid), IsSelf: true,
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
		// 实时广播（M2-DM-03）：同视频在线连接即时上屏
		s.hub.Broadcast(videoID, item)
	}

	return item, nil
}

// UserHash 用户 UID 的确定性哈希（HMAC-SHA256 截断 16 字符），供前端屏蔽用户且不暴露真实 UID。
func (s *Service) UserHash(uid int64) string {
	if uid <= 0 {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(s.secret))
	fmt.Fprintf(mac, "dm:%d", uid)
	return hex.EncodeToString(mac.Sum(nil))[:16]
}

// BlockItem 屏蔽设置项（对外）。
type BlockItem struct {
	ID        int64     `json:"id,string"`
	BlockType int8      `json:"block_type"` // 1关键词 2用户
	Keyword   string    `json:"keyword,omitempty"`
	BlockHash string    `json:"block_hash,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// ListBlocks 当前用户屏蔽列表。
func (s *Service) ListBlocks(_ context.Context, uid int64) ([]BlockItem, error) {
	list, err := s.repo.ListBlocks(uid)
	if err != nil {
		return nil, err
	}
	items := make([]BlockItem, 0, len(list))
	for _, b := range list {
		items = append(items, BlockItem{
			ID: b.ID, BlockType: b.BlockType, Keyword: b.Keyword,
			BlockHash: b.BlockHash, CreatedAt: b.CreatedAt,
		})
	}
	return items, nil
}

// AddBlock 新增屏蔽：关键词（1~64 字，上限 200 条）或用户（UID 转哈希 / 直接传哈希）。幂等。
func (s *Service) AddBlock(ctx context.Context, uid int64, blockType int8, keyword, targetUID, blockHash string) error {
	b := &DanmakuBlock{UserID: uid, BlockType: blockType}
	if blockType == BlockKeyword {
		keyword = strings.TrimSpace(keyword)
		if keyword == "" || len([]rune(keyword)) > 64 {
			return errcode.ErrInvalidParams.WithMsg("关键词需 1~64 字")
		}
		if n, _ := s.repo.CountBlocks(uid, BlockKeyword); n >= maxBlockWords {
			return errcode.ErrInvalidParams.WithMsg("屏蔽词最多 200 个")
		}
		b.Keyword = keyword
	} else if blockType == BlockUser {
		if blockHash == "" {
			// 前端无真实 UID 时按 UID 转哈希（弹幕列表/举报等后台场景）
			tid, err := strconv.ParseInt(targetUID, 10, 64)
			if err != nil || tid <= 0 {
				return errcode.ErrInvalidParams.WithMsg("用户 ID 不合法")
			}
			if tid == uid {
				return errcode.ErrInvalidParams.WithMsg("不能屏蔽自己")
			}
			blockHash = s.UserHash(tid)
		} else if len(blockHash) != 16 {
			return errcode.ErrInvalidParams.WithMsg("用户哈希不合法")
		}
		b.BlockHash = blockHash
	} else {
		return errcode.ErrInvalidParams
	}
	return s.repo.CreateBlock(b)
}

// DeleteBlock 删除屏蔽项（仅本人）。
func (s *Service) DeleteBlock(_ context.Context, uid, id int64) error {
	return s.repo.DeleteBlock(uid, id)
}

// filterByBlocks 内存过滤被屏蔽的弹幕（关键词命中 / 发送者被屏蔽）。
func (s *Service) filterByBlocks(items []Item, blocks []DanmakuBlock) []Item {
	if len(blocks) == 0 || len(items) == 0 {
		return items
	}
	var words []string
	hashes := make(map[string]bool)
	for _, b := range blocks {
		if b.BlockType == BlockKeyword {
			words = append(words, b.Keyword)
		} else {
			hashes[b.BlockHash] = true
		}
	}
	out := items[:0]
	for _, it := range items {
		if hashes[it.SenderHash] {
			continue
		}
		hit := false
		for _, w := range words {
			if strings.Contains(it.Content, w) {
				hit = true
				break
			}
		}
		if !hit {
			out = append(out, it)
		}
	}
	return out
}

// loadBlocks 加载用户屏蔽列表（登录用户；失败降级为空列表不阻塞拉取）。
func (s *Service) loadBlocks(ctx context.Context, uid int64) []DanmakuBlock {
	if uid <= 0 {
		return nil
	}
	list, err := s.repo.ListBlocks(uid)
	if err != nil {
		s.log.Warn("屏蔽列表加载失败", zap.Int64("uid", uid), zap.Error(err))
		return nil
	}
	return list
}

// ListSegment 分段拉取（Redis 缓存优先，miss 回源 MySQL 并回填）；登录用户过滤屏蔽项。
func (s *Service) ListSegment(ctx context.Context, bv string, seg int, viewerUID int64) ([]Item, error) {
	if seg < 0 {
		return nil, errcode.ErrInvalidParams
	}
	videoID, err := s.videoSvc.PublishedIDByBvid(ctx, bv)
	if err != nil {
		return nil, err
	}

	var items []Item
	key := segKey(videoID, seg)
	if cached, err := s.rdb.Get(ctx, key).Result(); err == nil {
		_ = json.Unmarshal([]byte(cached), &items)
	} else {
		list, err := s.repo.ListSegment(videoID, seg*SegmentMS, (seg+1)*SegmentMS)
		if err != nil {
			return nil, err
		}
		items = make([]Item, 0, len(list))
		for _, d := range list {
			items = append(items, Item{
				ID: fmt.Sprintf("%d", d.ID), Content: d.Content, TimeMs: d.TimeMs,
				Mode: d.Mode, Color: d.Color, FontSize: d.FontSize,
				SenderHash: s.UserHash(d.UserID),
			})
		}
		if buf, err := json.Marshal(items); err == nil {
			s.rdb.Set(ctx, key, buf, segCacheTTL)
		}
	}
	return s.filterByBlocks(items, s.loadBlocks(ctx, viewerUID)), nil
}

// ListAll 弹幕全量列表（列表面板，分页）；登录用户过滤屏蔽项。
func (s *Service) ListAll(ctx context.Context, bv string, viewerUID int64, page, size int) ([]Item, int64, error) {
	videoID, err := s.videoSvc.PublishedIDByBvid(ctx, bv)
	if err != nil {
		return nil, 0, err
	}
	list, total, err := s.repo.ListAll(videoID, page, size)
	if err != nil {
		return nil, 0, err
	}
	items := make([]Item, 0, len(list))
	for _, d := range list {
		items = append(items, Item{
			ID: fmt.Sprintf("%d", d.ID), Content: d.Content, TimeMs: d.TimeMs,
			Mode: d.Mode, Color: d.Color, FontSize: d.FontSize,
			SenderHash: s.UserHash(d.UserID),
		})
	}
	return s.filterByBlocks(items, s.loadBlocks(ctx, viewerUID)), total, nil
}

// DanmakuBrief 弹幕摘要（举报队列展示用）：内容 + 发送者。
func (s *Service) DanmakuBrief(_ context.Context, id int64) (content string, userID int64, err error) {
	d, err := s.repo.FindByID(id)
	if err != nil {
		return "", 0, err
	}
	if d == nil || d.Status == StatusDeleted {
		return "", 0, errcode.ErrNotFound
	}
	return d.Content, d.UserID, nil
}

// AdminDeleteDanmaku 管理员删除弹幕（举报处理用）。
func (s *Service) AdminDeleteDanmaku(_ context.Context, id int64) error {
	d, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}
	if d == nil || d.Status == StatusDeleted {
		return errcode.ErrNotFound
	}
	if err := s.repo.MarkDeleted(id); err != nil {
		return err
	}
	if d.Status == StatusNormal {
		s.rdb.Del(context.Background(), segKey(d.VideoID, d.TimeMs/SegmentMS))
		_ = s.videoSvc.AddStat(context.Background(), d.VideoID, "danmaku_cnt", -1)
	}
	return nil
}
