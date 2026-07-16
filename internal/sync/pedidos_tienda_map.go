package sync

import (
	"fmt"
	"math"
	"strings"
	"time"
)

const pedidoTiendaEstadoNuevo = "N"

func pedIDFromPaginaTienda(pedidoPaginaID int64) string {
	if pedidoPaginaID <= 0 {
		return "WEB-000000000"
	}
	return fmt.Sprintf("WEB-%09d", pedidoPaginaID%1_000_000_000)
}

func mapPedidoPaginaToCabecera(
	pedidoPaginaID int64,
	head map[string]interface{},
	lines []map[string]interface{},
	clienID int16,
) map[string]interface{} {
	total := sumPedidoPaginaLines(lines)
	nombre := stringField(head, "razonsocial", "email")
	if nombre == "" {
		nombre = "TIENDA WEB"
	}
	fechaPedido := time.Now().UTC().Truncate(24 * time.Hour)
	if parsed, ok := parseDateField(head["fecha"]); ok {
		fechaPedido = parsed
	}
	fechaMod, horaMod := resolveModifiedAtFromPagina(head, lines)

	obs := strings.TrimSpace(stringField(head, "comentario"))
	email := strings.TrimSpace(stringField(head, "email"))
	if email != "" {
		if obs != "" {
			obs += " | "
		}
		obs += "email:" + email
	}
	if obs == "" {
		obs = fmt.Sprintf("tienda_web;pagina_id=%d", pedidoPaginaID)
	}

	return map[string]interface{}{
		"ped_id":             pedIDFromPaginaTienda(pedidoPaginaID),
		"ped_fecha":          fechaPedido,
		"ped_nombre":         truncateRunes(nombre, 80),
		"ped_gravado":        total,
		"ped_no_gravado":     0,
		"ped_exento":         0,
		"ped_descuento":      0,
		"ped_iva":            0,
		"ped_total":          total,
		"usu_id":             "tienda",
		"clien_id":           clienID,
		"local_id":           "0000",
		"estado":             pedidoTiendaEstadoNuevo,
		"ped_obs":            truncateRunes(obs, 400),
		"fecha_modificacion": fechaMod,
		"hora_modificacion":  horaMod,
	}
}

func mapPedidoPaginaLinesToPedidosD(pedID string, lines []map[string]interface{}) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(lines))
	for idx, line := range lines {
		cantidad := floatField(line, "prod_cant", "ped_cantidad", "cantidad")
		precio := floatField(line, "prod_precio", "ped_precio_unitario", "precio")
		importe := math.Round(cantidad*precio*100) / 100
		prodID := stringField(line, "prod_codigo", "prod_id", "sku")
		if prodID == "" {
			prodID = "00000001"
		}
		desc := stringField(line, "prod_descripcion", "ped_prod_descripcion", "nombre")
		if desc == "" {
			desc = "Ítem tienda"
		}
		fechaMod, _ := resolveModifiedAtFromPagina(nil, []map[string]interface{}{line})

		out = append(out, map[string]interface{}{
			"ped_id":               pedID,
			"ped_item":             int16(idx + 1),
			"prod_id":              truncateRunes(normalizeProdID(prodID), 8),
			"ped_prod_descripcion": truncateRunes(desc, 200),
			"ped_cantidad":         cantidad,
			"ped_precio_unitario":  precio,
			"ped_importe":          importe,
			"ped_descuento":        0,
			"ped_iva":              0,
			"pendiente":            int16(math.Round(cantidad)),
			"estado":               pedidoTiendaEstadoNuevo,
			"fecha_modificacion":   fechaMod,
		})
	}
	return out
}

// resolveModifiedAtFromPagina devuelve fecha (date) y hora (time) separados como en Mica legacy.
func resolveModifiedAtFromPagina(head map[string]interface{}, lines []map[string]interface{}) (time.Time, time.Time) {
	now := time.Now().UTC()
	bestAt := now
	found := false

	for _, line := range lines {
		if at, ok := combinedModifiedAt(line); ok && (!found || at.After(bestAt)) {
			bestAt = at
			found = true
		}
	}

	if !found && head != nil {
		if fecha, ok := parseDateField(head["fecha"]); ok {
			bestAt = time.Date(
				fecha.Year(), fecha.Month(), fecha.Day(),
				now.Hour(), now.Minute(), now.Second(), 0, time.UTC,
			)
			found = true
		}
	}

	if !found {
		bestAt = now
	}

	fecha := time.Date(bestAt.Year(), bestAt.Month(), bestAt.Day(), 0, 0, 0, 0, time.UTC)
	hora := time.Date(0, 1, 1, bestAt.Hour(), bestAt.Minute(), bestAt.Second(), 0, time.UTC)
	return fecha, hora
}

