package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"sycronizafhir/internal/config"
	"sycronizafhir/internal/monitor"
)

type App struct {
	ctx     context.Context
	runtime *monitor.Runtime
	cfg     *config.Config
}

type ConfigSummary struct {
	AppName       string   `json:"app_name"`
	LocalDB       string   `json:"local_db"`
	RemoteDB      string   `json:"remote_db"`
	SourceSchema  string   `json:"source_schema"`
	ExcludeTables []string `json:"exclude_tables"`
	OutboundEvery string   `json:"outbound_every"`
	RealtimeURL   string   `json:"realtime_url"`
	Channel       string   `json:"channel"`
	Schema        string   `json:"schema"`
	Table         string   `json:"table"`
}

type LocalConnectionInput struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
	SSLMode  string `json:"ssl_mode"`
}

type LocalConnectionResult struct {
	Success bool     `json:"success"`
	Message string   `json:"message"`
	DSN     string   `json:"dsn,omitempty"`
	DBs     []string `json:"dbs,omitempty"`
}

func NewApp(rt *monitor.Runtime, cfg *config.Config) *App {
	return &App{
		runtime: rt,
		cfg:     cfg,
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	a.runtime.Subscribe(func(event monitor.Event) {
		if a.ctx == nil {
			return
		}
		wailsruntime.EventsEmit(a.ctx, event.Topic, event.Payload)
	})

	a.runtime.AddLog("frontend conectado")
}

func (a *App) shutdown(ctx context.Context) {
	a.runtime.AddLog("frontend desconectado")
}

func (a *App) GetSnapshot() monitor.Snapshot {
	return a.runtime.Snapshot()
}

func (a *App) RunScan() (monitor.ScanResult, error) {
	if a.ctx == nil {
		return monitor.ScanResult{}, errors.New("contexto no disponible")
	}
	return a.runtime.RunScan(a.ctx)
}

func (a *App) RunCompare() (monitor.ScanResult, error) {
	if a.ctx == nil {
		return monitor.ScanResult{}, errors.New("contexto no disponible")
	}
	return a.runtime.RunCompare(a.ctx)
}

func (a *App) ExportLastScan() *monitor.ScanResult {
	return a.runtime.LastScan()
}

func (a *App) GetConfigSummary() ConfigSummary {
	if a.cfg == nil {
		return ConfigSummary{AppName: "sycronizafhir"}
	}

	return ConfigSummary{
		AppName:       "sycronizafhir",
		LocalDB:       summarizePostgresURL(a.cfg.LocalPostgresURL),
		RemoteDB:      summarizePostgresURL(a.cfg.SupabaseDBDSN()),
		SourceSchema:  a.cfg.SourceSchema,
		ExcludeTables: append([]string{}, a.cfg.ExcludeTables...),
		OutboundEvery: a.cfg.OutboundInterval.String(),
		RealtimeURL:   redactSensitive(a.cfg.SupabaseRealtimeURL),
		Channel:       a.cfg.RealtimeChannel,
		Schema:        a.cfg.RealtimeSchema,
		Table:         a.cfg.RealtimeTable,
	}
}

func (a *App) GetLocalConnectionDraft() LocalConnectionInput {
	if a.cfg == nil {
		return LocalConnectionInput{
			Host:     "127.0.0.1",
			Port:     5432,
			User:     "postgres",
			Database: "postgres",
			SSLMode:  "disable",
		}
	}
	return parseLocalDSN(a.cfg.LocalPostgresURL)
}

func (a *App) TestLocalConnection(input LocalConnectionInput) LocalConnectionResult {
	dsn, err := buildLocalDSN(input)
	if err != nil {
		return LocalConnectionResult{Success: false, Message: err.Error()}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return LocalConnectionResult{Success: false, Message: err.Error()}
	}
	defer pool.Close()

	if err = pool.Ping(ctx); err != nil {
		return LocalConnectionResult{Success: false, Message: err.Error()}
	}

	return LocalConnectionResult{
		Success: true,
		Message: "Conexion local OK",
		DSN:     dsn,
	}
}

func (a *App) ListLocalDatabases(input LocalConnectionInput) LocalConnectionResult {
	dsn, err := buildLocalDSN(input)
	if err != nil {
		return LocalConnectionResult{Success: false, Message: err.Error()}
	}

	parsed, err := url.Parse(dsn)
	if err != nil {
		return LocalConnectionResult{Success: false, Message: err.Error()}
	}
	parsed.Path = "/postgres"
	adminDSN := parsed.String()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, adminDSN)
	if err != nil {
		return LocalConnectionResult{Success: false, Message: err.Error()}
	}
	defer pool.Close()

	if err = pool.Ping(ctx); err != nil {
		return LocalConnectionResult{Success: false, Message: err.Error()}
	}

	rows, err := pool.Query(ctx, `
		SELECT datname
		FROM pg_database
		WHERE datistemplate = false
		  AND datallowconn = true
		ORDER BY datname ASC
	`)
	if err != nil {
		return LocalConnectionResult{Success: false, Message: err.Error()}
	}
	defer rows.Close()

	dbs := make([]string, 0)
	for rows.Next() {
		var name string
		if scanErr := rows.Scan(&name); scanErr != nil {
			return LocalConnectionResult{Success: false, Message: scanErr.Error()}
		}
		dbs = append(dbs, name)
	}
	if rows.Err() != nil {
		return LocalConnectionResult{Success: false, Message: rows.Err().Error()}
	}
	sort.Strings(dbs)

	return LocalConnectionResult{
		Success: true,
		Message: "Bases detectadas",
		DBs:     dbs,
		DSN:     dsn,
	}
}

