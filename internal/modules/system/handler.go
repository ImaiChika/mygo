package system

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"mygo/internal/platform/httpx"
	"mygo/internal/platform/realtime"
)

// ReadinessCheck 用于按需探测外部依赖可用性。
type ReadinessCheck func(ctx context.Context) error

// Handler 提供系统级探活接口。
type Handler struct {
	appName   string
	env       string
	hub       *realtime.Hub
	readiness ReadinessCheck
}

func NewHandler(appName string, env string, hub *realtime.Hub, readiness ReadinessCheck) *Handler {
	return &Handler{
		appName:   appName,
		env:       env,
		hub:       hub,
		readiness: readiness,
	}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/api/v1/health/liveness", h.Liveness)
	r.Get("/api/v1/health/readiness", h.Readiness)
}

func (h *Handler) Liveness(w http.ResponseWriter, _ *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"app":    h.appName,
		"env":    h.env,
		"ts":     time.Now(),
	})
}

func (h *Handler) Readiness(w http.ResponseWriter, r *http.Request) {
	if h.readiness != nil {
		if err := h.readiness(r.Context()); err != nil {
			httpx.JSON(w, http.StatusServiceUnavailable, map[string]any{
				"status": "not_ready",
				"error":  err.Error(),
				"ts":     time.Now(),
			})
			return
		}
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"status":   "ready",
		"app":      h.appName,
		"env":      h.env,
		"realtime": h.hub.Snapshot(),
		"ts":       time.Now(),
	})
}
