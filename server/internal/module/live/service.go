package live

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"strconv"
	"time"

	"github.com/dlidli/server/internal/pkg/config"
	"github.com/dlidli/server/internal/pkg/errcode"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// fansRequired 开播资格：粉丝 ≥ 100（LIV-01）
const fansRequired = 100

// muteMin/muteMax 禁言时长范围：1 分钟 ~ 720 小时（spec §5C）
const (
	muteMin = time.Minute
	muteMax = 720 * time.Hour
)

type Service struct {
	repo   *Repo
	online *Online
	cfg    *config.Config
	log    *zap.Logger
	deps   *LiveDeps // 弹幕通道跨域依赖（M4-LIV-03，SetLiveDeps 注入）
	hub    *Hub      // 弹幕广播中心（SetHub 注入，避免 Service→Hub→Service 构造环）
}

func NewService(repo *Repo, online *Online, cfg *config.Config, log *zap.Logger) *Service {
	return &Service{repo: repo, online: online, cfg: cfg, log: log}
}

// SetLiveDeps 注入弹幕通道跨域依赖（router 装配）。
func (s *Service) SetLiveDeps(deps *LiveDeps) { s.deps = deps }

// SetHub 注入弹幕广播 Hub（router 装配）。
func (s *Service) SetHub(hub *Hub) { s.hub = hub }

// SetVipBadge 注入会员标识提供者（M4-VIP-01 权益落地：直播弹幕带会员徽章）。
//
// 写在 deps 上而非新增字段：会员标识与弹幕资料同属「弹幕通道的跨域只读依赖」，
// 分两处存放会让「deps 齐了但徽章没注入」这种半装配状态难以察觉。
// deps 为 nil 时惰性创建（装配顺序不该成为正确性的前提）。
func (s *Service) SetVipBadge(p VipBadgeProvider) {
	if s.deps == nil {
		s.deps = &LiveDeps{}
	}
	s.deps.Vip = p
}

// ---- DTO ----

// ApplyResult 申请开播返回：房间 + 推流地址 + 明文密钥（仅此一次）。
type ApplyResult struct {
	Room      *RoomInfo `json:"room"`
	PushURL   string    `json:"push_url"`
	StreamKey string    `json:"stream_key,omitempty"` // 明文密钥仅申请/重置时返回
}

// RoomListResult 直播中房间列表。
type RoomListResult struct {
	List  []*RoomInfo `json:"list"`
	Total int64       `json:"total"`
}

