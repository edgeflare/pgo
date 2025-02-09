package pgtest

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

// TestDB encapsulates test database functionality
type TestDB struct {
	Config *pgx.ConnConfig
}

// Connect creates a new database connection for testing
func Connect(ctx context.Context, t testing.TB) *pgx.Conn {
	config, err := pgx.ParseConfig(os.Getenv("TEST_DATABASE"))
	require.NoError(t, err)

	config.OnNotice = func(_ *pgconn.PgConn, n *pgconn.Notice) {
		t.Logf("PostgreSQL %s: %s", n.Severity, n.Message)
	}

	conn, err := pgx.ConnectConfig(ctx, config)
	require.NoError(t, err)

	t.Cleanup(func() {
		Close(t, conn)
	})

	return conn
}

// Close safely closes a database connection
func Close(t testing.TB, conn *pgx.Conn) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, conn.Close(ctx))
}

// WithConn provides a database connection to a test function and handles cleanup
func WithConn(t testing.TB, fn func(*pgx.Conn)) {
	ctx := context.Background()
	conn := Connect(ctx, t)
	defer Close(t, conn)
	fn(conn)
}

// ParseConfig returns a test connection config with logging
func ParseConfig(t testing.TB) *pgx.ConnConfig {
	config, err := pgx.ParseConfig(os.Getenv("TEST_DATABASE"))
	require.NoError(t, err)

	config.OnNotice = func(_ *pgconn.PgConn, n *pgconn.Notice) {
		t.Logf("PostgreSQL %s: %s", n.Severity, n.Message)
	}

	return config
}
