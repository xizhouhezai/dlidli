package live

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// 连接治理参数（plan live §2B.1）：
//   单房间连接上限沿用 danmaku 的 5000（当前单机部署，无压测依据前不调）；
//   客户端应用层心跳 ≤15s 一次（复用 spec live §5B 口径），40s 无上行判超时踢出；
//   服务端另以 30s ping 保活（对齐 danmaku/im hub 先例）。
const (
	maxRoomClients      = 5000
	readLimit           = 512
	writeWait           = 10 * time.Second
	pongWait            = 40 * time.Second // 30s ping 间隔 + 10s 余量
	pingPeriod          = 30 * time.Second
	sendBuf             = 64
	maxDanmakuLen       = 50
	roomFullCloseCode   = 4001
	onlineCallTimeoutMs = 2000
)

// WSMsg 服务端下发的实时帧（直播域消息类型多类，区别于视频弹幕 hub 的单一 type）。
type WSMsg struct {
	Type string      `json:"type"` // enter | danmaku | gift | pong
	Data interface{} `json:"data"`
}

// EnterInfo 进场消息负载。
type EnterInfo struct {
	UID      string `json:"uid"`                // 字符串避免 JS 精度丢失
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar,omitempty"`
	FanBadge int64  `json:"fan_badge"` // 进房间主播的粉丝徽章等级（非粉丝 0；badgeMax 后取 1）
}

// DanmakuInfo 弹幕帧负载。
type DanmakuInfo struct {
	ID       int64  `json:"id,string"`
	UserID   string `json:"user_id"` // 已是十进制字符串（itoa），不再二次转字符串
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar,omitempty"`
	Content  string `json:"content"`
	// 会员身份标识（M4-VIP-01 权益落地点之二）：服务端判定后随弹幕帧下发，
	// 前端只渲染。**弹幕是直播场景里身份标识最直观的落点**（滚动中出现频率最高）。
	IsVip     bool   `json:"is_vip,omitempty"`
	NickColor string `json:"nickname_color,omitempty"`
}

// HistoryResult 进房拉历史（本房间最近 N 条，按时间正序回放）。
type HistoryResult struct {
	List    []DanmakuInfo `json:"list"`
	Online  int64         `json:"online"`
	RoomID  int64         `json:"room_id"`
}

// room 一个直播间的连接集合。
type room struct {
	clients map[*client]bool
}

// client 一条 WS 连接。
type client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan []byte
	uid      int64
	roomID   int64
	nickname string
	avatar   string
	counted  bool // 本次连接是否计入在线数（EnterForce 结果；断开按此配对扣减）
}

// Hub 直播弹幕广播中心（M4-LIV-03，plan live §2B.1）：独立 live hub，
// 不复用 danmaku hub（房间键语义/消息类型/上行需求均不同，见 plan live §2A.2）。
// 与 danmaku hub 的关键差异：客户端可上行（心跳 + 弹幕）；连接生命周期即在线状态
// （断开即离房，回调 OnlineLeave）；同一 uid 旧连接被新连接顶替（单连接语义）。
type Hub struct {
	mu    sync.RWMutex
	rooms map[int64]*room

	// deps 由 router 注入：弹幕校验链（频控/禁言/机审/落库/广播）、
	// 心跳 TTL 刷新与断开离房回调（在线计数归 LIV-02 的 Online；counted 传递配对标志）。
	onlineBeat  func(ctx context.Context, roomID, uid int64)
	onlineLeave func(ctx context.Context, roomID, uid int64, counted bool)
	send        func(ctx context.Context, roomID, uid int64, content string) error
	log         *zap.Logger

	upgrader websocket.Upgrader
}

func NewHub(allowOrigins []string, log *zap.Logger) *Hub {
	allow := make(map[string]bool, len(allowOrigins))
	for _, o := range allowOrigins {
		allow[strings.TrimRight(o, "/")] = true
	}
	return &Hub{
		rooms: make(map[int64]*room),
		log:   log,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			// 校验 Origin 在白名单内（无 Origin 头放行，非浏览器客户端）；防跨站 WS 劫持
			CheckOrigin: func(r *http.Request) bool {
				o := r.Header.Get("Origin")
				if o == "" {
					return true
				}
				return allow[strings.TrimRight(o, "/")]
			},
		},
	}
}

