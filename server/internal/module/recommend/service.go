package recommend

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/dlidli/server/internal/module/video"
	"github.com/dlidli/server/internal/pkg/errcode"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	hotCacheTTL = 5 * time.Minute
	recPoolSize = 200 // 召回池大小（打散后分页截取）
	pageSizeMax = 50
)

// candidate 召回候选（打散/过滤用）。
type candidate struct {
	id  int64
	cat int
	up  int64
	// isNew 新稿扶持池命中（保底曝光，score 置为池内最低热度分）
	isNew bool
}

// abtestProvider A/B 实验分流（M3-OPS-03，接口避免循环依赖）。
type abtestProvider interface {
	Variant(ctx context.Context, uid int64, target string) (string, error)
}

// Service 推荐域服务：热度榜、混合召回推荐、行为采集、负反馈、推荐开关。
type Service struct {
	repo     *Repo
	videoSvc *video.Service
	rdb      *redis.Client
	log      *zap.Logger
	ab       abtestProvider
}

func NewService(repo *Repo, videoSvc *video.Service, rdb *redis.Client, log *zap.Logger) *Service {
	return &Service{repo: repo, videoSvc: videoSvc, rdb: rdb, log: log}
}

// SetABTest 注入 A/B 实验分流器（router 装配时调用）。
func (s *Service) SetABTest(ab abtestProvider) {
	s.ab = ab
}

func hotCacheKey(categoryID int) string {
	return fmt.Sprintf("rec:hot:%d", categoryID)
}

// HotVideos 全站/分区热度榜（加权分，Redis 缓存 5 分钟）。
func (s *Service) HotVideos(ctx context.Context, categoryID, page, size int) ([]video.Card, error) {
	key := hotCacheKey(categoryID)
	ids := s.hotIDsCached(ctx, key, categoryID)

	start := (page - 1) * size
	if start >= len(ids) {
		return []video.Card{}, nil
	}
	end := start + size
	if end > len(ids) {
		end = len(ids)
	}
	return s.videoSvc.CardsByIDs(ctx, ids[start:end])
}

// hotIDsCached 热度榜 ID 列表（缓存优先）。
func (s *Service) hotIDsCached(ctx context.Context, key string, categoryID int) []int64 {
	if cached, err := s.rdb.Get(ctx, key).Result(); err == nil {
		var ids []int64
		if json.Unmarshal([]byte(cached), &ids) == nil && len(ids) > 0 {
			return ids
		}
	}
	entries, err := s.repo.ListHot(categoryID, recPoolSize)
	if err != nil {
		s.log.Warn("热度榜查询失败", zap.Int("category", categoryID), zap.Error(err))
		return []int64{}
	}
	ids := make([]int64, 0, len(entries))
	for _, e := range entries {
		ids = append(ids, e.VideoID)
	}
	if buf, err := json.Marshal(ids); err == nil {
		s.rdb.Set(ctx, key, buf, hotCacheTTL)
	}
	return ids
}

// Recommend 混合召回推荐：热度榜 + 兴趣分区（有行为时）+ 新稿扶持池，过滤已看/负反馈后打散分页。
func (s *Service) Recommend(ctx context.Context, uid int64, page, size int) ([]video.Card, error) {
	if size <= 0 || size > pageSizeMax {
		size = 20
	}
	if page < 1 {
		page = 1
	}

	personalized := false
	if uid > 0 {
		if on, err := s.repo.RecommendOn(uid); err == nil && on {
			personalized = true
		}
	}

	// A/B 实验分流（M3-OPS-03）：target=recommend 的启用实验按用户哈希分 A/B 组。
	// variant=hot_only 时退化为纯热度榜（关闭个性化召回），hybrid 或空串走默认混合策略。
	variant := ""
	if s.ab != nil {
		if v, err := s.ab.Variant(ctx, uid, "recommend"); err == nil {
			variant = v
		}
	}
	if variant == "hot_only" {
		personalized = false
	}

	var cands []candidate
	if personalized {
		// 兴趣分区召回：最近观看/点击视频的分区热度榜
		cats, err := s.repo.RecentWatchCategories(uid, 3)
		if err == nil && len(cats) > 0 {
			for _, c := range cats {
				for _, id := range s.hotIDsCached(ctx, hotCacheKey(c), c) {
					cands = append(cands, candidate{id: id, cat: c, up: s.upOf(ctx, id)})
				}
			}
		}
		// ItemCF 相似视频召回（M3-REC-03）：最近观看 2 个视频的相似视频，提升个性化
		if watched, err := s.repo.RecentWatchedIDs(uid, 2); err == nil && len(watched) > 0 {
			if sims, err := s.repo.SimilarByVideos(watched, 12); err == nil {
				for _, id := range sims {
					cands = append(cands, candidate{id: id, cat: s.catOf(ctx, id), up: s.upOf(ctx, id)})
				}
			}
		}
	}
	// 全站热度兜底（个性化时也混入，保证召回量与多样性）
	hotAll := s.hotIDsCached(ctx, hotCacheKey(0), 0)
	for _, id := range hotAll {
		cands = append(cands, candidate{id: id, cat: s.catOf(ctx, id), up: s.upOf(ctx, id)})
	}
	// 新稿扶持池（保底曝光，排在后部）
	if newIDs, err := s.repo.NewPool(8); err == nil {
		for _, id := range newIDs {
			cands = append(cands, candidate{id: id, cat: s.catOf(ctx, id), up: s.upOf(ctx, id), isNew: true})
		}
	}

	// 过滤：已看 / 负反馈（内容/UP主/分区）
	watched := map[int64]bool{}
	if uid > 0 {
		if ids, err := s.repo.RecentWatchedIDs(uid, 200); err == nil {
			for _, id := range ids {
				watched[id] = true
			}
		}
	}
	dislikeVideos, dislikeUPs, dislikeCats := s.dislikeSets(ctx, uid)

	filtered := cands[:0]
	seen := map[int64]bool{}
	for _, c := range cands {
		if watched[c.id] || seen[c.id] {
			continue
		}
		if dislikeVideos[c.id] || dislikeUPs[c.up] || dislikeCats[c.cat] {
			continue
		}
		seen[c.id] = true
		filtered = append(filtered, c)
	}

	// 打散：同 UP 一屏 ≤1、同分区连续 ≤3，再分页截取
	ordered := s.diversify(filtered)
	start := (page - 1) * size
	if start >= len(ordered) {
		return []video.Card{}, nil
	}
	end := start + size
	if end > len(ordered) {
		end = len(ordered)
	}
	ids := ordered[start:end]
	return s.videoSvc.CardsByIDs(ctx, ids)
}

