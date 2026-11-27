// Package api is the HTTP layer of the ShopLink API: routes, handlers, and
// the JSON response helpers every handler uses.
package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/rafifn/shoplink/backend/internal/store"
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
	// Store runs every database query except the health ping.
	Store *store.Store
	// CookieSecure marks the session cookie Secure (HTTPS only).
	CookieSecure bool
}

// New returns the API's HTTP handler.
func New(opts Options) http.Handler {
	if opts.PingTimeout == 0 {
		opts.PingTimeout = 2 * time.Second
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthz(opts.DB, opts.Logger, opts.PingTimeout))
	mux.HandleFunc("GET /api/me", requireSeller(opts.Store, opts.Logger, me(opts.Store, opts.Logger)))
	mux.HandleFunc("POST /api/auth/signup", signup(opts.Store, opts.Logger, opts.CookieSecure))
	mux.HandleFunc("POST /api/auth/login", login(opts.Store, opts.Logger, opts.CookieSecure))
	mux.HandleFunc("POST /api/auth/logout", logout(opts.Store, opts.Logger, opts.CookieSecure))

	// CORS first, so preflights are answered before routing.
	return cors(opts.FrontendOrigin, jsonRouting(mux))
}
