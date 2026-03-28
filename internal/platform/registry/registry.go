package registry

import "context"

// ServiceRegistry 预留给 Consul / Kubernetes Service Discovery 等能力。
type ServiceRegistry interface {
	Register(ctx context.Context, serviceName string, address string) error
	Deregister(ctx context.Context, serviceName string, address string) error
}
