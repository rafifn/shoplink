// Package store is the API's only path to the database: every query lives
// here, so the seller_id rule (CLAUDE.md) is enforced in one place.
package store

import (
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound means the row doesn't exist, or isn't visible to the caller.
var ErrNotFound = errors.New("not found")

// Store runs queries against Postgres.
type Store struct {
	pool *pgxpool.Pool
	now  func() time.Time // replaced in tests to move the clock
}

// New returns a Store backed by pool.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool, now: time.Now}
}
