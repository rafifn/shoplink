#!/usr/bin/env bash
# Rebuilds the LOCAL database from scratch: deletes the Docker volume, starts a
# fresh Postgres (the init script recreates both users), runs all migrations,
# then loads the seed data. Run with: make db-reset
#
# Usage: scripts/db-reset.sh <DATABASE_URL>
# Refuses to run unless DATABASE_URL points at this machine, so a pasted
# production URL can never be wiped or seeded.
set -euo pipefail
cd "$(dirname "$0")/.."

url=${1:-}

# Host part of postgres://user:pass@HOST:port/db. Anything that doesn't parse
# as a URL leaves the whole string, which then fails the check below.
host=$(sed -E 's#^[a-zA-Z][a-zA-Z0-9+.-]*://([^@/]*@)?(\[[^]]*\]|[^:/?]*).*#\2#' <<<"$url")

case "$host" in
  localhost | 127.0.0.1 | "[::1]" | db) ;;
  *)
    echo "db-reset: refusing to run: DATABASE_URL host is '${host:-<empty>}', not this machine (localhost, 127.0.0.1, [::1], or db)." >&2
    exit 1
    ;;
esac

docker compose down -v
docker compose up -d --wait db
make -s migrate DATABASE_URL="$url"
make -s seed DATABASE_URL="$url"
echo "db-reset: done"
