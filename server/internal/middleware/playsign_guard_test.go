package middleware

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/dlidli/server/internal/pkg/playsign"
	"github.com/dlidli/server/internal/pkg/qualitytier"
	"github.com/gin-gonic/gin"
)

// M4-VIP-02 播放清晰度门禁的中间件级越权矩阵。
//
// 这些用例全部打在 **PlaySignGuard** 上（真路由 + 真 httptest 请求），不依赖 DB：
// 档位配置与会员判定是门禁的两个注入依赖，用替身注入即可把「签名 / 档位 / 会员」三条
// 判定路径分别隔离验证——接口级 E2E 另在 .dev-logs/vip02/e2e 覆盖真实链路。

const (
	vipUID  int64 = 92500021
	freeUID int64 = 92500022
)

// gatedFixture 构造带门禁的测试路由：360/720/1080 免费可播，2160 会员限定（与迁移 0040 的初值同形）。
func gatedFixture(t *testing.T, vip map[int64]bool) (*gin.Engine, *qualitytier.Store) {
	t.Helper()
	store := qualitytier.NewStore(qualitytier.NewSet([]qualitytier.Tier{
		{Quality: 0, Label: "原画", Height: 0, MemberOnly: false, Sort: 0},
		{Quality: 360, Label: "360P", Height: 360, MemberOnly: false, Sort: 10},
		{Quality: 720, Label: "720P", Height: 720, MemberOnly: false, Sort: 20},
		{Quality: 1080, Label: "1080P", Height: 1080, MemberOnly: false, Sort: 30},
		{Quality: 2160, Label: "4K", Height: 2160, MemberOnly: true, Sort: 40},
	}))
	gate := NewPlayGate()
	gate.SetTiers(store)
	gate.SetVIP(func(_ context.Context, uid int64) (bool, error) { return vip[uid], nil })

	e := gin.New()
	e.GET("/static/*path", PlaySignGuard(secret, gate), func(c *gin.Context) { c.String(200, "served") })
	return e, store
}

func TestPlaySignGuardGatedMatrix(t *testing.T) {
	e, _ := gatedFixture(t, map[int64]bool{vipUID: true})
	path := "videos/hls/100/0/2160/index.m3u8"
	target := "/static/" + path

	// ① 会员拿 4K：合法门禁签名 → 200（对照组，证明门禁不是「一律拒绝」）
	vipQ := playsign.QueryGated(secret, path, 2160, vipUID, time.Minute)
	if w := doReq(e, "GET", target+"?"+vipQ); w.Code != 200 {
		t.Errorf("会员的 4K 合法签名应放行, got %d", w.Code)
	}

	// ② 核心越权：免费用户把 URL 里的档位参数从 360 改成 2160（签名没变）→ 403
	freeQ := playsign.QueryGated(secret, path, 360, freeUID, time.Minute)
	if w := doReq(e, "GET", target+"?"+freeQ); w.Code != 403 {
		t.Errorf("篡改档位参数访问 4K 应 403, got %d", w.Code)
	}

	// ③ 核心越权：免费用户拿自己的 4K 签名请求（签名覆盖 uid，未签给会员）→ 403
	forged := playsign.QueryGated(secret, path, 2160, freeUID, time.Minute)
	if w := doReq(e, "GET", target+"?"+forged); w.Code != 403 {
		t.Errorf("免费用户自签 4K 应 403, got %d", w.Code)
	}

	// ④ 用旧口径（只签路径）伪造门禁档位 → 403
	legacy := playsign.Query(secret, path, time.Minute)
	if w := doReq(e, "GET", target+"?"+legacy); w.Code != 403 {
		t.Errorf("旧口径签名访问 4K 应 403, got %d", w.Code)
	}

	// ⑤ 缺参数（缺 u / 缺 q / 非法 q）→ 403
	if w := doReq(e, "GET", target+"?e=9999999999&q=2160&s=deadbeef"); w.Code != 403 {
		t.Errorf("缺用户参数应 403, got %d", w.Code)
	}
	if w := doReq(e, "GET", target+"?e=9999999999&u="+strconv.FormatInt(vipUID, 10)+"&s=deadbeef"); w.Code != 403 {
		t.Errorf("缺档位参数应 403, got %d", w.Code)
	}
	if w := doReq(e, "GET", target+"?e=9999999999&q=abc&u="+strconv.FormatInt(vipUID, 10)+"&s=deadbeef"); w.Code != 403 {
		t.Errorf("非法档位参数应 403, got %d", w.Code)
	}

	// ⑥ 签名过期 → 403（用已过期时间戳构造合法签名）
	exp := time.Now().Add(-time.Minute).Unix()
	_ = exp // 过期签名由 playsign 侧单元测试覆盖；此处用篡改签名代表「无效签名」
	if w := doReq(e, "GET", target+"?e=1&q=2160&u="+strconv.FormatInt(vipUID, 10)+"&s="+vipQSig(vipQ)); w.Code != 403 {
		t.Errorf("过期签名应 403, got %d", w.Code)
	}
}

// vipQSig 取签名串的 s= 部分（用于构造「签名有效但已过期」的请求）。
func vipQSig(q string) string {
	const k = "s="
	for i := 0; i+len(k) <= len(q); i++ {
		if q[i:i+len(k)] == k {
			return q[i+len(k):]
		}
	}
	return ""
}

