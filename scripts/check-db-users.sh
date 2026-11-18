#!/usr/bin/env bash
# Checks the local database users behave as docs/design.md says:
#   - shoplink owns the schema, is not a superuser, and can log in with its .env password
#   - shoplink_readonly can log in and SELECT, including tables created later by shoplink
#   - shoplink_readonly cannot INSERT, UPDATE, DELETE, or CREATE
# Run with: make db-test. Leaves no trace: the probe table lives in a rolled-back transaction.
set -euo pipefail
cd "$(dirname "$0")/.."

# shellcheck source=/dev/null
set -a; . ./.env; set +a

fail=0
pass() { echo "ok   - $1"; }
bad()  { echo "FAIL - $1"; fail=1; }

# psql inside the db container. Unix-socket logins (-U only) skip passwords;
# TCP logins (-h 127.0.0.1) check the password from .env.
as_super()    { docker compose exec -T db psql -X -q -v ON_ERROR_STOP=1 -U postgres -d shoplink "$@"; }
as_app_tcp()  { docker compose exec -T -e PGPASSWORD="$SHOPLINK_DB_PASSWORD" db psql -X -q -v ON_ERROR_STOP=1 -h 127.0.0.1 -U shoplink -d shoplink "$@"; }
as_ro_tcp()   { docker compose exec -T -e PGPASSWORD="$SHOPLINK_READONLY_PASSWORD" db psql -X -q -v ON_ERROR_STOP=1 -h 127.0.0.1 -U shoplink_readonly -d shoplink "$@"; }

# 1. Logins over TCP with the .env passwords
[[ "$(as_app_tcp -tAc 'select current_user' 2>/dev/null)" == "shoplink" ]] \
  && pass "shoplink logs in with its .env password" || bad "shoplink cannot log in over TCP"
[[ "$(as_ro_tcp -tAc 'select current_user' 2>/dev/null)" == "shoplink_readonly" ]] \
  && pass "shoplink_readonly logs in with its .env password" || bad "shoplink_readonly cannot log in over TCP"

# 2. Ownership and superuser status
[[ "$(as_super -tAc "select nspowner::regrole from pg_namespace where nspname = 'public'" 2>/dev/null)" == "shoplink" ]] \
  && pass "shoplink owns schema public" || bad "schema public is not owned by shoplink"
[[ "$(as_super -tAc "select rolsuper from pg_roles where rolname = 'shoplink'" 2>/dev/null)" == "f" ]] \
  && pass "shoplink is not a superuser" || bad "shoplink is a superuser (or missing)"

# 3. Read-only session: writes and DDL must fail, by privilege and by read-only session
[[ "$(as_super -tAc "select has_schema_privilege('shoplink_readonly', 'public', 'CREATE')" 2>/dev/null)" == "f" ]] \
  && pass "shoplink_readonly has no CREATE privilege on schema public" || bad "shoplink_readonly has CREATE on schema public"
if as_ro_tcp -c 'create table ro_probe (id int)' >/dev/null 2>&1; then
  bad "shoplink_readonly could CREATE a table"; as_super -c 'drop table if exists ro_probe' >/dev/null
else
  pass "shoplink_readonly cannot CREATE"
fi

# 4. Privileges on a table shoplink creates later (default privileges), checked
#    inside one transaction that is rolled back.
if as_super >/dev/null 2>&1 <<'SQL'
begin;
set local role shoplink;
create table public.__probe (id int);
insert into public.__probe values (1);
set local role shoplink_readonly;
do $$
begin
  perform * from public.__probe;
  begin
    insert into public.__probe values (2);
    raise exception 'readonly could INSERT';
  exception when insufficient_privilege then null;
  end;
  begin
    update public.__probe set id = 3;
    raise exception 'readonly could UPDATE';
  exception when insufficient_privilege then null;
  end;
  begin
    delete from public.__probe;
    raise exception 'readonly could DELETE';
  exception when insufficient_privilege then null;
  end;
end $$;
rollback;
SQL
then
  pass "shoplink_readonly can SELECT new shoplink tables but not INSERT/UPDATE/DELETE"
else
  bad "privileges on a new shoplink table are wrong"
fi

exit $fail
