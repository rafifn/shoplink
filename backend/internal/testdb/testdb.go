// Package testdb gives integration tests their own freshly migrated database
// schema, so tests can create and change rows freely without touching the dev
// data or each other. Use it only from _test.go files.
//
// Tests are skipped when DATABASE_URL is not set; `make check` sets it.
package testdb

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rafifn/shoplink/backend/internal/dbmigrate"
	"github.com/rafifn/shoplink/backend/migrations"
)

var counter atomic.Int64

// New creates an empty schema, applies every migration to it, and returns a
// pool whose connections use only that schema. The schema is dropped when the
// test ends.
func New(t *testing.T) *pgxpool.Pool {
	t.Helper()
	base := os.Getenv("DATABASE_URL")
	if base == "" {
		t.Skip("DATABASE_URL is not set; run the integration tests with `make check`")
	}
	ctx := context.Background() // t.Context() is cancelled before cleanups run
	schema := fmt.Sprintf("test_%d_%d", os.Getpid(), counter.Add(1))

	admin, err := pgxpool.New(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, "drop schema if exists "+schema+" cascade; create schema "+schema); err != nil {
		admin.Close()
		t.Fatal(err)
	}

	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()

	if err := dbmigrate.Up(ctx, u.String(), migrations.FS, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatalf("migrate test schema: %v", err)
	}
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		pool.Close()
		if _, err := admin.Exec(ctx, "drop schema if exists "+schema+" cascade"); err != nil {
			t.Errorf("drop test schema %s: %v", schema, err)
		}
		admin.Close()
	})
	return pool
}

// CreateUser inserts a seller directly (no password check needed in tests) and returns its ID.
func CreateUser(t *testing.T, pool *pgxpool.Pool, email, shopName string) int64 {
	t.Helper()
	var id int64
	err := pool.QueryRow(context.Background(),
		"insert into users (email, password_hash, shop_name) values ($1, 'not-a-real-hash', $2) returning id",
		email, shopName,
	).Scan(&id)
	if err != nil {
		t.Fatalf("create test user %s: %v", email, err)
	}
	return id
}
