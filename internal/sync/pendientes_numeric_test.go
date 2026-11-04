package sync

import (
	"math/big"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"sycronizafhir/internal/db"
)

func num(v int64, exp int32) pgtype.Numeric {
	return pgtype.Numeric{Int: big.NewInt(v), Exp: exp, Valid: true}
}

func TestPendienteCantidadNumeric(t *testing.T) {
	got, ok := pendienteCantidad(num(1000, -2))
	if !ok || got != 10 {
		t.Fatalf("want 10 ok, got %v %v", got, ok)
	}
	n := num(0, -2)
	if got, ok := pendienteCantidad(&n); !ok || got != 0 {
		t.Fatalf("want 0 ok, got %v %v", got, ok)
	}
	if _, ok := pendienteCantidad(pgtype.Numeric{}); ok {
		t.Fatalf("invalid numeric must not be ok")
	}
}

func TestMergePendienteInboundNumericPatchYBaja(t *testing.T) {
	meta := db.TableModifiedAtMeta{FechaIsDate: true, HasHoraModificacion: true}
	local := map[string]interface{}{
		"cantidad":           num(1400, -2),
		"fecha_modificacion": time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC),
		"hora_modificacion":  "13:48:29",
	}
	remote := map[string]interface{}{
		"cantidad":           num(1000, -2),
		"fecha_modificacion": time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC),
		"hora_modificacion":  "21:32:21",
	}
	d := mergePendienteInbound(local, remote, meta)
	if d.Delete || d.Patch["cantidad"] != float64(10) {
		t.Fatalf("want patch 10, got %+v", d)
	}
	remote["cantidad"] = num(0, -2)
	if d := mergePendienteInbound(local, remote, meta); !d.Delete {
		t.Fatalf("want delete, got %+v", d)
	}
}
