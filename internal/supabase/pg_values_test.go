package supabase

import (
	"reflect"
	"testing"
)

func TestNormalizeParamValue_integerArray(t *testing.T) {
	in := []interface{}{301, 412, int32(462)}
	got := normalizeParamValue(in)
	slice, ok := got.([]int32)
	if !ok {
		t.Fatalf("expected []int32, got %T", got)
	}
	want := []int32{301, 412, 462}
	if !reflect.DeepEqual(slice, want) {
		t.Fatalf("got %v want %v", slice, want)
	}
}

func TestNormalizeParamValue_textArray(t *testing.T) {
	in := []interface{}{"00200600", "00202440"}
	got := normalizeParamValue(in)
	slice, ok := got.([]string)
	if !ok {
		t.Fatalf("expected []string, got %T", got)
	}
	if !reflect.DeepEqual(slice, []string{"00200600", "00202440"}) {
		t.Fatalf("unexpected %v", slice)
	}
}

func TestNormalizeParamValue_emptyArray(t *testing.T) {
	got := normalizeParamValue([]interface{}{})
	slice, ok := got.([]string)
	if !ok || len(slice) != 0 {
		t.Fatalf("expected empty []string, got %T %v", got, got)
	}
}

func TestNormalizeParamValue_scalarPassthrough(t *testing.T) {
	if normalizeParamValue("hola") != "hola" {
		t.Fatal("scalar should pass through")
	}
	if normalizeParamValue(nil) != nil {
		t.Fatal("nil should pass through")
	}
}
