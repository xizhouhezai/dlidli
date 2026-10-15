package playsign

import (
	"net/url"
	"strconv"
	"testing"
	"time"
)

const secret = "play-secret"

func TestQueryVerifyRoundTrip(t *testing.T) {
	resPath := "videos/hls/x/index.m3u8"
	q := Query(secret, resPath, time.Minute)
	vals, err := url.ParseQuery(q)
	if err != nil {
		t.Fatalf("解析签名串失败: %v", err)
	}
	expStr := vals.Get("e")
	sig := vals.Get("s")
	if expStr == "" || sig == "" {
		t.Fatalf("签名串缺少参数: %q", q)
	}
	if !Verify(secret, resPath, expStr, sig) {
		t.Error("合法签名应通过校验")
	}
}

func TestVerifyTampered(t *testing.T) {
	resPath := "videos/hls/x/index.m3u8"
	q := Query(secret, resPath, time.Minute)
	vals, _ := url.ParseQuery(q)
	expStr := vals.Get("e")
	sig := vals.Get("s")
	if Verify(secret, resPath+"2", expStr, sig) {
		t.Error("资源路径被篡改应校验失败")
	}
	if Verify(secret, resPath, expStr, "deadbeef") {
		t.Error("签名被篡改应校验失败")
	}
	if Verify("other-secret", resPath, expStr, sig) {
		t.Error("错误密钥应校验失败")
	}
}

func TestVerifyInvalidExp(t *testing.T) {
	q := Query(secret, "videos/hls/x/index.m3u8", time.Minute)
	vals, _ := url.ParseQuery(q)
	sig := vals.Get("s")
	if Verify(secret, "videos/hls/x/index.m3u8", "not-a-number", sig) {
		t.Error("非法过期时间应校验失败")
	}
	if Verify(secret, "videos/hls/x/index.m3u8", "", sig) {
		t.Error("空过期时间应校验失败")
	}
}

func TestVerifyExpired(t *testing.T) {
	resPath := "videos/hls/x/index.m3u8"
	exp := time.Now().Add(-time.Minute).Unix() // 已过期
	sig := compute(secret, resPath, exp)
	if Verify(secret, resPath, strconv.FormatInt(exp, 10), sig) {
		t.Error("已过期签名应校验失败")
	}
}

// ---- M4-VIP-02 门禁签名（档位 + 用户绑定）----

const gatedPath = "videos/hls/100/0/2160/index.m3u8"

func TestGatedRoundTrip(t *testing.T) {
	const uid int64 = 92500021
	q := QueryGated(secret, gatedPath, 2160, uid, time.Minute)
	vals, err := url.ParseQuery(q)
	if err != nil {
		t.Fatalf("解析门禁签名串失败: %v", err)
	}
	qual, ok := ParseQuality(vals.Get("q"))
	if !ok || qual != 2160 {
		t.Fatalf("档位参数应回带 2160, got %q ok=%v", vals.Get("q"), ok)
	}
	if u, _ := strconv.ParseInt(vals.Get("u"), 10, 64); u != uid {
		t.Fatalf("用户参数应回带 %d, got %q", uid, vals.Get("u"))
	}
	if !VerifyGated(secret, gatedPath, vals.Get("e"), qual, uid, vals.Get("s")) {
		t.Error("合法门禁签名应通过校验")
	}
}

// 核心越权用例：篡改档位参数（URL 改一个数字即越权）必须被拒。
func TestGatedTamperedQuality(t *testing.T) {
	const uid int64 = 92500021
	q := QueryGated(secret, "videos/hls/100/0/360/index.m3u8", 360, uid, time.Minute)
	vals, _ := url.ParseQuery(q)
	expStr, sig := vals.Get("e"), vals.Get("s")
	// 同一份签名串，改成 4K 档位 → 必须失败
	if VerifyGated(secret, "videos/hls/100/0/360/index.m3u8", expStr, 2160, uid, sig) {
		t.Error("篡改档位（360→2160）应校验失败")
	}
	// 换到 4K 的路径也不放行
	if VerifyGated(secret, "videos/hls/100/0/2160/index.m3u8", expStr, 2160, uid, sig) {
		t.Error("换路径 + 改档位应校验失败")
	}
}

// 核心越权用例：把会员拿到的 4K 签名串转给他人（换 uid）必须被拒。
func TestGatedTamperedUID(t *testing.T) {
	const vipUID, freeUID int64 = 92500021, 92500022
	q := QueryGated(secret, gatedPath, 2160, vipUID, time.Minute)
	vals, _ := url.ParseQuery(q)
	if VerifyGated(secret, gatedPath, vals.Get("e"), 2160, freeUID, vals.Get("s")) {
		t.Error("把他人签名串换个用户 ID 使用应校验失败（uid 已绑定进签名）")
	}
}

// 旧口径签名不得用于门禁档位（防「只签路径」绕过）。
func TestGatedRejectsLegacySignature(t *testing.T) {
	const uid int64 = 92500021
	legacy := Query(secret, gatedPath, time.Minute)
	vals, _ := url.ParseQuery(legacy)
	if VerifyGated(secret, gatedPath, vals.Get("e"), 2160, uid, vals.Get("s")) {
		t.Error("旧口径（仅路径）签名不得通过门禁档位校验")
	}
}

func TestGatedExpiredAndBadSig(t *testing.T) {
	const uid int64 = 92500021
	exp := time.Now().Add(-time.Minute).Unix()
	oldSig := computeGated(secret, gatedPath, exp, 2160, uid)
	if VerifyGated(secret, gatedPath, strconv.FormatInt(exp, 10), 2160, uid, oldSig) {
		t.Error("已过期门禁签名应校验失败")
	}
	q := QueryGated(secret, gatedPath, 2160, uid, time.Minute)
	vals, _ := url.ParseQuery(q)
	if VerifyGated(secret, gatedPath, vals.Get("e"), 2160, uid, "deadbeef") {
		t.Error("签名被篡改应校验失败")
	}
	if VerifyGated(secret, "other/path.m3u8", vals.Get("e"), 2160, uid, vals.Get("s")) {
		t.Error("路径被篡改应校验失败")
	}
	if _, ok := ParseQuality("abc"); ok {
		t.Error("非法档位参数应解析失败")
	}
	if _, ok := ParseQuality(""); ok {
		t.Error("缺失档位参数应解析失败")
	}
}
