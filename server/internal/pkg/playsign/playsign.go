// Package playsign 为播放地址生成/校验 HMAC 签名（防盗链 + 时效控制 + 清晰度档位绑定）。
//
// 历史签名内容 = 资源相对路径（如 videos/hls/x/index.m3u8）+ 过期时间戳，
// 客户端拿到的播放 URL 形如 http://host/static/<path>?e=<exp>&s=<sig>。
//
// M4-VIP-02 起新增**门禁签名**（对会员限定档位强制使用）：
//
//	签名内容 = 路径 | 过期时间戳 | q=<档位> | u=<用户ID>
//
// 为什么必须把档位与用户都签进去（本卡的安全核心，plan monetization §2G.3）：
//   - 只签路径时，**改 URL 就完成越权**：把请求里的档位参数从 360 改成 2160，
//     路径与签名都没变，一个免费用户就能拿到 4K 产物；
//   - 因此门禁档位的签名必须覆盖档位号，档位被篡改 → 签名不匹配 → 403；
//   - 同时把**用户 ID** 绑进签名：否则免费用户可把会员拿到的 4K 签名串直接贴到自己的
//     URL 上（签名与档位都对，但它不是发给你的）——绑定 uid 后被转发即失效。
//
// 兼容口径（**已敲定**）：门禁签名是**新增函数**，历史调用点（原画/预览/非限档位）
// 的签名内容一字不变，故线上已有 URL 全部继续有效；反向也成立——
// 门禁档位一律走 `VerifyGated`，用旧口径（只签路径）伪造的签名在校验侧必然不匹配。
package playsign

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"time"
)

func compute(secret, resPath string, exp int64) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(resPath + "|" + strconv.FormatInt(exp, 10)))
	return hex.EncodeToString(mac.Sum(nil))
}

// Query 生成签名查询串 "e=<exp>&s=<sig>"，ttl 为有效期。
// 用于原画/预览等**非档位门禁**资源；会员限定档位必须用 QueryGated。
func Query(secret, resPath string, ttl time.Duration) string {
	exp := time.Now().Add(ttl).Unix()
	return "e=" + strconv.FormatInt(exp, 10) + "&s=" + compute(secret, resPath, exp)
}

// Verify 校验历史（仅路径）签名：过期或不匹配返回 false。
func Verify(secret, resPath, expStr, sig string) bool {
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil {
		return false
	}
	if time.Now().Unix() > exp {
		return false
	}
	want := compute(secret, resPath, exp)
	return hmac.Equal([]byte(want), []byte(sig))
}

// gatedContent 门禁签名的待签内容（档位 + 用户均在内）。
func gatedContent(resPath string, exp int64, quality int16, uid int64) string {
	return resPath + "|" + strconv.FormatInt(exp, 10) +
		"|q=" + strconv.FormatInt(int64(quality), 10) +
		"|u=" + strconv.FormatInt(uid, 10)
}

func computeGated(secret, resPath string, exp int64, quality int16, uid int64) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(gatedContent(resPath, exp, quality, uid)))
	return hex.EncodeToString(mac.Sum(nil))
}

// QueryGated 生成门禁签名查询串 "e=<exp>&q=<quality>&u=<uid>&s=<sig>"。
// 档位与用户一并回传，供 PlaySignGuard 校验时比对（缺任一即 403）。
func QueryGated(secret, resPath string, quality int16, uid int64, ttl time.Duration) string {
	exp := time.Now().Add(ttl).Unix()
	return "e=" + strconv.FormatInt(exp, 10) +
		"&q=" + strconv.FormatInt(int64(quality), 10) +
		"&u=" + strconv.FormatInt(uid, 10) +
		"&s=" + computeGated(secret, resPath, exp, quality, uid)
}

// ParseQuality 解析 URL 中的档位参数；缺失或非法返回 (0, false)。
func ParseQuality(qStr string) (int16, bool) {
	if qStr == "" {
		return 0, false
	}
	v, err := strconv.ParseInt(qStr, 10, 16)
	if err != nil {
		return 0, false
	}
	return int16(v), true
}

// VerifyGated 校验门禁签名：过期、档位不符或签名不匹配均返回 false。
// quality 由**调用方从请求参数解析后传入**——签名覆盖它，故篡改档位必然失败。
func VerifyGated(secret, resPath, expStr string, quality int16, uid int64, sig string) bool {
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil {
		return false
	}
	if time.Now().Unix() > exp {
		return false
	}
	want := computeGated(secret, resPath, exp, quality, uid)
	return hmac.Equal([]byte(want), []byte(sig))
}
