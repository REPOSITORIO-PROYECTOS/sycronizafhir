package supabase

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
)

// NormalizeParamValue convierte valores dinámicos (p. ej. []interface{} de rows.Values
// o json.Unmarshal) en tipos que pgx puede codificar en protocolo simple.
func NormalizeParamValue(value interface{}) interface{} {
	if value == nil {
		return nil
	}

	switch v := value.(type) {
	case []interface{}:
		normalized := normalizeInterfaceSlice(v)
		// #region agent log
		agentDebugLog("pg_values.go:NormalizeParamValue", "normalized []interface{}", "B", map[string]interface{}{
			"rawType":  fmt.Sprintf("%T", value),
			"normType": fmt.Sprintf("%T", normalized),
			"len":      len(v),
		})
		// #endregion
		return normalized
	case []byte:
		return v
	}

	rv := reflect.ValueOf(value)
	if rv.Kind() == reflect.Slice {
		if rv.IsNil() {
			return value
		}
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			return value
		}
		generic := make([]interface{}, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			generic[i] = rv.Index(i).Interface()
		}
		normalized := normalizeInterfaceSlice(generic)
		// #region agent log
		agentDebugLog("pg_values.go:NormalizeParamValue", "normalized typed slice", "B", map[string]interface{}{
			"rawType":  fmt.Sprintf("%T", value),
			"normType": fmt.Sprintf("%T", normalized),
			"len":      rv.Len(),
		})
		// #endregion
		return normalized
	}

	if s, ok := value.(string); ok {
		if parsed, ok := parsePostgresArrayLiteral(s); ok {
			// #region agent log
			agentDebugLog("pg_values.go:NormalizeParamValue", "parsed postgres array literal", "B", map[string]interface{}{
				"normType": fmt.Sprintf("%T", parsed),
				"len":      sliceLen(parsed),
			})
			// #endregion
			return parsed
		}
		if strings.HasPrefix(strings.TrimSpace(s), "[") {
			var decoded []interface{}
			if err := json.Unmarshal([]byte(s), &decoded); err == nil {
				return normalizeInterfaceSlice(decoded)
			}
		}
	}

	return value
}

// NormalizeRowMap normaliza todos los valores de una fila antes de upsert o lectura downstream.
func NormalizeRowMap(row map[string]interface{}) map[string]interface{} {
	for key, value := range row {
		row[key] = NormalizeParamValue(value)
	}
	return row
}

func normalizeParamValue(value interface{}) interface{} {
	return NormalizeParamValue(value)
}

func normalizeInterfaceSlice(arr []interface{}) interface{} {
	if len(arr) == 0 {
		return []int32{}
	}

	if allStrings(arr) {
		out := make([]string, len(arr))
		for i, el := range arr {
			out[i] = fmt.Sprint(el)
		}
		return out
	}

	if allBools(arr) {
		out := make([]bool, len(arr))
		for i, el := range arr {
			out[i] = el.(bool)
		}
		return out
	}

	if ints, ok := asInt32Slice(arr); ok {
		return ints
	}

	if ints, ok := asInt64Slice(arr); ok {
		return ints
	}

	// #region agent log
	firstTypes := make([]string, 0, 3)
	for i, el := range arr {
		if i >= 3 {
			break
		}
		firstTypes = append(firstTypes, fmt.Sprintf("%T", el))
	}
	agentDebugLog("pg_values.go:normalizeInterfaceSlice", "int conversion failed, fallback []string", "C", map[string]interface{}{
		"len":        len(arr),
		"firstTypes": firstTypes,
	})
	// #endregion

	out := make([]string, len(arr))
	for i, el := range arr {
		out[i] = fmt.Sprint(el)
	}
	return out
}

func parsePostgresArrayLiteral(raw string) (interface{}, bool) {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) < 2 || trimmed[0] != '{' || trimmed[len(trimmed)-1] != '}' {
		return nil, false
	}
	inner := strings.TrimSpace(trimmed[1 : len(trimmed)-1])
	if inner == "" {
		return []int32{}, true
	}

	parts := splitPostgresArrayElements(inner)
	generic := make([]interface{}, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || strings.EqualFold(part, "null") {
			generic = append(generic, nil)
			continue
		}
		if len(part) >= 2 && part[0] == '"' && part[len(part)-1] == '"' {
			generic = append(generic, strings.ReplaceAll(part[1:len(part)-1], `\"`, `"`))
			continue
		}
		var n int64
		if _, err := fmt.Sscan(part, &n); err == nil {
			generic = append(generic, n)
			continue
		}
		generic = append(generic, part)
	}
	return normalizeInterfaceSlice(generic), true
}

func splitPostgresArrayElements(inner string) []string {
	parts := make([]string, 0)
	var current strings.Builder
	inQuotes := false
	escaped := false
	for _, r := range inner {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' && inQuotes {
			escaped = true
			continue
		}
		if r == '"' {
			inQuotes = !inQuotes
			current.WriteRune(r)
			continue
		}
		if r == ',' && !inQuotes {
			parts = append(parts, current.String())
			current.Reset()
			continue
		}
		current.WriteRune(r)
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}

func allStrings(arr []interface{}) bool {
	for _, el := range arr {
		if el == nil {
			continue
		}
		if _, ok := el.(string); !ok {
			return false
		}
	}
	return true
}

func allBools(arr []interface{}) bool {
	for _, el := range arr {
		if el == nil {
			continue
		}
		if _, ok := el.(bool); !ok {
			return false
		}
	}
	return true
}

func asInt32Slice(arr []interface{}) ([]int32, bool) {
	out := make([]int32, len(arr))
	for i, el := range arr {
		if el == nil {
			out[i] = 0
			continue
		}
		n, ok := toInt64(el)
		if !ok {
			return nil, false
		}
		if n < math.MinInt32 || n > math.MaxInt32 {
			return nil, false
		}
		out[i] = int32(n)
	}
	return out, true
}

func asInt64Slice(arr []interface{}) ([]int64, bool) {
	out := make([]int64, len(arr))
	for i, el := range arr {
		if el == nil {
			out[i] = 0
			continue
		}
		n, ok := toInt64(el)
		if !ok {
			return nil, false
		}
		out[i] = n
	}
	return out, true
}

func toInt64(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int8:
		return int64(n), true
	case int16:
		return int64(n), true
	case int32:
		return int64(n), true
	case int64:
		return n, true
	case uint:
		return int64(n), true
	case uint8:
		return int64(n), true
	case uint16:
		return int64(n), true
	case uint32:
		return int64(n), true
	case uint64:
		if n > math.MaxInt64 {
			return 0, false
		}
		return int64(n), true
	case float32:
		if float64(n) != math.Trunc(float64(n)) {
			return 0, false
		}
		return int64(n), true
	case float64:
		if n != math.Trunc(n) {
			return 0, false
		}
		return int64(n), true
	default:
		return 0, false
	}
}
