package rag

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestEnsureTableConfig(t *testing.T) {
	ctx := context.Background()
	conn, err := setupTestDatabase(t)
	if err != nil {
		t.Fatalf("Failed to set up test database: %v", err)
	}
	defer conn.Close(ctx)

	c, err := NewClient(conn, Config{
		TableName:          "test_table",
		Dimensions:         3072,
		TablePrimaryKeyCol: "id",
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Ensure the test table is dropped before and after the test
	dropTable := func() {
		_, err := conn.Exec(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", c.Config.TableName))
		if err != nil {
			t.Fatalf("Failed to drop test table: %v", err)
		}
	}
	dropTable()
	defer dropTable()

	// Test case 1: Ensure column is added when the table doesn't exist
	t.Run("AddColumnWhenTableNotExists", func(t *testing.T) {
		err = c.ensureTableConfig(ctx)
		if err != nil {
			t.Fatalf("Failed to ensure embedding column: %v", err)
		}

		// Verify that the table was created with the correct columns
		var tableExists bool
		err = conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name=$1)", c.Config.TableName).Scan(&tableExists)
		if err != nil {
			t.Fatalf("Failed to check if table exists: %v", err)
		}
		if !tableExists {
			t.Errorf("Table was not created")
		}

		// Verify that the table has the correct primary key column
		var primaryKeyCol string
		err = conn.QueryRow(ctx, "SELECT column_name FROM information_schema.columns WHERE table_name=$1 AND column_name='id'", c.Config.TableName).Scan(&primaryKeyCol)
		if err != nil {
			t.Fatalf("Failed to check if primary key column exists: %v", err)
		}
		if primaryKeyCol != c.Config.TablePrimaryKeyCol {
			t.Errorf("Primary key column was not added correctly")
		}

		// Verify that the embedding column exists
		var columnExists bool
		err = conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name=$1 AND column_name='embedding')", c.Config.TableName).Scan(&columnExists)
		if err != nil {
			t.Fatalf("Failed to check if column exists: %v", err)
		}
		if !columnExists {
			t.Errorf("Embedding column was not added")
		}

		// Verify that the table has the correct content column
		var contentCol string
		err = conn.QueryRow(ctx, "SELECT column_name FROM information_schema.columns WHERE table_name=$1 AND column_name='content'", c.Config.TableName).Scan(&contentCol)
		if err != nil {
			t.Fatalf("Failed to check if content column exists: %v", err)
		}
		if contentCol != "content" {
			t.Errorf("Content column was not added correctly")
		}
	})

	// Test case 2: Ensure no error when column already exists
	t.Run("NoErrorWhenColumnExists", func(t *testing.T) {
		err = c.ensureTableConfig(ctx)
		if err != nil {
			t.Fatalf("Unexpected error when ensuring existing column: %v", err)
		}
	})
}

func setupTestDatabase(t *testing.T) (*pgx.Conn, error) {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}

	return conn, nil
}
