// Package moderate 文本内容预检（敏感词命中 → 影子屏蔽）。
// 词库支持运行时热加载：启动时从 DB 载入，管理后台增删改后调用 Reload 即时生效。
// TODO(M2-AUD)：升级为分级词库（禁止/替换/仅标记）+ 内容安全机审 API。
package moderate

import (
	"strings"
	"sync"
)

// 内置兜底词库：DB 词库为空或加载失败时至少拦截这些。
var builtinWords = []string{"广告代理", "加微信", "赌博", "代刷"}

var (
	mu    sync.RWMutex
	words = append([]string(nil), builtinWords...)
)

// SetWords 热加载词库（管理后台变更后调用）。为空时回退内置兜底词库。
// 传入的词会与内置兜底词合并去重，保证内置词始终生效。
func SetWords(list []string) {
	set := make(map[string]struct{}, len(list)+len(builtinWords))
	merged := make([]string, 0, len(list)+len(builtinWords))
	add := func(w string) {
		w = strings.TrimSpace(w)
		if w == "" {
			return
		}
		if _, ok := set[w]; ok {
			return
		}
		set[w] = struct{}{}
		merged = append(merged, w)
	}
	for _, w := range builtinWords {
		add(w)
	}
	for _, w := range list {
		add(w)
	}

	mu.Lock()
	words = merged
	mu.Unlock()
}

// Hit 返回内容是否命中敏感词。
func Hit(content string) bool {
	mu.RLock()
	defer mu.RUnlock()
	for _, w := range words {
		if strings.Contains(content, w) {
			return true
		}
	}
	return false
}
