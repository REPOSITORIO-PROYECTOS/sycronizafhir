package sync

import (
	"fmt"
	"testing"
)

func TestMergeRowsByPrimaryKeysAddsMissingPedidos(t *testing.T) {
	base := []map[string]interface{}{
		{"ped_id": "0000-00902800", "estado": "C"},
	}
	extra := []map[string]interface{}{
		{"ped_id": "0000-00902800", "estado": "P"},
		{"ped_id": "0000-00902801", "estado": "P"},
		{"ped_id": "0000-00902802", "estado": "C"},
		{"ped_id": "0000-00902803", "estado": "C"},
	}
	merged := mergeRowsByPrimaryKeys(base, extra, []string{"ped_id"})
	if len(merged) != 4 {
		t.Fatalf("want 4 rows, got %d", len(merged))
	}
	if estado := merged[0]["estado"]; estado != "C" {
		t.Fatalf("base row should win on duplicate ped_id, got %v", estado)
	}
	ids := make(map[string]bool, len(merged))
	for _, row := range merged {
		ids[fmt.Sprint(row["ped_id"])] = true
	}
	for _, want := range []string{"0000-00902800", "0000-00902801", "0000-00902802", "0000-00902803"} {
		if !ids[want] {
			t.Fatalf("missing ped_id %s in merged set", want)
		}
	}
}
