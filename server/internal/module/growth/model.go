// Package growth 成长域：经验/等级/权益规则引擎（M2-GRW-01）。
package growth

import "time"

// ExpLog 对应 exp_log 表（经验流水）。
type ExpLog struct {
	ID        int64 `gorm:"primaryKey;autoIncrement"`
	UserID    int64
	Delta     int
	Reason    string
	CreatedAt time.Time
}

func (ExpLog) TableName() string { return "exp_log" }

// 经验来源（reason 常量，与 exp_log.reason / Redis key 对齐）
const (
	ReasonDailyLogin  = "daily_login"  // 每日登录
	ReasonDailyWatch  = "daily_watch"  // 观看视频
	ReasonVideoUpload = "video_upload" // 投稿发布
	ReasonDanmakuSend = "danmaku_send" // 发送弹幕
	ReasonCommentSend = "comment_send" // 发表评论
)

// ExpRule 经验规则：单次经验 + 每日上限（Limit=1 表示每日一次）。
type ExpRule struct {
	Reason string `json:"reason"`
	Name   string `json:"name"`
	Delta  int    `json:"delta"`
	Limit  int    `json:"limit"`
}

// ExpRules 经验规则表（PRD ACC-20：登录/观看 +5、投稿 +10、弹幕评论互动等）。
var ExpRules = []ExpRule{
	{Reason: ReasonDailyLogin, Name: "每日登录", Delta: 5, Limit: 1},
	{Reason: ReasonDailyWatch, Name: "观看视频", Delta: 5, Limit: 1},
	{Reason: ReasonVideoUpload, Name: "投稿发布", Delta: 10, Limit: 2},
	{Reason: ReasonDanmakuSend, Name: "发送弹幕", Delta: 1, Limit: 20},
	{Reason: ReasonCommentSend, Name: "发表评论", Delta: 1, Limit: 20},
}

// ExpRuleOf 按 reason 取规则；未登记返回 nil。
func ExpRuleOf(reason string) *ExpRule {
	for i := range ExpRules {
		if ExpRules[i].Reason == reason {
			return &ExpRules[i]
		}
	}
	return nil
}

// LevelRule 等级规则：经验阈值与解锁权益。
type LevelRule struct {
	Level     int8   `json:"level"`
	MinExp    int    `json:"min_exp"` // 达到该等级所需累计经验
	Title     string `json:"title"`   // 等级称号
	Privilege string `json:"privilege"`
}

// LevelRules 等级表 Lv0-Lv6（阈值递增；注册即 Lv1 可发弹幕，Lv3 解锁彩色/顶底弹幕）。
var LevelRules = []LevelRule{
	{Level: 0, MinExp: 0},
	{Level: 1, MinExp: 0, Title: "注册会员", Privilege: "解锁弹幕发送"},
	{Level: 2, MinExp: 100, Title: "初级会员"},
	{Level: 3, MinExp: 300, Title: "中级会员", Privilege: "解锁彩色弹幕、顶部/底部弹幕"},
	{Level: 4, MinExp: 800, Title: "高级会员"},
	{Level: 5, MinExp: 1800, Title: "资深会员"},
	{Level: 6, MinExp: 3600, Title: "元老会员"},
}

// LevelByExp 按累计经验计算等级（阈值匹配取最高档）。
func LevelByExp(exp int) int8 {
	level := int8(0)
	for i := range LevelRules {
		if exp >= LevelRules[i].MinExp {
			level = LevelRules[i].Level
		}
	}
	return level
}

// RuleOfLevel 返回指定等级规则；未知等级返回 nil。
func RuleOfLevel(level int8) *LevelRule {
	for i := range LevelRules {
		if LevelRules[i].Level == level {
			return &LevelRules[i]
		}
	}
	return nil
}

// NextLevelRule 返回下一级规则；已满级返回 nil。
func NextLevelRule(level int8) *LevelRule {
	for i := range LevelRules {
		if LevelRules[i].Level == level && i+1 < len(LevelRules) {
			return &LevelRules[i+1]
		}
	}
	return nil
}
