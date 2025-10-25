package rest

import (
	"fmt"
	"strings"
)

func convertToPostgresArrayOrJSON(val string) string {
	val = strings.TrimSpace(val)

	// check if it's already JSON format (contains quotes around elements)
	// e.g., {"NVMe"} or ["value"] or {"key": "value"}
	if strings.Contains(val, "\"") || strings.HasPrefix(val, "[") {
		// already JSON format, return as-is for JSONB columns
		return val
	}

	// otherwise, treat as PostgREST array syntax {a,b,c}
	// remove outer braces if present
	if strings.HasPrefix(val, "{") && strings.HasSuffix(val, "}") {
		val = val[1 : len(val)-1]
	}

	// split by comma and trim whitespace
	parts := strings.Split(val, ",")
	quotedParts := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			// escape any quotes in the value
			part = strings.ReplaceAll(part, "\"", "\\\"")
			quotedParts = append(quotedParts, fmt.Sprintf("\"%s\"", part))
		}
	}

	// return as PostgreSQL array literal for array columns
	// or JSONB array for JSONB columns
	return fmt.Sprintf("{%s}", strings.Join(quotedParts, ","))
}
