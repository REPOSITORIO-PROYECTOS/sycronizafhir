package sync

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"sycronizafhir/internal/config"
	"sycronizafhir/internal/db"
	"sycronizafhir/internal/monitor"
	"sycronizafhir/internal/supabase"
)

const inboundPendientesStateKey = "inbound_pendientes_last_run_utc"

// pendienteInboundDecision: patch de cantidad o baja en Mica.
type pendienteInboundDecision struct {
	Delete bool
	Patch  map[string]interface{}
}

func mergePendienteInbound(
	localRow, remoteRow map[string]interface{},
	meta db.TableModifiedAtMeta,
) pendienteInboundDecision {
	remoteAt, remoteOK := db.RowModifiedAt(remoteRow, meta)
	localAt, localOK := db.RowModifiedAt(localRow, meta)
	remoteNewer := remoteOK && (!localOK || remoteAt.After(localAt))
	if !remoteNewer {
		return pendienteInboundDecision{}
	}

	remoteCant, remoteCantOK := pendienteCantidad(remoteRow["cantidad"])
	if remoteCantOK && remoteCant <= 0 {
		return pendienteInboundDecision{Delete: true}
	}

	localCant, localCantOK := pendienteCantidad(localRow["cantidad"])
	if remoteCantOK && localCantOK && remoteCant == localCant {
		return pendienteInboundDecision{}
	}
	if !remoteCantOK {
		return pendienteInboundDecision{}
	}
	return pendienteInboundDecision{
		Patch: map[string]interface{}{"cantidad": remoteCant},
	}
}

func pendienteCantidad(raw interface{}) (float64, bool) {
	if raw == nil {
		return 0, false
	}
	switch typed := raw.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			return 0, false
		}
		parsed, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	case []byte:
		return pendienteCantidad(string(typed))
	default:
		text := strings.TrimSpace(fmt.Sprintf("%v", typed))
		if text == "" || text == "<nil>" {
			return 0, false
		}
		parsed, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	}
}

// PendientesInboundWorker baja cantidad/baja de `pendientes` desde Supabase → Mica.
// La tienda escribe la réplica; sin este worker Gestiona no se entera.
type PendientesInboundWorker struct {
	localPG      *db.LocalPG
	remotePG     *supabase.PGClient
	queue        *db.QueueSQLite
	sourceSchema string
	pollInterval time.Duration
	lastRun      time.Time
	runtime      *monitor.Runtime
}

func NewPendientesInboundWorker(
	localPG *db.LocalPG,
	remotePG *supabase.PGClient,
	queue *db.QueueSQLite,
	cfg config.Config,
	runtime *monitor.Runtime,
) *PendientesInboundWorker {
	interval := cfg.InboundPendientesInterval
	if interval <= 0 {
		interval = cfg.InboundClientesInterval
	}
	if interval <= 0 {
		interval = 120 * time.Second
	}
	return &PendientesInboundWorker{
		localPG:      localPG,
		remotePG:     remotePG,
		queue:        queue,
		sourceSchema: cfg.SourceSchema,
		pollInterval: interval,
		lastRun:      time.Now().Add(-24 * time.Hour),
		runtime:      runtime,
	}
}

func (w *PendientesInboundWorker) Run(ctx context.Context) {
	if err := w.loadCheckpoint(ctx); err != nil {
		log.Printf("load inbound pendientes checkpoint failed, using startup window: %v", err)
	}
	w.runtime.SetComponentStatus("inbound_pendientes", "running", "worker iniciado")

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	if err := w.runCycle(ctx); err != nil {
		log.Printf("inbound pendientes initial cycle failed: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.runCycle(ctx); err != nil {
				log.Printf("inbound pendientes cycle failed: %v", err)
				w.runtime.SetComponentStatus("inbound_pendientes", "error", err.Error())
			} else {
				w.runtime.SetComponentStatus("inbound_pendientes", "running", "ciclo OK")
			}
		}
	}
}

func (w *PendientesInboundWorker) runCycle(ctx context.Context) error {
	table, err := w.resolvePendientesTable(ctx)
	if err != nil {
		return err
	}
	if table.Name == "" {
		w.runtime.AddLog("inbound pendientes: tabla pendientes no disponible en Mica")
		return nil
	}

	remoteRows, err := w.remotePG.LoadUpdatedRows(ctx, "public", table.Name, w.lastRun)
	if err != nil {
		return err
	}
	if len(remoteRows) == 0 {
		w.runtime.AddLog("inbound pendientes: sin cambios recientes en Supabase")
	} else {
		applied, applyErr := w.applyRemotePendienteRows(ctx, table, remoteRows)
		if applyErr != nil {
			return applyErr
		}
		if applied > 0 {
			w.runtime.AddLog(fmt.Sprintf("inbound pendientes: %d fila(s) actualizadas en Mica", applied))
		} else {
			w.runtime.AddLog(fmt.Sprintf("inbound pendientes: %d cambio(s) en nube sin patch local", len(remoteRows)))
		}
	}

	now := time.Now().UTC()
	w.lastRun = now
	if err = w.persistCheckpoint(ctx, now); err != nil {
		log.Printf("persist inbound pendientes checkpoint failed: %v", err)
	}
	return nil
}

func (w *PendientesInboundWorker) resolvePendientesTable(ctx context.Context) (db.SyncTable, error) {
	tables, err := w.localPG.ListSyncTables(ctx, w.sourceSchema, nil)
	if err != nil {
		return db.SyncTable{}, err
	}
	for _, table := range tables {
		if table.Name == "pendientes" {
			return table, nil
		}
	}
	return db.SyncTable{}, nil
}

func (w *PendientesInboundWorker) applyRemotePendienteRows(
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

		decision := mergePendienteInbound(localRows[0], remoteRow, meta)
		if decision.Delete {
			deleted, delErr := w.localPG.DeleteRowByPrimaryKeys(ctx, w.sourceSchema, table.Name, table.PrimaryKeys, pkOnly)
			if delErr != nil {
				return applied, delErr
			}
			if deleted {
				applied++
				w.runtime.AddLog(fmt.Sprintf("inbound pendientes: %v baja en Mica", pkOnly))
			}
			continue
		}
		if len(decision.Patch) == 0 {
			continue
		}

		updated, err := w.localPG.PatchRowColumns(ctx, w.sourceSchema, table.Name, table.PrimaryKeys, pkOnly, decision.Patch)
		if err != nil {
			return applied, err
		}
		if updated {
			applied++
		}
	}
	return applied, nil
}

func (w *PendientesInboundWorker) loadCheckpoint(ctx context.Context) error {
	rawValue, exists, err := w.queue.GetStateValue(ctx, inboundPendientesStateKey)
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

func (w *PendientesInboundWorker) persistCheckpoint(ctx context.Context, value time.Time) error {
	return w.queue.SetStateValue(ctx, inboundPendientesStateKey, value.Format(time.RFC3339Nano))
}
