package chat

import (
	"context"

	"github.com/google/uuid"
)

// Repository 定义聊天模块的数据访问契约。
type Repository interface {
	CreateConversation(ctx context.Context, input CreateConversationInput) (Conversation, error)
	ListConversationsByUser(ctx context.Context, userID string) ([]Conversation, error)
	IsConversationMember(ctx context.Context, conversationID string, userID string) (bool, error)
	GetConversationMember(ctx context.Context, conversationID string, userID string) (ConversationMember, error)
	ListConversationMembers(ctx context.Context, conversationID string) ([]ConversationMember, error)
	AddConversationMembers(ctx context.Context, input AddConversationMembersInput) ([]ConversationMember, error)
	GetMessage(ctx context.Context, conversationID uuid.UUID, messageID uuid.UUID) (Message, error)
	CreateMessage(ctx context.Context, input SendMessageInput) (Message, error)
	ListMessages(ctx context.Context, query ListMessagesQuery) ([]Message, error)
	MarkConversationRead(ctx context.Context, input MarkConversationReadInput) (ReadState, error)
}
