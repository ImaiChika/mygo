package attachment

import "context"

// Repository 定义附件元数据持久化契约。
type Repository interface {
	Create(ctx context.Context, attachment Attachment) (Attachment, error)
}
