package db

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestModifiedAtWhereClauseWithHora(t *testing.T) {
	meta := TableModifiedAtMeta{FechaIsDate: true, HasHoraModificacion: true}
	where := ModifiedAtWhereClause(meta)
	if where != "(fecha_modificacion::timestamp + COALESCE(hora_modificacion, TIME '00:00:00')) > $1" {
		t.Fatalf("unexpected where: %s", where)
	}
}

func TestModifiedAtWhereClauseDateOnly(t *testing.T) {
	meta := TableModifiedAtMeta{FechaIsDate: true, HasHoraModificacion: false}
	where := ModifiedAtWhereClause(meta)
	if where != "fecha_modificacion > $1::date" {
		t.Fatalf("unexpected where: %s", where)
	}
}

func TestRowModifiedAtCombinesFechaAndHora(t *testing.T) {
	meta := TableModifiedAtMeta{FechaIsDate: true, HasHoraModificacion: true}
	row := map[string]interface{}{
		"fecha_modificacion": time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC),
		"hora_modificacion": pgtype.Time{
			Microseconds: (17*3600 + 35*60 + 21) * 1_000_000,
			Valid:        true,
		},
	}

	modifiedAt, ok := RowModifiedAt(row, meta)
	if !ok {
		t.Fatal("expected modifiedAt")
	}
	if modifiedAt.Year() != 2026 || modifiedAt.Month() != time.July || modifiedAt.Day() != 2 {
		t.Fatalf("unexpected date: %s", modifiedAt)
	}
	if modifiedAt.Hour() != 17 || modifiedAt.Minute() != 35 || modifiedAt.Second() != 21 {
		t.Fatalf("unexpected time: %s", modifiedAt)
	}
}

func TestMaxRowModifiedAtPicksLatestSameDay(t *testing.T) {
	meta := TableModifiedAtMeta{FechaIsDate: true, HasHoraModificacion: true}
	rows := []map[string]interface{}{
		{
			"fecha_modificacion": time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC),
			"hora_modificacion": pgtype.Time{
				Microseconds: 10 * 3600 * 1_000_000,
				Valid:        true,
			},
		},
		{
			"fecha_modificacion": time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC),
			"hora_modificacion": pgtype.Time{
				Microseconds: (16*3600 + 4*60 + 55) * 1_000_000,
				Valid:        true,
			},
		},
	}

	maxAt, ok := MaxRowModifiedAt(rows, meta)
	if !ok {
		t.Fatal("expected maxAt")
	}
	if maxAt.Hour() != 16 || maxAt.Minute() != 4 || maxAt.Second() != 55 {
		t.Fatalf("expected latest row time, got %s", maxAt)
	}
}

func TestRowModifiedAtDateOnlyUsesMidnightUTC(t *testing.T) {
	meta := TableModifiedAtMeta{FechaIsDate: true, HasHoraModificacion: false}
	row := map[string]interface{}{
		"fecha_modificacion": time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC),
	}

	modifiedAt, ok := RowModifiedAt(row, meta)
	if !ok {
		t.Fatal("expected modifiedAt")
	}
	if !modifiedAt.Equal(time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected modifiedAt: %s", modifiedAt)
	}
}
