package supabase

import (
	"strings"
	"testing"
	"time"
)

func TestLocalClockWhereClauseConvierteAHoraArgentina(t *testing.T) {
	// 2026-11-03 15:00 UTC = 12:00 en Argentina.
	since := time.Date(2026, 11, 3, 15, 0, 0, 0, time.UTC)
	where, arg := LocalClockWhereClause(since, 0, true, true)
	if !strings.Contains(where, "hora_modificacion") || !strings.Contains(where, "$1::timestamp") {
		t.Fatalf("where inesperado: %s", where)
	}
	if arg != "2026-11-03 12:00:00" {
		t.Fatalf("arg: got %q want 2026-11-03 12:00:00", arg)
	}
}

func TestLocalClockWhereClauseLookback(t *testing.T) {
	since := time.Date(2026, 11, 3, 15, 0, 0, 0, time.UTC)
	_, arg := LocalClockWhereClause(since, 10*time.Minute, false, false)
	if arg != "2026-11-03 11:50:00" {
		t.Fatalf("arg: got %q", arg)
	}
}

func TestLocalClockWhereClauseFechaSolaUsaDiaArgentino(t *testing.T) {
	// 2026-11-04 01:30 UTC sigue siendo 03/11 en Argentina (22:30).
	since := time.Date(2026, 11, 4, 1, 30, 0, 0, time.UTC)
	where, arg := LocalClockWhereClause(since, 0, true, false)
	if where != "fecha_modificacion >= $1::date" || arg != "2026-11-03" {
		t.Fatalf("got %s / %s", where, arg)
	}
}

func TestLocalClockWhereClauseCambioARTNoSeSaltea(t *testing.T) {
	// Checkpoint 12:00 ART; una fila grabada 12:01 ART debe pasar el filtro.
	since := time.Date(2026, 11, 3, 15, 0, 0, 0, time.UTC)
	_, arg := LocalClockWhereClause(since, 0, true, true)
	row := "2026-11-03 12:01:00"
	if !(row > arg) {
		t.Fatalf("fila %s debería ser > checkpoint %s", row, arg)
	}
}
