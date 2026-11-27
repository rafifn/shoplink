package api

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/rafifn/shoplink/backend/internal/store"
	"github.com/rafifn/shoplink/backend/internal/testdb"
)

const loginPassword = "correct horse"

// newLoginTest has one seller, user@example.com, with password loginPassword.
func newLoginTest(t *testing.T) (http.Handler, *store.Store, *pgxpool.Pool) {
	t.Helper()
	pool := testdb.New(t)
	st := store.New(pool)
	hash, err := bcrypt.GenerateFromPassword([]byte(loginPassword), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateUser(t.Context(), "user@example.com", string(hash), "Toko User"); err != nil {
		t.Fatal(err)
	}
	h := New(Options{DB: pool, Store: st, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), CookieSecure: true})
	return h, st, pool
}

func sessionFrom(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie {
			return c
		}
	}
	return nil
}

func TestLoginSucceeds(t *testing.T) {
	h, _, _ := newLoginTest(t)

	rec := postJSON(t, h, "/api/auth/login", map[string]string{"email": "  User@Example.COM ", "password": loginPassword})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	if body := rec.Body.String(); !jsonEqual(body, `{"id":1,"email":"user@example.com","shopName":"Toko User"}`) {
		t.Errorf("body = %s", body)
	}
	c := sessionFrom(rec)
	if c == nil || !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode {
		t.Fatalf("session cookie = %+v, want HttpOnly, Secure, SameSite=Lax", c)
	}
	if me := getMe(t, h, c.Value); me.Code != http.StatusOK {
		t.Errorf("GET /api/me with the login cookie: status %d, want 200", me.Code)
	}
}

func TestLoginFailuresLookTheSame(t *testing.T) {
	h, _, _ := newLoginTest(t)

	wrongPassword := postJSON(t, h, "/api/auth/login", map[string]string{"email": "user@example.com", "password": "wrong password"})
	unknownEmail := postJSON(t, h, "/api/auth/login", map[string]string{"email": "nobody@example.com", "password": loginPassword})

	for name, rec := range map[string]*httptest.ResponseRecorder{"wrong password": wrongPassword, "unknown email": unknownEmail} {
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: status = %d, want 401", name, rec.Code)
		}
		assertErrorBody(t, rec.Body.Bytes(), "invalid_credentials")
		if sessionFrom(rec) != nil {
			t.Errorf("%s: a failed login set a session cookie", name)
		}
	}
	if wrongPassword.Body.String() != unknownEmail.Body.String() {
		t.Errorf("the two failures differ, which tells an attacker which emails have accounts:\n%s\n%s", wrongPassword.Body, unknownEmail.Body)
	}
}

// An unknown email still costs one bcrypt comparison, so response time
// doesn't reveal whether the account exists either.
func TestLoginUnknownEmailStillComparesAHash(t *testing.T) {
	h, _, _ := newLoginTest(t)
	calls := 0
	real := compareHash
	compareHash = func(hash, password []byte) error {
		calls++
		return real(hash, password)
	}
	t.Cleanup(func() { compareHash = real })

	postJSON(t, h, "/api/auth/login", map[string]string{"email": "nobody@example.com", "password": loginPassword})
	if calls != 1 {
		t.Errorf("unknown email: %d bcrypt comparisons, want 1", calls)
	}
	calls = 0
	postJSON(t, h, "/api/auth/login", map[string]string{"email": "user@example.com", "password": "wrong password"})
	if calls != 1 {
		t.Errorf("wrong password: %d bcrypt comparisons, want 1", calls)
	}
}

func TestLoginValidation(t *testing.T) {
	h, _, _ := newLoginTest(t)

	fields := validationFields(t, postJSON(t, h, "/api/auth/login", map[string]string{"email": " ", "password": ""}))

	if fields["email"] == "" || fields["password"] == "" || len(fields) != 2 {
		t.Errorf("fields = %v, want messages for email and password", fields)
	}
}

func postLogout(t *testing.T, h http.Handler, cookie string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/auth/logout", nil)
	req.Header.Set("Content-Type", "application/json")
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: cookie}) //nolint:gosec // G124: a cookie the test request sends, not one the server sets
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestLogoutEndsTheSession(t *testing.T) {
	h, _, _ := newLoginTest(t)
	login := postJSON(t, h, "/api/auth/login", map[string]string{"email": "user@example.com", "password": loginPassword})
	token := sessionFrom(login).Value

	rec := postLogout(t, h, token)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (body %s)", rec.Code, rec.Body)
	}
	if c := sessionFrom(rec); c == nil || c.MaxAge >= 0 || c.Value != "" {
		t.Errorf("logout should clear the cookie, got %+v", c)
	}
	if me := getMe(t, h, token); me.Code != http.StatusUnauthorized {
		t.Errorf("the old token still works after logout (status %d)", me.Code)
	}
}

func TestLogoutWithoutASessionIsFine(t *testing.T) {
	h, _, _ := newLoginTest(t)
	for name, cookie := range map[string]string{"no cookie": "", "garbage cookie": "not-a-session"} {
		if rec := postLogout(t, h, cookie); rec.Code != http.StatusNoContent {
			t.Errorf("%s: status = %d, want 204", name, rec.Code)
		}
	}
}
