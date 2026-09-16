package sync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"sycronizafhir/internal/config"
	"sycronizafhir/internal/db"
	"sycronizafhir/internal/monitor"
	"sycronizafhir/internal/supabase"
	"sycronizafhir/internal/support"
)

const outboundStateKey = "outbound_last_run_utc"
const outboundGenericDirection = "outbound_generic"

// Cola por clave: siempre reenviar los últimos N ped_id aunque fecha_modificacion no avance.
const outboundPedidosTailLimit = 20

// Tope por tabla para que un upsert/red colgado no frene el ciclo entero (pedidos incluidos).
const outboundTableTimeout = 90 * time.Second

type OutboundWorker struct {
	localPG       *db.LocalPG
	queue         *db.QueueSQLite
	pgClient      *supabase.PGClient
	imageResolver *ImageResolver
	pollInterval  time.Duration
	sourceSchema  string
	excludeTables []string
	tableSince    map[string]time.Time
	runtime       *monitor.Runtime
}

type queuedOutboundPayload struct {
	TableName       string                   `json:"table_name"`
	ConflictColumns []string                 `json:"conflict_columns"`
	Rows            []map[string]interface{} `json:"rows"`
}

type outboundCycleStats struct {
	productIDs []string
}

func NewOutboundWorker(
	localPG *db.LocalPG,
	queue *db.QueueSQLite,
	pgClient *supabase.PGClient,
	imageResolver *ImageResolver,
	cfg config.Config,
	runtime *monitor.Runtime,
) *OutboundWorker {
	return &OutboundWorker{
		localPG:       localPG,
		queue:         queue,
		pgClient:      pgClient,
		imageResolver: imageResolver,
		pollInterval:  cfg.OutboundInterval,
		sourceSchema:  cfg.SourceSchema,
		excludeTables: cfg.ExcludeTables,
		tableSince:    make(map[string]time.Time),
		runtime:       runtime,
	}
}

func (w *OutboundWorker) Run(ctx context.Context) {
	if err := w.loadCheckpoints(ctx); err != nil {
		log.Printf("load outbound checkpoints failed, using startup window: %v", err)
	}
	w.runtime.SetComponentStatus("outbound", "running", "worker iniciado")
	_ = support.WriteComponentState("outbound", "running", "worker iniciado", nil)

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	if err := w.runCycle(ctx); err != nil {
		log.Printf("outbound initial cycle failed: %v", err)
		_ = support.WriteComponentState("outbound", "error", err.Error(), nil)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.runCycle(ctx); err != nil {
				log.Printf("outbound cycle failed: %v", err)
				w.runtime.SetComponentStatus("outbound", "error", err.Error())
				_ = support.WriteComponentState("outbound", "error", err.Error(), nil)
			} else {
				w.runtime.SetComponentStatus("outbound", "running", "ciclo OK")
			}
		}
	}
}

