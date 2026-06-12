package server

import (
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog/log"
)

// errorResponse is the JSON body returned for any non-2xx API response.
type errorResponse struct {
	Error string `json:"error"`
}

// writeJSON writes v as a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// The status/headers are already sent; just record the failure.
		log.Error().Err(err).Msg("failed to encode JSON response")
	}
}

// writeError writes a JSON error body with the given status code.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}
