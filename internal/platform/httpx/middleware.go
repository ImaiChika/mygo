package httpx

import (
	"log/slog"
	"net/http"
	"time"
)

// RequestLogger 输出基础请求日志，方便后续接入 tracing。
func RequestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, r)
			logger.Info("HTTP 请求完成",
				"method", r.Method,
				"path", r.URL.Path,
				"remote_addr", r.RemoteAddr,
				"duration", time.Since(start).String(),
			)
		})
	}
}
