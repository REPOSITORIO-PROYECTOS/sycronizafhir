package sync

import (
	"testing"
	"time"

	"sycronizafhir/internal/monitor"
)

func TestReconnectBackoffExponentialCap(t *testing.T) {
	t.Parallel()

	backoff := newReconnectBackoff(5*time.Second, 60*time.Second)
	sequence := []time.Duration{
		backoff.Next(),
		backoff.Next(),
		backoff.Next(),
		backoff.Next(),
		backoff.Next(),
	}

	want := []time.Duration{
		5 * time.Second,
		10 * time.Second,
		20 * time.Second,
		40 * time.Second,
		60 * time.Second,
	}

	for i, got := range sequence {
		if got != want[i] {
			t.Fatalf("attempt %d = %s, want %s", i+1, got, want[i])
		}
	}

	backoff.Reset()
	if got := backoff.Next(); got != 5*time.Second {
		t.Fatalf("after reset Next() = %s, want 5s", got)
	}
}

func TestReconnectFailureLogAggregates(t *testing.T) {
	t.Parallel()

	runtime := monitor.NewRuntime()
	log := newReconnectFailureLog()
	log.minInterval = time.Hour

	log.Record(runtime, "inbound", "timeout", 5*time.Second)
	log.Record(runtime, "inbound", "timeout", 10*time.Second)
	log.Record(runtime, "inbound", "timeout", 20*time.Second)

	snapshot := runtime.Snapshot()
	if len(snapshot.Logs) != 1 {
		t.Fatalf("expected 1 aggregated log line, got %d: %v", len(snapshot.Logs), snapshot.Logs)
	}

	log.Reset(runtime, "inbound")
	snapshot = runtime.Snapshot()
	if len(snapshot.Logs) != 2 {
		t.Fatalf("expected recovery log, got %d: %v", len(snapshot.Logs), snapshot.Logs)
	}
}
