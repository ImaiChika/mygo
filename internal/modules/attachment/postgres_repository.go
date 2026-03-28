package attachment

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresRepository 使用 PostgreSQL 保存附件元数据。
type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Create(ctx context.Context, attachment Attachment) (Attachment, error) {
	var created Attachment
	if err := r.db.QueryRow(ctx, `
		INSERT INTO attachments (
			id, conversation_id, uploaded_by, original_name, content_type,
			size_bytes, storage_key, public_url, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, conversation_id, uploaded_by, original_name, content_type,
			size_bytes, storage_key, public_url, created_at
	`, attachment.ID, attachment.ConversationID, attachment.UploadedBy, attachment.OriginalName, attachment.ContentType,
		attachment.SizeBytes, attachment.StorageKey, attachment.PublicURL, attachment.CreatedAt).Scan(
		&created.ID,
		&created.ConversationID,
		&created.UploadedBy,
		&created.OriginalName,
		&created.ContentType,
		&created.SizeBytes,
		&created.StorageKey,
		&created.PublicURL,
		&created.CreatedAt,
	); err != nil {
		return Attachment{}, fmt.Errorf("写入附件元数据失败: %w", err)
	}

	return created, nil
}