func (w *OutboundWorker) runCycle(ctx context.Context) error {
	if err := w.retryQueuedOutbound(ctx); err != nil {
		log.Printf("retry queued outbound completed with errors: %v", err)
		w.runtime.AddLog(fmt.Sprintf("outbound retry queue warning: %v", err))
	}

	syncCfg, err := config.LoadSyncTablesConfig()
	if err != nil {
		return err
	}

	tables, err := w.localPG.ListSyncTables(ctx, w.sourceSchema, w.excludeTables)
	if err != nil {
		return err
	}

	failedTables := make([]string, 0)
	sentRows := 0
	tablesWithChanges := 0
	stats := outboundCycleStats{}
	for _, table := range tables {
		if !syncCfg.IsEnabled(table.Name) {
			continue
		}
		if skipPedidoPaginaGenericOutbound(table.Name) {
			continue
		}

		since := w.tableSinceFor(table.Name)
		rows, readErr := w.localPG.LoadUpdatedRows(ctx, w.sourceSchema, table.Name, since)
		if readErr != nil {
			return readErr
		}

		if table.Name == "pedidos" {
			rows = w.mergePedidosTailByPedID(ctx, rows, table.PrimaryKeys)
		}
		if table.Name == "pedidos_d" {
			rows = w.mergePedidosDTailByPedID(ctx, rows)
		}

		if len(rows) == 0 {
			continue
		}

		tableCtx, cancel := context.WithTimeout(ctx, outboundTableTimeout)

		if table.Name == "productos" && w.imageResolver != nil && w.imageResolver.Enabled() {
			rows = w.imageResolver.ResolveProductRows(tableCtx, rows)
		}

		if fields := syncCfg.CloudOwnedFieldsFor(table.Name); len(fields) > 0 {
			preserved, guardErr := applyCloudOwnedOutboundGuard(tableCtx, w.pgClient, "public", table.Name, table.PrimaryKeys, rows, fields)
			if guardErr != nil {
				log.Printf("outbound %s cloud-owned guard skipped: %v", table.Name, guardErr)
				w.runtime.AddLog(fmt.Sprintf("outbound %s: guarda campos nube omitida (%v)", table.Name, guardErr))
			} else if preserved > 0 {
				w.runtime.AddLog(fmt.Sprintf("outbound %s: preservados %d campos propiedad de la nube (no pisar)", table.Name, preserved))
			}
		}

		if fields := syncCfg.CloudAuthoritativeFieldsFor(table.Name); len(fields) > 0 {
			preserved, guardErr := applyCloudAuthoritativeOutboundGuard(tableCtx, w.pgClient, "public", table.Name, table.PrimaryKeys, rows, fields)
			if guardErr != nil {
				log.Printf("outbound %s cloud-authoritative guard skipped: %v", table.Name, guardErr)
				w.runtime.AddLog(fmt.Sprintf("outbound %s: guarda campos autoritativos nube omitida (%v)", table.Name, guardErr))
			} else if preserved > 0 {
				w.runtime.AddLog(fmt.Sprintf("outbound %s: preservados %d campos autoritativos nube (prod_orden)", table.Name, preserved))
			}
		}

		if preserved, guardErr := applyPedidosPickingEstadoOutboundGuard(tableCtx, w.pgClient, "public", table.Name, table.PrimaryKeys, rows); guardErr != nil {
			log.Printf("outbound %s picking-estado guard skipped: %v", table.Name, guardErr)
			w.runtime.AddLog(fmt.Sprintf("outbound %s: guarda estado picking omitida (%v)", table.Name, guardErr))
		} else if preserved > 0 {
			w.runtime.AddLog(fmt.Sprintf("outbound %s: preservados %d estado(s) picking en nube (no pisar K/V/E)", table.Name, preserved))
		}

		err = w.pgClient.UpsertRows(tableCtx, "public", table.Name, rows, table.PrimaryKeys)
		cancel()
		if err != nil {
			payload := queuedOutboundPayload{
				TableName:       table.Name,
				ConflictColumns: table.PrimaryKeys,
				Rows:            rows,
			}
			raw, marshalErr := json.Marshal(payload)
			if marshalErr == nil {
				_ = w.queue.Enqueue(ctx, outboundGenericDirection, string(raw))
			}
			failedTables = append(failedTables, table.Name)
			if errors.Is(err, context.DeadlineExceeded) {
				log.Printf("outbound table upsert timeout for %s after %s: %v", table.Name, outboundTableTimeout, err)
				w.runtime.AddLog(fmt.Sprintf("outbound table %s timeout %s (queued): %v", table.Name, outboundTableTimeout, err))
			} else {
				log.Printf("outbound table upsert failed for %s: %v", table.Name, err)
				w.runtime.AddLog(fmt.Sprintf("outbound table %s queued after upsert error: %v", table.Name, err))
			}
			continue
		}

		if advanceErr := w.advanceTableCheckpoint(ctx, table.Name, rows); advanceErr != nil {
			log.Printf("persist outbound checkpoint for %s failed: %v", table.Name, advanceErr)
		}

		sentRows += len(rows)
		tablesWithChanges++
		if table.Name == "productos" {
			stats.productIDs = collectProductoIDs(rows, 20)
		}
		w.runtime.AddLog(fmt.Sprintf("outbound: subidas %d filas a %s", len(rows), table.Name))
	}

	if sentRows == 0 {
		w.runtime.AddLog("outbound: ciclo sin cambios (0 filas con fecha_modificacion reciente)")
	} else {
		w.runtime.AddLog(fmt.Sprintf("outbound: ciclo OK — %d filas en %d tabla(s)", sentRows, tablesWithChanges))
	}

	details := w.componentStateDetails(syncCfg, sentRows, tablesWithChanges, failedTables, stats)

	if len(failedTables) > 0 {
		_ = support.WriteComponentState(
			"outbound",
			"error",
			fmt.Sprintf("ciclo con errores en: %s", strings.Join(failedTables, ", ")),
			details,
		)
		return fmt.Errorf("outbound completed with queued errors for tables: %s", strings.Join(failedTables, ", "))
	}

	message := "ciclo sin cambios"
	if sentRows > 0 {
		message = fmt.Sprintf("ciclo OK - %d filas en %d tabla(s)", sentRows, tablesWithChanges)
	}
	_ = support.WriteComponentState("outbound", "running", message, details)
	return nil
}

