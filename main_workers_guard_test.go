package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMainDoesNotRegisterPedidosTiendaInbound(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)
	forbidden := []string{
		"PedidosTiendaInbound",
		"pedidos_tienda_inbound",
		"NewPedidosTiendaInboundWorker",
		"InboundPedidosTiendaInterval",
	}
	for _, needle := range forbidden {
		if strings.Contains(src, needle) {
			t.Fatalf("main.go no debe registrar el worker tienda→pedidos; encontró %q", needle)
		}
	}
	required := []string{
		`"pedido_pagina_inbound"`,
		`"pedidos_inbound"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("main.go debería seguir registrando %s", needle)
		}
	}
}
