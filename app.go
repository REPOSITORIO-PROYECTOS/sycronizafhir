package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"sycronizafhir/internal/config"
	"sycronizafhir/internal/db"
	"sycronizafhir/internal/monitor"
	"sycronizafhir/internal/supabase"
	syncworker "sycronizafhir/internal/sync"
)

type App struct {
	ctx             context.Context
	runtime         *monitor.Runtime
	cfg             *config.Config
	queue           *db.QueueSQLite
	bootstrapStore  *db.QueueSQLite
	bootstrapMu     sync.Mutex
	bootstrapState  syncworker.BootstrapStatus
	bootstrapActive bool
	auditMu         sync.Mutex
	auditActive     bool
	lastAudit       syncworker.DataAuditReport
}

type ConfigSummary struct {
	AppName       string   `json:"app_name"`
	LocalDB       string   `json:"local_db"`
	RemoteDB      string   `json:"remote_db"`
	SourceSchema  string   `json:"source_schema"`
	ExcludeTables []string `json:"exclude_tables"`
	OutboundEvery          string `json:"outbound_every"`
	AuditEvery             string `json:"audit_every"`
	ImageSyncEvery         string `json:"image_sync_every"`
	StorageBucketProductos string `json:"storage_bucket_productos"`
	ImageSyncEnabled       bool   `json:"image_sync_enabled"`
	RealtimeURL            string `json:"realtime_url"`
	Channel       string   `json:"channel"`
	Schema        string   `json:"schema"`
	Table         string   `json:"table"`
}

type SyncTablesConfigDTO struct {
	EnabledTables          []string          `json:"enabled_tables"`
	TableMappings          map[string]string `json:"table_mappings"`
	AutoAuditIntervalHours int               `json:"auto_audit_interval_hours"`
	AutoSyncOnAudit        bool              `json:"auto_sync_on_audit"`
}

type AvailableSyncTable struct {
	Name        string `json:"name"`
	RemoteName  string `json:"remote_name"`
	PrimaryKeys []string `json:"primary_keys"`
	Enabled     bool   `json:"enabled"`
}

type DataAuditActionResult struct {
	Success bool                        `json:"success"`
	Message string                      `json:"message"`
	Report  syncworker.DataAuditReport  `json:"report"`
}

type SyncSelectedResult struct {
	Success    bool   `json:"success"`
	Message    string `json:"message"`
	SyncedRows int    `json:"synced_rows"`
}

