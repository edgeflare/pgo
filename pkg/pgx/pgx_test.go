package pgx

import (
	"context"
	"os"
	"testing"

	// pg "github.com/edgeflare/pgo/pkg/pgx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// Compile-time interface compliance checks
var (
	_ Conn = (*pgx.Conn)(nil)
	_ Conn = (*pgxpool.Pool)(nil)
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
	conn Conn
	t    testing.TB
}

// NewTestRunner creates a new test runner instance with a database connection
func NewTestRunner(t testing.TB) *TestRunner {
	ctx := context.Background()

	mgr := NewPoolManager()
	cfg := Pool{
		Name:       "test_pool",
		ConnString: testConnString,
	}

	require.NoError(t, mgr.Add(ctx, cfg), "failed to create test pool")

	pool, err := mgr.Get("test_pool")
	require.NoError(t, err, "failed to get test pool")

	return &TestRunner{
		conn: pool,
		ctx:  ctx,
		t:    t,
	}
}
