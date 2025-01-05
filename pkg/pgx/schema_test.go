package pgx

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadSchema(t *testing.T) {
	suite := NewTestRunner(t)

	// Create test tables
	_, err := suite.conn.Exec(suite.ctx, `
		DROP TABLE IF EXISTS test_orders;
		DROP TABLE IF EXISTS test_users;
		
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
	`)
	require.NoError(t, err)

	t.Run("load schema successfully", func(t *testing.T) {
		tables, err := LoadSchema(suite.ctx, suite.conn, "public")
		require.NoError(t, err)

		// Verify test_users table
		usersTable, exists := tables["test_users"]
		require.True(t, exists)
		assert.Equal(t, "public", usersTable.Schema)
		assert.Equal(t, "test_users", usersTable.Name)
		assert.Equal(t, []string{"id"}, usersTable.PrimaryKey)

		// Verify test_users columns
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

		for _, col := range usersTable.Columns {
			expected, exists := expectedUserColumns[col.Name]
			require.True(t, exists)
			assert.Equal(t, expected, col)
		}

		// Verify test_orders table
		ordersTable, exists := tables["test_orders"]
		require.True(t, exists)
		assert.Equal(t, "public", ordersTable.Schema)
		assert.Equal(t, "test_orders", ordersTable.Name)
		assert.Equal(t, []string{"id"}, ordersTable.PrimaryKey)

		// Verify foreign keys
		require.Len(t, ordersTable.ForeignKeys, 1)
		assert.Equal(t, ForeignKey{
			Column:           "user_id",
			ReferencedTable:  "test_users",
			ReferencedColumn: "id",
		}, ordersTable.ForeignKeys[0])
	})

	t.Run("load schema with invalid schema name", func(t *testing.T) {
		tables, err := LoadSchema(suite.ctx, suite.conn, "nonexistent_schema")
		require.NoError(t, err)
		assert.Empty(t, tables)
	})

	t.Run("load schema with context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(suite.ctx)
		cancel() // Cancel immediately
		_, err := LoadSchema(ctx, suite.conn, "public")
		assert.Error(t, err)
	})
}
