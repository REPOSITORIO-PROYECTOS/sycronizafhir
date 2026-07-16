package sync

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"sycronizafhir/internal/config"
	"sycronizafhir/internal/db"
	"sycronizafhir/internal/monitor"
	"sycronizafhir/internal/supabase"
)

const inboundPedidosTiendaStateKey = "inbound_pedidos_tienda_last_id"

const defaultClienteTiendaID int16 = 1337

// PedidosTiendaInboundWorker convierte pedido_pagina (estado N) de Supabase → pedidos + pedidos_d en Mica.
type PedidosTiendaInboundWorker struct {
	localPG      *db.LocalPG
	remotePG     *supabase.PGClient
	queue        *db.QueueSQLite
	sourceSchema string
	pollInterval time.Duration
	lastPedidoID int64
	runtime      *monitor.Runtime
}

func NewPedidosTiendaInboundWorker(
	localPG *db.LocalPG,
	remotePG *supabase.PGClient,
	queue *db.QueueSQLite,
	cfg config.Config,
	runtime *monitor.Runtime,
) *PedidosTiendaInboundWorker {
	return &PedidosTiendaInboundWorker{
		localPG:      localPG,
		remotePG:     remotePG,
		queue:        queue,
		sourceSchema: cfg.SourceSchema,
		pollInterval: cfg.InboundPedidosTiendaInterval,
		lastPedidoID: 0,
		runtime:      runtime,
	}
}

func (w *PedidosTiendaInboundWorker) Run(ctx context.Context) {
	if err := w.loadCheckpoint(ctx); err != nil {
		log.Printf("load inbound pedidos_tienda checkpoint failed: %v", err)
	}
	w.runtime.SetComponentStatus("inbound_pedidos_tienda", "running", "worker iniciado")

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	if err := w.runCycle(ctx); err != nil {
		log.Printf("inbound pedidos_tienda initial cycle failed: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.runCycle(ctx); err != nil {
				log.Printf("inbound pedidos_tienda cycle failed: %v", err)
				w.runtime.SetComponentStatus("inbound_pedidos_tienda", "error", err.Error())
			} else {
				w.runtime.SetComponentStatus("inbound_pedidos_tienda", "running", "ciclo OK")
			}
		}
	}
}

