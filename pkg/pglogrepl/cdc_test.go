package pglogrepl

import (
	"encoding/json"
	"os"
	"testing"
)

// TestDebeziumConformanceCDC tests the conformance of the CDC struct to the Debezium CDC format.
func TestDebeziumConformanceCDC(t *testing.T) {
	file, err := os.Open("sample.cdc.json")
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	var event CDC
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&event); err != nil {
		t.Fatalf("Failed to decode JSON: %v", err)
	}
}
