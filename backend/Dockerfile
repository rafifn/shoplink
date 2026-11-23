# The Go API. Built in CI (#10); the server only pulls the image.
# Build locally with: docker build -t shoplink-api backend

FROM golang:1.27.4-alpine AS build
WORKDIR /src
# Download modules first, so code changes don't invalidate this layer.
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Static binary (no cgo) so it runs on distroless; -trimpath drops local paths.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

# distroless: no shell or package manager, just the binary and CA certificates,
# running as an unprivileged user (uid 65532).
FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /out/api /api
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/api"]
