# ShopLink

ShopLink is a portfolio project: sellers add products, create a checkout form, and share its link, and buyers order through that link without an account. It is a React + TypeScript frontend talking to a Go JSON API backed by PostgreSQL, built with AI coding agents (Claude Code) writing most of the code from GitHub issues and me reviewing every pull request. See [docs/design.md](docs/design.md) for the full design.

## API and checks

Needs Go and Docker.

| Command | What it does |
| --- | --- |
| `make api` | Run the Go API on `localhost:8080` against the local database; `curl localhost:8080/healthz` should return `{"status":"ok"}`. Ctrl-C stops it cleanly |
| `make check` | What every pull request must pass: `gofmt`, `go vet`, `golangci-lint`, and the Go unit and integration tests (starts Postgres if needed) |

The API reads `DATABASE_URL`, `PORT` (default 8080) and `FRONTEND_ORIGIN` from `.env`. `FRONTEND_ORIGIN` is the only browser origin allowed to call the API with cookies (the Vite dev server); leave it empty in production, where the app and the API share a domain.

## Local database

Needs Docker. `make db-up` creates `.env` from `.env.example` on first run and starts Postgres on `localhost:5432`.

| Command | What it does |
| --- | --- |
| `make db-up` | Start Postgres and wait until it is healthy |
| `make db-shell` | Read-only `psql` as `shoplink_readonly` |
| `make migrate` | Apply pending migrations from `backend/migrations` (needs Go) |
| `make migration name=<name>` | Create a new numbered SQL migration file |
| `make seed` | Load the demo seller (`DEMO_SELLER_EMAIL` / `DEMO_SELLER_PASSWORD` from `.env`) and 4 sample products; safe to run again |
| `make db-reset` | Rebuild the local database from scratch: new volume, migrations, seed (refuses non-local `DATABASE_URL`s) |
| `make db-test` | Check the database users, migrations, seed, and the `db-reset` guard |
| `make db-down` | Stop Postgres (data is kept) |

DBeaver: connect to `localhost:5432`, database `shoplink`, as `shoplink` (read-write) or `shoplink_readonly`, with the passwords from `.env`. The users are created only when the data volume is new, so after changing passwords run `docker compose down -v` to start over.
