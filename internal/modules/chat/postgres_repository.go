package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresRepository 使用 PostgreSQL 持久化聊天数据。
type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) CreateConversation(ctx context.Context, input CreateConversationInput) (Conversation, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Conversation{}, fmt.Errorf("开启事务失败: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	now := time.Now()
	conversation := Conversation{
		ID:        uuid.New(),
		Name:      input.Name,
		Kind:      input.Kind,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO conversations (id, name, kind, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
	`, conversation.ID, conversation.Name, conversation.Kind, conversation.CreatedAt, conversation.UpdatedAt); err != nil {
		return Conversation{}, fmt.Errorf("写入 conversations 失败: %w", err)
	}

	for _, userID := range input.MemberIDs {
		role := "member"
		if userID == input.OwnerID {
			role = "owner"
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO conversation_members (conversation_id, user_id, role, joined_at)
			VALUES ($1, $2, $3, $4)
		`, conversation.ID, userID, role, now); err != nil {
			return Conversation{}, fmt.Errorf("写入 conversation_members 失败: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return Conversation{}, fmt.Errorf("提交事务失败: %w", err)
	}

	return conversation, nil
}

func (r *PostgresRepository) ListConversationsByUser(ctx context.Context, userID string) ([]Conversation, error) {
	rows, err := r.db.Query(ctx, `
		SELECT c.id, c.name, c.kind, c.created_at, c.updated_at, cm.role,
			COALESCE((
				SELECT COUNT(1)
				FROM messages m
				LEFT JOIN conversation_read_states crs
					ON crs.conversation_id = c.id AND crs.user_id = cm.user_id
				WHERE m.conversation_id = c.id
					AND m.sender_id <> cm.user_id
					AND m.created_at > COALESCE(crs.last_read_at, to_timestamp(0))
			), 0) AS unread_count
		FROM conversations c
		INNER JOIN conversation_members cm ON cm.conversation_id = c.id
		WHERE cm.user_id = $1
		ORDER BY c.updated_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("查询会话失败: %w", err)
	}
	defer rows.Close()

	var conversations []Conversation
	for rows.Next() {
		var conversation Conversation
		if err := rows.Scan(
			&conversation.ID,
			&conversation.Name,
			&conversation.Kind,
			&conversation.CreatedAt,
			&conversation.UpdatedAt,
			&conversation.MemberRole,
			&conversation.UnreadCount,
		); err != nil {
			return nil, fmt.Errorf("扫描会话失败: %w", err)
		}
		conversations = append(conversations, conversation)
	}

	return conversations, rows.Err()
}

func (r *PostgresRepository) IsConversationMember(ctx context.Context, conversationID string, userID string) (bool, error) {
	var exists bool
	if err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM conversation_members
			WHERE conversation_id = $1 AND user_id = $2
		)
	`, conversationID, userID).Scan(&exists); err != nil {
		return false, fmt.Errorf("查询会话成员失败: %w", err)
	}
	return exists, nil
}

func (r *PostgresRepository) GetConversationMember(ctx context.Context, conversationID string, userID string) (ConversationMember, error) {
	var member ConversationMember
	err := r.db.QueryRow(ctx, `
		SELECT conversation_id, user_id, role, joined_at
		FROM conversation_members
		WHERE conversation_id = $1 AND user_id = $2
	`, conversationID, userID).Scan(
		&member.ConversationID,
		&member.UserID,
		&member.Role,
		&member.JoinedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ConversationMember{}, errors.New("会话成员不存在")
	}
	if err != nil {
		return ConversationMember{}, fmt.Errorf("查询会话成员失败: %w", err)
	}
	return member, nil
}

func (r *PostgresRepository) ListConversationMembers(ctx context.Context, conversationID string) ([]ConversationMember, error) {
	rows, err := r.db.Query(ctx, `
		SELECT cm.conversation_id, cm.user_id, cm.role, cm.joined_at, u.username, u.display_name, u.email
		FROM conversation_members cm
		INNER JOIN users u ON u.id = cm.user_id
		WHERE cm.conversation_id = $1
		ORDER BY joined_at ASC
	`, conversationID)
	if err != nil {
		return nil, fmt.Errorf("查询会话成员列表失败: %w", err)
	}
	defer rows.Close()

	var members []ConversationMember
	for rows.Next() {
		var member ConversationMember
		if err := rows.Scan(
			&member.ConversationID,
			&member.UserID,
			&member.Role,
			&member.JoinedAt,
			&member.Username,
			&member.DisplayName,
			&member.Email,
		); err != nil {
			return nil, fmt.Errorf("扫描会话成员失败: %w", err)
		}
		members = append(members, member)
	}

	return members, rows.Err()
}

func (r *PostgresRepository) AddConversationMembers(ctx context.Context, input AddConversationMembersInput) ([]ConversationMember, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("开启事务失败: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	now := time.Now()
	added := make([]ConversationMember, 0, len(input.Members))
	for _, member := range input.Members {
		var created ConversationMember
		if err := tx.QueryRow(ctx, `
			INSERT INTO conversation_members (conversation_id, user_id, role, joined_at)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (conversation_id, user_id)
			DO UPDATE SET role = EXCLUDED.role
			RETURNING conversation_id, user_id, role, joined_at
		`, input.ConversationID, member.UserID, member.Role, now).Scan(
			&created.ConversationID,
			&created.UserID,
			&created.Role,
			&created.JoinedAt,
		); err != nil {
			return nil, fmt.Errorf("写入会话成员失败: %w", err)
		}
		added = append(added, created)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE conversations
		SET updated_at = $2
		WHERE id = $1
	`, input.ConversationID, now); err != nil {
		return nil, fmt.Errorf("更新会话时间失败: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("提交事务失败: %w", err)
	}

	return added, nil
}

func (r *PostgresRepository) CreateMessage(ctx context.Context, input SendMessageInput) (Message, error) {
	metadata := input.Metadata
	if len(metadata) == 0 {
		metadata = json.RawMessage(`{}`)
	}

	now := time.Now()
	message := Message{
		ID:             uuid.New(),
		ConversationID: input.ConversationID,
		SenderID:       input.SenderID,
		Kind:           input.Kind,
		Content:        input.Content,
		Metadata:       metadata,
		CreatedAt:      now,
	}

	if _, err := r.db.Exec(ctx, `
		INSERT INTO messages (id, conversation_id, sender_id, kind, content, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, message.ID, message.ConversationID, message.SenderID, message.Kind, message.Content, message.Metadata, message.CreatedAt); err != nil {
		return Message{}, fmt.Errorf("写入消息失败: %w", err)
	}

	if _, err := r.db.Exec(ctx, `
		UPDATE conversations
		SET updated_at = $2
		WHERE id = $1
	`, message.ConversationID, now); err != nil {
		return Message{}, fmt.Errorf("更新会话时间失败: %w", err)
	}

	return message, nil
}

func (r *PostgresRepository) ListMessages(ctx context.Context, query ListMessagesQuery) ([]Message, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, conversation_id, sender_id, kind, content, metadata, created_at
		FROM messages
		WHERE conversation_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, query.ConversationID, query.Limit)
	if err != nil {
		return nil, fmt.Errorf("查询消息失败: %w", err)
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var message Message
		if err := rows.Scan(
			&message.ID,
			&message.ConversationID,
			&message.SenderID,
			&message.Kind,
			&message.Content,
			&message.Metadata,
			&message.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描消息失败: %w", err)
		}
		messages = append(messages, message)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// 为了更符合聊天界面的读取习惯，这里把最近 N 条消息反转成时间正序。
	for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
		messages[left], messages[right] = messages[right], messages[left]
	}

	return messages, nil
}

func (r *PostgresRepository) MarkConversationRead(ctx context.Context, input MarkConversationReadInput) (ReadState, error) {
	now := time.Now()
	lastReadAt := now
	if input.LastReadMessageID != nil {
		if err := r.db.QueryRow(ctx, `
			SELECT created_at
			FROM messages
			WHERE id = $1 AND conversation_id = $2
		`, *input.LastReadMessageID, input.ConversationID).Scan(&lastReadAt); err != nil {
			return ReadState{}, fmt.Errorf("查询已读消息失败: %w", err)
		}
	}

	state := ReadState{}
	if err := r.db.QueryRow(ctx, `
		INSERT INTO conversation_read_states (conversation_id, user_id, last_read_message_id, last_read_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (conversation_id, user_id)
		DO UPDATE SET
			last_read_message_id = EXCLUDED.last_read_message_id,
			last_read_at = EXCLUDED.last_read_at,
			updated_at = EXCLUDED.updated_at
		RETURNING conversation_id, user_id, last_read_message_id, last_read_at, updated_at
	`, input.ConversationID, input.UserID, input.LastReadMessageID, lastReadAt, now).Scan(
		&state.ConversationID,
		&state.UserID,
		&state.LastReadMessageID,
		&state.LastReadAt,
		&state.UpdatedAt,
	); err != nil {
		return ReadState{}, fmt.Errorf("写入已读状态失败: %w", err)
	}

	return state, nil
}
