package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// SessionTTL is how long a login lasts.
const SessionTTL = 30 * 24 * time.Hour

// CreateSession starts a login for userID. It returns the token to put in the
// cookie and when it expires. Only the token's SHA-256 hash is stored, so a
// leaked database (or backup) can't be used to log in as anyone.
func (s *Store) CreateSession(ctx context.Context, userID int64) (token string, expiresAt time.Time, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", time.Time{}, fmt.Errorf("session token: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	expiresAt = s.now().Add(SessionTTL)

	_, err = s.pool.Exec(ctx,
		"insert into sessions (id, user_id, expires_at) values ($1, $2, $3)",
		hashToken(token), userID, expiresAt)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("create session: %w", err)
	}
	return token, expiresAt, nil
}

// SessionUserID returns the user a token belongs to. An unknown, tampered or
// expired token gives ErrNotFound; an expired session is deleted on the way.
func (s *Store) SessionUserID(ctx context.Context, token string) (int64, error) {
	if token == "" {
		return 0, ErrNotFound
	}
	id := hashToken(token)

	var userID int64
	var expiresAt time.Time
	err := s.pool.QueryRow(ctx, "select user_id, expires_at from sessions where id = $1", id).Scan(&userID, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("look up session: %w", err)
	}

	if !s.now().Before(expiresAt) {
		if _, err := s.pool.Exec(ctx, "delete from sessions where id = $1", id); err != nil {
			return 0, fmt.Errorf("delete expired session: %w", err)
		}
		return 0, ErrNotFound
	}
	return userID, nil
}

// DeleteSession ends a login. Deleting a session that doesn't exist is not an error.
func (s *Store) DeleteSession(ctx context.Context, token string) error {
	if _, err := s.pool.Exec(ctx, "delete from sessions where id = $1", hashToken(token)); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