func (a *App) SaveLocalConnection(input LocalConnectionInput) LocalConnectionResult {
	test := a.TestLocalConnection(input)
	if !test.Success {
		return test
	}

	if err := config.SaveLocalDBOverride(config.LocalDBOverride{
		LocalPostgresURL: test.DSN,
	}); err != nil {
		return LocalConnectionResult{Success: false, Message: err.Error()}
	}

	if a.cfg != nil {
		a.cfg.LocalPostgresURL = test.DSN
	}
	a.runtime.SetMeta("local_db", summarizePostgresURL(test.DSN))
	a.runtime.AddLog("conexion local actualizada desde UI")

	return LocalConnectionResult{
		Success: true,
		Message: "Configuracion guardada. Reinicia la app para aplicar en workers ya iniciados.",
		DSN:     test.DSN,
	}
}

func summarizePostgresURL(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "invalid connection string"
	}

	dbName := strings.TrimPrefix(parsed.Path, "/")
	if dbName == "" {
		dbName = "postgres"
	}

	username := parsed.User.Username()
	if username == "" {
		username = "unknown-user"
	}

	host := parsed.Host
	if host == "" {
		host = "unknown-host"
	}

	return fmt.Sprintf("%s@%s/%s", username, host, dbName)
}

func redactSensitive(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "***"
	}

	parsed.RawQuery = ""
	return parsed.String()
}

func parseLocalDSN(raw string) LocalConnectionInput {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return LocalConnectionInput{
			Host:     "127.0.0.1",
			Port:     5432,
			User:     "postgres",
			Database: "postgres",
			SSLMode:  "disable",
		}
	}

	host := parsed.Hostname()
	if host == "" {
		host = "127.0.0.1"
	}
	port := 5432
	if parsed.Port() != "" {
		if p, convErr := strconv.Atoi(parsed.Port()); convErr == nil {
			port = p
		}
	}
	user := parsed.User.Username()
	password, _ := parsed.User.Password()
	db := strings.TrimPrefix(parsed.Path, "/")
	if db == "" {
		db = "postgres"
	}
	sslMode := parsed.Query().Get("sslmode")
	if sslMode == "" {
		sslMode = "disable"
	}

	return LocalConnectionInput{
		Host:     host,
		Port:     port,
		User:     user,
		Password: password,
		Database: db,
		SSLMode:  sslMode,
	}
}

func buildLocalDSN(input LocalConnectionInput) (string, error) {
	host := strings.TrimSpace(input.Host)
	if host == "" {
		return "", errors.New("host requerido")
	}
	port := input.Port
	if port <= 0 {
		port = 5432
	}
	user := strings.TrimSpace(input.User)
	if user == "" {
		return "", errors.New("usuario requerido")
	}
	password := strings.TrimSpace(input.Password)
	if password == "" {
		return "", errors.New("password requerido")
	}
	database := strings.TrimSpace(input.Database)
	if database == "" {
		return "", errors.New("base requerida")
	}
	sslMode := strings.TrimSpace(strings.ToLower(input.SSLMode))
	if sslMode == "" {
		sslMode = "disable"
	}

	escapedUser := url.QueryEscape(user)
	escapedPassword := url.QueryEscape(password)
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		escapedUser,
		escapedPassword,
		host,
		port,
		database,
		sslMode,
	), nil
}
