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

const inboundPedidoPaginaStateKey = "inbound_pedido_pagina_last_id"

// PedidoPaginaInboundWorker replica pedido_pagina (+ detalle) Supabase → Mica local.
type PedidoPaginaInboundWorker struct {
	localPG       *db.LocalPG
	remotePG      *supabase.PGClient
	queue         *db.QueueSQLite
	sourceSchema  string
	pollInterval  time.Duration
	lastPedidoID  int64
	runtime       *monitor.Runtime
}

func NewPedidoPaginaInboundWorker(
	localPG *db.LocalPG,
	remotePG *supabase.PGClient,
	queue *db.QueueSQLite,
	cfg config.Config,
	runtime *monitor.Runtime,
) *PedidoPaginaInboundWorker {
	return &PedidoPaginaInboundWorker{
		localPG:      localPG,
		remotePG:     remotePG,
		queue:        queue,
		sourceSchema: cfg.SourceSchema,
		pollInterval: cfg.InboundPedidoPaginaInterval,
		lastPedidoID: 0,
		runtime:      runtime,
	}
}

func (w *PedidoPaginaInboundWorker) Run(ctx context.Context) {
	if err := w.loadCheckpoint(ctx); err != nil {
		log.Printf("load inbound pedido_pagina checkpoint failed: %v", err)
	}
	w.runtime.SetComponentStatus("inbound_pedido_pagina", "running", "worker iniciado")

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	if err := w.runCycle(ctx); err != nil {
		log.Printf("inbound pedido_pagina initial cycle failed: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.runCycle(ctx); err != nil {
				log.Printf("inbound pedido_pagina cycle failed: %v", err)
				w.runtime.SetComponentStatus("inbound_pedido_pagina", "error", err.Error())
			} else {
				w.runtime.SetComponentStatus("inbound_pedido_pagina", "running", "ciclo OK")
			}
		}
	}
}

func (w *PedidoPaginaInboundWorker) runCycle(ctx context.Context) error {
	localExists, err := w.localPG.TableExists(ctx, w.sourceSchema, "pedido_pagina")
	if err != nil {
		return err
	}
	if !localExists {
		w.runtime.AddLog("inbound pedido_pagina: tabla local pedido_pagina no existe en Mica")
		return nil
	}

	remoteExists, err := w.remotePG.TableExists(ctx, "public", "pedido_pagina")
	if err != nil {
		return err
	}
	if !remoteExists {
		w.runtime.AddLog("inbound pedido_pagina: tabla remota pedido_pagina no existe en Supabase")
		return nil
	}

	heads, err := w.remotePG.LoadPedidoPaginaHeadsAfterID(ctx, "public", w.lastPedidoID, 100)
	if err != nil {
		return err
	}
	if len(heads) == 0 {
		w.runtime.AddLog("inbound pedido_pagina: sin pedidos nuevos en nube")
		return nil
	}

	localDetailTable, err := w.localPG.ResolvePedidoPaginaDetailTable(ctx, w.sourceSchema)
	if err != nil {
		return err
	}
	remoteDetailTable, err := w.remotePG.ResolvePedidoPaginaDetailTable(ctx, "public")
	if err != nil {
		return err
	}

	synced := 0
	maxID := w.lastPedidoID
	for _, head := range heads {
		pedidoID, parseErr := pedidoPaginaID(head["pedido_id"])
		if parseErr != nil {
			continue
		}
		if pedidoID <= w.lastPedidoID {
			continue
		}

		if err = w.localPG.UpsertPedidoPaginaHead(ctx, w.sourceSchema, head); err != nil {
			return err
		}

		if localDetailTable != "" && remoteDetailTable != "" {
			lines, loadErr := w.remotePG.LoadPedidoPaginaDetails(ctx, "public", remoteDetailTable, pedidoID)
			if loadErr != nil {
				return loadErr
			}
			if err = w.localPG.ReplacePedidoPaginaDetails(ctx, w.sourceSchema, localDetailTable, pedidoID, lines); err != nil {
				return err
			}
		}

		synced++
		if pedidoID > maxID {
			maxID = pedidoID
		}
	}

	w.lastPedidoID = maxID
	if err = w.persistCheckpoint(ctx, maxID); err != nil {
		log.Printf("persist inbound pedido_pagina checkpoint failed: %v", err)
	}

	if synced > 0 {
		w.runtime.AddLog(fmt.Sprintf("inbound pedido_pagina: %d pedido(s) replicados a Mica (hasta id=%d)", synced, maxID))
	}
	return nil
}

func pedidoPaginaID(raw interface{}) (int64, error) {
	switch typed := raw.(type) {
	case int64:
		return typed, nil
	case int32:
		return int64(typed), nil
	case int:
		return int64(typed), nil
	case float64:
		return int64(typed), nil
	case string:
		return strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
	default:
		return strconv.ParseInt(strings.TrimSpace(fmt.Sprint(raw)), 10, 64)
	}
}

func (w *PedidoPaginaInboundWorker) loadCheckpoint(ctx context.Context) error {
	rawValue, exists, err := w.queue.GetStateValue(ctx, inboundPedidoPaginaStateKey)
	if err != nil {
		return err
	}
	if !exists || rawValue == "" {
		return nil
	}
	parsed, parseErr := strconv.ParseInt(rawValue, 10, 64)
	if parseErr != nil {
		return parseErr
	}
	w.lastPedidoID = parsed
	return nil
}

func (w *PedidoPaginaInboundWorker) persistCheckpoint(ctx context.Context, value int64) error {
	return w.queue.SetStateValue(ctx, inboundPedidoPaginaStateKey, strconv.FormatInt(value, 10))
}
