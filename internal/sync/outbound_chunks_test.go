package sync

import (
	"math/big"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"sycronizafhir/internal/db"
)

var pxpMeta = db.TableModifiedAtMeta{FechaIsDate: true, HasHoraModificacion: true}

func pxpRow(prod, provi string, activo bool, hora string) map[string]interface{} {
	return map[string]interface{}{
		"prod_id": prod, "provi_id": provi, "activo": activo,
		"fecha_modificacion": time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC),
		"hora_modificacion":  hora,
	}
}

func TestChunkRowsParteEnTramos(t *testing.T) {
	rows := make([]map[string]interface{}, 5)
	for i := range rows {
		rows[i] = map[string]interface{}{}
	}
	got := chunkRows(rows, 2)
	if len(got) != 3 || len(got[0]) != 2 || len(got[2]) != 1 {
		t.Fatalf("tramos inesperados: %d", len(got))
	}
	if len(chunkRows(nil, 2)) != 0 {
		t.Fatalf("sin filas => sin tramos")
	}
}

func TestChunkCheckpointNoAvanzaSiSiguienteTieneMismaMarca(t *testing.T) {
	a := []map[string]interface{}{pxpRow("1", "1", true, "14:25:40")}
	b := []map[string]interface{}{pxpRow("2", "1", true, "14:25:40")}
	if _, ok := chunkCheckpoint(a, b, pxpMeta); ok {
		t.Fatalf("no debe avanzar: perderia filas con la misma marca")
	}
	c := []map[string]interface{}{pxpRow("3", "1", true, "14:25:41")}
	at, ok := chunkCheckpoint(a, c, pxpMeta)
	if !ok || at.Format("15:04:05") != "14:25:40" {
		t.Fatalf("debe avanzar a 14:25:40, got %v %v", at, ok)
	}
	if _, ok := chunkCheckpoint(c, nil, pxpMeta); !ok {
		t.Fatalf("ultimo tramo avanza")
	}
}

func TestSortRowsByModifiedAt(t *testing.T) {
	rows := []map[string]interface{}{pxpRow("b", "1", true, "10:00:02"), {"prod_id": "x"}, pxpRow("a", "1", true, "10:00:01")}
	sortRowsByModifiedAt(rows, pxpMeta)
	if rows[0]["prod_id"] != "a" || rows[2]["prod_id"] != "x" {
		t.Fatalf("orden inesperado: %v %v %v", rows[0]["prod_id"], rows[1]["prod_id"], rows[2]["prod_id"])
	}
}

func TestFilterUnchangedRowsSalteaIguales(t *testing.T) {
	local := []map[string]interface{}{
		pxpRow("1", "1", true, "14:25:40"),
		pxpRow("2", "1", true, "14:25:40"),
		pxpRow("3", "1", true, "14:25:40"),
	}
	cloudTime := pgtype.Time{Microseconds: (14*3600 + 25*60 + 40) * 1_000_000, Valid: true}
	remote := []map[string]interface{}{
		{"prod_id": "1", "provi_id": "1", "activo": true, "fecha_modificacion": time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC), "hora_modificacion": cloudTime},
		{"prod_id": "2", "provi_id": "1", "activo": false, "fecha_modificacion": time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC), "hora_modificacion": "14:25:40"},
	}
	got := filterUnchangedRows(local, remote, []string{"prod_id", "provi_id"})
	if len(got) != 2 || got[0]["prod_id"] != "2" || got[1]["prod_id"] != "3" {
		t.Fatalf("esperaba enviar 2 (distinta) y 3 (falta en nube), got %v", got)
	}
}

func TestComparableValueNumeric(t *testing.T) {
	n := pgtype.Numeric{Int: big.NewInt(1000), Exp: -2, Valid: true}
	if comparableValue(n) != comparableValue(float64(10)) {
		t.Fatalf("numeric 10.00 debe igualar 10")
	}
}

func TestQueuedTableName(t *testing.T) {
	if queuedTableName(`{"table_name":"productos_x_provincias","rows":[]}`) != "productos_x_provincias" {
		t.Fatalf("nombre de tabla")
	}
	if queuedTableName("no-json") != "" {
		t.Fatalf("json invalido => vacio")
	}
}