func (w *OutboundWorker) retryQueuedOutbound(ctx context.Context) error {
	jobs, err := w.queue.PeekByDirection(ctx, outboundGenericDirection, 100)
	if err != nil {
		return err
	}

	syncCfg, _ := config.LoadSyncTablesConfig()

	failedJobs := make([]string, 0)
	for _, job := range jobs {
		var payload queuedOutboundPayload
		if err = json.Unmarshal([]byte(job.PayloadJSON), &payload); err != nil {
			_ = w.queue.Delete(ctx, job.ID)
			continue
		}

		rows := payload.Rows
		if skipPedidoPaginaGenericOutbound(payload.TableName) {
			if payload.TableName != pedidoPaginaHeadTable {
				_ = w.queue.Delete(ctx, job.ID)
				w.runtime.AddLog(fmt.Sprintf("retry outbound: drop cola %s (detalle nube, no upsert)", payload.TableName))
				continue
			}
			rows = restrictPedidoPaginaOutboundRows(rows)
			if len(rows) == 0 {
				_ = w.queue.Delete(ctx, job.ID)
				continue
			}
			patched := 0
			jobFailed := false
			for _, row := range rows {
				ok, patchErr := w.pgClient.PatchExistingColumns(ctx, "public", pedidoPaginaHeadTable, []string{"pedido_id"}, row)
				if patchErr != nil {
					jobFailed = true
					log.Printf("retry queued outbound job failed id=%d table=%s: %v", job.ID, payload.TableName, patchErr)
					continue
				}
				if ok {
					patched++
				}
			}
			if jobFailed {
				failedJobs = append(failedJobs, fmt.Sprintf("%d:%s", job.ID, payload.TableName))
				continue
			}
			if patched > 0 {
				w.runtime.AddLog(fmt.Sprintf("retry outbound pedido_pagina: %d estado(s) PATCH", patched))
			}
			if err = w.queue.Delete(ctx, job.ID); err != nil {
				return err
			}
			continue
		}

		if payload.TableName == "productos" && w.imageResolver != nil && w.imageResolver.Enabled() {
			rows = w.imageResolver.ResolveProductRows(ctx, rows)
		}

		if fields := syncCfg.CloudOwnedFieldsFor(payload.TableName); len(fields) > 0 {
			preserved, guardErr := applyCloudOwnedOutboundGuard(ctx, w.pgClient, "public", payload.TableName, payload.ConflictColumns, rows, fields)
			if guardErr != nil {
				log.Printf("retry outbound %s cloud-owned guard skipped: %v", payload.TableName, guardErr)
			} else if preserved > 0 {
				w.runtime.AddLog(fmt.Sprintf("retry outbound %s: preservados %d campos propiedad de la nube", payload.TableName, preserved))
			}
		}

		if fields := syncCfg.CloudAuthoritativeFieldsFor(payload.TableName); len(fields) > 0 {
			preserved, guardErr := applyCloudAuthoritativeOutboundGuard(ctx, w.pgClient, "public", payload.TableName, payload.ConflictColumns, rows, fields)
			if guardErr != nil {
				log.Printf("retry outbound %s cloud-authoritative guard skipped: %v", payload.TableName, guardErr)
			} else if preserved > 0 {
				w.runtime.AddLog(fmt.Sprintf("retry outbound %s: preservados %d campos autoritativos nube", payload.TableName, preserved))
			}
		}

		if preserved, guardErr := applyPedidosPickingEstadoOutboundGuard(ctx, w.pgClient, "public", payload.TableName, payload.ConflictColumns, rows); guardErr != nil {
			log.Printf("retry outbound %s picking-estado guard skipped: %v", payload.TableName, guardErr)
		} else if preserved > 0 {
			w.runtime.AddLog(fmt.Sprintf("retry outbound %s: preservados %d estado(s) picking en nube", payload.TableName, preserved))
		}

		if err = w.pgClient.UpsertRows(ctx, "public", payload.TableName, rows, payload.ConflictColumns); err != nil {
			failedJobs = append(failedJobs, fmt.Sprintf("%d:%s", job.ID, payload.TableName))
			log.Printf("retry queued outbound job failed id=%d table=%s: %v", job.ID, payload.TableName, err)
			w.runtime.AddLog(fmt.Sprintf("retry queued outbound failed id=%d table=%s: %v", job.ID, payload.TableName, err))
			continue
		}

		if advanceErr := w.advanceTableCheckpoint(ctx, payload.TableName, rows); advanceErr != nil {
			log.Printf("persist outbound checkpoint for %s after retry failed: %v", payload.TableName, advanceErr)
		}

		if err = w.queue.Delete(ctx, job.ID); err != nil {
			return err
		}
	}

	if len(failedJobs) > 0 {
		return fmt.Errorf("queued outbound jobs still failing: %s", strings.Join(failedJobs, ", "))
	}

	return nil
}

