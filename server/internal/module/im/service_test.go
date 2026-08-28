package im

import "testing"

func TestParseUID(t *testing.T) {
	uid, err := parseUID("12345")
	if err != nil || uid != 12345 {
		t.Errorf("合法 UID 应解析成功, got %d %v", uid, err)
	}
	if _, err := parseUID("abc"); err == nil {
		t.Error("非数字应报错")
	}
	if _, err := parseUID(""); err == nil {
		t.Error("空串应报错")
	}
}
