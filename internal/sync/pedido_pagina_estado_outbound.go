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

// PedidoPaginaEstadoOutboundWorker sube solo estado ERP → Supabase.
// No INSERT de cabezas; no toca detalle. Gate: enabled_tables incluye pedido_pagina.
type PedidoPaginaEstadoOutboundWorker struct {
	localPG      *db.LocalPG
	remotePG     *supabase.PGClient
	sourceSchema string
	pollInterval time.Duration
	runtime      *monitor.Runtime
}

func NewPedidoPaginaEstadoOutboundWorker(
	localPG *db.LocalPG,
	remotePG *supabase.PGClient,
	cfg config.Config,
	runtime *monitor.Runtime,
) *PedidoPaginaEstadoOutboundWorker {
	interval := cfg.InboundPedidoPaginaInterval
	if interval <= 0 {
		interval = 120 * time.Second
	}
	return &PedidoPaginaEstadoOutboundWorker{
		localPG:      localPG,
		remotePG:     remotePG,
		sourceSchema: cfg.SourceSchema,
		pollInterval: interval,
		runtime:      runtime,
	}
}

func (w *PedidoPaginaEstadoOutboundWorker) Run(ctx context.Context) {
	w.runtime.SetComponentStatus("pedido_pagina_estado_outbound", "running", "worker iniciado")

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	if err := w.runCycle(ctx); err != nil {
		log.Printf("outbound pedido_pagina estado initial cycle failed: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.runCycle(ctx); err != nil {
				log.Printf("outbound pedido_pagina estado cycle failed: %v", err)
				w.runtime.SetComponentStatus("pedido_pagina_estado_outbound", "error", err.Error())
			} else {
				w.runtime.SetComponentStatus("pedido_pagina_estado_outbound", "running", "ciclo OK")
			}
		}
	}
}

func (w *PedidoPaginaEstadoOutboundWorker) runCycle(ctx context.Context) error {
	syncCfg, err := config.LoadSyncTablesConfig()
	if err != nil {
		return err
	}
	if !syncCfg.IsEnabled(pedidoPaginaHeadTable) {
		w.runtime.AddLog("outbound pedido_pagina estado: tabla no habilitada en SYSTEM")
		return nil
	}

	localExists, err := w.localPG.TableExists(ctx, w.sourceSchema, pedidoPaginaHeadTable)
	if err != nil {
		return err
	}
	if !localExists {
		w.runtime.AddLog("outbound pedido_pagina estado: tabla local no existe")
		return nil
	}
	remoteExists, err := w.remotePG.TableExists(ctx, "public", pedidoPaginaHeadTable)
	if err != nil {
		return err
	}
	if !remoteExists {
		w.runtime.AddLog("outbound pedido_pagina estado: tabla remota no existe")
		return nil
	}

	pkColumns := []string{"pedido_id"}
	localPKs, err := w.localPG.LoadPrimaryKeyRows(ctx, w.sourceSchema, pedidoPaginaHeadTable, pkColumns)
	if err != nil {
		return err
	}
	if len(localPKs) == 0 {
		w.runtime.AddLog("outbound pedido_pagina estado: sin filas locales")
		return nil
	}

	applied := 0
	for start := 0; start < len(localPKs); start += 200 {
		end := start + 200
		if end > len(localPKs) {
			end = len(localPKs)
		}
		batch := localPKs[start:end]
		localRows, loadErr := w.localPG.LoadRowsByPrimaryKeys(ctx, w.sourceSchema, pedidoPaginaHeadTable, pkColumns, batch)
		if loadErr != nil {
			return loadErr
		}
		remoteRows, loadErr := w.remotePG.LoadRowsByPrimaryKeys(ctx, "public", pedidoPaginaHeadTable, pkColumns, batch)
		if loadErr != nil {
			return loadErr
		}
		n, patchErr := patchPedidoPaginaEstados(ctx, w.remotePG, localRows, remoteRows)
		if patchErr != nil {
			return patchErr
		}
		applied += n
	}

	if applied > 0 {
		w.runtime.AddLog(fmt.Sprintf("outbound pedido_pagina estado: %d PATCH (solo estado)", applied))
	} else {
		w.runtime.AddLog("outbound pedido_pagina estado: sin diferencias de estado")
	}
	return nil
}
