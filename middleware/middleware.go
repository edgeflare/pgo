package middleware

import (
	"net/http"
)

// Middleware is a function that wraps an HTTP handler.
type Middleware func(http.Handler) http.Handler

// middlewareRegistry manages middleware functions.
var middlewareRegistry []Middleware

// Register adds a new middleware function to the registry.
func Register(m Middleware) {
	middlewareRegistry = append(middlewareRegistry, m)
}

// Apply applies all registered middleware functions to the given handler.
func Apply(h http.Handler) http.Handler {
	for i := len(middlewareRegistry) - 1; i >= 0; i-- {
		h = middlewareRegistry[i](h)
	}
	return h
}
