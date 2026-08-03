package account

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/dlidli/server/internal/module/growth"
	"github.com/dlidli/server/internal/pkg/captcha"
	"github.com/dlidli/server/internal/pkg/config"
	"github.com/dlidli/server/internal/pkg/errcode"
	"github.com/dlidli/server/internal/pkg/jwtx"
	"github.com/dlidli/server/internal/pkg/snowflake"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

const (
	smsCodeTTL      = 5 * time.Minute
	smsSendCooldown = 60 * time.Second
	refreshTTL      = 30 * 24 * time.Hour
	loginLockTTL    = 15 * time.Minute
	maxLoginFails   = 5
)

type Service struct {
	repo      *Repo
	rdb       *redis.Client
	cfg       *config.Config
	log       *zap.Logger
	growthSvc *growth.Service
}

func NewService(repo *Repo, rdb *redis.Client, cfg *config.Config, log *zap.Logger, growthSvc *growth.Service) *Service {
	return &Service{repo: repo, rdb: rdb, cfg: cfg, log: log, growthSvc: growthSvc}
}

// SendSmsCode 发送短信验证码。当前为 mock 实现：仅写入 Redis 并打日志；
// dev 环境额外返回 debugCode 便于联调（M1-ACC-01 接入真实短信服务后移除）。
func (s *Service) SendSmsCode(ctx context.Context, phone string) (debugCode string, err error) {
	cooldownKey := "sms:cd:" + phone
	ok, err := s.rdb.SetNX(ctx, cooldownKey, 1, smsSendCooldown).Result()
	if err != nil {
		return "", err
	}
	if !ok {
		return "", errcode.ErrSmsTooFrequent
	}

	code := randomDigits(6)
	if err := s.rdb.Set(ctx, "sms:code:"+phone, code, smsCodeTTL).Err(); err != nil {
		return "", err
	}
	s.log.Info("mock 短信验证码", zap.String("phone", phone), zap.String("code", code))

	if s.cfg.App.Env == "dev" {
		return code, nil
	}
	return "", nil
}

// LoginBySms 验证码登录：未注册手机号自动注册。
func (s *Service) LoginBySms(ctx context.Context, phone, code string) (*TokenPair, error) {
	key := "sms:code:" + phone
	saved, err := s.rdb.Get(ctx, key).Result()
	if err == redis.Nil || saved != code {
		return nil, errcode.ErrSmsCodeInvalid
	}
	if err != nil && err != redis.Nil {
		return nil, err
	}
	_ = s.rdb.Del(ctx, key).Err() // 一次性使用

	auth, err := s.repo.FindAuth(IdentityPhone, phone)
	if err != nil {
		return nil, err
	}

	var user *User
	if auth == nil {
		// 自动注册（注册礼：5 硬币）
		user = &User{
			ID:       snowflake.NextID(),
			Nickname: defaultNickname(),
			Level:    1, // 手机注册即 Lv1（可发弹幕）
			Coin:     5,
		}
		if err := s.repo.CreateUserWithAuth(user, &UserAuth{
			IdentityType: IdentityPhone,
			Identifier:   phone, // TODO(M1-ACC): 加密存储
		}); err != nil {
			return nil, err
		}
	} else {
		user, err = s.repo.FindUserByID(auth.UserID)
		if err != nil {
			return nil, err
		}
	}
	return s.issueTokens(ctx, user)
}

// GenerateCaptcha 生成图形验证码。
func (s *Service) GenerateCaptcha(ctx context.Context) (*captcha.Result, error) {
	return captcha.Generate(ctx, s.rdb)
}

// LoginByPassword 密码登录：account 为手机号或邮箱。
func (s *Service) LoginByPassword(ctx context.Context, account, password, captchaID, captchaCode string) (*TokenPair, error) {
	// 图形验证码校验
	if !captcha.Verify(ctx, s.rdb, captchaID, captchaCode) {
		return nil, errcode.ErrInvalidParams.WithMsg("验证码错误或已过期")
	}

	lockKey := "login:lock:" + account
	if n, _ := s.rdb.Get(ctx, lockKey).Int(); n >= maxLoginFails {
		return nil, errcode.ErrLoginLocked
	}

	identityType := int8(IdentityPhone)
	if strings.Contains(account, "@") {
		identityType = IdentityEmail
	}
	auth, err := s.repo.FindAuth(identityType, account)
	if err != nil {
		return nil, err
	}
	if auth == nil || auth.Credential == "" ||
		bcrypt.CompareHashAndPassword([]byte(auth.Credential), []byte(password)) != nil {
		// 累计失败次数
		n, _ := s.rdb.Incr(ctx, lockKey).Result()
		if n == 1 {
			_ = s.rdb.Expire(ctx, lockKey, loginLockTTL).Err()
		}
		return nil, errcode.ErrPasswordMismatch
	}
	_ = s.rdb.Del(ctx, lockKey).Err()

	user, err := s.repo.FindUserByID(auth.UserID)
	if err != nil {
		return nil, err
	}
	return s.issueTokens(ctx, user)
}

