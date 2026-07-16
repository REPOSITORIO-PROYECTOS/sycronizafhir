package sync

import (
	"testing"
)

func TestOutboundTableStateKey(t *testing.T) {
	key := outboundTableStateKey("productos")
	if key != "outbound_last_run_utc_productos" {
		t.Fatalf("unexpected key: %s", key)
	}
}
