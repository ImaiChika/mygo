package eventing

import (
	"context"
	"testing"
	"time"
)

func TestLocalBusPublish(t *testing.T) {
	bus := NewLocalBus()
	done := make(chan struct{})

	closer, err := bus.Subscribe(context.Background(), TopicChatMessageCreated, func(_ context.Context, event Event) {
		if event.Aggregate != "conversation-1" {
			t.Fatalf("聚合根不匹配: %s", event.Aggregate)
		}
		close(done)
	})
	if err != nil {
		t.Fatalf("Subscribe 返回错误: %v", err)
	}
	defer func() {
		_ = closer.Close()
	}()

	if err := bus.Publish(context.Background(), Event{
		Topic:      TopicChatMessageCreated,
		Aggregate:  "conversation-1",
		OccurredAt: time.Now(),
		Payload:    []byte(`{"ok":true}`),
	}); err != nil {
		t.Fatalf("Publish 返回错误: %v", err)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("事件没有被消费者接收到")
	}
}
