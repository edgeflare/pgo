package schema

import (
	"context"
	"testing"

	"github.com/edgeflare/pgo/internal/testutil/pgtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	ctx := context.Background()
	conn := pgtest.Connect(context.Background(), t)

	// Create test tables
	_, err := conn.Exec(ctx, `
		DROP SCHEMA IF EXISTS test_schema CASCADE;
		CREATE SCHEMA test_schema;
		
		DROP TABLE IF EXISTS test_orders;
		DROP TABLE IF EXISTS test_users;
		DROP TABLE IF EXISTS test_schema.test_products;
		DROP TABLE IF EXISTS test_schema.test_items;
		
		CREATE TABLE test_users (
			id SERIAL PRIMARY KEY,
			username VARCHAR(50) NOT NULL,
			email VARCHAR(100) UNIQUE NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		
		CREATE TABLE test_orders (
			id SERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL,
			amount DECIMAL(10,2) NOT NULL,
			status VARCHAR(20) NOT NULL,
			FOREIGN KEY (user_id) REFERENCES test_users(id)
		);

		CREATE TABLE test_schema.test_products (
			id SERIAL PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			price DECIMAL(10,2) NOT NULL
		);
		
		CREATE TABLE test_schema.test_items (
			id SERIAL PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			price DECIMAL(10,2) NOT NULL
		);
	`)
	require.NoError(t, err)

	// Helper function to verify test_users table
	verifyUsersTable := func(t *testing.T, tables map[string]Table) {
		t.Helper()
		usersTable, exists := tables["public.test_users"]
		require.True(t, exists)
		assert.Equal(t, "public", usersTable.Schema)
		assert.Equal(t, "test_users", usersTable.Name)
		assert.Equal(t, []string{"id"}, usersTable.PrimaryKey)

		expectedUserColumns := map[string]Column{
			"id": {
				Name:         "id",
				DataType:     "integer",
				IsNullable:   false,
				IsPrimaryKey: true,
			},
			"username": {
				Name:         "username",
				DataType:     "character varying",
				IsNullable:   false,
				IsPrimaryKey: false,
			},
			"email": {
				Name:         "email",
				DataType:     "character varying",
				IsNullable:   false,
				IsPrimaryKey: false,
			},
			"created_at": {
				Name:         "created_at",
				DataType:     "timestamp without time zone",
				IsNullable:   true,
				IsPrimaryKey: false,
			},
		}

		assert.Len(t, usersTable.Columns, len(expectedUserColumns))
		for _, col := range usersTable.Columns {
			expected, exists := expectedUserColumns[col.Name]
			require.True(t, exists)
			assert.Equal(t, expected, col)
		}
	}

	// Helper function to verify test_orders table
	verifyOrdersTable := func(t *testing.T, tables map[string]Table) {
		t.Helper()
		ordersTable, exists := tables["public.test_orders"]
		require.True(t, exists)
		assert.Equal(t, "public", ordersTable.Schema)
		assert.Equal(t, "test_orders", ordersTable.Name)
		assert.Equal(t, []string{"id"}, ordersTable.PrimaryKey)

		expectedOrderColumns := map[string]Column{
			"id": {
				Name:         "id",
				DataType:     "integer",
				IsNullable:   false,
				IsPrimaryKey: true,
			},
			"user_id": {
				Name:         "user_id",
				DataType:     "integer",
				IsNullable:   false,
				IsPrimaryKey: false,
			},
			"amount": {
				Name:         "amount",
				DataType:     "numeric",
				IsNullable:   false,
				IsPrimaryKey: false,
			},
			"status": {
				Name:         "status",
				DataType:     "character varying",
				IsNullable:   false,
				IsPrimaryKey: false,
			},
		}

		assert.Len(t, ordersTable.Columns, len(expectedOrderColumns))
		for _, col := range ordersTable.Columns {
			expected, exists := expectedOrderColumns[col.Name]
			require.True(t, exists)
			assert.Equal(t, expected, col)
		}

		require.Len(t, ordersTable.ForeignKeys, 1)
		assert.Equal(t, ForeignKey{
			Column:           "user_id",
			ReferencedTable:  "test_users",
			ReferencedColumn: "id",
		}, ordersTable.ForeignKeys[0])
	}

	// Helper function to verify test_products table
	verifyProductsTable := func(t *testing.T, tables map[string]Table) {
		t.Helper()
		productsTable, exists := tables["test_schema.test_products"]
		require.True(t, exists)
		assert.Equal(t, "test_schema", productsTable.Schema)
		assert.Equal(t, "test_products", productsTable.Name)
		assert.Equal(t, []string{"id"}, productsTable.PrimaryKey)

		expectedProductColumns := map[string]Column{
			"id": {
				Name:         "id",
				DataType:     "integer",
				IsNullable:   false,
				IsPrimaryKey: true,
			},
			"name": {
				Name:         "name",
				DataType:     "character varying",
				IsNullable:   false,
				IsPrimaryKey: false,
			},
			"price": {
				Name:         "price",
				DataType:     "numeric",
				IsNullable:   false,
				IsPrimaryKey: false,
			},
		}

		assert.Len(t, productsTable.Columns, len(expectedProductColumns))
		for _, col := range productsTable.Columns {
			expected, exists := expectedProductColumns[col.Name]
			require.True(t, exists)
			assert.Equal(t, expected, col)
		}
	}

	t.Run("load specific schema with multiple tables", func(t *testing.T) {
		tables, err := Load(ctx, conn, "public")
		require.NoError(t, err)

		// Should find all tables in public schema
		verifyUsersTable(t, tables)
		verifyOrdersTable(t, tables)

		// Should not find test_schema tables
		_, exists := tables["test_schema.test_products"]
		assert.False(t, exists)
		_, exists = tables["test_schema.test_items"]
		assert.False(t, exists)
	})

	t.Run("load multiple specific schemas", func(t *testing.T) {
		tables, err := Load(ctx, conn, "public", "test_schema")
		require.NoError(t, err)

		verifyUsersTable(t, tables)
		verifyOrdersTable(t, tables)
		verifyProductsTable(t, tables)
	})

	t.Run("load all schemas (no schema names provided)", func(t *testing.T) {
		tables, err := Load(ctx, conn)
		require.NoError(t, err)

		verifyUsersTable(t, tables)
		verifyOrdersTable(t, tables)
		verifyProductsTable(t, tables)

		// Verify system schemas are not included
		for tableName := range tables {
			assert.NotContains(t, tableName, "information_schema.")
			assert.NotContains(t, tableName, "pg_catalog.")
		}
	})

	t.Run("load schema with invalid schema name", func(t *testing.T) {
		tables, err := Load(ctx, conn, "nonexistent_schema")
		require.NoError(t, err)
		assert.Empty(t, tables)
	})

	t.Run("load schema with context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(ctx)
		cancel() // Cancel immediately
		_, err := Load(ctx, conn, "public")
		assert.Error(t, err)
	})

	t.Run("load multiple schemas with one invalid", func(t *testing.T) {
		tables, err := Load(ctx, conn, "public", "nonexistent_schema")
		require.NoError(t, err)

		// Should still get public schema tables
		verifyUsersTable(t, tables)
		verifyOrdersTable(t, tables)
	})

	// Cleanup
	_, err = conn.Exec(ctx, `
		DROP TABLE IF EXISTS test_orders;
		DROP TABLE IF EXISTS test_users;
		DROP SCHEMA IF EXISTS test_schema CASCADE;
	`)
	require.NoError(t, err)
}
