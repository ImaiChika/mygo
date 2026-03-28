package eventing

import (
	"context"
	"fmt"
	"io"
	"sync"
)

// LocalBus 用于单实例场景，也适合作为测试环境总线。
type LocalBus struct {
	mu          sync.RWMutex
	subscribers map[string]map[int]Handler
	nextID      int
}

func NewLocalBus() *LocalBus {
	return &LocalBus{
		subscribers: make(map[string]map[int]Handler),
	}
}

func (b *LocalBus) Publish(ctx context.Context, event Event) error {
	b.mu.RLock()
	handlers := b.subscribers[event.Topic]
	list := make([]Handler, 0, len(handlers))
	for _, handler := range handlers {
		list = append(list, handler)
	}
	b.mu.RUnlock()

	for _, handler := range list {
		handler(ctx, event)
	}

	return nil
}

func (b *LocalBus) Subscribe(_ context.Context, topic string, handler Handler) (io.Closer, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.nextID++
	id := b.nextID

	if _, ok := b.subscribers[topic]; !ok {
		b.subscribers[topic] = make(map[int]Handler)
	}
	b.subscribers[topic][id] = handler

	return closerFunc(func() error {
		b.mu.Lock()
		defer b.mu.Unlock()

		handlers, ok := b.subscribers[topic]
		if !ok {
			return fmt.Errorf("topic %s 不存在", topic)
		}

		delete(handlers, id)
		if len(handlers) == 0 {
			delete(b.subscribers, topic)
		}

		return nil
	}), nil
}

type closerFunc func() error

func (fn closerFunc) Close() error {
	return fn()
}
