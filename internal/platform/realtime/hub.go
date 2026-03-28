package realtime

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	// DefaultSendBufferSize 控制单连接待发送消息缓冲区大小。
	DefaultSendBufferSize = 64
)

// Envelope 是 WebSocket 对外推送的统一消息包装。
type Envelope struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

// Client 代表一个在线连接。
type Client struct {
	ID     string
	UserID string
	Send   chan []byte
}

// Hub 管理在线连接与房间订阅关系。
type Hub struct {
	logger *slog.Logger

	mu          sync.RWMutex
	clients     map[string]*Client
	rooms       map[string]map[string]*Client
	userClients map[string]map[string]*Client
}

func NewHub(logger *slog.Logger) *Hub {
	return &Hub{
		logger:      logger,
		clients:     make(map[string]*Client),
		rooms:       make(map[string]map[string]*Client),
		userClients: make(map[string]map[string]*Client),
	}
}

func (h *Hub) NewClient(userID string) *Client {
	return &Client{
		ID:     uuid.NewString(),
		UserID: userID,
		Send:   make(chan []byte, DefaultSendBufferSize),
	}
}

func (h *Hub) Register(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.clients[client.ID] = client
	if _, ok := h.userClients[client.UserID]; !ok {
		h.userClients[client.UserID] = make(map[string]*Client)
	}
	h.userClients[client.UserID][client.ID] = client

	h.logger.Info("WebSocket 客户端已连接", "client_id", client.ID, "user_id", client.UserID)
}

func (h *Hub) Unregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.clients, client.ID)
	if clients, ok := h.userClients[client.UserID]; ok {
		delete(clients, client.ID)
		if len(clients) == 0 {
			delete(h.userClients, client.UserID)
		}
	}

	for roomID, roomClients := range h.rooms {
		delete(roomClients, client.ID)
		if len(roomClients) == 0 {
			delete(h.rooms, roomID)
		}
	}

	close(client.Send)
	h.logger.Info("WebSocket 客户端已断开", "client_id", client.ID, "user_id", client.UserID)
}

func (h *Hub) Subscribe(roomID string, client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.rooms[roomID]; !ok {
		h.rooms[roomID] = make(map[string]*Client)
	}
	h.rooms[roomID][client.ID] = client
}

func (h *Hub) Unsubscribe(roomID string, client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	roomClients, ok := h.rooms[roomID]
	if !ok {
		return
	}

	delete(roomClients, client.ID)
	if len(roomClients) == 0 {
		delete(h.rooms, roomID)
	}
}

// PublishToRoom 向某个会话中的全部在线连接推送消息。
func (h *Hub) PublishToRoom(roomID string, envelope Envelope) {
	payload, err := json.Marshal(envelope)
	if err != nil {
		h.logger.Error("序列化 WebSocket 消息失败", "err", err)
		return
	}

	h.mu.RLock()
	clients := h.rooms[roomID]
	list := make([]*Client, 0, len(clients))
	for _, client := range clients {
		list = append(list, client)
	}
	h.mu.RUnlock()

	for _, client := range list {
		select {
		case client.Send <- payload:
		default:
			h.logger.Warn("WebSocket 发送缓冲区已满，准备断开连接", "client_id", client.ID)
			go h.Unregister(client)
		}
	}
}

// ConnectedUsers 返回当前在线用户数量。
func (h *Hub) ConnectedUsers() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.userClients)
}

// Snapshot 返回调试用在线概览。
func (h *Hub) Snapshot() map[string]any {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return map[string]any{
		"connected_users": len(h.userClients),
		"connections":     len(h.clients),
		"rooms":           len(h.rooms),
		"generated_at":    time.Now(),
	}
}
