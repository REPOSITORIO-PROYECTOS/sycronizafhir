package sync

import "testing"

func TestPreserveCloudOwnedFlagsGenerico(t *testing.T) {
	local := []map[string]interface{}{
		{"id": 1, "web": "N", "publicado": "N"},
		{"id": 2, "web": "S", "publicado": "N"},
	}
	remote := []map[string]interface{}{
		{"id": 1, "web": "S", "publicado": "S"},
		{"id": 2, "web": "N", "publicado": "S"},
	}

	preserved := preserveCloudOwnedFlags(local, remote, []string{"id"}, []string{"web", "publicado"})
	if preserved != 3 {
		t.Fatalf("preserved=%d want 3 (1.web, 1.publicado, 2.publicado)", preserved)
	}
	if local[0]["web"] != "S" || local[0]["publicado"] != "S" {
		t.Fatalf("id=1 debe conservar remoto habilitado, got web=%v publicado=%v", local[0]["web"], local[0]["publicado"])
	}
	if local[1]["web"] != "S" {
		t.Fatalf("id=2 local S no debe bajar a N remoto, got %v", local[1]["web"])
	}
	if local[1]["publicado"] != "S" {
		t.Fatalf("id=2 publicado debe subir a remoto S, got %v", local[1]["publicado"])
	}
}

func TestPreserveCloudOwnedFlagsSinCampos(t *testing.T) {
	local := []map[string]interface{}{{"id": 1, "web": "N"}}
	remote := []map[string]interface{}{{"id": 1, "web": "S"}}
	if n := preserveCloudOwnedFlags(local, remote, []string{"id"}, nil); n != 0 {
		t.Fatalf("sin campos allowlist no debe preservar nada, got %d", n)
	}
	if local[0]["web"] != "N" {
		t.Fatalf("web no debe tocarse sin allowlist, got %v", local[0]["web"])
	}
}

func TestCloudFieldEnabled(t *testing.T) {
	for _, raw := range []interface{}{"N", "", nil, "0", "false"} {
		if cloudFieldEnabled(raw) {
			t.Fatalf("%v debería estar deshabilitado", raw)
		}
	}
	for _, raw := range []interface{}{"S", "s", "1", "true", "https://x.com"} {
		if !cloudFieldEnabled(raw) {
			t.Fatalf("%v debería estar habilitado", raw)
		}
	}
}
