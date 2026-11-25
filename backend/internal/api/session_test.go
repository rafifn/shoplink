package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rafifn/shoplink/backend/internal/store"
	"github.com/rafifn/shoplink/backend/internal/testdb"
)

func TestSessionCookie(t *testing.T) {
	expires := time.Now().Add(store.SessionTTL)
	for _, secure := range []bool{true, false} {
		rec := httptest.NewRecorder()
		setSessionCookie(rec, "the-token", expires, secure)

		cookies := rec.Result().Cookies()
		if len(cookies) != 1 {
			t.Fatalf("want one cookie, got %d", len(cookies))
		}
		c := cookies[0]
		if c.Name != sessionCookie || c.Value != "the-token" || c.Path != "/" {
			t.Errorf("cookie = %s=%s path %s", c.Name, c.Value, c.Path)
		}
		if !c.HttpOnly {
			t.Error("cookie must be HttpOnly, so scripts can't read the session")
		}
		if c.SameSite != http.SameSiteLaxMode {
			t.Errorf("SameSite = %v, want Lax", c.SameSite)
		}
		if c.Secure != secure {
			t.Errorf("Secure = %v, want %v", c.Secure, secure)
		}
		if want := int(store.SessionTTL.Seconds()); c.MaxAge < want-60 || c.MaxAge > want {
			t.Errorf("Max-Age = %d, want about %d (30 days)", c.MaxAge, want)
		}
	}
}

func TestClearSessionCookie(t *testing.T) {
	rec := httptest.NewRecorder()
	clearSessionCookie(rec, true)

	c := rec.Result().Cookies()[0]
	if c.Name != sessionCookie || c.Value != "" || c.MaxAge >= 0 || !c.HttpOnly || !c.Secure {
		t.Errorf("clearing cookie = %+v, want an empty, expired, HttpOnly, Secure %s", c, sessionCookie)
	}
}

func newMeTest(t *testing.T) (http.Handler, *store.Store, int64) {
	t.Helper()
	pool := testdb.New(t)
	st := store.New(pool)
	userID := testdb.CreateUser(t, pool, "seller@example.com", "Toko Seller")
	h := New(Options{DB: pool, Store: st, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	return h, st, userID
}

func getMe(t *testing.T, h http.Handler, cookie string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/me", nil)
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: cookie}) //nolint:gosec // G124: a cookie the test request sends, not one the server sets
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestMeReturnsTheLoggedInSeller(t *testing.T) {
	h, st, userID := newMeTest(t)
	token, _, err := st.CreateSession(t.Context(), userID)
	if err != nil {
		t.Fatal(err)
	}

	rec := getMe(t, h, token)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	var got struct {
		ID       int64  `json:"id"`
		Email    string `json:"email"`
		ShopName string `json:"shopName"`
	}
	dec := json.NewDecoder(strings.NewReader(rec.Body.String()))
	dec.DisallowUnknownFields() // never leak password_hash or anything else
	if err := dec.Decode(&got); err != nil {
		t.Fatalf("body %s: %v", rec.Body, err)
	}
	if got.ID != userID || got.Email != "seller@example.com" || got.ShopName != "Toko Seller" {
		t.Errorf("me = %+v", got)
	}
}

func TestMeNeedsAValidSession(t *testing.T) {
	h, st, userID := newMeTest(t)
	loggedOut, _, err := st.CreateSession(t.Context(), userID)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteSession(t.Context(), loggedOut); err != nil {
		t.Fatal(err)
	}

	for name, cookie := range map[string]string{
		"no cookie":         "",
		"garbage cookie":    "not-a-session",
		"logged-out cookie": loggedOut,
	} {
		rec := getMe(t, h, cookie)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: status = %d, want 401", name, rec.Code)
			continue
		}
		assertErrorBody(t, rec.Body.Bytes(), "unauthenticated")
	}
}
