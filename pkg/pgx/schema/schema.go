package schema

import (
	"context"
	"fmt"

	"github.com/edgeflare/pgo/pkg/pgx"
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

// Load queries and returns the tables in the given schema.
func Load(ctx context.Context, conn pgx.Conn, schemaName string) (map[string]Table, error) {
	tables, err := getTables(ctx, conn, schemaName)
	if err != nil {
		return nil, fmt.Errorf("failed to get tables: %w", err)
	}

	cache := make(map[string]Table)
	for schema, tableName := range tables {
		columns, primaryKey, err := getColumns(ctx, conn, schema, tableName)
		if err != nil {
			return nil, fmt.Errorf("failed to get columns for table %s: %w", tableName, err)
		}

		foreignKeys, err := getForeignKeys(ctx, conn, schema, tableName)
		if err != nil {
			return nil, fmt.Errorf("failed to get foreign keys for table %s: %w", tableName, err)
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

// getTables returns a map of schema to table names
func getTables(ctx context.Context, conn pgx.Conn, schemaName string) (map[string]string, error) {
	rows, err := conn.Query(ctx, `
        SELECT table_schema, table_name
        FROM information_schema.tables
        WHERE table_schema = $1 AND table_type = 'BASE TABLE';
    `, schemaName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tables := make(map[string]string)
	for rows.Next() {
		var schema, tableName string
		if err := rows.Scan(&schema, &tableName); err != nil {
			return nil, err
		}
		tables[schema] = tableName
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return tables, nil
}

func getColumns(ctx context.Context, conn pgx.Conn, schema, table string) ([]Column, []string, error) {
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
	if err = rows.Err(); err != nil {
		return nil, nil, err
	}

	return columns, primaryKey, nil
}

func getForeignKeys(ctx context.Context, conn pgx.Conn, schema, table string) ([]ForeignKey, error) {
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
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return foreignKeys, nil
}
