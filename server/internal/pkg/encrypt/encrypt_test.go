package encrypt

import (
	"strings"
	"testing"
)

func TestDeriveKey(t *testing.T) {
	k := DeriveKey("secret")
	if len(k) != 32 {
		t.Errorf("密钥应为 32 字节, got %d", len(k))
	}
	if string(k) == string(DeriveKey("other")) {
		t.Error("不同 secret 派生不同密钥")
	}
	if string(k) != string(DeriveKey("secret")) {
		t.Error("同 secret 派生密钥应确定")
	}
}

func TestEncryptDecrypt(t *testing.T) {
	key := DeriveKey("test-secret")
	plain := "13800138000"
	c, err := Encrypt(key, plain)
	if err != nil {
		t.Fatalf("Encrypt 失败: %v", err)
	}
	if c == plain {
		t.Error("密文不应等于明文")
	}
	if strings.Contains(c, plain) {
		t.Error("密文不应包含明文")
	}
	// 两次加密密文不同（随机 nonce）
	c2, _ := Encrypt(key, plain)
	if c == c2 {
		t.Error("同明文两次加密密文应不同（随机 nonce）")
	}
	// 解密还原
	d, err := Decrypt(key, c)
	if err != nil {
		t.Fatalf("Decrypt 失败: %v", err)
	}
	if d != plain {
		t.Errorf("解密应还原明文, got %q", d)
	}
}

func TestDecryptWrongKey(t *testing.T) {
	c, _ := Encrypt(DeriveKey("key-a"), "12345678901")
	if _, err := Decrypt(DeriveKey("key-b"), c); err == nil {
		t.Error("错误密钥应解密失败")
	}
}

func TestDecryptMalformed(t *testing.T) {
	key := DeriveKey("test-secret")
	if _, err := Decrypt(key, "!@#$%^&*"); err == nil {
		t.Error("非法 base64 应解密失败")
	}
	if _, err := Decrypt(key, ""); err == nil {
		t.Error("空串应解密失败")
	}
}

func TestIdentifierHash(t *testing.T) {
	// 确定性：同类型同标识同哈希
	if IdentifierHash(1, "13800138000") != IdentifierHash(1, "13800138000") {
		t.Error("同标识哈希应确定")
	}
	// 不同类型隔离：手机号 1 与邮箱 2 的 1 不应碰撞
	if IdentifierHash(1, "13800138000") == IdentifierHash(2, "13800138000") {
		t.Error("不同类型同串不应碰撞")
	}
	// 不同标识不同哈希
	if IdentifierHash(1, "13800138000") == IdentifierHash(1, "13800138001") {
		t.Error("不同手机号哈希不应相同")
	}
	// 长度 = SHA-256 hex = 64
	if got := IdentifierHash(1, "13800138000"); len(got) != 64 {
		t.Errorf("哈希长度应为 64, got %d", len(got))
	}
	// 不含明文
	if strings.Contains(IdentifierHash(1, "13800138000"), "138") {
		t.Error("哈希不应含明文片段")
	}
}
