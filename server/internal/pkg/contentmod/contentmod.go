// Package contentmod 内容安全机审（M2-AUD-01）：
// 文本检测抽象（敏感词热加载词库 + 广告/联系方式规则正则），命中结果供影子屏蔽/风险分级使用；
// 预留外部内容安全 API 接入点（部署环境可通过配置切换，见 TODO）。
package contentmod

import (
	"regexp"

	"github.com/dlidli/server/internal/pkg/moderate"
)

// 检测场景（按内容类型区分策略）
const (
	SceneVideo   = "video"   // 稿件标题/简介
	SceneComment = "comment" // 评论
	SceneDanmaku = "danmaku" // 弹幕
	SceneDynamic = "dynamic" // 动态
	SceneProfile = "profile" // 昵称/签名
)

// Result 检测结果。
type Result struct {
	Pass  bool     // true=通过（无风险）
	Words []string // 命中的词/规则名
}

// 规则正则（广告/联系方式诱导外流特征，命中即标记风险）
var rules = []struct {
	name string
	re   *regexp.Regexp
}{
	{"手机号", regexp.MustCompile(`1[3-9]\d{9}`)},
	{"QQ号", regexp.MustCompile(`(?i)qq\s*[:：]?\s*\d{5,12}`)},
	{"微信号", regexp.MustCompile(`(?i)微信\s*[:：]?\s*[a-zA-Z][a-zA-Z0-9_-]{5,19}`)},
	{"外链", regexp.MustCompile(`(?i)(https?://|www\.)[^\s]+`)},
}

// CheckText 本地机审：敏感词词库（管理后台热加载）+ 规则正则。
// 命中即视为有风险，由调用方决定影子屏蔽或风险分级。
// TODO(M2-AUD)：接入外部内容安全 API（图审/文本分类）时在此增加 external 实现并保持接口不变。
func CheckText(scene, content string) Result {
	if content == "" {
		return Result{Pass: true}
	}
	var words []string
	if moderate.Hit(content) {
		words = append(words, "敏感词")
	}
	for _, r := range rules {
		if r.re.MatchString(content) {
			words = append(words, r.name)
		}
	}
	return Result{Pass: len(words) == 0, Words: words}
}
