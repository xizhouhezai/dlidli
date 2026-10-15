// Package contentmod 内容安全机审（M2-AUD-01）：
// 文本检测抽象（敏感词热加载词库 + 广告/联系方式规则正则），命中结果供影子屏蔽/风险分级使用；
// 预留外部内容安全 API 接入点（部署环境可通过配置切换，见 TODO）。
package contentmod

import (
	"image"
	_ "image/jpeg" // 注册解码器：截帧图与外部图审回传可能是 jpeg
	_ "image/png"
	"os"
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
	SceneLive    = "live"    // 直播截帧（M4-LIV-05）
)

// Result 检测结果。
type Result struct {
	Pass  bool     // true=通过（无风险）
	Words []string // 命中的词/规则名
	// Degraded 机审链路不可用（无法判定），与「判定通过」严格区分。
	// 调用方按 fail-open 处置：不据此切断，只记降级事实（plan live §2D.2 待替换点）。
	Degraded bool
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

// CheckImage 图片/画面机审（M4-LIV-05 直播截帧；本地规则，无外部依赖）。
//
// 本卡实现的是「低信息量帧」判据——黑屏与纯色遮挡（YMAX-YMIN 动态范围坍缩）。
// 这类画面本身是规避自动审核的常见手段，作为本地规则有独立价值，且不依赖任何外部服务。
//
// 灰度按 BT.601 全值域 Y = (299R + 587G + 114B) / 1000 计算；大图按步长抽样，
// 保证复杂度与图片尺寸无关（截帧图可达 1080p）。
//
// 无法判定（文件不存在/解码失败/尺寸为 0）时返回 Degraded=true 且 Pass=true：
// 调用方按 fail-open 处置——不切断，只记降级事实（plan live §2D.2）。
//
// TODO(M2-AUD/M4-LIV-05)：接入外部内容安全 API（图审/画面分类）时在本函数内增加
// external 分支并保持签名不变，调用方（live 审计服务）不改。
func CheckImage(scene, imagePath string, blackYAvgMax, flatRangeMax float64) Result {
	if imagePath == "" {
		return Result{Pass: true, Degraded: true}
	}
	f, err := os.Open(imagePath)
	if err != nil {
		return Result{Pass: true, Degraded: true}
	}
	defer func() { _ = f.Close() }()

	img, _, err := image.Decode(f)
	if err != nil {
		return Result{Pass: true, Degraded: true}
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return Result{Pass: true, Degraded: true}
	}

	// 抽样步长：目标采样点约 65536 个（256x256），与分辨率解耦
	step := 1
	for (w/step)*(h/step) > 1<<16 {
		step++
	}

	var sum, yMin, yMax float64
	var n int
	yMax = -1
	yMin = 1 << 30
	for y := b.Min.Y; y < b.Max.Y; y += step {
		for x := b.Min.X; x < b.Max.X; x += step {
			r, g, bl, _ := img.At(x, y).RGBA() // 16-bit 分量
			// 转 8-bit 后按 BT.601 加权（整数系数，避免浮点误差累积）
			lum := float64((299*(r>>8) + 587*(g>>8) + 114*(bl>>8)) / 1000)
			sum += lum
			if lum < yMin {
				yMin = lum
			}
			if lum > yMax {
				yMax = lum
			}
			n++
		}
	}
	if n == 0 {
		return Result{Pass: true, Degraded: true}
	}
	yAvg := sum / float64(n)

	var words []string
	if yAvg <= blackYAvgMax {
		words = append(words, "黑屏")
	}
	if yMax-yMin <= flatRangeMax {
		words = append(words, "纯色遮挡")
	}
	return Result{Pass: len(words) == 0, Words: words}
}
