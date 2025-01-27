package httputil

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zitadel/oidc/v3/pkg/oidc"
)

var (
	ErrTooManyRows        = errors.New("too many rows")
	ErrPoolNotInitialized = errors.New("default pool not initialized")
	// defaultPool           *pgxpool.Pool
)

// Conn retrieves the OIDC user and a pgxpool.Conn from the request context.
// It returns an error if the user or connection is not found in the context.
// Currently it only supports OIDC users. But the authZ middleware chain works, and error occurs here.
func Conn(r *http.Request) (*oidc.IntrospectionResponse, *pgxpool.Conn, *pgconn.PgError) {
	// TODO: Add support for Basic Auth
	// basicAuthUser := r.Context().Value(pgo.BasicAuthCtxKey).(string)
	user, ok := OIDCUser(r)
	if !ok || !user.Active {
		return nil, nil, &pgconn.PgError{
			Code:    "28000", // SQLSTATE for invalid authorization specification
			Message: "User not found in context",
		}
	}

	conn, ok := r.Context().Value(PgConnCtxKey).(*pgxpool.Conn)
	if !ok || conn == nil {
		return nil, nil, &pgconn.PgError{
			Code:    "08003", // SQLSTATE for connection does not exist
			Message: "Failed to get connection from context",
		}
	}

	return user, conn, nil
}

// ConnWithRole retrieves the OIDC user, a pgxpool.Conn, and checks for a role
// from the request context. It's designed for use with Row Level Security (RLS)
// enabled on a table.
//
// This function accomplishes the following:
//  1. Calls Conn(r) to retrieve the OIDC user and a pgxpool.Conn from the context.
//  2. Attempts to retrieve the role value from the context using PgRoleCtxKey.
//  3. Marshals the user's claims to a JSON string.
//  4. Escapes the JSON string for safe insertion into the SQL query.
//  5. Constructs a combined query that sets both the role and request claims.
//     - The role is set using the retrieved value from the context.
//     - The request claims are set using either the environment variable
//     PGO_POSTGRES_OIDC_REQUEST_JWT_CLAIMS (if set) or a default value
//     "request.jwt.claims" (aligned with PostgREST behavior).
//     https://docs.postgrest.org/en/v12/references/transactions.html#request-headers-cookies-and-jwt-claims
//  6. Executes the combined query on the connection.
//  7. Handles potential errors:
//     - If Conn(r) returns an error, it's propagated directly.
//     - If the role is not found in the context, a custom PgError is returned.
//     - If marshalling claims or executing the query fails, a custom PgError
//     is returned with appropriate details.
//
// ConnWithRole is useful for scenarios where RLS policies rely on user claims
// to restrict access to specific rows.
//
// Below example RLS policy allows a user to only select rows where user_id (column of the table) matches the OIDC sub claim
// PostgreSQL:
// ALTER TABLE wallets ENABLE ROW LEVEL SECURITY;
// ALTER TABLE wallets FORCE ROW LEVEL SECURITY;
// DROP POLICY IF EXISTS select_own ON wallets;
// CREATE POLICY select_own ON wallets FOR
// SELECT USING (user_id = (current_setting('request.jwt.claims', true)::json->>'sub')::TEXT);
// ALTER POLICY select_own ON wallets TO authn;
func ConnWithRole(r *http.Request) (*oidc.IntrospectionResponse, *pgxpool.Conn, *pgconn.PgError) {
	user, conn, pgErr := Conn(r)
	if pgErr != nil {
		return nil, nil, pgErr
	}

	role, ok := r.Context().Value(PgRoleCtxKey).(string)
	if !ok {
		return nil, nil, &pgconn.PgError{
			Code:    "28000",
			Message: "Role not found in context",
		}
	}

	claimsJSON, err := json.Marshal(user.Claims)
	if err != nil {
		return nil, nil, &pgconn.PgError{
			Code:    "28000",
			Message: fmt.Sprintf("Failed to marshal claims: %v", err),
		}
	}
	escapedClaimsJSON := strings.ReplaceAll(string(claimsJSON), "'", "''")

	setRoleQuery := fmt.Sprintf("SET ROLE %s;", role)
	reqClaims, ok := os.LookupEnv("PGO_POSTGRES_OIDC_REQUEST_JWT_CLAIMS")
	if !ok {
		reqClaims = "request.jwt.claims"
	}
	setReqClaimsQuery := fmt.Sprintf("SET %s TO '%s';", reqClaims, escapedClaimsJSON)
	combinedQuery := setRoleQuery + setReqClaimsQuery

	_, execErr := conn.Exec(context.Background(), combinedQuery)
	if execErr != nil {
		conn.Release()
		if pgErr, ok := execErr.(*pgconn.PgError); ok {
			return nil, nil, pgErr
		}
		return nil, nil, &pgconn.PgError{
			Code:    "P0000", // Generic SQLSTATE code
			Message: "Failed to set role and claims",
		}
	}

	return user, conn, nil
}
