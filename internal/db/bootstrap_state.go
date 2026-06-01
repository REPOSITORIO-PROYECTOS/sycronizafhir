package db

import (
	"context"
)

const bootstrapStateKey = "bootstrap_full_load_state"

// MigrateBootstrapStateIfNeeded copies bootstrap progress from sync_queue.db (legacy)
// into bootstrap_state.db when the dedicated file has no checkpoint yet.
func MigrateBootstrapStateIfNeeded(ctx context.Context, bootstrapStore, legacyQueue *QueueSQLite) error {
	if bootstrapStore == nil || legacyQueue == nil {
		return nil
	}

	_, exists, err := bootstrapStore.GetStateValue(ctx, bootstrapStateKey)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	raw, legacyExists, err := legacyQueue.GetStateValue(ctx, bootstrapStateKey)
	if err != nil {
		return err
	}
	if !legacyExists || raw == "" {
		return nil
	}

	return bootstrapStore.SetStateValue(ctx, bootstrapStateKey, raw)
}
