package util

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Jq is a helper function to extract a value from a JSON-like map using a path
func Jq(input map[string]interface{}, path string) (string, error) {
	path = strings.TrimPrefix(path, ".")
	keys := strings.Split(path, ".")
	var current interface{} = input

	for _, key := range keys {
		if currentMap, ok := current.(map[string]interface{}); ok {
			if strings.Contains(key, "[") && strings.Contains(key, "]") {
				arrayKey := key[:strings.Index(key, "[")]
				indexStr := key[strings.Index(key, "[")+1 : strings.Index(key, "]")]
				index, err := strconv.Atoi(indexStr)
				if err != nil {
					return "", fmt.Errorf("invalid array index in path: %s", key)
				}
				if array, ok := currentMap[arrayKey].([]interface{}); ok {
					if index < 0 || index >= len(array) {
						return "", fmt.Errorf("index out of range in path: %s", key)
					}
					current = array[index]
				} else {
					return "", fmt.Errorf("expected array at path: %s", key)
				}
			} else {
				current = currentMap[key]
			}
		} else {
			return "", fmt.Errorf("expected map at path: %s", key)
		}
	}

	if role, ok := current.(string); ok {
		return role, nil
	}
	return "", errors.New("role not found or not a string")
}
