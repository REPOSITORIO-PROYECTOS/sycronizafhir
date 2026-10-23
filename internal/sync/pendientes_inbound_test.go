package sync

import (
	"testing"

	"sycronizafhir/internal/db"
)

func TestMergePendienteInboundRemoteNewerPisaCantidad(t *testing.T) {
	meta := db.TableModifiedAtMeta{FechaIsDate: true, HasHoraModificacion: true}
	local := map[string]interface{}{
		"ped_id":             "0000-00900001",
		"prod_id":            "00001234",
		"cantidad":           10.0,
		"fecha_modificacion": "2026-10-01",
		"hora_modificacion":  "10:00:00",
	}
	remote := map[string]interface{}{
		"ped_id":             "0000-00900001",
		"prod_id":            "00001234",
		"cantidad":           4.0,
		"fecha_modificacion": "2026-10-23",
		"hora_modificacion":  "12:40:00",
	}

	got := mergePendienteInbound(local, remote, meta)
	if got.Delete {
		t.Fatal("no debe borrar si la cantidad remota es > 0")
	}
	if got.Patch["cantidad"] != 4.0 {
		t.Fatalf("cantidad: got %v want 4", got.Patch["cantidad"])
	}
}

func TestMergePendienteInboundLocalMasNuevaNoPisa(t *testing.T) {
	meta := db.TableModifiedAtMeta{FechaIsDate: true, HasHoraModificacion: true}
	local := map[string]interface{}{
		"cantidad":           10.0,
		"fecha_modificacion": "2026-10-23",
		"hora_modificacion":  "18:00:00",
	}
	remote := map[string]interface{}{
		"cantidad":           1.0,
		"fecha_modificacion": "2026-10-20",
		"hora_modificacion":  "09:00:00",
	}

	got := mergePendienteInbound(local, remote, meta)
	if got.Delete || len(got.Patch) != 0 {
		t.Fatalf("ERP más nueva no debe ceder: %+v", got)
	}
}

func TestMergePendienteInboundCantidadCeroBorra(t *testing.T) {
	meta := db.TableModifiedAtMeta{FechaIsDate: true, HasHoraModificacion: true}
	local := map[string]interface{}{
		"cantidad":           7.0,
		"fecha_modificacion": "2026-10-01",
		"hora_modificacion":  "10:00:00",
	}
	remote := map[string]interface{}{
		"cantidad":           0,
		"fecha_modificacion": "2026-10-23",
		"hora_modificacion":  "12:50:00",
	}

	got := mergePendienteInbound(local, remote, meta)
	if !got.Delete {
		t.Fatalf("cantidad 0 en nube más nueva debe borrar ERP: %+v", got)
	}
	if len(got.Patch) != 0 {
		t.Fatalf("delete no lleva patch: %+v", got.Patch)
	}
}

func TestMergePendienteInboundMismaCantidadNoPatch(t *testing.T) {
	meta := db.TableModifiedAtMeta{FechaIsDate: true, HasHoraModificacion: true}
	local := map[string]interface{}{
		"cantidad":           "10.00",
		"fecha_modificacion": "2026-10-01",
		"hora_modificacion":  "10:00:00",
	}
	remote := map[string]interface{}{
		"cantidad":           10.0,
		"fecha_modificacion": "2026-10-23",
		"hora_modificacion":  "12:00:00",
	}

	got := mergePendienteInbound(local, remote, meta)
	if got.Delete || len(got.Patch) != 0 {
		t.Fatalf("misma cantidad no debe patch: %+v", got)
	}
}

func TestPreserveCloudAuthoritativeFieldsCantidadCero(t *testing.T) {
	local := []map[string]interface{}{
		{"ped_id": "P1", "prod_id": "A", "cantidad": 10.0},
		{"ped_id": "P1", "prod_id": "B", "cantidad": 3.0},
	}
	remote := []map[string]interface{}{
		{"ped_id": "P1", "prod_id": "A", "cantidad": 0},
		{"ped_id": "P1", "prod_id": "B", "cantidad": 5.0},
	}

	n := preserveCloudAuthoritativeFields(local, remote, []string{"ped_id", "prod_id"}, []string{"cantidad"})
	if n != 2 {
		t.Fatalf("preserved=%d want 2 (A=0 y B=5)", n)
	}
	if !cloudValuesEqual(local[0]["cantidad"], 0) {
		t.Fatalf("A debe conservar 0 de la nube, got %v", local[0]["cantidad"])
	}
	if !cloudValuesEqual(local[1]["cantidad"], 5.0) {
		t.Fatalf("B debe quedar 5, got %v", local[1]["cantidad"])
	}
}
