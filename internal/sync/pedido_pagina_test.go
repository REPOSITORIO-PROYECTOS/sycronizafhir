package sync

import (
	"context"
	"testing"
)

func TestSkipPedidoPaginaGenericOutbound(t *testing.T) {
	if !skipPedidoPaginaGenericOutbound("pedido_pagina") {
		t.Fatal("cabeza debe saltearse en outbound genérico")
	}
	if !skipPedidoPaginaGenericOutbound("pedido_pagina_detail") {
		t.Fatal("detalle no debe upsertarse")
	}
	if skipPedidoPaginaGenericOutbound("pedidos") {
		t.Fatal("pedidos ERP sigue por outbound genérico")
	}
}

func TestRestrictPedidoPaginaOutboundRowsSoloEstado(t *testing.T) {
	rows := []map[string]interface{}{
		{
			"pedido_id":   900150,
			"email":       "a@b.com",
			"razonsocial": "Cliente",
			"cuit":        "20123456789",
			"estado":      "S",
			"comentario":  "no pisar",
		},
		{
			"pedido_id": 900151,
			"email":     "c@d.com",
			"estado":    "n",
		},
		{
			"pedido_id": 900152,
			"email":     "sin-estado@x.com",
		},
	}
	got := restrictPedidoPaginaOutboundRows(rows)
	if len(got) != 2 {
		t.Fatalf("len=%d want 2 (sin estado se descarta)", len(got))
	}
	if _, ok := got[0]["email"]; ok {
		t.Fatal("email no debe viajar en outbound")
	}
	if got[0]["estado"] != "S" || got[1]["estado"] != "N" {
		t.Fatalf("estados: %+v", got)
	}
}

func TestMergePedidoPaginaEstadoOutboundPatch(t *testing.T) {
	patch := mergePedidoPaginaEstadoOutboundPatch(
		map[string]interface{}{"pedido_id": 1, "estado": "S", "email": "erp@x.com"},
		map[string]interface{}{"pedido_id": 1, "estado": "N", "email": "nube@x.com"},
	)
	if patch["estado"] != "S" {
		t.Fatalf("ERP debe mandar S, got %v", patch)
	}
	if _, ok := patch["email"]; ok {
		t.Fatal("email no va en el patch")
	}

	same := mergePedidoPaginaEstadoOutboundPatch(
		map[string]interface{}{"estado": "S"},
		map[string]interface{}{"estado": "s"},
	)
	if same != nil {
		t.Fatal("mismo estado no parchea")
	}

	noRemote := mergePedidoPaginaEstadoOutboundPatch(
		map[string]interface{}{"estado": "S"},
		nil,
	)
	if noRemote != nil {
		t.Fatal("sin fila remota no INSERT")
	}
}

type stubPedidoPaginaPatcher struct {
	rows []map[string]interface{}
}

func (s *stubPedidoPaginaPatcher) PatchExistingColumns(
	_ context.Context,
	_, _ string,
	_ []string,
	row map[string]interface{},
) (bool, error) {
	s.rows = append(s.rows, row)
	return true, nil
}

func TestPatchPedidoPaginaEstadosNoInsertMissingRemote(t *testing.T) {
	stub := &stubPedidoPaginaPatcher{}
	local := []map[string]interface{}{
		{"pedido_id": 1, "estado": "S", "email": "erp@x.com"},
		{"pedido_id": 2, "estado": "S", "email": "solo-erp@x.com"},
	}
	remote := []map[string]interface{}{
		{"pedido_id": 1, "estado": "N", "email": "nube@x.com"},
	}
	n, err := patchPedidoPaginaEstados(context.Background(), stub, local, remote)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("applied=%d want 1 (id 2 no existe en nube)", n)
	}
	if len(stub.rows) != 1 {
		t.Fatalf("patches=%d want 1", len(stub.rows))
	}
	if _, ok := stub.rows[0]["email"]; ok {
		t.Fatal("PATCH no debe incluir email")
	}
	if stub.rows[0]["estado"] != "S" {
		t.Fatalf("estado=%v want S", stub.rows[0]["estado"])
	}
}

func TestNormalizePedidoPaginaEstadoSoloNS(t *testing.T) {
	if got := normalizePedidoPaginaEstado("S"); got != "S" {
		t.Fatalf("S: %q", got)
	}
	if got := normalizePedidoPaginaEstado("n"); got != "N" {
		t.Fatalf("n: %q", got)
	}
	if got := normalizePedidoPaginaEstado("V"); got != "" {
		t.Fatalf("V no es estado de pagina, got %q", got)
	}
	if got := normalizePedidoPaginaEstado("P"); got != "" {
		t.Fatalf("P no es estado de pagina, got %q", got)
	}
}
