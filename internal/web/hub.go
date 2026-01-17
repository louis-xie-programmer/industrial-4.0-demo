package web

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"log/slog"
	"net/http"
	"sync"
)

// Hub 负责管理所有的 WebSocket 客户端连接，并向它们广播消息
type Hub struct {
	clients    map[*websocket.Conn]bool // 存储所有活跃的客户端连接
	broadcast  chan []byte              // 广播通道，用于接收需要发送给所有客户端的消息
	register   chan *websocket.Conn     // 注册通道，用于接收新连接
	unregister chan *websocket.Conn     // 注销通道，用于处理断开的连接
	mu         sync.Mutex               // 互斥锁，保护 clients 映射的并发访问
}

// NewHub 创建一个新的 Hub 实例
func NewHub() *Hub {
	return &Hub{
		broadcast:  make(chan []byte),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
		clients:    make(map[*websocket.Conn]bool),
	}
}

// Run 启动 Hub 的主循环，监听并处理来自各个通道的事件
func (h *Hub) Run() {
	for {
		select {
		case conn := <-h.register:
			h.mu.Lock()
			h.clients[conn] = true
			h.mu.Unlock()
		case conn := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[conn]; ok {
				delete(h.clients, conn)
				conn.Close()
			}
			h.mu.Unlock()
		case message := <-h.broadcast:
			h.mu.Lock()
			// 向所有连接的客户端广播消息
			for conn := range h.clients {
				if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
					slog.Warn("写入 WebSocket 失败", "error", err)
					conn.Close()
					delete(h.clients, conn)
				}
			}
			h.mu.Unlock()
		}
	}
}

// BroadcastState 将状态序列化为 JSON 并发送到广播通道
func (h *Hub) BroadcastState(state interface{}) {
	message, err := json.Marshal(state)
	if err != nil {
		slog.Error("序列化状态失败", "error", err)
		return
	}
	h.broadcast <- message
}

// upgrader 将普通的 HTTP 连接升级为 WebSocket 连接
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// 允许所有来源的连接，生产环境中应配置为特定的域名
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// ServeWs 处理来自客户端的 WebSocket 请求
func (h *Hub) ServeWs(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("升级 WebSocket 失败", "error", err)
		return
	}
	h.register <- conn
	// 注意：这里没有启动 read pump，因为我们只关心从服务器到客户端的单向通信
}
