package account

import (
	"testing"
	"time"
)

// TestRandomInviteCode 邀请码格式与字符集（去掉易混淆 0/O/1/I）。
func TestRandomInviteCode(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		c := randomInviteCode()
		if len(c) != 8 {
			t.Fatalf("邀请码长度应 8, got %q", c)
		}
		for _, ch := range c {
			switch ch {
			case '0', 'O', '1', 'I':
				t.Errorf("邀请码含易混淆字符: %q", c)
			}
		}
		if seen[c] {
			t.Errorf("邀请码重复: %q", c)
		}
		seen[c] = true
	}
}

// TestInviteExpired 过期判断（requireInvite 的校验语义：过期/已用/不存在）。
func TestInviteValidationSemantics(t *testing.T) {
	now := time.Now()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)
	uid := int64(7)

	codes := map[string]*InviteCode{
		"valid":   {Code: "VALID000", ExpiresAt: &future},
		"expired": {Code: "EXPIRED0", ExpiresAt: &past},
		"used":    {Code: "USED0000", UsedBy: &uid, ExpiresAt: &future},
		"forever": {Code: "FOREVER0", ExpiresAt: nil},
	}
	if codes["valid"].expiredAt(now) {
		t.Error("未过期码不应判过期")
	}
	if !codes["expired"].expiredAt(now) {
		t.Error("过期码应判过期")
	}
	if codes["forever"].expiredAt(now) {
		t.Error("永久码不应判过期")
	}
	if codes["used"].UsedBy == nil {
		t.Error("used 码应记录使用者")
	}
}
