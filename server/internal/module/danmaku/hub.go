package danmaku

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/dlidli/server/internal/middleware"
	"github.com/dlidli/server/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// maxRoomClients 单视频房间在线连接上限（超出拒绝，客户端回退 HTTP 分段拉取）。
const maxRoomClients = 5000

// WSMsg 服务端下发的实时消息帧。
type WSMsg struct {
	Type string `json:"type"` // danmaku
	Data Item   `json:"data"`
}

// Hub 弹幕实时广播中心（M2-DM-03）：按视频 ID 分房间，发送成功即时广播。
type Hub struct {
	mu    sync.RWMutex
	rooms map[int64]*room
	log   *zap.Logger
	// upgrader 按允许来源白名单构建（防跨站 WS 劫持）
	upgrader websocket.Upgrader
}

type room struct {
	clients map[*client]bool
}

type client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
	uid  int64 // 已登录用户 ID（游客 0，仅读）
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

// Broadcast 向视频房间广播一条弹幕（无在线连接时 no-op）。
func (h *Hub) Broadcast(videoID int64, item *Item) {
	msg, err := json.Marshal(WSMsg{Type: "danmaku", Data: *item})
	if err != nil {
		return
	}
	h.mu.RLock()
	r, ok := h.rooms[videoID]
	h.mu.RUnlock()
	if !ok {
		return
	}
	for c := range r.clients {
		select {
		case c.send <- msg:
		default: // 慢消费者丢消息，不阻塞广播
			h.log.Warn("弹幕广播丢弃（慢消费者）", zap.Int64("video", videoID))
		}
	}
}

// Serve 升级 WS 连接并加入视频房间；房间满员时拒绝并提示回退 HTTP。
func (h *Hub) Serve(c *gin.Context, videoID int64, uid int64) {
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	h.mu.Lock()
	r, ok := h.rooms[videoID]
	if !ok {
		r = &room{clients: make(map[*client]bool)}
		h.rooms[videoID] = r
	}
	if len(r.clients) >= maxRoomClients {
		h.mu.Unlock()
		_ = conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "room full"), time.Now().Add(time.Second))
		_ = conn.Close()
		return
	}
	cl := &client{hub: h, conn: conn, send: make(chan []byte, 32), uid: uid}
	r.clients[cl] = true
	h.mu.Unlock()

	go cl.writeLoop()
	cl.readLoop(videoID)
}

// writeLoop 出站消息 + 30s ping 保活。
func (cl *client) writeLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		_ = cl.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-cl.send:
			if !ok {
				_ = cl.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			_ = cl.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := cl.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = cl.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := cl.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// readLoop 客户端只读（弹幕发送走 HTTP）；异常/关闭时移除连接。
func (cl *client) readLoop(videoID int64) {
	defer func() {
		cl.hub.remove(cl, videoID)
		_ = cl.conn.Close()
	}()
	cl.conn.SetReadLimit(512)
	_ = cl.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	cl.conn.SetPongHandler(func(string) error {
		return cl.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	})
	for {
		if _, _, err := cl.conn.ReadMessage(); err != nil {
			return
		}
	}
}

// remove 移除连接；房间空后回收。
func (h *Hub) remove(cl *client, videoID int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if r, ok := h.rooms[videoID]; ok {
		delete(r.clients, cl)
		close(cl.send)
		if len(r.clients) == 0 {
			delete(h.rooms, videoID)
		}
	}
}

// wsHandler 弹幕实时连接（optionalAuth：游客只读可连，登录态用于屏蔽过滤）。
func (h *Handler) ws(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	videoID, err := h.svc.videoSvc.PublishedIDByBvid(c.Request.Context(), c.Param("bvid"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	h.svc.hub.Serve(c, videoID, uid)
}
