package sync

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

var pedidoPaginaDetailTables = []string{
	"pedido_pagina_detail",
	"pedido_pagina_d",
	"pedido_pagina_detalle",
}

const pedidoPaginaHeadTable = "pedido_pagina"

func pedidoPaginaDetailTable(columns map[string]bool) string {
	for _, name := range pedidoPaginaDetailTables {
		if columns[name] {
			return name
		}
	}
	return ""
}

func isPedidoPaginaDetailTable(tableName string) bool {
	for _, name := range pedidoPaginaDetailTables {
		if tableName == name {
			return true
		}
	}
	return false
}

// Letras de pedido_pagina.estado que Gestiona puede mandar a Supabase.
// N = nuevo (alta nube). S = comprometido / Traer. Cualquier otra se ignora
// para no crear valores que Shops no entiende.
var pedidoPaginaEstadosPermitidos = map[string]bool{
	"N": true,
	"S": true,
}

func pedidoPaginaEstadoPermitido(estado string) bool {
	return pedidoPaginaEstadosPermitidos[estado]
}

// skipPedidoPaginaGenericOutbound: el upsert genérico no debe tocar cabeza ni
// detalle. Cabeza: solo PATCH de estado (worker dedicado). Detalle: nube.
func skipPedidoPaginaGenericOutbound(tableName string) bool {
	return tableName == pedidoPaginaHeadTable || isPedidoPaginaDetailTable(tableName)
}

func pedidoPaginaHeadExists(row map[string]interface{}) bool {
	if row == nil {
		return false
	}
	for _, key := range []string{"pedido_id", "email", "cuit"} {
		if strings.TrimSpace(fmt.Sprint(row[key])) != "" {
			return true
		}
	}
	return false
}

func normalizePedidoPaginaEstado(raw interface{}) string {
	if raw == nil {
		return ""
	}
	var text string
	switch typed := raw.(type) {
	case string:
		text = typed
	case []byte:
		text = string(typed)
	default:
		text = strings.TrimSpace(fmt.Sprintf("%v", typed))
	}
	text = strings.ToUpper(strings.TrimSpace(text))
	if text == "" || text == "<NIL>" {
		return ""
	}
	letter := text[:1]
	if !pedidoPaginaEstadoPermitido(letter) {
		return ""
	}
	return letter
}

// pedidoPaginaClienIDValue lee clien_id de la cabeza (nube o local).
// Cero / vacío / nil → ausente (hay que hidratar por CUIT en SERVIDOR).
func pedidoPaginaClienIDValue(raw interface{}) (int16, bool) {
	if raw == nil {
		return 0, false
	}
	switch typed := raw.(type) {
	case int16:
		if typed == 0 {
			return 0, false
		}
		return typed, true
	case int32:
		if typed == 0 {
			return 0, false
		}
		return int16(typed), true
	case int:
		if typed == 0 {
			return 0, false
		}
		return int16(typed), true
	case int64:
		if typed == 0 {
			return 0, false
		}
		return int16(typed), true
	case float64:
		if typed == 0 {
			return 0, false
		}
		return int16(typed), true
	case string:
		text := strings.TrimSpace(typed)
		if text == "" || strings.EqualFold(text, "<nil>") {
			return 0, false
		}
		n, err := strconv.ParseInt(text, 10, 16)
		if err != nil || n == 0 {
			return 0, false
		}
		return int16(n), true
	case []byte:
		return pedidoPaginaClienIDValue(string(typed))
	default:
		text := strings.TrimSpace(fmt.Sprint(raw))
		if text == "" || strings.EqualFold(text, "<nil>") {
			return 0, false
		}
		n, err := strconv.ParseInt(text, 10, 16)
		if err != nil || n == 0 {
			return 0, false
		}
		return int16(n), true
	}
}

func digitsOnlyCuit(raw interface{}) string {
	if raw == nil {
		return ""
	}
	text := strings.TrimSpace(fmt.Sprint(raw))
	if text == "" || strings.EqualFold(text, "<nil>") {
		return ""
	}
	var b strings.Builder
	for _, r := range text {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// hydratePedidoPaginaHeadClienID: si la nube no trae clien_id, lo resuelve en
// clientes locales (Gestiona) por CUIT. No pisa un clien_id ya presente.
func hydratePedidoPaginaHeadClienID(
	head map[string]interface{},
	lookup func(cuitDigits string) (int16, bool, error),
) (filled bool, err error) {
	if head == nil || lookup == nil {
		return false, nil
	}
	if _, ok := pedidoPaginaClienIDValue(head["clien_id"]); ok {
		return false, nil
	}
	cuit := digitsOnlyCuit(head["cuit"])
	if cuit == "" {
		return false, nil
	}
	id, found, lookupErr := lookup(cuit)
	if lookupErr != nil {
		return false, lookupErr
	}
	if !found || id == 0 {
		return false, nil
	}
	head["clien_id"] = id
	return true, nil
}

// restrictPedidoPaginaOutboundRows deja solo PK + estado. Defensa si un retry
// de cola todavía trae la fila completa.
func restrictPedidoPaginaOutboundRows(rows []map[string]interface{}) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		pedidoID, ok := row["pedido_id"]
		if !ok || pedidoID == nil {
			continue
		}
		estado := normalizePedidoPaginaEstado(row["estado"])
		if estado == "" {
			continue
		}
		out = append(out, map[string]interface{}{
			"pedido_id": pedidoID,
			"estado":    estado,
		})
	}
	return out
}

// mergePedidoPaginaEstadoOutboundPatch: ERP (local) manda estado; nube (remote)
// debe existir. No INSERT. No toca email/líneas.
func mergePedidoPaginaEstadoOutboundPatch(localRow, remoteRow map[string]interface{}) map[string]interface{} {
	if localRow == nil || remoteRow == nil {
		return nil
	}
	localEstado := normalizePedidoPaginaEstado(localRow["estado"])
	if localEstado == "" {
		return nil
	}
	remoteEstado := normalizePedidoPaginaEstado(remoteRow["estado"])
	if localEstado == remoteEstado {
		return nil
	}
	return map[string]interface{}{"estado": localEstado}
}

type pedidoPaginaEstadoPatcher interface {
	PatchExistingColumns(ctx context.Context, schemaName, tableName string, pkColumns []string, row map[string]interface{}) (bool, error)
}

func patchPedidoPaginaEstados(
	ctx context.Context,
	patcher pedidoPaginaEstadoPatcher,
	localRows, remoteRows []map[string]interface{},
) (int, error) {
	if patcher == nil {
		return 0, nil
	}
	remoteByPK := mapRowsByPK(remoteRows, []string{"pedido_id"})
	applied := 0
	for _, local := range localRows {
		key, err := PKKey(local, []string{"pedido_id"})
		if err != nil {
			continue
		}
		remote, ok := remoteByPK[key]
		if !ok {
			continue
		}
		patch := mergePedidoPaginaEstadoOutboundPatch(local, remote)
		if len(patch) == 0 {
			continue
		}
		row := map[string]interface{}{
			"pedido_id": local["pedido_id"],
			"estado":    patch["estado"],
		}
		updated, patchErr := patcher.PatchExistingColumns(ctx, "public", pedidoPaginaHeadTable, []string{"pedido_id"}, row)
		if patchErr != nil {
			return applied, patchErr
		}
		if updated {
			applied++
		}
	}
	return applied, nil
}
