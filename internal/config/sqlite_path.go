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
