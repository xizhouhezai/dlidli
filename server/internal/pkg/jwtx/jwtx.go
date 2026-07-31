// Package jwtx 封装 access token 的签发与校验。
package jwtx

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID int64 `json:"uid"`
	// Admin 为 true 表示后台管理员令牌（与 C 端用户令牌互不通用）
	Admin bool `json:"adm,omitempty"`
	jwt.RegisteredClaims
}

// Generate 签发 access token。
func Generate(secret string, userID int64, ttl time.Duration) (string, error) {
	return generate(secret, userID, false, ttl)
}

// GenerateAdmin 签发后台管理员 token。
func GenerateAdmin(secret string, adminID int64, ttl time.Duration) (string, error) {
	return generate(secret, adminID, true, ttl)
}

func generate(secret string, id int64, admin bool, ttl time.Duration) (string, error) {
	claims := Claims{
		UserID: id,
		Admin:  admin,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "dlidli",
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

// Parse 校验并解析用户 token，返回用户 ID（拒绝管理员令牌）。
func Parse(secret, token string) (int64, error) {
	claims, err := parse(secret, token)
	if err != nil {
		return 0, err
	}
	if claims.Admin {
		return 0, errors.New("令牌类型不匹配")
	}
	return claims.UserID, nil
}

// ParseAdmin 校验并解析管理员 token。
func ParseAdmin(secret, token string) (int64, error) {
	claims, err := parse(secret, token)
	if err != nil {
		return 0, err
	}
	if !claims.Admin {
		return 0, errors.New("令牌类型不匹配")
	}
	return claims.UserID, nil
}

func parse(secret, token string) (*Claims, error) {
	var claims Claims
	t, err := jwt.ParseWithClaims(token, &claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("非预期的签名算法")
		}
		return []byte(secret), nil
	})
	if err != nil || !t.Valid {
		return nil, errors.New("token 无效或已过期")
	}
	return &claims, nil
}
