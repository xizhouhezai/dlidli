package account

import "testing"

// TestEmailRe 邮箱格式校验（ACC-02 注册前置）。
func TestEmailRe(t *testing.T) {
	cases := []struct {
		email string
		ok    bool
	}{
		{"user@example.com", true},
		{"a.b+c@sub.domain.cn", true},
		{"USER123@example.io", true},
		{"not-an-email", false},
		{"@example.com", false},
		{"user@", false},
		{"user@example", false},
		{"user @example.com", false},
		{"", false},
	}
	for _, c := range cases {
		if got := emailRe.MatchString(c.email); got != c.ok {
			t.Errorf("emailRe(%q) 应为 %v, got %v", c.email, c.ok, got)
		}
	}
}
