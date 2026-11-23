// Package api is the HTTP layer of the ShopLink API: routes, handlers, and
// the JSON response helpers every handler uses.
package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

// Pinger checks that the database is reachable. *pgxpool.Pool satisfies it.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Options are the dependencies and settings for New.
type Options struct {
	DB     Pinger
	Logger *slog.Logger
	// FrontendOrigin is the only origin CORS allows; empty turns CORS off.
	FrontendOrigin string
	// PingTimeout bounds the database check in /healthz. Defaults to 2s.
	PingTimeout time.Duration
}

// New returns the API's HTTP handler.
func New(opts Options) http.Handler {
	if opts.PingTimeout == 0 {
		opts.PingTimeout = 2 * time.Second
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthz(opts.DB, opts.Logger, opts.PingTimeout))

	return cors(opts.FrontendOrigin, mux)
}
