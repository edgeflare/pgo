package mqtt

import (
	"regexp"
	"strconv"
	"strings"
)

type TopicToField struct {
	Field   string `json:"field" yaml:"field"`
	Index   string `json:"index,omitempty" yaml:"index,omitempty"` // "0", "1", "-1", etc.
	Pattern string `json:"pattern,omitempty" yaml:"pattern,omitempty"`
	Static  string `json:"static,omitempty" yaml:"static,omitempty"`
	Type    string `json:"type,omitempty" yaml:"type,omitempty"`
}

func extractFieldsFromTopic(topic string, fields []TopicToField) map[string]any {
	segments := strings.Split(strings.Trim(topic, "/"), "/")
	result := make(map[string]any)

	for _, field := range fields {
		var rawValue string

		// Static value
		if field.Static != "" {
			rawValue = field.Static
		} else if field.Index != "" {
			// Index-based extraction - parse string to int
			if idx, err := strconv.Atoi(field.Index); err == nil {
				if idx < 0 {
					idx = len(segments) + idx // Support negative indices
				}
				if idx >= 0 && idx < len(segments) {
					rawValue = segments[idx]
				}
			}
		} else if field.Pattern != "" {
			// Regex extraction
			if re, err := regexp.Compile(field.Pattern); err == nil {
				if matches := re.FindStringSubmatch(topic); len(matches) > 1 {
					rawValue = matches[1]
				}
			}
		}

		// Skip empty values
		if rawValue == "" {
			continue
		}

		// Type conversion
		result[field.Field] = convertValue(rawValue, field.Type)
	}

	return result
}

func convertValue(value, fieldType string) any {
	switch fieldType {
	case "int":
		if v, err := strconv.Atoi(value); err == nil {
			return v
		}
	case "float":
		if v, err := strconv.ParseFloat(value, 64); err == nil {
			return v
		}
	case "bool":
		if v, err := strconv.ParseBool(value); err == nil {
			return v
		}
	}
	return value // Default to string
}
