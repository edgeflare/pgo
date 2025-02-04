package httputil

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zitadel/oidc/v3/pkg/oidc"
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
// enabled on a table. JWT claims are set using environment variable
// PGO_POSTGRES_OIDC_REQUEST_JWT_CLAIMS, defaulting to "request.jwt.claims" for PostgREST compatibility.
// See https://docs.postgrest.org/en/v12/references/transactions.html#request-headers-cookies-and-jwt-claims for more.
//
// Below example RLS policy allows a user to only select rows where user_id (column of the table) matches the OIDC sub claim
// PostgreSQL:
//
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
