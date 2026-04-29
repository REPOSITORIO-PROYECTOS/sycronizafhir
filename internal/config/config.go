package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	LocalPostgresURL    string
	SQLitePath          string
	SupabaseURL         string
	SupabaseServiceRole string
	SupabaseRealtimeURL string
	SupabaseDBHost      string
	SupabaseDBPort      int
	SupabaseDBUser      string
	SupabaseDBPassword  string
	SupabaseDBName      string
	SupabaseDBSSLMode   string
	SupabaseDBURL       string
	OutboundInterval    time.Duration
	RealtimeChannel     string
	RealtimeSchema      string
	RealtimeTable       string
	SourceSchema        string
	ExcludeTables       []string
}

func Load() (Config, error) {
	_ = godotenv.Load()
	applyEmbeddedDefaults()

	intervalSeconds, err := readIntWithDefault("OUTBOUND_INTERVAL_SECONDS", 60)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		LocalPostgresURL:    os.Getenv("LOCAL_POSTGRES_URL"),
		SQLitePath:          readStringWithDefault("SQLITE_QUEUE_PATH", "./sync_queue.db"),
		SupabaseURL:         os.Getenv("SUPABASE_URL"),
		SupabaseServiceRole: os.Getenv("SUPABASE_SERVICE_ROLE_KEY"),
		SupabaseRealtimeURL: os.Getenv("SUPABASE_REALTIME_URL"),
		SupabaseDBHost:      os.Getenv("HOST_SUPABASE"),
		SupabaseDBPort:      readIntOrDefaultNoError("PUERTO_SUPABASE", 5432),
		SupabaseDBUser:      os.Getenv("USUARIO_SUPABASE"),
		SupabaseDBPassword:  os.Getenv("CONTRASENA_SUPABASE"),
		SupabaseDBName:      readStringWithDefault("SUPABASE_DB_NAME", "postgres"),
		SupabaseDBSSLMode:   readStringWithDefault("SUPABASE_DB_SSLMODE", "require"),
		SupabaseDBURL:       os.Getenv("SUPABASE_DB_URL"),
		OutboundInterval:    time.Duration(intervalSeconds) * time.Second,
		RealtimeChannel:     readStringWithDefault("SUPABASE_REALTIME_CHANNEL", "realtime:public:pedidos"),
		RealtimeSchema:      readStringWithDefault("SUPABASE_REALTIME_SCHEMA", "public"),
		RealtimeTable:       readStringWithDefault("SUPABASE_REALTIME_TABLE", "pedidos"),
		SourceSchema:        readStringWithDefault("SYNC_SOURCE_SCHEMA", "public"),
		ExcludeTables:       readCSV("SYNC_EXCLUDE_TABLES"),
	}

	if cfg.LocalPostgresURL == "" {
		return Config{}, errors.New("LOCAL_POSTGRES_URL is required")
	}

	if cfg.SupabaseDBHost == "" || cfg.SupabaseDBUser == "" || cfg.SupabaseDBPassword == "" {
		return Config{}, errors.New("HOST_SUPABASE, USUARIO_SUPABASE and CONTRASENA_SUPABASE are required")
	}

	if strings.TrimSpace(cfg.SupabaseRealtimeURL) == "" {
		return Config{}, errors.New("SUPABASE_REALTIME_URL is required")
	}

	if strings.TrimSpace(cfg.SupabaseServiceRole) == "" {
		return Config{}, errors.New("SUPABASE_SERVICE_ROLE_KEY is required")
	}

	if isPlaceholderSecret(cfg.SupabaseServiceRole) {
		return Config{}, errors.New("SUPABASE_SERVICE_ROLE_KEY is using a placeholder value")
	}

	return cfg, nil
}

func (c Config) SupabaseDBDSN() string {
	if strings.TrimSpace(c.SupabaseDBURL) != "" {
		return c.SupabaseDBURL
	}

	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.SupabaseDBUser,
		c.SupabaseDBPassword,
		c.SupabaseDBHost,
		c.SupabaseDBPort,
		c.SupabaseDBName,
		c.SupabaseDBSSLMode,
	)
}

func readCSV(key string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return []string{}
	}

	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" {
			continue
		}
		values = append(values, item)
	}

	return values
}

func readStringWithDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}

func readIntWithDefault(key string, fallback int) (int, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid integer: %w", key, err)
	}

	if parsed <= 0 {
		return 0, fmt.Errorf("%s must be greater than zero", key)
	}

	return parsed, nil
}

func readIntOrDefaultNoError(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func isPlaceholderSecret(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return true
	}

	placeholderFragments := []string{
		"your-service-role-key",
		"your-project",
		"changeme",
		"replace-me",
		"placeholder",
	}

	for _, fragment := range placeholderFragments {
		if strings.Contains(normalized, fragment) {
			return true
		}
	}

	return false
}
