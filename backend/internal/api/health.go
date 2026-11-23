package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

// healthz answers GET /healthz: 200 {"status":"ok"} when the database answers
// a ping within timeout, otherwise 503 with the standard error shape. The deploy
// step and Docker use it to decide whether the API is up.
func healthz(db Pinger, logger *slog.Logger, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")

		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		if err := db.Ping(ctx); err != nil {
			// The details go to the log only; callers just learn that the database is down.
			logger.ErrorContext(r.Context(), "healthz: database ping failed", "err", err)
			writeError(w, http.StatusServiceUnavailable, "db_unavailable", "The database is not reachable.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
