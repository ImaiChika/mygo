package chat

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"mygo/internal/platform/eventing"
)

type stubRepository struct {
	createConversationInput CreateConversationInput
	createConversationFn    func(ctx context.Context, input CreateConversationInput) (Conversation, error)
	listConversationsFn     func(ctx context.Context, userID string) ([]Conversation, error)
	isMemberFn              func(ctx context.Context, conversationID string, userID string) (bool, error)
	getMemberFn             func(ctx context.Context, conversationID string, userID string) (ConversationMember, error)
	listMembersFn           func(ctx context.Context, conversationID string) ([]ConversationMember, error)
	addMembersFn            func(ctx context.Context, input AddConversationMembersInput) ([]ConversationMember, error)
	createMessageFn         func(ctx context.Context, input SendMessageInput) (Message, error)
	listMessagesFn          func(ctx context.Context, query ListMessagesQuery) ([]Message, error)
	markReadFn              func(ctx context.Context, input MarkConversationReadInput) (ReadState, error)
}

func (s *stubRepository) CreateConversation(ctx context.Context, input CreateConversationInput) (Conversation, error) {
	s.createConversationInput = input
	return s.createConversationFn(ctx, input)
}

func (s *stubRepository) ListConversationsByUser(ctx context.Context, userID string) ([]Conversation, error) {
	return s.listConversationsFn(ctx, userID)
}

func (s *stubRepository) IsConversationMember(ctx context.Context, conversationID string, userID string) (bool, error) {
	return s.isMemberFn(ctx, conversationID, userID)
}

func (s *stubRepository) GetConversationMember(ctx context.Context, conversationID string, userID string) (ConversationMember, error) {
	return s.getMemberFn(ctx, conversationID, userID)
}

func (s *stubRepository) ListConversationMembers(ctx context.Context, conversationID string) ([]ConversationMember, error) {
	return s.listMembersFn(ctx, conversationID)
}

func (s *stubRepository) AddConversationMembers(ctx context.Context, input AddConversationMembersInput) ([]ConversationMember, error) {
	return s.addMembersFn(ctx, input)
}

func (s *stubRepository) CreateMessage(ctx context.Context, input SendMessageInput) (Message, error) {
	return s.createMessageFn(ctx, input)
}

func (s *stubRepository) ListMessages(ctx context.Context, query ListMessagesQuery) ([]Message, error) {
	return s.listMessagesFn(ctx, query)
}

func (s *stubRepository) MarkConversationRead(ctx context.Context, input MarkConversationReadInput) (ReadState, error) {
	return s.markReadFn(ctx, input)
}

type stubBus struct {
	published []eventing.Event
}

func (b *stubBus) Publish(_ context.Context, event eventing.Event) error {
	b.published = append(b.published, event)
	return nil
}

func (b *stubBus) Subscribe(context.Context, string, eventing.Handler) (io.Closer, error) {
	return io.NopCloser(strings.NewReader("")), nil
}

func TestCreateConversationDeduplicatesMembers(t *testing.T) {
	repo := &stubRepository{
		createConversationFn: func(_ context.Context, input CreateConversationInput) (Conversation, error) {
			return Conversation{
				ID:        uuid.New(),
				Name:      input.Name,
				Kind:      input.Kind,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}, nil
		},
		listConversationsFn: func(context.Context, string) ([]Conversation, error) { return nil, nil },
		isMemberFn:          func(context.Context, string, string) (bool, error) { return true, nil },
		getMemberFn:         func(context.Context, string, string) (ConversationMember, error) { return ConversationMember{}, nil },
		listMembersFn:       func(context.Context, string) ([]ConversationMember, error) { return nil, nil },
		addMembersFn:        func(context.Context, AddConversationMembersInput) ([]ConversationMember, error) { return nil, nil },
		createMessageFn:     func(context.Context, SendMessageInput) (Message, error) { return Message{}, nil },
		listMessagesFn:      func(context.Context, ListMessagesQuery) ([]Message, error) { return nil, nil },
		markReadFn:          func(context.Context, MarkConversationReadInput) (ReadState, error) { return ReadState{}, nil },
	}
	service := NewService(repo, &stubBus{})

	_, err := service.CreateConversation(context.Background(), CreateConversationInput{
		Name:      "核心项目群",
		Kind:      ConversationKindGroup,
		MemberIDs: []string{"u-1", "u-2", "u-2", "u-3"},
		OwnerID:   "u-1",
	})
	if err != nil {
		t.Fatalf("CreateConversation 返回错误: %v", err)
	}

	if got, want := len(repo.createConversationInput.MemberIDs), 3; got != want {
		t.Fatalf("成员去重失败，got=%d want=%d", got, want)
	}
}

func TestSendMessagePublishesEvent(t *testing.T) {
	conversationID := uuid.New()
	now := time.Now()
	repo := &stubRepository{
		createConversationFn: func(context.Context, CreateConversationInput) (Conversation, error) {
			return Conversation{}, nil
		},
		listConversationsFn: func(context.Context, string) ([]Conversation, error) {
			return nil, nil
		},
		isMemberFn: func(_ context.Context, gotConversationID string, gotUserID string) (bool, error) {
			if gotConversationID != conversationID.String() || gotUserID != "u-1" {
				t.Fatalf("成员校验参数异常: %s %s", gotConversationID, gotUserID)
			}
			return true, nil
		},
		createMessageFn: func(_ context.Context, input SendMessageInput) (Message, error) {
			return Message{
				ID:             uuid.New(),
				ConversationID: input.ConversationID,
				SenderID:       input.SenderID,
				Kind:           input.Kind,
				Content:        input.Content,
				CreatedAt:      now,
			}, nil
		},
		listMessagesFn: func(context.Context, ListMessagesQuery) ([]Message, error) {
			return nil, nil
		},
		getMemberFn: func(context.Context, string, string) (ConversationMember, error) {
			return ConversationMember{}, nil
		},
		listMembersFn: func(context.Context, string) ([]ConversationMember, error) {
			return nil, nil
		},
		addMembersFn: func(context.Context, AddConversationMembersInput) ([]ConversationMember, error) {
			return nil, nil
		},
		markReadFn: func(context.Context, MarkConversationReadInput) (ReadState, error) {
			return ReadState{}, nil
		},
	}
	bus := &stubBus{}
	service := NewService(repo, bus)

	message, err := service.SendMessage(context.Background(), SendMessageInput{
		ConversationID: conversationID,
		SenderID:       "u-1",
		Content:        "hello mygo",
	})
	if err != nil {
		t.Fatalf("SendMessage 返回错误: %v", err)
	}

	if message.Content != "hello mygo" {
		t.Fatalf("消息内容不正确: %s", message.Content)
	}

	if got := len(bus.published); got != 1 {
		t.Fatalf("事件数量异常，got=%d want=1", got)
	}

	var payload map[string]any
	if err := json.Unmarshal(bus.published[0].Payload, &payload); err != nil {
		t.Fatalf("事件负载解析失败: %v", err)
	}
	if payload["conversation_id"] == nil {
		t.Fatalf("事件负载中缺少 conversation_id")
	}
}