func (w *OutboundWorker) tableSinceFor(tableName string) time.Time {
	if since, ok := w.tableSince[tableName]; ok && !since.IsZero() {
		return since
	}
	return time.Now().UTC().Add(-24 * time.Hour)
}

func (w *OutboundWorker) advanceTableCheckpoint(ctx context.Context, tableName string, rows []map[string]interface{}) error {
	meta, err := w.localPG.LoadTableModifiedAtMeta(ctx, w.sourceSchema, tableName)
	if err != nil {
		return err
	}

	maxAt, ok := db.MaxRowModifiedAt(rows, meta)
	if !ok {
		return nil
	}

	current := w.tableSinceFor(tableName)
	if !current.IsZero() && !maxAt.After(current) {
		return nil
	}

	maxAt = maxAt.UTC()
	w.tableSince[tableName] = maxAt
	return w.persistTableCheckpoint(ctx, tableName, maxAt)
}

func (w *OutboundWorker) loadCheckpoints(ctx context.Context) error {
	tables, err := w.localPG.ListSyncTables(ctx, w.sourceSchema, w.excludeTables)
	if err != nil {
		return err
	}

	legacyGlobal, hasLegacy, err := w.readCheckpoint(ctx, outboundStateKey)
	if err != nil {
		return err
	}

	for _, table := range tables {
		tableKey := outboundTableStateKey(table.Name)
		since, exists, readErr := w.readCheckpoint(ctx, tableKey)
		if readErr != nil {
			return readErr
		}
		switch {
		case exists:
			w.tableSince[table.Name] = since
		case hasLegacy:
			w.tableSince[table.Name] = legacyGlobal
		default:
			w.tableSince[table.Name] = time.Now().UTC().Add(-24 * time.Hour)
		}
	}
	return nil
}

func (w *OutboundWorker) readCheckpoint(ctx context.Context, key string) (time.Time, bool, error) {
	rawValue, exists, err := w.queue.GetStateValue(ctx, key)
	if err != nil {
		return time.Time{}, false, err
	}
	if !exists || strings.TrimSpace(rawValue) == "" {
		return time.Time{}, false, nil
	}

	parsed, err := time.Parse(time.RFC3339Nano, rawValue)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("parse checkpoint %s: %w", key, err)
	}
	return parsed.UTC(), true, nil
}

func (w *OutboundWorker) persistTableCheckpoint(ctx context.Context, tableName string, value time.Time) error {
	return w.queue.SetStateValue(ctx, outboundTableStateKey(tableName), value.UTC().Format(time.RFC3339Nano))
}

func outboundTableStateKey(tableName string) string {
	return outboundStateKey + "_" + tableName
}

func (w *OutboundWorker) componentStateDetails(
	syncCfg config.SyncTablesConfig,
	sentRows int,
	tablesWithChanges int,
	failedTables []string,
	stats outboundCycleStats,
) map[string]string {
	details := map[string]string{
		"sent_rows":            fmt.Sprintf("%d", sentRows),
		"tables_with_changes":  fmt.Sprintf("%d", tablesWithChanges),
		"poll_interval":        w.pollInterval.String(),
		"productos_enabled":    fmt.Sprintf("%t", syncCfg.IsEnabled("productos")),
		"productos_checkpoint": "",
	}

	enabled := make([]string, 0)
	for _, name := range syncCfg.EnabledTables {
		trimmed := strings.TrimSpace(name)
		if trimmed != "" {
			enabled = append(enabled, trimmed)
		}
	}
	if len(enabled) > 0 {
		details["enabled_tables"] = strings.Join(enabled, ",")
	}
	if len(failedTables) > 0 {
		details["failed_tables"] = strings.Join(failedTables, ",")
	}

	if checkpoint, ok := w.tableSince["productos"]; ok && !checkpoint.IsZero() {
		details["productos_checkpoint"] = checkpoint.UTC().Format(time.RFC3339Nano)
	}
	if len(stats.productIDs) > 0 {
		details["productos_updated_ids"] = strings.Join(stats.productIDs, ",")
		details["productos_updated_count"] = fmt.Sprintf("%d", len(stats.productIDs))
	}

	return details
}

