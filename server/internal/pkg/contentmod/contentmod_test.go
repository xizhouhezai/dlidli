package contentmod

import (
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
