package eventing

import (
	"context"
	"encoding/json"
	"io"
	"time"
)

const (
	// TopicChatMessageCreated 是聊天消息写入成功后的广播主题。
	TopicChatMessageCreated = "chat.message.created"
)

// Event 是模块间传播的标准事件结构。
type Event struct {
	Topic      string          `json:"topic"`
	Aggregate  string          `json:"aggregate"`
	OccurredAt time.Time       `json:"occurred_at"`
	Payload    json.RawMessage `json:"payload"`
}

// Handler 代表事件订阅回调。
type Handler func(ctx context.Context, event Event)

// Bus 表示事件总线抽象。
type Bus interface {
	Publish(ctx context.Context, event Event) error
	Subscribe(ctx context.Context, topic string, handler Handler) (io.Closer, error)
}
