package dbmigrate

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jackc/pgx/v5"

	"github.com/rafifn/shoplink/backend/migrations"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

// freshSchema creates an empty schema in the local database and returns a
// DATABASE_URL whose search_path points at it, so the migrations run in
// isolation. The schema is dropped when the test ends.
func freshSchema(t *testing.T) (dbURL, schema string) {
	t.Helper()
	base := os.Getenv("DATABASE_URL")
	if base == "" {
		t.Skip("DATABASE_URL is not set; run the integration tests with `make check`")
	}
	schema = fmt.Sprintf("migtest_%d", os.Getpid())
	conn := connect(t, base)
	if _, err := conn.Exec(t.Context(), "drop schema if exists "+schema+" cascade; create schema "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { // t.Context() is already cancelled when cleanups run
		if _, err := conn.Exec(context.Background(), "drop schema if exists "+schema+" cascade"); err != nil {
			t.Errorf("drop test schema: %v", err)
		}
	})

	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	return u.String(), schema
}

func connect(t *testing.T, dbURL string) *pgx.Conn {
	t.Helper()
	conn, err := pgx.Connect(t.Context(), dbURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	return conn
}

func queryString(t *testing.T, conn *pgx.Conn, sql string, args ...any) string {
	t.Helper()
	var s string
	if err := conn.QueryRow(t.Context(), sql, args...).Scan(&s); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return s
}

// latestVersion is the number in front of the newest migration file, e.g. 2 for 00002_money_in_sen.sql.
func latestVersion(t *testing.T) string {
	t.Helper()
	files, err := fs.Glob(migrations.FS, "*.sql")
	if err != nil || len(files) == 0 {
		t.Fatalf("no embedded migrations: %v", err)
	}
	n, err := strconv.Atoi(strings.SplitN(files[len(files)-1], "_", 2)[0])
	if err != nil {
		t.Fatal(err)
	}
	return strconv.Itoa(n)
}

func TestUpAppliesEmbeddedMigrations(t *testing.T) {
	dbURL, schema := freshSchema(t)
	conn := connect(t, dbURL)
	tables := func() string {
		return queryString(t, conn, `select coalesce(string_agg(table_name, ',' order by table_name), '')
			from information_schema.tables where table_schema = $1`, schema)
	}
	version := func() string {
		return queryString(t, conn, "select max(version_id)::text from goose_db_version")
	}

	if err := Up(t.Context(), dbURL, migrations.FS, quiet); err != nil {
		t.Fatalf("Up on an empty schema: %v", err)
	}
	if got, want := tables(), "goose_db_version,products,sessions,users"; got != want {
		t.Fatalf("tables = %q, want %q", got, want)
	}
	if got, want := version(), latestVersion(t); got != want {
		t.Fatalf("version = %s, want the newest migration %s", got, want)
	}

	// A restart with nothing pending is a no-op, not an error.
	if err := Up(t.Context(), dbURL, migrations.FS, quiet); err != nil {
		t.Fatalf("Up with nothing pending: %v", err)
	}
	if got, want := version(), latestVersion(t); got != want {
		t.Fatalf("version after a second Up = %s, want %s", got, want)
	}
}

func TestUpFailsOnBrokenMigration(t *testing.T) {
	dbURL, _ := freshSchema(t)
	broken := fstest.MapFS{
		"00001_ok.sql":     {Data: []byte("-- +goose Up\ncreate table ok (id int);\n-- +goose Down\ndrop table ok;\n")},
		"00002_broken.sql": {Data: []byte("-- +goose Up\ncreate tabel oops (id int);\n-- +goose Down\nselect 1;\n")},
	}

	err := Up(t.Context(), dbURL, broken, quiet)
	if err == nil {
		t.Fatal("Up succeeded with a broken migration")
	}
	if !strings.Contains(err.Error(), "00002_broken.sql") {
		t.Errorf("error should name the failing file: %v", err)
	}
	// The good migration stays applied; the broken one is rolled back.
	if got := queryString(t, connect(t, dbURL), "select max(version_id)::text from goose_db_version"); got != "1" {
		t.Errorf("version after the failure = %s, want 1", got)
	}
}

func TestUpFailsWhenDatabaseUnreachable(t *testing.T) {
	err := Up(t.Context(), "postgres://shoplink:secret@127.0.0.1:1/shoplink?sslmode=disable&connect_timeout=1", migrations.FS, quiet)
	if err == nil {
		t.Fatal("Up succeeded without a database")
	}
	if strings.Contains(err.Error(), "secret") {
		t.Errorf("error leaks the database password: %v", err)
	}
}
