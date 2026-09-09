// Package wechat 微信公众号 JSSDK 签名（M2-H5-06）：
// 提供 jsapi_ticket 获取（Redis 缓存）与 wx.config 签名，支撑 H5 微信内分享卡片。
// 未配置 appId/secret 时签名接口返回未启用错误，前端静默降级为复制链接。
package wechat

import (
	"context"
	crand "crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dlidli/server/internal/pkg/errcode"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	ticketURL   = "https://api.weixin.qq.com/cgi-bin/ticket/getticket?type=jsapi&access_token=%s"
	tokenURL    = "https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=%s&secret=%s"
	ticketCache = "wx:jsapi_ticket"
	tokenCache  = "wx:access_token"
	cacheTTL    = 6600 * time.Second // 官方 7200s，提前 10 分钟过期防边界
)

// SignResult wx.config 所需签名参数。
type SignResult struct {
	AppID     string `json:"app_id"`
	Timestamp int64  `json:"timestamp"`
	NonceStr  string `json:"nonce_str"`
	Signature string `json:"signature"`
}

// Enabled 是否已配置公众号凭据。
func Enabled(cfgAppID string) bool { return cfgAppID != "" }

// Sign 生成指定 URL 的 wx.config 签名（appId/secret/redis 由 Service 持有）。
func (s *Service) Sign(ctx context.Context, pageURL string) (*SignResult, error) {
	if s.appID == "" || s.appSecret == "" {
		return nil, errcode.ErrForbidden.WithMsg("微信分享未启用")
	}
	ticket, err := s.ticket(ctx)
	if err != nil {
		return nil, err
	}
	ts := time.Now().Unix()
	nonce := randomNonce()
	// 官方算法：对 jsapi_ticket=...&noncestr=...&timestamp=...&url=... 做 SHA1
	pairs := []string{
		"jsapi_ticket=" + ticket,
		"noncestr=" + nonce,
		"timestamp=" + fmt.Sprintf("%d", ts),
		"url=" + pageURL,
	}
	sum := sha1.Sum([]byte(strings.Join(pairs, "&")))
	return &SignResult{
		AppID:     s.appID,
		Timestamp: ts,
		NonceStr:  nonce,
		Signature: hex.EncodeToString(sum[:]),
	}, nil
}

// ticket 取 jsapi_ticket（Redis 缓存，miss 时拉 access_token 再取 ticket）。
func (s *Service) ticket(ctx context.Context) (string, error) {
	if t, err := s.rdb.Get(ctx, ticketCache).Result(); err == nil && t != "" {
		return t, nil
	}
	token, err := s.accessToken(ctx)
	if err != nil {
		return "", err
	}
	res, err := httpGetJSON(fmt.Sprintf(ticketURL, token))
	if err != nil {
		return "", err
	}
	if code, _ := res["errcode"].(float64); code != 0 {
		return "", fmt.Errorf("获取 jsapi_ticket 失败: errcode=%v errmsg=%v", res["errcode"], res["errmsg"])
	}
	ticket, _ := res["ticket"].(string)
	if ticket == "" {
		return "", fmt.Errorf("jsapi_ticket 为空")
	}
	s.rdb.Set(ctx, ticketCache, ticket, cacheTTL)
	return ticket, nil
}

// accessToken 取 access_token（Redis 缓存）。
func (s *Service) accessToken(ctx context.Context) (string, error) {
	if t, err := s.rdb.Get(ctx, tokenCache).Result(); err == nil && t != "" {
		return t, nil
	}
	res, err := httpGetJSON(fmt.Sprintf(tokenURL, url.QueryEscape(s.appID), url.QueryEscape(s.appSecret)))
	if err != nil {
		return "", err
	}
	if code, _ := res["errcode"].(float64); code != 0 {
		return "", fmt.Errorf("获取 access_token 失败: errcode=%v errmsg=%v", res["errcode"], res["errmsg"])
	}
	token, _ := res["access_token"].(string)
	if token == "" {
		return "", fmt.Errorf("access_token 为空")
	}
	s.rdb.Set(ctx, tokenCache, token, cacheTTL)
	return token, nil
}

// httpGetJSON 简单 GET JSON（10s 超时）。
func httpGetJSON(raw string) (map[string]any, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(raw)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

// randomNonce 16 位随机串（wx.config nonceStr）。
func randomNonce() string {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 16)
	if _, err := crand.Read(b); err != nil {
		// 随机源失败时用时间兜底（nonce 仅防重放，非安全敏感）
		n := time.Now().UnixNano()
		for i := range b {
			b[i] = alphabet[int(n>>uint(i%8*8))%len(alphabet)]
		}
		return string(b)
	}
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b)
}

// Service 微信 JSSDK 服务。
type Service struct {
	appID     string
	appSecret string
	rdb       *redis.Client
	log       *zap.Logger
}

// NewService 构建服务；redis 未就绪时签名不可用（返回错误）。
func NewService(appID, appSecret string, rdb *redis.Client, log *zap.Logger) *Service {
	return &Service{appID: appID, appSecret: appSecret, rdb: rdb, log: log}
}
