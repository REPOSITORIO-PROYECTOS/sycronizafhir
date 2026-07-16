package sync

import (
	"testing"

	"sycronizafhir/internal/db"
)

func TestMergePedidoEstadoPatchPToK(t *testing.T) {
	meta := db.TableModifiedAtMeta{FechaIsDate: true}
	local := map[string]interface{}{
		"ped_id":             "2026-00000001",
		"estado":             "P",
		"fecha_modificacion": "2026-07-01",
	}
	remote := map[string]interface{}{
		"ped_id":             "2026-00000001",
		"estado":             "K",
		"fecha_modificacion": "2026-07-08",
	}

	patch := mergePedidoEstadoPatch(local, remote, meta)
	if patch["estado"] != "K" {
		t.Fatalf("estado: got %v", patch["estado"])
	}
}

func TestMergePedidoEstadoPatchNoDowngradeSinFechaNueva(t *testing.T) {
	meta := db.TableModifiedAtMeta{FechaIsDate: true}
	local := map[string]interface{}{
		"estado":             "V",
		"fecha_modificacion": "2026-07-08",
	}
	remote := map[string]interface{}{
		"estado":             "K",
		"fecha_modificacion": "2026-07-01",
	}

	if len(mergePedidoEstadoPatch(local, remote, meta)) != 0 {
		t.Fatal("expected empty patch when remote is older")
	}
}

func TestMergePedidoEstadoPatchBulto(t *testing.T) {
	meta := db.TableModifiedAtMeta{FechaIsDate: true}
	local := map[string]interface{}{
		"estado":             "K",
		"bulto":              nil,
		"fecha_modificacion": "2026-07-01",
	}
	remote := map[string]interface{}{
		"estado":             "V",
		"bulto":              3,
		"fecha_modificacion": "2026-07-08",
	}

	patch := mergePedidoEstadoPatch(local, remote, meta)
	if patch["estado"] != "V" {
		t.Fatalf("estado: got %v", patch["estado"])
	}
	if patch["bulto"] != 3 {
		t.Fatalf("bulto: got %v", patch["bulto"])
	}
}
