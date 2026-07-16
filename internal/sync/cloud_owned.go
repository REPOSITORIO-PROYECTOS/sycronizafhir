package sync

import (
	"context"
	"fmt"
	"strings"
)

// cloudOwnedRowLoader es la capacidad mínima que necesita la guarda: leer las
// filas remotas por clave primaria para comparar el estado en la nube.
type cloudOwnedRowLoader interface {
	LoadRowsByPrimaryKeys(ctx context.Context, schemaName, tableName string, pkColumns []string, pkRows []map[string]interface{}) ([]map[string]interface{}, error)
}

// cloudFieldEnabled interpreta un valor como "habilitado" (alta activa en la
// nube): S/1/true/URL o cualquier texto no vacío distinto de N/0/false.
func cloudFieldEnabled(raw interface{}) bool {
	if isBlankValue(raw) {
		return false
	}
	text := strings.TrimSpace(fmt.Sprintf("%v", raw))
	low := strings.ToLower(text)
	switch low {
	case "n", "0", "false", "no", "f":
		return false
	case "s", "1", "true", "y", "yes", "t":
		return true
	}
	if strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") {
		return true
	}
	return len(text) >= 1
}

// preserveCloudOwnedFlags evita que el outbound pise un campo "propiedad de la
// nube" habilitado (S) con un valor local deshabilitado (N/vacío). Muta las
// filas locales in situ y devuelve cuántas celdas preservó.
func preserveCloudOwnedFlags(
	localRows []map[string]interface{},
	remoteRows []map[string]interface{},
	pkColumns []string,
	fields []string,
) int {
	if len(localRows) == 0 || len(remoteRows) == 0 || len(fields) == 0 {
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
		for _, column := range fields {
			localValue, hasLocal := local[column]
			if !hasLocal {
				continue
			}
			remoteValue, hasRemote := remote[column]
			if !hasRemote {
				continue
			}
			if !cloudFieldEnabled(localValue) && cloudFieldEnabled(remoteValue) {
				local[column] = remoteValue
				preserved++
			}
		}
	}
	return preserved
}

// applyCloudOwnedOutboundGuard carga las filas remotas por PK y preserva los
// campos propiedad de la nube. Si falla la lectura remota, no bloquea el upsert.
func applyCloudOwnedOutboundGuard(
	ctx context.Context,
	loader cloudOwnedRowLoader,
	remoteSchema, remoteTable string,
	pkColumns []string,
	rows []map[string]interface{},
	fields []string,
) (int, error) {
	if loader == nil || len(rows) == 0 || len(pkColumns) == 0 || len(fields) == 0 {
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
	remoteRows, err := loader.LoadRowsByPrimaryKeys(ctx, remoteSchema, remoteTable, pkColumns, pkOnly)
	if err != nil {
		return 0, err
	}
	return preserveCloudOwnedFlags(rows, remoteRows, pkColumns, fields), nil
}
