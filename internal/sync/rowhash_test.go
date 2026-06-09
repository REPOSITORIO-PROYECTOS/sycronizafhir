package sync

import (
	"testing"
	"time"
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

func TestRowHashIgnoresCharPadding(t *testing.T) {
	local := map[string]interface{}{
		"prod_id": "ABC123  ",
		"stock":   int32(10),
	}
	remote := map[string]interface{}{
		"prod_id": "ABC123",
		"stock":   int16(10),
	}

	localHash, err := RowHash(local, []string{"prod_id", "stock"})
	if err != nil {
		t.Fatalf("local hash: %v", err)
	}
	remoteHash, err := RowHash(remote, []string{"prod_id", "stock"})
	if err != nil {
		t.Fatalf("remote hash: %v", err)
	}
	if localHash != remoteHash {
		t.Fatalf("expected equal hash after padding trim, got %s vs %s", localHash, remoteHash)
	}
}

func TestRowHashNumericEquivalence(t *testing.T) {
	local := map[string]interface{}{
		"precio": float64(1234.5),
	}
	remote := map[string]interface{}{
		"precio": "1234.50",
	}

	localHash, err := RowHash(local, []string{"precio"})
	if err != nil {
		t.Fatalf("local hash: %v", err)
	}
	remoteHash, err := RowHash(remote, []string{"precio"})
	if err != nil {
		t.Fatalf("remote hash: %v", err)
	}
	if localHash != remoteHash {
		t.Fatalf("expected numeric equivalence, got %s vs %s", localHash, remoteHash)
	}
}

func TestRowHashDateEquivalence(t *testing.T) {
	local := map[string]interface{}{
		"fecha_modificacion": time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
	}
	remote := map[string]interface{}{
		"fecha_modificacion": "2026-03-15",
	}

	localHash, err := RowHash(local, []string{"fecha_modificacion"})
	if err != nil {
		t.Fatalf("local hash: %v", err)
	}
	remoteHash, err := RowHash(remote, []string{"fecha_modificacion"})
	if err != nil {
		t.Fatalf("remote hash: %v", err)
	}
	if localHash != remoteHash {
		t.Fatalf("expected date equivalence, got %s vs %s", localHash, remoteHash)
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
