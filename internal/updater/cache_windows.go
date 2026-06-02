//go:build windows

package updater

import (
	"os"
	"path/filepath"
	"strings"
)

func WebviewUserDataPath() string {
	baseDir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(baseDir, "sycronizafhir", "webview2")
}

func webviewVersionMarkerPath() string {
	baseDir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(baseDir, "sycronizafhir", "webview-version.marker")
}

// ClearWebviewCacheIfVersionChanged removes WebView2 profile data when the running
// binary version changes so the embedded UI is not served from a stale cache.
func ClearWebviewCacheIfVersionChanged() {
	current := NormalizeVersion(ProductVersion())
	if current == "" {
		return
	}

	markerPath := webviewVersionMarkerPath()
	previous := ""
	if raw, err := os.ReadFile(markerPath); err == nil {
		previous = NormalizeVersion(string(raw))
	}

	if previous == current {
		return
	}

	webviewPath := WebviewUserDataPath()
	if webviewPath != "" {
		_ = os.RemoveAll(webviewPath)
	}

	if markerPath != "" {
		_ = os.MkdirAll(filepath.Dir(markerPath), 0o755)
		_ = os.WriteFile(markerPath, []byte(current), 0o600)
	}
}

func FormatDisplayVersion(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "desconocida"
	}
	if !strings.HasPrefix(strings.ToLower(trimmed), "v") {
		return "v" + trimmed
	}
	return trimmed
}
