package recommend

import "testing"

// TestDiversify 推荐打散：同 UP 只保留首个、同分区连续最多 3 个、整体顺序保持。
func TestDiversify(t *testing.T) {
	s := &Service{}

	// 空候选
	if got := s.diversify(nil); len(got) != 0 {
		t.Errorf("空候选应返回空, got %v", got)
	}

	// 同 UP 去重：后出现的同 UP 候选剔除
	cands := []candidate{
		{id: 1, cat: 1, up: 100},
		{id: 2, cat: 1, up: 100},
		{id: 3, cat: 2, up: 200},
	}
	if got := s.diversify(cands); len(got) != 2 || got[0] != 1 || got[1] != 3 {
		t.Errorf("同 UP 应只保留首个, got %v", got)
	}

	// 同分区连续上限：第 4 个同分区被剔除
	cands = []candidate{
		{id: 1, cat: 7, up: 1},
		{id: 2, cat: 7, up: 2},
		{id: 3, cat: 7, up: 3},
		{id: 4, cat: 7, up: 4},
		{id: 5, cat: 8, up: 5},
	}
	if got := s.diversify(cands); len(got) != 4 || got[3] != 5 {
		t.Errorf("同分区连续最多 3 个, got %v", got)
	}

	// 混合：UP 去重 + 分区打散 + 顺序保持
	cands = []candidate{
		{id: 1, cat: 1, up: 10},
		{id: 2, cat: 1, up: 20},
		{id: 3, cat: 1, up: 30},
		{id: 4, cat: 1, up: 10}, // 同 UP 剔除
		{id: 5, cat: 2, up: 40},
		{id: 6, cat: 1, up: 50}, // 同分区连续第 4 个，剔除
	}
	if got := s.diversify(cands); len(got) != 4 || got[0] != 1 || got[1] != 2 || got[2] != 3 || got[3] != 5 {
		t.Errorf("打散结果不符合预期, got %v", got)
	}
}
