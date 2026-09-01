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

func TestMergePedidoEstadoPatchKToP(t *testing.T) {
	meta := db.TableModifiedAtMeta{FechaIsDate: true}
	local := map[string]interface{}{
		"estado":             "K",
		"fecha_modificacion": "2026-07-01",
	}
	remote := map[string]interface{}{
		"estado":             "P",
		"fecha_modificacion": "2026-07-08",
	}

	patch := mergePedidoEstadoPatch(local, remote, meta)
	if patch["estado"] != "P" {
		t.Fatalf("estado: got %v", patch["estado"])
	}
}

func TestPreserveCloudPedidoEstadoNoPisaKConPDeERP(t *testing.T) {
	if !shouldPreserveCloudPedidoEstado("P", "K") {
		t.Fatal("ERP P no debe pisar K de picking en la nube")
	}
	if shouldPreserveCloudPedidoEstado("C", "P") {
		t.Fatal("ERP C (Gestiona) debe ganar sobre P en la nube")
	}
	if !shouldPreserveCloudPedidoEstado("K", "P") {
		t.Fatal("Picking soltó a P: no re-subir K del ERP")
	}
	if shouldPreserveCloudPedidoEstado("V", "K") {
		t.Fatal("ERP ya en V no debe volver a K de una nube vieja")
	}
}

func TestMergePedidoEstadoPatchNoDowngradeVToP(t *testing.T) {
	meta := db.TableModifiedAtMeta{FechaIsDate: true}
	local := map[string]interface{}{
		"estado":             "V",
		"fecha_modificacion": "2026-07-01",
	}
	remote := map[string]interface{}{
		"estado":             "P",
		"fecha_modificacion": "2026-07-08",
	}
	if len(mergePedidoEstadoPatch(local, remote, meta)) != 0 {
		t.Fatal("P remoto no debe bajar un V local")
	}
}

func TestPreservePickingPedidoEstadoCopiaStamp(t *testing.T) {
	local := []map[string]interface{}{{
		"ped_id":             "0000-00901750",
		"estado":             "P",
		"fecha_modificacion": "2026-09-01",
		"hora_modificacion":  "10:00:00",
	}}
	remote := []map[string]interface{}{{
		"ped_id":             "0000-00901750",
		"estado":             "K",
		"fecha_modificacion": "2026-09-01",
		"hora_modificacion":  "12:40:00",
	}}
	n := preservePickingPedidoEstado(local, remote, []string{"ped_id"})
	if n != 1 {
		t.Fatalf("preserved=%d", n)
	}
	if local[0]["estado"] != "K" {
		t.Fatalf("estado: got %v", local[0]["estado"])
	}
	if local[0]["hora_modificacion"] != "12:40:00" {
		t.Fatalf("hora: got %v", local[0]["hora_modificacion"])
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
