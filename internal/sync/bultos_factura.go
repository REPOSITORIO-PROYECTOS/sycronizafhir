package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"sycronizafhir/internal/config"
	"sycronizafhir/internal/db"
	"sycronizafhir/internal/monitor"
	"sycronizafhir/internal/support"
)

const (
	DefaultBultosFacturaMax        = 30
	DefaultBultosFacturaWindowDays = 30
	bultosFacturaLogName           = "bultos_factura.jsonl"
)

// Acciones del paso facturas_vtas.bultos → pedidos.bultos.
const (
	BultosFacturaCopy         = "copy"
	BultosFacturaSkipOverCap  = "skip_over_cap"
	BultosFacturaSkipExisting = "skip_existing"
	BultosFacturaSkipInvalid  = "skip_invalid"
)

// BultosFacturaLogEntry registra cada decisión para auditoría y rollback.
// No incluye datos de cliente.
type BultosFacturaLogEntry struct {
	Timestamp     time.Time `json:"timestamp"`
	DryRun        bool      `json:"dry_run"`
	Action        string    `json:"action"`
	PedID         string    `json:"ped_id"`
	ValorAnterior *int      `json:"valor_anterior"`
	ValorNuevo    int       `json:"valor_nuevo"`
	Factura       string    `json:"factura"`
	FacturaFecha  string    `json:"factura_fecha,omitempty"`
	Applied       bool      `json:"applied"`
}

// BultosFacturaCycleResult resume un ciclo (dry-run o apply).
type BultosFacturaCycleResult struct {
	Candidates   int
	Copied       int
	SkippedCap   int
	SkippedOther int
	Applied      int
	Entries      []BultosFacturaLogEntry
}

// ClassifyBultosFactura decide si se puede copiar el valor de la factura.
// Copia solo 1..max cuando el pedido está vacío (nil o 0). Nunca pisa un valor
// ya cargado. Los mayores a max quedan para revisar (skip_over_cap).
func ClassifyBultosFactura(pedidoBultos *int16, facturaBultos int, max int) string {
	if facturaBultos < 1 {
		return BultosFacturaSkipInvalid
	}
	if pedidoBultos != nil && *pedidoBultos > 0 {
		return BultosFacturaSkipExisting
	}
	if max <= 0 {
		max = DefaultBultosFacturaMax
	}
	if facturaBultos > max {
		return BultosFacturaSkipOverCap
	}
	return BultosFacturaCopy
}

func intPtrFromInt16(v *int16) *int {
	if v == nil {
		return nil
	}
	n := int(*v)
	return &n
}

func facturaFechaString(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

func buildBultosFacturaEntry(c db.BultosFacturaCandidate, action string, dryRun bool, now time.Time) BultosFacturaLogEntry {
	return BultosFacturaLogEntry{
		Timestamp:     now.UTC(),
		DryRun:        dryRun,
		Action:        action,
		PedID:         c.PedID,
		ValorAnterior: intPtrFromInt16(c.PedidoBultos),
		ValorNuevo:    int(c.FacturaBultos),
		Factura:       c.FacturaComprobante,
		FacturaFecha:  facturaFechaString(c.FacturaFecha),
		Applied:       false,
	}
}

// BultosFacturaLogPath is the JSONL audit/rollback log.
func BultosFacturaLogPath() (string, error) {
	if err := support.EnsureDirs(); err != nil {
		return "", err
	}
	stateDir, err := support.StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(stateDir, bultosFacturaLogName), nil
}

func appendBultosFacturaLog(entries []BultosFacturaLogEntry) error {
	if len(entries) == 0 {
		return nil
	}
	path, err := BultosFacturaLogPath()
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, entry := range entries {
		if err := enc.Encode(entry); err != nil {
			return err
		}
	}
	return nil
}

// BultosFacturaWorker copia facturas_vtas.bultos → pedidos.bultos cuando el
// pedido está vacío. Por defecto corre en dry-run (no escribe).
type BultosFacturaWorker struct {
	localPG      *db.LocalPG
	sourceSchema string
	enabled      bool
	dryRun       bool
	maxBultos    int
	windowDays   int
	interval     time.Duration
	runtime      *monitor.Runtime
}

func NewBultosFacturaWorker(localPG *db.LocalPG, cfg config.Config, runtime *monitor.Runtime) *BultosFacturaWorker {
	maxBultos := cfg.BultosFacturaMax
	if maxBultos <= 0 {
		maxBultos = DefaultBultosFacturaMax
	}
	windowDays := cfg.BultosFacturaWindowDays
	if windowDays <= 0 {
		windowDays = DefaultBultosFacturaWindowDays
	}
	interval := cfg.BultosFacturaInterval
	if interval <= 0 {
		interval = time.Minute
	}
	return &BultosFacturaWorker{
		localPG:      localPG,
		sourceSchema: cfg.SourceSchema,
		enabled:      cfg.BultosFacturaEnabled,
		dryRun:       cfg.BultosFacturaDryRun,
		maxBultos:    maxBultos,
		windowDays:   windowDays,
		interval:     interval,
		runtime:      runtime,
	}
}

func (w *BultosFacturaWorker) Run(ctx context.Context) {
	if !w.enabled {
		w.runtime.SetComponentStatus("bultos_factura", "idle", "deshabilitado (BULTOS_FACTURA_ENABLED=false)")
		<-ctx.Done()
		return
	}
	mode := "apply"
	if w.dryRun {
		mode = "dry-run"
	}
	w.runtime.SetComponentStatus("bultos_factura", "running", "worker iniciado ("+mode+")")

	if err := w.runCycle(ctx); err != nil {
		log.Printf("bultos factura initial cycle failed: %v", err)
		w.runtime.SetComponentStatus("bultos_factura", "error", err.Error())
	}

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.runCycle(ctx); err != nil {
				log.Printf("bultos factura cycle failed: %v", err)
				w.runtime.SetComponentStatus("bultos_factura", "error", err.Error())
			}
		}
	}
}