func (w *OutboundWorker) mergePedidosTailByPedID(
	ctx context.Context,
	rows []map[string]interface{},
	pkColumns []string,
) []map[string]interface{} {
	tail, err := w.localPG.LoadLatestRowsByColumn(ctx, w.sourceSchema, "pedidos", "ped_id", outboundPedidosTailLimit)
	if err != nil {
		log.Printf("outbound pedidos tail by ped_id failed: %v", err)
		w.runtime.AddLog(fmt.Sprintf("outbound pedidos: cola ped_id omitida (%v)", err))
		return rows
	}
	if len(tail) == 0 {
		return rows
	}
	before := len(rows)
	merged := mergeRowsByPrimaryKeys(rows, tail, pkColumns)
	added := len(merged) - before
	if added > 0 {
		w.runtime.AddLog(fmt.Sprintf("outbound pedidos: cola por ped_id +%d (tail=%d)", added, len(tail)))
	}
	return merged
}

func (w *OutboundWorker) mergePedidosDTailByPedID(
	ctx context.Context,
	rows []map[string]interface{},
) []map[string]interface{} {
	heads, err := w.localPG.LoadLatestRowsByColumn(ctx, w.sourceSchema, "pedidos", "ped_id", outboundPedidosTailLimit)
	if err != nil {
		log.Printf("outbound pedidos_d tail heads failed: %v", err)
		return rows
	}
	values := make([]interface{}, 0, len(heads))
	seen := make(map[string]struct{}, len(heads))
	for _, head := range heads {
		raw, ok := head["ped_id"]
		if !ok || raw == nil {
			continue
		}
		key := fmt.Sprint(raw)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		values = append(values, raw)
	}
	if len(values) == 0 {
		return rows
	}
	tail, err := w.localPG.LoadRowsWhereColumnIn(ctx, w.sourceSchema, "pedidos_d", "ped_id", values)
	if err != nil {
		log.Printf("outbound pedidos_d tail by ped_id failed: %v", err)
		w.runtime.AddLog(fmt.Sprintf("outbound pedidos_d: cola ped_id omitida (%v)", err))
		return rows
	}
	if len(tail) == 0 {
		return rows
	}
	before := len(rows)
	merged := mergeRowsByPrimaryKeys(rows, tail, []string{"ped_id", "ped_item"})
	added := len(merged) - before
	if added > 0 {
		w.runtime.AddLog(fmt.Sprintf("outbound pedidos_d: cola por ped_id +%d (tail_lines=%d heads=%d)", added, len(tail), len(values)))
	}
	return merged
}

func mergeRowsByPrimaryKeys(
	base, extra []map[string]interface{},
	pkColumns []string,
) []map[string]interface{} {
	if len(extra) == 0 {
		return base
	}
	if len(pkColumns) == 0 {
		return append(append([]map[string]interface{}{}, base...), extra...)
	}
	out := make([]map[string]interface{}, 0, len(base)+len(extra))
	seen := make(map[string]struct{}, len(base)+len(extra))
	appendUnique := func(row map[string]interface{}) {
		key, err := PKKey(row, pkColumns)
		if err != nil {
			out = append(out, row)
			return
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, row)
	}
	for _, row := range base {
		appendUnique(row)
	}
	for _, row := range extra {
		appendUnique(row)
	}
	return out
}

func collectProductoIDs(rows []map[string]interface{}, maxItems int) []string {
	if maxItems <= 0 {
		return nil
	}
	out := make([]string, 0, maxItems)
	seen := map[string]bool{}
	for _, row := range rows {
		raw := strings.TrimSpace(fmt.Sprintf("%v", row["prod_id"]))
		if raw == "" || raw == "<nil>" || seen[raw] {
			continue
		}
		seen[raw] = true
		out = append(out, raw)
		if len(out) >= maxItems {
			break
		}
	}
	return out
}
