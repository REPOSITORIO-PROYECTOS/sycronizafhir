//go:build windows

package support

import (
	"os/exec"
	"path/filepath"
	"strings"
)

func OpenFolder(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	return exec.Command("explorer", abs).Start()
}
