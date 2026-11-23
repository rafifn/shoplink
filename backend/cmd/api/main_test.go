package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/rafifn/shoplink/backend/internal/config"
)

const frontend = "http://localhost:5173"

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

// startServer runs serve on a random local port until ctx ends. It returns the
// base URL and a channel that receives serve's result.
func startServer(t *testing.T, ctx context.Context, dbURL string) (string, <-chan error) {
	t.Helper()
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	cfg := config.Config{DatabaseURL: dbURL, Port: "0", FrontendOrigin: frontend}
	go func() { done <- serve(ctx, cfg, ln, quiet) }()
	return "http://" + ln.Addr().String(), done
}

// result is what a test needs from a response, read in full so the body is closed.
type result struct {
	status int
	header http.Header
	body   []byte
}

func fetchHealthz(base string) (result, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, base+"/healthz", nil)
	if err != nil {
		return result{}, err
	}
	req.Header.Set("Origin", frontend)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return result{}, err
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	return result{resp.StatusCode, resp.Header, body}, err
}

func getHealthz(t *testing.T, base string) result {
	t.Helper()
	res, err := fetchHealthz(base)
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	return res
}

func waitStopped(t *testing.T, done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve returned %v, want a clean shutdown", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down within 5s")
	}
}

func localDatabaseURL(t *testing.T) string {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL is not set; run the integration tests with `make check`")
	}
	return url
}

// Integration: the real server against the Docker Postgres.
func TestHealthzAgainstDatabase(t *testing.T) {
	dbURL := localDatabaseURL(t)
	ctx, cancel := context.WithCancel(context.Background())
	base, done := startServer(t, ctx, dbURL)

	res := getHealthz(t, base)
	if res.status != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", res.status, res.body)
	}
	var got map[string]string
	if err := json.Unmarshal(res.body, &got); err != nil || got["status"] != "ok" {
		t.Fatalf(`body = %s, want {"status":"ok"}`, res.body)
	}
	if got := res.header.Get("Access-Control-Allow-Origin"); got != frontend {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, frontend)
	}

	cancel()
	waitStopped(t, done)
}

// Integration: nothing listens on port 1, so the ping fails and healthz says so.
func TestHealthzWhenDatabaseUnreachable(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	base, done := startServer(t, ctx, "postgres://shoplink:secret@127.0.0.1:1/shoplink?sslmode=disable&connect_timeout=1")

	res := getHealthz(t, base)
	if res.status != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 (body %s)", res.status, res.body)
	}
	var got struct {
		Error struct{ Code, Message string } `json:"error"`
	}
	if err := json.Unmarshal(res.body, &got); err != nil || got.Error.Code != "db_unavailable" || got.Error.Message == "" {
		t.Fatalf("body = %s, want the standard error shape with code db_unavailable", res.body)
	}

	cancel()
	waitStopped(t, done)
}

// Graceful shutdown: SIGTERM (docker stop) and SIGINT (Ctrl-C) both stop the
// server cleanly through the same context main uses.
func TestShutdownOnSignal(t *testing.T) {
	dbURL := localDatabaseURL(t)
	for _, sig := range []syscall.Signal{syscall.SIGTERM, syscall.SIGINT} {
		t.Run(sig.String(), func(t *testing.T) {
			ctx, stop := shutdownContext()
			defer stop()
			base, done := startServer(t, ctx, dbURL)

			if res := getHealthz(t, base); res.status != http.StatusOK {
				t.Fatalf("status = %d before the signal, want 200 (body %s)", res.status, res.body)
			}
			if err := syscall.Kill(syscall.Getpid(), sig); err != nil {
				t.Fatal(err)
			}
			waitStopped(t, done)

			_, err := fetchHealthz(base)
			if err == nil || !errors.Is(err, syscall.ECONNREFUSED) {
				t.Errorf("GET after shutdown: err = %v, want connection refused", err)
			}
		})
	}
}
