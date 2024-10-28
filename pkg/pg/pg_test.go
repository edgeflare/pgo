package pg_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/edgeflare/pgo/pkg/pg"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// Compile-time interface compliance checks
var (
	_ pg.Conn = (*pgx.Conn)(nil)
	_ pg.Conn = (*pgxpool.Pool)(nil)
)

var testConnString string

func init() {
	// Use environment variable for test database connection
	testConnString = os.Getenv("TEST_POSTGRES_CONN_STRING")
	if testConnString == "" {
		testConnString = "postgres://postgres:secret@localhost:5432/testdb?sslmode=disable"
	}
}

// TestRunner holds common test resources for all pg package tests
type TestRunner struct {
	ctx  context.Context
	conn pg.Conn
	t    testing.TB
}

// NewTestRunner creates a new test runner instance with a database connection
func NewTestRunner(t testing.TB) *TestRunner {
	ctx := context.Background()
	pm := pg.GetPoolManager()

	// Setup test pool with notice handler
	cfg := pg.PoolConfig{
		Name:        "test_pool",
		ConnString:  testConnString,
		MaxConns:    5,
		MinConns:    1,
		MaxIdleTime: 5 * time.Minute,
		MaxLifetime: 30 * time.Minute,
		HealthCheck: 1 * time.Minute,
		OnNotice: func(_ *pgconn.PgConn, n *pgconn.Notice) {
			t.Logf("PostgreSQL %s: %s", n.Severity, n.Message)
		},
	}

	require.NoError(t, pm.Add(ctx, cfg), "failed to create test pool")

	pool, err := pm.Get("test_pool")
	require.NoError(t, err, "failed to get test pool")

	return &TestRunner{
		conn: pool,
		ctx:  ctx,
		t:    t,
	}
}
