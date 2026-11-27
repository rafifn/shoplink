package store

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rafifn/shoplink/backend/internal/testdb"
)

func setup(t *testing.T) (*Store, *pgxpool.Pool, int64) {
	t.Helper()
	pool := testdb.New(t)
	userID := testdb.CreateUser(t, pool, "a@example.com", "Shop A")
	return New(pool), pool, userID
}

func count(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(t.Context(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return n
}

func TestCreateSessionStoresOnlyAHash(t *testing.T) {
	s, pool, userID := setup(t)

	token, expires, err := s.CreateSession(t.Context(), userID)
	if err != nil {
		t.Fatal(err)
	}

	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != 32 {
		t.Fatalf("token %q should be 32 random bytes, base64url without padding (%v)", token, err)
	}
	if other, _, _ := s.CreateSession(t.Context(), userID); other == token {
		t.Fatal("two sessions got the same token")
	}

	sum := sha256.Sum256([]byte(token))
	if n := count(t, pool, "select count(*) from sessions where id = $1 and user_id = $2", hex.EncodeToString(sum[:]), userID); n != 1 {
		t.Errorf("want one session row keyed by the token's SHA-256, got %d", n)
	}
	if n := count(t, pool, "select count(*) from sessions where id = $1", token); n != 0 {
		t.Error("the raw token is stored in the database")
	}

	if d := time.Until(expires); d < SessionTTL-time.Minute || d > SessionTTL+time.Minute {
		t.Errorf("session expires in %v, want about %v", d, SessionTTL)
	}
	if SessionTTL != 30*24*time.Hour {
		t.Errorf("SessionTTL = %v, want 30 days", SessionTTL)
	}
}

func TestSessionUserID(t *testing.T) {
	s, _, userID := setup(t)
	token, _, err := s.CreateSession(t.Context(), userID)
	if err != nil {
		t.Fatal(err)
	}

	got, err := s.SessionUserID(t.Context(), token)
	if err != nil || got != userID {
		t.Fatalf("SessionUserID(valid) = %d, %v; want %d", got, err, userID)
	}

	tampered := token[:len(token)-1] + map[bool]string{true: "A", false: "B"}[token[len(token)-1] != 'A']
	for name, tok := range map[string]string{
		"unknown":  base64.RawURLEncoding.EncodeToString(make([]byte, 32)),
		"tampered": tampered,
		"empty":    "",
	} {
		if _, err := s.SessionUserID(t.Context(), tok); !errors.Is(err, ErrNotFound) {
			t.Errorf("%s token: err = %v, want ErrNotFound", name, err)
		}
	}
}

func TestExpiredSessionIsRejectedAndDeleted(t *testing.T) {
	s, pool, userID := setup(t)
	token, _, err := s.CreateSession(t.Context(), userID)
	if err != nil {
		t.Fatal(err)
	}

	s.now = func() time.Time { return time.Now().Add(SessionTTL + time.Hour) }

	if _, err := s.SessionUserID(t.Context(), token); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expired session: err = %v, want ErrNotFound", err)
	}
	if n := count(t, pool, "select count(*) from sessions where user_id = $1", userID); n != 0 {
		t.Errorf("the expired session row is still there (%d rows)", n)
	}
}

func TestDeleteSession(t *testing.T) {
	s, _, userID := setup(t)
	token, _, err := s.CreateSession(t.Context(), userID)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.DeleteSession(t.Context(), token); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SessionUserID(t.Context(), token); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after delete: err = %v, want ErrNotFound", err)
	}
	if err := s.DeleteSession(t.Context(), token); err != nil {
		t.Errorf("deleting an already deleted session: %v, want nil", err)
	}
}

func TestUserByID(t *testing.T) {
	s, _, userID := setup(t)

	u, err := s.UserByID(t.Context(), userID)
	if err != nil {
		t.Fatal(err)
	}
	if u != (User{ID: userID, Email: "a@example.com", ShopName: "Shop A"}) {
		t.Errorf("UserByID = %+v", u)
	}
	if _, err := s.UserByID(t.Context(), userID+1000); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing user: err = %v, want ErrNotFound", err)
	}
}

func TestCreateUser(t *testing.T) {
	pool := testdb.New(t)
	s := New(pool)

	u, err := s.CreateUser(t.Context(), "new@example.com", "$2a$10$hash", "Toko Baru")
	if err != nil {
		t.Fatal(err)
	}
	if u.ID == 0 || u.Email != "new@example.com" || u.ShopName != "Toko Baru" {
		t.Errorf("CreateUser = %+v", u)
	}
	if _, err := s.CreateUser(t.Context(), "new@example.com", "$2a$10$other", "Another"); !errors.Is(err, ErrEmailTaken) {
		t.Errorf("same email again: err = %v, want ErrEmailTaken", err)
	}
}

func TestUserCredentials(t *testing.T) {
	pool := testdb.New(t)
	s := New(pool)
	created, err := s.CreateUser(t.Context(), "cred@example.com", "$2a$10$the-hash", "Toko Cred")
	if err != nil {
		t.Fatal(err)
	}

	u, hash, err := s.UserCredentials(t.Context(), "cred@example.com")
	if err != nil || u != created || hash != "$2a$10$the-hash" {
		t.Errorf("UserCredentials = %+v, %q, %v", u, hash, err)
	}
	if _, _, err := s.UserCredentials(t.Context(), "missing@example.com"); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing email: err = %v, want ErrNotFound", err)
	}
}
