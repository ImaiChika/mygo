package collab

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Service 为未来协作编辑网关提供票据与房间元数据。
type Service struct {
	secret string
}

func NewService(secret string) *Service {
	return &Service{secret: secret}
}

type CreateSessionInput struct {
	RoomID      uuid.UUID `json:"room_id"`
	UserID      string    `json:"user_id"`
	ProviderURL string    `json:"provider_url"`
}

func (s *Service) IssueAccessTicket(_ context.Context, input CreateSessionInput) (AccessTicket, error) {
	expiresAt := time.Now().Add(30 * time.Minute)
	payload := map[string]any{
		"room_id":    input.RoomID.String(),
		"user_id":    input.UserID,
		"expires_at": expiresAt.Unix(),
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return AccessTicket{}, fmt.Errorf("序列化协作票据失败: %w", err)
	}

	mac := hmac.New(sha256.New, []byte(s.secret))
	_, _ = mac.Write(raw)
	signature := mac.Sum(nil)

	token := base64.RawURLEncoding.EncodeToString(raw) + "." + base64.RawURLEncoding.EncodeToString(signature)

	return AccessTicket{
		RoomID:      input.RoomID,
		UserID:      input.UserID,
		Token:       token,
		ExpiresAt:   expiresAt,
		ProviderURL: input.ProviderURL,
	}, nil
}

func (s *Service) DescribeRoom(_ context.Context, roomID uuid.UUID) Room {
	return Room{
		ID:        roomID,
		Name:      "默认协作房间",
		Provider:  "yjs",
		Metadata:  json.RawMessage(`{"sync":"reserved","status":"not_enabled_yet"}`),
		CreatedAt: time.Now(),
	}
}
