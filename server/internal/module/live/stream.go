package live

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/dlidli/server/internal/pkg/errcode"
	"go.uber.org/zap"
)

// PublishAuthReq SRS http_hooks 回调体（on_publish / on_unpublish）。
// 字段名对齐 SRS 5 的 body；对缺失与类型差异容错（client_id 各版本有字符串/数字两态）。
type PublishAuthReq struct {
	Action   string `json:"action"`
	ClientID any    `json:"client_id"`
	IP       string `json:"ip"`
	VHost    string `json:"vhost"`
	App      string `json:"app"`
	Stream   string `json:"stream"`
	Param    string `json:"param"` // 推流 URL 的查询串，SRS 原样放入（形如 "?key=xxx"）
}

// pubKeyFmt 推流在线标记（观测用）：on_publish 写入，on_unpublish 清除。
const pubKeyFmt = "live:pub:%d"

// pubKeyTTL 标记的 TTL：回调丢失（服务器崩溃）时不至于永久残留。
const pubKeyTTL = 24 * time.Hour

// streamQuery 回调解析出的推流寻址信息。
type streamQuery struct {
	roomID int64
	key    string // 明文推流密钥（on_unpublish 场景可缺省）
}

func (s *Service) logger() *zap.Logger {
	if s.log == nil {
		return zap.NewNop()
	}
	return s.log
}

// CheckCallbackToken 回调共享 token 的常量时间比较（plan liv01 D2）：
// 取 ?token= 为主、X-Live-Callback-Token 头为辅；未配置 callbackToken 时一律拒绝（fail-closed）。
func (s *Service) CheckCallbackToken(queryToken, headerToken string) bool {
	want := ""
	if s.cfg != nil {
		want = s.cfg.Live.CallbackToken
	}
	got := queryToken
	if got == "" {
		got = headerToken
	}
	if want == "" || got == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

// parsePublish 解析回调寻址（plan liv01 D1）：流名必须是 room_<id>（公开、不含密钥），
// 兼容 "room_1?key=xxx" 形式；密钥优先取 param（SRS 放推流查询串的位置），其次取流名自带查询串。
func (s *Service) parsePublish(req *PublishAuthReq) (streamQuery, error) {
	if req == nil {
		return streamQuery{}, errcode.ErrLiveStreamFormatInvalid
	}
	// app 是寻址的一部分（D1 固定 live）：非空即须与配置一致，避免跨 app 抢流。
	if req.App != "" && req.App != s.streamApp() {
		return streamQuery{}, errcode.ErrLiveStreamFormatInvalid
	}
	name, rawQuery := req.Stream, ""
	if i := strings.IndexByte(name, '?'); i >= 0 {
		name, rawQuery = name[:i], name[i+1:]
	}
	if !strings.HasPrefix(name, "room_") {
		return streamQuery{}, errcode.ErrLiveStreamFormatInvalid
	}
	id, err := strconv.ParseInt(strings.TrimPrefix(name, "room_"), 10, 64)
	if err != nil || id <= 0 {
		return streamQuery{}, errcode.ErrLiveStreamFormatInvalid
	}
	q := rawQuery
	if p := strings.TrimPrefix(req.Param, "?"); p != "" {
		q = p
	}
	key := ""
	if q != "" {
		vals, err := url.ParseQuery(q)
		if err != nil {
			return streamQuery{}, errcode.ErrLiveStreamFormatInvalid
		}
		key = vals.Get("key")
	}
	return streamQuery{roomID: id, key: key}, nil
}

// AuthorizePublish on_publish 鉴权（plan liv01 D2）：房间存在 → 状态直播中(1) → bcrypt 校验 key。
// 任一步失败返回业务错误，handler 以 HTTP 403 下发（SRS 非 2xx 即拒绝推流）。
func (s *Service) AuthorizePublish(ctx context.Context, req *PublishAuthReq) (int64, error) {
	q, err := s.parsePublish(req)
	if err != nil {
		return 0, err
	}
	room, err := s.repo.RoomByID(q.roomID)
	if err != nil {
		return 0, err
	}
	if room == nil {
		return 0, errcode.ErrLiveRoomNotFound
	}
	if room.Status != RoomStatusLive {
		return 0, errcode.ErrLiveNotLive
	}
	// 封禁房间即使密钥正确也拒绝推流（M4-LIV-05，plan §2D.3）：
	// 否则被切断的主播立刻重推，快速切断形同虚设。
	if room.BanStatus == RoomBanApplied {
		return 0, errcode.ErrLiveRoomBanned
	}
	if q.key == "" {
		return 0, errcode.ErrLiveStreamKeyInvalid
	}
	ok, err := s.VerifyStreamKey(q.roomID, q.key)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, errcode.ErrLiveStreamKeyInvalid
	}
	if s.deps != nil && s.deps.RDB != nil {
		_ = s.deps.RDB.Set(ctx, fmt.Sprintf(pubKeyFmt, q.roomID),
			time.Now().Format(time.RFC3339), pubKeyTTL).Err()
	}
	s.logger().Info("SRS on_publish 鉴权通过",
		zap.Int64("room", q.roomID), zap.String("ip", req.IP), zap.Any("client_id", req.ClientID))
	return q.roomID, nil
}

// StopByStream on_unpublish 下播（plan liv01 D3，复用主播下播路径停止本场直播）；幂等：
// 未知流 / 房间已非直播中一律视为无操作。返回是否确有下播动作（供 handler 区分并记日志）。
func (s *Service) StopByStream(ctx context.Context, req *PublishAuthReq) (bool, error) {
	q, err := s.parsePublish(req)
	if err != nil {
		return false, err
	}
	room, err := s.repo.RoomByID(q.roomID)
	if err != nil {
		return false, err
	}
	if room == nil || room.Status != RoomStatusLive {
		return false, nil
	}
	if _, err := s.stopRoom(ctx, room); err != nil {
		return false, err
	}
	if s.deps != nil && s.deps.RDB != nil {
		_ = s.deps.RDB.Del(ctx, fmt.Sprintf(pubKeyFmt, q.roomID)).Err()
	}
	s.logger().Info("SRS on_unpublish 已下播", zap.Int64("room", q.roomID), zap.String("ip", req.IP))
	return true, nil
}
