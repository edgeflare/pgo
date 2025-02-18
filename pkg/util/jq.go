package util

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var (
	errInvalidInput = errors.New("invalid input or empty path")
	errNoWildcard   = errors.New("no matching elements found for wildcard path")
)

// Jq extracts values from a JSON-like map using a dotted path notation like the jq cli
func Jq(input map[string]any, path string) (any, error) {
	if input == nil || path == "" {
		return nil, errInvalidInput
	}

	// Avoid allocation if no leading dot
	if path[0] == '.' {
		path = path[1:]
	}

	// Preallocate keys slice with estimated capacity
	keys := make([]string, 0, 5) // Most paths are < 5 segments
	start := 0
	for i := 0; i < len(path); i++ {
		if path[i] == '.' {
			if i > start {
				keys = append(keys, path[start:i])
			}
			start = i + 1
		}
	}
	if start < len(path) {
		keys = append(keys, path[start:])
	}

	var current any = input
	for i, key := range keys {
		isLastKey := i == len(keys)-1

		currentMap, ok := current.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("expected map at path segment: %s", key)
		}

		// Fast path for non-array keys
		if !strings.ContainsRune(key, '[') {
			value, exists := currentMap[key]
			if !exists {
				return nil, fmt.Errorf("key not found: %s", key)
			}
			current = value
			continue
		}

		// Handle array notation
		arrayKey, indexStr, err := splitKeyAndIndex(key)
		if err != nil {
			return nil, err
		}

		array, ok := currentMap[arrayKey].([]any)
		if !ok {
			return nil, fmt.Errorf("expected array at key: %s", arrayKey)
		}

		// Handle wildcards
		if indexStr == "*" || indexStr == "" {
			if isLastKey {
				return array, nil
			}
			return handleWildcard(array, keys[i+1:])
		}

		// Parse index
		index, err := strconv.Atoi(indexStr)
		if err != nil || index < 0 || index >= len(array) {
			return nil, fmt.Errorf("invalid index %s at key: %s", indexStr, arrayKey)
		}
		current = array[index]
	}

	return current, nil
}

// splitKeyAndIndex separates a key and its array index with minimal allocations
func splitKeyAndIndex(key string) (string, string, error) {
	start := strings.IndexByte(key, '[')
	end := strings.IndexByte(key, ']')
	if start == -1 || end == -1 || end < start {
		return "", "", fmt.Errorf("malformed array syntax in key: %s", key)
	}
	return key[:start], key[start+1 : end], nil
}

// handleWildcard processes wildcard notation with pre-allocated results slice
func handleWildcard(array []any, remainingKeys []string) (any, error) {
	remainingPath := strings.Join(remainingKeys, ".")
	results := make([]any, 0, len(array)) // Preallocate with capacity

	for _, item := range array {
		if itemMap, ok := item.(map[string]any); ok {
			value, err := Jq(itemMap, remainingPath)
			if err == nil {
				switch v := value.(type) {
				case []any:
					results = append(results, v...)
				default:
					results = append(results, v)
				}
			}
		}
	}

	if len(results) == 0 {
		return nil, errNoWildcard
	}
	return results, nil
}
