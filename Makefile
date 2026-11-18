COMPOSE := docker compose

.PHONY: db-up db-down db-shell db-test

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

## db-test: check the two database users have the right access
db-test: db-up
	./scripts/check-db-users.sh
