package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemovedCoreTables(t *testing.T) {
	removed := RemovedCoreTables([]string{"clientes"})
	if len(removed) != 4 {
		t.Fatalf("removed=%v want [productos productos_depositos rubro subrubro]", removed)
	}
	if got := RemovedCoreTables([]string{"clientes", "productos", "productos_depositos", "rubro", "subrubro", "otra"}); len(got) != 0 {
		t.Fatalf("con todas las core presentes no debe reportar removidas, got %v", got)
	}
}

func TestHasEnabledTables(t *testing.T) {
	if HasEnabledTables([]string{"", "  "}) {
		t.Fatal("set solo con vacíos no debe contar como habilitado")
	}
	if !HasEnabledTables([]string{"clientes"}) {
		t.Fatal("clientes debe contar como habilitado")
	}
}

func TestDefaultCloudOwnedFieldsProtegeClientesWeb(t *testing.T) {
	cfg := DefaultSyncTablesConfig()
	fields := cfg.CloudOwnedFieldsFor("clientes")
	want := map[string]bool{
		"web": true, "clien_celular": true, "celular": true,
		"clien_cp": true, "cp": true, "coordenadas": true,
	}
	got := map[string]bool{}
	for _, f := range fields {
		got[f] = true
	}
	for name := range want {
		if !got[name] {
			t.Fatalf("clientes debe proteger %s por defecto, got %v", name, fields)
		}
	}
	if cfg.CloudOwnedFieldsFor("productos") != nil {
		t.Fatalf("productos no debe tener cloud-owned flags por defecto")
	}
	auth := cfg.CloudAuthoritativeFieldsFor("productos")
	if len(auth) != 1 || auth[0] != "prod_orden" {
		t.Fatalf("productos authoritative want [prod_orden], got %v", auth)
	}
	pagina := cfg.CloudAuthoritativeFieldsFor("pedido_pagina")
	if len(pagina) < 1 {
		t.Fatal("pedido_pagina debe declarar columnas nube por defecto")
	}
	cliAuth := cfg.CloudAuthoritativeFieldsFor("clientes")
	if len(cliAuth) < 1 {
		t.Fatal("clientes debe declarar columnas nube autoritativas por defecto")
	}
}

func TestNormalizeCloudOwnedFieldsDropVacios(t *testing.T) {
	in := map[string][]string{
		"clientes": {"web", "", " "},
		"  ":       {"x"},
		"vacia":    {""},
	}
	out := normalizeCloudOwnedFields(in)
	if len(out["clientes"]) != 1 || out["clientes"][0] != "web" {
		t.Fatalf("clientes normalizado inesperado: %v", out["clientes"])
	}
	if _, ok := out["vacia"]; ok {
		t.Fatal("tabla con solo campos vacíos debe descartarse")
	}
	if _, ok := out[""]; ok {
		t.Fatal("nombre de tabla vacío debe descartarse")
	}
}

func TestLoadSyncTablesConfigDefaultsAutoSyncForLegacyFile(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("APPDATA", tempDir)

	cfgDir := filepath.Join(tempDir, "sycronizafhir")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	payload := []byte(`{"enabled_tables":["clientes","rubro","subrubro"]}`)
	if err := os.WriteFile(filepath.Join(cfgDir, "sync-tables.json"), payload, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := LoadSyncTablesConfig()
	if err != nil {
		t.Fatalf("LoadSyncTablesConfig: %v", err)
	}
	if !cfg.AutoSyncOnAudit {
		t.Fatal("auto_sync_on_audit ausente debe heredar true")
	}
	if cfg.AutoAuditIntervalHours != 6 {
		t.Fatalf("AutoAuditIntervalHours=%d want 6", cfg.AutoAuditIntervalHours)
	}
}

func TestLoadSyncTablesConfigStripsUTF8BOM(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("APPDATA", tempDir)

	cfgDir := filepath.Join(tempDir, "sycronizafhir")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	payload := append([]byte{0xEF, 0xBB, 0xBF}, []byte(`{"enabled_tables":["clientes","productos","productos_depositos","pedidos","pedidos_d","rubro","subrubro"]}`)...)
	if err := os.WriteFile(filepath.Join(cfgDir, "sync-tables.json"), payload, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := LoadSyncTablesConfig()
	if err != nil {
		t.Fatalf("LoadSyncTablesConfig con BOM debe parsear: %v", err)
	}
	if !cfg.IsEnabled("pedidos") {
		t.Fatal("pedidos debe quedar habilitado tras strip BOM")
	}
}

func TestStripUTF8BOM(t *testing.T) {
	plain := []byte(`{"a":1}`)
	if got := stripUTF8BOM(plain); string(got) != string(plain) {
		t.Fatalf("sin BOM no debe cambiar, got %q", got)
	}
	with := append([]byte{0xEF, 0xBB, 0xBF}, plain...)
	if got := stripUTF8BOM(with); string(got) != string(plain) {
		t.Fatalf("con BOM debe quitar prefijo, got %q", got)
	}
}
