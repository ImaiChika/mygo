package chat

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const (
	ConversationKindGroup  = "group"
	MessageKindFile        = "file"
	MessageKindImage       = "image"
	MessageKindText        = "text"
	ConversationRoleOwner  = "owner"
	ConversationRoleAdmin  = "admin"
	ConversationRoleMember = "member"
)

// Conversation 代表一个聊天会话。
type Conversation struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Kind        string    `json:"kind"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	MemberRole  string    `json:"member_role,omitempty"`
	UnreadCount int64     `json:"unread_count"`
}

// ConversationMember 记录用户与会话之间的关系。
type ConversationMember struct {
	ConversationID uuid.UUID `json:"conversation_id"`
	UserID         string    `json:"user_id"`
	Role           string    `json:"role"`
	JoinedAt       time.Time `json:"joined_at"`
	Username       string    `json:"username,omitempty"`
	DisplayName    string    `json:"display_name,omitempty"`
	Email          string    `json:"email,omitempty"`
}

// Message 代表一条聊天消息。
type Message struct {
	ID             uuid.UUID       `json:"id"`
	ConversationID uuid.UUID       `json:"conversation_id"`
	SenderID       string          `json:"sender_id"`
	Kind           string          `json:"kind"`
	Content        string          `json:"content"`
	Metadata       json.RawMessage `json:"metadata"`
	CreatedAt      time.Time       `json:"created_at"`
}

// ReplyReference 表示消息中携带的引用摘要。
type ReplyReference struct {
	MessageID uuid.UUID `json:"message_id"`
	SenderID  string    `json:"sender_id"`
	Kind      string    `json:"kind"`
	Content   string    `json:"content"`
}

type CreateConversationInput struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	MemberIDs []string `json:"member_ids"`
	OwnerID   string   `json:"owner_id"`
}

type SendMessageInput struct {
	ConversationID uuid.UUID       `json:"conversation_id"`
	SenderID       string          `json:"sender_id"`
	Kind           string          `json:"kind"`
	Content        string          `json:"content"`
	Metadata       json.RawMessage `json:"metadata"`
}

type ListMessagesQuery struct {
	ConversationID uuid.UUID
	UserID         string
	Limit          int
}

type AddConversationMembersInput struct {
	ConversationID uuid.UUID                 `json:"conversation_id"`
	OperatorID     string                    `json:"operator_id"`
	Members        []ConversationMemberInput `json:"members"`
}

type ConversationMemberInput struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

type MarkConversationReadInput struct {
	ConversationID    uuid.UUID  `json:"conversation_id"`
	UserID            string     `json:"user_id"`
	LastReadMessageID *uuid.UUID `json:"last_read_message_id,omitempty"`
}

type ReadState struct {
	ConversationID    uuid.UUID  `json:"conversation_id"`
	UserID            string     `json:"user_id"`
	LastReadMessageID *uuid.UUID `json:"last_read_message_id,omitempty"`
	LastReadAt        time.Time  `json:"last_read_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}
