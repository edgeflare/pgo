package schema

import (
	"context"
	"fmt"
	"sync"

	"github.com/edgeflare/pgo/pkg/pgx"
)

const (
	schemaCacheReloadChannel = "pgrst"
	schemaCacheReloadPayload = "reload schema"
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

// Cache represents a thread-safe cache of database schema information
type Cache struct {
	conn   pgx.Conn
	tables map[string]Table
	mu     sync.RWMutex
}

// getTables returns a map of schema to table names
func getTables(ctx context.Context, conn pgx.Conn, schemaName string) ([]struct {
	Schema string
	Name   string
}, error) {
	rows, err := conn.Query(ctx, `
        SELECT table_schema, table_name
        FROM information_schema.tables
        WHERE table_schema = $1 AND table_type = 'BASE TABLE';
    `, schemaName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []struct {
		Schema string
		Name   string
	}
	for rows.Next() {
		var table struct {
			Schema string
			Name   string
		}
		if err = rows.Scan(&table.Schema, &table.Name); err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return tables, nil
}

// loadSchema queries and returns the tables in the given schema.
func loadSchema(ctx context.Context, conn pgx.Conn, schemaName string) (map[string]Table, error) {
	tables, err := getTables(ctx, conn, schemaName)
	if err != nil {
		return nil, fmt.Errorf("failed to get tables: %w", err)
	}

	cache := make(map[string]Table)
	for _, table := range tables {
		columns, primaryKey, err := getColumns(ctx, conn, table.Schema, table.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to get columns for table %s: %w", table.Name, err)
		}

		foreignKeys, err := getForeignKeys(ctx, conn, table.Schema, table.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to get foreign keys for table %s: %w", table.Name, err)
		}

		tableInfo := Table{
			Schema:      table.Schema,
			Name:        table.Name,
			Columns:     columns,
			PrimaryKey:  primaryKey,
			ForeignKeys: foreignKeys,
		}
		cache[tableInfo.schemaQualifiedName()] = tableInfo
	}

	return cache, nil
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
		if err = rows.Scan(&col.Name, &col.DataType, &col.IsNullable, &col.IsPrimaryKey); err != nil {
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
		if err = rows.Scan(&fk.Column, &fk.ReferencedTable, &fk.ReferencedColumn); err != nil {
			return nil, err
		}
		foreignKeys = append(foreignKeys, fk)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return foreignKeys, nil
}

// getSchemas returns a list of all schema names in the database
func getSchemas(ctx context.Context, conn pgx.Conn) ([]string, error) {
	rows, err := conn.Query(ctx, `
		SELECT schema_name 
		FROM information_schema.schemata
		ORDER BY schema_name;
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var schemas []string
	for rows.Next() {
		var schema string
		if err = rows.Scan(&schema); err != nil {
			return nil, err
		}
		schemas = append(schemas, schema)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return schemas, nil
}

// isSystemSchema returns true if the schema name is a PostgreSQL system schema
func isSystemSchema(schema string) bool {
	systemSchemas := map[string]bool{
		"information_schema": true,
		"pg_catalog":         true,
		"pg_toast":           true,
		"pg_temp_1":          true,
		"pg_toast_temp_1":    true,
	}
	return systemSchemas[schema]
}

// schemaQualifiedName returns the fully qualified table name (schema.table)
func (t *Table) schemaQualifiedName() string {
	return fmt.Sprintf("%s.%s", t.Schema, t.Name)
}

// Load queries and returns table information for the specified schemas.
// If no schema names are provided, it loads information for all non-system schemas.
func Load(ctx context.Context, conn pgx.Conn, schemaNames ...string) (map[string]Table, error) {
	if len(schemaNames) == 0 {
		// Load all non-system schemas
		schemas, err := getSchemas(ctx, conn)
		if err != nil {
			return nil, fmt.Errorf("failed to get schemas: %w", err)
		}

		cache := make(map[string]Table)
		for _, schemaName := range schemas {
			if isSystemSchema(schemaName) {
				continue
			}

			tables, err := loadSchema(ctx, conn, schemaName)
			if err != nil {
				return nil, fmt.Errorf("failed to load schema %s: %w", schemaName, err)
			}

			for fullName, table := range tables {
				cache[fullName] = table
			}
		}

		return cache, nil
	}

	// Load specific schemas
	cache := make(map[string]Table)
	for _, schemaName := range schemaNames {
		tables, err := loadSchema(ctx, conn, schemaName)
		if err != nil {
			return nil, fmt.Errorf("failed to load schema %s: %w", schemaName, err)
		}

		for fullName, table := range tables {
			cache[fullName] = table
		}
	}

	return cache, nil
}

// NewCache creates a new schema cache instance
func NewCache(conn pgx.Conn) *Cache {
	return &Cache{
		conn:   conn,
		tables: make(map[string]Table),
	}
}

// Init initializes the cache and starts listening for schema changes
// Reload can be triggered with `NOTIFY pgrst, 'reload schema';`
// See https://docs.postgrest.org/en/stable/references/schema_cache.html
func (c *Cache) Init(ctx context.Context) error {
	// Initial load of schema
	if err := c.reload(ctx); err != nil {
		return fmt.Errorf("initial schema load failed: %w", err)
	}

	// Listen for schema changes
	notifications, errors := pgx.Listen(ctx, c.conn, schemaCacheReloadChannel)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case err := <-errors:
				fmt.Printf("Schema notification error: %v\n", err)
			case notification := <-notifications:
				// Only reload if payload is "reload schema"
				if notification.Payload == schemaCacheReloadPayload {
					if err := c.reload(ctx); err != nil {
						fmt.Printf("Failed to reload schema: %v\n", err)
					}
				}
			}
		}
	}()

	return nil
}

// reload reloads all schema information
func (c *Cache) reload(ctx context.Context) error {
	tables, err := Load(ctx, c.conn)
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.tables = tables
	c.mu.Unlock()

	return nil
}

// Table returns a table from the cache by its schema-qualified name
func (c *Cache) Table(schemaQualifiedName string) (Table, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	table, exists := c.tables[schemaQualifiedName]
	return table, exists
}

// Tables returns all tables in the cache
func (c *Cache) Tables() map[string]Table {
	c.mu.RLock()
	defer c.mu.RUnlock()
	// Create a copy to prevent concurrent map access
	tables := make(map[string]Table, len(c.tables))
	for k, v := range c.tables {
		tables[k] = v
	}
	return tables
}
