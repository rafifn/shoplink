// Package dbmigrate applies SQL migrations with goose, used by the API at
// startup in production (MIGRATE_ON_START=true). Locally, `make migrate` runs
// the same files through the goose command instead.
package dbmigrate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

// Up applies every pending migration in fsys to the database at dbURL, in
// order, each in its own transaction. A Postgres advisory lock makes a second
// API instance wait instead of migrating at the same time. It returns an error
// naming the failing file if any migration fails; earlier ones stay applied.
func Up(ctx context.Context, dbURL string, fsys fs.FS, logger *slog.Logger) error {
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		return fmt.Errorf("migrations: open database: %w", err) // pgx redacts the password
	}
	defer func() { _ = db.Close() }()

	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return fmt.Errorf("migrations: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, fsys, goose.WithSessionLocker(locker))
	if err != nil {
		return fmt.Errorf("migrations: load files: %w", err)
	}

	results, err := provider.Up(ctx)
	for _, r := range results {
		logger.InfoContext(ctx, "migration applied", "file", r.Source.Path, "duration", r.Duration.String())
	}
	if err != nil {
		var partial *goose.PartialError
		if errors.As(err, &partial) && partial.Failed != nil {
			for _, r := range partial.Applied {
				logger.InfoContext(ctx, "migration applied", "file", r.Source.Path, "duration", r.Duration.String())
			}
			return fmt.Errorf("migrations: %s failed: %w", partial.Failed.Source.Path, partial.Err)
		}
		return fmt.Errorf("migrations: %w", err)
	}
	if len(results) == 0 {
		logger.InfoContext(ctx, "migrations: database is up to date")
	}
	return nil
}
