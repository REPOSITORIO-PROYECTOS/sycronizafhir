package supabase

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"time"
)

// #region agent log
func agentDebugLog(location, message, hypothesisID string, data map[string]interface{}) {
	payload := map[string]interface{}{
		"sessionId":    "475a38",
		"location":     location,
		"message":      message,
		"data":         data,
		"timestamp":    time.Now().UnixMilli(),
		"hypothesisId": hypothesisID,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return
	}
	line := append(raw, '\n')
	for _, path := range agentDebugLogPaths() {
		_ = appendAgentDebugLine(path, line)
	}
}

func agentDebugLogPaths() []string {
	paths := []string{"debug-475a38.log"}
	if base, err := os.UserConfigDir(); err == nil {
		paths = append(paths, filepath.Join(base, "sycronizafhir", "debug-475a38.log"))
	}
	if exe, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(exe), "debug-475a38.log"))
	}
	return paths
}

func appendAgentDebugLine(path string, line []byte) error {
	if dir := filepath.Dir(path); dir != "." {
		_ = os.MkdirAll(dir, 0o755)
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(line)
	return err
}

func isSliceLike(value interface{}) bool {
	if value == nil {
		return false
	}
	kind := reflect.TypeOf(value).Kind()
	return kind == reflect.Slice || kind == reflect.Array
}

func sliceLen(value interface{}) int {
	v := reflect.ValueOf(value)
	if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
		return 0
	}
	return v.Len()
}

// #endregion