func (w *BultosFacturaWorker) runCycle(ctx context.Context) error {
	result, err := RunBultosFacturaCycle(ctx, w.localPG, w.sourceSchema, w.windowDays, w.maxBultos, w.dryRun)
	if err != nil {
		return err
	}
	mode := "apply"
	if w.dryRun {
		mode = "dry-run"
	}
	msg := fmt.Sprintf(
		"%s: %d candidato(s), %d a copiar, %d fuera de tope %d, %d aplicados",
		mode, result.Candidates, result.Copied, result.SkippedCap, w.maxBultos, result.Applied,
	)
	w.runtime.AddLog("bultos factura: " + msg)
	w.runtime.SetComponentStatus("bultos_factura", "running", msg)
	if result.SkippedCap > 0 {
		w.runtime.AddLog(fmt.Sprintf("bultos factura: %d pedido(s) con bultos > %d quedan sin copiar (ver %s)", result.SkippedCap, w.maxBultos, bultosFacturaLogName))
	}
	return nil
}

// RunBultosFacturaCycle ejecuta un ciclo (usado por el worker y el CLI).
func RunBultosFacturaCycle(
	ctx context.Context,
	localPG *db.LocalPG,
	schema string,
	windowDays, maxBultos int,
	dryRun bool,
) (BultosFacturaCycleResult, error) {
	var result BultosFacturaCycleResult
	if schema == "" {
		schema = "public"
	}
	if maxBultos <= 0 {
		maxBultos = DefaultBultosFacturaMax
	}
	if windowDays <= 0 {
		windowDays = DefaultBultosFacturaWindowDays
	}

	candidates, err := localPG.ListBultosFacturaForEmptyPedidos(ctx, schema, windowDays)
	if err != nil {
		return result, err
	}
	result.Candidates = len(candidates)
	now := time.Now()
	entries := make([]BultosFacturaLogEntry, 0, len(candidates))

	for _, c := range candidates {
		action := ClassifyBultosFactura(c.PedidoBultos, int(c.FacturaBultos), maxBultos)
		entry := buildBultosFacturaEntry(c, action, dryRun, now)
		switch action {
		case BultosFacturaCopy:
			result.Copied++
			if !dryRun {
				applied, applyErr := localPG.ApplyPedidoBultosIfEmpty(ctx, schema, c.PedID, c.FacturaBultos)
				if applyErr != nil {
					return result, applyErr
				}
				entry.Applied = applied
				if applied {
					result.Applied++
				}
			}
		case BultosFacturaSkipOverCap:
			result.SkippedCap++
		default:
			result.SkippedOther++
		}
		entries = append(entries, entry)
	}
	result.Entries = entries
	if err := appendBultosFacturaLog(entries); err != nil {
		return result, fmt.Errorf("append bultos factura log: %w", err)
	}
	return result, nil
}