// SetDeps 注入发送链、心跳与离房回调（router 装配时调用，避免 Hub→Service 构造环）。
func (h *Hub) SetDeps(
	send func(ctx context.Context, roomID, uid int64, content string) error,
	onlineBeat func(ctx context.Context, roomID, uid int64),
	onlineLeave func(ctx context.Context, roomID, uid int64, counted bool),
) {
	h.send = send
	h.onlineBeat = onlineBeat
	h.onlineLeave = onlineLeave
}

// Serve 升级 WS 并进房（GET /live/rooms/:id/ws，auth 中间件已解出 uid）。
// 断开即离房：remove 后经 onlineLeave 扣减在线计数（LIV-02 口径）。
func (h *Hub) Serve(c *gin.Context, roomID, uid int64, nickname, avatar string) {
	h.ServeCounted(c, roomID, uid, nickname, avatar, false)
}

// ServeCounted 同 Serve，另携带进房计数标志（断开按标志配对扣减，防 TTL 先过期漏扣）。
func (h *Hub) ServeCounted(c *gin.Context, roomID, uid int64, nickname, avatar string, counted bool) {
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	h.mu.Lock()
	r, ok := h.rooms[roomID]
	if !ok {
		r = &room{clients: make(map[*client]bool)}
		h.rooms[roomID] = r
	}
	if len(r.clients) >= maxRoomClients {
		h.mu.Unlock()
		_ = conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(roomFullCloseCode, "room full"), time.Now().Add(time.Second))
		_ = conn.Close()
		return
	}
	cl := &client{hub: h, conn: conn, send: make(chan []byte, sendBuf),
		uid: uid, roomID: roomID, nickname: nickname, avatar: avatar, counted: counted}
	// 单连接语义：同 uid 旧连接顶替（防同账号多连接重复广播/重复计数）
	var old *client
	for other := range r.clients {
		if other.uid == uid {
			old = other
			break
		}
	}
	if old != nil {
		delete(r.clients, old)
		close(old.send)
	}
	r.clients[cl] = true
	h.mu.Unlock()
	if old != nil {
		_ = old.conn.Close() // 旧连接 readLoop 随即退出（不重复回调 remove，已不在集合）
	}

	go cl.writeLoop()
	cl.readLoop()
}

// broadcast 向房间广播一帧（无在线连接 no-op）。
// 迭代期间全程持读锁：send 为非阻塞投递不会死锁，且与 remove 的写锁
// （delete + close(send)）互斥，避免并发迭代/写 map 与 send on closed channel。
func (h *Hub) broadcast(roomID int64, msg WSMsg) {
	buf, err := json.Marshal(msg)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	r, ok := h.rooms[roomID]
	if !ok {
		return
	}
	for c := range r.clients {
		select {
		case c.send <- buf:
		default: // 慢消费者丢消息，不阻塞广播
			h.log.Warn("直播弹幕广播丢弃（慢消费者）", zap.Int64("room", roomID))
		}
	}
}

// BroadcastEnter 进场消息：观众进房向全房间广播（含粉丝徽章轻量信息，spec LIV-12）。
func (h *Hub) BroadcastEnter(roomID int64, info EnterInfo) {
	h.broadcast(roomID, WSMsg{Type: "enter", Data: info})
}

// BroadcastDanmaku 弹幕广播（房间全员；发送者不排除——直播场景发送者同屏确认）。
func (h *Hub) BroadcastDanmaku(roomID int64, item DanmakuInfo) {
	h.broadcast(roomID, WSMsg{Type: "danmaku", Data: item})
}

// BroadcastGift 礼物特效广播（M4-LIV-04，type=gift）：普通礼物前端走弹幕区连击样式，
// 高级礼物全屏特效（按 effect/level 分级渲染，见 GiftFrame）。
func (h *Hub) BroadcastGift(roomID int64, frame GiftFrame) {
	h.broadcast(roomID, WSMsg{Type: "gift", Data: frame})
}

// OnlineCount 当前 WS 在线连接数（实测比对用；对外在线人数仍以 Redis 计数为准）。
func (h *Hub) OnlineCount(roomID int64) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.rooms[roomID].clients)
}

// InRoom uid 是否已有连接在该房间（弹幕上行前置校验：未进房拒绝）。
func (h *Hub) InRoom(roomID, uid int64) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	r, ok := h.rooms[roomID]
	if !ok {
		return false
	}
	for c := range r.clients {
		if c.uid == uid {
			return true
		}
	}
	return false
}

