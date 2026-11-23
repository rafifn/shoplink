COMPOSE := docker compose
GOOSE := cd backend && go tool goose -dir migrations
# golangci-lint is pinned in its own modfile so its dependencies stay out of go.mod.
GOLANGCI_LINT := cd backend && go tool -modfile=golangci-lint.mod golangci-lint

# Local settings (DATABASE_URL, passwords). Values given on the command line win,
# e.g. make migrate DATABASE_URL=postgres://...
-include .env
# Passed to the API and the Go tests through the environment, so make never
# echoes the database password as part of a command line.
export DATABASE_URL PORT FRONTEND_ORIGIN

.PHONY: check go-check web-check dev e2e api db-up db-down db-shell db-test db-reset migration migrate seed

## check: everything a pull request must pass (Go and frontend)
check: go-check web-check

## dev: start Postgres, the Go API (:8080) and the Vite dev server (:5173); Ctrl-C stops them
dev: db-up
	@./scripts/dev.sh

## web-check: TypeScript type-check, ESLint, and Vitest for the frontend
web-check: frontend/node_modules
	cd frontend && npm run -s typecheck
	cd frontend && npm run -s lint
	cd frontend && npm test

# Install frontend dependencies when package-lock.json changes.
frontend/node_modules: frontend/package-lock.json
	cd frontend && npm ci
	@touch $@

## e2e: Playwright end-to-end tests in Chromium; starts the API and the frontend, or reuses `make dev`
e2e: db-up frontend/node_modules e2e/node_modules
	cd e2e && npx playwright install chromium
	cd e2e && npx playwright test

e2e/node_modules: e2e/package-lock.json
	cd e2e && npm ci
	@touch $@

## go-check: gofmt, go vet, golangci-lint, then unit and integration tests against the local Postgres
go-check: db-up
	@test -n "$(DATABASE_URL)" || { echo "DATABASE_URL is not set; see .env.example" >&2; exit 1; }
	@unformatted=$$(cd backend && gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt: these files need formatting (fix with: cd backend && gofmt -w .):" >&2; \
		echo "$$unformatted" >&2; exit 1; \
	fi
	cd backend && go vet ./...
	$(GOLANGCI_LINT) run ./...
	cd backend && go test -race -count=1 ./...

## api: run the Go API on PORT (default 8080) against the local database; Ctrl-C stops it
api: db-up
	cd backend && go run ./cmd/api

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
