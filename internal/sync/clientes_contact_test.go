package sync

import (
	"testing"

	"sycronizafhir/internal/db"
)

func TestMergeClienteContactPatchSoloCamposVaciosLocales(t *testing.T) {
	local := map[string]interface{}{
		"clien_id":        10,
		"clien_telefono":  "",
		"clien_domicilio": "Ya tiene",
		"clien_localidad": "",
	}
	remote := map[string]interface{}{
		"clien_id":        10,
		"clien_telefono":  "2644112233",
		"clien_domicilio": "Otro domicilio",
		"clien_localidad": "San Juan",
	}

	patch := mergeClienteContactPatch(local, remote)
	if patch["clien_telefono"] != "2644112233" {
		t.Fatalf("telefono: got %v", patch["clien_telefono"])
	}
	if patch["clien_localidad"] != "San Juan" {
		t.Fatalf("localidad: got %v", patch["clien_localidad"])
	}
	if _, ok := patch["clien_domicilio"]; ok {
		t.Fatalf("no debe pisar domicilio local: %v", patch)
	}
}

func TestMergeClienteContactPatchIgnoraRemotoVacio(t *testing.T) {
	local := map[string]interface{}{"telefono": ""}
	remote := map[string]interface{}{"telefono": "   "}
	if len(mergeClienteContactPatch(local, remote)) != 0 {
		t.Fatal("expected empty patch")
	}
}

func TestMergeClienteInboundPatchRemoteNewerPisaLocal(t *testing.T) {
	meta := db.TableModifiedAtMeta{FechaIsDate: true, HasHoraModificacion: true}
	local := map[string]interface{}{
		"clien_telefono":      "2644000000",
		"clien_domicilio":     "Viejo",
		"fecha_modificacion":  "2026-07-01",
		"hora_modificacion":   "10:00:00",
	}
	remote := map[string]interface{}{
		"clien_telefono":     "2644112233",
		"clien_domicilio":    "Nuevo desde Picking",
		"fecha_modificacion": "2026-07-08",
		"hora_modificacion":  "16:00:00",
	}

	patch := mergeClienteInboundPatch(local, remote, meta)
	if patch["clien_telefono"] != "2644112233" {
		t.Fatalf("telefono: got %v", patch["clien_telefono"])
	}
	if patch["clien_domicilio"] != "Nuevo desde Picking" {
		t.Fatalf("domicilio: got %v", patch["clien_domicilio"])
	}
}
