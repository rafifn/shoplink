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
			// The caller went away (a closed tab, or React StrictMode aborting its
			// first request in dev). Nobody is waiting for an answer, and it says
			// nothing about the database. Only the request's own context counts
			// here: if just the ping timeout fired, the database is too slow.
			if r.Context().Err() != nil {
				logger.DebugContext(r.Context(), "healthz: client went away before the database answered")
				return
			}
			// The details go to the log only; callers just learn that the database is down.
			logger.ErrorContext(r.Context(), "healthz: database ping failed", "err", err)
			writeError(w, http.StatusServiceUnavailable, "db_unavailable", "The database is not reachable.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
