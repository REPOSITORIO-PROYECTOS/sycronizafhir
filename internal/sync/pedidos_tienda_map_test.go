package sync

import (
	"testing"
	"time"
)

func TestPedIDFromPaginaTienda(t *testing.T) {
	if got := pedIDFromPaginaTienda(12345); got != "WEB-000012345" {
		t.Fatalf("pedIDFromPaginaTienda(12345) = %q, want WEB-000012345", got)
	}
	if got := pedIDFromPaginaTienda(0); got != "WEB-000000000" {
		t.Fatalf("pedIDFromPaginaTienda(0) = %q", got)
	}
}

func TestMapPedidoPaginaToCabecera(t *testing.T) {
	head := map[string]interface{}{
		"razonsocial": "Cliente Test",
		"email":       "test@example.com",
		"comentario":  "obs",
		"fecha":       "2026-07-08",
	}
	lines := []map[string]interface{}{
		{
			"prod_cant":           2,
			"prod_precio":         10.5,
			"fecha_modificacion":  "2026-07-08",
			"hora_modificacion":   "14:30:00",
		},
		{"prod_cant": 1, "prod_precio": 5.0},
	}
	cab := mapPedidoPaginaToCabecera(42, head, lines, 100)
	if cab["ped_id"] != "WEB-000000042" {
		t.Fatalf("ped_id = %v", cab["ped_id"])
	}
	if cab["estado"] != "N" {
		t.Fatalf("estado = %v, want N", cab["estado"])
	}
	if cab["ped_total"] != 26.0 {
		t.Fatalf("ped_total = %v, want 26", cab["ped_total"])
	}
	if cab["clien_id"] != int16(100) {
		t.Fatalf("clien_id = %v", cab["clien_id"])
	}
	fecha, ok := cab["fecha_modificacion"].(time.Time)
	if !ok || fecha.Format("2006-01-02") != "2026-07-08" {
		t.Fatalf("fecha_modificacion = %v", cab["fecha_modificacion"])
	}
	hora, ok := cab["hora_modificacion"].(time.Time)
	if !ok || hora.Hour() != 14 || hora.Minute() != 30 {
		t.Fatalf("hora_modificacion = %v", cab["hora_modificacion"])
	}
}

func TestMapPedidoPaginaLinesToPedidosD(t *testing.T) {
	lines := []map[string]interface{}{
		{
			"prod_codigo":      "00123456",
			"prod_descripcion": "Producto A",
			"prod_cant":        3,
			"prod_precio":      12.5,
		},
	}
	out := mapPedidoPaginaLinesToPedidosD("WEB-000000001", lines)
	if len(out) != 1 {
		t.Fatalf("len = %d", len(out))
	}
	if out[0]["estado"] != "N" {
		t.Fatalf("estado = %v", out[0]["estado"])
	}
	if out[0]["prod_id"] != "00123456" {
		t.Fatalf("prod_id = %v", out[0]["prod_id"])
	}
	if out[0]["ped_importe"] != 37.5 {
		t.Fatalf("ped_importe = %v", out[0]["ped_importe"])
	}
}

func TestNormalizeCuitDigits(t *testing.T) {
	if got := normalizeCuitDigits("20-12345678-9"); got != "20123456789" {
		t.Fatalf("normalizeCuitDigits = %q", got)
	}
}

func TestNormalizeProdID(t *testing.T) {
	if got := normalizeProdID("123"); got != "00000123" {
		t.Fatalf("normalizeProdID(123) = %q", got)
	}
	if got := normalizeProdID("0012345678"); got != "12345678" {
		t.Fatalf("normalizeProdID long = %q", got)
	}
}
