package pgx

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// InsertRow inserts a new record into the specified table using the provided JSON payload.
// If no schema is provided, it defaults to the "public" schema.
func InsertRow(ctx context.Context, conn Conn, tableName string, jsonData []byte, schema ...string) error {
	// Determine the schema to use (default to "public")
	schemaName := "public"
	if len(schema) > 0 && schema[0] != "" {
		schemaName = schema[0]
	}

	// Parse the JSON data into a map
	var row map[string]interface{}
	if err := json.Unmarshal(jsonData, &row); err != nil {
		return fmt.Errorf("failed to parse JSON data: %w", err)
	}

	// Prepare the column names and placeholders for the SQL query
	var columns []string
	var placeholders []string
	var values []interface{}

	i := 1
	for key, value := range row {
		columns = append(columns, pgx.Identifier{key}.Sanitize())
		placeholders = append(placeholders, fmt.Sprintf("$%d", i))
		values = append(values, value)
		i++
	}

	// Construct the SQL query
	query := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		pgx.Identifier{schemaName, tableName}.Sanitize(),
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
	)

	// Execute the query
	_, err := conn.Exec(ctx, query, values...)
	if err != nil {
		return fmt.Errorf("failed to insert record: %w", err)
	}

	return nil
}
