package attachment

import (
	"time"

	"github.com/google/uuid"
)

// Attachment 表示一份已上传的附件元数据。
type Attachment struct {
	ID             uuid.UUID `json:"id"`
	ConversationID uuid.UUID `json:"conversation_id"`
	UploadedBy     string    `json:"uploaded_by"`
	OriginalName   string    `json:"original_name"`
	ContentType    string    `json:"content_type"`
	SizeBytes      int64     `json:"size_bytes"`
	StorageKey     string    `json:"storage_key"`
	PublicURL      string    `json:"public_url"`
	CreatedAt      time.Time `json:"created_at"`
}
