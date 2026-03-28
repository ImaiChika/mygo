package eventing

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/redis/go-redis/v9"
)

// RedisBus 用于多实例部署下的事件分发。
type RedisBus struct {
	client *redis.Client
}

func NewRedisBus(client *redis.Client) *RedisBus {
	return &RedisBus{client: client}
}

func (b *RedisBus) Publish(ctx context.Context, event Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("序列化事件失败: %w", err)
	}

	if err := b.client.Publish(ctx, event.Topic, payload).Err(); err != nil {
		return fmt.Errorf("发布 Redis 事件失败: %w", err)
	}

	return nil
}

func (b *RedisBus) Subscribe(ctx context.Context, topic string, handler Handler) (io.Closer, error) {
	pubsub := b.client.Subscribe(ctx, topic)
	if _, err := pubsub.Receive(ctx); err != nil {
		_ = pubsub.Close()
		return nil, fmt.Errorf("订阅 Redis 事件失败: %w", err)
	}

	go func() {
		channel := pubsub.Channel()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-channel:
				if !ok {
					return
				}

				var event Event
				if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
					continue
				}

				handler(ctx, event)
			}
		}
	}()

	return closerFunc(func() error {
		return pubsub.Close()
	}), nil
}
