package middleware

import (
	"net/http"
	"strings"
)

// CORSOptions defines configuration for CORS.
type CORSOptions struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	AllowCredentials bool
}

// defaultCORSOptions returns the default CORS options.
func defaultCORSOptions() *CORSOptions {
	return &CORSOptions{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Content-Length", "Accept-Encoding", "X-CSRF-Token", "Authorization", "accept", "origin", "Cache-Control", "X-Requested-With"},
		AllowCredentials: true,
	}
}

// CORSWithOptions creates a CORS middleware with the provided configuration.
// If options is nil, it will use the default CORS settings.
// If options is an empty struct (CORSOptions{}), it will create a middleware with no CORS headers.
func CORSWithOptions(options *CORSOptions) func(http.Handler) http.Handler {
	if options == nil {
		options = defaultCORSOptions()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(options.AllowedOrigins) > 0 {
				w.Header().Set("Access-Control-Allow-Origin", strings.Join(options.AllowedOrigins, ","))
			}
			if len(options.AllowedMethods) > 0 {
				w.Header().Set("Access-Control-Allow-Methods", strings.Join(options.AllowedMethods, ","))
			}
			if len(options.AllowedHeaders) > 0 {
				w.Header().Set("Access-Control-Allow-Headers", strings.Join(options.AllowedHeaders, ","))
			}
			if options.AllowCredentials {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			// Handle preflight request
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
