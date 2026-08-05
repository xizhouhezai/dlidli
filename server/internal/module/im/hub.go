package im

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// Hub 私信实时推送中心（M3-IM-02，PRD MSG-13）：按用户 ID 分房间，在线即推。
type Hub struct {
	mu       sync.RWMutex
	users    map[int64]*userRoom
	log      *zap.Logger
	upgrader websocket.Upgrader
}

type userRoom struct {
	clients map[*client]bool
}

type client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
	uid  int64
}

// NewHub 构建私信 Hub（Origin 白名单防跨站 WS 劫持）。
func NewHub(allowOrigins []string, log *zap.Logger) *Hub {
	allow := make(map[string]bool, len(allowOrigins))
	for _, o := range allowOrigins {
		allow[strings.TrimRight(o, "/")] = true
	}
	return &Hub{
		users: make(map[int64]*userRoom),
		log:   log,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
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

// Push 向用户推送一条消息（在线时；无连接 no-op）。
func (h *Hub) Push(uid int64, item MessageItem) {
	msg, err := json.Marshal(WSMsg{Type: "message", Data: item})
	if err != nil {
		return
	}
	h.mu.RLock()
	r, ok := h.users[uid]
	h.mu.RUnlock()
	if !ok {
		return
	}
	for c := range r.clients {
		select {
		case c.send <- msg:
		default:
			h.log.Warn("私信推送丢弃（慢消费者）", zap.Int64("uid", uid))
		}
	}
}

// Serve 升级 WS 连接并加入用户房间（token 经 query 校验：WS 无法自定义 header）。
func (h *Hub) Serve(c *gin.Context, uid int64) {
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	h.mu.Lock()
	r, ok := h.users[uid]
	if !ok {
		r = &userRoom{clients: make(map[*client]bool)}
		h.users[uid] = r
	}
	cl := &client{hub: h, conn: conn, send: make(chan []byte, 32), uid: uid}
	r.clients[cl] = true
	h.mu.Unlock()

	go cl.writeLoop()
	cl.readLoop()
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

// readLoop 客户端只读（发送走 HTTP）；异常/关闭时移除。
func (cl *client) readLoop() {
	defer func() {
		cl.hub.remove(cl)
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
func (h *Hub) remove(cl *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if r, ok := h.users[cl.uid]; ok {
		delete(r.clients, cl)
		close(cl.send)
		if len(r.clients) == 0 {
			delete(h.users, cl.uid)
		}
	}
}
