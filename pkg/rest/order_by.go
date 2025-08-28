package rest

import (
	"regexp"
	"strings"
)

type OrderParam struct {
	Column        string
	Direction     string // asc or desc
	NullsPosition string // first or last
	Similarity    string // https://www.postgresql.org/docs/current/pgtrgm.html
}

func parseOrderParam(order string) []OrderParam {
	parts := splitOrderParamString(order) // split by comma, but ignore commas inside similarity()
	result := make([]OrderParam, 0, len(parts))

	similarityRegex := regexp.MustCompile(`similarity\(([^,]+),\s*([^)]+)\)`)

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Default direction is ascending
		direction := "asc"
		nullsPosition := "last" // PostgreSQL default
		similarity := ""
		columnName := part

		if matches := similarityRegex.FindStringSubmatch(part); matches != nil {
			columnName = strings.TrimSpace(matches[1])
			similarity = strings.Trim(strings.TrimSpace(matches[2]), `'"`) // remove quotes if present
			direction = "desc"                                             // default to desc for similarity (most similar first)

			// check explicit direction
			originalPart := part
			if strings.HasSuffix(originalPart, ".desc") {
				direction = "desc"
			} else if strings.HasSuffix(originalPart, ".asc") {
				direction = "asc"
			}
		} else {
			// Check for explicit direction in regular ordering column name
			if strings.HasSuffix(part, ".desc") {
				part = strings.TrimSuffix(part, ".desc")
				direction = "desc"
			} else if strings.HasSuffix(part, ".asc") {
				part = strings.TrimSuffix(part, ".asc")
				direction = "asc"
			}
			// If no direction specified, use default "asc"
			columnName = part
		}

		// Check for nulls position
		if strings.HasSuffix(columnName, ".nullsfirst") {
			columnName = strings.TrimSuffix(columnName, ".nullsfirst")
			nullsPosition = "first"
		} else if strings.HasSuffix(columnName, ".nullslast") {
			columnName = strings.TrimSuffix(columnName, ".nullslast")
			nullsPosition = "last"
		}

		result = append(result, OrderParam{
			Column:        columnName,
			Direction:     direction,
			NullsPosition: nullsPosition,
			Similarity:    similarity,
		})
	}

	return result
}

// splitOrderString splits the order string by commas, but ignores commas inside parentheses eg similarity(name,search_string)
func splitOrderParamString(order string) []string {
	var parts []string
	var current strings.Builder
	parenDepth := 0

	for _, char := range order {
		switch char {
		case '(':
			parenDepth++
			current.WriteRune(char)
		case ')':
			parenDepth--
			current.WriteRune(char)
		case ',':
			if parenDepth == 0 {
				// not inside parentheses, so this comma is a separator
				parts = append(parts, current.String())
				current.Reset()
			} else {
				// inside parentheses, keep the comma as part of the current segment
				current.WriteRune(char)
			}
		default:
			current.WriteRune(char)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}
