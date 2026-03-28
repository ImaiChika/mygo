package registry

import "context"

// NoopRegistry 在当前阶段不做真实注册，但保留扩展位。
type NoopRegistry struct{}

func NewNoopRegistry() *NoopRegistry {
	return &NoopRegistry{}
}

func (r *NoopRegistry) Register(context.Context, string, string) error {
	return nil
}

func (r *NoopRegistry) Deregister(context.Context, string, string) error {
	return nil
}
