package bvid

import "testing"

func TestEncodeDecodeRoundTrip(t *testing.T) {
	cases := []int64{1, 2, 62, 63, 1000, 2082030064353939456}
	for _, id := range cases {
		enc := Encode(id)
		dec := Decode(enc)
		if dec != id {
			t.Errorf("Encode(%d)=%s Decode()=%d, want %d", id, enc, dec, id)
		}
		if len(enc) < 3 || enc[:2] != "DV" {
			t.Errorf("Encode(%d)=%s 应以 DV 前缀", id, enc)
		}
	}
}

func TestEncodeNonPositive(t *testing.T) {
	if got := Encode(0); got != "" {
		t.Errorf("Encode(0)=%q, want 空串", got)
	}
	if got := Encode(-5); got != "" {
		t.Errorf("Encode(-5)=%q, want 空串", got)
	}
}

func TestDecodeInvalid(t *testing.T) {
	cases := []string{"", "AV123", "DV", "DV@@", "DV 12"}
	for _, s := range cases {
		if got := Decode(s); got != 0 {
			t.Errorf("Decode(%q)=%d, want 0", s, got)
		}
	}
}

func TestEncodeSortOrder(t *testing.T) {
	// 编码结果应近似长度递增、ID 递增（粗验唯一性/单调）
	seen := map[string]bool{}
	prevLen := 0
	for id := int64(1); id < 10000; id += 97 {
		enc := Encode(id)
		if seen[enc] {
			t.Errorf("重复编码: %d -> %s", id, enc)
		}
		seen[enc] = true
		if len(enc[2:]) < prevLen {
			t.Errorf("编码长度不应变短: %d(%.4s)", id, enc)
		}
		prevLen = len(enc[2:])
	}
}
