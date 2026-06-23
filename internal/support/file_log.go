package support

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

type FileLogWriter struct {
	mu sync.Mutex
}

func NewFileLogWriter() *FileLogWriter {
	return &FileLogWriter{}
}

func (w *FileLogWriter) Write(p []byte) (int, error) {
	line := strings.TrimSpace(string(p))
	if line == "" {
		return len(p), nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if err := EnsureDirs(); err != nil {
		return len(p), nil
	}

	logPath, err := AppLogPath()
	if err != nil {
		return len(p), nil
	}

	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return len(p), nil
	}
	defer file.Close()

	formatted := fmt.Sprintf("%s | %s\n", time.Now().Format(time.RFC3339), line)
	_, _ = file.WriteString(formatted)

	if looksLikeError(line) {
		_ = RecordIncident("error", "log", line, nil)
	}

	return len(p), nil
}

func looksLikeError(line string) bool {
	lower := strings.ToLower(line)
	needles := []string{
		"error",
		"failed",
		"fallo",
		"fatal",
		"panic",
		"warn:",
		"warn ",
	}
	for _, needle := range needles {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}
