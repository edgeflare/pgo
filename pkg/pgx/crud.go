package pgx

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

type queryBuilder struct {
	schema    string
	table     string
	columns   []string
	values    []any
	nextIndex int
}

func newQueryBuilder(tableName string, schema ...string) *queryBuilder {
	schemaName := "public"
	if len(schema) > 0 && schema[0] != "" {
		schemaName = schema[0]
	}
	return &queryBuilder{
		schema:    schemaName,
		table:     tableName,
		nextIndex: 1,
	}
}

func (qb *queryBuilder) addValue(column string, value any) {
	qb.columns = append(qb.columns, column)
	qb.values = append(qb.values, value)
}

func (qb *queryBuilder) placeholder() string {
	placeholder := fmt.Sprintf("$%d", qb.nextIndex)
	qb.nextIndex++
	return placeholder
}

func (qb *queryBuilder) tableIdentifier() string {
	return pgx.Identifier{qb.schema, qb.table}.Sanitize()
}

// InsertRow inserts a new record into the specified table using the provided data.
func InsertRow(ctx context.Context, conn Conn, tableName string, data any, schema ...string) error {
	qb := newQueryBuilder(tableName, schema...)

	dataMap, ok := data.(map[string]any)
	if !ok {
		return fmt.Errorf("data is not in expected format map[string]any")
	}

	var columns, placeholders []string
	for key, value := range dataMap {
		columns = append(columns, pgx.Identifier{key}.Sanitize())
		placeholders = append(placeholders, qb.placeholder())
		qb.addValue("", value)
	}

	query := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		qb.tableIdentifier(),
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
	)

	_, err := conn.Exec(ctx, query, qb.values...)
	if err != nil {
		return fmt.Errorf("failed to insert record: %w", err)
	}
	return nil
}

// UpdateRow updates an existing record in the specified table using the provided data.
func UpdateRow(ctx context.Context, conn Conn, tableName string, data any, where map[string]any, schema ...string) error {
	qb := newQueryBuilder(tableName, schema...)

	var setClauses, whereClauses []string

	dataMap, ok := data.(map[string]any)
	if !ok {
		return fmt.Errorf("data is not in expected format map[string]any")
	}

	// Build SET clause
	for key, value := range dataMap {
		setClauses = append(setClauses, fmt.Sprintf("%s = %s",
			pgx.Identifier{key}.Sanitize(),
			qb.placeholder()))
		qb.addValue("", value)
	}

	// Build WHERE clause
	for key, value := range where {
		whereClauses = append(whereClauses, fmt.Sprintf("%s = %s",
			pgx.Identifier{key}.Sanitize(),
			qb.placeholder()))
		qb.addValue("", value)
	}

	if len(whereClauses) == 0 {
		return fmt.Errorf("no WHERE conditions provided")
	}

	query := fmt.Sprintf(
		"UPDATE %s SET %s WHERE %s",
		qb.tableIdentifier(),
		strings.Join(setClauses, ", "),
		strings.Join(whereClauses, " AND "),
	)

	result, err := conn.Exec(ctx, query, qb.values...)
	if err != nil {
		return fmt.Errorf("failed to update record: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("no rows were updated")
	}

	return nil
}

// convenience functions for JSON input
func InsertRowJSON(ctx context.Context, conn Conn, tableName string, jsonData []byte, schema ...string) error {
	data, err := parseJSON(jsonData)
	if err != nil {
		return err
	}
	return InsertRow(ctx, conn, tableName, data, schema...)
}

func UpdateRowJSON(ctx context.Context, conn Conn, tableName string, jsonData []byte, where map[string]any, schema ...string) error {
	data, err := parseJSON(jsonData)
	if err != nil {
		return err
	}
	return UpdateRow(ctx, conn, tableName, data, where, schema...)
}

func parseJSON(data []byte) (map[string]any, error) {
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON data: %w", err)
	}
	return result, nil
}
