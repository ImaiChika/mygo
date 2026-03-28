package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strings"

	"github.com/google/uuid"

	"mygo/internal/platform/eventing"
)

// Service 负责聊天模块的业务编排。
type Service struct {
	repo Repository
	bus  eventing.Bus
}

func NewService(repo Repository, bus eventing.Bus) *Service {
	return &Service{
		repo: repo,
		bus:  bus,
	}
}

func (s *Service) CreateConversation(ctx context.Context, input CreateConversationInput) (Conversation, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Kind = strings.TrimSpace(input.Kind)
	input.OwnerID = strings.TrimSpace(input.OwnerID)

	if input.Name == "" {
		return Conversation{}, errors.New("会话名称不能为空")
	}
	if input.Kind == "" {
		input.Kind = ConversationKindGroup
	}
	if input.OwnerID == "" {
		return Conversation{}, errors.New("创建者不能为空")
	}

	members := make([]string, 0, len(input.MemberIDs)+1)
	seen := make(map[string]struct{})
	for _, memberID := range append(input.MemberIDs, input.OwnerID) {
		memberID = strings.TrimSpace(memberID)
		if memberID == "" {
			continue
		}
		if _, ok := seen[memberID]; ok {
			continue
		}
		seen[memberID] = struct{}{}
		members = append(members, memberID)
	}
	if len(members) == 0 {
		return Conversation{}, errors.New("至少需要一个会话成员")
	}

	input.MemberIDs = members
	return s.repo.CreateConversation(ctx, input)
}

func (s *Service) ListConversations(ctx context.Context, userID string) ([]Conversation, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, errors.New("用户 ID 不能为空")
	}
	return s.repo.ListConversationsByUser(ctx, userID)
}

func (s *Service) CanAccessConversation(ctx context.Context, conversationID uuid.UUID, userID string) (bool, error) {
	return s.repo.IsConversationMember(ctx, conversationID.String(), strings.TrimSpace(userID))
}

func (s *Service) ListConversationMembers(ctx context.Context, conversationID uuid.UUID, userID string) ([]ConversationMember, error) {
	allowed, err := s.repo.IsConversationMember(ctx, conversationID.String(), strings.TrimSpace(userID))
	if err != nil {
		return nil, fmt.Errorf("校验会话成员失败: %w", err)
	}
	if !allowed {
		return nil, errors.New("当前用户不属于该会话")
	}

	return s.repo.ListConversationMembers(ctx, conversationID.String())
}

func (s *Service) AddConversationMembers(ctx context.Context, input AddConversationMembersInput) ([]ConversationMember, error) {
	if input.ConversationID == uuid.Nil {
		return nil, errors.New("会话 ID 不能为空")
	}
	input.OperatorID = strings.TrimSpace(input.OperatorID)
	if input.OperatorID == "" {
		return nil, errors.New("操作者不能为空")
	}
	if len(input.Members) == 0 {
		return nil, errors.New("至少需要一个成员")
	}

	operator, err := s.repo.GetConversationMember(ctx, input.ConversationID.String(), input.OperatorID)
	if err != nil {
		return nil, fmt.Errorf("查询操作者会话权限失败: %w", err)
	}
	if operator.Role != ConversationRoleOwner && operator.Role != ConversationRoleAdmin {
		return nil, errors.New("当前用户没有管理会话成员的权限")
	}

	normalized := make([]ConversationMemberInput, 0, len(input.Members))
	seen := make(map[string]struct{})
	for _, member := range input.Members {
		member.UserID = strings.TrimSpace(member.UserID)
		member.Role = strings.TrimSpace(member.Role)
		if member.UserID == "" {
			continue
		}
		if _, ok := seen[member.UserID]; ok {
			continue
		}
		seen[member.UserID] = struct{}{}
		if member.Role == "" {
			member.Role = ConversationRoleMember
		}
		switch member.Role {
		case ConversationRoleOwner, ConversationRoleAdmin, ConversationRoleMember:
		default:
			return nil, fmt.Errorf("不支持的成员角色: %s", member.Role)
		}
		normalized = append(normalized, member)
	}

	if len(normalized) == 0 {
		return nil, errors.New("没有可添加的有效成员")
	}

	input.Members = normalized
	return s.repo.AddConversationMembers(ctx, input)
}

