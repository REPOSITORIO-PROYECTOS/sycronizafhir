package sync

import (
	"fmt"
	"strings"
)

var pedidoPaginaDetailTables = []string{
	"pedido_pagina_detail",
	"pedido_pagina_d",
	"pedido_pagina_detalle",
}

func pedidoPaginaDetailTable(columns map[string]bool) string {
	for _, name := range pedidoPaginaDetailTables {
		if columns[name] {
			return name
		}
	}
	return ""
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
