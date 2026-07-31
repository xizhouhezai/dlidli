// Package bvid 稿件对外短 ID：DV + 雪花 ID 的 base62 编码，防遍历且可逆。
package bvid

import "strings"

const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// Encode 生成对外展示 ID。
func Encode(id int64) string {
	if id <= 0 {
		return ""
	}
	var sb strings.Builder
	n := uint64(id)
	for n > 0 {
		sb.WriteByte(alphabet[n%62])
		n /= 62
	}
	// 反转
	b := []byte(sb.String())
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
	return "DV" + string(b)
}