func (w *PedidosTiendaInboundWorker) runCycle(ctx context.Context) error {
	pedidosExists, err := w.localPG.TableExists(ctx, w.sourceSchema, "pedidos")
	if err != nil {
		return err
	}
	if !pedidosExists {
		w.runtime.AddLog("inbound pedidos_tienda: tabla local pedidos no existe en Mica")
		return nil
	}

	detailTable, err := w.localPG.ResolvePedidosDetalleTable(ctx, w.sourceSchema)
	if err != nil {
		return err
	}
	if detailTable == "" {
		w.runtime.AddLog("inbound pedidos_tienda: tabla local pedidos_d no existe en Mica")
		return nil
	}

	remoteExists, err := w.remotePG.TableExists(ctx, "public", "pedido_pagina")
	if err != nil {
		return err
	}
	if !remoteExists {
		w.runtime.AddLog("inbound pedidos_tienda: tabla remota pedido_pagina no existe en Supabase")
		return nil
	}

	remoteDetailTable, err := w.remotePG.ResolvePedidoPaginaDetailTable(ctx, "public")
	if err != nil {
		return err
	}
	if remoteDetailTable == "" {
		w.runtime.AddLog("inbound pedidos_tienda: detalle remoto pedido_pagina no encontrado")
		return nil
	}

	heads, err := w.remotePG.LoadPedidoPaginaHeadsEstadoNAfterID(ctx, "public", w.lastPedidoID, 100)
	if err != nil {
		return err
	}
	if len(heads) == 0 {
		w.runtime.AddLog("inbound pedidos_tienda: sin pedidos tienda nuevos (estado N)")
		return nil
	}

	synced := 0
	skipped := 0
	maxID := w.lastPedidoID
	for _, head := range heads {
		pedidoPaginaID, parseErr := pedidoPaginaID(head["pedido_id"])
		if parseErr != nil {
			continue
		}
		if pedidoPaginaID <= w.lastPedidoID {
			continue
		}

		pedID := pedIDFromPaginaTienda(pedidoPaginaID)
		exists, existsErr := w.localPG.PedidoCabeceraExists(ctx, w.sourceSchema, pedID)
		if existsErr != nil {
			return existsErr
		}
		if exists {
			skipped++
			if pedidoPaginaID > maxID {
				maxID = pedidoPaginaID
			}
			continue
		}

		lines, loadErr := w.remotePG.LoadPedidoPaginaDetails(ctx, "public", remoteDetailTable, pedidoPaginaID)
		if loadErr != nil {
			return loadErr
		}
		if len(lines) == 0 {
			w.runtime.AddLog(fmt.Sprintf("inbound pedidos_tienda: pedido_pagina id=%d sin líneas, omitido", pedidoPaginaID))
			if pedidoPaginaID > maxID {
				maxID = pedidoPaginaID
			}
			continue
		}

		clienID, lookupErr := w.resolveClienteID(ctx, head)
		if lookupErr != nil {
			return lookupErr
		}

		cabecera := mapPedidoPaginaToCabecera(pedidoPaginaID, head, lines, clienID)
		detalle := mapPedidoPaginaLinesToPedidosD(pedID, lines)

		if err = w.localPG.UpsertPedidoCabeceraTienda(ctx, w.sourceSchema, cabecera); err != nil {
			return err
		}
		if err = w.localPG.ReplacePedidosDetalleTienda(ctx, w.sourceSchema, detailTable, pedID, detalle); err != nil {
			return err
		}

		synced++
		w.runtime.AddLog(fmt.Sprintf(
			"inbound pedidos_tienda: pedido %s creado desde pedido_pagina id=%d (estado N, %d ítems)",
			pedID,
			pedidoPaginaID,
			len(detalle),
		))
		if pedidoPaginaID > maxID {
			maxID = pedidoPaginaID
		}
	}

	w.lastPedidoID = maxID
	if err = w.persistCheckpoint(ctx, maxID); err != nil {
		log.Printf("persist inbound pedidos_tienda checkpoint failed: %v", err)
	}

	if synced > 0 || skipped > 0 {
		w.runtime.AddLog(fmt.Sprintf(
			"inbound pedidos_tienda: %d creado(s), %d ya existían (hasta pedido_pagina id=%d)",
			synced,
			skipped,
			maxID,
		))
	}
	return nil
}

func (w *PedidosTiendaInboundWorker) resolveClienteID(ctx context.Context, head map[string]interface{}) (int16, error) {
	cuitRaw := stringField(head, "cuit")
	cuit := normalizeCuitDigits(cuitRaw)
	if cuit == "" {
		return defaultClienteTiendaID, nil
	}

	clienID, found, err := w.localPG.LookupClienteIDByCuit(ctx, w.sourceSchema, cuit)
	if err != nil {
		return 0, err
	}
	if found {
		return clienID, nil
	}
	w.runtime.AddLog(fmt.Sprintf("inbound pedidos_tienda: CUIT %s no encontrado, clien_id=%d", cuitRaw, defaultClienteTiendaID))
	return defaultClienteTiendaID, nil
}

func (w *PedidosTiendaInboundWorker) loadCheckpoint(ctx context.Context) error {
	rawValue, exists, err := w.queue.GetStateValue(ctx, inboundPedidosTiendaStateKey)
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

func (w *PedidosTiendaInboundWorker) persistCheckpoint(ctx context.Context, value int64) error {
	return w.queue.SetStateValue(ctx, inboundPedidosTiendaStateKey, strconv.FormatInt(value, 10))
}
