package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"sycronizafhir/internal/config"
	"sycronizafhir/internal/db"
	"sycronizafhir/internal/models"
	"sycronizafhir/internal/monitor"
	"sycronizafhir/internal/supabase"
)

type InboundWorker struct {
	localPG        *db.LocalPG
	queue          *db.QueueSQLite
	realtimeClient *supabase.RealtimeClient
	runtime        *monitor.Runtime
}

func NewInboundWorker(localPG *db.LocalPG, queue *db.QueueSQLite, cfg config.Config, runtime *monitor.Runtime) *InboundWorker {
	return &InboundWorker{
		localPG: localPG,
		queue:   queue,
		realtimeClient: supabase.NewRealtimeClient(
			cfg.SupabaseRealtimeURL,
			cfg.SupabaseServiceRole,
			cfg.RealtimeChannel,
			cfg.RealtimeSchema,
			cfg.RealtimeTable,
		),
		runtime: runtime,
	}
}

func (w *InboundWorker) Run(ctx context.Context) {
	w.runtime.SetComponentStatus("inbound", "running", "worker iniciado")
	if err := w.retryQueuedInbound(ctx); err != nil {
		log.Printf("retry queued inbound failed: %v", err)
	}

	go w.retryLoop(ctx, 30*time.Second)

	backoff := newReconnectBackoff(5*time.Second, 60*time.Second)
	failureLog := newReconnectFailureLog()
	connected := false

	for {
		select {
		case <-ctx.Done():
			return
		default:
			err := w.realtimeClient.ListenPedidos(ctx, func(pedido models.Pedido) error {
				return w.insertOrQueue(ctx, pedido)
			}, func() {
				backoff.Reset()
				failureLog.Reset(w.runtime, "inbound")
				connected = true
				w.runtime.SetComponentStatus("inbound", "running", "realtime conectado")
			})
			if err == nil {
				return
			}

			wait := backoff.Next()
			summary := supabase.NormalizeRealtimeError(err)
			failureLog.Record(w.runtime, "inbound", summary, wait)

			if supabase.IsRealtimeTransientError(err) {
				w.runtime.SetComponentStatus(
					"inbound",
					"reconnecting",
					fmt.Sprintf("reconectando en %s: %s", wait, summary),
				)
			} else {
				w.runtime.SetComponentStatus("inbound", "error", summary)
			}

			if connected {
				log.Printf("realtime listen lost, reconnecting in %s: %v", wait, err)
			} else {
				log.Printf("realtime listen failed, reconnecting in %s: %v", wait, err)
			}
			connected = false

			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
	}
}

func (w *InboundWorker) retryLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.retryQueuedInbound(ctx); err != nil {
				log.Printf("periodic retry queued inbound failed: %v", err)
			}
		}
	}
}

func (w *InboundWorker) insertOrQueue(ctx context.Context, pedido models.Pedido) error {
	if err := w.localPG.InsertPedidoIntoBuzon(ctx, pedido); err == nil {
		w.runtime.AddLog(fmt.Sprintf("inbound: pedido recibido id=%s cliente=%d", pedido.IDPedidoNube, pedido.IDCliente))
		return nil
	}

	raw, marshalErr := json.Marshal(pedido)
	if marshalErr != nil {
		return marshalErr
	}

	return w.queue.Enqueue(ctx, "inbound_pedidos", string(raw))
}

func (w *InboundWorker) retryQueuedInbound(ctx context.Context) error {
	jobs, err := w.queue.PeekByDirection(ctx, "inbound_pedidos", 100)
	if err != nil {
		return err
	}

	for _, job := range jobs {
		var pedido models.Pedido
		if err = json.Unmarshal([]byte(job.PayloadJSON), &pedido); err != nil {
			_ = w.queue.Delete(ctx, job.ID)
			continue
		}

		if err = w.localPG.InsertPedidoIntoBuzon(ctx, pedido); err != nil {
			return err
		}
		w.runtime.AddLog(fmt.Sprintf("inbound: pedido reintentado id=%s cliente=%d", pedido.IDPedidoNube, pedido.IDCliente))

		if err = w.queue.Delete(ctx, job.ID); err != nil {
			return err
		}
	}

	return nil
}
