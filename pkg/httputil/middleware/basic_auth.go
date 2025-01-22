package middleware

import (
	"context"
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/edgeflare/pgo/pkg/httputil"
)

// BasicAuthConfig holds the username-password pairs for basic authentication.
type BasicAuthConfig struct {
	Credentials map[string]string
}

// NewBasicAuthCreds creates a new instance of BasicAuthConfig with multiple username/password pairs.
func BasicAuthCreds(credentials map[string]string) *BasicAuthConfig {
	return &BasicAuthConfig{
		Credentials: credentials,
	}
}

// VerifyBasicAuth is a middleware function for basic authentication.
func VerifyBasicAuth(config *BasicAuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
				http.Error(w, "Authorization header missing", http.StatusUnauthorized)
				return
			}

			// Check if the authorization header is in the correct format
			if !strings.HasPrefix(authHeader, "Basic ") {
				http.Error(w, "Invalid authorization format", http.StatusUnauthorized)
				return
			}

			// Decode the base64 encoded credentials
			encodedCredentials := strings.TrimPrefix(authHeader, "Basic ")
			credentials, err := base64.StdEncoding.DecodeString(encodedCredentials)
			if err != nil {
				http.Error(w, "Invalid base64 encoding", http.StatusUnauthorized)
				return
			}

			// Split the credentials into username and password
			creds := strings.SplitN(string(credentials), ":", 2)
			if len(creds) != 2 {
				http.Error(w, "Invalid credentials format", http.StatusUnauthorized)
				return
			}

			username, password := creds[0], creds[1]

			// Verify the credentials
			if validPassword, ok := config.Credentials[username]; !ok || validPassword != password {
				w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
				http.Error(w, "Invalid credentials", http.StatusUnauthorized)
				return
			}

			// Store authenticated user in context
			ctx := context.WithValue(r.Context(), httputil.BasicAuthCtxKey, username)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
