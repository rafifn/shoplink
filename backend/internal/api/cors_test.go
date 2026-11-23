package api

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const frontend = "http://localhost:5173"

func newCORSHandler(origin string) http.Handler {
	return New(Options{
		DB:             fakeDB{},
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		FrontendOrigin: origin,
	})
}

func request(t *testing.T, h http.Handler, method, origin string, header map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), method, "/healthz", nil)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestCORSAllowsFrontendOrigin(t *testing.T) {
	rec := request(t, newCORSHandler(frontend), http.MethodGet, frontend, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != frontend {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, frontend)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("Access-Control-Allow-Credentials = %q, want true", got)
	}
	if !strings.Contains(strings.Join(rec.Header().Values("Vary"), ","), "Origin") {
		t.Errorf("Vary = %q, want it to include Origin", rec.Header().Values("Vary"))
	}
}

func TestCORSIgnoresOtherOrigins(t *testing.T) {
	tests := []struct {
		name       string
		configured string
		origin     string
	}{
		{"different site", frontend, "https://evil.example.com"},
		{"different port", frontend, "http://localhost:5174"},
		{"prefix match", frontend, "http://localhost:51730"},
		{"null origin", frontend, "null"},
		{"no frontend origin configured", "", frontend},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := request(t, newCORSHandler(tt.configured), http.MethodGet, tt.origin, nil)
			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
				t.Errorf("Access-Control-Allow-Origin = %q, want none", got)
			}
			if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "" {
				t.Errorf("Access-Control-Allow-Credentials = %q, want none", got)
			}
		})
	}
}

func TestCORSPreflight(t *testing.T) {
	preflight := map[string]string{
		"Access-Control-Request-Method":  "PATCH",
		"Access-Control-Request-Headers": "content-type",
	}

	t.Run("allowed origin", func(t *testing.T) {
		rec := request(t, newCORSHandler(frontend), http.MethodOptions, frontend, preflight)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", rec.Code)
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != frontend {
			t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, frontend)
		}
		if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
			t.Errorf("Access-Control-Allow-Credentials = %q, want true", got)
		}
		if got := rec.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, "PATCH") {
			t.Errorf("Access-Control-Allow-Methods = %q, want it to include PATCH", got)
		}
		if got := strings.ToLower(rec.Header().Get("Access-Control-Allow-Headers")); !strings.Contains(got, "content-type") {
			t.Errorf("Access-Control-Allow-Headers = %q, want it to include Content-Type", got)
		}
	})

	t.Run("other origin", func(t *testing.T) {
		rec := request(t, newCORSHandler(frontend), http.MethodOptions, "https://evil.example.com", preflight)
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("Access-Control-Allow-Origin = %q, want none", got)
		}
		if got := rec.Header().Get("Access-Control-Allow-Methods"); got != "" {
			t.Errorf("Access-Control-Allow-Methods = %q, want none", got)
		}
	})
}
