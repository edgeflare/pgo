package pgo

import (
	"encoding/json"
	"net/http"

	"github.com/zitadel/oidc/v3/pkg/oidc"
)

type ContextKey string

const (
	RequestIDCtxKey ContextKey = "RequestID"
	LogEntryCtxKey  ContextKey = "LogEntry"
	OIDCUserCtxKey  ContextKey = "OIDCUser"
	BasicAuthCtxKey ContextKey = "BasicAuth"
	PgConnCtxKey    ContextKey = "PgConn"
	PgRoleCtxKey    ContextKey = "PgRole"
)

// OIDCUser extracts the OIDC user from the request context.
func OIDCUser(r *http.Request) (*oidc.IntrospectionResponse, bool) {
	user, ok := r.Context().Value(OIDCUserCtxKey).(*oidc.IntrospectionResponse)
	if !ok || user == nil {
		return nil, false
	}
	return user, true
}

// BasicAuthUser retrieves the authenticated username from the context.
func BasicAuthUser(r *http.Request) (string, bool) {
	user, ok := r.Context().Value(BasicAuthCtxKey).(string)
	return user, ok
}

// BindOrRespondError decodes the JSON body of an HTTP request, r, into the given destination object, dst.
// If decoding fails, it responds with a 400 Bad Request error.
func BindOrRespondError(r *http.Request, w http.ResponseWriter, dst interface{}) error {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		RespondError(w, http.StatusBadRequest, err.Error())
		return err
	}
	return nil
}

// RespondJSON writes a JSON response with the given status code and data.
func RespondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// RespondText writes a plain text response with the given status code and text content.
func RespondText(w http.ResponseWriter, statusCode int, text string) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(statusCode)
	if _, err := w.Write([]byte(text)); err != nil {
		http.Error(w, "Failed to write response", http.StatusInternalServerError)
	}
}

// RespondHTML writes an HTML response with the given status code and HTML content.
func RespondHTML(w http.ResponseWriter, statusCode int, html string) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(statusCode)
	if _, err := w.Write([]byte(html)); err != nil {
		http.Error(w, "Failed to write response", http.StatusInternalServerError)
	}
}

// RespondBinary writes a binary response with the given status code and data.
func RespondBinary(w http.ResponseWriter, statusCode int, data []byte, contentType string) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(statusCode)
	if _, err := w.Write(data); err != nil {
		http.Error(w, "Failed to write response", http.StatusInternalServerError)
	}
}

// ErrorResponse represents a structured error response.
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// RespondError sends a JSON response with an error code and message.
func RespondError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	errorResponse := ErrorResponse{
		Code:    statusCode,
		Message: message,
	}
	if err := json.NewEncoder(w).Encode(errorResponse); err != nil {
		// Fallback if JSON encoding fails
		http.Error(w, "Failed to encode error response", http.StatusInternalServerError)
	}
}
