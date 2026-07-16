package config

import "testing"

func TestRemovedCoreTables(t *testing.T) {
	removed := RemovedCoreTables([]string{"clientes"})
	if len(removed) != 2 {
		t.Fatalf("removed=%v want [productos productos_depositos]", removed)
	}
	if got := RemovedCoreTables([]string{"clientes", "productos", "productos_depositos", "otra"}); len(got) != 0 {
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
		t.Fatalf("productos no debe tener campos protegidos por defecto")
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
