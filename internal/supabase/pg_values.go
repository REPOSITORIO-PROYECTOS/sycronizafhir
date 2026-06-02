package supabase

import (
	"fmt"
	"math"
)

// normalizeParamValue convierte []interface{} (arrays leídos con rows.Values) en slices
// tipados que pgx puede enviar a Postgres en protocolo simple.
func normalizeParamValue(value interface{}) interface{} {
	if value == nil {
		return nil
	}
	switch v := value.(type) {
	case []interface{}:
		return normalizeInterfaceSlice(v)
	default:
		return value
	}
}

func normalizeInterfaceSlice(arr []interface{}) interface{} {
	if len(arr) == 0 {
		return []string{}
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

	out := make([]string, len(arr))
	for i, el := range arr {
		out[i] = fmt.Sprint(el)
	}
	return out
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
