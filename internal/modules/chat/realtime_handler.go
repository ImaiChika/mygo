package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"mygo/internal/platform/auth"
	"mygo/internal/platform/eventing"
	"mygo/internal/platform/realtime"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 54 * time.Second
	maxMessageSize = 4096
)

// RealtimeHandler 负责 WebSocket 接入与消息扇出。
type RealtimeHandler struct {
	logger         *slog.Logger
	service        *Service
	hub            *realtime.Hub
	allowedOrigins map[string]struct{}
}

func NewRealtimeHandler(logger *slog.Logger, service *Service, hub *realtime.Hub, allowedOrigins []string) *RealtimeHandler {
	originSet := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		originSet[origin] = struct{}{}
	}

	return &RealtimeHandler{
		logger:         logger,
		service:        service,
		hub:            hub,
		allowedOrigins: originSet,
	}
}

func (h *RealtimeHandler) RegisterConsumers(ctx context.Context, bus eventing.Bus) (io.Closer, error) {
	return bus.Subscribe(ctx, eventing.TopicChatMessageCreated, func(_ context.Context, event eventing.Event) {
		var payload struct {
			ConversationID uuid.UUID `json:"conversation_id"`
			Message        Message   `json:"message"`
		}
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			h.logger.Warn("反序列化聊天事件失败", "err", err)
			return
		}

		h.hub.PublishToRoom(payload.ConversationID.String(), realtime.Envelope{
			Type: event.Topic,
			Data: payload,
		})
	})
}

func (h *RealtimeHandler) ServeWS(w http.ResponseWriter, r *http.Request) {
	user := auth.MustUser(r.Context())
	client := h.hub.NewClient(user.ID)

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			if len(h.allowedOrigins) == 0 {
				return true
			}

			origin := r.Header.Get("Origin")
			if origin == "" {
				return true
			}
			_, ok := h.allowedOrigins[origin]
			return ok
		},
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("升级 WebSocket 连接失败", "err", err)
		return
	}

	h.hub.Register(client)
	defer func() {
		_ = conn.Close()
		h.hub.Unregister(client)
	}()

	welcome, _ := json.Marshal(realtime.Envelope{
		Type: "system.connected",
		Data: map[string]any{
			"client_id": client.ID,
			"user_id":   client.UserID,
		},
	})
	client.Send <- welcome

	go h.writePump(conn, client)
	h.readPump(r.Context(), conn, client)
}

func (h *RealtimeHandler) readPump(ctx context.Context, conn *websocket.Conn, client *realtime.Client) {
	conn.SetReadLimit(maxMessageSize)
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			if !websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				h.logger.Info("WebSocket 连接关闭", "client_id", client.ID)
			} else {
				h.logger.Warn("WebSocket 读消息失败", "client_id", client.ID, "err", err)
			}
			return
		}

		var inbound struct {
			Type           string `json:"type"`
			ConversationID string `json:"conversation_id"`
		}
		if err := json.Unmarshal(raw, &inbound); err != nil {
			h.writeDirect(conn, realtime.Envelope{
				Type: "system.error",
				Data: map[string]any{"message": "消息格式错误"},
			})
			continue
		}

		switch inbound.Type {
		case "subscribe":
			h.handleSubscribe(ctx, conn, client, inbound.ConversationID)
		case "unsubscribe":
			h.hub.Unsubscribe(inbound.ConversationID, client)
		case "ping":
			h.writeDirect(conn, realtime.Envelope{Type: "pong", Data: map[string]any{"ts": time.Now()}})
		default:
			h.writeDirect(conn, realtime.Envelope{
				Type: "system.error",
				Data: map[string]any{"message": fmt.Sprintf("未知指令: %s", inbound.Type)},
			})
		}
	}
}

func (h *RealtimeHandler) handleSubscribe(ctx context.Context, conn *websocket.Conn, client *realtime.Client, conversationID string) {
	parsed, err := uuid.Parse(conversationID)
	if err != nil {
		h.writeDirect(conn, realtime.Envelope{
			Type: "system.error",
			Data: map[string]any{"message": "conversation_id 无效"},
		})
		return
	}

	allowed, err := h.service.CanAccessConversation(ctx, parsed, client.UserID)
	if err != nil {
		h.writeDirect(conn, realtime.Envelope{
			Type: "system.error",
			Data: map[string]any{"message": "校验会话权限失败"},
		})
		return
	}
	if !allowed {
		h.writeDirect(conn, realtime.Envelope{
			Type: "system.error",
			Data: map[string]any{"message": "当前用户不能订阅该会话"},
		})
		return
	}

	h.hub.Subscribe(parsed.String(), client)
	h.writeDirect(conn, realtime.Envelope{
		Type: "system.subscribed",
		Data: map[string]any{"conversation_id": parsed.String()},
	})
}

func (h *RealtimeHandler) writePump(conn *websocket.Conn, client *realtime.Client) {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case message, ok := <-client.Send:
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (h *RealtimeHandler) writeDirect(conn *websocket.Conn, envelope realtime.Envelope) {
	payload, err := json.Marshal(envelope)
	if err != nil {
		return
	}

	_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
	_ = conn.WriteMessage(websocket.TextMessage, payload)
}
