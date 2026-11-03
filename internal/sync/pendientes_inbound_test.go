package sync

import (
	"testing"
	"time"

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

func TestPendientesInboundSinceUsesArgentinaWallClock(t *testing.T) {
	// 18:00 UTC = 15:00 ART; con 5 min de solape → 14:55 reloj argentino.
	lastRun := time.Date(2026, 11, 3, 18, 0, 0, 0, time.UTC)
	got := pendientesInboundSince(lastRun)
	want := time.Date(2026, 11, 3, 14, 55, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("since = %v, want %v", got, want)
	}
}

func TestPendientesInboundSinceIncludesRecentTiendaChange(t *testing.T) {
	// La tienda modificó a las 15:01 ART; el ciclo anterior corrió 15:00 ART (18:00 UTC).
	lastRun := time.Date(2026, 11, 3, 18, 0, 0, 0, time.UTC)
	meta := db.TableModifiedAtMeta{HasHoraModificacion: true, FechaIsDate: true}
	row := map[string]interface{}{"fecha_modificacion": "2026-11-03", "hora_modificacion": "15:01:00"}
	modAt, ok := db.RowModifiedAt(row, meta)
	if !ok {
		t.Fatal("RowModifiedAt failed")
	}
	if !modAt.After(pendientesInboundSince(lastRun)) {
		t.Fatalf("cambio %v quedó fuera de la ventana %v", modAt, pendientesInboundSince(lastRun))
	}
	// Con el bug viejo (comparar contra UTC) quedaba afuera:
	if modAt.After(lastRun) {
		t.Fatal("el test ya no reproduce el bug original")
	}
}

func TestPendientesInboundSinceAroundMidnight(t *testing.T) {
	// 01:00 UTC del día 4 = 22:00 ART del día 3.
	got := pendientesInboundSince(time.Date(2026, 11, 4, 1, 5, 0, 0, time.UTC))
	if got.Day() != 3 || got.Hour() != 22 || got.Minute() != 0 {
		t.Fatalf("since = %v, want 2026-11-03 22:00", got)
	}
}
