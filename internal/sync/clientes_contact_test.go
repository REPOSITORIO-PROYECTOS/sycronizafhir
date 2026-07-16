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

func TestPreserveClienteWebAgainstDowngrade(t *testing.T) {
	local := []map[string]interface{}{
		{"clien_id": 1358, "web": "N", "clien_web": ""},
		{"clien_id": 302, "web": "S", "clien_web": ""},
		{"clien_id": 1, "web": "N", "clien_web": ""},
	}
	remote := []map[string]interface{}{
		{"clien_id": 1358, "web": "S", "clien_web": ""},
		{"clien_id": 302, "web": "N", "clien_web": ""},
		{"clien_id": 1, "web": "N", "clien_web": ""},
	}

	n := preserveClienteWebAgainstDowngrade(local, remote, []string{"clien_id"})
	if n != 1 {
		t.Fatalf("preserved count: got %d want 1", n)
	}
	if local[0]["web"] != "S" {
		t.Fatalf("1358 debe conservar web remoto S, got %v", local[0]["web"])
	}
	if local[1]["web"] != "S" {
		t.Fatalf("302 local S no debe bajar a N remoto, got %v", local[1]["web"])
	}
	if local[2]["web"] != "N" {
		t.Fatalf("1 ambos N sin cambio, got %v", local[2]["web"])
	}
}

func TestClienteWebEnabled(t *testing.T) {
	if clienteWebEnabled("N") || clienteWebEnabled("") || clienteWebEnabled(nil) {
		t.Fatal("N/blank deben ser disabled")
	}
	if !clienteWebEnabled("S") || !clienteWebEnabled("s") || !clienteWebEnabled("1") {
		t.Fatal("S/1 deben ser enabled")
	}
}

func TestMergeClienteInboundPatchRemoteNewerPisaLocal(t *testing.T) {
	meta := db.TableModifiedAtMeta{FechaIsDate: true, HasHoraModificacion: true}
	local := map[string]interface{}{
		"clien_telefono":     "2644000000",
		"clien_domicilio":    "Viejo",
		"fecha_modificacion": "2026-07-01",
		"hora_modificacion":  "10:00:00",
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

func TestMergeClienteInboundNoConfundeClienWebConFlag(t *testing.T) {
	meta := db.TableModifiedAtMeta{FechaIsDate: true, HasHoraModificacion: true}
	local := map[string]interface{}{
		"web":                "N",
		"clien_web":          "",
		"fecha_modificacion": "2026-07-01",
		"hora_modificacion":  "10:00:00",
	}
	remote := map[string]interface{}{
		"web":                "S",
		"clien_web":          "https://ejemplo.com",
		"fecha_modificacion": "2026-07-10",
		"hora_modificacion":  "12:00:00",
	}
	patch := mergeClienteInboundPatch(local, remote, meta)
	if patch["web"] != "S" {
		t.Fatalf("flag web: got %v want S", patch["web"])
	}
	if patch["clien_web"] != "https://ejemplo.com" {
		t.Fatalf("clien_web URL: got %v", patch["clien_web"])
	}
}

func TestPreserveClienteWebSoloColumnaFlag(t *testing.T) {
	local := []map[string]interface{}{
		{"clien_id": 1, "web": "N", "clien_web": ""},
	}
	remote := []map[string]interface{}{
		{"clien_id": 1, "web": "S", "clien_web": "https://x.com"},
	}
	n := preserveClienteWebAgainstDowngrade(local, remote, []string{"clien_id"})
	if n != 1 {
		t.Fatalf("preserved=%d want 1 (solo web)", n)
	}
	if local[0]["web"] != "S" {
		t.Fatalf("web got %v", local[0]["web"])
	}
	if local[0]["clien_web"] != "" {
		t.Fatalf("clien_web no debe tocarse como flag, got %v", local[0]["clien_web"])
	}
}

func TestMergeClienteInboundPermiteDestildWebSiRemotoMasNuevo(t *testing.T) {
	meta := db.TableModifiedAtMeta{FechaIsDate: true, HasHoraModificacion: true}
	local := map[string]interface{}{
		"web":                "S",
		"fecha_modificacion": "2026-07-01",
		"hora_modificacion":  "10:00:00",
	}
	remote := map[string]interface{}{
		"web":                "N",
		"fecha_modificacion": "2026-07-10",
		"hora_modificacion":  "12:00:00",
	}
	patch := mergeClienteInboundPatch(local, remote, meta)
	if patch["web"] != "N" {
		t.Fatalf("inbound debe poder destildar si Picking/nube es más nueva: got %v", patch["web"])
	}
}
