package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORSWithOptions(t *testing.T) {
	tests := []struct {
		options         *CORSOptions
		expectedHeaders map[string]string
		name            string
		method          string
		expectedStatus  int
	}{
		{
			name:    "default options",
			method:  http.MethodGet,
			options: defaultCORSOptions(),
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin":      "*",
				"Access-Control-Allow-Methods":     "GET,POST,PUT,DELETE,OPTIONS",
				"Access-Control-Allow-Headers":     "Content-Type,Content-Length,Accept-Encoding,X-CSRF-Token,Authorization,accept,origin,Cache-Control,X-Requested-With",
				"Access-Control-Allow-Credentials": "true",
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:   "custom options",
			method: http.MethodGet,
			options: &CORSOptions{
				AllowedOrigins:   []string{"http://example.com"},
				AllowedMethods:   []string{"GET", "POST"},
				AllowedHeaders:   []string{"Content-Type"},
				AllowCredentials: false,
			},
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin":  "http://example.com",
				"Access-Control-Allow-Methods": "GET,POST",
				"Access-Control-Allow-Headers": "Content-Type",
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:            "empty options",
			method:          http.MethodGet,
			options:         &CORSOptions{},
			expectedHeaders: map[string]string{
				// No CORS headers should be set
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:    "preflight request",
			method:  http.MethodOptions,
			options: defaultCORSOptions(),
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin":      "*",
				"Access-Control-Allow-Methods":     "GET,POST,PUT,DELETE,OPTIONS",
				"Access-Control-Allow-Headers":     "Content-Type,Content-Length,Accept-Encoding,X-CSRF-Token,Authorization,accept,origin,Cache-Control,X-Requested-With",
				"Access-Control-Allow-Credentials": "true",
			},
			expectedStatus: http.StatusNoContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest(tt.method, "http://example.com", nil)
			rr := httptest.NewRecorder()

			handler := CORSWithOptions(tt.options)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			handler.ServeHTTP(rr, req)

			for header, expectedValue := range tt.expectedHeaders {
				if value := rr.Header().Get(header); value != expectedValue {
					t.Errorf("header %s: expected %v, got %v", header, expectedValue, value)
				}
			}

			if status := rr.Code; status != tt.expectedStatus {
				t.Errorf("status code: expected %v, got %v", tt.expectedStatus, status)
			}
		})
	}
}
