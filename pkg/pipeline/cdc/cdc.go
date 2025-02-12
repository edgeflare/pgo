package cdc

import (
	"fmt"
	"time"

	"github.com/jackc/pglogrepl"
)

// CDC represents a change data capture event in Debezium format.
// Reference: https://debezium.io/documentation/reference/stable/connectors/postgresql.html
type CDC struct {
	Schema struct {
		Type     string  `json:"type"`     // Always "struct"
		Optional bool    `json:"optional"` // Schema optionality
		Name     string  `json:"name"`     // Record name for Kafka Connect
		Fields   []Field `json:"fields"`   // Schema field definitions
	} `json:"schema"`
	Payload struct {
		Before interface{} `json:"before"` // Row data before change (null for INSERT)
		After  interface{} `json:"after"`  // Row data after change (null for DELETE)
		Source struct {
			Version   string `json:"version"`        // Connector version
			Connector string `json:"connector"`      // Always "postgresql"
			Name      string `json:"name"`           // Logical server name
			TsMs      int64  `json:"ts_ms"`          // Timestamp of change
			Snapshot  bool   `json:"snapshot"`       // "true"/"false"/"last"
			Db        string `json:"db"`             // Database name
			Sequence  string `json:"sequence"`       // Only for incremental snapshot
			Schema    string `json:"schema"`         // Schema name
			Table     string `json:"table"`          // Table name
			TxID      int64  `json:"txId"`           // Transaction ID
			Lsn       int64  `json:"lsn"`            // Log Sequence Number
			Xmin      *int64 `json:"xmin,omitempty"` // XID for in-progress transaction
		} `json:"source"`
		Op          string `json:"op"`    // Operation type: c=create, u=update, d=delete, r=read
		TsMs        int64  `json:"ts_ms"` // Processing timestamp
		Transaction *struct {
			ID                  string `json:"id"`
			TotalOrder          int64  `json:"total_order"`
			DataCollectionOrder int64  `json:"data_collection_order"`
		} `json:"transaction,omitempty"`
	} `json:"payload"`
}

// Field represents a schema field in Debezium's format
type Field struct {
	Field    string  `json:"field"`            // Field name
	Type     string  `json:"type"`             // Field type
	Optional bool    `json:"optional"`         // Field optionality
	Name     string  `json:"name,omitempty"`   // Schema name for complex types
	Fields   []Field `json:"fields,omitempty"` // Nested fields for complex types
}

// getDefaultSchema returns the default schema structure for a change event
func GetDefaultSchema() struct {
	Type     string  `json:"type"`
	Optional bool    `json:"optional"`
	Name     string  `json:"name"`
	Fields   []Field `json:"fields"`
} {
	return struct {
		Type     string  `json:"type"`
		Optional bool    `json:"optional"`
		Name     string  `json:"name"`
		Fields   []Field `json:"fields"`
	}{
		Type:     "struct",
		Optional: false,
		Name:     "io.debezium.connector.postgresql.Source",
		Fields: []Field{
			{Field: "before", Type: "struct", Optional: true},
			{Field: "after", Type: "struct", Optional: true},
			{Field: "source", Type: "struct", Optional: false, Fields: []Field{
				{Field: "version", Type: "string", Optional: false},
				{Field: "connector", Type: "string", Optional: false},
				{Field: "name", Type: "string", Optional: false},
				{Field: "ts_ms", Type: "int64", Optional: false},
				{Field: "snapshot", Type: "string", Optional: true},
				{Field: "db", Type: "string", Optional: false},
				{Field: "sequence", Type: "string", Optional: true},
				{Field: "schema", Type: "string", Optional: false},
				{Field: "table", Type: "string", Optional: false},
				{Field: "txId", Type: "int64", Optional: true},
				{Field: "lsn", Type: "int64", Optional: false},
				{Field: "xmin", Type: "int64", Optional: true},
			}},
			{Field: "op", Type: "string", Optional: false},
			{Field: "ts_ms", Type: "int64", Optional: true},
			{Field: "transaction", Type: "struct", Optional: true, Fields: []Field{
				{Field: "id", Type: "string", Optional: false},
				{Field: "total_order", Type: "int64", Optional: false},
				{Field: "data_collection_order", Type: "int64", Optional: false},
			}},
		},
	}
}

// CreateCDCPayloadSource creates a source struct with common fields populated
func CreateCDCPayloadSource(serverName, dbName string, msg interface{}, rel *pglogrepl.RelationMessageV2, lsn int64) struct {
	Version   string `json:"version"`
	Connector string `json:"connector"`
	Name      string `json:"name"`
	TsMs      int64  `json:"ts_ms"`
	Snapshot  bool   `json:"snapshot"`
	Db        string `json:"db"`
	Sequence  string `json:"sequence"`
	Schema    string `json:"schema"`
	Table     string `json:"table"`
	TxID      int64  `json:"txId"`
	Lsn       int64  `json:"lsn"`
	Xmin      *int64 `json:"xmin,omitempty"`
} {
	var txID int64
	switch m := msg.(type) {
	case *pglogrepl.InsertMessageV2:
		txID = int64(m.Xid)
	case *pglogrepl.UpdateMessageV2:
		txID = int64(m.Xid)
	case *pglogrepl.DeleteMessageV2:
		txID = int64(m.Xid)
	case *pglogrepl.TruncateMessageV2:
		txID = int64(m.Xid)
	}

	return struct {
		Version   string `json:"version"`
		Connector string `json:"connector"`
		Name      string `json:"name"`
		TsMs      int64  `json:"ts_ms"`
		Snapshot  bool   `json:"snapshot"`
		Db        string `json:"db"`
		Sequence  string `json:"sequence"`
		Schema    string `json:"schema"`
		Table     string `json:"table"`
		TxID      int64  `json:"txId"`
		Lsn       int64  `json:"lsn"`
		Xmin      *int64 `json:"xmin,omitempty"`
	}{
		Version:   "2.5", // Match your Debezium version
		Connector: "postgresql",
		Name:      serverName,
		TsMs:      time.Now().UnixMilli(),
		Snapshot:  false,
		Db:        dbName,
		Sequence:  fmt.Sprintf("[%d,%d]", lsn, lsn),
		Schema:    rel.Namespace,
		Table:     rel.RelationName,
		TxID:      txID,
		Lsn:       lsn,
	}
}
