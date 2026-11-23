# Caddy with the built React app. Built in CI (#10) from the repo root:
#   docker build -f deploy/web.Dockerfile -t shoplink-web .
# The server only pulls the image; nothing is built there.

FROM node:22-alpine AS build
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
# Production calls the API on the same domain with relative paths (/api, /healthz),
# so no API origin is baked into the bundle. This also overrides any VITE_API_URL
# a .env file could carry (web.Dockerfile.dockerignore keeps .env out anyway).
ENV VITE_API_URL=""
RUN npm run build

FROM caddy:2.11-alpine
COPY deploy/Caddyfile /etc/caddy/Caddyfile
COPY --from=build /app/frontend/dist /srv
