# ShopLink

ShopLink is a portfolio project: sellers add products, create a checkout form, and share its link, and buyers order through that link without an account. It is a React + TypeScript frontend talking to a Go JSON API backed by PostgreSQL, built with AI coding agents (Claude Code) writing most of the code from GitHub issues and me reviewing every pull request. See [docs/design.md](docs/design.md) for the full design.

## Local database

Needs Docker. `make db-up` creates `.env` from `.env.example` on first run and starts Postgres on `localhost:5432`.

| Command | What it does |
| --- | --- |
| `make db-up` | Start Postgres and wait until it is healthy |
| `make db-shell` | Read-only `psql` as `shoplink_readonly` |
| `make db-test` | Check both database users have the right access |
| `make db-down` | Stop Postgres (data is kept) |

DBeaver: connect to `localhost:5432`, database `shoplink`, as `shoplink` (read-write) or `shoplink_readonly`, with the passwords from `.env`. The users are created only when the data volume is new, so after changing passwords run `docker compose down -v` to start over.
