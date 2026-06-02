package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type SyncTablesConfig struct {
	EnabledTables          []string          `json:"enabled_tables"`
	TableMappings          map[string]string `json:"table_mappings"`
	AutoAuditIntervalHours int               `json:"auto_audit_interval_hours"`
	AutoSyncOnAudit        bool              `json:"auto_sync_on_audit"`
}

func DefaultSyncTablesConfig() SyncTablesConfig {
	return SyncTablesConfig{
		EnabledTables: []string{"clientes", "productos"},
		TableMappings: map[string]string{
			"articulos": "productos",
		},
		AutoAuditIntervalHours: 6,
		AutoSyncOnAudit:        true,
	}
}

func syncTablesConfigPath() (string, error) {
	baseDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(baseDir, "sycronizafhir", "sync-tables.json"), nil
}

func LoadSyncTablesConfig() (SyncTablesConfig, error) {
	defaults := DefaultSyncTablesConfig()
	path, err := syncTablesConfigPath()
	if err != nil {
		return defaults, err
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return defaults, nil
		}
		return defaults, err
	}

	var cfg SyncTablesConfig
	if err = json.Unmarshal(raw, &cfg); err != nil {
		return defaults, err
	}

	if len(cfg.EnabledTables) == 0 {
		cfg.EnabledTables = defaults.EnabledTables
	}
	if cfg.TableMappings == nil {
		cfg.TableMappings = defaults.TableMappings
	} else {
		for key, value := range defaults.TableMappings {
			if _, exists := cfg.TableMappings[key]; !exists {
				cfg.TableMappings[key] = value
			}
		}
	}
	if cfg.AutoAuditIntervalHours <= 0 {
		cfg.AutoAuditIntervalHours = defaults.AutoAuditIntervalHours
	}

	return cfg, nil
}

func SaveSyncTablesConfig(cfg SyncTablesConfig) error {
	path, err := syncTablesConfigPath()
	if err != nil {
		return err
	}

	if err = os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	normalized := SyncTablesConfig{
		EnabledTables:          normalizeTableNames(cfg.EnabledTables),
		TableMappings:          normalizeTableMappings(cfg.TableMappings),
		AutoAuditIntervalHours: cfg.AutoAuditIntervalHours,
		AutoSyncOnAudit:        cfg.AutoSyncOnAudit,
	}
	if normalized.AutoAuditIntervalHours <= 0 {
		normalized.AutoAuditIntervalHours = 6
	}

	payload, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, payload, 0o600)
}

func (c SyncTablesConfig) ResolveRemoteTable(localTable string) string {
	if mapped, ok := c.TableMappings[localTable]; ok && strings.TrimSpace(mapped) != "" {
		return mapped
	}
	return localTable
}

func (c SyncTablesConfig) IsEnabled(tableName string) bool {
	for _, name := range c.EnabledTables {
		if name == tableName {
			return true
		}
	}
	return false
}

func normalizeTableNames(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		name := strings.TrimSpace(value)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		result = append(result, name)
	}
	return result
}

func normalizeTableMappings(values map[string]string) map[string]string {
	result := map[string]string{}
	for key, value := range values {
		localName := strings.TrimSpace(key)
		remoteName := strings.TrimSpace(value)
		if localName == "" || remoteName == "" {
			continue
		}
		result[localName] = remoteName
	}
	return result
}
