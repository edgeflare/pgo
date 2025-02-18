package testutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
)

// LoadJSON reads and unmarshals a JSON file. If target is provided, it attempts to unmarshal the JSON into the target struct.
func LoadJSON(filename string, target ...any) (map[string]any, error) {
	var result map[string]any

	_, currentFile, _, _ := runtime.Caller(0)
	dir := filepath.Dir(currentFile)

	data, err := os.ReadFile(filepath.Join(dir, filename))
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(data, &result)
	if err != nil {
		return nil, err
	}

	if len(target) > 0 && target[0] != nil {
		err = json.Unmarshal(data, target[0])
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}
