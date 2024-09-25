package pgo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/edgeflare/pgxutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zitadel/oidc/v3/pkg/oidc"
)

var ErrTooManyRows = fmt.Errorf("too many rows")

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

// Select executes a SELECT query and returns the results as a slice of T
func Select[T any](r *http.Request, query string, args []any, scanFn pgx.RowToFunc[T], keepConn ...bool) ([]T, *pgconn.PgError) {
	_, conn, connErr := ConnWithRole(r)
	if connErr != nil {
		return nil, connErr
	}

	if len(keepConn) == 0 || !keepConn[0] {
		defer conn.Release()
	}

	rows, err := pgxutil.Select(r.Context(), conn, query, args, scanFn)
	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok {
			return nil, pgErr
		}
		return nil, &pgconn.PgError{
			Code:    "P0000",
			Message: "Failed to collect rows",
		}
	}

	return rows, nil
}

// SelectAndRespondJSON executes a SELECT query and responds with the results as JSON
func SelectAndRespondJSON[T any](w http.ResponseWriter, r *http.Request, query string, args []any, scanFn pgx.RowToFunc[T]) {
	results, pgErr := Select[T](r, query, args, scanFn)
	if pgErr != nil {
		RespondError(w, PgErrorCodeToHTTPStatus(pgErr.Code), pgErr.Message)
		return
	}
	RespondJSON(w, http.StatusOK, results)
}

// SelectRow executes sql with args on db and returns the T produced by scanFn. The query should return one row. If no
// rows are found returns an error where errors.Is(pgx.ErrNoRows) is true. Returns an error if more than one row is returned.
func SelectRow[T any](r *http.Request, query string, args []any, scanFn pgx.RowToFunc[T], keepConn ...bool) (*T, *pgconn.PgError) {
	_, conn, err := ConnWithRole(r)
	if err != nil {
		return nil, err
	}

	if len(keepConn) == 0 || !keepConn[0] {
		defer conn.Release()
	}

	row, pgErr := pgxutil.SelectRow(r.Context(), conn, query, args, scanFn)
	if pgErr != nil {
		if pgErr, ok := pgErr.(*pgconn.PgError); ok {
			return nil, pgErr
		}
		return nil, &pgconn.PgError{
			Code:    "P0000", // Generic SQLSTATE code
			Message: "Failed to collect rows",
		}
	}
	return &row, nil
}

// SelectRowAndRespondJSON executes a SELECT query and responds with the results as JSON
// It responds with an error if the query or parsing the results fails.
func SelectRowAndRespondJSON[T any](w http.ResponseWriter, r *http.Request, query string, args []any, scanFn pgx.RowToFunc[T]) {
	result, err := SelectRow(r, query, args, scanFn)
	if err != nil {
		if err == ErrTooManyRows {
			RespondError(w, http.StatusInternalServerError, "too many rows returned")
			return
		}
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	RespondJSON(w, http.StatusOK, result)
}

// InsertRow inserts a row into the specified table
func InsertRow(r *http.Request, tableName any, row map[string]any, keepConn ...bool) (pgconn.CommandTag, *pgconn.PgError) {
	_, conn, pgErr := ConnWithRole(r)
	if pgErr != nil {
		return pgconn.CommandTag{}, pgErr
	}

	if len(keepConn) == 0 || !keepConn[0] {
		defer conn.Release()
	}

	err := pgxutil.InsertRow(r.Context(), conn, tableName, row)
	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok {
			return pgconn.CommandTag{}, pgErr
		}
		return pgconn.CommandTag{}, &pgconn.PgError{
			Code: "P0000", // Generic SQLSTATE code
			// Message: "Failed to execute query",
			Message: err.Error(),
		}
	}

	return pgconn.CommandTag{}, nil
}

// InsertRowAndRespondJSON inserts a row into the specified table and responds with the results as JSON
func InsertRowAndRespondJSON(w http.ResponseWriter, r *http.Request, tableName any, row map[string]any) {
	cmdTag, pgErr := InsertRow(r, tableName, row)
	if pgErr != nil {
		RespondError(w, PgErrorCodeToHTTPStatus(pgErr.Code), pgErr.Message)
		return
	}
	RespondJSON(w, http.StatusCreated, cmdTag)
}