// writeLoop 出站消息 + 30s ping 保活（对齐 danmaku/im hub 先例）。
func (cl *client) writeLoop() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = cl.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-cl.send:
			if !ok {
				_ = cl.conn.WriteControl(websocket.CloseMessage, nil, time.Now().Add(writeWait))
				return
			}
			_ = cl.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := cl.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = cl.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := cl.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// upMsg 客户端上行帧。
type upMsg struct {
	Type    string `json:"type"` // heartbeat | danmaku
	Content string `json:"content,omitempty"`
}

// readLoop 客户端上行：心跳（续命 + 刷 Redis TTL）与弹幕（走 Service.SendDanmaku 校验链）。
// 读超时/异常/关闭时移除连接并回调离房（断开即离房，plan live §2A.3 的 WS 生命周期口径）。
func (cl *client) readLoop() {
	defer func() {
		cl.hub.remove(cl)
		_ = cl.conn.Close()
	}()
	cl.conn.SetReadLimit(readLimit)
	_ = cl.conn.SetReadDeadline(time.Now().Add(pongWait))
	cl.conn.SetPongHandler(func(string) error {
		return cl.conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		mt, payload, err := cl.conn.ReadMessage()
		if err != nil {
			return
		}
		if mt != websocket.TextMessage {
			continue
		}
		var m upMsg
		if json.Unmarshal(payload, &m) != nil {
			continue // 坏帧静默丢弃
		}
		switch m.Type {
		case "heartbeat":
			_ = cl.conn.SetReadDeadline(time.Now().Add(pongWait))
			go cl.hub.onHeartbeat(cl)
		case "danmaku":
			content := strings.TrimSpace(m.Content)
			if content == "" || len([]rune(content)) > maxDanmakuLen {
				continue
			}
			go cl.hub.onDanmaku(cl, content)
		}
	}
}

// onHeartbeat 心跳处理：刷 LIV-02 的 Redis TTL（live:room:{id}:hb:{uid}），回 pong。
func (h *Hub) onHeartbeat(cl *client) {
	ctx, cancel := context.WithTimeout(context.Background(), onlineCallTimeoutMs*time.Millisecond)
	defer cancel()
	if h.onlineBeat != nil {
		h.onlineBeat(ctx, cl.roomID, cl.uid)
	}
	h.sendTo(cl.roomID, cl.uid, WSMsg{Type: "pong", Data: gin.H{"uid": strconv.FormatInt(cl.uid, 10), "ts": time.Now().UnixMilli()}})
}

// onDanmaku 弹幕上行：交 Service 校验链（频控/禁言/机审/落库/广播）。
func (h *Hub) onDanmaku(cl *client, content string) {
	if h.send == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := h.send(ctx, cl.roomID, cl.uid, content); err != nil {
		h.log.Warn("直播弹幕处理失败", zap.Int64("room", cl.roomID),
			zap.Int64("uid", cl.uid), zap.Error(err))
	}
}

// remove 移除连接；确有移除时回调离房（断开即离房扣计数），房间空后回收。
// 被顶替的旧连接（Serve 中已先删）不触发回调——其 uid 的新连接还在房间里。
func (h *Hub) remove(cl *client) {
	h.mu.Lock()
	removed := false
	if r, ok := h.rooms[cl.roomID]; ok {
		if _, still := r.clients[cl]; still {
			delete(r.clients, cl)
			close(cl.send)
			if len(r.clients) == 0 {
				delete(h.rooms, cl.roomID)
			}
			removed = true
		}
	}
	h.mu.Unlock()
	if !removed || cl.hub.onlineLeave == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), onlineCallTimeoutMs*time.Millisecond)
	defer cancel()
	cl.hub.onlineLeave(ctx, cl.roomID, cl.uid, cl.counted)
}

// sendTo 向房间内指定 uid 的连接单播一帧（心跳 pong 用，避免全房间广播放大）。
func (h *Hub) sendTo(roomID, uid int64, msg WSMsg) {
	buf, err := json.Marshal(msg)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	r, ok := h.rooms[roomID]
	if !ok {
		return
	}
	for c := range r.clients {
		if c.uid != uid {
			continue
		}
		select {
		case c.send <- buf:
		default:
		}
		return
	}
}
