package sync

import (
	"os"
	"path/filepath"
	"testing"
)

// Guarda Misan: el worker que INSERTABA pedido_pagina → pedidos ERP no debe volver.
func TestNoPedidosTiendaInboundSource(t *testing.T) {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	banned := []string{
		"pedidos_tienda_inbound.go",
		"pedidos_tienda_map.go",
		"pedidos_tienda_map_test.go",
	}
	for _, name := range banned {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			t.Fatalf("archivo prohibido presente (reactivaría INSERT a pedidos ERP): %s", path)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat %s: %v", path, err)
		}
	}
}
