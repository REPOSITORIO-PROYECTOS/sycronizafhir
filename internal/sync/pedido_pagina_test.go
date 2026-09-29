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

func TestPedidoPaginaClienIDValue(t *testing.T) {
	if _, ok := pedidoPaginaClienIDValue(nil); ok {
		t.Fatal("nil debe ausente")
	}
	if _, ok := pedidoPaginaClienIDValue(int16(0)); ok {
		t.Fatal("0 debe ausente")
	}
	if _, ok := pedidoPaginaClienIDValue(""); ok {
		t.Fatal("string vacío debe ausente")
	}
	got, ok := pedidoPaginaClienIDValue(int16(142))
	if !ok || got != 142 {
		t.Fatalf("int16: got=%d ok=%v", got, ok)
	}
	got, ok = pedidoPaginaClienIDValue(float64(1358))
	if !ok || got != 1358 {
		t.Fatalf("float64: got=%d ok=%v", got, ok)
	}
	got, ok = pedidoPaginaClienIDValue("302")
	if !ok || got != 302 {
		t.Fatalf("string: got=%d ok=%v", got, ok)
	}
}

func TestDigitsOnlyCuit(t *testing.T) {
	if got := digitsOnlyCuit("20-12345678-9"); got != "20123456789" {
		t.Fatalf("got %q", got)
	}
	if got := digitsOnlyCuit(nil); got != "" {
		t.Fatalf("nil: %q", got)
	}
}

func TestHydratePedidoPaginaHeadClienID(t *testing.T) {
	t.Run("ya presente no pisa", func(t *testing.T) {
		head := map[string]interface{}{"clien_id": int16(10), "cuit": "20123456789"}
		filled, err := hydratePedidoPaginaHeadClienID(head, func(string) (int16, bool, error) {
			t.Fatal("lookup no debe llamarse")
			return 0, false, nil
		})
		if err != nil || filled {
			t.Fatalf("filled=%v err=%v", filled, err)
		}
		if head["clien_id"].(int16) != 10 {
			t.Fatalf("clien_id=%v", head["clien_id"])
		}
	})

	t.Run("resuelve por CUIT", func(t *testing.T) {
		head := map[string]interface{}{"cuit": "20-36253099-8", "estado": "N"}
		filled, err := hydratePedidoPaginaHeadClienID(head, func(cuit string) (int16, bool, error) {
			if cuit != "20362530998" {
				t.Fatalf("cuit digits=%q", cuit)
			}
			return 999, true, nil
		})
		if err != nil || !filled {
			t.Fatalf("filled=%v err=%v", filled, err)
		}
		if head["clien_id"].(int16) != 999 {
			t.Fatalf("clien_id=%v", head["clien_id"])
		}
	})

	t.Run("sin match deja null", func(t *testing.T) {
		head := map[string]interface{}{"cuit": "20111111112"}
		filled, err := hydratePedidoPaginaHeadClienID(head, func(string) (int16, bool, error) {
			return 0, false, nil
		})
		if err != nil || filled {
			t.Fatalf("filled=%v err=%v", filled, err)
		}
		if _, ok := head["clien_id"]; ok {
			t.Fatalf("no debe setear clien_id: %v", head["clien_id"])
		}
	})

	t.Run("sin CUIT no lookup", func(t *testing.T) {
		head := map[string]interface{}{"estado": "N"}
		filled, err := hydratePedidoPaginaHeadClienID(head, func(string) (int16, bool, error) {
			t.Fatal("lookup no debe llamarse")
			return 0, false, nil
		})
		if err != nil || filled {
			t.Fatalf("filled=%v err=%v", filled, err)
		}
	})
}
