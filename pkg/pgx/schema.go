package pgx

import (
	"context"
	"fmt"
)

// Table represents a database table.
type Table struct {
	Schema      string
	Name        string
	Columns     []Column
	PrimaryKey  []string
	ForeignKeys []ForeignKey
}

// Column represents a column in a table.
type Column struct {
	Name         string
	DataType     string
	IsNullable   bool
	IsPrimaryKey bool
}

// ForeignKey represents a foreign key relationship.
type ForeignKey struct {
	Column           string
	ReferencedTable  string
	ReferencedColumn string
}

// LoadSchema queries and returns the tables in the given schema.
func LoadSchema(ctx context.Context, conn Conn, schemaName string) (map[string]Table, error) {
	cache := make(map[string]Table)

	// Query tables
	rows, err := conn.Query(ctx, `
		SELECT table_schema, table_name 
		FROM information_schema.tables 
		WHERE table_schema = $1 AND table_type = 'BASE TABLE';
	`, schemaName)
	if err != nil {
		return nil, fmt.Errorf("failed to query tables: %w", err)
	}
	defer rows.Close()

	// Process tables
	for rows.Next() {
		var schema, tableName string
		if err := rows.Scan(&schema, &tableName); err != nil {
			return nil, err
		}

		// Fetch columns for the table
		columns, primaryKey, err := getColumns(ctx, conn, schema, tableName)
		if err != nil {
			return nil, err
		}

		// Fetch foreign keys
		foreignKeys, err := getForeignKeys(ctx, conn, schema, tableName)
		if err != nil {
			return nil, err
		}

		cache[tableName] = Table{
			Schema:      schema,
			Name:        tableName,
			Columns:     columns,
			PrimaryKey:  primaryKey,
			ForeignKeys: foreignKeys,
		}
	}

	return cache, nil
}

func getColumns(ctx context.Context, conn Conn, schema, table string) ([]Column, []string, error) {
	rows, err := conn.Query(ctx, `
		SELECT 
			c.column_name, 
			c.data_type, 
			c.is_nullable = 'YES',
			(EXISTS (
				SELECT 1 
				FROM information_schema.table_constraints tc
				JOIN information_schema.key_column_usage kcu 
					ON tc.constraint_name = kcu.constraint_name
					AND tc.table_schema = kcu.table_schema
				WHERE tc.constraint_type = 'PRIMARY KEY'
					AND tc.table_schema = $1 
					AND tc.table_name = $2
					AND kcu.column_name = c.column_name
			)) AS is_primary_key
		FROM information_schema.columns c
		WHERE c.table_schema = $1 AND c.table_name = $2;
	`, schema, table)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var columns []Column
	var primaryKey []string
	for rows.Next() {
		var col Column
		if err := rows.Scan(&col.Name, &col.DataType, &col.IsNullable, &col.IsPrimaryKey); err != nil {
			return nil, nil, err
		}
		columns = append(columns, col)
		if col.IsPrimaryKey {
			primaryKey = append(primaryKey, col.Name)
		}
	}

	return columns, primaryKey, nil
}

func getForeignKeys(ctx context.Context, conn Conn, schema, table string) ([]ForeignKey, error) {
	rows, err := conn.Query(ctx, `
		SELECT 
			kcu.column_name, 
			ccu.table_name AS referenced_table, 
			ccu.column_name AS referenced_column 
		FROM information_schema.table_constraints AS tc 
		JOIN information_schema.key_column_usage AS kcu 
			ON tc.constraint_name = kcu.constraint_name 
			AND tc.table_schema = kcu.table_schema 
		JOIN information_schema.constraint_column_usage AS ccu 
			ON ccu.constraint_name = tc.constraint_name 
			AND ccu.table_schema = tc.table_schema 
		WHERE tc.constraint_type = 'FOREIGN KEY' 
			AND tc.table_schema = $1 
			AND tc.table_name = $2;
	`, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var foreignKeys []ForeignKey
	for rows.Next() {
		var fk ForeignKey
		if err := rows.Scan(&fk.Column, &fk.ReferencedTable, &fk.ReferencedColumn); err != nil {
			return nil, err
		}
		foreignKeys = append(foreignKeys, fk)
	}

	return foreignKeys, nil
}
