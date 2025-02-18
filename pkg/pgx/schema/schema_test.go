package schema

import (
	"cmp"
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestSchemaWatch(t *testing.T) {
	ctx := context.Background()
	connString := cmp.Or(os.Getenv("TEST_DATABASE"), "postgres://postgres:secret@localhost:5432/testdb")

	// Connect to create a test table
	pool, err := pgxpool.New(ctx, connString)
	require.NoError(t, err)
	defer pool.Close()

	// Create a test table to trigger a schema change
	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS test_watch (
			id SERIAL PRIMARY KEY,
			name TEXT
		)
	`)
	require.NoError(t, err)
	defer pool.Exec(ctx, "DROP TABLE IF EXISTS test_watch")

	// Initialize cache
	cache, err := NewCache(connString)
	require.NoError(t, err)
	defer cache.Close()

	err = cache.Init(ctx)
	require.NoError(t, err)

	// Create a done channel to stop watching after our test
	done := make(chan bool)
	go func() {
		// Watch for changes
		for tables := range cache.Watch() {
			// Verify we get table data
			require.NotEmpty(t, tables)
			done <- true
			return
		}
	}()

	// Notify schema change
	_, err = pool.Exec(ctx, "NOTIFY "+reloadChannel+", '"+reloadPayload+"'")
	require.NoError(t, err)

	// Wait for the watch to pick up changes
	select {
	case <-done:
		// Test passed
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for schema change notification")
	}
}