// Refresh 刷新令牌（轮换：旧 refresh 失效）。
func (s *Service) Refresh(ctx context.Context, refreshToken string) (*TokenPair, error) {
	key := "sess:" + refreshToken
	uid, err := s.rdb.Get(ctx, key).Int64()
	if err == redis.Nil {
		return nil, errcode.ErrRefreshInvalid
	}
	if err != nil {
		return nil, err
	}
	_ = s.rdb.Del(ctx, key).Err()

	user, err := s.repo.FindUserByID(uid)
	if err != nil {
		return nil, err
	}
	return s.issueTokens(ctx, user)
}

// Logout 注销当前会话。
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	return s.rdb.Del(ctx, "sess:"+refreshToken).Err()
}

// Me 获取当前用户资料。
func (s *Service) Me(_ context.Context, uid int64) (*Profile, error) {
	user, err := s.repo.FindUserByID(uid)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errcode.ErrAccountNotExists
	}
	p := toProfile(user)
	return &p, nil
}

// Briefs 批量获取用户简况（供其他模块拼装读模型，如稿件卡片的 UP 主信息）。
func (s *Service) Briefs(_ context.Context, ids []int64) (map[int64]Profile, error) {
	users, err := s.repo.FindUsersByIDs(ids)
	if err != nil {
		return nil, err
	}
	m := make(map[int64]Profile, len(users))
	for i := range users {
		m[users[i].ID] = toProfile(&users[i])
	}
	return m, nil
}

// ChangePassword 修改密码；从未设置过密码（短信注册）时旧密码可留空。
func (s *Service) ChangePassword(_ context.Context, uid int64, oldPwd, newPwd string) error {
	if len(newPwd) < 8 || len(newPwd) > 32 {
		return errcode.ErrInvalidParams.WithMsg("新密码长度需 8~32 位")
	}
	auth, err := s.repo.FindAuthByUser(uid, IdentityPhone)
	if err != nil {
		return err
	}
	if auth == nil {
		return errcode.ErrAccountNotExists
	}
	if auth.Credential != "" &&
		bcrypt.CompareHashAndPassword([]byte(auth.Credential), []byte(oldPwd)) != nil {
		return errcode.ErrPasswordMismatch.WithMsg("旧密码不正确")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPwd), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.repo.UpdateCredentialByUser(uid, IdentityPhone, string(hash))
}

// ResetPassword 忘记密码：短信验证码重置（同时解除登录错误锁定）。
func (s *Service) ResetPassword(ctx context.Context, phone, code, newPwd string) error {
	if len(newPwd) < 8 || len(newPwd) > 32 {
		return errcode.ErrInvalidParams.WithMsg("新密码长度需 8~32 位")
	}
	key := "sms:code:" + phone
	saved, err := s.rdb.Get(ctx, key).Result()
	if err == redis.Nil || saved != code {
		return errcode.ErrSmsCodeInvalid
	}
	if err != nil && err != redis.Nil {
		return err
	}
	_ = s.rdb.Del(ctx, key).Err() // 一次性使用

	auth, err := s.repo.FindAuth(IdentityPhone, phone)
	if err != nil {
		return err
	}
	if auth == nil {
		return errcode.ErrAccountNotExists
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPwd), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if err := s.repo.UpdateCredentialByUser(auth.UserID, IdentityPhone, string(hash)); err != nil {
		return err
	}
	_ = s.rdb.Del(ctx, "login:lock:"+phone).Err()
	return nil
}

// SearchUsers 昵称搜索（供 search 模块）。
func (s *Service) SearchUsers(_ context.Context, keyword string, page, size int) ([]Profile, int64, error) {
	users, total, err := s.repo.SearchByNickname(keyword, page, size)
	if err != nil {
		return nil, 0, err
	}
	profiles := make([]Profile, 0, len(users))
	for i := range users {
		profiles = append(profiles, toProfile(&users[i]))
	}
	return profiles, total, nil
}

// IsNewUser 判断用户注册是否不足 days 天（机审风险分级等风控场景用）。
func (s *Service) IsNewUser(_ context.Context, uid int64, days int) (bool, error) {
	user, err := s.repo.FindUserByID(uid)
	if err != nil {
		return false, err
	}
	if user == nil {
		return false, errcode.ErrAccountNotExists
	}
	return time.Since(user.CreatedAt) < time.Duration(days)*24*time.Hour, nil
}

// GetYouthMode 青少年模式状态（M2-AUD-04）。
func (s *Service) GetYouthMode(_ context.Context, uid int64) (bool, error) {
	user, err := s.repo.FindUserByID(uid)
	if err != nil {
		return false, err
	}
	if user == nil {
		return false, errcode.ErrAccountNotExists
	}
	return user.YouthMode == 1, nil
}