func (s *Service) SendMessage(ctx context.Context, input SendMessageInput) (Message, error) {
	input.SenderID = strings.TrimSpace(input.SenderID)
	input.Kind = strings.TrimSpace(input.Kind)
	input.Content = strings.TrimSpace(input.Content)

	if input.ConversationID == uuid.Nil {
		return Message{}, errors.New("会话 ID 不能为空")
	}
	if input.SenderID == "" {
		return Message{}, errors.New("发送者不能为空")
	}
	if input.Kind == "" {
		input.Kind = MessageKindText
	}

	normalizedMetadata, replyToMessageID, err := normalizeOutgoingMetadata(input.Metadata)
	if err != nil {
		return Message{}, err
	}
	if input.Content == "" && len(normalizedMetadata) == 0 {
		return Message{}, errors.New("消息内容不能为空")
	}

	allowed, err := s.repo.IsConversationMember(ctx, input.ConversationID.String(), input.SenderID)
	if err != nil {
		return Message{}, fmt.Errorf("校验会话成员失败: %w", err)
	}
	if !allowed {
		return Message{}, errors.New("当前用户不属于该会话")
	}

	if replyToMessageID != nil {
		referencedMessage, err := s.repo.GetMessage(ctx, input.ConversationID, *replyToMessageID)
		if err != nil {
			return Message{}, fmt.Errorf("查询被引用消息失败: %w", err)
		}

		enrichedMetadata, err := withReplyReference(normalizedMetadata, referencedMessage)
		if err != nil {
			return Message{}, err
		}
		normalizedMetadata = enrichedMetadata
	}

	input.Metadata = normalizedMetadata

	message, err := s.repo.CreateMessage(ctx, input)
	if err != nil {
		return Message{}, err
	}

	payload, err := json.Marshal(map[string]any{
		"conversation_id": input.ConversationID,
		"message":         message,
	})
	if err != nil {
		return Message{}, fmt.Errorf("序列化消息事件失败: %w", err)
	}

	if err := s.bus.Publish(ctx, eventing.Event{
		Topic:      eventing.TopicChatMessageCreated,
		Aggregate:  input.ConversationID.String(),
		OccurredAt: message.CreatedAt,
		Payload:    payload,
	}); err != nil {
		return Message{}, fmt.Errorf("发布消息事件失败: %w", err)
	}

	return message, nil
}

func (s *Service) ListMessages(ctx context.Context, query ListMessagesQuery) ([]Message, error) {
	if query.ConversationID == uuid.Nil {
		return nil, errors.New("会话 ID 不能为空")
	}
	if strings.TrimSpace(query.UserID) == "" {
		return nil, errors.New("用户 ID 不能为空")
	}

	allowed, err := s.repo.IsConversationMember(ctx, query.ConversationID.String(), query.UserID)
	if err != nil {
		return nil, fmt.Errorf("校验会话成员失败: %w", err)
	}
	if !allowed {
		return nil, errors.New("当前用户不属于该会话")
	}

	if query.Limit <= 0 {
		query.Limit = 50
	}
	if query.Limit > 100 {
		query.Limit = 100
	}

	return s.repo.ListMessages(ctx, query)
}

func (s *Service) MarkConversationRead(ctx context.Context, input MarkConversationReadInput) (ReadState, error) {
	if input.ConversationID == uuid.Nil {
		return ReadState{}, errors.New("会话 ID 不能为空")
	}
	input.UserID = strings.TrimSpace(input.UserID)
	if input.UserID == "" {
		return ReadState{}, errors.New("用户 ID 不能为空")
	}

	allowed, err := s.repo.IsConversationMember(ctx, input.ConversationID.String(), input.UserID)
	if err != nil {
		return ReadState{}, fmt.Errorf("校验会话成员失败: %w", err)
	}
	if !allowed {
		return ReadState{}, errors.New("当前用户不属于该会话")
	}

	return s.repo.MarkConversationRead(ctx, input)
}

func normalizeOutgoingMetadata(raw json.RawMessage) (json.RawMessage, *uuid.UUID, error) {
	if len(raw) == 0 {
		return nil, nil, nil
	}

	var metadata map[string]any
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return nil, nil, errors.New("消息元数据格式错误")
	}

	replyValue, ok := metadata["reply"]
	if !ok {
		normalized, err := json.Marshal(metadata)
		if err != nil {
			return nil, nil, errors.New("消息元数据序列化失败")
		}
		return normalized, nil, nil
	}

	replyMap, ok := replyValue.(map[string]any)
	if !ok {
		return nil, nil, errors.New("reply 元数据格式错误")
	}

	rawMessageID, ok := replyMap["message_id"].(string)
	if !ok || strings.TrimSpace(rawMessageID) == "" {
		return nil, nil, errors.New("reply.message_id 不能为空")
	}

	messageID, err := uuid.Parse(strings.TrimSpace(rawMessageID))
	if err != nil {
		return nil, nil, errors.New("reply.message_id 无效")
	}

	delete(metadata, "reply")
	normalized, err := json.Marshal(metadata)
	if err != nil {
		return nil, nil, errors.New("消息元数据序列化失败")
	}

	if string(normalized) == "{}" {
		normalized = nil
	}

	return normalized, &messageID, nil
}

func withReplyReference(raw json.RawMessage, referenced Message) (json.RawMessage, error) {
	metadata := make(map[string]any)
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &metadata); err != nil {
			return nil, errors.New("消息元数据格式错误")
		}
	}

	replyContent := referenced.Content
	if trimmed := strings.TrimSpace(replyContent); trimmed == "" {
		replyContent = "[无文本内容]"
	}
	runes := []rune(replyContent)
	if len(runes) > 120 {
		replyContent = string(runes[:120]) + "..."
	}

	reply := map[string]any{
		"message_id": referenced.ID.String(),
		"sender_id":  referenced.SenderID,
		"kind":       referenced.Kind,
		"content":    replyContent,
	}

	merged := make(map[string]any, len(metadata)+1)
	maps.Copy(merged, metadata)
	merged["reply"] = reply

	normalized, err := json.Marshal(merged)
	if err != nil {
		return nil, errors.New("消息元数据序列化失败")
	}

	return normalized, nil
}
