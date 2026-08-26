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
	// CloudOwnedFields lista, por tabla, las columnas "propiedad de la nube":
	// campos cuyo alta/estado gobierna Supabase/Picking y que el outbound NO debe
	// pisar con un valor local deshabilitado (evita drift tipo destildado `web`).
	CloudOwnedFields map[string][]string `json:"cloud_owned_fields"`
	// CloudAuthoritativeFields: la nube gana aunque el ERP tenga otro valor
	// habilitado. Caso: prod_orden escrito por la vitrina en Supabase; sin esta
	// guarda el outbound SQL→nube lo pisa con el número viejo del ERP.
	CloudAuthoritativeFields map[string][]string `json:"cloud_authoritative_fields"`
}

// DefaultCloudOwnedFields protege por defecto el flag `web` de `clientes`
// (caso Riera 1358: stamps masivos de fecha_modificacion destildaban la tienda).
func DefaultCloudOwnedFields() map[string][]string {
	return map[string][]string{
		"clientes": {"web"},
	}
}

// DefaultCloudAuthoritativeFields: la nube gana aunque el ERP tenga otro valor.
func DefaultCloudAuthoritativeFields() map[string][]string {
	return map[string][]string{
		"productos": {"prod_orden"},
	}
}

// CoreTables son las tablas que sostienen la tienda (catálogo, clientes, stock).
// Nunca deben quedar deshabilitadas por accidente desde la UI (ver incidente MICA
// 2026-07-16: una "subida rápida" apagó `productos` para siempre).
var CoreTables = []string{"clientes", "productos", "productos_depositos", "rubro", "subrubro"}

func DefaultSyncTablesConfig() SyncTablesConfig {
	return SyncTablesConfig{
		EnabledTables: append([]string{}, CoreTables...),
		TableMappings: map[string]string{
			"articulos": "productos",
		},
		AutoAuditIntervalHours:   6,
		AutoSyncOnAudit:          true,
		CloudOwnedFields:         DefaultCloudOwnedFields(),
		CloudAuthoritativeFields: DefaultCloudAuthoritativeFields(),
	}
}

// RemovedCoreTables devuelve las tablas core que NO están en el set habilitado.
func RemovedCoreTables(enabled []string) []string {
	present := map[string]bool{}
	for _, name := range enabled {
		present[strings.TrimSpace(name)] = true
	}
	removed := make([]string, 0, len(CoreTables))
	for _, core := range CoreTables {
		if !present[core] {
			removed = append(removed, core)
		}
	}
	return removed
}

// HasEnabledTables indica si el set (ya normalizado) tiene al menos una tabla.
func HasEnabledTables(enabled []string) bool {
	return len(normalizeTableNames(enabled)) > 0
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
	var rawFields map[string]json.RawMessage
	if err = json.Unmarshal(raw, &rawFields); err != nil {
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
	if _, exists := rawFields["auto_sync_on_audit"]; !exists {
		cfg.AutoSyncOnAudit = defaults.AutoSyncOnAudit
	}
	if cfg.CloudOwnedFields == nil {
		cfg.CloudOwnedFields = defaults.CloudOwnedFields
	} else {
		// Garantiza que las protecciones por defecto (p. ej. clientes.web) sigan
		// vigentes aunque la config en disco no las declare.
		for table, fields := range defaults.CloudOwnedFields {
			if _, exists := cfg.CloudOwnedFields[table]; !exists {
				cfg.CloudOwnedFields[table] = fields
			}
		}
	}
	if cfg.CloudAuthoritativeFields == nil {
		cfg.CloudAuthoritativeFields = defaults.CloudAuthoritativeFields
	} else {
		for table, fields := range defaults.CloudAuthoritativeFields {
			if _, exists := cfg.CloudAuthoritativeFields[table]; !exists {
				cfg.CloudAuthoritativeFields[table] = fields
			}
		}
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
		EnabledTables:            normalizeTableNames(cfg.EnabledTables),
		TableMappings:            normalizeTableMappings(cfg.TableMappings),
		AutoAuditIntervalHours:   cfg.AutoAuditIntervalHours,
		AutoSyncOnAudit:          cfg.AutoSyncOnAudit,
		CloudOwnedFields:         normalizeCloudOwnedFields(cfg.CloudOwnedFields),
		CloudAuthoritativeFields: normalizeCloudOwnedFields(cfg.CloudAuthoritativeFields),
	}
	if len(normalized.CloudOwnedFields) == 0 {
		normalized.CloudOwnedFields = DefaultCloudOwnedFields()
	}
	if len(normalized.CloudAuthoritativeFields) == 0 {
		normalized.CloudAuthoritativeFields = DefaultCloudAuthoritativeFields()
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

// CloudOwnedFieldsFor devuelve las columnas propiedad de la nube para una tabla.
func (c SyncTablesConfig) CloudOwnedFieldsFor(tableName string) []string {
	if c.CloudOwnedFields == nil {
		return nil
	}
	return c.CloudOwnedFields[tableName]
}

// CloudAuthoritativeFieldsFor: columnas donde la nube gana aunque el ERP tenga otro valor.
func (c SyncTablesConfig) CloudAuthoritativeFieldsFor(tableName string) []string {
	if c.CloudAuthoritativeFields == nil {
		return nil
	}
	return c.CloudAuthoritativeFields[tableName]
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

func normalizeCloudOwnedFields(values map[string][]string) map[string][]string {
	if len(values) == 0 {
		return nil
	}
	result := map[string][]string{}
	for table, fields := range values {
		tableName := strings.TrimSpace(table)
		if tableName == "" {
			continue
		}
		normalizedFields := normalizeTableNames(fields)
		if len(normalizedFields) == 0 {
			continue
		}
		result[tableName] = normalizedFields
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