// diversify 打散：同 UP 只保留首个、同分区连续最多 3 个。
func (s *Service) diversify(cands []candidate) []int64 {
	seenUP := map[int64]bool{}
	catStreak := map[int]int{}
	out := make([]int64, 0, len(cands))
	for _, c := range cands {
		if seenUP[c.up] {
			continue
		}
		if catStreak[c.cat] >= 3 {
			continue
		}
		out = append(out, c.id)
		seenUP[c.up] = true
		catStreak[c.cat]++
	}
	return out
}

// dislikeSets 负反馈集合（内容/UP主/分区）。
func (s *Service) dislikeSets(ctx context.Context, uid int64) (map[int64]bool, map[int64]bool, map[int]bool) {
	vids, ups, cats := map[int64]bool{}, map[int64]bool{}, map[int]bool{}
	if uid <= 0 {
		return vids, ups, cats
	}
	list, err := s.repo.DislikesOf(uid)
	if err != nil {
		return vids, ups, cats
	}
	for _, d := range list {
		switch d.TargetType {
		case DislikeVideo:
			vids[d.TargetID] = true
		case DislikeUP:
			ups[d.TargetID] = true
		case DislikeCategory:
			cats[int(d.TargetID)] = true
		}
	}
	return vids, ups, cats
}

// catOf / upOf 候选稿件的基础元信息（小批量逐查，池 200 内可接受；规模化后随召回一并取）。
func (s *Service) catOf(ctx context.Context, videoID int64) int {
	return s.videoSvc.CategoryOf(ctx, videoID)
}

func (s *Service) upOf(ctx context.Context, videoID int64) int64 {
	if uid, err := s.videoSvc.OwnerID(ctx, videoID); err == nil {
		return uid
	}
	return 0
}

// ReportBehavior 行为上报（曝光/点击/播放/互动），旁路失败仅日志。
func (s *Service) ReportBehavior(_ context.Context, uid int64, items []BehaviorItem) {
	if len(items) == 0 {
		return
	}
	list := make([]UserBehavior, 0, len(items))
	for _, it := range items {
		id, err := strconv.ParseInt(it.VideoID, 10, 64)
		if err != nil || id <= 0 || it.Action < ActionExpose || it.Action > ActionInteract {
			continue
		}
		list = append(list, UserBehavior{UserID: uid, VideoID: id, Action: it.Action})
	}
	if len(list) == 0 {
		return
	}
	if err := s.repo.AddBehaviors(list); err != nil {
		s.log.Warn("行为日志写入失败", zap.Int("n", len(list)), zap.Error(err))
	}
}

// AddDislike 负反馈（不感兴趣）。
func (s *Service) AddDislike(_ context.Context, uid int64, targetType int8, targetID string) error {
	id, err := strconv.ParseInt(targetID, 10, 64)
	if err != nil || id <= 0 || targetType < DislikeVideo || targetType > DislikeCategory {
		return errcode.ErrInvalidParams
	}
	return s.repo.AddDislike(&UserDislike{UserID: uid, TargetType: targetType, TargetID: id})
}

// RecommendSetting 推荐开关状态。
func (s *Service) RecommendSetting(_ context.Context, uid int64) (bool, error) {
	return s.repo.RecommendOn(uid)
}

// SetRecommendSetting 开关个性化推荐（合规）。
func (s *Service) SetRecommendSetting(_ context.Context, uid int64, on bool) error {
	return s.repo.SetRecommendOn(uid, on)
}
