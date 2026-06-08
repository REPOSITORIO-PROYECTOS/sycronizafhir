package supabase

import (
	"reflect"
	"testing"
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
