#!/usr/bin/env bash
# Checks the migrations in backend/migrations against a throwaway database:
#   - `make migrate` works on an empty database and again when nothing is pending
#   - the tables and constraints from docs/design.md exist and reject bad rows
#   - migrating all the way down leaves no tables, and migrating up again works
# The throwaway database (shoplink_migtest) is dropped at the end; your dev data is untouched.
# Run with: make db-test (needs the local database from `make db-up`).
set -euo pipefail
cd "$(dirname "$0")/.."

# shellcheck source=/dev/null
set -a; . ./.env; set +a

DB=shoplink_migtest
URL="postgres://shoplink:${SHOPLINK_DB_PASSWORD}@localhost:5432/${DB}?sslmode=disable"

fail=0
pass() { echo "ok   - $1"; }
bad()  { echo "FAIL - $1"; fail=1; }

admin() { docker compose exec -T db psql -X -q -v ON_ERROR_STOP=1 -U postgres -d postgres "$@"; }
sql()   { docker compose exec -T db psql -X -q -v ON_ERROR_STOP=1 -v VERBOSITY=verbose -U postgres -d "$DB" "$@"; }

cleanup() { admin -c "drop database if exists ${DB} with (force)" >/dev/null 2>&1 || true; }
trap cleanup EXIT
cleanup
admin -c "create database ${DB} owner shoplink" >/dev/null

# expect_err <description> <sqlstate> <sql>: the statement must fail with that SQLSTATE.
expect_err() {
  local out
  if out=$(sql -c "$3" 2>&1); then
    bad "$1 (statement succeeded)"
  elif grep -q "ERROR:  $2" <<<"$out"; then
    pass "$1"
  else
    bad "$1 (wrong error: $(grep -m1 ERROR <<<"$out"))"
  fi
}
tables() { sql -tAc "select string_agg(tablename, ',' order by tablename) from pg_tables where schemaname = 'public'"; }

# 1. Up on an empty database, then again with nothing pending.
make -s migrate DATABASE_URL="$URL" >/dev/null 2>&1 && pass "make migrate on an empty database" || bad "make migrate on an empty database"
make -s migrate DATABASE_URL="$URL" >/dev/null 2>&1 && pass "make migrate again with nothing pending" || bad "make migrate is not idempotent"

[[ "$(tables)" == "goose_db_version,products,sessions,users" ]] \
  && pass "tables: users, sessions, products" || bad "unexpected tables: $(tables)"

# 2. Constraints from the data model.
sql -c "insert into users (email, password_hash, shop_name) values ('a@example.com', 'x', 'A')" >/dev/null
expect_err "users.email is unique"                      23505 "insert into users (email, password_hash, shop_name) values ('a@example.com', 'x', 'B')"
expect_err "users.email must be stored lowercase"        23514 "insert into users (email, password_hash, shop_name) values ('B@example.com', 'x', 'B')"
expect_err "products.stock cannot go below zero"         23514 "insert into products (seller_id, name, price_cents, stock) select id, 'P', 100, -1 from users"
expect_err "products.price_cents cannot be negative"     23514 "insert into products (seller_id, name, price_cents, stock) select id, 'P', -1, 1 from users"
expect_err "products.seller_id must be a real user"      23503 "insert into products (seller_id, name, price_cents, stock) values (999999, 'P', 100, 1)"
expect_err "sessions.user_id must be a real user"        23503 "insert into sessions (id, user_id, expires_at) values ('s1', 999999, now())"
expect_err "users with products cannot be deleted"       23503 "with p as (insert into products (seller_id, name, price_cents, stock) select id, 'P', 100, 1 from users returning seller_id) delete from users where id in (select seller_id from p)"

if sql -c "insert into products (seller_id, name, price_cents, stock) select id, 'P', 100, 0 from users" >/dev/null 2>&1 \
  && [[ "$(sql -tAc "select count(*) from products where archived_at is null")" -ge 1 ]]; then
  pass "a valid product inserts with stock 0 and archived_at null"
else
  bad "a valid product could not be inserted"
fi

# 3. All the way down, then up again.
sql -c "truncate users, sessions, products" >/dev/null
(cd backend && go tool goose -dir migrations postgres "$URL" reset >/dev/null 2>&1) \
  && [[ "$(tables)" == "goose_db_version" ]] && pass "down migrations remove every table" || bad "down migrations left: $(tables)"
make -s migrate DATABASE_URL="$URL" >/dev/null 2>&1 && pass "make migrate works again after migrating down" || bad "make migrate failed after migrating down"

exit $fail
