package pgx

import (
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Compile-time interface compliance checks
var (
	_ Conn = (*pgx.Conn)(nil)
	_ Conn = (*pgxpool.Pool)(nil)
)
