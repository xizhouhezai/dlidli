package moderate

import "testing"

func TestHitBuiltinWords(t *testing.T) {
	for _, w := range []string{"广告代理", "加微信", "赌博", "代刷"} {
		if !Hit("内容包含" + w + "的文本") {
			t.Errorf("内置敏感词 %q 应命中", w)
		}
	}
	if Hit("正常的内容") {
		t.Error("正常文本不应命中")
	}
	if Hit("") {
		t.Error("空文本不应命中")
	}
}

func TestSetWordsMergeAndFallback(t *testing.T) {
	SetWords([]string{"自定义词", "自定义词", "  ", "加微信"})
	if !Hit("自定义词") {
		t.Error("热加载词应生效")
	}
	if !Hit("代刷") {
		t.Error("内置兜底词应始终保留")
	}
	// 空列表回退内置兜底
	SetWords(nil)
	if Hit("自定义词") {
		t.Error("清空后热加载词不应残留")
	}
	if !Hit("赌博") {
		t.Error("空列表应回退内置兜底词库")
	}
}
