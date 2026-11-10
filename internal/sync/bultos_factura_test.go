package sync

import (
	"encoding/json"
	"testing"
	"time"

	"sycronizafhir/internal/db"
)

func TestClassifyBultosFacturaCopiaEnRango(t *testing.T) {
	if got := ClassifyBultosFactura(nil, 1, 30); got != BultosFacturaCopy {
		t.Fatalf("nil,1: %s", got)
	}
	if got := ClassifyBultosFactura(int16Ptr(0), 30, 30); got != BultosFacturaCopy {
		t.Fatalf("0,30: %s", got)
	}
	if got := ClassifyBultosFactura(nil, 21, 30); got != BultosFacturaCopy {
		t.Fatalf("nil,21 (tope 30, no 20): %s", got)
	}
}

func TestClassifyBultosFacturaNoPisaExistente(t *testing.T) {
	if got := ClassifyBultosFactura(int16Ptr(1), 4, 30); got != BultosFacturaSkipExisting {
		t.Fatalf("existing 1: %s", got)
	}
	if got := ClassifyBultosFactura(int16Ptr(5), 90, 30); got != BultosFacturaSkipExisting {
		t.Fatalf("existing wins over cap: %s", got)
	}
}

func TestClassifyBultosFacturaTope30(t *testing.T) {
	if got := ClassifyBultosFactura(nil, 31, 30); got != BultosFacturaSkipOverCap {
		t.Fatalf("31: %s", got)
	}
	if got := ClassifyBultosFactura(nil, 90, 30); got != BultosFacturaSkipOverCap {
		t.Fatalf("90: %s", got)
	}
	if got := ClassifyBultosFactura(int16Ptr(0), 99, 30); got != BultosFacturaSkipOverCap {
		t.Fatalf("99: %s", got)
	}
}

func TestClassifyBultosFacturaInvalido(t *testing.T) {
	if got := ClassifyBultosFactura(nil, 0, 30); got != BultosFacturaSkipInvalid {
		t.Fatalf("0: %s", got)
	}
	if got := ClassifyBultosFactura(nil, -1, 30); got != BultosFacturaSkipInvalid {
		t.Fatalf("-1: %s", got)
	}
}

func TestBuildBultosFacturaEntryJSON(t *testing.T) {
	c := db.BultosFacturaCandidate{
		PedID:              "0000-00900001",
		PedidoBultos:       nil,
		FacturaBultos:      3,
		FacturaComprobante: "A-0001-00001234",
		FacturaFecha:       time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC),
	}
	entry := buildBultosFacturaEntry(c, BultosFacturaCopy, true, time.Date(2026, 11, 10, 15, 30, 0, 0, time.UTC))
	if entry.PedID != c.PedID || entry.ValorNuevo != 3 || !entry.DryRun || entry.Applied {
		t.Fatalf("unexpected entry: %+v", entry)
	}
	if entry.ValorAnterior != nil {
		t.Fatalf("valor_anterior should be null")
	}
	if entry.FacturaFecha != "2026-11-10" {
		t.Fatalf("fecha: %s", entry.FacturaFecha)
	}
	raw, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, need := range []string{`"ped_id"`, `"valor_anterior"`, `"valor_nuevo"`, `"factura"`, `"timestamp"`, `"dry_run"`} {
		if !contains(s, need) {
			t.Fatalf("missing %s in %s", need, s)
		}
	}
}

func int16Ptr(v int16) *int16 { return &v }

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})())
}
