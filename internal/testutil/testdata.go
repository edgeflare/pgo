package testutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"

	"github.com/edgeflare/pgo/pkg/pglogrepl"
)

// LoadCDC returns a CDC object in Debezium format
func LoadCDC() (pglogrepl.CDC, error) {
	var cdc pglogrepl.CDC

	// Get the directory containing this file
	_, currentFile, _, _ := runtime.Caller(0)
	dir := filepath.Dir(currentFile)

	data, err := os.ReadFile(filepath.Join(dir, "sample.cdc.json"))
	if err != nil {
		return cdc, err
	}

	err = json.Unmarshal(data, &cdc)
	if err != nil {
		return cdc, err
	}

	return cdc, nil
}
