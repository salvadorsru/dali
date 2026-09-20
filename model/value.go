package model

import "fmt"

func asString(v any) string {
	s, _ := v.(string)
	return s
}

func asFloat(v any) float64 {
	switch n := v.(type) {
	case float32:
		return float64(n)
	case float64:
		return n
	case int32:
		return float64(n)
	case uint32:
		return float64(n)
	case int64:
		return float64(n)
	case uint64:
		return float64(n)
	}
	return 0
}

func asInt(v any) int64 {
	switch n := v.(type) {
	case int32:
		return int64(n)
	case uint32:
		return int64(n)
	case int64:
		return n
	case uint64:
		return int64(n)
	case float32:
		return int64(n)
	case float64:
		return int64(n)
	}
	return 0
}

func asBool(v any) bool {
	b, _ := v.(bool)
	return b
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func asSlice(v any) []any {
	s, _ := v.([]any)
	return s
}

func guid(m map[string]any) string {
	if m == nil {
		return ""
	}
	g := asMap(m["guid"])
	if g == nil {
		return ""
	}
	return formatGUID(g)
}

func formatGUID(g map[string]any) string {
	return fmt.Sprintf("%d:%d", asInt(g["sessionID"]), asInt(g["localID"]))
}

func guidValue(v any) string {
	g := asMap(v)
	if g == nil {
		return ""
	}
	return formatGUID(g)
}

// guidOf unwraps a single-level guid wrapper such as {"guid": {...}}.
func guidOf(v any) string {
	return guid(asMap(v))
}

func parentGUID(m map[string]any) string {
	pi := asMap(m["parentIndex"])
	return guid(pi)
}
