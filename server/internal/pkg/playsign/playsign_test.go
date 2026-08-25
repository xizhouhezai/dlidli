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