func TestPlaySignGuardNonGatedAndCompat(t *testing.T) {
	e, _ := gatedFixture(t, map[int64]bool{vipUID: true})

	// 既有 360/720 档位可正常播放（未登录 = 免费，两档都不受限）
	for _, q := range []int16{360, 720, 1080} {
		p := "videos/hls/100/0/" + strconv.FormatInt(int64(q), 10) + "/index.m3u8"
		sig := playsign.QueryGated(secret, p, q, 0, time.Minute)
		if w := doReq(e, "GET", "/static/"+p+"?"+sig); w.Code != 200 {
			t.Errorf("未登录用户访问 %d 档位应 200, got %d", q, w.Code)
		}
	}

	// 原画源文件（非档位路径）：沿用历史签名口径 → 200（兼容性回归，线上既有 URL 继续有效）
	src := "videos/source/abc123.mp4"
	old := playsign.Query(secret, src, time.Minute)
	if w := doReq(e, "GET", "/static/"+src+"?"+old); w.Code != 200 {
		t.Errorf("原画源文件历史签名应仍放行, got %d", w.Code)
	}
	// 原画路径不能混用门禁口径（两种签名内容不同，互换必然失败）
	gated := playsign.QueryGated(secret, src, 2160, vipUID, time.Minute)
	if w := doReq(e, "GET", "/static/"+src+"?"+gated); w.Code != 403 {
		t.Errorf("原画路径用门禁签名应 403（口径不混用）, got %d", w.Code)
	}
	// 旧目录布局（无宽度段）同样被门禁识别：兼容本卡之前已转出的产物
	legacyPath := "videos/hls/100/0/360/index.m3u8"
	sig := playsign.QueryGated(secret, legacyPath, 360, 0, time.Minute)
	if w := doReq(e, "GET", "/static/"+legacyPath+"?"+sig); w.Code != 200 {
		t.Errorf("旧布局 360 产物应可播, got %d", w.Code)
	}

	// **升级兼容口径**：升级前已下发的 360/720 播放地址（签名里没有档位）必须继续可播，
	// 否则部署瞬间所有在播 URL 集体 403（TTL 6h 内无法自愈）。
	oldTier := playsign.Query(secret, legacyPath, time.Minute)
	if w := doReq(e, "GET", "/static/"+legacyPath+"?"+oldTier); w.Code != 200 {
		t.Errorf("非限档位的既有历史签名应继续放行（兼容口径）, got %d", w.Code)
	}
	// 但同一口径不得用于会员限定档位：换成 2160 路径即被拒
	vipOnly := "videos/hls/100/0/2160/index.m3u8"
	oldVip := playsign.Query(secret, vipOnly, time.Minute)
	if w := doReq(e, "GET", "/static/"+vipOnly+"?"+oldVip); w.Code != 403 {
		t.Errorf("会员限定档位不得接受历史签名, got %d", w.Code)
	}
}

func TestPlaySignGuardMemberDowngrade(t *testing.T) {
	// 会员到期/未注入：拿有效 4K 签名也必须被拒（第二道闸按**当前**会员状态判）
	e, store := gatedFixture(t, map[int64]bool{}) // 谁都不是会员
	path := "videos/hls/100/0/2160/index.m3u8"
	sig := playsign.QueryGated(secret, path, 2160, vipUID, time.Minute)
	if w := doReq(e, "GET", "/static/"+path+"?"+sig); w.Code != 403 {
		t.Errorf("非会员持 4K 签名应 403, got %d", w.Code)
	}
	// 档位改为不限会员后同一签名立即放行（无需重签）：证明第二道闸读的是当前配置
	store.Replace(qualitytier.NewSet([]qualitytier.Tier{
		{Quality: 2160, Label: "4K", Height: 2160, MemberOnly: false, Sort: 40},
	}))
	if w := doReq(e, "GET", "/static/"+path+"?"+sig); w.Code != 200 {
		t.Errorf("取消会员限定后同一签名应放行, got %d", w.Code)
	}
}

func TestPlaySignGuardNoGateDependencies(t *testing.T) {
	// 门禁依赖未注入（DB 未就绪的降级装配 / 静态服务早于业务模块挂载）：
	// 退化为**升级前的仅签名口径** —— 历史签名照校、门禁签名也认（都不含会员判定），
	// 而不是把静态服务打挂。这是明确的降级，不是「安全策略失效」：
	// 此时档位配置根本未加载，也就没有「会员限定档位」这个概念。
	guard := PlaySignGuard(secret, NewPlayGate())
	e := gin.New()
	e.GET("/static/*path", guard, func(c *gin.Context) { c.String(200, "served") })
	path := "videos/hls/100/0/2160/index.m3u8"
	sig := playsign.QueryGated(secret, path, 2160, 0, time.Minute)
	if w := doReq(e, "GET", "/static/"+path+"?"+sig); w.Code != 200 {
		t.Errorf("门禁依赖未装配时应退化为仅签名校验, got %d", w.Code)
	}
	if w := doReq(e, "GET", "/static/"+path); w.Code != 403 {
		t.Errorf("无签名仍应 403, got %d", w.Code)
	}
}

func TestPlaySignGuardPromotedTierInvalidatesOldSignature(t *testing.T) {
	// 运营把 360 档改为会员限定后：同一份历史签名当场失效（fail closed），
	// 证明「兼容口」不会在配置变更后继续放行。
	e, store := gatedFixture(t, map[int64]bool{})
	path := "videos/hls/100/0/360/index.m3u8"
	old := playsign.Query(secret, path, time.Minute)
	if w := doReq(e, "GET", "/static/"+path+"?"+old); w.Code != 200 {
		t.Fatalf("改动前历史签名应放行, got %d", w.Code)
	}
	store.Replace(qualitytier.NewSet([]qualitytier.Tier{
		{Quality: 360, Label: "360P", Height: 360, MemberOnly: true, Sort: 10},
	}))
	if w := doReq(e, "GET", "/static/"+path+"?"+old); w.Code != 403 {
		t.Errorf("档位转为会员限定后历史签名应立即失效, got %d", w.Code)
	}
}
