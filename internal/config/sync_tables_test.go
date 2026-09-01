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
	if len(fields) != 1 || fields[0] != "web" {
		t.Fatalf("clientes debe proteger web por defecto, got %v", fields)
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
