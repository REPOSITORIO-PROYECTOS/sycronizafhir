package db

import (
	"context"
	"path/filepath"
	"testing"
)

func TestSharedSQLiteQueueSingleConnection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sync_queue.db")

	first, err := NewSQLiteQueue(path)
	if err != nil {
		t.Fatalf("open first: %v", err)
	}

	second, err := NewSQLiteQueue(path)
	if err != nil {
		t.Fatalf("open second: %v", err)
	}
	if first != second {
		t.Fatal("expected shared queue pointer")
	}

	ctx := context.Background()
	if err = first.SetStateValue(ctx, "test_key", "one"); err != nil {
		t.Fatalf("set state: %v", err)
	}

	if err = first.Close(); err != nil {
		t.Fatalf("close first: %v", err)
	}

	value, exists, err := second.GetStateValue(ctx, "test_key")
	if err != nil {
		t.Fatalf("read after first close: %v", err)
	}
	if !exists || value != "one" {
		t.Fatalf("unexpected state after first close: exists=%v value=%q", exists, value)
	}

	if err = second.Close(); err != nil {
		t.Fatalf("close second: %v", err)
	}
}