// RoomInfo 对外房间信息（密钥哈希永不出参）。
type RoomInfo struct {
	ID         int64      `json:"id"`
	AnchorID   string     `json:"anchor_id"` // 字符串避免 JS 精度丢失
	Title      string     `json:"title"`
	Cover      string     `json:"cover"`
	CategoryID int64      `json:"category_id"`
	Status     int8       `json:"status"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	EndedAt    *time.Time `json:"ended_at,omitempty"`
	Online     int64      `json:"online"`
	PlayURLs   PlayURLs   `json:"play_urls"` // 拉流地址（公开、不含密钥，M4-LIV-01 D4）
}

// PlayURLs 拉流地址（plan liv01 D4）：由 live.playDomain + stream 名拼装，不含推流密钥。
type PlayURLs struct {
	HLS string `json:"hls"`
	FLV string `json:"flv"`
}

func (s *Service) roomInfo(ctx context.Context, r *LiveRoom) (*RoomInfo, error) {
	n, err := s.online.Count(ctx, r.ID)
	if err != nil {
		return nil, err
	}
	return &RoomInfo{
		ID:         r.ID,
		AnchorID:   itoa(r.AnchorID),
		Title:      r.Title,
		Cover:      r.Cover,
		CategoryID: r.CategoryID,
		Status:     r.Status,
		StartedAt:  r.StartedAt,
		EndedAt:    r.EndedAt,
		Online:     n,
		PlayURLs:   s.playURLs(r.ID),
	}, nil
}

// ---- 开播资格（LIV-01 校验逻辑）----

// CheckQualification 资格三闸（逐项短路，错误可区分）：
// 白名单 → 实名标记 → 粉丝 ≥ 100。
func (s *Service) CheckQualification(ctx context.Context, uid int64) error {
	ok, err := s.repo.Whitelisted(uid)
	if err != nil {
		return err
	}
	if !ok {
		return errcode.ErrLiveNotWhitelisted
	}
	if !s.realnamed(uid) {
		return errcode.ErrLiveNotRealnamed
	}
	fans, err := s.repo.CountFollowers(uid)
	if err != nil {
		return err
	}
	if fans < fansRequired {
		return errcode.ErrLiveFansNotEnough
	}
	return nil
}

// realnamed 实名判定（ACC-23 能力位）：本卡按「user_auth 存在该用户的手机/邮箱记录」实现。
// 真实实名单/第三方核验接入不阻塞本卡（spec live §5A.3）。
func (s *Service) realnamed(uid int64) bool {
	ok, err := s.repo.Realnamed(uid)
	return err == nil && ok
}

// ---- 房间生命周期 ----

// Apply 申请开播：资格校验 → 建房（一主播一房）→ 生成推流密钥（明文仅此一次返回）。
func (s *Service) Apply(ctx context.Context, uid int64) (*ApplyResult, error) {
	if err := s.CheckQualification(ctx, uid); err != nil {
		return nil, err
	}
	room, err := s.repo.RoomByAnchor(uid)
	if err != nil {
		return nil, err
	}
	if room != nil {
		return nil, errcode.ErrLiveRoomExists
	}
	key, hash, err := genStreamKey()
	if err != nil {
		return nil, err
	}
	room = &LiveRoom{AnchorID: uid, Status: RoomStatusIdle, StreamKeyHash: hash}
	if err := s.repo.CreateRoom(room); err != nil {
		return nil, err
	}
	info, err := s.roomInfo(ctx, room)
	if err != nil {
		return nil, err
	}
	return &ApplyResult{Room: info, PushURL: s.pushURL(room.ID, key), StreamKey: key}, nil
}

// Start 开播：status 0/2 → 1，写 started_at、清 ended_at、在线计数清零。
func (s *Service) Start(ctx context.Context, uid int64) (*RoomInfo, error) {
	room, err := s.mustOwnedRoom(uid)
	if err != nil {
		return nil, err
	}
	if room.Status == RoomStatusLive {
		return nil, errcode.ErrLiveAlreadyLive
	}
	now := time.Now()
	if err := s.repo.UpdateRoomFields(room.ID, map[string]any{
		"status": RoomStatusLive, "started_at": now, "ended_at": nil,
	}); err != nil {
		return nil, err
	}
	if err := s.online.Reset(ctx, room.ID); err != nil {
		return nil, err
	}
	room.Status, room.StartedAt, room.EndedAt = RoomStatusLive, &now, nil
	return s.roomInfo(ctx, room)
}

// Stop 下播：status 1 → 2，写 ended_at，在线计数清零。
func (s *Service) Stop(ctx context.Context, uid int64) (*RoomInfo, error) {
	room, err := s.mustOwnedRoom(uid)
	if err != nil {
		return nil, err
	}
	if room.Status != RoomStatusLive {
		return nil, errcode.ErrLiveNotLive
	}
	return s.stopRoom(ctx, room)
}

// stopRoom 下播落库（status 1→2、ended_at、在线清零、清弹幕历史）。
// 调用方保证 room.Status == RoomStatusLive；主播下播与 SRS on_unpublish 回调共用本路径。
func (s *Service) stopRoom(ctx context.Context, room *LiveRoom) (*RoomInfo, error) {
	now := time.Now()
	if err := s.repo.UpdateRoomFields(room.ID, map[string]any{
		"status": RoomStatusEnded, "ended_at": now,
	}); err != nil {
		return nil, err
	}
	if err := s.online.Reset(ctx, room.ID); err != nil {
		return nil, err
	}
	if err := s.repo.ClearRoomMsg(room.ID); err != nil {
		return nil, err
	}
	room.Status, room.EndedAt = RoomStatusEnded, &now
	return s.roomInfo(ctx, room)
}

// UpdateRoom 修改标题/封面/分区（任意状态可改；标题 1-80 字，分区须存在）。
func (s *Service) UpdateRoom(ctx context.Context, uid int64, title, cover string, categoryID int64) (*RoomInfo, error) {
	room, err := s.mustOwnedRoom(uid)
	if err != nil {
		return nil, err
	}
	if len([]rune(title)) < 1 || len([]rune(title)) > 80 {
		return nil, errcode.ErrLiveTitleRequired
	}
	if categoryID <= 0 {
		return nil, errcode.ErrLiveCategoryInvalid
	}
	ok, err := s.repo.CategoryExists(categoryID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errcode.ErrLiveCategoryInvalid
	}
	fields := map[string]any{"title": title, "cover": cover, "category_id": categoryID}
	if err := s.repo.UpdateRoomFields(room.ID, fields); err != nil {
		return nil, err
	}
	room.Title, room.Cover, room.CategoryID = title, cover, categoryID
	return s.roomInfo(ctx, room)
}

// ResetStreamKey 重置推流密钥（旧密钥立即失效；明文一次性返回）。
func (s *Service) ResetStreamKey(ctx context.Context, uid int64) (*ApplyResult, error) {
	room, err := s.mustOwnedRoom(uid)
	if err != nil {
		return nil, err
	}
	key, hash, err := genStreamKey()
	if err != nil {
		return nil, err
	}
	if err := s.repo.UpdateRoomFields(room.ID, map[string]any{"stream_key_hash": hash}); err != nil {
		return nil, err
	}
	info, err := s.roomInfo(ctx, room)
	if err != nil {
		return nil, err
	}
	return &ApplyResult{Room: info, PushURL: s.pushURL(room.ID, key), StreamKey: key}, nil
}

// MyRoom 我的房间（含推流基址；明文密钥不回传，故不带 key 查询串）。
func (s *Service) MyRoom(ctx context.Context, uid int64) (*ApplyResult, error) {
	room, err := s.mustOwnedRoom(uid)
	if err != nil {
		return nil, err
	}
	info, err := s.roomInfo(ctx, room)
	if err != nil {
		return nil, err
	}
	return &ApplyResult{Room: info, PushURL: s.basePushURL(room.ID)}, nil
}

// mustOwnedRoom 取本人房间；无房或非主播报错。
func (s *Service) mustOwnedRoom(uid int64) (*LiveRoom, error) {
	room, err := s.repo.RoomByAnchor(uid)
	if err != nil {
		return nil, err
	}
	if room == nil {
		return nil, errcode.ErrLiveRoomNotFound
	}
	return room, nil
}

// streamApp 配置的 RTMP 应用名（缺省 live）。
func (s *Service) streamApp() string {
	if s.cfg != nil && s.cfg.Live.StreamApp != "" {
		return s.cfg.Live.StreamApp
	}
	return "live"
}

// StreamName RTMP 流名：room_<roomID>（公开，不含密钥）。
func StreamName(roomID int64) string { return "room_" + itoa(roomID) }

// basePushURL 推流基址（不含密钥）：rtmp://<pushDomain>/<app>/room_<id>
// MyRoom 等无明文密钥的回传口径用它。
func (s *Service) basePushURL(roomID int64) string {
	domain := ""
	if s.cfg != nil {
		domain = s.cfg.Live.PushDomain
	}
	if domain == "" {
		domain = "push.placeholder.dlidli.local"
	}
	return "rtmp://" + domain + "/" + s.streamApp() + "/" + StreamName(roomID)
}

// pushURL 完整推流地址（仅 Apply/ResetStreamKey 拿到明文密钥时拼 key，plan liv01 D1）。
func (s *Service) pushURL(roomID int64, key string) string {
	u := s.basePushURL(roomID)
	if key == "" {
		return u
	}
	return u + "?key=" + url.QueryEscape(key)
}

// playURLs 拉流地址（plan liv01 D4）：http://<playDomain>/<app>/room_<id>.m3u8|.flv，不含密钥。
func (s *Service) playURLs(roomID int64) PlayURLs {
	domain := ""
	if s.cfg != nil {
		domain = s.cfg.Live.PlayDomain
	}
	if domain == "" {
		domain = "play.placeholder.dlidli.local"
	}
	base := "http://" + domain + "/" + s.streamApp() + "/" + StreamName(roomID)
	return PlayURLs{HLS: base + ".m3u8", FLV: base + ".flv"}
}

// genStreamKey 生成 32 位随机密钥（crypto/rand hex）+ bcrypt 哈希。
func genStreamKey() (key, hash string, err error) {
	b := make([]byte, 16)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	key = hex.EncodeToString(b)
	h, err := bcrypt.GenerateFromPassword([]byte(key), bcrypt.DefaultCost)
	if err != nil {
		return "", "", err
	}
	return key, string(h), nil
}

// VerifyStreamKey 校验推流密钥（bcrypt 比对；LIV-01 推流回调使用）。
func (s *Service) VerifyStreamKey(roomID int64, key string) (bool, error) {
	room, err := s.repo.RoomByID(roomID)
	if err != nil || room == nil {
		return false, err
	}
	return bcrypt.CompareHashAndPassword([]byte(room.StreamKeyHash), []byte(key)) == nil, nil
}

func itoa(v int64) string {
	return strconv.FormatInt(v, 10)
}
