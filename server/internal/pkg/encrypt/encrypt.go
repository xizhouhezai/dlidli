// Package encrypt 提供 PII 标识的可逆加密（AES-GCM）与确定性哈希（SHA-256）。
// 用途：手机号等登录标识的密文存储 + 精确查重/查询（哈希不可逆但可精确匹配）。
package encrypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

// ErrDecrypt 解密失败（密文损坏 / 密钥不匹配 / 非法输入）。
var ErrDecrypt = errors.New("解密失败")

// DeriveKey 由任意长度 secret 派生 32 字节 AES-256 密钥（SHA-256 定长）。
func DeriveKey(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

// Encrypt 用 AES-256-GCM 加密明文，返回 base64(nonce||ciphertext)。
// 随机 nonce 随密文拼接，解密时从密文前缀取出。
func Encrypt(key []byte, plaintext string) (string, error) {
	if len(key) != 32 {
		return "", fmt.Errorf("AES-256 需要 32 字节密钥, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt 解密 Encrypt 的输出；密文非法/被篡改/密钥不匹配返回 ErrDecrypt。
func Decrypt(key []byte, encoded string) (string, error) {
	if len(key) != 32 {
		return "", fmt.Errorf("AES-256 需要 32 字节密钥, got %d", len(key))
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrDecrypt, err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	ns := gcm.NonceSize()
	if len(raw) < ns {
		return "", ErrDecrypt
	}
	nonce, ciphertext := raw[:ns], raw[ns:]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrDecrypt, err)
	}
	return string(plain), nil
}

// IdentifierHash 标识的确定性哈希（SHA-256 hex），用于查重/精确查询。
// 含 identityType 前缀隔离不同类型标识，避免手机号与邮箱碰撞。
func IdentifierHash(identityType int8, identifier string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d:%s", identityType, identifier)))
	return hex.EncodeToString(sum[:])
}
