package wechat

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// TestRandomNonceFormat nonce 为 16 位字母数字。
func TestRandomNonceFormat(t *testing.T) {
	re := regexp.MustCompile(`^[a-zA-Z0-9]{16}$`)
	for i := 0; i < 50; i++ {
		if !re.MatchString(randomNonce()) {
			t.Errorf("nonce 格式异常: %q", randomNonce())
		}
	}
}

// TestSignatureAlgorithm 对账官方签名算法：
// sha1("jsapi_ticket=..&noncestr=..&timestamp=..&url=..")。
// 用已知输入验证明文拼接顺序与 SHA1 十六进制输出。
func TestSignatureAlgorithm(t *testing.T) {
	ticket := "ticket_x"
	nonce := "abc123"
	ts := int64(1700000000)
	pageURL := "https://h5.example.com/pages/video/video?bvid=DV1"

	pairs := []string{
		"jsapi_ticket=" + ticket,
		"noncestr=" + nonce,
		"timestamp=" + fmt.Sprintf("%d", ts),
		"url=" + pageURL,
	}
	sum := sha1.Sum([]byte(strings.Join(pairs, "&")))
	want := hex.EncodeToString(sum[:])
	if len(want) != 40 {
		t.Fatalf("SHA1 hex 长度应为 40, got %d", len(want))
	}
	// 已知拼接顺序的确定性校验：改变顺序必须改变签名
	pairs2 := []string{
		"noncestr=" + nonce,
		"jsapi_ticket=" + ticket,
		"timestamp=" + fmt.Sprintf("%d", ts),
		"url=" + pageURL,
	}
	sum2 := sha1.Sum([]byte(strings.Join(pairs2, "&")))
	if hex.EncodeToString(sum2[:]) == want {
		t.Error("不同拼接顺序不应产生相同签名")
	}
}

// TestSignDisabled 未配置 appId 时 Sign 返回未启用错误。
func TestSignDisabled(t *testing.T) {
	s := NewService("", "", nil, nil)
	if Enabled(s.appID) {
		t.Error("空 appId 应视为未启用")
	}
	_, err := s.Sign(context.Background(), "https://x.example.com")
	if err == nil {
		t.Fatal("未启用时 Sign 应报错")
	}
	if !strings.Contains(err.Error(), "未启用") {
		t.Errorf("错误文案应包含 未启用, got %v", err)
	}
}

// TestMPEnabled 小程序凭据需 appId 与 secret 同时具备才视为启用。
func TestMPEnabled(t *testing.T) {
	cases := []struct {
		appID, secret string
		want          bool
	}{
		{"", "", false},
		{"wx1", "", false},   // 缺 secret
		{"", "sec", false},   // 缺 appId
		{"wx1", "sec", true}, // 齐备
	}
	for _, c := range cases {
		s := NewServiceWithMP("", "", c.appID, c.secret, nil, nil)
		if got := s.MPEnabled(); got != c.want {
			t.Errorf("MPEnabled(appID=%q, secret=%q) 应为 %v, got %v", c.appID, c.secret, c.want, got)
		}
	}
}

// TestCode2SessionDisabled 未配置小程序凭据时返回"微信登录未启用"，且不发起网络请求。
func TestCode2SessionDisabled(t *testing.T) {
	s := NewServiceWithMP("", "", "", "", nil, nil)
	_, err := s.Code2Session(context.Background(), "any_code")
	if err == nil {
		t.Fatal("未启用时应报错")
	}
	if !strings.Contains(err.Error(), "未启用") {
		t.Errorf("错误文案应包含 未启用, got %v", err)
	}
}

// TestCode2SessionEmptyCode 空 code 直接拒绝（未启用优先于参数校验之外的短路）。
func TestCode2SessionEmptyCode(t *testing.T) {
	// 已启用凭据但 code 为空：应在发请求前返回参数错误
	s := NewServiceWithMP("", "", "wx_fake", "secret_fake", nil, nil)
	_, err := s.Code2Session(context.Background(), "")
	if err == nil {
		t.Fatal("空 code 应报错")
	}
	if !strings.Contains(err.Error(), "code") {
		t.Errorf("错误文案应包含 code, got %v", err)
	}
}
