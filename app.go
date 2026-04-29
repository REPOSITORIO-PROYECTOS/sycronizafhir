package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

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
