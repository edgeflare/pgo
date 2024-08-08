package handlers

import (
	"net/http"

	"github.com/edgeflare/pgo"
)

// HealthHandler returns the request ID as a plain text response with a 200 OK status code.
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	requestID := r.Context().Value(pgo.RequestIDCtxKey).(string)
	if requestID == "" {
		http.Error(w, "Request ID not found", http.StatusInternalServerError)
		return
	}

	pgo.RespondText(w, http.StatusOK, requestID)
}
