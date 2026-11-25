#!/usr/bin/env bash
# Checks the production stack (deploy/docker-compose.prod.yml) on this machine
# with DOMAIN=localhost, which Caddy serves with its own locally trusted
# certificate:
#   - deploy/.env.example lists every variable the compose file uses
#   - the images build, and web + api + db start (the API migrates the database first)
#   - HTTPS: http redirects to https; the app, deep links (SPA fallback) and assets load
#   - /healthz and /api/* reach the Go API; no CORS headers in production
#   - the bundle calls the API with relative paths (no localhost:8080 baked in)
#   - db publishes no port, the API runs as a non-root user, everything restarts unless stopped
#   - the Playwright smoke test sees "API OK" over https://localhost
# It runs as its own compose project (shoplink-prodtest) with throwaway passwords
# and volumes, removed afterwards. Your dev database is untouched.
# Needs Docker and free ports 80 and 443. Run with: make prod-test
set -euo pipefail
cd "$(dirname "$0")/.."

PROJECT=shoplink-prodtest
COMPOSE_FILE=deploy/docker-compose.prod.yml
ENV_FILE=$(mktemp)
cat >"$ENV_FILE" <<EOF
DOMAIN=localhost
IMAGE_TAG=prodtest
POSTGRES_PASSWORD=prodtest-postgres-$RANDOM$RANDOM
SHOPLINK_DB_PASSWORD=prodtest-shoplink-$RANDOM$RANDOM
SHOPLINK_READONLY_PASSWORD=prodtest-readonly-$RANDOM$RANDOM
EOF

compose() { docker compose -p "$PROJECT" -f "$COMPOSE_FILE" --env-file "$ENV_FILE" "$@"; }
cleanup() {
  compose down -v --remove-orphans >/dev/null 2>&1 || true
  rm -f "$ENV_FILE"
}
trap cleanup EXIT

fail=0
pass() { echo "ok   - $1"; }
bad()  { echo "FAIL - $1"; fail=1; }
https() { curl -sSk --max-time 10 "$@"; } # -k: Caddy's local CA isn't trusted by curl

# 0. Every ${VAR} the compose file uses has a placeholder in deploy/.env.example.
missing=""
for v in $(grep -oE '\$\{[A-Z_]+' "$COMPOSE_FILE" | tr -d '${' | sort -u); do
  grep -q "^${v}=" deploy/.env.example || missing+=" $v"
done
[[ -z "$missing" ]] && pass "deploy/.env.example lists every variable" || bad "missing from deploy/.env.example:$missing"

for port in 80 443; do
  if nc -z localhost "$port" 2>/dev/null; then
    echo "port $port is already in use; stop whatever is using it and try again" >&2
    exit 1
  fi
done

# 1. Build and start. --wait returns once db is healthy and api and web are running.
if ! compose up -d --build --wait >/dev/null 2>&1; then
  bad "the production stack did not start"
  compose logs --tail 40 >&2 || true
  exit 1
fi
pass "images build and web, api, db start"

# The API listens only after migrating, and Caddy answers 502 until then.
for _ in $(seq 1 30); do
  [[ "$(https -o /dev/null -w '%{http_code}' https://localhost/healthz 2>/dev/null)" == "200" ]] && break
  sleep 1
done

# 2. HTTPS and routing.
[[ "$(https https://localhost/healthz)" == '{"status":"ok"}' ]] \
  && pass "https://localhost/healthz reaches the API and the database" || bad "/healthz did not answer {\"status\":\"ok\"}"

redirect=$(curl -sS --max-time 10 -o /dev/null -w '%{http_code} %{redirect_url}' http://localhost/some/page)
[[ "$redirect" == "308 https://localhost/some/page" ]] \
  && pass "http redirects to https" || bad "http did not redirect to https (got: $redirect)"

index=$(https https://localhost/)
grep -q '<div id="root">' <<<"$index" && pass "the app's index.html is served" || bad "https://localhost/ is not the app"

deep=$(https -w '\n%{http_code}' https://localhost/orders/123)
[[ "${deep##*$'\n'}" == "200" ]] && grep -q '<div id="root">' <<<"$deep" \
  && pass "deep links fall back to index.html" || bad "a deep link did not get index.html"

asset=$(grep -oE '/assets/[^"]+\.js' <<<"$index" | head -1)
bundle=$(https -w '\n%{http_code} %{content_type}' "https://localhost${asset}")
[[ -n "$asset" && "${bundle##*$'\n'}" == "200 text/javascript"* ]] \
  && pass "assets are served (${asset})" || bad "asset ${asset:-<none found>} not served as JavaScript: ${bundle##*$'\n'}"
grep -q 'localhost:8080' <<<"$bundle" \
  && bad "the bundle has localhost:8080 baked in (VITE_API_URL leaked into the build)" \
  || pass "the bundle calls the API with relative paths"

# Only the Go API answers unknown /api paths with its JSON 404; Caddy would serve index.html.
grep -q '"code":"not_found"' <<<"$(https https://localhost/api/no-such-route)" \
  && pass "/api/* is proxied to the Go API" || bad "/api/* did not reach the Go API"

https -D - -o /dev/null -H 'Origin: https://evil.example.com' https://localhost/healthz | grep -qi '^access-control-allow' \
  && bad "production sends CORS headers" || pass "no CORS headers in production"

# 3. Containers.
[[ -z "$(docker port "$(compose ps -q db)")" ]] \
  && pass "db publishes no port" || bad "db publishes a port: $(docker port "$(compose ps -q db)")"

api_user=$(docker inspect -f '{{.Config.User}}' "$(compose ps -q api)")
[[ -n "$api_user" && "$api_user" != "root" && "$api_user" != "0" && "$api_user" != 0:* ]] \
  && pass "the API runs as non-root user '${api_user}'" || bad "the API runs as root (User='${api_user}')"

policies=$(for s in web api db; do docker inspect -f '{{.HostConfig.RestartPolicy.Name}}' "$(compose ps -q "$s")"; done | sort -u)
[[ "$policies" == "unless-stopped" ]] && pass "web, api and db restart unless stopped" || bad "restart policies: $policies"

newest=$(ls backend/migrations/*.sql | tail -1 | xargs basename | cut -d_ -f1 | sed 's/^0*//')
applied=$(compose exec -T db psql -X -tA -U postgres -d shoplink -c "select max(version_id) from goose_db_version")
[[ "$applied" == "$newest" ]] && pass "the API applied every migration (version $applied)" || bad "database at version '$applied', newest migration is $newest"

# 4. The real browser check.
(cd e2e && E2E_BASE_URL=https://localhost E2E_IGNORE_HTTPS_ERRORS=1 npx playwright test >/dev/null 2>&1) \
  && pass "Playwright smoke test: the home page shows \"API OK\" over HTTPS" \
  || bad "Playwright smoke test failed against https://localhost (rerun: cd e2e && E2E_BASE_URL=https://localhost E2E_IGNORE_HTTPS_ERRORS=1 npx playwright test)"

exit $fail
