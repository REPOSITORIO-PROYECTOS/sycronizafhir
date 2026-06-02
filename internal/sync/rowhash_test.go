package sync

import (
	"testing"
)

func TestRowHashStableAcrossColumnOrder(t *testing.T) {
	rowA := map[string]interface{}{
		"clien_id": int16(10),
		"nombre":   "Ana",
	}
	rowB := map[string]interface{}{
		"nombre":   "Ana",
		"clien_id": int16(10),
	}

	hashA, err := RowHash(rowA, []string{"clien_id", "nombre"})
	if err != nil {
		t.Fatalf("hash A: %v", err)
	}
	hashB, err := RowHash(rowB, []string{"nombre", "clien_id"})
	if err != nil {
		t.Fatalf("hash B: %v", err)
	}
	if hashA != hashB {
		t.Fatalf("expected stable hash, got %s vs %s", hashA, hashB)
	}
}

func TestPKKeyComposite(t *testing.T) {
	row := map[string]interface{}{
		"a": 1,
		"b": "x",
	}
	key, err := PKKey(row, []string{"a", "b"})
	if err != nil {
		t.Fatalf("pk key: %v", err)
	}
	if key != "a=1|b=x" {
		t.Fatalf("unexpected key: %s", key)
	}
}
