package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type LocalDBOverride struct {
	LocalPostgresURL string `json:"local_postgres_url"`
}

func localOverridePath() (string, error) {
	baseDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(baseDir, "sycronizafhir", "local-db.json"), nil
}

func LoadLocalDBOverride() (LocalDBOverride, bool, error) {
	path, err := localOverridePath()
	if err != nil {
		return LocalDBOverride{}, false, err
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return LocalDBOverride{}, false, nil
		}
		return LocalDBOverride{}, false, err
	}

	var override LocalDBOverride
	if err = json.Unmarshal(raw, &override); err != nil {
		return LocalDBOverride{}, false, err
	}

	override.LocalPostgresURL = strings.TrimSpace(override.LocalPostgresURL)
	if override.LocalPostgresURL == "" {
		return LocalDBOverride{}, false, nil
	}

	return override, true, nil
}

func SaveLocalDBOverride(override LocalDBOverride) error {
	path, err := localOverridePath()
	if err != nil {
		return err
	}

	if err = os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	normalized := LocalDBOverride{
		LocalPostgresURL: strings.TrimSpace(override.LocalPostgresURL),
	}
	payload, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, payload, 0o600)
}

