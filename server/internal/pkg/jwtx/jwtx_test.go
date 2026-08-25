package jwtx

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-secret"

func TestGenerateParseRoundTrip(t *testing.T) {
	token, err := Generate(testSecret, 10001, time.Minute)
	if err != nil {
		t.Fatalf("Generate 失败: %v", err)
	}
	uid, err := Parse(testSecret, token)
	if err != nil {
		t.Fatalf("Parse 失败: %v", err)
	}
	if uid != 10001 {
		t.Errorf("Parse uid=%d, want 10001", uid)
	}
}

func TestAdminIsolation(t *testing.T) {
	adminToken, err := GenerateAdmin(testSecret, 7, time.Minute)
	if err != nil {
		t.Fatalf("GenerateAdmin 失败: %v", err)
	}
	// 管理员 token 不能被用户 Parse 使用
	if _, err := Parse(testSecret, adminToken); err == nil {
		t.Error("Parse 不应接受管理员 token")
	}
	// 管理员 token 能被 ParseAdmin 使用
	if id, err := ParseAdmin(testSecret, adminToken); err != nil || id != 7 {
		t.Errorf("ParseAdmin=%d err=%v, want 7 nil", id, err)
	}
	// 用户 token 不能被 ParseAdmin 使用
	userToken, _ := Generate(testSecret, 5, time.Minute)
	if _, err := ParseAdmin(testSecret, userToken); err == nil {
		t.Error("ParseAdmin 不应接受用户 token")
	}
}

func TestParseErrors(t *testing.T) {
	if _, err := Parse(testSecret, "not-a-token"); err == nil {
		t.Error("非法 token 应报错")
	}
}

func TestExpiredToken(t *testing.T) {
	claims := Claims{
		UserID: 1,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "dlidli",
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Minute)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute)), // 已过期
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("签名失败: %v", err)
	}
	if _, err := Parse(testSecret, token); err == nil {
		t.Error("已过期 token 应校验失败")
	}
}

func TestWrongSecret(t *testing.T) {
	token, err := Generate(testSecret, 9, time.Minute)
	if err != nil {
		t.Fatalf("Generate 失败: %v", err)
	}
	if _, err := Parse("other-secret", token); err == nil {
		t.Error("错误密钥应校验失败")
	}
}
