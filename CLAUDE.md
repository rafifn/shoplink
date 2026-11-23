# ShopLink

Portfolio project: sellers create products and shareable checkout links; buyers order without an account.
Design: docs/design.md. Read the section your ticket points to before starting.

## Stack
- backend/: Go 1.22+, net/http standard library routing, pgx, PostgreSQL, SQL migrations in backend/migrations
- frontend/: React + Vite + TypeScript, React Router
- e2e/: Playwright

## Commands
- make dev      # start Postgres (Docker Compose), API, and frontend
- make check    # format check, vet, lint, type-check, unit + integration tests. Must pass before any PR.
- make e2e      # Playwright end-to-end tests
- make db-shell # read-only psql into the LOCAL Docker database (user shoplink_readonly)
- make migration name=<name>  # create a new SQL migration file
- make migrate  # apply migrations to the local database
- make seed     # load the demo seller and sample products
- make db-reset # drop and rebuild the LOCAL database: migrations + seed

## How to work
1. Work on one GitHub issue at a time. Comment on the issue when you start (gh issue comment).
2. Branch name: <issue-number>-short-description.
3. Write a failing test for the acceptance criteria first, then the code.
4. Run make check after every change and fix all failures before moving on.
5. Open a PR whose description says "Closes #<issue>" and lists what you tested.
6. Never merge, never push to main, never change branch protection.

## Rules
- Every seller query filters by seller_id from the session. Return 404, not 403, for other sellers' records.
- Money is rupiah in integer sen: `bigint` columns named `*_sen` (Rp 1.000,75 is 100075). Never use floats for money.
- Stock changes happen inside a transaction with row locking.
- Errors are JSON: {"error": {"code": "...", "message": "..."}}.
- No secrets in code, tests, logs, or commits. Use environment variables.
- Keep changes inside the ticket's scope. If you find other problems, open a new issue instead.
- Database: only inspect the local Docker database, through make db-shell. Never connect to a hosted or production database.
- Change the schema only by adding a migration file. Never run ALTER, DROP, DELETE, or TRUNCATE by hand.
