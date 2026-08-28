package account

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dlidli/server/internal/module/growth"
	"github.com/dlidli/server/internal/pkg/captcha"
	"github.com/dlidli/server/internal/pkg/config"
	"github.com/dlidli/server/internal/pkg/encrypt"
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
	activateTTL     = 24 * time.Hour // 邮箱激活 token 有效期（ACC-02）
)

var emailRe = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

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

// phKey 手机号/标识加密密钥：由 JWT secret 派生（32 字节 AES-256）。
// 不新增配置项，避免密钥滥用；生产通过 DLIDLI_JWT_SECRET 注入正式密钥（config 已有强校验）。
func (s *Service) phKey() []byte { return encrypt.DeriveKey(s.cfg.JWT.Secret) }

// encryptIdentifier 加密手机号标识（AES-GCM），并返回其确定性哈希。
// 返回 (密文, 哈希, 错误)；哈希用于查重/精确查询，密文用于存储。
func (s *Service) encryptIdentifier(identityType int8, plain string) (ciphertext, hash string, err error) {
	ciphertext, err = encrypt.Encrypt(s.phKey(), plain)
	if err != nil {
		return "", "", err
	}
	hash = encrypt.IdentifierHash(identityType, plain)
	return ciphertext, hash, nil
}

// migrateAuth 存量明文账号惰性加密回填（ACC-43 两阶段第 2 步前奏）：
// 命中旧明文的 auth，用明文重新加密并写入密文+哈希。手机号与邮箱标识都需加密；
// 三方 openid 不加密（非 PII、无展示需求与精确查询冲突）。
func (s *Service) migrateAuth(auth *UserAuth, plain string) {
	if auth.IdentifierHash != "" {
		return
	}
	ciphertext, hash, err := s.encryptIdentifier(auth.IdentityType, plain)
	if err != nil {
		s.log.Warn("存量标识加密回填失败", zap.Int64("auth_id", auth.ID), zap.Error(err))
		return
	}
	if err := s.repo.UpdateIdentifierEncrypted(auth.ID, ciphertext, hash); err != nil {
		s.log.Warn("存量标识加密回填落库失败", zap.Int64("auth_id", auth.ID), zap.Error(err))
		return
	}
	auth.Identifier, auth.IdentifierHash = ciphertext, hash
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
func (s *Service) LoginBySms(ctx context.Context, phone, code, inviteCode string) (*TokenPair, error) {
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
	// 命中旧明文（hash 为空）时惰性加密回填
	if auth != nil {
		s.migrateAuth(auth, phone)
	}

	var user *User
	if auth == nil {
		// 自动注册（注册礼：5 硬币）
		ciphertext, hash, err := s.encryptIdentifier(IdentityPhone, phone)
		if err != nil {
			return nil, err
		}
		user = &User{
			ID:       snowflake.NextID(),
			Nickname: defaultNickname(),
			Level:    1, // 手机注册即 Lv1（可发弹幕）
			Coin:     5,
		}
		if err := s.repo.CreateUserWithAuth(user, &UserAuth{
			IdentityType:   IdentityPhone,
			Identifier:     ciphertext, // ACC-43：密文存储
			IdentifierHash: hash,
		}); err != nil {
			return nil, err
		}
		// 内测邀请码（ACC-44）：占用失败补偿删除已建账号
		if err := s.requireInvite(ctx, inviteCode, user.ID); err != nil {
			_ = s.repo.DeleteUser(user.ID)
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

// RegisterByEmail 邮箱注册（ACC-02）：校验格式/查重 → 创建待激活账号 → 生成激活 token 并 mock 发送激活邮件。
// dev 环境返回 debug 激活链接（真实邮件服务接入后移除）。
func (s *Service) RegisterByEmail(ctx context.Context, email, password, inviteCode string) (debugURL string, err error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if !emailRe.MatchString(email) {
		return "", errcode.ErrInvalidParams.WithMsg("邮箱格式不正确")
	}
	if len(password) < 6 || len(password) > 64 {
		return "", errcode.ErrInvalidParams.WithMsg("密码长度须为 6-64 位")
	}
	existing, err := s.repo.FindAuth(IdentityEmail, email)
	if err != nil {
		return "", err
	}
	if existing != nil {
		return "", errcode.ErrAccountExists
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	user := &User{
		ID:       snowflake.NextID(),
		Nickname: defaultNickname(),
		Level:    1, // 与手机注册一致：注册即 Lv1
		Coin:     5, // 注册礼
	}
	auth := &UserAuth{
		IdentityType:   IdentityEmail,
		Identifier:     email, // 邮箱明文存储（ACC-43 加密仅针对手机号）
		IdentifierHash: encrypt.IdentifierHash(IdentityEmail, email),
		Credential:     string(hash),
		Activated:      0, // 待激活
	}
	if err := s.repo.CreateUserWithAuth(user, auth); err != nil {
		return "", err
	}
	// 内测邀请码（ACC-44）：占用失败补偿删除已建账号
	if err := s.requireInvite(ctx, inviteCode, user.ID); err != nil {
		_ = s.repo.DeleteUser(user.ID)
		return "", err
	}
	token, err := randomToken()
	if err != nil {
		return "", err
	}
	if err := s.rdb.Set(ctx, "ac:act:"+token, strconv.FormatInt(user.ID, 10), activateTTL).Err(); err != nil {
		return "", err
	}
	debugURL = fmt.Sprintf("/api/v1/auth/activate?token=%s", token)
	s.log.Info("mock 激活邮件", zap.String("email", email), zap.String("url", debugURL))
	if s.cfg.App.Env == "dev" {
		return debugURL, nil
	}
	return "", nil
}

// GenerateInviteCodes 批量生成内测邀请码（ACC-44，admin 调用）。
func (s *Service) GenerateInviteCodes(ctx context.Context, adminID int64, count int, expiresDays int) ([]string, error) {
	if count <= 0 || count > 100 {
		return nil, errcode.ErrInvalidParams.WithMsg("生成数量须在 1-100 之间")
	}
	var expiresAt *time.Time
	if expiresDays > 0 {
		t := time.Now().Add(time.Duration(expiresDays) * 24 * time.Hour)
		expiresAt = &t
	}
	codes := make([]InviteCode, 0, count)
	for i := 0; i < count; i++ {
		codes = append(codes, InviteCode{Code: randomInviteCode(), CreatedBy: adminID, ExpiresAt: expiresAt})
	}
	if _, err := s.repo.CreateInviteCodes(codes); err != nil {
		return nil, err
	}
	got := make([]string, 0, count)
	for _, c := range codes {
		got = append(got, c.Code)
	}
	return got, nil
}

// requireInvite 内测开关（ACC-44）：开启时校验并占用一次性邀请码。
// 先校验状态（不存在/已使用/已过期），再按 uid 原子占用；占用失败（并发竞态）返回已被使用。
func (s *Service) requireInvite(ctx context.Context, code string, uid int64) error {
	if !s.cfg.App.InviteCodeRequired {
		return nil
	}
	if code == "" {
		return errcode.ErrInvalidParams.WithMsg("内测期间请填写邀请码")
	}
	c, err := s.repo.FindInviteCode(code)
	if err != nil {
		return err
	}
	if c == nil {
		return errcode.ErrInvalidParams.WithMsg("邀请码不存在")
	}
	if c.UsedBy != nil {
		return errcode.ErrInvalidParams.WithMsg("邀请码已被使用")
	}
	if c.expiredAt(time.Now()) {
		return errcode.ErrInvalidParams.WithMsg("邀请码已过期")
	}
	ok, err := s.repo.ClaimInviteCode(code, uid)
	if err != nil {
		return err
	}
	if !ok {
		return errcode.ErrInvalidParams.WithMsg("邀请码已被使用")
	}
	return nil
}

// randomInviteCode 生成 8 位邀请码（去掉易混淆 0/O/1/I）。
func randomInviteCode() string {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 8)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		b[i] = alphabet[n.Int64()]
	}
	return string(b)
}

// ActivateEmail 激活邮箱账号（ACC-02）：校验激活 token → 置 activated=1 → 一次性消费。
func (s *Service) ActivateEmail(ctx context.Context, token string) error {
	key := "ac:act:" + token
	uidStr, err := s.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return errcode.ErrInvalidParams.WithMsg("激活链接无效或已过期")
	}
	if err != nil {
		return err
	}
	uid, _ := strconv.ParseInt(uidStr, 10, 64)
	if err := s.repo.UpdateAuthActivated(uid, IdentityEmail, 1); err != nil {
		return err
	}
	_ = s.rdb.Del(ctx, key).Err() // 一次性使用
	return nil
}

// randomToken 生成 64 位十六进制随机 token（激活/邀请等一次性令牌）。
func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
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
	// 命中旧明文（hash 为空）时惰性加密回填
	if auth != nil {
		s.migrateAuth(auth, account)
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

	// 邮箱注册的待激活账号拒绝登录（ACC-02）
	if auth.Activated == 0 {
		return nil, errcode.ErrAccountInactive
	}

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
	// 命中旧明文（hash 为空）时惰性加密回填
	s.migrateAuth(auth, phone)

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
	"coin_video":  "投币",
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
