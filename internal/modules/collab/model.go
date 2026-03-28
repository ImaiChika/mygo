package collab

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Room 表示协作房间元数据。
type Room struct {
	ID             uuid.UUID       `json:"id"`
	ConversationID *uuid.UUID      `json:"conversation_id,omitempty"`
	Name           string          `json:"name"`
	Provider       string          `json:"provider"`
	Metadata       json.RawMessage `json:"metadata"`
	CreatedAt      time.Time       `json:"created_at"`
}

// AccessTicket 用于未来接入 Yjs Provider 的临时授权。
type AccessTicket struct {
	RoomID      uuid.UUID `json:"room_id"`
	UserID      string    `json:"user_id"`
	Token       string    `json:"token"`
	ExpiresAt   time.Time `json:"expires_at"`
	ProviderURL string    `json:"provider_url"`
}
