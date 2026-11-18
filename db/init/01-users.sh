#!/bin/sh
# Creates the two database users from docs/design.md (Database access):
#   shoplink           owns the shoplink database and its public schema; used by the app and migrations
#   shoplink_readonly  can only read; used by the agent through `make db-shell`
# Runs once, when the Postgres data volume is first created.
set -eu

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres \
  -v app_pw="$SHOPLINK_DB_PASSWORD" \
  -v ro_pw="$SHOPLINK_READONLY_PASSWORD" <<'SQL'
create role shoplink login password :'app_pw';
create role shoplink_readonly login password :'ro_pw';
create database shoplink owner shoplink;

\connect shoplink

alter schema public owner to shoplink;
revoke all on schema public from public;
grant usage on schema public to shoplink_readonly;

-- Read access to everything shoplink creates from now on (migrations run as shoplink).
alter default privileges for role shoplink in schema public grant select on tables to shoplink_readonly;
alter default privileges for role shoplink in schema public grant select on sequences to shoplink_readonly;

-- Belt and braces: read-only sessions refuse writes even if a grant slips through.
alter role shoplink_readonly set default_transaction_read_only = on;
SQL
