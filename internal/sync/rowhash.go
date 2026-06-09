package sync

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
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

	payload := make(map[string]string, len(columns))
	for _, column := range columns {
		payload[column] = canonicalHashString(row[column])
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

func canonicalHashString(value interface{}) string {
	if value == nil {
		return ""
	}

	switch typed := value.(type) {
	case []byte:
		return trimCharPadding(string(typed))
	case string:
		return normalizeStringHashValue(trimCharPadding(typed))
	case bool:
		return strconv.FormatBool(typed)
	case time.Time:
		return formatTimeHashValue(typed)
	case float32:
		return formatFloatHashValue(float64(typed))
	case float64:
		return formatFloatHashValue(typed)
	case int:
		return strconv.FormatInt(int64(typed), 10)
	case int8:
		return strconv.FormatInt(int64(typed), 10)
	case int16:
		return strconv.FormatInt(int64(typed), 10)
	case int32:
		return strconv.FormatInt(int64(typed), 10)
	case int64:
		return strconv.FormatInt(typed, 10)
	case uint:
		return strconv.FormatUint(uint64(typed), 10)
	case uint8:
		return strconv.FormatUint(uint64(typed), 10)
	case uint16:
		return strconv.FormatUint(uint64(typed), 10)
	case uint32:
		return strconv.FormatUint(uint64(typed), 10)
	case uint64:
		return strconv.FormatUint(typed, 10)
	case json.Number:
		if asInt, err := typed.Int64(); err == nil {
			return strconv.FormatInt(asInt, 10)
		}
		if asFloat, err := typed.Float64(); err == nil {
			return formatFloatHashValue(asFloat)
		}
		return normalizeStringHashValue(typed.String())
	default:
		return normalizeStringHashValue(trimCharPadding(fmt.Sprint(typed)))
	}
}

func trimCharPadding(value string) string {
	return strings.TrimRight(value, " \t")
}

func formatTimeHashValue(value time.Time) string {
	utc := value.UTC()
	if utc.Hour() == 0 && utc.Minute() == 0 && utc.Second() == 0 && utc.Nanosecond() == 0 {
		return utc.Format("2006-01-02")
	}
	return utc.Format(time.RFC3339Nano)
}

func formatFloatHashValue(value float64) string {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return fmt.Sprint(value)
	}
	if value == math.Trunc(value) && value >= math.MinInt64 && value <= math.MaxInt64 {
		return strconv.FormatInt(int64(value), 10)
	}
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func normalizeStringHashValue(value string) string {
	if value == "" {
		return ""
	}
	if asFloat, err := strconv.ParseFloat(value, 64); err == nil {
		return formatFloatHashValue(asFloat)
	}
	return value
}
