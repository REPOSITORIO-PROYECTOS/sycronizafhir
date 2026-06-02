package sync

import (
	"context"
	"log"
	"time"

	"sycronizafhir/internal/config"
	"sycronizafhir/internal/db"
	"sycronizafhir/internal/monitor"
	"sycronizafhir/internal/supabase"
)

const auditLastRunStateKey = "data_audit_last_run_utc"

type AuditWorker struct {
	localPG      *db.LocalPG
	remotePG     *supabase.PGClient
	queue        *db.QueueSQLite
	sourceSchema string
	exclude      []string
	interval     time.Duration
	runtime      *monitor.Runtime
}

func NewAuditWorker(
	localPG *db.LocalPG,
	remotePG *supabase.PGClient,
	queue *db.QueueSQLite,
	sourceSchema string,
	exclude []string,
	interval time.Duration,
	runtime *monitor.Runtime,
) *AuditWorker {
	if interval <= 0 {
		interval = 6 * time.Hour
	}
	return &AuditWorker{
		localPG:      localPG,
		remotePG:     remotePG,
		queue:        queue,
		sourceSchema: sourceSchema,
		exclude:      exclude,
		interval:     interval,
		runtime:      runtime,
	}
}

func (w *AuditWorker) Run(ctx context.Context) {
	w.runtime.SetComponentStatus("data_audit", "running", "auditoria programada activa")
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	if err := w.runScheduledCycle(ctx); err != nil {
		log.Printf("audit initial cycle failed: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.runScheduledCycle(ctx); err != nil {
				log.Printf("audit cycle failed: %v", err)
				w.runtime.SetComponentStatus("data_audit", "error", err.Error())
			} else {
				w.runtime.SetComponentStatus("data_audit", "running", "ultima auditoria OK")
			}
		}
	}
}

func (w *AuditWorker) runScheduledCycle(ctx context.Context) error {
	syncCfg, err := config.LoadSyncTablesConfig()
	if err != nil {
		return err
	}

	service := NewReconcileService(w.localPG, w.remotePG, w.sourceSchema, w.exclude, w.runtime)
	report, err := service.RunAudit(ctx, syncCfg, "scheduled", true)
	if err != nil {
		return err
	}

	if saveErr := SaveAuditReport(ctx, w.queue, report); saveErr != nil {
		log.Printf("persist audit report failed: %v", saveErr)
	}

	now := time.Now().UTC()
	if saveErr := w.queue.SetStateValue(ctx, auditLastRunStateKey, now.Format(time.RFC3339Nano)); saveErr != nil {
		log.Printf("persist audit last run failed: %v", saveErr)
	}

	w.runtime.SetMeta("audit_last_run", now.Format(time.RFC3339))
	w.runtime.SetMeta("audit_next_run", now.Add(w.interval).Format(time.RFC3339))
	return nil
}
