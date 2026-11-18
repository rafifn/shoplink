COMPOSE := docker compose
GOOSE := cd backend && go tool goose -dir migrations

# Local settings (DATABASE_URL, passwords). Values given on the command line win,
# e.g. make migrate DATABASE_URL=postgres://...
-include .env

.PHONY: db-up db-down db-shell db-test db-reset migration migrate seed

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

## db-test: check the database users, the migrations, the seed, and the db-reset guard
db-test: db-up
	./scripts/check-db-users.sh
	./scripts/check-migrations.sh
	./scripts/check-seed.sh

## db-reset: rebuild the LOCAL database from scratch (new volume, migrations, seed)
db-reset: .env
	@./scripts/db-reset.sh "$(DATABASE_URL)"

## migration: create a new SQL migration file, e.g. make migration name=create_orders
migration:
	@test -n "$(name)" || { echo "usage: make migration name=<name>" >&2; exit 1; }
	$(GOOSE) -s create $(name) sql

## migrate: apply all pending migrations to DATABASE_URL (the local database by default)
migrate: .env
	@test -n "$(DATABASE_URL)" || { echo "DATABASE_URL is not set; see .env.example" >&2; exit 1; }
	$(GOOSE) postgres "$(DATABASE_URL)" up

## seed: load the demo seller and sample products into DATABASE_URL (safe to run again)
seed: .env
	@test -n "$(DATABASE_URL)" || { echo "DATABASE_URL is not set; see .env.example" >&2; exit 1; }
	cd backend && DATABASE_URL="$(DATABASE_URL)" \
		DEMO_SELLER_EMAIL="$(DEMO_SELLER_EMAIL)" DEMO_SELLER_PASSWORD="$(DEMO_SELLER_PASSWORD)" \
		go run ./cmd/seed
