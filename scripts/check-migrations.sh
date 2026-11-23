#!/usr/bin/env bash
# Checks the migrations in backend/migrations against a throwaway database:
#   - `make migrate` works on an empty database and again when nothing is pending
#   - the tables and constraints from docs/design.md exist and reject bad rows
#   - prices are bigint sen up to Number.MAX_SAFE_INTEGER, and migrating the money
#     change down keeps a price that fits in integer and refuses one that doesn't
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
expect_err "products.stock cannot go below zero"         23514 "insert into products (seller_id, name, price_sen, stock) select id, 'P', 100, -1 from users"
expect_err "products.price_sen cannot be negative"     23514 "insert into products (seller_id, name, price_sen, stock) select id, 'P', -1, 1 from users"
expect_err "products.seller_id must be a real user"      23503 "insert into products (seller_id, name, price_sen, stock) values (999999, 'P', 100, 1)"
expect_err "sessions.user_id must be a real user"        23503 "insert into sessions (id, user_id, expires_at) values ('s1', 999999, now())"
expect_err "users with products cannot be deleted"       23503 "with p as (insert into products (seller_id, name, price_sen, stock) select id, 'P', 100, 1 from users returning seller_id) delete from users where id in (select seller_id from p)"

if sql -c "insert into products (seller_id, name, price_sen, stock) select id, 'P', 100, 0 from users" >/dev/null 2>&1 \
  && [[ "$(sql -tAc "select count(*) from products where archived_at is null")" -ge 1 ]]; then
  pass "a valid product inserts with stock 0 and archived_at null"
else
  bad "a valid product could not be inserted"
fi

# 3. Money: bigint sen, capped at JavaScript's Number.MAX_SAFE_INTEGER.
INT4_MAX_PLUS_1=2147483648      # Rp 21.474.836,48, one sen above what integer can hold
JS_SAFE_MAX=9007199254740991
price_type=$(sql -tAc "select data_type from information_schema.columns where table_name = 'products' and column_name = 'price_sen'")
[[ "$price_type" == "bigint" ]] && pass "products.price_sen is bigint" || bad "products.price_sen type is '${price_type}', want bigint"

if sql -c "insert into products (seller_id, name, price_sen) select id, 'Big', ${INT4_MAX_PLUS_1} from users" >/dev/null 2>&1; then
  pass "a price above the old integer limit fits"
else
  bad "a price of ${INT4_MAX_PLUS_1} sen was rejected"
fi
if sql -c "insert into products (seller_id, name, price_sen) select id, 'Max', ${JS_SAFE_MAX} from users" >/dev/null 2>&1; then
  pass "a price of Number.MAX_SAFE_INTEGER sen fits"
else
  bad "a price of ${JS_SAFE_MAX} sen was rejected"
fi
expect_err "products.price_sen cannot exceed Number.MAX_SAFE_INTEGER" 23514 "insert into products (seller_id, name, price_sen) select id, 'P', $((JS_SAFE_MAX + 1)) from users"

# Down one step with data: refuse while a price is too big for integer, never truncate...
version() { sql -tAc "select max(version_id) from goose_db_version where is_applied"; }
before=$(version)
if (cd backend && go tool goose -dir migrations postgres "$URL" down >/dev/null 2>&1); then
  bad "migrating the money change down succeeded with a price too big for integer"
elif [[ "$(version)" == "$before" ]] && [[ "$(sql -tAc "select count(*) from products where price_sen = ${INT4_MAX_PLUS_1}")" == "1" ]]; then
  pass "migrating down refuses (and changes nothing) while a price is too big for integer"
else
  bad "a failed down migration left the schema or the data changed"
fi
# ...and keep prices that fit, in both directions.
sql -c "update products set price_sen = 2147483647 where price_sen > 2147483647" >/dev/null
if (cd backend && go tool goose -dir migrations postgres "$URL" down >/dev/null 2>&1) \
  && [[ "$(sql -tAc "select count(*) from products where price_cents = 2147483647")" == "2" ]] \
  && make -s migrate DATABASE_URL="$URL" >/dev/null 2>&1 \
  && [[ "$(sql -tAc "select count(*) from products where price_sen = 2147483647")" == "2" ]]; then
  pass "migrating down and up again keeps prices that fit in integer"
else
  bad "prices changed when migrating the money change down and up"
fi

# 4. All the way down, then up again.
sql -c "truncate users, sessions, products" >/dev/null
(cd backend && go tool goose -dir migrations postgres "$URL" reset >/dev/null 2>&1) \
  && [[ "$(tables)" == "goose_db_version" ]] && pass "down migrations remove every table" || bad "down migrations left: $(tables)"
make -s migrate DATABASE_URL="$URL" >/dev/null 2>&1 && pass "make migrate works again after migrating down" || bad "make migrate failed after migrating down"

exit $fail
