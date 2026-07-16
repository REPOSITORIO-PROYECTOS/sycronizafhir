package supabase

import (
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestNormalizeParamValue_integerArray(t *testing.T) {
	in := []interface{}{301, 412, int32(462)}
	got := NormalizeParamValue(in)
	slice, ok := got.([]int32)
	if !ok {
		t.Fatalf("expected []int32, got %T", got)
	}
	want := []int32{301, 412, 462}
	if !reflect.DeepEqual(slice, want) {
		t.Fatalf("got %v want %v", slice, want)
	}
}

func TestNormalizeParamValue_reproducedIntegerArray(t *testing.T) {
	in := []interface{}{
		403, 425, 1430, 10, 314, 573, 742, 901, 1130, 1327, 138, 477, 760, 207, 227,
	}
	got := NormalizeParamValue(in)
	slice, ok := got.([]int32)
	if !ok {
		t.Fatalf("expected []int32, got %T", got)
	}
	if len(slice) != len(in) {
		t.Fatalf("len mismatch: got %d want %d", len(slice), len(in))
	}
}

func TestNormalizeParamValue_typedIntSlice(t *testing.T) {
	in := []int{403, 425, 1430}
	got := NormalizeParamValue(in)
	slice, ok := got.([]int32)
	if !ok {
		t.Fatalf("expected []int32, got %T", got)
	}
	if len(slice) != 3 {
		t.Fatalf("unexpected len %d", len(slice))
	}
}

func TestNormalizeParamValue_jsonFloatArray(t *testing.T) {
	in := []interface{}{403.0, 425.0, 1430.0}
	got := NormalizeParamValue(in)
	slice, ok := got.([]int32)
	if !ok {
		t.Fatalf("expected []int32, got %T", got)
	}
	if len(slice) != 3 {
		t.Fatalf("unexpected len %d", len(slice))
	}
}

func TestNormalizeParamValue_postgresArrayLiteral(t *testing.T) {
	got := NormalizeParamValue("{403,425,1430}")
	slice, ok := got.([]int32)
	if !ok {
		t.Fatalf("expected []int32, got %T", got)
	}
	if len(slice) != 3 {
		t.Fatalf("unexpected %v", slice)
	}
}

func TestNormalizeParamValue_textArray(t *testing.T) {
	in := []interface{}{"00200600", "00202440"}
	got := NormalizeParamValue(in)
	slice, ok := got.([]string)
	if !ok {
		t.Fatalf("expected []string, got %T", got)
	}
	if !reflect.DeepEqual(slice, []string{"00200600", "00202440"}) {
		t.Fatalf("unexpected %v", slice)
	}
}

func TestNormalizeParamValue_emptyArray(t *testing.T) {
	got := NormalizeParamValue([]interface{}{})
	slice, ok := got.([]int32)
	if !ok || len(slice) != 0 {
		t.Fatalf("expected empty []int32, got %T %v", got, got)
	}
}

func TestNormalizeParamValue_scalarPassthrough(t *testing.T) {
	if NormalizeParamValue("hola") != "hola" {
		t.Fatal("scalar should pass through")
	}
	if NormalizeParamValue(nil) != nil {
		t.Fatal("nil should pass through")
	}
}

func TestNormalizeParamValue_pgtypeTime(t *testing.T) {
	usecs := int64((17*3600 + 35*60 + 21) * 1_000_000)
	got := NormalizeParamValue(pgtype.Time{Valid: true, Microseconds: usecs})
	if got != "17:35:21" {
		t.Fatalf("got %v want 17:35:21", got)
	}
}

func TestNormalizeParamValue_decodedTimeMap(t *testing.T) {
	got := NormalizeParamValue(map[string]interface{}{
		"Microseconds": float64(16*3600*1_000_000 + 4*60*1_000_000 + 55*1_000_000),
		"Valid":        true,
	})
	if got != "16:04:55" {
		t.Fatalf("got %v want 16:04:55", got)
	}
}

func TestNormalizeParamValue_decodedTimeMapFloatScientific(t *testing.T) {
	got := NormalizeParamValue(map[string]interface{}{
		"Microseconds": 2.8791622e+10,
		"Valid":        true,
	})
	if got != "07:59:51" {
		t.Fatalf("got %v want 07:59:51", got)
	}
}

func TestNormalizeParamValue_pgtypeInterval(t *testing.T) {
	got := NormalizeParamValue(pgtype.Interval{Valid: true, Days: 2, Microseconds: 3_600_000_000})
	if got != "01:00:00" {
		t.Fatalf("got %v", got)
	}
}

func TestNormalizeRowMap(t *testing.T) {
	row := map[string]interface{}{
		"id":   1,
		"perm": []interface{}{403, 425},
	}
	NormalizeRowMap(row)
	if _, ok := row["perm"].([]int32); !ok {
		t.Fatalf("perm should be []int32, got %T", row["perm"])
	}
}
