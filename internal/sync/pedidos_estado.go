package sync

import (
	"fmt"
	"strings"

	"sycronizafhir/internal/db"
)

// Estados que Picking escribe en Supabase y deben bajar a Mica.
var pedidosEstadoPicking = map[string]struct{}{
	"K": {},
	"V": {},
	"E": {},
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
