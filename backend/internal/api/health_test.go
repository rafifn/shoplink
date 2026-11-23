package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakeDB is a Pinger whose Ping returns err, or blocks until the context ends when block is set.
type fakeDB struct {
	err   error
	block bool
}

func (f fakeDB) Ping(ctx context.Context) error {
	if f.block {
		<-ctx.Done()
		return ctx.Err()
	}
	return f.err
}

func newTestHandler(db Pinger) http.Handler {
	return New(Options{
		DB:          db,
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		PingTimeout: 50 * time.Millisecond,
	})
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
	return rec
}

func TestHealthzOK(t *testing.T) {
	rec := get(t, newTestHandler(fakeDB{}), "/healthz")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %v (%q)", err, rec.Body.String())
	}
	if len(body) != 1 || body["status"] != "ok" {
		t.Errorf(`body = %v, want {"status":"ok"}`, body)
	}
}

func TestHealthzDatabaseDown(t *testing.T) {
	tests := []struct {
		name string
		db   fakeDB
	}{
		{"ping fails", fakeDB{err: errors.New("failed to connect to `user=shoplink`: password authentication failed")}},
		{"ping hangs past the timeout", fakeDB{block: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()
			rec := get(t, newTestHandler(tt.db), "/healthz")

			if elapsed := time.Since(start); elapsed > time.Second {
				t.Errorf("healthz took %v; the ping timeout should cut it short", elapsed)
			}
			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("status = %d, want 503", rec.Code)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}
			assertErrorBody(t, rec.Body.Bytes(), "db_unavailable")
			if strings.Contains(rec.Body.String(), "password") || strings.Contains(rec.Body.String(), "shoplink") {
				t.Errorf("body leaks database details: %s", rec.Body.String())
			}
		})
	}
}

// assertErrorBody checks the standard error shape {"error":{"code":"...","message":"..."}}.
func assertErrorBody(t *testing.T, raw []byte, wantCode string) {
	t.Helper()
	var body struct {
		Error *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		t.Fatalf("error body has the wrong shape: %v (%s)", err, raw)
	}
	if body.Error == nil {
		t.Fatalf(`error body has no "error" object: %s`, raw)
	}
	if body.Error.Code != wantCode {
		t.Errorf("error code = %q, want %q", body.Error.Code, wantCode)
	}
	if body.Error.Message == "" {
		t.Errorf("error message is empty")
	}
}
