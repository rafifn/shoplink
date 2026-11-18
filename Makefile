COMPOSE := docker compose
GOOSE := cd backend && go tool goose -dir migrations

# Local settings (DATABASE_URL, passwords). Values given on the command line win,
# e.g. make migrate DATABASE_URL=postgres://...
-include .env

.PHONY: db-up db-down db-shell db-test migration migrate

# Create .env from the committed example the first time.
.env:
	cp .env.example .env

## db-up: start the local Postgres and wait until it is healthy
db-up: .env
	$(COMPOSE) up -d --wait db

## db-down: stop the local Postgres (data is kept in the pgdata volume)
db-down:
	$(COMPOSE) down

## db-shell: read-only psql into the LOCAL database as shoplink_readonly
db-shell:
	$(COMPOSE) exec db psql -U shoplink_readonly shoplink

## db-test: check the database users and the migrations
db-test: db-up
	./scripts/check-db-users.sh
	./scripts/check-migrations.sh

## migration: create a new SQL migration file, e.g. make migration name=create_orders
migration:
	@test -n "$(name)" || { echo "usage: make migration name=<name>" >&2; exit 1; }
	$(GOOSE) -s create $(name) sql

## migrate: apply all pending migrations to DATABASE_URL (the local database by default)
migrate: .env
	@test -n "$(DATABASE_URL)" || { echo "DATABASE_URL is not set; see .env.example" >&2; exit 1; }
	$(GOOSE) postgres "$(DATABASE_URL)" up
