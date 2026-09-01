package sync

import (
	"context"
	"fmt"
	"strings"

	"sycronizafhir/internal/db"
)

// Estados que Picking escribe en Supabase y deben bajar a Mica.
// P no va en este set: solo se aplica K→P (soltar). C/A siguen siendo del ERP.
var pedidosEstadoPicking = map[string]struct{}{
	"K": {},
	"V": {},
	"E": {},
}

var pedidoEstadoStampColumns = []string{
	"fecha_modificacion",
	"hora_modificacion",
}

var pedidoBultoColumnCandidates = []string{
	"bulto",
	"nro_bulto",
	"ped_bulto",
	"cant_bultos",
}

func mergePedidoEstadoPatch(
	localRow, remoteRow map[string]interface{},
	meta db.TableModifiedAtMeta,
) map[string]interface{} {
	patch := make(map[string]interface{})

	remoteEstado := normalizePedidoEstado(remoteRow["estado"])
	if remoteEstado == "" {
		return patch
	}

	localEstado := normalizePedidoEstado(localRow["estado"])
	remoteAt, remoteOK := db.RowModifiedAt(remoteRow, meta)
	localAt, localOK := db.RowModifiedAt(localRow, meta)
	remoteNewer := remoteOK && (!localOK || remoteAt.After(localAt))

	if shouldApplyPedidoEstado(localEstado, remoteEstado, remoteNewer) {
		patch["estado"] = remoteEstado
	}

	for _, column := range pedidoBultoColumnCandidates {
		remoteValue, remoteOK := remoteRow[column]
		if !remoteOK || isBlankValue(remoteValue) {
			continue
		}
		if _, localHas := localRow[column]; !localHas {
			continue
		}
		localValue := localRow[column]
		if isBlankValue(localValue) || remoteNewer {
			patch[column] = remoteValue
		}
	}

	return patch
}

func shouldApplyPedidoEstado(localEstado, remoteEstado string, remoteNewer bool) bool {
	if remoteEstado == "" || remoteEstado == localEstado {
		return false
	}
	if !remoteNewer {
		return false
	}
	if remoteEstado == "P" {
		return localEstado == "K"
	}
	if _, ok := pedidosEstadoPicking[remoteEstado]; ok {
		return true
	}
	return pedidoEstadoRank(remoteEstado) > pedidoEstadoRank(localEstado)
}

func normalizePedidoEstado(raw interface{}) string {
	text := strings.ToUpper(strings.TrimSpace(fmtPedidoEstado(raw)))
	if text == "" {
		return ""
	}
	return text[:1]
}

func fmtPedidoEstado(raw interface{}) string {
	if raw == nil {
		return ""
	}
	switch typed := raw.(type) {
	case string:
		return typed
	case []byte:
		return string(typed)
	default:
		return fmt.Sprintf("%v", typed)
	}
}

func shouldPreserveCloudPedidoEstado(erpEstado, cloudEstado string) bool {
	cloud := normalizePedidoEstado(cloudEstado)
	erp := normalizePedidoEstado(erpEstado)
	if cloud == "" || cloud == erp {
		return false
	}
	// Soltar: Picking ya dejó P en la nube y Mica sigue en K.
	if cloud == "P" && erp == "K" {
		return true
	}
	if _, ok := pedidosEstadoPicking[cloud]; !ok {
		return false
	}
	if cloud == "P" {
		return false
	}
	return pedidoEstadoRank(cloud) >= pedidoEstadoRank(erp)
}

func preservePickingPedidoEstado(
	localRows []map[string]interface{},
	remoteRows []map[string]interface{},
	pkColumns []string,
) int {
	if len(localRows) == 0 || len(remoteRows) == 0 {
		return 0
	}
	remoteByPK := mapRowsByPK(remoteRows, pkColumns)
	preserved := 0
	for _, local := range localRows {
		key, err := PKKey(local, pkColumns)
		if err != nil {
			continue
		}
		remote, ok := remoteByPK[key]
		if !ok {
			continue
		}
		if !shouldPreserveCloudPedidoEstado(
			fmtPedidoEstado(local["estado"]),
			fmtPedidoEstado(remote["estado"]),
		) {
			continue
		}
		local["estado"] = remote["estado"]
		preserved++
		for _, column := range pedidoEstadoStampColumns {
			remoteValue, hasRemote := remote[column]
			if !hasRemote || isBlankValue(remoteValue) {
				continue
			}
			if _, hasLocal := local[column]; !hasLocal {
				continue
			}
			local[column] = remoteValue
		}
	}
	return preserved
}

func applyPedidosPickingEstadoOutboundGuard(
	ctx context.Context,
	loader cloudOwnedRowLoader,
	remoteSchema, tableName string,
	pkColumns []string,
	rows []map[string]interface{},
) (int, error) {
	if tableName != "pedidos" || loader == nil || len(rows) == 0 || len(pkColumns) == 0 {
		return 0, nil
	}
	pkOnly := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		one := make(map[string]interface{}, len(pkColumns))
		complete := true
		for _, column := range pkColumns {
			value, ok := row[column]
			if !ok || value == nil {
				complete = false
				break
			}
			one[column] = value
		}
		if complete {
			pkOnly = append(pkOnly, one)
		}
	}
	if len(pkOnly) == 0 {
		return 0, nil
	}
	remoteRows, err := loader.LoadRowsByPrimaryKeys(ctx, remoteSchema, tableName, pkColumns, pkOnly)
	if err != nil {
		return 0, err
	}
	return preservePickingPedidoEstado(rows, remoteRows, pkColumns), nil
}

func pedidoEstadoRank(estado string) int {
	switch normalizePedidoEstado(estado) {
	case "P":
		return 10
	case "C":
		return 15
	case "K":
		return 20
	case "E":
		return 25
	case "V":
		return 30
	default:
		return 0
	}
}
