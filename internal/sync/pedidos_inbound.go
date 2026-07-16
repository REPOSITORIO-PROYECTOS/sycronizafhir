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

const inboundPedidosEstadoStateKey = "inbound_pedidos_estado_last_run_utc"

// PedidosInboundWorker baja estados K/V/E (y bultos) de Supabase → Mica cuando Picking opera.
type PedidosInboundWorker struct {
	localPG      *db.LocalPG
	remotePG     *supabase.PGClient
	queue        *db.QueueSQLite
	sourceSchema string
	pollInterval time.Duration
	lastRun      time.Time
	runtime      *monitor.Runtime
}

func NewPedidosInboundWorker(
	localPG *db.LocalPG,
	remotePG *supabase.PGClient,
	queue *db.QueueSQLite,
	cfg config.Config,
	runtime *monitor.Runtime,
) *PedidosInboundWorker {
	return &PedidosInboundWorker{
		localPG:      localPG,
		remotePG:     remotePG,
		queue:        queue,
		sourceSchema: cfg.SourceSchema,
		pollInterval: cfg.InboundPedidosInterval,
		lastRun:      time.Now().Add(-24 * time.Hour),
		runtime:      runtime,
	}
}

func (w *PedidosInboundWorker) Run(ctx context.Context) {
	if err := w.loadCheckpoint(ctx); err != nil {
		log.Printf("load inbound pedidos checkpoint failed, using startup window: %v", err)
	}
	w.runtime.SetComponentStatus("inbound_pedidos", "running", "worker iniciado")

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	if err := w.runCycle(ctx); err != nil {
		log.Printf("inbound pedidos initial cycle failed: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.runCycle(ctx); err != nil {
				log.Printf("inbound pedidos cycle failed: %v", err)
				w.runtime.SetComponentStatus("inbound_pedidos", "error", err.Error())
			} else {
				w.runtime.SetComponentStatus("inbound_pedidos", "running", "ciclo OK")
			}
		}
	}
}

func (w *PedidosInboundWorker) runCycle(ctx context.Context) error {
	table, err := w.resolvePedidosTable(ctx)
	if err != nil {
		return err
	}
	if table.Name == "" {
		w.runtime.AddLog("inbound pedidos: tabla pedidos no disponible en Mica")
		return nil
	}

	meta, err := w.localPG.LoadTableModifiedAtMeta(ctx, w.sourceSchema, table.Name)
	if err != nil {
		return err
	}

	remoteRows, err := w.remotePG.LoadUpdatedRows(ctx, "public", table.Name, w.lastRun)
	if err != nil {
		return err
	}
	if len(remoteRows) == 0 {
		w.runtime.AddLog("inbound pedidos: sin cambios recientes en Supabase")
	} else {
		applied, applyErr := w.applyRemotePedidoRows(ctx, table, meta, remoteRows)
		if applyErr != nil {
			return applyErr
		}
		if applied > 0 {
			w.runtime.AddLog(fmt.Sprintf("inbound pedidos: %d cabecera(s) actualizadas en Mica (K/V/E)", applied))
		} else {
			w.runtime.AddLog(fmt.Sprintf("inbound pedidos: %d cambio(s) en nube sin patch local", len(remoteRows)))
		}
	}

	now := time.Now().UTC()
	w.lastRun = now
	if err = w.persistCheckpoint(ctx, now); err != nil {
		log.Printf("persist inbound pedidos checkpoint failed: %v", err)
	}
	return nil
}

func (w *PedidosInboundWorker) resolvePedidosTable(ctx context.Context) (db.SyncTable, error) {
	tables, err := w.localPG.ListSyncTables(ctx, w.sourceSchema, nil)
	if err != nil {
		return db.SyncTable{}, err
	}
	for _, table := range tables {
		if table.Name == "pedidos" {
			return table, nil
		}
	}
	return db.SyncTable{}, nil
}

func (w *PedidosInboundWorker) applyRemotePedidoRows(
	ctx context.Context,
	table db.SyncTable,
	meta db.TableModifiedAtMeta,
	remoteRows []map[string]interface{},
) (int, error) {
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

		patch := mergePedidoEstadoPatch(localRows[0], remoteRow, meta)
		if len(patch) == 0 {
			continue
		}

		updated, err := w.localPG.PatchRowColumns(ctx, w.sourceSchema, table.Name, table.PrimaryKeys, pkOnly, patch)
		if err != nil {
			return applied, err
		}
		if updated {
			applied++
			if estado, ok := patch["estado"]; ok {
				w.runtime.AddLog(fmt.Sprintf(
					"inbound pedidos: %v estado %s→%s",
					pkOnly,
					normalizePedidoEstado(localRows[0]["estado"]),
					normalizePedidoEstado(estado),
				))
			}
		}
	}
	return applied, nil
}

func (w *PedidosInboundWorker) loadCheckpoint(ctx context.Context) error {
	rawValue, exists, err := w.queue.GetStateValue(ctx, inboundPedidosEstadoStateKey)
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

func (w *PedidosInboundWorker) persistCheckpoint(ctx context.Context, value time.Time) error {
	return w.queue.SetStateValue(ctx, inboundPedidosEstadoStateKey, value.Format(time.RFC3339Nano))
}
