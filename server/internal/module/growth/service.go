package growth

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const dailyTTL = 48 * time.Hour

// Service 成长域服务：经验发放（每日去重/限量）、等级重算、成长总览。
type Service struct {
	repo *Repo
	rdb  *redis.Client
	log  *zap.Logger
}

func NewService(repo *Repo, rdb *redis.Client, log *zap.Logger) *Service {
	return &Service{repo: repo, rdb: rdb, log: log}
}

func onceKey(uid int64, reason string) string {
	return fmt.Sprintf("exp:once:%d:%s:%s", uid, time.Now().Format("2006-01-02"), reason)
}

func cntKey(uid int64, reason string) string {
	return fmt.Sprintf("exp:cnt:%d:%s:%s", uid, time.Now().Format("2006-01-02"), reason)
}

// AddExpOncePerDay 每日一次型经验（登录/观看）；当日已获取则静默跳过。
// 旁路逻辑：Redis/DB 异常仅记录日志，不影响调用方主流程。
func (s *Service) AddExpOncePerDay(ctx context.Context, uid int64, reason string) {
	rule := ExpRuleOf(reason)
	if rule == nil || uid <= 0 {
		return
	}
	ok, err := s.rdb.SetNX(ctx, onceKey(uid, reason), 1, dailyTTL).Result()
	if err != nil || !ok {
		return
	}
	s.grant(ctx, uid, reason, rule.Delta)
}

// AddExpWithLimit 每日限量型经验（投稿/弹幕/评论）；超过当日上限后不再发放。
func (s *Service) AddExpWithLimit(ctx context.Context, uid int64, reason string) {
	rule := ExpRuleOf(reason)
	if rule == nil || uid <= 0 {
		return
	}
	key := cntKey(uid, reason)
	n, err := s.rdb.Incr(ctx, key).Result()
	if err != nil {
		return
	}
	if n == 1 {
		_ = s.rdb.Expire(ctx, key, dailyTTL).Err()
	}
	if n > int64(rule.Limit) {
		return
	}
	s.grant(ctx, uid, reason, rule.Delta)
}

// grant 记流水 + 累加经验 + 重算等级。
func (s *Service) grant(ctx context.Context, uid int64, reason string, delta int) {
	if err := s.repo.CreateExpLog(&ExpLog{UserID: uid, Delta: delta, Reason: reason}); err != nil {
		s.log.Warn("经验流水写入失败", zap.Int64("uid", uid), zap.String("reason", reason), zap.Error(err))
		return
	}
	if _, level, err := s.repo.AddExp(uid, delta); err != nil {
		s.log.Warn("经验/等级更新失败", zap.Int64("uid", uid), zap.String("reason", reason), zap.Error(err))
	} else if level > 0 {
		s.log.Debug("用户经验更新", zap.Int64("uid", uid), zap.Int("level", level))
	}
}

// TaskItem 今日任务状态（经验获取入口）。
type TaskItem struct {
	Reason  string `json:"reason"`
	Name    string `json:"name"`
	Delta   int    `json:"delta"`
	Current int    `json:"current"` // 今日已完成次数
	Limit   int    `json:"limit"`   // 每日上限（1 = 每日一次）
	Done    bool   `json:"done"`
}

// Summary 成长总览：等级/经验进度/今日任务。
type Summary struct {
	Level     int8       `json:"level"`
	Exp       int        `json:"exp"`
	NextLevel int8       `json:"next_level"` // 0 = 已满级
	NextExp   int        `json:"next_exp"`   // 下一级所需累计经验（满级为当前级阈值）
	Progress  int        `json:"progress"`   // 当前级内进度 0-100（满级恒 100）
	Tasks     []TaskItem `json:"tasks"`
}

// Summary 汇总当前用户成长状态。
func (s *Service) Summary(ctx context.Context, uid int64) (*Summary, error) {
	exp, level, err := s.repo.LevelAndExp(uid)
	if err != nil {
		return nil, err
	}
	sum := &Summary{Level: int8(level), Exp: exp, Tasks: make([]TaskItem, 0, len(ExpRules))}

	if next := NextLevelRule(int8(level)); next != nil {
		sum.NextLevel = next.Level
		sum.NextExp = next.MinExp
		cur := RuleOfLevel(int8(level))
		if cur != nil {
			span := next.MinExp - cur.MinExp
			if span > 0 {
				sum.Progress = (exp - cur.MinExp) * 100 / span
			}
		}
	} else {
		sum.NextExp = RuleOfLevel(int8(level)).MinExp
		sum.Progress = 100
	}

	for _, r := range ExpRules {
		item := TaskItem{Reason: r.Reason, Name: r.Name, Delta: r.Delta, Limit: r.Limit}
		if r.Limit == 1 {
			n, _ := s.rdb.Exists(ctx, onceKey(uid, r.Reason)).Result()
			item.Done = n > 0
		} else {
			n, _ := s.rdb.Get(ctx, cntKey(uid, r.Reason)).Int()
			item.Current = n
			item.Done = n >= r.Limit
		}
		sum.Tasks = append(sum.Tasks, item)
	}
	return sum, nil
}

// ExpLogItem 经验流水项（ID 字符串化避免 JS 精度丢失）。
type ExpLogItem struct {
	ID         string    `json:"id"`
	Delta      int       `json:"delta"`
	Reason     string    `json:"reason"`
	ReasonName string    `json:"reason_name"`
	CreatedAt  time.Time `json:"created_at"`
}

// ExpLogs 经验流水分页。
func (s *Service) ExpLogs(_ context.Context, uid int64, page, size int) ([]ExpLogItem, int64, error) {
	list, total, err := s.repo.ListExpLogs(uid, page, size)
	if err != nil {
		return nil, 0, err
	}
	items := make([]ExpLogItem, 0, len(list))
	for _, l := range list {
		item := ExpLogItem{
			ID: fmt.Sprintf("%d", l.ID), Delta: l.Delta, Reason: l.Reason,
			ReasonName: l.Reason, CreatedAt: l.CreatedAt,
		}
		if rule := ExpRuleOf(l.Reason); rule != nil {
			item.ReasonName = rule.Name
		}
		items = append(items, item)
	}
	return items, total, nil
}
