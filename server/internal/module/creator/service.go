package creator

import (
	"context"
	"fmt"
	"time"

	"github.com/dlidli/server/internal/pkg/errcode"
	"go.uber.org/zap"
)

// ratePerView 激励单价：每有效播放 1 分（1000 播放 = 10 元，MVP 常量；后续接入系统配置）
const ratePerView = 1

// Service 创作者中心服务：看板聚合、单稿分析、激励结算。
type Service struct {
	repo *Repo
	log  *zap.Logger
}

func NewService(repo *Repo, log *zap.Logger) *Service {
	return &Service{repo: repo, log: log}
}

// settle 请求时全量结算（幂等 upsert；失败仅日志不阻塞看板）。
func (s *Service) settle(ctx context.Context, uid int64) {
	if err := s.repo.SettleAll(uid, ratePerView); err != nil {
		s.log.Warn("创作激励结算失败", zap.Int64("uid", uid), zap.Error(err))
	}
}

// Overview 创作者总览（触发结算后聚合）。
// 卡片累计值与趋势图同源：总有效播放=行为日志 action=3、赞/币/藏=user_action、粉丝=relation、收益=日结算。
func (s *Service) Overview(ctx context.Context, uid int64) (*Overview, error) {
	s.settle(ctx, uid)

	videoCnt, err := s.repo.CountMineVideos(uid)
	if err != nil {
		return nil, err
	}
	totalView, err := s.repo.BehaviorCnt(uid, 3)
	if err != nil {
		return nil, err
	}
	totalLike, err := s.repo.ActionCnt(uid, 1)
	if err != nil {
		return nil, err
	}
	totalCoin, err := s.repo.ActionCnt(uid, 2)
	if err != nil {
		return nil, err
	}
	totalFav, err := s.repo.ActionCnt(uid, 3)
	if err != nil {
		return nil, err
	}
	trend, err := s.repo.PlayTrend(uid, 7, TrendMetric["play"])
	if err != nil {
		return nil, err
	}
	fans, err := s.repo.FanCount(uid)
	if err != nil {
		return nil, err
	}
	earnings, err := s.repo.SumSettlements(uid)
	if err != nil {
		return nil, err
	}

	ov := &Overview{
		VideoCnt: videoCnt, TotalView: totalView, TotalLike: totalLike,
		TotalCoin: totalCoin, TotalFav: totalFav, Fans: fans, Earnings: earnings,
	}
	for _, t := range trend {
		ov.WeekView += t.Views
	}
	return ov, nil
}

// VideoStats 稿件数据列表（含有效播放与收益）。
func (s *Service) VideoStats(ctx context.Context, uid int64, page, size int) ([]VideoStatItem, int64, error) {
	s.settle(ctx, uid)

	list, total, err := s.repo.ListMineVideos(uid, page, size)
	if err != nil {
		return nil, 0, err
	}
	stats, err := s.repo.StatsByVideos(uid)
	if err != nil {
		return nil, 0, err
	}
	valid, err := s.repo.ValidViewStats(uid)
	if err != nil {
		return nil, 0, err
	}
	settles, _, err := s.repo.ListSettlements(uid, 1, 100000)
	if err != nil {
		return nil, 0, err
	}

	statMap := map[int64]struct {
		ViewCnt, LikeCnt, CoinCnt, FavCnt, CommentCnt, DanmakuCnt int64
	}{}
	for _, st := range stats {
		statMap[st.VideoID] = struct {
			ViewCnt, LikeCnt, CoinCnt, FavCnt, CommentCnt, DanmakuCnt int64
		}{st.ViewCnt, st.LikeCnt, st.CoinCnt, st.FavCnt, st.CommentCnt, st.DanmakuCnt}
	}
	validMap := map[int64]int64{}
	for _, v := range valid {
		validMap[v.VideoID] = v.Cnt
	}
	earningMap := map[int64]int64{}
	for _, s := range settles {
		earningMap[s.VideoID] += s.Amount
	}

	items := make([]VideoStatItem, 0, len(list))
	for _, v := range list {
		st := statMap[v.ID]
		items = append(items, VideoStatItem{
			Bvid: v.Bvid, Title: v.Title, Cover: v.Cover, Status: v.Status,
			View: st.ViewCnt, Like: st.LikeCnt, Coin: st.CoinCnt, Fav: st.FavCnt,
			Comment: st.CommentCnt, Danmaku: st.DanmakuCnt,
			ValidViews: validMap[v.ID], Earnings: earningMap[v.ID],
			PublishedAt: v.PublishedAt,
		})
	}
	return items, total, nil
}

// TrendMetric 趋势指标映射：
// play 有效播放 / like 点赞 / coin 投币 / fav 收藏 / interact 互动 / fans 新增粉丝 / earning 收益 / click 点击 / expose 曝光。
var TrendMetric = map[string]int8{
	"play":     3,
	"like":     1,
	"coin":     2,
	"fav":      3,
	"interact": 4,
	"click":    2,
	"expose":   1,
}

// Trend 近 N 天数据趋势（指标切换 + 天数切换，补零对齐）。
// play/click/expose 基于行为日志 action；like/coin/fav 基于 user_action；fans 基于新增关注；earning 基于日结算。
func (s *Service) Trend(ctx context.Context, uid int64, days int, metric string) ([]TrendPoint, error) {
	if days <= 0 || days > 30 {
		days = 7
	}
	var (
		list []TrendPoint
		err  error
	)
	switch metric {
	case "like", "coin":
		list, err = s.repo.ActionTrend(uid, days, TrendMetric[metric])
	case "fav":
		list, err = s.repo.ActionTrend(uid, days, 3)
	case "fans":
		list, err = s.repo.FanTrend(uid, days)
	case "earning":
		s.settle(ctx, uid) // 收益趋势前先结算
		list, err = s.repo.EarningTrend(uid, days)
	case "interact":
		list, err = s.repo.PlayTrend(uid, days, 4)
	case "click", "expose", "play":
		list, err = s.repo.PlayTrend(uid, days, TrendMetric[metric])
	default:
		list, err = s.repo.PlayTrend(uid, days, 3)
	}
	if err != nil {
		return nil, err
	}
	m := map[string]int64{}
	for _, t := range list {
		m[t.Date] = t.Views
	}
	out := make([]TrendPoint, 0, days)
	for i := days - 1; i >= 0; i-- {
		d := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		out = append(out, TrendPoint{Date: d[5:], Views: m[d]})
	}
	return out, nil
}

// Settlements 收益明细（分页）。
func (s *Service) Settlements(ctx context.Context, uid int64, page, size int) ([]SettlementItem, int64, error) {
	s.settle(ctx, uid)
	return s.repo.ListSettlements(uid, page, size)
}

// EarningsTotal 累计收益（分，字符串化展示用）。
func (s *Service) EarningsTotal(_ context.Context, uid int64) (string, error) {
	n, err := s.repo.SumSettlements(uid)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d", n), nil
}

// ensureCreator 校验稿件归属（预留：后续单稿分析详情用）。
func (s *Service) ensureCreator(_ context.Context, uid int64) error {
	if uid <= 0 {
		return errcode.ErrUnauthorized
	}
	return nil
}
