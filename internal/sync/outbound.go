package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"sycronizafhir/internal/config"
	"sycronizafhir/internal/db"
	"sycronizafhir/internal/monitor"
	"sycronizafhir/internal/supabase"
)

const outboundStateKey = "outbound_last_run_utc"
const outboundGenericDirection = "outbound_generic"

type OutboundWorker struct {
	localPG       *db.LocalPG
	queue         *db.QueueSQLite
	pgClient      *supabase.PGClient
	pollInterval  time.Duration
	sourceSchema  string
	excludeTables []string
	lastRun       time.Time
	runtime       *monitor.Runtime
}

type queuedOutboundPayload struct {
	TableName       string                   `json:"table_name"`
	ConflictColumns []string                 `json:"conflict_columns"`
	Rows            []map[string]interface{} `json:"rows"`
}

func NewOutboundWorker(localPG *db.LocalPG, queue *db.QueueSQLite, pgClient *supabase.PGClient, cfg config.Config, runtime *monitor.Runtime) *OutboundWorker {
	return &OutboundWorker{
		localPG:       localPG,
		queue:         queue,
		pgClient:      pgClient,
		pollInterval:  cfg.OutboundInterval,
		sourceSchema:  cfg.SourceSchema,
		excludeTables: cfg.ExcludeTables,
		lastRun:       time.Now().Add(-24 * time.Hour),
		runtime:       runtime,
	}
}

func (w *OutboundWorker) Run(ctx context.Context) {
	if err := w.loadCheckpoint(ctx); err != nil {
		log.Printf("load outbound checkpoint failed, using startup window: %v", err)
	}
	w.runtime.SetComponentStatus("outbound", "running", "worker iniciado")

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	if err := w.runCycle(ctx); err != nil {
		log.Printf("outbound initial cycle failed: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.runCycle(ctx); err != nil {
				log.Printf("outbound cycle failed: %v", err)
				w.runtime.SetComponentStatus("outbound", "error", err.Error())
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

	tables, err := w.localPG.ListSyncTables(ctx, w.sourceSchema, w.excludeTables)
	if err != nil {
		return err
	}

	failedTables := make([]string, 0)
	for _, table := range tables {
		rows, readErr := w.localPG.LoadUpdatedRows(ctx, w.sourceSchema, table.Name, w.lastRun)
		if readErr != nil {
			return readErr
		}
		if len(rows) == 0 {
			continue
		}

		if err = w.pgClient.UpsertRows(ctx, "public", table.Name, rows, table.PrimaryKeys); err != nil {
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
			log.Printf("outbound table upsert failed for %s: %v", table.Name, err)
			w.runtime.AddLog(fmt.Sprintf("outbound table %s queued after upsert error: %v", table.Name, err))
		}
	}

	now := time.Now().UTC()
	w.lastRun = now
	if err = w.persistCheckpoint(ctx, now); err != nil {
		log.Printf("persist outbound checkpoint failed: %v", err)
	}

	if len(failedTables) > 0 {
		return fmt.Errorf("outbound completed with queued errors for tables: %s", strings.Join(failedTables, ", "))
	}
	return nil
}

func (w *OutboundWorker) retryQueuedOutbound(ctx context.Context) error {
	jobs, err := w.queue.PeekByDirection(ctx, outboundGenericDirection, 100)
	if err != nil {
		return err
	}

	failedJobs := make([]string, 0)
	for _, job := range jobs {
		var payload queuedOutboundPayload
		if err = json.Unmarshal([]byte(job.PayloadJSON), &payload); err != nil {
			_ = w.queue.Delete(ctx, job.ID)
			continue
		}

		if err = w.pgClient.UpsertRows(ctx, "public", payload.TableName, payload.Rows, payload.ConflictColumns); err != nil {
			failedJobs = append(failedJobs, fmt.Sprintf("%d:%s", job.ID, payload.TableName))
			log.Printf("retry queued outbound job failed id=%d table=%s: %v", job.ID, payload.TableName, err)
			w.runtime.AddLog(fmt.Sprintf("retry queued outbound failed id=%d table=%s: %v", job.ID, payload.TableName, err))
			continue
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

func (w *OutboundWorker) loadCheckpoint(ctx context.Context) error {
	rawValue, exists, err := w.queue.GetStateValue(ctx, outboundStateKey)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	parsed, err := time.Parse(time.RFC3339Nano, rawValue)
	if err != nil {
		return fmt.Errorf("parse checkpoint: %w", err)
	}

	w.lastRun = parsed
	return nil
}

func (w *OutboundWorker) persistCheckpoint(ctx context.Context, value time.Time) error {
	return w.queue.SetStateValue(ctx, outboundStateKey, value.Format(time.RFC3339Nano))
}