type ImageSyncResult struct {
	Success bool                     `json:"success"`
	Message string                   `json:"message"`
	Stats   syncworker.ImageSyncStats `json:"stats"`
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

type DatabaseSourceResult struct {
	Success    bool                 `json:"success"`
	Message    string               `json:"message"`
	Selected   *db.SourceCandidate  `json:"selected,omitempty"`
	Candidates []db.SourceCandidate `json:"candidates,omitempty"`
}

func NewApp(rt *monitor.Runtime, cfg *config.Config, queue, bootstrapStore *db.QueueSQLite) *App {
	return &App{
		runtime:        rt,
		cfg:            cfg,
		queue:          queue,
		bootstrapStore: bootstrapStore,
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

	a.wireSupportRecorder()
	a.runtime.AddLog("frontend conectado")
	go a.resumeBootstrapIfNeeded()
	a.loadPersistedAuditReport()
}

func (a *App) loadPersistedAuditReport() {
	if a.queue == nil {
		return
	}
	report, exists, err := syncworker.LoadAuditReport(context.Background(), a.queue)
	if err != nil || !exists {
		return
	}
	a.auditMu.Lock()
	a.lastAudit = report
	a.auditMu.Unlock()
}

func (a *App) GetSyncTablesConfig() SyncTablesConfigDTO {
	cfg, err := config.LoadSyncTablesConfig()
	if err != nil {
		return toSyncTablesDTO(config.DefaultSyncTablesConfig())
	}
	return toSyncTablesDTO(cfg)
}

func (a *App) SaveSyncTablesConfig(input SyncTablesConfigDTO) SyncTablesConfigDTO {
	cfg := config.SyncTablesConfig{
		EnabledTables:          input.EnabledTables,
		TableMappings:          input.TableMappings,
		AutoAuditIntervalHours: input.AutoAuditIntervalHours,
		AutoSyncOnAudit:        input.AutoSyncOnAudit,
	}
	if saveErr := config.SaveSyncTablesConfig(cfg); saveErr != nil {
		a.runtime.AddLog("sync tables config save failed: " + saveErr.Error())
	}
	return toSyncTablesDTO(cfg)
}

func (a *App) ListAvailableSyncTables() ([]AvailableSyncTable, error) {
	if a.cfg == nil {
		return nil, errors.New("config no cargada")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	localPG, remotePG, err := a.openReconcileConnections(ctx)
	if err != nil {
		return nil, err
	}
	defer localPG.Close()
	defer remotePG.Close()

	syncCfg, _ := config.LoadSyncTablesConfig()
	service := syncworker.NewReconcileService(localPG, remotePG, a.newImageResolver(), a.cfg.SourceSchema, a.cfg.ExcludeTables, a.runtime)
	tables, err := service.ListAvailableTables(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]AvailableSyncTable, 0, len(tables))
	for _, table := range tables {
		result = append(result, AvailableSyncTable{
			Name:        table.Name,
			RemoteName:  syncCfg.ResolveRemoteTable(table.Name),
			PrimaryKeys: append([]string{}, table.PrimaryKeys...),
			Enabled:     syncCfg.IsEnabled(table.Name),
		})
	}
	return result, nil
}

func (a *App) RunDataAudit(applySync bool) DataAuditActionResult {
	if a.cfg == nil {
		return DataAuditActionResult{Success: false, Message: "config no cargada"}
	}

	a.auditMu.Lock()
	if a.auditActive {
		a.auditMu.Unlock()
		return DataAuditActionResult{Success: false, Message: "auditoria ya en curso"}
	}
	a.auditActive = true
	a.auditMu.Unlock()

	defer func() {
		a.auditMu.Lock()
		a.auditActive = false
		a.auditMu.Unlock()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	localPG, remotePG, err := a.openReconcileConnections(ctx)
	if err != nil {
		return DataAuditActionResult{Success: false, Message: err.Error()}
	}
	defer localPG.Close()
	defer remotePG.Close()

	syncCfg, _ := config.LoadSyncTablesConfig()
	service := syncworker.NewReconcileService(localPG, remotePG, a.newImageResolver(), a.cfg.SourceSchema, a.cfg.ExcludeTables, a.runtime)
	report, err := service.RunAudit(ctx, syncCfg, "manual", applySync)
	if err != nil {
		return DataAuditActionResult{Success: false, Message: err.Error()}
	}

	if a.queue != nil {
		_ = syncworker.SaveAuditReport(ctx, a.queue, report)
	}

	a.auditMu.Lock()
	a.lastAudit = report
	a.auditMu.Unlock()

	return DataAuditActionResult{
		Success: true,
		Message: report.Summary,
		Report:  report,
	}
}

func (a *App) GetLastDataAudit() syncworker.DataAuditReport {
	a.auditMu.Lock()
	memReport := a.lastAudit
	a.auditMu.Unlock()

	var persisted syncworker.DataAuditReport
	if a.queue != nil {
		report, exists, err := syncworker.LoadAuditReport(context.Background(), a.queue)
		if err == nil && exists {
			persisted = report
		}
	}

	newest := memReport
	if persisted.AuditedAt.After(newest.AuditedAt) {
		newest = persisted
		a.auditMu.Lock()
		if persisted.AuditedAt.After(a.lastAudit.AuditedAt) {
			a.lastAudit = persisted
		}
		a.auditMu.Unlock()
	}

	if newest.AuditedAt.IsZero() {
		return syncworker.DataAuditReport{}
	}
	return newest
}

func (a *App) SyncSelectedTables(tableNames []string) SyncSelectedResult {
	if a.cfg == nil {
		return SyncSelectedResult{Success: false, Message: "config no cargada"}
	}
	if len(tableNames) == 0 {
		return SyncSelectedResult{Success: false, Message: "selecciona al menos una tabla"}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	localPG, remotePG, err := a.openReconcileConnections(ctx)
	if err != nil {
		return SyncSelectedResult{Success: false, Message: err.Error()}
	}
	defer localPG.Close()
	defer remotePG.Close()

	syncCfg, _ := config.LoadSyncTablesConfig()
	service := syncworker.NewReconcileService(localPG, remotePG, a.newImageResolver(), a.cfg.SourceSchema, a.cfg.ExcludeTables, a.runtime)
	synced, err := service.SyncSelectedTables(ctx, syncCfg, tableNames)
	if err != nil {
		return SyncSelectedResult{Success: false, Message: err.Error(), SyncedRows: synced}
	}

	report, auditErr := service.RunAudit(ctx, syncCfg, "post-sync", false)
	if auditErr == nil {
		if a.queue != nil {
			_ = syncworker.SaveAuditReport(ctx, a.queue, report)
		}
		a.auditMu.Lock()
		a.lastAudit = report
		a.auditMu.Unlock()
	} else {
		a.runtime.AddLog(fmt.Sprintf("post-sync audit failed: %v", auditErr))
	}

	message := fmt.Sprintf("Sincronizadas %d filas en %d tabla(s)", synced, len(tableNames))
	if auditErr == nil {
		message += fmt.Sprintf(". Re-audit: %s", report.Summary)
	}
	a.runtime.AddLog(message)
	return SyncSelectedResult{Success: true, Message: message, SyncedRows: synced}
}

func (a *App) SyncProductImagesNow(force bool) ImageSyncResult {
	if a.cfg == nil {
		return ImageSyncResult{Success: false, Message: "config no cargada"}
	}
	if !a.cfg.ImageSyncEnabled {
		return ImageSyncResult{Success: false, Message: "image sync deshabilitado"}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	localPG, remotePG, err := a.openReconcileConnections(ctx)
	if err != nil {
		return ImageSyncResult{Success: false, Message: err.Error()}
	}
	defer localPG.Close()
	defer remotePG.Close()

	worker := syncworker.NewImageSyncWorker(
		localPG,
		remotePG,
		a.queue,
		a.newImageResolver(),
		*a.cfg,
		a.runtime,
	)
	stats, err := worker.RunOnce(ctx, force)
	if err != nil {
		return ImageSyncResult{
			Success: false,
			Message: err.Error(),
			Stats:   stats,
		}
	}

	return ImageSyncResult{
		Success: true,
		Message: stats.Message,
		Stats:   stats,
	}
}

func (a *App) GetPendingProductImages() syncworker.PendingProductImagesSummary {
	if a.cfg == nil {
		return syncworker.PendingProductImagesSummary{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	localPG, remotePG, err := a.openReconcileConnections(ctx)
	if err != nil {
		a.runtime.AddLog("pending product images preview failed: " + err.Error())
		return syncworker.PendingProductImagesSummary{LocalBase: a.cfg.ImageLocalBasePath}
	}
	defer localPG.Close()
	defer remotePG.Close()

	summary, err := syncworker.PreviewPendingProductImages(
		ctx,
		localPG,
		a.queue,
		a.cfg.SourceSchema,
		a.cfg.ImageLocalBasePath,
		50,
	)
	if err != nil {
		a.runtime.AddLog("pending product images preview failed: " + err.Error())
		return syncworker.PendingProductImagesSummary{LocalBase: a.cfg.ImageLocalBasePath}
	}
	return summary
}

func (a *App) GetImageSyncStatus() syncworker.ImageSyncStats {
	if a.queue == nil {
		return syncworker.ImageSyncStats{}
	}
	stats, exists, err := syncworker.LoadImageSyncStatus(context.Background(), a.queue)
	if err != nil || !exists {
		return syncworker.ImageSyncStats{}
	}
	return stats
}

func (a *App) newImageResolver() *syncworker.ImageResolver {
	if a.cfg == nil {
		return syncworker.NewImageResolver(config.Config{}, a.queue, a.runtime)
	}
	return syncworker.NewImageResolver(*a.cfg, a.queue, a.runtime)
}

func (a *App) openReconcileConnections(ctx context.Context) (*db.LocalPG, *supabase.PGClient, error) {
	resolution, err := db.ResolveLocalPostgresSource(ctx, *a.cfg)
	if err != nil {
		return nil, nil, err
	}

	localPG, err := db.NewLocalPG(ctx, resolution.Selected.DSN)
	if err != nil {
		return nil, nil, fmt.Errorf("conexion local: %w", err)
	}

	remotePG, err := supabase.NewPGClient(ctx, a.cfg.SupabaseDBDSN())
	if err != nil {
		localPG.Close()
		return nil, nil, fmt.Errorf("conexion supabase: %w", err)
	}

	return localPG, remotePG, nil
}

func toSyncTablesDTO(cfg config.SyncTablesConfig) SyncTablesConfigDTO {
	mappings := map[string]string{}
	for key, value := range cfg.TableMappings {
		mappings[key] = value
	}
	return SyncTablesConfigDTO{
		EnabledTables:          append([]string{}, cfg.EnabledTables...),
		TableMappings:          mappings,
		AutoAuditIntervalHours: cfg.AutoAuditIntervalHours,
		AutoSyncOnAudit:        cfg.AutoSyncOnAudit,
	}
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
		AppName:                "sycronizafhir",
		LocalDB:                summarizePostgresURL(a.cfg.LocalPostgresURL),
		RemoteDB:               summarizePostgresURL(a.cfg.SupabaseDBDSN()),
		SourceSchema:           a.cfg.SourceSchema,
		ExcludeTables:          append([]string{}, a.cfg.ExcludeTables...),
		OutboundEvery:          a.cfg.OutboundInterval.String(),
		AuditEvery:             a.cfg.AuditInterval.String(),
		ImageSyncEvery:         a.cfg.ImageSyncInterval.String(),
		StorageBucketProductos: a.cfg.StorageBucketProductos,
		ImageSyncEnabled:       a.cfg.ImageSyncEnabled,
		RealtimeURL:            redactSensitive(a.cfg.SupabaseRealtimeURL),
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
			Database: "mascotas",
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

func (a *App) ResolveDatabaseSource() DatabaseSourceResult {
	if a.cfg == nil {
		return DatabaseSourceResult{Success: false, Message: "config no cargada"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	resolution, err := db.ResolveLocalPostgresSource(ctx, *a.cfg)
	if err != nil {
		return DatabaseSourceResult{
			Success:    false,
			Message:    err.Error(),
			Candidates: resolution.Candidates,
		}
	}

	a.runtime.SetMeta("local_db_source", resolution.Selected.Kind)
	a.runtime.SetMeta("local_db", summarizePostgresURL(resolution.Selected.DSN))
	return DatabaseSourceResult{
		Success:    true,
		Message:    "fuente local resuelta",
		Selected:   &resolution.Selected,
		Candidates: resolution.Candidates,
	}
}

func (a *App) StartInitialFullLoad() DatabaseSourceResult {
	if a.cfg == nil {
		return DatabaseSourceResult{Success: false, Message: "config no cargada"}
	}
	if a.ctx == nil {
		return DatabaseSourceResult{Success: false, Message: "contexto no disponible"}
	}

	ctx, cancel := context.WithTimeout(a.ctx, 15*time.Second)
	resolution, err := db.ResolveLocalPostgresSource(ctx, *a.cfg)
	cancel()
	if err != nil {
		return DatabaseSourceResult{
			Success:    false,
			Message:    err.Error(),
			Candidates: resolution.Candidates,
		}
	}

	a.bootstrapMu.Lock()
	if a.bootstrapActive {
		a.bootstrapMu.Unlock()
		return DatabaseSourceResult{Success: false, Message: "bootstrap ya en curso", Selected: &resolution.Selected}
	}
	a.bootstrapActive = true
	a.bootstrapState = syncworker.BootstrapStatus{
		State:      "running",
		SourceKind: resolution.Selected.Kind,
		StartedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	a.bootstrapMu.Unlock()

	go a.runBootstrap(resolution.Selected)
	return DatabaseSourceResult{
		Success:    true,
		Message:    "carga inicial iniciada",
		Selected:   &resolution.Selected,
		Candidates: resolution.Candidates,
	}
}

func (a *App) GetInitialLoadStatus() syncworker.BootstrapStatus {
	a.bootstrapMu.Lock()
	memState := a.bootstrapState
	a.bootstrapMu.Unlock()

	if a.cfg == nil || a.bootstrapStore == nil {
		return memState
	}

	persisted, err := syncworker.LoadBootstrapStatus(context.Background(), a.bootstrapStore)
	if err != nil || persisted.State == "pending" {
		return memState
	}
	return persisted
}

func (a *App) runBootstrap(selected db.SourceCandidate) {
	defer func() {
		a.bootstrapMu.Lock()
		a.bootstrapActive = false
		a.bootstrapMu.Unlock()
	}()

	if a.cfg == nil {
		return
	}

	ctx := context.Background()
	localPG, err := db.NewLocalPG(ctx, selected.DSN)
	if err != nil {
		a.setBootstrapFailed(fmt.Sprintf("conexion fuente bootstrap: %v", err))
		return
	}
	defer localPG.Close()

	if a.bootstrapStore == nil {
		a.setBootstrapFailed("estado bootstrap sqlite no disponible")
		return
	}

	supabasePG, err := supabase.NewPGClient(ctx, a.cfg.SupabaseDBDSN())
	if err != nil {
		a.setBootstrapFailed(fmt.Sprintf("supabase bootstrap: %v", err))
		return
	}
	defer supabasePG.Close()

	worker := syncworker.NewBootstrapWorker(localPG, a.bootstrapStore, supabasePG, a.cfg.SourceSchema, a.cfg.ExcludeTables, a.runtime, a.cfg.BootstrapChunkSize)
	status, runErr := worker.RunFullLoad(ctx, selected.Kind)

	a.bootstrapMu.Lock()
	a.bootstrapState = status
	a.bootstrapMu.Unlock()

	if runErr != nil {
		if strings.TrimSpace(status.LastError) == "" {
			a.setBootstrapFailed(runErr.Error())
			return
		}
		a.runtime.AddLog("bootstrap full load fallo: " + runErr.Error())
		return
	}
}

func (a *App) setBootstrapFailed(message string) {
	a.bootstrapMu.Lock()
	defer a.bootstrapMu.Unlock()
	a.bootstrapState.State = "failed"
	a.bootstrapState.LastError = message
	a.bootstrapState.UpdatedAt = time.Now().UTC()
	a.runtime.SetComponentStatus("bootstrap", "error", message)
}

func (a *App) resumeBootstrapIfNeeded() {
	if a.cfg == nil || a.bootstrapStore == nil {
		return
	}

	status, err := syncworker.LoadBootstrapStatus(context.Background(), a.bootstrapStore)
	if err != nil || status.State != "running" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	resolution, resolveErr := db.ResolveLocalPostgresSource(ctx, *a.cfg)
	cancel()
	if resolveErr != nil {
		a.runtime.AddLog("bootstrap: no se pudo reanudar tras reinicio: " + resolveErr.Error())
		return
	}

	a.bootstrapMu.Lock()
	if a.bootstrapActive {
		a.bootstrapMu.Unlock()
		return
	}
	a.bootstrapActive = true
	a.bootstrapState = status
	a.bootstrapMu.Unlock()

	a.runtime.AddLog(fmt.Sprintf(
		"bootstrap: reanudando carga pendiente (%s, %d/%d filas, tabla %s)",
		status.SourceKind, status.ProcessedRows, status.TotalRows, status.CurrentTable,
	))
	go a.runBootstrap(resolution.Selected)
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
			Database: "mascotas",
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
