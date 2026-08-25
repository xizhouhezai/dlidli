package snowflake

import "testing"

func init() {
	if err := Init(1); err != nil {
		panic(err)
	}
}

func TestNextIDMonotonicAndUnique(t *testing.T) {
	prev := int64(0)
	seen := map[int64]bool{}
	for i := 0; i < 5000; i++ {
		id := NextID()
		if id <= prev {
			t.Errorf("ID 应单调递增: %d <= %d", id, prev)
		}
		if seen[id] {
			t.Errorf("ID 重复: %d", id)
		}
		seen[id] = true
		prev = id
	}
}

func TestNextIDPositive(t *testing.T) {
	for i := 0; i < 100; i++ {
		if id := NextID(); id <= 0 {
			t.Errorf("ID 应为正数: %d", id)
		}
	}
}

func TestEveryNodeUnique(t *testing.T) {
	// 不同 nodeID 应生成不同 ID 段（粗验多实例隔离）
	idA := NextID()
	if err := Init(2); err != nil {
		t.Fatalf("Init(2) 失败: %v", err)
	}
	idB := NextID()
	if idA == idB {
		t.Errorf("不同 node 不应产出相同 ID: %d", idA)
	}
	_ = Init(1) // 复位为测试用的 node
}
