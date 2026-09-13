package account

import (
	"context"
	"strings"
	"testing"

	"github.com/dlidli/server/internal/pkg/errcode"
)

// TestLoginByWeChatRequiresOpenID 空 openid 直接拒绝，不触库。
func TestLoginByWeChatRequiresOpenID(t *testing.T) {
	svc := &Service{}
	_, err := svc.LoginByWeChat(context.Background(), "", "")
	if err == nil {
		t.Fatal("空 openid 应报错")
	}
	if !strings.Contains(err.Error(), "openid") {
		t.Errorf("错误文案应包含 openid, got %v", err)
	}
}

// TestIdentityWeChatConstant 微信身份类型固定为 3（与 0029 迁移注释一致：
// 手机/微信凭据默认 activated=1，故微信账号无需激活流程）。
func TestIdentityWeChatConstant(t *testing.T) {
	if IdentityWeChat != 3 {
		t.Errorf("IdentityWeChat 应为 3, got %d", IdentityWeChat)
	}
	if IdentityPhone != 1 || IdentityEmail != 2 {
		t.Errorf("身份类型常量被意外改动: phone=%d email=%d", IdentityPhone, IdentityEmail)
	}
}

// TestWxLoginErrcodes 微信登录错误码已登记且文案明确（未启用 vs 授权失败需可区分）。
func TestWxLoginErrcodes(t *testing.T) {
	if errcode.ErrWxLoginDisabled == nil || errcode.ErrWxCodeInvalid == nil {
		t.Fatal("微信登录错误码未定义")
	}
	if !strings.Contains(errcode.ErrWxLoginDisabled.Error(), "未启用") {
		t.Errorf("未启用文案异常: %v", errcode.ErrWxLoginDisabled)
	}
	if errcode.ErrWxLoginDisabled.Code == errcode.ErrWxCodeInvalid.Code {
		t.Error("两个错误码不应相同")
	}
}
