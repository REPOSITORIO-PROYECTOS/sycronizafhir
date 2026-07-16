package sync

import (
	"context"
	"fmt"
	"strings"

	"sycronizafhir/internal/db"
)

// Columnas editables desde Picking/tienda en Supabase → Mica.
// `web` = flag alta tienda (S/N). `clien_web` = URL/contacto legacy (NO es el flag).
var clientesInboundColumnPairs = [][2]string{
	{"clien_telefono", "telefono"},
	{"clien_domicilio", "domicilio"},
	{"clien_localidad", "localidad"},
	{"clien_email", "email"},
	{"clien_cuit", "cuit"},
	{"clien_web", "clien_web"},
	{"web", "web"},
}

// Solo la columna char `web` gobierna el alta tienda; no mezclar con clien_web.
var clienteWebFlagColumns = []string{"web"}

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

// preserveClienteWebAgainstDowngrade es un envoltorio del guard genérico para el
// flag `web` de clientes (caso Riera 1358). La lógica vive en cloud_owned.go.
func preserveClienteWebAgainstDowngrade(
	localRows []map[string]interface{},
	remoteRows []map[string]interface{},
	pkColumns []string,
) int {
	return preserveCloudOwnedFlags(localRows, remoteRows, pkColumns, clienteWebFlagColumns)
}

// applyClienteWebOutboundGuard delega en el guard genérico de campos propiedad de
// la nube usando la allowlist específica de clientes.
func applyClienteWebOutboundGuard(
	ctx context.Context,
	loader cloudOwnedRowLoader,
	remoteSchema, remoteTable string,
	pkColumns []string,
	rows []map[string]interface{},
) (int, error) {
	return applyCloudOwnedOutboundGuard(ctx, loader, remoteSchema, remoteTable, pkColumns, rows, clienteWebFlagColumns)
}

// clienteWebEnabled alinea con picking `cliente_web_habilitado`: S/1/true o no vacío ≠ N.
func clienteWebEnabled(raw interface{}) bool {
	return cloudFieldEnabled(raw)
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
