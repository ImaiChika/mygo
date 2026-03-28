package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"mygo/internal/app"
	"mygo/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("加载配置失败", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx, cfg); err != nil {
		slog.Error("应用退出", "err", err)
		os.Exit(1)
	}
}
