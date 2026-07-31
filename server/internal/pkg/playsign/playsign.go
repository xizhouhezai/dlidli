// Package playsign 为播放地址生成/校验 HMAC 签名（防盗链 + 时效控制）。
// 签名内容 = 资源相对路径（如 videos/hls/x/index.m3u8）+ 过期时间戳，
// 客户端拿到的播放 URL 形如 http://host/static/<path>?e=<exp>&s=<sig>。
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
func Query(secret, resPath string, ttl time.Duration) string {
	exp := time.Now().Add(ttl).Unix()
	return "e=" + strconv.FormatInt(exp, 10) + "&s=" + compute(secret, resPath, exp)
}

// Verify 校验签名：过期或不匹配返回 false。
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
