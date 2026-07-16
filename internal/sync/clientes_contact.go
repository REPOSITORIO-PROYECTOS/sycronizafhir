package sync

import (
	"fmt"
	"strings"

	"sycronizafhir/internal/db"
)

// Columnas editables desde Picking/tienda en Supabase → Mica (pares clien_* / corto).
var clientesInboundColumnPairs = [][2]string{
	{"clien_telefono", "telefono"},
	{"clien_domicilio", "domicilio"},
	{"clien_localidad", "localidad"},
	{"clien_email", "email"},
	{"clien_cuit", "cuit"},
	{"clien_web", "web"},
	{"web", "web"},
}

func mergeClienteContactPatch(localRow, remoteRow map[string]interface{}) map[string]interface{} {
	return mergeClienteInboundPatch(localRow, remoteRow, db.TableModifiedAtMeta{})
}

func mergeClienteInboundPatch(
	localRow, remoteRow map[string]interface{},
	meta db.TableModifiedAtMeta,
) map[string]interface{} {
	patch := make(map[string]interface{})
	seen := map[string]bool{}

	remoteAt, remoteOK := db.RowModifiedAt(remoteRow, meta)
	localAt, localOK := db.RowModifiedAt(localRow, meta)
	remoteNewer := remoteOK && (!localOK || remoteAt.After(localAt))

	for _, pair := range clientesInboundColumnPairs {
		for _, column := range pair {
			if seen[column] {
				continue
			}
			remoteValue, remoteOK := remoteRow[column]
			if !remoteOK || isBlankValue(remoteValue) {
				continue
			}
			localValue, localOK := localRow[column]
			localBlank := !localOK || isBlankValue(localValue)
			if localBlank || remoteNewer {
				patch[column] = remoteValue
				seen[column] = true
			}
		}
	}

	return patch
}

func isBlankValue(raw interface{}) bool {
	if raw == nil {
		return true
	}
	switch typed := raw.(type) {
	case string:
		return strings.TrimSpace(typed) == ""
	case []byte:
		return strings.TrimSpace(string(typed)) == ""
	default:
		text := strings.TrimSpace(fmt.Sprintf("%v", typed))
		return text == "" || text == "<nil>"
	}
}
