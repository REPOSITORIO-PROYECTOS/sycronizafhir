package sync

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func PKKey(row map[string]interface{}, pkColumns []string) (string, error) {
	if len(pkColumns) == 0 {
		return "", fmt.Errorf("primary key columns required")
	}

	parts := make([]string, 0, len(pkColumns))
	for _, column := range pkColumns {
		value, ok := row[column]
		if !ok || value == nil {
			return "", fmt.Errorf("missing pk column %s", column)
		}
		parts = append(parts, fmt.Sprintf("%s=%v", column, value))
	}
	return strings.Join(parts, "|"), nil
}

func RowHash(row map[string]interface{}, columns []string) (string, error) {
	if len(columns) == 0 {
		return "", fmt.Errorf("columns required for hash")
	}

	payload := make(map[string]interface{}, len(columns))
	for _, column := range columns {
		payload[column] = normalizeHashValue(row[column])
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func CommonColumns(localRow map[string]interface{}, remoteColumns map[string]bool) []string {
	columns := make([]string, 0)
	for column := range localRow {
		if remoteColumns[column] {
			columns = append(columns, column)
		}
	}
	sort.Strings(columns)
	return columns
}

func normalizeHashValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case []byte:
		return string(typed)
	default:
		return typed
	}
}
