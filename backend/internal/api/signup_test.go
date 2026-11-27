package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/rafifn/shoplink/backend/internal/store"
	"github.com/rafifn/shoplink/backend/internal/testdb"
)

func newSignupTest(t *testing.T) (http.Handler, *pgxpool.Pool) {
	t.Helper()
	pool := testdb.New(t)
	h := New(Options{DB: pool, Store: store.New(pool), Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), CookieSecure: true})
	return h, pool
}

func postJSON(t *testing.T, h http.Handler, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, strings.NewReader(string(raw)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func validationFields(t *testing.T, rec *httptest.ResponseRecorder) map[string]string {
	t.Helper()
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body %s)", rec.Code, rec.Body)
	}
	assertErrorBody(t, rec.Body.Bytes(), "validation_failed")
	var body struct {
		Error struct {
			Fields map[string]string `json:"fields"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body.Error.Fields
}

func TestSignupCreatesSellerAndLogsIn(t *testing.T) {
	h, pool := newSignupTest(t)

	rec := postJSON(t, h, "/api/auth/signup", map[string]string{
		"email": "  Rafi@Example.COM ", "password": "correct horse", "shopName": "  Toko Rafi ",
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body %s)", rec.Code, rec.Body)
	}
	var got meResponse
	dec := json.NewDecoder(strings.NewReader(rec.Body.String()))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&got); err != nil {
		t.Fatalf("body %s: %v", rec.Body, err)
	}
	if got.ID == 0 || got.Email != "rafi@example.com" || got.ShopName != "Toko Rafi" {
		t.Errorf("signup body = %+v, want the trimmed, lowercased email and trimmed shop name", got)
	}

	// The cookie it sets is a working, secure session.
	var session *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie {
			session = c
		}
	}
	if session == nil || !session.HttpOnly || !session.Secure || session.SameSite != http.SameSiteLaxMode {
		t.Fatalf("session cookie = %+v, want HttpOnly, Secure, SameSite=Lax", session)
	}
	if me := getMe(t, h, session.Value); me.Code != http.StatusOK {
		t.Errorf("GET /api/me with the signup cookie: status %d, want 200", me.Code)
	}

	// Only a bcrypt hash of the password is stored.
	var hash string
	if err := pool.QueryRow(t.Context(), "select password_hash from users where id = $1", got.ID).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(hash, "correct horse") || bcrypt.CompareHashAndPassword([]byte(hash), []byte("correct horse")) != nil {
		t.Errorf("password_hash %q is not a bcrypt hash of the password", hash)
	}
}

func TestSignupValidation(t *testing.T) {
	valid := map[string]string{"email": "a@example.com", "password": "12345678", "shopName": "Toko"}
	with := func(key, value string) map[string]string {
		m := map[string]string{}
		for k, v := range valid {
			m[k] = v
		}
		m[key] = value
		return m
	}

	tests := []struct {
		name      string
		body      map[string]string
		wantField string
	}{
		{"empty email", with("email", ""), "email"},
		{"email without @", with("email", "not-an-email"), "email"},
		{"email with a name part", with("email", "Rafi <rafi@example.com>"), "email"},
		{"password too short", with("password", "1234567"), "password"},
		{"password over 72 bytes", with("password", strings.Repeat("a", 73)), "password"},
		{"empty shop name", with("shopName", ""), "shopName"},
		{"blank shop name", with("shopName", "   "), "shopName"},
		{"shop name over 80 characters", with("shopName", strings.Repeat("é", 81)), "shopName"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, _ := newSignupTest(t)
			fields := validationFields(t, postJSON(t, h, "/api/auth/signup", tt.body))
			if len(fields) != 1 || fields[tt.wantField] == "" {
				t.Errorf("fields = %v, want only a message for %s", fields, tt.wantField)
			}
		})
	}

	t.Run("every problem at once", func(t *testing.T) {
		h, _ := newSignupTest(t)
		fields := validationFields(t, postJSON(t, h, "/api/auth/signup", map[string]string{"email": "x", "password": "x", "shopName": ""}))
		for _, f := range []string{"email", "password", "shopName"} {
			if fields[f] == "" {
				t.Errorf("no message for %s in %v", f, fields)
			}
		}
	})

	t.Run("limits are inclusive", func(t *testing.T) {
		h, _ := newSignupTest(t)
		rec := postJSON(t, h, "/api/auth/signup", map[string]string{
			"email": "edge@example.com", "password": strings.Repeat("a", 72), "shopName": strings.Repeat("é", 80),
		})
		if rec.Code != http.StatusCreated {
			t.Errorf("72-byte password and 80-character shop name: status %d, want 201 (%s)", rec.Code, rec.Body)
		}
	})
}

func TestSignupEmailTaken(t *testing.T) {
	h, pool := newSignupTest(t)
	testdb.CreateUser(t, pool, "taken@example.com", "First")

	rec := postJSON(t, h, "/api/auth/signup", map[string]string{
		"email": "Taken@Example.com", "password": "12345678", "shopName": "Second",
	})

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 (body %s)", rec.Code, rec.Body)
	}
	assertErrorBody(t, rec.Body.Bytes(), "email_taken")
	if len(rec.Result().Cookies()) != 0 {
		t.Error("a failed signup must not set a cookie")
	}
}

// jsonEqual reports whether two JSON documents hold the same value.
func jsonEqual(a, b string) bool {
	var x, y any
	if json.Unmarshal([]byte(a), &x) != nil || json.Unmarshal([]byte(b), &y) != nil {
		return false
	}
	xa, _ := json.Marshal(x)
	ya, _ := json.Marshal(y)
	return string(xa) == string(ya)
}
