package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// writeJSON sends v as a JSON response with the given status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// The status line is already sent, so an encoding error can't be reported to the client.
	_ = json.NewEncoder(w).Encode(v)
}

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// internalError logs err with the request and sends a 500 that reveals nothing.
func internalError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, err error) {
	logger.ErrorContext(r.Context(), "request failed", "method", r.Method, "path", r.URL.Path, "err", err)
	writeError(w, http.StatusInternalServerError, "internal_error", "Something went wrong on our side. Please try again.")
}

// writeError sends the one error shape the frontend understands:
// {"error": {"code": "...", "message": "..."}}. The code is stable and
// machine-readable; the message is safe to show to users.
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorBody{Error: errorDetail{Code: code, Message: message}})
}
