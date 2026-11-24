// Package migrations holds the SQL migrations and embeds them in the API
// binary, which applies them at startup when MIGRATE_ON_START is set.
// Create new ones with `make migration name=<name>`.
package migrations

import "embed"

// FS contains every migration file in this directory.
//
//go:embed *.sql
var FS embed.FS
