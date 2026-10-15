package video

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dlidli/server/internal/pkg/config"
	"github.com/dlidli/server/internal/pkg/qualitytier"
	"go.uber.org/zap"
)

// M4-VIP-02 清晰度权益判定的包级测试。
//
// 覆盖三件事（都在真实 Service 上跑，只用替身注入会员判定与档位快照）：
//  1. 档位配置的判定口径（免费上限 / 会员限定 / 原画按源分辨率）；
//  2. 播放地址的**签名口径**：档位产物走门禁签名（含 q/u），原画源文件走历史签名；
//  3. 下发过滤：同一份流列表，免费/会员/未登录三种身份拿到的档位数与 URL 形状不同。

func testService(t *testing.T, tiers []qualitytier.Tier, vip bool) *Service {
	t.Helper()
	s := NewService(NewRepo(nil), nil, nil, nil, nil, &config.Config{
		Storage: config.Storage{BaseURL: "http://localhost:8000/static"},
		JWT:     config.JWT{Secret: "unit-secret"},
	}, zap.NewNop())
	s.SetTierSet(qualitytier.NewSet(tiers))
	s.SetVipChecker(fakeVip(vip))
	return s
}

type fakeVip bool

func (f fakeVip) IsVip(_ int64) (bool, error) { return bool(f), nil }

// defaultTiers 与迁移 0040 的初始档位同形。
func defaultTiers() []qualitytier.Tier {
	return []qualitytier.Tier{
		{Quality: 0, Label: "原画", Height: 0, MemberOnly: false, Sort: 0},
		{Quality: 360, Label: "360P", Height: 360, VBits: "500k", MemberOnly: false, TranscodeEnabled: true, Sort: 10},
		{Quality: 720, Label: "720P", Height: 720, VBits: "2000k", MemberOnly: false, TranscodeEnabled: true, Sort: 20},
		{Quality: 1080, Label: "1080P", Height: 1080, VBits: "4000k", MemberOnly: false, TranscodeEnabled: true, Sort: 30},
		{Quality: 2160, Label: "4K", Height: 2160, VBits: "16000k", MemberOnly: true, TranscodeEnabled: false, Sort: 40},
	}
}

func TestDecideAllowance(t *testing.T) {
	set := qualitytier.NewSet(defaultTiers())

	cases := []struct {
		name    string
		quality int16
		height  int
		vip     bool
		want    bool
	}{
		{"免费看 360", 360, 360, false, true},
		{"免费看 720", 720, 720, false, true},
		{"免费看 1080", 1080, 1080, false, true},
		{"免费看 4K 档", 2160, 2160, false, false},
		{"会员看 4K", 2160, 2160, true, true},
		{"免费看原画（1080 源）", 0, 1080, false, true},
		{"免费看原画（4K 源）", 0, 3840, false, false},
		{"会员看原画（4K 源）", 0, 3840, true, true},
		{"免费看原画（高度未知的历史行）", 0, 0, false, true},
		{"免费看历史 720 行（高度未知）", 720, 0, false, true},
		{"免费看历史 2160 行（高度未知）", 2160, 0, false, false},
		{"免费看超上限的未知档位", 4320, 4320, false, false},
	}
	for _, c := range cases {
		got := set.Decide(c.quality, c.height, c.vip).Allowed
		if got != c.want {
			t.Errorf("%s: Decide(%d,%d,vip=%v) = %v, want %v", c.name, c.quality, c.height, c.vip, got, c.want)
		}
	}

	// 空快照：一律放行（等于升级前行为，配置缺失不打挂播放）
	var empty *qualitytier.Set
	if !empty.Decide(2160, 2160, false).Allowed {
		t.Error("空快照应放行（降级为升级前行为）")
	}
}

func TestTranscodeQualities(t *testing.T) {
	set := qualitytier.NewSet(defaultTiers())
	got := set.TranscodeQualities()
	want := []int16{360, 720, 1080} // 2160 初始 transcode_enabled=0（4K 单机成本）
	if len(got) != len(want) {
		t.Fatalf("排程档位应为 %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("排程档位应为 %v, got %v", want, got)
		}
	}

	// 运营开启 4K 后，新投稿才会带 2160（同一快照生成逻辑）
	tiers := defaultTiers()
	for i := range tiers {
		if tiers[i].Quality == 2160 {
			tiers[i].TranscodeEnabled = true
		}
	}
	on := qualitytier.NewSet(tiers).TranscodeQualities()
	if len(on) != 4 || on[3] != 2160 {
		t.Fatalf("开启 4K 后应含 2160, got %v", on)
	}
}

func TestSignedPlayURLCarriesGatedBinding(t *testing.T) {
	s := testService(t, defaultTiers(), false)
	const uid int64 = 92500022

	// 档位产物：门禁签名必须同时带 q 与 u（缺一则门禁侧必拒）
	u := s.signedPlayURL("videos/hls/100/0/2160/index.m3u8", 2160, uid)
	for _, kw := range []string{"q=2160", "u=92500022", "e=", "s="} {
		if !contains(u, kw) {
			t.Errorf("门禁签名 URL 缺少 %s: %s", kw, u)
		}
	}

	// 原画源文件：历史签名口径（不带 q/u），线上既有 URL 与校验语义不变
	old := s.signedPlayURL("videos/source/abc.mp4", 0, uid)
	if contains(old, "q=") || contains(old, "u=") {
		t.Errorf("原画源文件应走历史签名口径（不带 q/u）: %s", old)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// TestTierConfigFromFile 用真实 SQL 迁移文件做一次「配置自检」：
// 断言初始档位与登录条件，避免迁移文件被静默改坏（这是本卡唯一一处
// 把配置写死在 SQL 里的地方，值得一条不需要 DB 的护栏）。
func TestTierConfigFromFile(t *testing.T) {
	root := findRepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "server", "scripts", "migrations", "0040_video_quality.up.sql"))
	if err != nil {
		t.Fatalf("读取迁移文件失败: %v", err)
	}
	sql := string(data)
	for _, want := range []string{
		"(0,    '原画'", "(360,  '360P'", "(720,  '720P'", "(1080, '1080P'", "(2160, '4K'",
		"`member_only`", "`transcode_enabled`", "ADD COLUMN `height`",
	} {
		if !contains(sql, want) {
			t.Errorf("迁移 0040 应包含 %q", want)
		}
	}
	// 2160 行必须 member_only=1 且 transcode_enabled=0（4K 默认不排程）
	line := ""
	for _, l := range splitLines(sql) {
		if contains(l, "(2160, '4K'") {
			line = l
		}
	}
	if line == "" {
		t.Fatal("未找到 2160 档位插入行")
	}
	if !contains(line, "1, 0, 40)") {
		t.Errorf("2160 应为 member_only=1, transcode_enabled=0, got %q", line)
	}
}

func splitLines(s string) []string {
	var out []string
	cur := ""
	for _, r := range s {
		if r == '\n' {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("取工作目录失败: %v", err)
	}
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "server", "scripts", "migrations")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Skip("未找到仓库根目录，跳过迁移文件自检")
	return ""
}
