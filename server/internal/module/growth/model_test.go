package growth

import "testing"

// TestExpRuleOf 经验规则表完整性：PRD ACC-20 登录/观看 +5、投稿 +10、弹幕评论 +1。
func TestExpRuleOf(t *testing.T) {
	cases := []struct {
		reason string
		delta  int
		limit  int
	}{
		{ReasonDailyLogin, 5, 1},
		{ReasonDailyWatch, 5, 1},
		{ReasonVideoUpload, 10, 2},
		{ReasonDanmakuSend, 1, 20},
		{ReasonCommentSend, 1, 20},
	}
	for _, c := range cases {
		r := ExpRuleOf(c.reason)
		if r == nil {
			t.Errorf("规则 %s 不应缺失", c.reason)
			continue
		}
		if r.Delta != c.delta || r.Limit != c.limit {
			t.Errorf("规则 %s 应为 delta=%d limit=%d, got %+v", c.reason, c.delta, c.limit, r)
		}
	}
	if ExpRuleOf("unknown_reason") != nil {
		t.Error("未登记 reason 应返回 nil")
	}
}

// TestLevelByExp 等级阈值边界（Lv1 阈值 0：注册即 Lv1；各档取最高匹配）。
func TestLevelByExp(t *testing.T) {
	cases := []struct {
		exp   int
		level int8
	}{
		{0, 1},    // 注册即 Lv1
		{99, 1},   // Lv2 临界下
		{100, 2},  // Lv2 阈值
		{299, 2},  // Lv3 临界下
		{300, 3},  // Lv3 阈值（彩色/顶底弹幕解锁）
		{800, 4},  // Lv4
		{1800, 5}, // Lv5
		{3600, 6}, // Lv6
		{999999, 6},
	}
	for _, c := range cases {
		if got := LevelByExp(c.exp); got != c.level {
			t.Errorf("exp=%d 应为 Lv%d, got Lv%d", c.exp, c.level, got)
		}
	}
}

// TestRuleOfLevel 等级规则查询与越界。
func TestRuleOfLevel(t *testing.T) {
	if r := RuleOfLevel(3); r == nil || r.MinExp != 300 {
		t.Errorf("Lv3 应为阈值 300, got %+v", r)
	}
	if RuleOfLevel(7) != nil {
		t.Error("未知等级应返回 nil")
	}
}

// TestNextLevelRule 升级链与满级。
func TestNextLevelRule(t *testing.T) {
	if r := NextLevelRule(1); r == nil || r.Level != 2 {
		t.Errorf("Lv1 的下一级应为 Lv2, got %+v", r)
	}
	if NextLevelRule(6) != nil {
		t.Error("Lv6 已满级应返回 nil")
	}
}