// UpdateRow updates a row in the specified table
func Update(r *http.Request, tableName any, row map[string]any, setValues, whereValues map[string]any, keepConn ...bool) (pgconn.CommandTag, *pgconn.PgError) {
	_, conn, pgErr := ConnWithRole(r)
	if pgErr != nil {
		return pgconn.CommandTag{}, pgErr
	}

	if len(keepConn) == 0 || !keepConn[0] {
		defer conn.Release()
	}

	cmdTag, err := pgxutil.Update(r.Context(), conn, tableName, setValues, whereValues)
	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok {
			return pgconn.CommandTag{}, pgErr
		}
		return pgconn.CommandTag{}, &pgconn.PgError{
			Code: "P0000", // Generic SQLSTATE code
			// Message: "Failed to execute query",
			Message: err.Error(),
		}
	}
	return cmdTag, nil
}

// UpdateAndRespondJSON updates a row in the specified table and responds with the results as JSON
func UpdateAndRespondJSON(w http.ResponseWriter, r *http.Request, tableName any, row map[string]any, setValues, whereValues map[string]any) {
	ct, pgErr := Update(r, tableName, row, setValues, whereValues)
	if pgErr != nil {
		RespondError(w, PgErrorCodeToHTTPStatus(pgErr.Code), pgErr.Message)
		return
	}
	RespondJSON(w, http.StatusOK, ct)
}

// ExecRow executes SQL with args. It returns an error unless exactly one row is affected.
func ExecRow(r *http.Request, sql string, args []any, keepConn ...bool) (pgconn.CommandTag, error) {
	_, conn, pgErr := ConnWithRole(r)
	if pgErr != nil {
		return pgconn.CommandTag{}, pgErr
	}
	if len(keepConn) == 0 || !keepConn[0] {
		defer conn.Release()
	}

	ct, err := pgxutil.ExecRow(r.Context(), conn, sql, args...)
	if err != nil {
		fmt.Println(err)
		return ct, err
	}
	return ct, nil
}

// ExecRowAndRespondJSON executes SQL with args and responds with the results as JSON
func ExecRowAndRespondJSON(w http.ResponseWriter, r *http.Request, sql string, args ...any) {
	ct, err := ExecRow(r, sql, args)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	RespondJSON(w, http.StatusOK, ct)
}

// RowMap takes a struct and returns a map with non-zero fields
func RowMap(v interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	rv := reflect.ValueOf(v)
	rt := reflect.TypeOf(v)

	// Ensure the input is a struct
	if rv.Kind() != reflect.Struct {
		panic("RowMap: expected a struct")
	}

	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		value := rv.Field(i)

		// Skip unexported fields
		if field.PkgPath != "" {
			continue
		}

		// Check if the field has a JSON tag
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" {
			jsonTag = field.Name
		} else {
			jsonTag = strings.Split(jsonTag, ",")[0]
		}

		// Check if the value is zero
		if isZero(value) {
			continue
		}

		// Add to result map
		result[jsonTag] = value.Interface()
	}

	return result
}

// Checks if a reflect.Value is zero
func isZero(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		return v.String() == ""
	case reflect.Slice, reflect.Map, reflect.Chan:
		return v.Len() == 0
	case reflect.Struct:
		// Handle time.Time zero value
		if v.Type() == reflect.TypeOf(time.Time{}) {
			return v.Interface().(time.Time).IsZero()
		}
		// Handle uuid.UUID zero value
		if v.Type() == reflect.TypeOf(uuid.UUID{}) {
			return v.Interface().(uuid.UUID) == uuid.Nil
		}
		// Handle pgtype.CIDR zero value
		if v.Type() == reflect.TypeOf(pgtype.InetCodec{}) {
			cidr := v.Interface().(pgtype.Type)
			return cidr.OID != pgtype.InetOID
		}
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Bool:
		return !v.Bool()
	}
	return false
}

// MapPgErrorCodeToHTTPStatus maps PostgreSQL error codes to HTTP status codes
func PgErrorCodeToHTTPStatus(code string) int {
	switch code {
	case "28000": // Invalid authorization specification
		return http.StatusUnauthorized
	case "08003": // Connection does not exist
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}
