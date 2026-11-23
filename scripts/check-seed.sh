#!/usr/bin/env bash
# Checks `make seed` and the safety guard on `make db-reset`:
#   - seed creates one demo seller with a bcrypt hash of DEMO_SELLER_PASSWORD, plus 3-5 products
#   - running seed again adds no duplicates and doesn't fail
#   - db-reset refuses to run when DATABASE_URL points anywhere but this machine
# Seeding runs in a throwaway database (shoplink_seedtest) that is dropped afterwards.
# Run with: make db-test (needs the local database from `make db-up`).
set -euo pipefail
cd "$(dirname "$0")/.."

# shellcheck source=/dev/null
set -a; . ./.env; set +a
: "${DEMO_SELLER_EMAIL:?DEMO_SELLER_EMAIL missing from .env; copy it from .env.example}"
: "${DEMO_SELLER_PASSWORD:?DEMO_SELLER_PASSWORD missing from .env; copy it from .env.example}"

DB=shoplink_seedtest
URL="postgres://shoplink:${SHOPLINK_DB_PASSWORD}@localhost:5432/${DB}?sslmode=disable"

fail=0
pass() { echo "ok   - $1"; }
bad()  { echo "FAIL - $1"; fail=1; }

admin() { docker compose exec -T db psql -X -q -v ON_ERROR_STOP=1 -U postgres -d postgres "$@"; }
sql()   { docker compose exec -T db psql -X -q -v ON_ERROR_STOP=1 -U postgres -d "$DB" -tA "$@"; }

cleanup() { admin -c "drop database if exists ${DB} with (force)" >/dev/null 2>&1 || true; }
trap cleanup EXIT
cleanup
admin -c "create database ${DB} owner shoplink" >/dev/null
make -s migrate DATABASE_URL="$URL" >/dev/null 2>&1

# 1. First seed.
make -s seed DATABASE_URL="$URL" >/dev/null 2>&1 && pass "make seed on a migrated database" || bad "make seed failed"

[[ "$(sql -c "select count(*) from users where email = '${DEMO_SELLER_EMAIL}'")" == "1" ]] \
  && pass "one demo seller with DEMO_SELLER_EMAIL" || bad "demo seller missing"

# Verify the hash independently of the Go code, with pgcrypto (throwaway database only).
sql -c "create extension if not exists pgcrypto" >/dev/null
[[ "$(sql -c "select password_hash like '\$2_\$%' and password_hash = crypt('${DEMO_SELLER_PASSWORD}', password_hash) from users where email = '${DEMO_SELLER_EMAIL}'")" == "t" ]] \
  && pass "password_hash is a bcrypt hash of DEMO_SELLER_PASSWORD" || bad "password_hash does not match DEMO_SELLER_PASSWORD"

products=$(sql -c "select count(*) from products p join users u on u.id = p.seller_id where u.email = '${DEMO_SELLER_EMAIL}'")
(( products >= 3 && products <= 5 )) && pass "demo seller has ${products} sample products" || bad "expected 3-5 products, got ${products}"

[[ "$(sql -c "select count(*) from products where price_sen <= 0 or stock < 0 or archived_at is not null")" == "0" ]] \
  && pass "sample products have positive prices and are not archived" || bad "a sample product has a bad price, stock, or is archived"

# Prices are rupiah in sen: anything under Rp 1.000 (100000 sen) is a leftover dollar-cents value.
[[ "$(sql -c "select count(*) from products where price_sen < 100000")" == "0" ]] \
  && pass "sample prices are realistic rupiah amounts in sen" || bad "a sample price is under Rp 1.000; prices must be in sen"
[[ "$(sql -c "select count(*) from products where price_sen > 2147483647")" -ge 1 ]] \
  && pass "a sample price is above the old integer limit" || bad "no sample price above Rp 21.474.836,47 to show bigint at work"

# 2. Second seed: no errors, no duplicates.
make -s seed DATABASE_URL="$URL" >/dev/null 2>&1 && pass "make seed again succeeds" || bad "make seed is not re-runnable"
[[ "$(sql -c "select count(*) from users")" == "1" && "$(sql -c "select count(*) from products")" == "$products" ]] \
  && pass "second seed adds no duplicate users or products" || bad "second seed created duplicates"

# 3. db-reset guard: must refuse non-local DATABASE_URLs before touching anything.
for remote in "postgres://u:p@db.example.com:5432/shoplink" "postgres://u:p@10.0.0.5/shoplink" "postgres://u:p@localhost.evil.com/shoplink"; do
  if out=$(make -s db-reset DATABASE_URL="$remote" 2>&1); then
    bad "db-reset ran against ${remote}"
  elif grep -q "refusing" <<<"$out"; then
    pass "db-reset refuses ${remote}"
  else
    bad "db-reset failed for ${remote} but not with the guard message"
  fi
done

exit $fail
