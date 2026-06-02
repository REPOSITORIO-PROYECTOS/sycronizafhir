//go:build windows

package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

const defaultInstallDir = `C:\Program Files\sycronizafhir`

func Check(ctx context.Context) Status {
	installDir, fromInstall := resolveInstallDir()
	current := readInstalledVersion(installDir)
	if current == "" {
		current = ProductVersion()
	}

	cfg, cfgErr := loadConfig(installDir)
	if cfgErr != nil {
		return Status{
			CurrentVersion: current,
			CanApply:       fromInstall,
			Message:        cfgErr.Error(),
		}
	}
	if !cfg.Enabled {
		return Status{
			CurrentVersion: current,
			CanApply:       false,
			Message:        "Auto-actualizacion deshabilitada en github-update-config.json",
		}
	}

	release, err := fetchLatestRelease(ctx, cfg.GithubOwner, cfg.GithubRepo, cfg.GithubToken)
	if err != nil {
		return Status{
			CurrentVersion: current,
			CanApply:       fromInstall,
			Message:        err.Error(),
		}
	}

	latest := release.TagName
	available := !VersionsEqual(current, latest)
	message := "Estas en la ultima version."
	if available {
		message = fmt.Sprintf("Hay una actualizacion disponible: %s", latest)
	}

	return Status{
		Available:      available,
		CurrentVersion: current,
		LatestVersion:  latest,
		ReleaseURL:     release.HTMLURL,
		ReleaseNotes:   strings.TrimSpace(release.Body),
		CanApply:       fromInstall && available,
		Message:        message,
	}
}

func Apply(reopenMonitor bool) ApplyResult {
	installDir, fromInstall := resolveInstallDir()
	if !fromInstall {
		return ApplyResult{
			Success: false,
			Message: "La actualizacion in-app solo esta disponible en la instalacion de Windows (Program Files).",
		}
	}

	scriptPath := filepath.Join(installDir, "actualizar-sycronizafhir.ps1")
	if _, err := os.Stat(scriptPath); err != nil {
		return ApplyResult{
			Success: false,
			Message: fmt.Sprintf("No se encontro el script de actualizacion: %s", scriptPath),
		}
	}

	reopenFlag := ""
	if reopenMonitor {
		reopenFlag = " -ReopenMonitor"
	}
	elevated := fmt.Sprintf(
		`Start-Process powershell -Verb RunAs -WindowStyle Hidden -ArgumentList '-NoProfile -ExecutionPolicy Bypass -File \"%s\"%s'`,
		scriptPath,
		reopenFlag,
	)

	cmd := exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", elevated)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		return ApplyResult{
			Success: false,
			Message: fmt.Sprintf("No se pudo iniciar la actualizacion elevada: %v", err),
		}
	}

	return ApplyResult{
		Success: true,
		Message: "Actualizacion iniciada. La aplicacion se cerrara y se reabrira al terminar.",
	}
}

func resolveInstallDir() (string, bool) {
	exePath, err := os.Executable()
	if err != nil {
		return defaultInstallDir, false
	}
	dir := filepath.Clean(filepath.Dir(exePath))
	lower := strings.ToLower(dir)
	if strings.Contains(lower, `\program files\sycronizafhir`) ||
		strings.Contains(lower, `\program files (x86)\sycronizafhir`) {
		return dir, true
	}
	if filepath.Base(dir) == "sycronizafhir" && fileExists(filepath.Join(dir, "actualizar-sycronizafhir.ps1")) {
		return dir, true
	}
	return dir, false
}

func readInstalledVersion(installDir string) string {
	path := filepath.Join(installDir, "version.txt")
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

func loadConfig(installDir string) (githubConfig, error) {
	path := filepath.Join(installDir, "github-update-config.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return githubConfig{}, fmt.Errorf("no se pudo leer github-update-config.json: %w", err)
	}
	var cfg githubConfig
	if err = json.Unmarshal(raw, &cfg); err != nil {
		return githubConfig{}, fmt.Errorf("config de actualizacion invalida: %w", err)
	}
	return cfg, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
