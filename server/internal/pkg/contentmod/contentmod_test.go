package contentmod

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestCheckTextEmptyPass(t *testing.T) {
	r := CheckText(SceneVideo, "")
	if !r.Pass || len(r.Words) != 0 {
		t.Errorf("空文本应通过, got %+v", r)
	}
}

func TestCheckTextCleanPass(t *testing.T) {
	r := CheckText(SceneComment, "这个视频讲得很清楚，赞！")
	if !r.Pass {
		t.Errorf("正常文本应通过, got %+v", r)
	}
}

func TestCheckTextRuleHits(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{"手机号", "请联系 13812345678", "手机号"},
		{"QQ号", "加QQ:123456789", "QQ号"},
		{"微信号", "微信：abc_12345", "微信号"},
		{"外链http", "详情 https://example.com/x", "外链"},
		{"外链www", "看 www.example.com", "外链"},
	}
	for _, c := range cases {
		r := CheckText(SceneVideo, c.content)
		if r.Pass {
			t.Errorf("%s: 应命中风险, got pass", c.name)
		}
		if !slices.Contains(r.Words, c.want) {
			t.Errorf("%s: Words=%v 缺少 %q", c.name, r.Words, c.want)
		}
	}
}

func TestCheckTextSensitiveWord(t *testing.T) {
	r := CheckText(SceneDanmaku, "专业代刷等你来")
	if r.Pass {
		t.Fatal("内置敏感词应命中")
	}
	if !slices.Contains(r.Words, "敏感词") {
		t.Errorf("Words=%v 缺少 敏感词", r.Words)
	}
}

func TestCheckTextMultiHits(t *testing.T) {
	r := CheckText(SceneProfile, "加微信 abc_12345 电话13912345678")
	if len(r.Words) < 2 {
		t.Errorf("多规则应全部命中, got %v", r.Words)
	}
}

// ---- CheckImage：直播截帧画面机审（M4-LIV-05，spec §5E.2 / plan §2D.2）----

// 默认阈值，与 configs/dev.yaml 的 live.audit 对齐（plan §2D.5 实测标定）。
const (
	testBlackYAvgMax = 24.0
	testFlatRangeMax = 12.0
)

func writePNGForTest(t *testing.T, path string, w, h int, c color.Color) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

// TestCheckImageBlackHits 纯黑帧命中「黑屏」（实测 YAVG=0）。
func TestCheckImageBlackHits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "black.png")
	writePNGForTest(t, path, 640, 360, color.RGBA{R: 0, G: 0, B: 0, A: 255})

	res := CheckImage(SceneLive, path, testBlackYAvgMax, testFlatRangeMax)
	if res.Pass {
		t.Fatalf("纯黑帧应命中，got %+v", res)
	}
	if !slices.Contains(res.Words, "黑屏") {
		t.Fatalf("应报「黑屏」，got %v", res.Words)
	}
	if res.Degraded {
		t.Fatal("纯黑帧是可判定的，不应 Degraded")
	}
}

// TestCheckImageFlatHits 纯色遮挡命中「纯色遮挡」（动态范围坍缩）。
func TestCheckImageFlatHits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "flat.png")
	writePNGForTest(t, path, 640, 360, color.RGBA{R: 0x10, G: 0x10, B: 0x10, A: 255})

	res := CheckImage(SceneLive, path, testBlackYAvgMax, testFlatRangeMax)
	if res.Pass {
		t.Fatalf("纯色遮挡应命中，got %+v", res)
	}
	if !slices.Contains(res.Words, "纯色遮挡") {
		t.Fatalf("应报「纯色遮挡」，got %v", res.Words)
	}
}

// TestCheckImageNormalPasses 有动态范围的画面应通过且非降级。
func TestCheckImageNormalPasses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "normal.png")
	img := image.NewRGBA(image.Rect(0, 0, 320, 180))
	for y := 0; y < 180; y++ {
		for x := 0; x < 320; x++ {
			v := uint8(x * 255 / 319) // 左黑右白的渐变：YMAX-YMIN 远超阈值
			img.Set(x, y, color.RGBA{R: v, G: v, B: v, A: 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	res := CheckImage(SceneLive, path, testBlackYAvgMax, testFlatRangeMax)
	if !res.Pass || res.Degraded {
		t.Fatalf("渐变正常帧应通过且非降级，got %+v", res)
	}
	if len(res.Words) != 0 {
		t.Fatalf("正常帧不应有命中词，got %v", res.Words)
	}
}

// TestCheckImageUndecidableDegraded 无法判定时 Degraded=true 且 Pass=true
// （fail-open：调用方据此不切断，plan §2D.2 待替换点）。
func TestCheckImageUndecidableDegraded(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.png")
	if err := os.WriteFile(bad, []byte("not a png"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		path string
	}{
		{"空路径", ""},
		{"文件不存在", filepath.Join(dir, "missing.png")},
		{"非图片内容", bad},
		{"目录而非文件", dir},
	}
	for _, c := range cases {
		res := CheckImage(SceneLive, c.path, testBlackYAvgMax, testFlatRangeMax)
		if !res.Degraded {
			t.Errorf("%s: 应 Degraded=true，got %+v", c.name, res)
		}
		if !res.Pass {
			t.Errorf("%s: fail-open 要求 Pass=true，got %+v", c.name, res)
		}
	}
}

// TestCheckImageSamplingScales 大图（1080p/4K）不得因抽样出问题：
// 抽样后仍须命中黑屏，且不因图片变大而 Degraded。
func TestCheckImageSamplingScales(t *testing.T) {
	for _, sz := range []struct{ w, h int }{{1920, 1080}, {3840, 2160}} {
		path := filepath.Join(t.TempDir(), "big.png")
		writePNGForTest(t, path, sz.w, sz.h, color.RGBA{R: 0, G: 0, B: 0, A: 255})
		res := CheckImage(SceneLive, path, testBlackYAvgMax, testFlatRangeMax)
		if res.Degraded {
			t.Fatalf("%dx%d: 不应 Degraded", sz.w, sz.h)
		}
		if res.Pass || !slices.Contains(res.Words, "黑屏") {
			t.Fatalf("%dx%d: 黑帧应命中，got %+v", sz.w, sz.h, res)
		}
	}
}
