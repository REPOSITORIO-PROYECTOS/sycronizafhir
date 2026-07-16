package sync

import (
	"context"
	"fmt"
	"log"
	"time"

	"sycronizafhir/internal/config"
	"sycronizafhir/internal/db"
	"sycronizafhir/internal/monitor"
	"sycronizafhir/internal/supabase"
)

const inboundClientesStateKey = "inbound_clientes_last_run_utc"

// ClientesInboundWorker baja a Mica datos de contacto completados en Supabase (tienda web).
type ClientesInboundWorker struct {
	localPG      *db.LocalPG
	remotePG     *supabase.PGClient
	queue        *db.QueueSQLite
	sourceSchema string
	pollInterval time.Duration
	lastRun      time.Time
	runtime      *monitor.Runtime
}

func NewClientesInboundWorker(
	localPG *db.LocalPG,
	remotePG *supabase.PGClient,
	queue *db.QueueSQLite,
	cfg config.Config,
	runtime *monitor.Runtime,
) *ClientesInboundWorker {
	return &ClientesInboundWorker{
		localPG:      localPG,
		remotePG:     remotePG,
		queue:        queue,
		sourceSchema: cfg.SourceSchema,
		pollInterval: cfg.InboundClientesInterval,
		lastRun:      time.Now().Add(-24 * time.Hour),
		runtime:      runtime,
	}
}

func (w *ClientesInboundWorker) Run(ctx context.Context) {
	if err := w.loadCheckpoint(ctx); err != nil {
		log.Printf("load inbound clientes checkpoint failed, using startup window: %v", err)
	}
	w.runtime.SetComponentStatus("inbound_clientes", "running", "worker iniciado")

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	if err := w.runCycle(ctx); err != nil {
		log.Printf("inbound clientes initial cycle failed: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.runCycle(ctx); err != nil {
				log.Printf("inbound clientes cycle failed: %v", err)
				w.runtime.SetComponentStatus("inbound_clientes", "error", err.Error())
			} else {
				w.runtime.SetComponentStatus("inbound_clientes", "running", "ciclo OK")
			}
		}
	}
}

func (w *ClientesInboundWorker) runCycle(ctx context.Context) error {
	table, err := w.resolveClientesTable(ctx)
	if err != nil {
		return err
	}
	if table.Name == "" {
		w.runtime.AddLog("inbound clientes: tabla clientes no disponible en Mica")
		return nil
	}

	remoteRows, err := w.remotePG.LoadUpdatedRows(ctx, "public", table.Name, w.lastRun)
	if err != nil {
		return err
	}
	if len(remoteRows) == 0 {
		w.runtime.AddLog("inbound clientes: sin cambios recientes en Supabase")
	} else {
		applied, applyErr := w.applyRemoteContactRows(ctx, table, remoteRows)
		if applyErr != nil {
			return applyErr
		}
		if applied > 0 {
			w.runtime.AddLog(fmt.Sprintf("inbound clientes: %d fila(s) actualizadas en Mica", applied))
		} else {
			w.runtime.AddLog(fmt.Sprintf("inbound clientes: %d cambio(s) en nube sin patch local", len(remoteRows)))
		}
	}

	now := time.Now().UTC()
	w.lastRun = now
	if err = w.persistCheckpoint(ctx, now); err != nil {
		log.Printf("persist inbound clientes checkpoint failed: %v", err)
	}
	return nil
}

func (w *ClientesInboundWorker) resolveClientesTable(ctx context.Context) (db.SyncTable, error) {
	tables, err := w.localPG.ListSyncTables(ctx, w.sourceSchema, nil)
	if err != nil {
		return db.SyncTable{}, err
	}
	for _, table := range tables {
		if table.Name == "clientes" {
			return table, nil
		}
	}
	return db.SyncTable{}, nil
}

func (w *ClientesInboundWorker) applyRemoteContactRows(
	ctx context.Context,
	table db.SyncTable,
	remoteRows []map[string]interface{},
) (int, error) {
	meta, err := w.localPG.LoadTableModifiedAtMeta(ctx, w.sourceSchema, table.Name)
	if err != nil {
		return 0, err
	}

	applied := 0
	for _, remoteRow := range remoteRows {
		pkOnly := make(map[string]interface{}, len(table.PrimaryKeys))
		for _, column := range table.PrimaryKeys {
			value, ok := remoteRow[column]
			if !ok || value == nil {
				continue
			}
			pkOnly[column] = value
		}
		if len(pkOnly) != len(table.PrimaryKeys) {
			continue
		}

		localRows, err := w.localPG.LoadRowsByPrimaryKeys(ctx, w.sourceSchema, table.Name, table.PrimaryKeys, []map[string]interface{}{pkOnly})
		if err != nil {
			return applied, err
		}
		if len(localRows) == 0 {
			continue
		}

		patch := mergeClienteInboundPatch(localRows[0], remoteRow, meta)
		if len(patch) == 0 {
			continue
		}

		updated, err := w.localPG.PatchRowColumns(ctx, w.sourceSchema, table.Name, table.PrimaryKeys, pkOnly, patch)
		if err != nil {
			return applied, err
		}
		if updated {
			applied++
		}
	}
	return applied, nil
}

func (w *ClientesInboundWorker) loadCheckpoint(ctx context.Context) error {
	rawValue, exists, err := w.queue.GetStateValue(ctx, inboundClientesStateKey)
	if err != nil {
		return err
	}
	if !exists || rawValue == "" {
		return nil
	}
	parsed, parseErr := time.Parse(time.RFC3339Nano, rawValue)
	if parseErr != nil {
		return parseErr
	}
	w.lastRun = parsed
	return nil
}

func (w *ClientesInboundWorker) persistCheckpoint(ctx context.Context, value time.Time) error {
	return w.queue.SetStateValue(ctx, inboundClientesStateKey, value.Format(time.RFC3339Nano))
}
