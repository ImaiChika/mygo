package redisx

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"

	"mygo/internal/config"
)

// New 创建 Redis 客户端，并在启动阶段执行一次 PING。
func New(ctx context.Context, cfg config.RedisConfig) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:                     cfg.Addr,
		Password:                 cfg.Password,
		DB:                       cfg.DB,
		DialTimeout:              3 * time.Second,
		ReadTimeout:              3 * time.Second,
		WriteTimeout:             3 * time.Second,
		DisableIdentity:          true,
		MaintNotificationsConfig: &maintnotifications.Config{Mode: maintnotifications.ModeDisabled},
	})

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("连接 Redis 失败: %w", err)
	}

	return client, nil
}
