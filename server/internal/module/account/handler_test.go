package account

import "testing"

// TestPhoneRe 手机号格式校验（1 开头 11 位数字）。
func TestPhoneRe(t *testing.T) {
	cases := []struct {
		phone string
		ok    bool
	}{
		{"13800138000", true},
		{"19912345678", true},
		{"23800138000", false},  // 非 1 开头
		{"1380013800", false},   // 10 位
		{"138001380001", false}, // 12 位
		{"1380013800a", false},  // 含字母
		{"", false},
	}
	for _, c := range cases {
		if got := phoneRe.MatchString(c.phone); got != c.ok {
			t.Errorf("phoneRe(%q) 应为 %v, got %v", c.phone, c.ok, got)
		}
	}
}