func combinedModifiedAt(row map[string]interface{}) (time.Time, bool) {
	fecha, ok := parseDateField(row["fecha_modificacion"])
	if !ok {
		fecha, ok = parseDateField(row["fecha"])
	}
	if !ok {
		return time.Time{}, false
	}

	clock := time.Date(0, 1, 1, 0, 0, 0, 0, time.UTC)
	if parsedHora, horaOK := parseTimeField(row["hora_modificacion"]); horaOK {
		clock = parsedHora
	}

	return time.Date(
		fecha.Year(), fecha.Month(), fecha.Day(),
		clock.Hour(), clock.Minute(), clock.Second(), 0, time.UTC,
	), true
}

func sumPedidoPaginaLines(lines []map[string]interface{}) float64 {
	total := 0.0
	for _, line := range lines {
		cantidad := floatField(line, "prod_cant", "ped_cantidad", "cantidad")
		precio := floatField(line, "prod_precio", "ped_precio_unitario", "precio")
		total += cantidad * precio
	}
	return math.Round(total*100) / 100
}

func normalizeProdID(raw string) string {
	text := strings.TrimSpace(raw)
	if text == "" {
		return "00000001"
	}
	if len(text) > 8 && strings.HasPrefix(text, "0") {
		return text[len(text)-8:]
	}
	if len(text) < 8 && isAllDigits(text) {
		return fmt.Sprintf("%08s", text)
	}
	return text
}

func isAllDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func stringField(row map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := row[key]; ok && value != nil {
			text := strings.TrimSpace(fmt.Sprint(value))
			if text != "" && text != "<nil>" {
				return text
			}
		}
	}
	return ""
}

func floatField(row map[string]interface{}, keys ...string) float64 {
	for _, key := range keys {
		value, ok := row[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case float64:
			return typed
		case float32:
			return float64(typed)
		case int64:
			return float64(typed)
		case int32:
			return float64(typed)
		case int:
			return float64(typed)
		default:
			text := strings.TrimSpace(fmt.Sprint(typed))
			if text == "" {
				continue
			}
			var parsed float64
			if _, err := fmt.Sscan(text, &parsed); err == nil {
				return parsed
			}
		}
	}
	return 0
}

func truncateRunes(value string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}

func parseDateField(raw interface{}) (time.Time, bool) {
	switch value := raw.(type) {
	case time.Time:
		return value.Truncate(24 * time.Hour), true
	case string:
		text := strings.TrimSpace(value)
		if text == "" {
			return time.Time{}, false
		}
		layouts := []string{time.RFC3339Nano, "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02"}
		for _, layout := range layouts {
			if parsed, err := time.Parse(layout, text); err == nil {
				return parsed.Truncate(24 * time.Hour), true
			}
		}
	}
	return time.Time{}, false
}

func parseTimeField(raw interface{}) (time.Time, bool) {
	switch value := raw.(type) {
	case time.Time:
		return time.Date(0, 1, 1, value.Hour(), value.Minute(), value.Second(), 0, time.UTC), true
	case string:
		text := strings.TrimSpace(value)
		if text == "" {
			return time.Time{}, false
		}
		layouts := []string{"15:04:05.999999", "15:04:05", time.RFC3339Nano, "2006-01-02T15:04:05"}
		for _, layout := range layouts {
			if parsed, err := time.Parse(layout, text); err == nil {
				return time.Date(0, 1, 1, parsed.Hour(), parsed.Minute(), parsed.Second(), 0, time.UTC), true
			}
		}
	}
	return time.Time{}, false
}

func normalizeCuitDigits(raw string) string {
	var digits strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	return digits.String()
}
