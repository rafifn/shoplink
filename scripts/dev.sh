#!/usr/bin/env bash
# Runs the Go API and the Vite dev server together, against the local Postgres
# that `make dev` has already started. Ctrl-C stops both; if either one exits
# (a build error, a busy port), the other is stopped too.
#
# Run with: make dev (it passes DATABASE_URL, PORT and FRONTEND_ORIGIN from .env).
set -euo pipefail
cd "$(dirname "$0")/.."

# Build first, so a compile error stops here instead of scrolling past Vite's output.
(cd backend && go build -o bin/api ./cmd/api)
[[ -d frontend/node_modules ]] || (cd frontend && npm ci)

api_pid=""
web_pid=""
stop() {
  trap - INT TERM EXIT
  # `|| true`: one of them may already be gone, and set -e must not cut the cleanup short.
  [[ -z "$api_pid" ]] || kill "$api_pid" 2>/dev/null || true
  [[ -z "$web_pid" ]] || kill "$web_pid" 2>/dev/null || true
  wait 2>/dev/null || true
  echo "dev: stopped"
}
# Ctrl-C reaches the API and Vite directly too (same process group); a stop
# you asked for is a success, not "one process exited".
trap 'stop; exit 0' INT TERM
trap stop EXIT

backend/bin/api &
api_pid=$!
# exec, so the pid is Vite itself and the kill above reaches it.
(cd frontend && exec ./node_modules/.bin/vite) &
web_pid=$!

echo "dev: app on ${FRONTEND_ORIGIN:-http://localhost:5173}, API on http://localhost:${PORT:-8080}. Ctrl-C stops both."

# macOS ships bash 3.2, which has no `wait -n`, so poll instead.
while kill -0 "$api_pid" 2>/dev/null && kill -0 "$web_pid" 2>/dev/null; do
  sleep 1
done
echo "dev: one process exited; stopping the other" >&2
exit 1
