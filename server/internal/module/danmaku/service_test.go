package danmaku

import "testing"

func TestSegKey(t *testing.T) {
	if got := segKey(42, 3); got != "dm:v:42:3" {
		t.Errorf("segKey(42,3) 应为 dm:v:42:3, got %q", got)
	}
}

// TestUserHash 确定性、区分度、掩码（不暴露真实 UID）。
func TestUserHash(t *testing.T) {
	s := &Service{secret: "test-secret"}
	h1 := s.UserHash(10001)
	h2 := s.UserHash(10002)
	if h1 == "" || len(h1) != 16 {
		t.Fatalf("哈希应为 16 字符截断, got %q", h1)
	}
	if h1 == h2 {
		t.Error("不同 UID 哈希不应相同")
	}
	if s.UserHash(10001) != h1 {
		t.Error("同 UID 哈希应确定（同 secret 下一致）")
	}
	if s.UserHash(0) != "" || s.UserHash(-1) != "" {
		t.Error("非法 UID 应返回空串")
	}
}

// TestFilterByBlocks 屏蔽过滤：关键词命中 / 发送者命中 / 顺序保持。
// 注意 filterByBlocks 原地复用输入切片，每个用例独立构造数据。
func TestFilterByBlocks(t *testing.T) {
	s := &Service{}
	mk := func() []Item {
		return []Item{
			{ID: "1", Content: "你好世界", SenderHash: "aaa"},
			{ID: "2", Content: "包含广告词的内容", SenderHash: "bbb"},
			{ID: "3", Content: "正常弹幕", SenderHash: "ccc"},
			{ID: "4", Content: "普通内容", SenderHash: "bbb"},
		}
	}

	// 无屏蔽：原样通过
	got := s.filterByBlocks(mk(), nil)
	if len(got) != 4 {
		t.Fatalf("无屏蔽应全量保留, got %d", len(got))
	}

	// 关键词屏蔽：命中内容的被剔除
	got = s.filterByBlocks(mk(), []DanmakuBlock{{BlockType: BlockKeyword, Keyword: "广告词"}})
	if len(got) != 3 || got[0].ID != "1" || got[1].ID != "3" || got[2].ID != "4" {
		t.Errorf("关键词命中应被剔除且顺序保持, got %+v", got)
	}

	// 用户屏蔽：发送者哈希命中的全部剔除（ID 2 与 4 均来自 bbb）
	got = s.filterByBlocks(mk(), []DanmakuBlock{{BlockType: BlockUser, BlockHash: "bbb"}})
	if len(got) != 2 || got[0].ID != "1" || got[1].ID != "3" {
		t.Errorf("被屏蔽数据来源应全部剔除, got %+v", got)
	}

	// 组合：关键词 + 用户
	blocks := []DanmakuBlock{
		{BlockType: BlockKeyword, Keyword: "广告词"},
		{BlockType: BlockUser, BlockHash: "aaa"},
	}
	got = s.filterByBlocks(mk(), blocks)
	if len(got) != 2 || got[0].ID != "3" || got[1].ID != "4" {
		t.Errorf("组合屏蔽应剩 2 条, got %+v", got)
	}

	// 空列表：不 panic
	if n := len(s.filterByBlocks([]Item{}, blocks)); n != 0 {
		t.Errorf("空输入应返回空, got %d", n)
	}
}
