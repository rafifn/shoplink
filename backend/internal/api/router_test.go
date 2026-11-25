package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// testRouter has one JSON route, so the content-type rule can be tested.
func testRouter() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/echo", echoHandler)
	mux.HandleFunc("GET /api/thing", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"ok": "yes"})
	})
	mux.HandleFunc("DELETE /api/thing", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	return jsonRouting(mux)
}

func send(t *testing.T, h http.Handler, method, path, contentType, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req := httptest.NewRequestWithContext(t.Context(), method, path, r)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestUnknownPathIsJSON404(t *testing.T) {
	for _, path := range []string{"/nope", "/api/nope", "/api/thing/extra"} {
		rec := send(t, testRouter(), http.MethodGet, path, "", "")
		if rec.Code != http.StatusNotFound {
			t.Fatalf("GET %s: status = %d, want 404", path, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("GET %s: Content-Type = %q, want application/json", path, ct)
		}
		assertErrorBody(t, rec.Body.Bytes(), "not_found")
	}
}

func TestWrongMethodIsJSON405(t *testing.T) {
	rec := send(t, testRouter(), http.MethodPut, "/api/thing", "application/json", `{}`)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405 (body %s)", rec.Code, rec.Body)
	}
	assertErrorBody(t, rec.Body.Bytes(), "method_not_allowed")
	allow := rec.Header().Get("Allow")
	for _, m := range []string{"GET", "DELETE"} {
		if !strings.Contains(allow, m) {
			t.Errorf("Allow = %q, want it to list %s", allow, m)
		}
	}
}

func TestWritesNeedJSONContentType(t *testing.T) {
	tests := []struct {
		contentType string
		wantStatus  int
	}{
		{"application/json", http.StatusOK},
		{"application/json; charset=utf-8", http.StatusOK},
		{"Application/JSON", http.StatusOK},
		{"", http.StatusUnsupportedMediaType},
		{"text/plain", http.StatusUnsupportedMediaType},
		{"application/x-www-form-urlencoded", http.StatusUnsupportedMediaType},
		{"multipart/form-data; boundary=x", http.StatusUnsupportedMediaType},
		{"application/json-patch+json", http.StatusUnsupportedMediaType},
	}
	for _, tt := range tests {
		t.Run(tt.contentType, func(t *testing.T) {
			rec := send(t, testRouter(), http.MethodPost, "/api/echo", tt.contentType, `{"name":"Mug"}`)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tt.wantStatus, rec.Body)
			}
			if tt.wantStatus == http.StatusUnsupportedMediaType {
				assertErrorBody(t, rec.Body.Bytes(), "unsupported_media_type")
			}
		})
	}
}

func TestReadsAndDeletesNeedNoContentType(t *testing.T) {
	if rec := send(t, testRouter(), http.MethodGet, "/api/thing", "", ""); rec.Code != http.StatusOK {
		t.Errorf("GET: status = %d, want 200", rec.Code)
	}
	if rec := send(t, testRouter(), http.MethodDelete, "/api/thing", "", ""); rec.Code != http.StatusNoContent {
		t.Errorf("DELETE: status = %d, want 204", rec.Code)
	}
}

// The real handler from New: unknown routes are JSON, /healthz still works.
func TestNewUsesJSONRouting(t *testing.T) {
	h := newTestHandler(fakeDB{}, io.Discard)

	if rec := send(t, h, http.MethodGet, "/healthz", "", ""); rec.Code != http.StatusOK {
		t.Errorf("GET /healthz: status = %d, want 200", rec.Code)
	}
	rec := send(t, h, http.MethodGet, "/api/no-such-route", "", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /api/no-such-route: status = %d, want 404", rec.Code)
	}
	assertErrorBody(t, rec.Body.Bytes(), "not_found")

	rec = send(t, h, http.MethodPost, "/healthz", "application/json", `{}`)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /healthz: status = %d, want 405", rec.Code)
	}
	assertErrorBody(t, rec.Body.Bytes(), "method_not_allowed")
}
