package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config 汇总整个应用需要的配置项。
type Config struct {
	App      AppConfig
	HTTP     HTTPConfig
	Postgres PostgresConfig
	Redis    RedisConfig
	Auth     AuthConfig
	Storage  StorageConfig
}

type AppConfig struct {
	Name string
	Env  string
}

type HTTPConfig struct {
	Addr           string
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	IdleTimeout    time.Duration
	AllowedOrigins []string
}

type PostgresConfig struct {
	Enabled  bool
	DSN      string
	MaxConns int32
	MinConns int32
}

type RedisConfig struct {
	Enabled  bool
	Addr     string
	Password string
	DB       int
}

type AuthConfig struct {
	AccessSecret string
	CollabSecret string
	AccessTTL    time.Duration
}

type StorageConfig struct {
	UploadDir   string
	PublicBase  string
	MaxFileSize int64
}

// Load 从环境变量加载配置，并附带合理默认值。
func Load() (Config, error) {
	loadDotEnv(".env")

	cfg := Config{
		App: AppConfig{
			Name: getEnv("APP_NAME", "mygo"),
			Env:  getEnv("APP_ENV", "dev"),
		},
		HTTP: HTTPConfig{
			Addr:           getEnv("HTTP_ADDR", ":8080"),
			ReadTimeout:    getEnvDuration("HTTP_READ_TIMEOUT", 10*time.Second),
			WriteTimeout:   getEnvDuration("HTTP_WRITE_TIMEOUT", 15*time.Second),
			IdleTimeout:    getEnvDuration("HTTP_IDLE_TIMEOUT", 60*time.Second),
			AllowedOrigins: splitCSV(getEnv("WS_ALLOWED_ORIGINS", "http://localhost:5173,http://127.0.0.1:5173")),
		},
		Postgres: PostgresConfig{
			Enabled:  getEnvBool("POSTGRES_ENABLED", true),
			DSN:      getEnv("POSTGRES_DSN", "postgres://mygo:mygo@127.0.0.1:5432/mygo?sslmode=disable"),
			MaxConns: int32(getEnvInt("POSTGRES_MAX_CONNS", 10)),
			MinConns: int32(getEnvInt("POSTGRES_MIN_CONNS", 2)),
		},
		Redis: RedisConfig{
			Enabled:  getEnvBool("REDIS_ENABLED", true),
			Addr:     getEnv("REDIS_ADDR", "127.0.0.1:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
		},
		Auth: AuthConfig{
			AccessSecret: getEnv("AUTH_ACCESS_SECRET", "change-me"),
			CollabSecret: getEnv("AUTH_COLLAB_SECRET", "change-me-too"),
			AccessTTL:    getEnvDuration("AUTH_ACCESS_TTL", 24*time.Hour),
		},
		Storage: StorageConfig{
			UploadDir:   getEnv("STORAGE_UPLOAD_DIR", "./uploads"),
			PublicBase:  strings.TrimRight(getEnv("STORAGE_PUBLIC_BASE_URL", "http://127.0.0.1:8080"), "/"),
			MaxFileSize: getEnvInt64("STORAGE_MAX_FILE_SIZE", 10*1024*1024),
		},
	}

	if cfg.Postgres.Enabled && cfg.Postgres.DSN == "" {
		return Config{}, fmt.Errorf("POSTGRES_DSN 不能为空")
	}

	if cfg.Redis.Enabled && cfg.Redis.Addr == "" {
		return Config{}, fmt.Errorf("REDIS_ADDR 不能为空")
	}

	if strings.TrimSpace(cfg.Auth.AccessSecret) == "" {
		return Config{}, fmt.Errorf("AUTH_ACCESS_SECRET 不能为空")
	}

	if strings.TrimSpace(cfg.Storage.UploadDir) == "" {
		return Config{}, fmt.Errorf("STORAGE_UPLOAD_DIR 不能为空")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	value, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func getEnvBool(key string, fallback bool) bool {
	value, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvInt(key string, fallback int) int {
	value, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvInt64(key string, fallback int64) int64 {
	value, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	value, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// loadDotEnv 仅在开发态作为兜底使用，不覆盖已存在的系统环境变量。
func loadDotEnv(filename string) {
	file, err := os.Open(filepath.Clean(filename))
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		_ = os.Setenv(key, value)
	}
}
