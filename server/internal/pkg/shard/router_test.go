package shard

import (
	"regexp"
	"testing"
)

func TestIndexStableAndBounded(t *testing.T) {
	keys := []string{"video-1", "video-1", "video-2", "", "用户-42"}
	for _, key := range keys {
		got := IndexByString(key)
		if got >= Count {
			t.Fatalf("index out of range: key=%q index=%d", key, got)
		}
		if got != IndexByString(key) {
			t.Fatalf("index is not stable: key=%q", key)
		}
	}
	if IndexByUint64(0) != 0 || IndexByUint64(16) != 0 || IndexByUint64(31) != 15 {
		t.Fatalf("expected power-of-two mask routing")
	}
}

func TestTableForAllowlist(t *testing.T) {
	namePattern := regexp.MustCompile(`^(comment|danmaku|user_action)_[0-9]{2}$`)
	for _, base := range []string{BaseComment, BaseDanmaku, BaseUserAction} {
		name, err := TableFor(base, 42)
		if err != nil || !namePattern.MatchString(name) {
			t.Fatalf("unexpected table: base=%q name=%q err=%v", base, name, err)
		}
		tables, err := Tables(base)
		if err != nil || len(tables) != int(Count) {
			t.Fatalf("expected %d tables: base=%q len=%d err=%v", Count, base, len(tables), err)
		}
		for i, table := range tables {
			want := base + "_" + twoDigits(i)
			if table != want {
				t.Fatalf("table order unstable: got=%q want=%q", table, want)
			}
		}
	}
	for _, bad := range []string{"comment;DROP TABLE user", "users", ""} {
		if _, err := TableFor(bad, 1); err == nil {
			t.Fatalf("unapproved base should fail: %q", bad)
		}
	}
}

func TestTableForStringMatchesHashRoute(t *testing.T) {
	name1, err := TableForString(BaseDanmaku, "video-42")
	if err != nil {
		t.Fatal(err)
	}
	name2, err := TableFor(BaseDanmaku, HashString("video-42"))
	if err != nil {
		t.Fatal(err)
	}
	if name1 != name2 {
		t.Fatalf("string and uint64 routes differ: %q != %q", name1, name2)
	}
}

func twoDigits(value int) string {
	if value < 10 {
		return "0" + string(rune('0'+value))
	}
	return string(rune('0'+value/10)) + string(rune('0'+value%10))
}
