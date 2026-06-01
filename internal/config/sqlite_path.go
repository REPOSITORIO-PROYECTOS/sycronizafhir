package config

import (
	"os"
	"path/filepath"
	"strings"
)

// ResolveSQLiteQueuePath turns relative SQLITE_QUEUE_PATH values into an absolute path
// under the user config directory so GUI/background processes are not tied to CWD.
func ResolveSQLiteQueuePath(configured string) (string, error) {
	trimmed := strings.TrimSpace(configured)
	if trimmed == "" {
		trimmed = "sync_queue.db"
	}

	if filepath.IsAbs(trimmed) {
		return trimmed, nil
	}

	baseDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(baseDir, "sycronizafhir", filepath.Base(trimmed)), nil
}

// ResolveBootstrapStateSQLitePath stores bootstrap progress in a dedicated file so
// outbound/inbound workers on sync_queue.db cannot block bootstrap checkpoints.
func ResolveBootstrapStateSQLitePath(queueConfigured string) (string, error) {
	queuePath, err := ResolveSQLiteQueuePath(queueConfigured)
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(queuePath), "bootstrap_state.db"), nil
}
