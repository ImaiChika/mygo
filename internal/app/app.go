package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"mygo/internal/config"
	"mygo/internal/modules/attachment"
	"mygo/internal/modules/chat"
	"mygo/internal/modules/collab"
	"mygo/internal/modules/system"
	"mygo/internal/modules/user"
	"mygo/internal/platform/auth"
	"mygo/internal/platform/eventing"
	"mygo/internal/platform/httpx"
	"mygo/internal/platform/postgres"
	"mygo/internal/platform/realtime"
	"mygo/internal/platform/redisx"
	"mygo/internal/platform/registry"
	"mygo/internal/platform/storage"
)

// Run 完成应用启动、依赖注入与优雅退出。
func Run(ctx context.Context, cfg config.Config) error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	var (
		db          *pgxpool.Pool
		redisClient *redis.Client
		err         error
	)

	if !cfg.Postgres.Enabled {
		return errors.New("当前版本要求启用 PostgreSQL")
	}

	db, err = postgres.New(ctx, cfg.Postgres)
	if err != nil {
		return err
	}
	defer db.Close()

	bus := eventing.NewLocalBus()
	if cfg.Redis.Enabled {
		redisClient, err = redisx.New(ctx, cfg.Redis)
		if err != nil {
			return err
		}
		defer func() {
			_ = redisClient.Close()
		}()
		bus = nil
	}

	var eventBus eventing.Bus
	if redisClient != nil {
		eventBus = eventing.NewRedisBus(redisClient)
	} else {
		eventBus = bus
	}

	serviceRegistry := registry.NewNoopRegistry()
	if err := serviceRegistry.Register(ctx, cfg.App.Name, cfg.HTTP.Addr); err != nil {
		return fmt.Errorf("注册服务失败: %w", err)
	}
	defer func() {
		_ = serviceRegistry.Deregister(context.Background(), cfg.App.Name, cfg.HTTP.Addr)
	}()

	hub := realtime.NewHub(logger)
	jwtIssuer := auth.NewIssuer(cfg.Auth.AccessSecret, cfg.Auth.AccessTTL)
	authenticator := auth.NewAuthenticator(jwtIssuer)

	userRepo := user.NewPostgresRepository(db)
	userService := user.NewService(userRepo, jwtIssuer)
	userHTTPHandler := user.NewHTTPHandler(userService)

	chatRepo := chat.NewPostgresRepository(db)
	chatService := chat.NewService(chatRepo, eventBus)
	chatHTTPHandler := chat.NewHTTPHandler(chatService)
	chatRealtimeHandler := chat.NewRealtimeHandler(logger, chatService, hub, cfg.HTTP.AllowedOrigins)

	collabService := collab.NewService(cfg.Auth.CollabSecret)
	collabHTTPHandler := collab.NewHTTPHandler(collabService)

	store := storage.NewLocalStore(cfg.Storage.UploadDir, cfg.Storage.PublicBase)
	attachmentRepo := attachment.NewPostgresRepository(db)
	attachmentService := attachment.NewService(attachmentRepo, store, chatService, cfg.Storage.MaxFileSize)
	attachmentHTTPHandler := attachment.NewHTTPHandler(attachmentService)

	realtimeCloser, err := chatRealtimeHandler.RegisterConsumers(ctx, eventBus)
	if err != nil {
		return fmt.Errorf("注册实时事件消费者失败: %w", err)
	}
	defer func() {
		if realtimeCloser != nil {
			_ = realtimeCloser.Close()
		}
	}()

	router := chi.NewRouter()
	router.Use(chimiddleware.RequestID)
	router.Use(chimiddleware.RealIP)
	router.Use(chimiddleware.Recoverer)
	router.Use(httpx.RequestLogger(logger))
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.HTTP.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-User-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	systemHandler := system.NewHandler(cfg.App.Name, cfg.App.Env, hub, func(ctx context.Context) error {
		if err := db.Ping(ctx); err != nil {
			return fmt.Errorf("postgres not ready: %w", err)
		}
		if redisClient != nil {
			if err := redisClient.Ping(ctx).Err(); err != nil {
				return fmt.Errorf("redis not ready: %w", err)
			}
		}
		return nil
	})
	systemHandler.RegisterRoutes(router)

	router.Handle("/uploads/*", http.StripPrefix("/uploads/", http.FileServer(http.Dir(cfg.Storage.UploadDir))))

	router.Route("/api/v1", func(r chi.Router) {
		userHTTPHandler.RegisterPublicRoutes(r)
		r.Group(func(r chi.Router) {
			r.Use(authenticator.Middleware)
			userHTTPHandler.RegisterProtectedRoutes(r)
			chatHTTPHandler.RegisterRoutes(r)
			attachmentHTTPHandler.RegisterRoutes(r)
			collabHTTPHandler.RegisterRoutes(r)
			r.Get("/ws", chatRealtimeHandler.ServeWS)
		})
	})

	server := &http.Server{
		Addr:         cfg.HTTP.Addr,
		Handler:      router,
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
		IdleTimeout:  cfg.HTTP.IdleTimeout,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("HTTP 服务启动", "addr", cfg.HTTP.Addr, "env", cfg.App.Env)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		logger.Info("准备优雅关闭服务")
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}