// SetYouthMode 开关青少年模式（M2-AUD-04）。
func (s *Service) SetYouthMode(_ context.Context, uid int64, on bool) error {
	v := int8(0)
	if on {
		v = 1
	}
	return s.repo.UpdateUserFields(uid, map[string]any{"youth_mode": v})
}

// SpendCoins 消耗硬币（投币等场景，余额不足报错）。
func (s *Service) SpendCoins(_ context.Context, uid int64, count int, reason string) error {
	if count <= 0 {
		return errcode.ErrInvalidParams
	}
	ok, err := s.repo.AddCoins(uid, -count, reason)
	if err != nil {
		return err
	}
	if !ok {
		return errcode.ErrCoinNotEnough
	}
	return nil
}

// GrantCoins 发放硬币（奖励场景）。
func (s *Service) GrantCoins(_ context.Context, uid int64, count int, reason string) error {
	if count <= 0 {
		return errcode.ErrInvalidParams
	}
	_, err := s.repo.AddCoins(uid, count, reason)
	return err
}

// coinReasonNames 硬币流水来源文案（未登记回退原样）。
var coinReasonNames = map[string]string{
	"daily_login": "每日登录",
	"coin_video": "投币",
	"coin_refund": "投币退款",
}

// CoinLogItem 硬币流水项（ID 字符串化避免 JS 精度丢失）。
type CoinLogItem struct {
	ID         string    `json:"id"`
	Delta      int       `json:"delta"`
	Reason     string    `json:"reason"`
	ReasonName string    `json:"reason_name"`
	CreatedAt  time.Time `json:"created_at"`
}

// CoinLogs 硬币流水分页（M2-GRW-03 硬币明细页）。
func (s *Service) CoinLogs(_ context.Context, uid int64, page, size int) ([]CoinLogItem, int64, error) {
	list, total, err := s.repo.ListCoinLogs(uid, page, size)
	if err != nil {
		return nil, 0, err
	}
	items := make([]CoinLogItem, 0, len(list))
	for _, l := range list {
		name := l.Reason
		if v, ok := coinReasonNames[l.Reason]; ok {
			name = v
		}
		items = append(items, CoinLogItem{
			ID: fmt.Sprintf("%d", l.ID), Delta: l.Delta, Reason: l.Reason,
			ReasonName: name, CreatedAt: l.CreatedAt,
		})
	}
	return items, total, nil
}

func (s *Service) issueTokens(ctx context.Context, user *User) (*TokenPair, error) {
	if user == nil {
		return nil, errcode.ErrAccountNotExists
	}
	if err := s.resolveBanOnLogin(user); err != nil {
		return nil, err
	}

	// 每日首次登录 +1 硬币（ACC-21；完整每日任务体系随 M2-GRW）
	dailyKey := fmt.Sprintf("coin:daily:%d:%s", user.ID, time.Now().Format("2006-01-02"))
	if ok, _ := s.rdb.SetNX(ctx, dailyKey, 1, 48*time.Hour).Result(); ok {
		if _, err := s.repo.AddCoins(user.ID, 1, "daily_login"); err == nil {
			user.Coin++
		}
	}
	// 每日首次登录 +5 经验（M2-GRW-01）
	if s.growthSvc != nil {
		s.growthSvc.AddExpOncePerDay(ctx, user.ID, growth.ReasonDailyLogin)
	}
	accessTTL := time.Duration(s.cfg.JWT.AccessTTLMin) * time.Minute
	access, err := jwtx.Generate(s.cfg.JWT.Secret, user.ID, accessTTL)
	if err != nil {
		return nil, err
	}

	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return nil, err
	}
	refresh := hex.EncodeToString(buf)
	if err := s.rdb.Set(ctx, "sess:"+refresh, user.ID, refreshTTL).Err(); err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    int(accessTTL.Seconds()),
		User:         toProfile(user),
	}, nil
}

func toProfile(u *User) Profile {
	return Profile{
		ID:          fmt.Sprintf("%d", u.ID),
		Nickname:    u.Nickname,
		Avatar:      u.Avatar,
		Signature:   u.Signature,
		Gender:      u.Gender,
		Level:       u.Level,
		Coin:        u.Coin,
		Status:      u.Status,
		MutedUntil:  u.MutedUntil,
		BannedUntil: u.BannedUntil,
	}
}

func defaultNickname() string {
	return "dli_" + randomDigits(8)
}

func randomDigits(n int) string {
	var sb strings.Builder
	for i := 0; i < n; i++ {
		v, _ := rand.Int(rand.Reader, big.NewInt(10))
		sb.WriteString(v.String())
	}
	return sb.String()
}
