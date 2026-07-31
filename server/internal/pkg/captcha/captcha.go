// Package captcha 图形验证码：纯 Go 生成 SVG（无外部依赖），Redis 存储一次性校验。
package captcha

import (
	"context"
	crand "crypto/rand"
	"encoding/binary"
	"fmt"
	"strings"
	"time"

	"github.com/dlidli/server/internal/pkg/snowflake"
	"github.com/redis/go-redis/v9"
)

const (
	ttl     = 5 * time.Minute
	keyPfx  = "captcha:"
	codeLen = 4
	chars   = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // 去除易混字符
	imgW    = 120
	imgH    = 40
)

// Result 验证码生成结果。
type Result struct {
	ID  string `json:"id"`  // 验证码 ID（前端回传校验）
	SVG string `json:"svg"` // 内联 SVG 图片
}

// Generate 生成验证码：返回 ID + SVG 图片，答案存 Redis。
func Generate(ctx context.Context, rdb *redis.Client) (*Result, error) {
	code := randomCode()
	id := fmt.Sprintf("%d", snowflake.NextID())
	if err := rdb.Set(ctx, keyPfx+id, strings.ToLower(code), ttl).Err(); err != nil {
		return nil, err
	}
	return &Result{ID: id, SVG: renderSVG(code)}, nil
}

// Verify 校验验证码（一次性，校验后删除）。
func Verify(ctx context.Context, rdb *redis.Client, id, code string) bool {
	if id == "" || code == "" {
		return false
	}
	key := keyPfx + id
	saved, err := rdb.Get(ctx, key).Result()
	if err != nil {
		return false
	}
	_ = rdb.Del(ctx, key).Err() // 一次性
	return saved == strings.ToLower(code)
}

func randomCode() string {
	var sb strings.Builder
	for i := 0; i < codeLen; i++ {
		sb.WriteByte(chars[randInt(len(chars))])
	}
	return sb.String()
}

// randInt 使用 crypto/rand 生成 [0, n) 的安全随机整数。
func randInt(n int) int {
	var buf [4]byte
	_, _ = crand.Read(buf[:])
	return int(binary.BigEndian.Uint32(buf[:]) % uint32(n))
}

// renderSVG 生成带干扰线的 SVG 验证码图片。
func renderSVG(code string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`, imgW, imgH, imgW, imgH))
	sb.WriteString(`<rect width="100%" height="100%" fill="#f5f5f5"/>`)

	// 干扰线
	for i := 0; i < 4; i++ {
		x1, y1 := randInt(imgW), randInt(imgH)
		x2, y2 := randInt(imgW), randInt(imgH)
		color := fmt.Sprintf("#%02x%02x%02x", randInt(180), randInt(180), randInt(180))
		sb.WriteString(fmt.Sprintf(`<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="%s" stroke-width="1"/>`, x1, y1, x2, y2, color))
	}

	// 字符
	for i, ch := range code {
		x := 15 + i*26 + randInt(6)
		y := 24 + randInt(8)
		rotate := randInt(30) - 15
		color := fmt.Sprintf("#%02x%02x%02x", randInt(100), randInt(100), randInt(100)+80)
		sb.WriteString(fmt.Sprintf(`<text x="%d" y="%d" font-size="22" font-family="monospace" fill="%s" transform="rotate(%d %d %d)">%c</text>`,
			x, y, color, rotate, x, y, ch))
	}

	sb.WriteString(`</svg>`)
	return sb.String()
}
