package db

import (
	"context"
	"path/filepath"
	"testing"
)

func TestMigrateBootstrapStateIfNeeded(t *testing.T) {
	dir := t.TempDir()
	legacyPath := filepath.Join(dir, "sync_queue.db")
	bootstrapPath := filepath.Join(dir, "bootstrap_state.db")

	legacy, err := NewSQLiteQueue(legacyPath)
	if err != nil {
		t.Fatalf("open legacy: %v", err)
	}
	defer legacy.Close()

	bootstrap, err := NewSQLiteQueue(bootstrapPath)
	if err != nil {
		t.Fatalf("open bootstrap store: %v", err)
	}
	defer bootstrap.Close()

	ctx := context.Background()
	payload := `{"state":"running","processed_rows":10}`
	if err = legacy.SetStateValue(ctx, bootstrapStateKey, payload); err != nil {
		t.Fatalf("seed legacy: %v", err)
	}

	if err = MigrateBootstrapStateIfNeeded(ctx, bootstrap, legacy); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	value, exists, err := bootstrap.GetStateValue(ctx, bootstrapStateKey)
	if err != nil {
		t.Fatalf("read migrated: %v", err)
	}
	if !exists || value != payload {
		t.Fatalf("unexpected migrated value: exists=%v value=%q", exists, value)
	}
}
