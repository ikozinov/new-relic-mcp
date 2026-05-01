package tools

import (
	"encoding/json"
	"fmt"
	"strings"
)

func EscNrql(s string) string {
	return strings.ReplaceAll(s, "'", "\\'")
}

func FormatFacet(facetVal interface{}) string {
	if facetVal == nil {
		return "unknown"
	}
	switch v := facetVal.(type) {
	case []interface{}:
		var strs []string
		for _, item := range v {
			strs = append(strs, fmt.Sprintf("%v", item))
		}
		return strings.Join(strs, ", ")
	default:
		return fmt.Sprintf("%v", v)
	}
}

func FormatValue(v interface{}) string {
	if v == nil {
		return "null"
	}
	switch val := v.(type) {
	case float64:
		if val == float64(int(val)) {
			return fmt.Sprintf("%v", int(val))
		}
		return fmt.Sprintf("%.4f", val)
	case map[string]interface{}:
		isPercentile := true
		var entries []string
		for k, val2 := range val {
			if _, ok := val2.(float64); !ok {
				isPercentile = false
				break
			}
			entries = append(entries, fmt.Sprintf("p%s=%.4f", k, val2.(float64)))
		}
		if isPercentile && len(val) <= 5 {
			return strings.Join(entries, ", ")
		}
		jsonStr, _ := json.Marshal(val)
		return string(jsonStr)
	case []interface{}:
		var strs []string
		for _, item := range val {
			strs = append(strs, fmt.Sprintf("%v", item))
		}
		return strings.Join(strs, ", ")
	default:
		return fmt.Sprintf("%v", val)
	}
}
