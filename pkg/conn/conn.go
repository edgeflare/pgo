// Package conn provides a thin abstraction layer over pgx/v5 for a more consistent
// API across different connection types (e.g., pgx.Conn, pgxpool.Conn).
//
// This package is primarily intended for internal use within larger frameworks
// and libraries that need to work with both single connections and connection
// pools from pgx/v5. Users typically wouldn't import this package directly
// but would use the specific connection implementations from pgx/v5.
package conn

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Conn defines a common interface for interacting with PostgreSQL connections.
// This interface abstracts away the underlying connection type (e.g., pgx.Conn,
// pgxpool.Conn) allowing for easier use within frameworks and libraries that
// need to work with both single connections and connection pools.
//
// Implementations of this interface must provide methods for executing SQL
// statements (`Exec`), querying data (`Query`), and fetching a single row
// (`QueryRow`).
type Conn interface {
	// Exec executes a SQL statement in the context of the given context 'ctx'.
	// It returns a CommandTag containing details about the executed statement,
	// or an error if there was an issue during execution.
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)

	// Query executes a SQL query in the context of the given context 'ctx'.
	// It returns a Rows object that can be used to iterate over the results
	// of the query, or an error if there was an issue during execution.
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)

	// QueryRow executes a query that is expected to return at most one row.
	// It returns a Row object that can be used to retrieve the single row,
	// or an error if there was an issue during execution.
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}
