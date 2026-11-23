// Command api runs the ShopLink JSON API. Run it locally with `make api`.
//
// Settings come from environment variables (see .env.example): DATABASE_URL
// (required), PORT (default 8080), and FRONTEND_ORIGIN (the one origin CORS
// allows; empty in production, where the app and the API share a domain).
//
// On SIGINT (Ctrl-C) or SIGTERM (docker stop) it stops taking new connections,
// lets in-flight requests finish, closes the database pool, and exits 0.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rafifn/shoplink/backend/internal/api"
	"github.com/rafifn/shoplink/backend/internal/config"
)

// shutdownGrace is how long in-flight requests get after a stop signal. It is
// below Docker's default 10s stop timeout, so Docker never has to kill the process.
const shutdownGrace = 8 * time.Second

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := runUntilSignal(logger); err != nil {
		logger.Error("api stopped", "err", err)
		os.Exit(1)
	}
}

func runUntilSignal(logger *slog.Logger) error {
	ctx, stop := shutdownContext()
	defer stop()
	return run(ctx, os.Getenv, logger)
}

// shutdownContext is cancelled by SIGINT or SIGTERM.
func shutdownContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

// run loads the settings, opens the port, and serves until ctx is cancelled.
func run(ctx context.Context, getenv func(string) string, logger *slog.Logger) error {
	cfg, err := config.Load(getenv)
	if err != nil {
		return err
	}
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", ":"+cfg.Port)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	return serve(ctx, cfg, ln, logger)
}

// serve handles requests on ln until ctx is cancelled, then shuts down
// gracefully. It returns nil after a clean shutdown.
func serve(ctx context.Context, cfg config.Config, ln net.Listener, logger *slog.Logger) error {
	// The pool connects lazily, so the API starts even while the database is
	// down; /healthz reports 503 until it comes back.
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		_ = ln.Close()
		return fmt.Errorf("database settings: %w", err) // pgx redacts the password
	}
	defer pool.Close()

	srv := &http.Server{
		Handler: api.New(api.Options{
			DB:             pool,
			Logger:         logger,
			FrontendOrigin: cfg.FrontendOrigin,
		}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelWarn),
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()
	logger.Info("api listening", "addr", ln.Addr().String(), "frontend_origin", cfg.FrontendOrigin)

	select {
	case err := <-serveErr:
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}

	logger.Info("api shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	logger.Info("api stopped cleanly")
	return nil
}
